package service

import (
	"context"
	"fmt"
	"io"
	"log/slog"
	"strings"
	"sync"
	"time"

	"github.com/google/uuid"
	"ipmanlk/plume/internal/apperr"
	"ipmanlk/plume/internal/domain"
	"ipmanlk/plume/internal/lexorank"
	"ipmanlk/plume/internal/port"
)

// recurringCheckInterval is the minimum time between recurring-template
// scans. Prevents a DB scan on every task-list request (the call site).
const recurringCheckInterval = 1 * time.Minute

type TaskTemplateService struct {
	templateRepo     port.TaskTemplateRepository
	taskRepo         port.TaskRepository
	statusRepo       port.TaskStatusRepository
	projRepo         port.ProjectRepository
	userRepo         port.UserRepository
	taskSvc          port.TaskService
	log              *slog.Logger
	mu               sync.Mutex
	lastRecurringRun time.Time
}

var _ port.TaskTemplateService = (*TaskTemplateService)(nil)

// SetTaskService injects the task service so Instantiate can delegate to
// TaskService.Create for proper notifications, WS broadcasts, and activity
// recording. Optional: when nil, Instantiate creates the task directly.
func (s *TaskTemplateService) SetTaskService(svc port.TaskService) {
	s.taskSvc = svc
}

func NewTaskTemplateService(
	templateRepo port.TaskTemplateRepository,
	taskRepo port.TaskRepository,
	statusRepo port.TaskStatusRepository,
	projRepo port.ProjectRepository,
	userRepo port.UserRepository,
	log *slog.Logger,
) *TaskTemplateService {
	if log == nil {
		log = slog.New(slog.NewTextHandler(io.Discard, nil)) // no-op fallback
	}
	return &TaskTemplateService{
		templateRepo: templateRepo,
		taskRepo:     taskRepo,
		statusRepo:   statusRepo,
		projRepo:     projRepo,
		userRepo:     userRepo,
		log:          log,
	}
}

func (s *TaskTemplateService) List(ctx context.Context, orgID, projectID string) ([]*domain.TaskTemplate, error) {
	return s.templateRepo.ListByProject(ctx, orgID, projectID)
}

func (s *TaskTemplateService) Get(ctx context.Context, orgID, projectID, id string) (*domain.TaskTemplate, error) {
	t, err := s.templateRepo.GetByID(ctx, orgID, id)
	if err != nil {
		return nil, apperr.NotFound("task template", err)
	}
	if t.ProjectID != projectID {
		return nil, apperr.ErrForbidden
	}
	return t, nil
}

var validRecurrencePatterns = map[string]bool{
	domain.RecurrenceNone:    true,
	domain.RecurrenceDaily:   true,
	domain.RecurrenceWeekly:  true,
	domain.RecurrenceMonthly: true,
}

func (s *TaskTemplateService) Create(ctx context.Context, p domain.CreateTaskTemplateParams) (*domain.TaskTemplate, error) {
	p.Name = strings.TrimSpace(p.Name)
	if p.Name == "" {
		return nil, apperr.InvalidInput("name is required")
	}
	if p.Priority == "" {
		p.Priority = domain.PriorityNone
	}
	if !validRecurrencePatterns[p.RecurrencePattern] {
		p.RecurrencePattern = domain.RecurrenceNone
	}

	// Validate project + status belong to org
	proj, err := s.projRepo.GetByID(ctx, p.OrgID, p.ProjectID)
	if err != nil {
		return nil, fmt.Errorf("get project: %w", err)
	}
	if proj.OrgID != p.OrgID {
		return nil, apperr.InvalidInput("project does not belong to organization")
	}
	status, err := s.statusRepo.GetByID(ctx, p.StatusID)
	if err != nil {
		return nil, fmt.Errorf("get status: %w", err)
	}
	if status.ProjectID != p.ProjectID {
		return nil, apperr.InvalidInput("status does not belong to project")
	}

	// Validate assignees
	if len(p.AssigneeIDs) > 0 {
		users, err := s.userRepo.ListByIDs(ctx, p.AssigneeIDs)
		if err != nil {
			return nil, fmt.Errorf("validate assignees: %w", err)
		}
		found := make(map[string]bool, len(users))
		for _, u := range users {
			if u.OrgID == p.OrgID && u.IsActive {
				found[u.ID] = true
			}
		}
		for _, id := range p.AssigneeIDs {
			if !found[id] {
				return nil, apperr.InvalidInput("assignee " + id + " is not a member of this organization")
			}
		}
	}

	// Compute next_run_at for recurring templates
	var nextRunAt *time.Time
	if p.RecurrencePattern != domain.RecurrenceNone {
		nextRunAt = computeNextRun(p.RecurrencePattern, p.RecurrenceDays, time.Now())
	}

	t := &domain.TaskTemplate{
		ID:                uuid.New().String(),
		OrgID:             p.OrgID,
		ProjectID:         p.ProjectID,
		Name:              p.Name,
		Description:       p.Description,
		Priority:          p.Priority,
		StatusID:          p.StatusID,
		AssigneeIDs:       p.AssigneeIDs,
		Estimate:          p.Estimate,
		RecurrencePattern: p.RecurrencePattern,
		RecurrenceDays:    p.RecurrenceDays,
		NextRunAt:         nextRunAt,
		CreatedBy:         p.CreatedBy,
	}

	if err := s.templateRepo.Create(ctx, t); err != nil {
		return nil, fmt.Errorf("create template: %w", err)
	}
	return t, nil
}

func (s *TaskTemplateService) Update(ctx context.Context, p domain.UpdateTaskTemplateParams) (*domain.TaskTemplate, error) {
	existing, err := s.templateRepo.GetByID(ctx, p.OrgID, p.ID)
	if err != nil {
		return nil, apperr.NotFound("task template", err)
	}
	// Prevent cross-project IDOR: the template must belong to the project in
	// the request URL. A user with PermProjectManage on project A must not be
	// able to mutate a template from project B by swapping the templateId.
	if existing.ProjectID != p.ProjectID {
		return nil, apperr.ErrForbidden
	}

	p.Name = strings.TrimSpace(p.Name)
	if p.Name == "" {
		return nil, apperr.InvalidInput("name is required")
	}
	if !validRecurrencePatterns[p.RecurrencePattern] {
		p.RecurrencePattern = domain.RecurrenceNone
	}

	// Validate status
	status, err := s.statusRepo.GetByID(ctx, p.StatusID)
	if err != nil {
		return nil, fmt.Errorf("get status: %w", err)
	}
	if status.ProjectID != existing.ProjectID {
		return nil, apperr.InvalidInput("status does not belong to project")
	}

	// Recompute next_run if recurrence changed
	var nextRunAt *time.Time
	if p.RecurrencePattern != domain.RecurrenceNone {
		if existing.NextRunAt != nil && existing.RecurrencePattern == p.RecurrencePattern && existing.RecurrenceDays == p.RecurrenceDays {
			nextRunAt = existing.NextRunAt
		} else {
			nextRunAt = computeNextRun(p.RecurrencePattern, p.RecurrenceDays, time.Now())
		}
	}

	existing.Name = p.Name
	existing.Description = p.Description
	existing.Priority = p.Priority
	existing.StatusID = p.StatusID
	existing.AssigneeIDs = p.AssigneeIDs
	existing.Estimate = p.Estimate
	existing.RecurrencePattern = p.RecurrencePattern
	existing.RecurrenceDays = p.RecurrenceDays
	existing.NextRunAt = nextRunAt

	if err := s.templateRepo.Update(ctx, existing); err != nil {
		return nil, fmt.Errorf("update template: %w", err)
	}
	return existing, nil
}

func (s *TaskTemplateService) Delete(ctx context.Context, orgID, projectID, id string) error {
	// Verify ownership before deleting to prevent cross-project deletion by ID.
	if _, err := s.Get(ctx, orgID, projectID, id); err != nil {
		return err
	}
	return s.templateRepo.Delete(ctx, orgID, id)
}

// Instantiate creates a real task from a template. If the template is
// recurring, the next_run_at is advanced to the next occurrence.
func (s *TaskTemplateService) Instantiate(ctx context.Context, orgID, projectID, id, createdBy string) (*domain.Task, error) {
	tmpl, err := s.Get(ctx, orgID, projectID, id)
	if err != nil {
		return nil, err
	}

	// When a TaskService is wired, delegate to Create so the new task gets
	// proper notifications, WS broadcasts, and activity recording; same as
	// any manually created task.
	if s.taskSvc != nil {
		task, err := s.taskSvc.Create(ctx, domain.CreateTaskParams{
			OrgID:       orgID,
			ProjectID:   tmpl.ProjectID,
			CreatedBy:   createdBy,
			Title:       tmpl.Name,
			Description: tmpl.Description,
			StatusID:    tmpl.StatusID,
			Priority:    tmpl.Priority,
			AssigneeIDs: tmpl.AssigneeIDs,
			Estimate:    tmpl.Estimate,
		})
		if err != nil {
			return nil, fmt.Errorf("create task from template: %w", err)
		}
		// Stamp the template_id on the created task so we can trace it back.
		task.TemplateID = &tmpl.ID
		_ = s.taskRepo.Update(ctx, task)

		// Advance next_run_at for recurring templates
		if tmpl.RecurrencePattern != domain.RecurrenceNone {
			nextRun := computeNextRun(tmpl.RecurrencePattern, tmpl.RecurrenceDays, time.Now())
			if err := s.templateRepo.UpdateNextRun(ctx, orgID, id, nextRun); err != nil {
				return nil, fmt.Errorf("update next run: %w", err)
			}
		}
		return task, nil
	}

	// Fallback: create directly (no notifications/broadcast) when TaskService
	// is not wired (e.g. in unit tests).
	lastKey, err := s.taskRepo.GetLastPositionKey(ctx, orgID, tmpl.ProjectID, tmpl.StatusID)
	if err != nil {
		return nil, fmt.Errorf("get last position key: %w", err)
	}
	positionKey, err := lexorank.GenerateKeyBetween(lastKey, "")
	if err != nil {
		return nil, fmt.Errorf("generate position key: %w", err)
	}

	status, err := s.statusRepo.GetByID(ctx, tmpl.StatusID)
	if err != nil {
		return nil, fmt.Errorf("get status: %w", err)
	}

	now := time.Now()
	task := &domain.Task{
		ID:          uuid.New().String(),
		OrgID:       orgID,
		ProjectID:   tmpl.ProjectID,
		CreatedBy:   createdBy,
		Title:       tmpl.Name,
		Description: tmpl.Description,
		StatusID:    tmpl.StatusID,
		Priority:    tmpl.Priority,
		PositionKey: positionKey,
		Estimate:    tmpl.Estimate,
		CompletedAt: completedAtForStatusCategory(status.Category, now),
		CreatedAt:   now,
		UpdatedAt:   now,
		TemplateID:  &tmpl.ID,
	}

	assignees := make([]domain.TaskAssignee, len(tmpl.AssigneeIDs))
	for i, id := range tmpl.AssigneeIDs {
		assignees[i] = domain.TaskAssignee{ID: id}
	}
	task.Assignees = assignees

	if err := s.taskRepo.Create(ctx, task); err != nil {
		return nil, fmt.Errorf("create task from template: %w", err)
	}

	// Advance next_run_at for recurring templates
	if tmpl.RecurrencePattern != domain.RecurrenceNone {
		nextRun := computeNextRun(tmpl.RecurrencePattern, tmpl.RecurrenceDays, now)
		if err := s.templateRepo.UpdateNextRun(ctx, orgID, id, nextRun); err != nil {
			return nil, fmt.Errorf("update next run: %w", err)
		}
	}

	return task, nil
}

// ProcessDueRecurring finds all recurring templates whose next_run_at has
// passed and instantiates a task for each. Uses an in-memory throttle to
// avoid a DB scan on every task-list request, and uses compare-and-set
// (ClaimDueRecurring) to prevent duplicate instantiation under concurrent
// calls; even if two goroutines find the same due template, only one
// claims and instantiates it.
func (s *TaskTemplateService) ProcessDueRecurring(ctx context.Context) error {
	s.mu.Lock()
	if time.Since(s.lastRecurringRun) < recurringCheckInterval {
		s.mu.Unlock()
		return nil
	}
	s.lastRecurringRun = time.Now()
	s.mu.Unlock()

	templates, err := s.templateRepo.ListDueRecurring(ctx, time.Now())
	if err != nil {
		return fmt.Errorf("list due recurring: %w", err)
	}
	for _, tmpl := range templates {
		if tmpl.NextRunAt == nil {
			s.log.Debug("recurring template skipped: next_run_at is nil", "template_id", tmpl.ID)
			continue
		}
		newNext := computeNextRun(tmpl.RecurrencePattern, tmpl.RecurrenceDays, time.Now())
		if newNext == nil {
			s.log.Debug("recurring template skipped: no next run computed", "template_id", tmpl.ID, "pattern", tmpl.RecurrencePattern)
			continue
		}
		claimed, err := s.templateRepo.ClaimDueRecurring(ctx, tmpl.OrgID, tmpl.ID, tmpl.NextRunAt, newNext)
		if err != nil {
			s.log.Warn("failed to claim recurring template", "template_id", tmpl.ID, "error", err)
			continue
		}
		if !claimed {
			s.log.Debug("recurring template already claimed by another goroutine", "template_id", tmpl.ID)
			continue // another goroutine already claimed this template
		}
		// next_run_at is already advanced by the CAS update, so even if
		// Instantiate fails we won't re-claim with the old value.
		if _, err := s.Instantiate(ctx, tmpl.OrgID, tmpl.ProjectID, tmpl.ID, tmpl.CreatedBy); err != nil {
			s.log.Error("failed to instantiate recurring task",
				"template_id", tmpl.ID, "project_id", tmpl.ProjectID, "error", err)
			// Persist the error so operators have visibility into the silent
			// failure. Best-effort: never blocks the loop.
			_ = s.templateRepo.SetLastError(ctx, tmpl.OrgID, tmpl.ID, err.Error())
			continue
		}
		// Success; clear any prior error.
		_ = s.templateRepo.SetLastError(ctx, tmpl.OrgID, tmpl.ID, "")
	}
	return nil
}

// computeNextRun calculates the next occurrence time for a recurrence pattern.
func computeNextRun(pattern, days string, from time.Time) *time.Time {
	switch pattern {
	case domain.RecurrenceDaily:
		t := from.AddDate(0, 0, 1)
		return &t
	case domain.RecurrenceWeekly:
		// days = "0,1,2" (0=Sunday)
		if days == "" {
			t := from.AddDate(0, 0, 7)
			return &t
		}
		// Find the next matching day of week
		dayNums := parseDayNums(days)
		if len(dayNums) == 0 {
			t := from.AddDate(0, 0, 7)
			return &t
		}
		for i := 1; i <= 14; i++ {
			candidate := from.AddDate(0, 0, i)
			for _, d := range dayNums {
				if int(candidate.Weekday()) == d {
					return &candidate
				}
			}
		}
		t := from.AddDate(0, 0, 7)
		return &t
	case domain.RecurrenceMonthly:
		// days = "15" (day of month)
		if days == "" {
			t := from.AddDate(0, 1, 0)
			return &t
		}
		dayNums := parseDayNums(days)
		if len(dayNums) > 0 {
			day := dayNums[0]
			if day < 1 {
				day = 1
			}
			if day > 28 {
				day = 28 // Clamp to 28 to avoid month-length overflow
			}
			// Next month, on the target day.
			t := time.Date(from.Year(), from.Month(), 1, 0, 0, 0, 0, from.Location()).AddDate(0, 1, day-1)
			return &t
		}
		t := from.AddDate(0, 1, 0)
		return &t
	default:
		return nil
	}
}

func parseDayNums(s string) []int {
	var out []int
	for _, part := range strings.Split(s, ",") {
		part = strings.TrimSpace(part)
		if part == "" {
			continue
		}
		var n int
		if _, err := fmt.Sscanf(part, "%d", &n); err == nil {
			out = append(out, n)
		}
	}
	return out
}
