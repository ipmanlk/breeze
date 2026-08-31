package service

import (
	"context"
	"fmt"
	"time"

	"ipmanlk/plume/internal/apperr"
	"ipmanlk/plume/internal/domain"
	"ipmanlk/plume/internal/lexorank"
	"ipmanlk/plume/internal/port"

	"github.com/google/uuid"
)

type TaskService struct {
	taskRepo     port.TaskRepository
	projRepo     port.ProjectRepository
	statusRepo   port.TaskStatusRepository
	cycleRepo    port.CycleRepository
	notifSvc     port.NotificationService
	userRepo     port.UserRepository
	convRepo     port.ConversationRepository
	mentions     mentionHydrator
	broadcaster  port.Broadcaster
	activityRepo port.TaskActivityRepository
	access       port.AccessChecker
}

var _ port.TaskService = (*TaskService)(nil)

type TaskServiceDeps struct {
	TaskRepo     port.TaskRepository
	ProjRepo     port.ProjectRepository
	StatusRepo   port.TaskStatusRepository
	CycleRepo    port.CycleRepository
	NotifSvc     port.NotificationService
	UserRepo     port.UserRepository
	ConvRepo     port.ConversationRepository
	Broadcaster  port.Broadcaster
	ActivityRepo port.TaskActivityRepository
	Access       port.AccessChecker
}

func NewTaskService(
	taskRepo port.TaskRepository,
	projRepo port.ProjectRepository,
	statusRepo port.TaskStatusRepository,
	cycleRepo port.CycleRepository,
	notifSvc port.NotificationService,
	userRepo port.UserRepository,
	convRepo port.ConversationRepository,
) *TaskService {
	return &TaskService{
		taskRepo:   taskRepo,
		projRepo:   projRepo,
		statusRepo: statusRepo,
		cycleRepo:  cycleRepo,
		notifSvc:   notifSvc,
		userRepo:   userRepo,
		convRepo:   convRepo,
		mentions:   newMentionHydrator(userRepo, projRepo, taskRepo, convRepo),
	}
}

// NewTaskServiceWithDeps constructs a TaskService with all dependencies,
// including an optional broadcaster for real-time WS task events. Prefer
// this constructor when wiring in app.go.
func NewTaskServiceWithDeps(deps TaskServiceDeps) *TaskService {
	return &TaskService{
		taskRepo:     deps.TaskRepo,
		projRepo:     deps.ProjRepo,
		statusRepo:   deps.StatusRepo,
		cycleRepo:    deps.CycleRepo,
		notifSvc:     deps.NotifSvc,
		userRepo:     deps.UserRepo,
		convRepo:     deps.ConvRepo,
		mentions:     newMentionHydrator(deps.UserRepo, deps.ProjRepo, deps.TaskRepo, deps.ConvRepo),
		broadcaster:  deps.Broadcaster,
		activityRepo: deps.ActivityRepo,
		access:       deps.Access,
	}
}

var validPriorities = map[string]bool{
	domain.PriorityNone: true, domain.PriorityLow: true, domain.PriorityMedium: true, domain.PriorityHigh: true, domain.PriorityUrgent: true,
}

const maxBatchSize = 500

// broadcastTask publishes a task lifecycle event to the project room so
// connected clients see real-time updates. It is best-effort: a broadcast
// failure is logged but never blocks the caller.
func (s *TaskService) broadcastTask(eventType domain.WsMessageType, orgID, projectID string, task *domain.Task) {
	if s.broadcaster == nil || task == nil {
		return
	}
	_ = s.broadcaster.Broadcast(
		domain.RoomKeyProject(orgID, projectID),
		string(eventType),
		map[string]any{"task": task},
	)
}

// recordActivity persists a task activity entry. Best-effort: errors are
// logged but never returned to the caller so a logging failure cannot block
// a task mutation. No-op when activityRepo is nil.
// broadcastTaskActivity broadcasts a task_activity_recorded WS event to the
// project room so open task-detail dialogs can refresh their activity feed.
// Best-effort: broadcast errors are ignored (never blocks the caller).
func (s *TaskService) broadcastTaskActivity(taskID, orgID, projectID string) {
	if s.broadcaster == nil {
		return
	}
	_ = s.broadcaster.Broadcast(
		domain.RoomKeyProject(orgID, projectID),
		string(domain.WsTypeTaskActivityRecorded),
		map[string]any{"task_id": taskID},
	)
}

// recordActivity persists a task activity entry. Best-effort: errors are
// logged but never returned to the caller so a logging failure cannot block
// a task mutation. No-op when activityRepo is nil.
func (s *TaskService) recordActivity(ctx context.Context, task *domain.Task, actorID string, action domain.ActivityAction, field, oldVal, newVal string) {
	if s.activityRepo == nil {
		return
	}
	_ = s.activityRepo.Create(ctx, &domain.TaskActivity{
		ID:        uuid.New().String(),
		TaskID:    task.ID,
		OrgID:     task.OrgID,
		ProjectID: task.ProjectID,
		ActorID:   actorID,
		Action:    action,
		Field:     field,
		OldValue:  oldVal,
		NewValue:  newVal,
	})
	s.broadcastTaskActivity(task.ID, task.OrgID, task.ProjectID)
}

// isCompletedStatusCategory reports whether tasks in this status category
// are considered finished (done or canceled). These are the categories that
// should have completed_at set.
func isCompletedStatusCategory(category string) bool {
	return category == domain.StatusCategoryDone || category == domain.StatusCategoryCanceled
}

// completedAtForStatusCategory returns a pointer to the given time when the
// status category represents a finished state; nil otherwise. This keeps the
// completed_at column in sync with the semantic status category.
func completedAtForStatusCategory(category string, now time.Time) *time.Time {
	if isCompletedStatusCategory(category) {
		return &now
	}
	return nil
}

// updatedCompletedAt determines the correct completed_at value after a status
// transition. Moving from a non-finished category to done/canceled stamps
// completed_at; moving out of done/canceled clears it; staying within the
// same finished/unfinished state leaves the existing value alone.
func updatedCompletedAt(current *time.Time, oldCategory, newCategory string, now time.Time) *time.Time {
	oldDone := isCompletedStatusCategory(oldCategory)
	newDone := isCompletedStatusCategory(newCategory)
	if !oldDone && newDone {
		return &now
	}
	if oldDone && !newDone {
		return nil
	}
	return current
}

// ptrTimeOrEmpty formats a *time.Time as a friendly date for activity log
// values, returning an empty string when the pointer is nil.
func ptrTimeOrEmpty(t *time.Time) string {
	if t == nil {
		return ""
	}
	return t.Format("Jan 2, 2006")
}

// truncateActivityText limits a string to n characters for activity log values.
// When the input exceeds n, it returns the first n characters followed by "…".
func truncateActivityText(s string, n int) string {
	if len(s) <= n {
		return s
	}
	return s[:n] + "…"
}

// priorityDisplayName returns a human-readable priority string suitable for
// activity log values. Maps lowercase constants to capitalized display names.
func priorityDisplayName(p string) string {
	switch p {
	case domain.PriorityNone:
		return "None"
	case domain.PriorityLow:
		return "Low"
	case domain.PriorityMedium:
		return "Medium"
	case domain.PriorityHigh:
		return "High"
	case domain.PriorityUrgent:
		return "Urgent"
	default:
		return p
	}
}

// ptrOrEmpty returns the string value of a *string pointer, or "" when nil.
func ptrOrEmpty(p *string) string {
	if p == nil {
		return ""
	}
	return *p
}

// ptrIntOrEmpty formats a *int as a string, returning "" when the pointer is nil.
func ptrIntOrEmpty(p *int) string {
	if p == nil {
		return ""
	}
	return fmt.Sprintf("%d", *p)
}

// cycleNameOrID returns the human-readable cycle name for a cycle ID, or "" when
// the cycle ID is empty. On any error (e.g. cycle not found), falls back to the
// raw cycle ID so activity recording never blocks on a missing lookup.
func (s *TaskService) cycleNameOrID(ctx context.Context, cycleID, projectID string) string {
	if cycleID == "" {
		return ""
	}
	if s.cycleRepo != nil {
		if c, err := s.cycleRepo.GetByID(ctx, cycleID, projectID); err == nil {
			return c.Name
		}
	}
	return cycleID
}

// validateAssignees checks that every user ID in ids belongs to orgID and is
// active. Returns apperr.InvalidInput if any ID is invalid. Empty/nil slices
// are allowed (clears assignees).
func (s *TaskService) validateAssignees(ctx context.Context, orgID string, ids []string) error {
	if len(ids) == 0 {
		return nil
	}
	users, err := s.userRepo.ListByIDs(ctx, ids)
	if err != nil {
		return fmt.Errorf("validate assignees: %w", err)
	}
	found := make(map[string]bool, len(users))
	for _, u := range users {
		if u.OrgID == orgID && u.IsActive {
			found[u.ID] = true
		}
	}
	for _, id := range ids {
		if !found[id] {
			return apperr.InvalidInput("assignee " + id + " is not a member of this organization")
		}
	}
	return nil
}

// createTaskRow is the transaction-capable write core of Create: it
// validates params, builds the row, and inserts it through the supplied
// repository. Side effects (notifications, WS broadcasts, activity) stay in
// Create so callers can run several inserts inside one transaction and emit
// events only after it commits.
func (s *TaskService) createTaskRow(ctx context.Context, repo port.TaskRepository, p domain.CreateTaskParams) (*domain.Task, error) {
	if p.Title == "" {
		return nil, apperr.InvalidInput("title is required")
	}
	if p.StatusID == "" {
		return nil, apperr.InvalidInput("status_id is required")
	}
	if !validPriorities[p.Priority] {
		p.Priority = domain.PriorityNone
	}

	// Defense-in-depth: verify the caller has permission to create tasks in
	// this project (duplicated from middleware for non-HTTP call paths).
	if s.access != nil {
		if err := s.access.RequireProjectAccess(ctx, p.CreatedBy, p.OrgID, p.ProjectID, domain.PermTaskCreate); err != nil {
			return nil, err
		}
	}

	status, err := s.statusRepo.GetByID(ctx, p.StatusID)
	if err != nil {
		return nil, fmt.Errorf("get status: %w", err)
	}
	if status.ProjectID != p.ProjectID {
		return nil, apperr.InvalidInput("status does not belong to project")
	}

	if p.CycleID != nil && *p.CycleID != "" {
		cycle, err := s.cycleRepo.GetByID(ctx, *p.CycleID, p.ProjectID)
		if err != nil {
			return nil, fmt.Errorf("get cycle: %w", err)
		}
		if cycle.ProjectID != p.ProjectID {
			return nil, apperr.InvalidInput("cycle does not belong to project")
		}
	}

	if p.ParentID != nil && *p.ParentID != "" {
		parent, err := repo.GetByID(ctx, p.OrgID, *p.ParentID, p.ProjectID)
		if err != nil {
			return nil, apperr.InvalidInput("parent task not found")
		}
		if parent.ProjectID != p.ProjectID {
			return nil, apperr.InvalidInput("parent_task does not belong to project")
		}
		// Enforce 1-level nesting: a subtask cannot have its own subtasks.
		// This also eliminates cycles by construction (no ancestor chain).
		if parent.ParentID != nil && *parent.ParentID != "" {
			return nil, apperr.InvalidInput("subtasks cannot have subtasks (maximum depth is 1)")
		}
	}

	if p.Estimate != nil && *p.Estimate < 0 {
		return nil, apperr.InvalidInput("estimate must be non-negative")
	}

	if err := s.validateAssignees(ctx, p.OrgID, p.AssigneeIDs); err != nil {
		return nil, err
	}

	positionKey, err := repo.GeneratePositionKey(ctx, p.OrgID, p.ProjectID, p.StatusID)
	if err != nil {
		return nil, fmt.Errorf("generate position key: %w", err)
	}

	// When creating a subtask, also generate a parent-scoped position so the
	// children can be reordered independently of the project-wide position_key.
	subtaskPosition := ""
	if p.ParentID != nil && *p.ParentID != "" {
		subPos, err := repo.GenerateSubtaskPositionKey(ctx, p.OrgID, *p.ParentID)
		if err != nil {
			return nil, fmt.Errorf("generate subtask position: %w", err)
		}
		subtaskPosition = subPos
	}

	now := time.Now()
	assignees := make([]domain.TaskAssignee, len(p.AssigneeIDs))
	for i, id := range p.AssigneeIDs {
		assignees[i] = domain.TaskAssignee{ID: id}
	}

	task := &domain.Task{
		ID:              uuid.New().String(),
		OrgID:           p.OrgID,
		ProjectID:       p.ProjectID,
		CycleID:         p.CycleID,
		ParentID:        p.ParentID,
		CreatedBy:       p.CreatedBy,
		Assignees:       assignees,
		Title:           p.Title,
		Description:     p.Description,
		StatusID:        p.StatusID,
		Priority:        p.Priority,
		PositionKey:     positionKey,
		SubtaskPosition: subtaskPosition,
		Estimate:        p.Estimate,
		StartedAt:       p.StartedAt,
		DueAt:           p.DueAt,
		CompletedAt:     completedAtForStatusCategory(status.Category, now),
		CreatedAt:       now,
		UpdatedAt:       now,
	}

	if err := repo.Create(ctx, task); err != nil {
		return nil, err
	}

	created, err := repo.GetByID(ctx, p.OrgID, task.ID, p.ProjectID)
	if err != nil {
		return nil, err
	}
	return created, nil
}

func (s *TaskService) Create(ctx context.Context, p domain.CreateTaskParams) (*domain.Task, error) {
	created, err := s.createTaskRow(ctx, s.taskRepo, p)
	if err != nil {
		return nil, err
	}

	project, err := s.projRepo.GetByID(ctx, p.OrgID, p.ProjectID)
	if err != nil {
		return nil, err
	}

	for _, a := range created.Assignees {
		if a.ID == p.CreatedBy {
			continue
		}
		if err := s.notifSvc.Notify(ctx, p.OrgID, a.ID, domain.NotifTaskAssigned,
			fmt.Sprintf("Assigned to: %s", created.Title),
			fmt.Sprintf("%s assigned you to %s", p.CreatedBy, created.Title),
			fmt.Sprintf("/projects/%s?task=%s", project.Slug, created.ID),
			"task", created.ID, p.CreatedBy); err != nil {
			// Best-effort: don't fail the create over a notification error.
		}
	}

	s.broadcastTask(domain.WsTypeTaskCreated, p.OrgID, p.ProjectID, created)
	s.recordActivity(ctx, created, p.CreatedBy, domain.ActivityCreated, "", "", created.Title)

	return created, nil
}

func (s *TaskService) List(ctx context.Context, orgID, projectID string, filter domain.TaskFilter) ([]*domain.Task, error) {
	tasks, err := s.taskRepo.ListByProject(ctx, orgID, projectID, filter)
	if err != nil {
		return nil, err
	}
	s.hydrateTasks(ctx, orgID, tasks)
	return tasks, nil
}

func (s *TaskService) ListTasks(ctx context.Context, orgID, userID string, role domain.Role, filter domain.TaskListFilter) (*domain.TaskListResult, error) {
	if filter.Limit <= 0 || filter.Limit > 50 {
		filter.Limit = 20
	}
	// Non-elevated roles (viewer/guest) must only see tasks in projects they
	// have explicit membership in. Elevated roles (owner/admin/member) have
	// implicit access to all projects in the org. This business rule lives in
	// the service, not the handler.
	filter.RequireProjectMembership = !domain.IsOrgElevatedRole(role)
	return s.taskRepo.ListByUser(ctx, orgID, userID, filter)
}

func (s *TaskService) GetByID(ctx context.Context, orgID, id, projectID string) (*domain.Task, error) {
	task, err := s.taskRepo.GetByID(ctx, orgID, id, projectID)
	if err != nil {
		return nil, err
	}
	s.hydrateTasks(ctx, orgID, []*domain.Task{task})
	return task, nil
}

// ListSubtasks returns the direct children of a task, enriched with assignees,
// labels, and resolved mentions. Verifies the parent task exists in the
// caller's org + project first.
func (s *TaskService) ListSubtasks(ctx context.Context, orgID, projectID, parentID string) ([]*domain.Task, error) {
	if _, err := s.taskRepo.GetByID(ctx, orgID, parentID, projectID); err != nil {
		return nil, apperr.NotFound("task", err)
	}
	subtasks, err := s.taskRepo.ListSubtasks(ctx, orgID, parentID)
	if err != nil {
		return nil, err
	}
	s.hydrateTasks(ctx, orgID, subtasks)
	return subtasks, nil
}

// ListActivity returns the activity history for a task. Verifies the task
// belongs to the caller's org + project before returning entries.
func (s *TaskService) ListActivity(ctx context.Context, orgID, projectID, taskID string, filter domain.TaskActivityFilter) (*domain.TaskActivityResult, error) {
	if _, err := s.taskRepo.GetByID(ctx, orgID, taskID, projectID); err != nil {
		return nil, err
	}
	if s.activityRepo == nil {
		return &domain.TaskActivityResult{Items: []*domain.TaskActivity{}}, nil
	}
	return s.activityRepo.List(ctx, taskID, filter)
}

func (s *TaskService) Update(ctx context.Context, actorID string, t *domain.Task) error {
	if t.Estimate != nil && *t.Estimate < 0 {
		return apperr.InvalidInput("estimate must be non-negative")
	}
	if t.CycleID != nil && *t.CycleID != "" {
		cycle, err := s.cycleRepo.GetByID(ctx, *t.CycleID, t.ProjectID)
		if err != nil {
			return fmt.Errorf("get cycle: %w", err)
		}
		if cycle.ProjectID != t.ProjectID {
			return apperr.InvalidInput("cycle does not belong to project")
		}
	}
	if t.ParentID != nil && *t.ParentID != "" {
		if *t.ParentID == t.ID {
			return apperr.InvalidInput("a task cannot be its own parent")
		}
		parent, err := s.taskRepo.GetByID(ctx, t.OrgID, *t.ParentID, t.ProjectID)
		if err != nil {
			return apperr.InvalidInput("parent task not found")
		}
		if parent.ProjectID != t.ProjectID {
			return apperr.InvalidInput("parent_task does not belong to project")
		}
		// Enforce 1-level nesting: reject if the prospective parent is itself
		// a subtask (would create a grandchild). Also blocks cycles since a
		// 1-level tree cannot loop.
		if parent.ParentID != nil && *parent.ParentID != "" {
			return apperr.InvalidInput("subtasks cannot have subtasks (maximum depth is 1)")
		}
	}

	assigneeIDs := make([]string, len(t.Assignees))
	for i, a := range t.Assignees {
		assigneeIDs[i] = a.ID
	}
	if err := s.validateAssignees(ctx, t.OrgID, assigneeIDs); err != nil {
		return err
	}

	old, err := s.taskRepo.GetByID(ctx, t.OrgID, t.ID, t.ProjectID)
	if err != nil {
		return fmt.Errorf("fetch old task: %w", err)
	}

	oldStatus := old.StatusID
	oldStatusObj, err := s.statusRepo.GetByID(ctx, oldStatus)
	if err != nil {
		return fmt.Errorf("fetch old status: %w", err)
	}
	newStatus, err := s.statusRepo.GetByID(ctx, t.StatusID)
	if err != nil {
		return fmt.Errorf("fetch new status: %w", err)
	}
	if newStatus.ProjectID != t.ProjectID {
		return apperr.InvalidInput("status does not belong to project")
	}

	oldAssignees := make(map[string]bool)
	for _, a := range old.Assignees {
		oldAssignees[a.ID] = true
	}

	if t.StatusID != oldStatus {
		pk, err := s.taskRepo.GeneratePositionKey(ctx, t.OrgID, t.ProjectID, t.StatusID)
		if err != nil {
			return fmt.Errorf("generate position key: %w", err)
		}
		t.PositionKey = pk
	}

	// Reparenting regenerates the parent-scoped subtask_position so the moved
	// task sorts at the end of its new parent's children (and a promoted task
	// clears it). When the parent is unchanged, preserve the existing value so
	// the UPDATE writes back the same key (the incoming request doesn't carry
	// subtask_position, so without this it would be nulled out).
	oldParentID := ""
	if old.ParentID != nil {
		oldParentID = *old.ParentID
	}
	newParentID := ""
	if t.ParentID != nil {
		newParentID = *t.ParentID
	}
	if oldParentID != newParentID {
		if newParentID != "" {
			subPos, err := s.taskRepo.GenerateSubtaskPositionKey(ctx, t.OrgID, newParentID)
			if err != nil {
				return fmt.Errorf("generate subtask position: %w", err)
			}
			t.SubtaskPosition = subPos
		} else {
			t.SubtaskPosition = ""
		}
	} else {
		t.SubtaskPosition = old.SubtaskPosition
	}

	now := time.Now()
	t.UpdatedAt = now
	// Sync completed_at with status category transitions. Moving into a done/
	// canceled category marks the task complete; moving out clears it.
	t.CompletedAt = updatedCompletedAt(old.CompletedAt, oldStatusObj.Category, newStatus.Category, now)
	if err := s.taskRepo.Update(ctx, t); err != nil {
		return err
	}

	updated, err := s.taskRepo.GetByID(ctx, t.OrgID, t.ID, t.ProjectID)
	if err != nil {
		return err
	}

	orgID := updated.OrgID

	project, err := s.projRepo.GetByID(ctx, orgID, updated.ProjectID)
	if err != nil {
		return err
	}

	if updated.StatusID != oldStatus {
		s.notifyStatusChange(ctx, updated, project.Slug, newStatus.Name, actorID)
	}

	newAssignees := make(map[string]bool)
	for _, a := range updated.Assignees {
		newAssignees[a.ID] = true
	}

	s.notifyNewAssignees(ctx, updated, oldAssignees, newAssignees, project.Slug, actorID)

	s.broadcastTask(domain.WsTypeTaskUpdated, orgID, updated.ProjectID, updated)
	s.recordTaskUpdateActivity(ctx, updated, old, actorID, oldAssignees, newAssignees, oldStatusObj.Name, newStatus.Name)

	return nil
}

// notifyStatusChange sends a "task moved to X" notification to all assignees
// when the task's status changes. Best-effort: notification errors are
// logged but don't fail the update.
func (s *TaskService) notifyStatusChange(ctx context.Context, updated *domain.Task, projectSlug, statusName, actorID string) {
	for _, a := range updated.Assignees {
		if err := s.notifSvc.Notify(ctx, updated.OrgID, a.ID, domain.NotifTaskStatusChanged,
			fmt.Sprintf("Status changed: %s", updated.Title),
			fmt.Sprintf("Task moved to %s", statusName),
			fmt.Sprintf("/projects/%s?task=%s", projectSlug, updated.ID),
			"task", updated.ID, actorID); err != nil {
		}
	}
}

// notifyNewAssignees sends "you were assigned" notifications to users who
// were added to the task. Best-effort: notification errors don't fail the update.
func (s *TaskService) notifyNewAssignees(ctx context.Context, updated *domain.Task, oldAssignees, newAssignees map[string]bool, projectSlug, actorID string) {
	for _, a := range updated.Assignees {
		if !oldAssignees[a.ID] {
			if err := s.notifSvc.Notify(ctx, updated.OrgID, a.ID, domain.NotifTaskAssigned,
				fmt.Sprintf("Assigned to: %s", updated.Title),
				fmt.Sprintf("You were assigned to %s", updated.Title),
				fmt.Sprintf("/projects/%s?task=%s", projectSlug, updated.ID),
				"task", updated.ID, actorID); err != nil {
			}
		}
	}
}

// recordTaskUpdateActivity records activity entries for the key field changes
// in a task update. Each change is a separate entry so the feed reads as a
// chronological list of atomic edits.
// oldStatusName / newStatusName are the resolved human-readable status names
// (not the raw IDs), used for status_changed recording.
func (s *TaskService) recordTaskUpdateActivity(ctx context.Context, updated, old *domain.Task, actorID string, oldAssignees, newAssignees map[string]bool, oldStatusName, newStatusName string) {
	if updated.Title != old.Title {
		s.recordActivity(ctx, updated, actorID, domain.ActivityTitleChanged, "title", old.Title, updated.Title)
	}
	if updated.Description != old.Description {
		s.recordActivity(ctx, updated, actorID, domain.ActivityDescriptionChanged, "description", truncateActivityText(old.Description, 120), truncateActivityText(updated.Description, 120))
	}
	if updated.StatusID != old.StatusID {
		s.recordActivity(ctx, updated, actorID, domain.ActivityStatusChanged, "status", oldStatusName, newStatusName)
	}
	if updated.Priority != old.Priority {
		s.recordActivity(ctx, updated, actorID, domain.ActivityPriorityChanged, "priority", priorityDisplayName(old.Priority), priorityDisplayName(updated.Priority))
	}
	oldDue := ptrTimeOrEmpty(old.DueAt)
	newDue := ptrTimeOrEmpty(updated.DueAt)
	if oldDue != newDue {
		s.recordActivity(ctx, updated, actorID, domain.ActivityDueDateChanged, "due_at", oldDue, newDue)
	}
	for _, a := range updated.Assignees {
		if !oldAssignees[a.ID] {
			s.recordActivity(ctx, updated, actorID, domain.ActivityAssigned, "assignee", "", a.Name)
		}
	}
	for _, a := range old.Assignees {
		if !newAssignees[a.ID] {
			s.recordActivity(ctx, updated, actorID, domain.ActivityUnassigned, "assignee", a.Name, "")
		}
	}

	// Estimate change.
	oldEst := ptrIntOrEmpty(old.Estimate)
	newEst := ptrIntOrEmpty(updated.Estimate)
	if oldEst != newEst {
		s.recordActivity(ctx, updated, actorID, domain.ActivityEstimateChanged, "estimate", oldEst, newEst)
	}

	// Cycle change.
	oldCycle := ptrOrEmpty(old.CycleID)
	newCycle := ptrOrEmpty(updated.CycleID)
	if oldCycle != newCycle {
		oldCycleName := s.cycleNameOrID(ctx, oldCycle, updated.ProjectID)
		newCycleName := s.cycleNameOrID(ctx, newCycle, updated.ProjectID)
		s.recordActivity(ctx, updated, actorID, domain.ActivityCycleChanged, "cycle", oldCycleName, newCycleName)
	}

	// Parent (reparenting) change.
	oldParent := ptrOrEmpty(old.ParentID)
	newParent := ptrOrEmpty(updated.ParentID)
	if oldParent != newParent {
		// Resolve parent task titles for readability; best-effort.
		oldParentTitle := oldParent
		if old.ParentID != nil && *old.ParentID != "" {
			if pt, err := s.taskRepo.GetByID(ctx, updated.OrgID, *old.ParentID, updated.ProjectID); err == nil {
				oldParentTitle = pt.Title
			}
		}
		newParentTitle := newParent
		if updated.ParentID != nil && *updated.ParentID != "" {
			if pt, err := s.taskRepo.GetByID(ctx, updated.OrgID, *updated.ParentID, updated.ProjectID); err == nil {
				newParentTitle = pt.Title
			}
		}
		s.recordActivity(ctx, updated, actorID, domain.ActivityReparented, "parent_task_id", oldParentTitle, newParentTitle)
	}

	// StartedAt change.
	oldStarted := ptrTimeOrEmpty(old.StartedAt)
	newStarted := ptrTimeOrEmpty(updated.StartedAt)
	if oldStarted != newStarted {
		s.recordActivity(ctx, updated, actorID, domain.ActivityStartedAtChanged, "started_at", oldStarted, newStarted)
	}
}

func (s *TaskService) Delete(ctx context.Context, orgID, id, projectID string, mode domain.DeleteSubtaskMode, actorID string) error {
	// Count direct children so we can enforce the chosen mode. 1-level nesting
	// is enforced at create/update, so no recursion is needed.
	count, err := s.taskRepo.CountSubtasks(ctx, orgID, id)
	if err != nil {
		return fmt.Errorf("count subtasks: %w", err)
	}
	if count > 0 && mode == domain.DeleteSubtaskModeBlock {
		return apperr.Conflict(fmt.Sprintf("task has %d subtask(s); specify cascade or promote mode", count))
	}

	// The whole delete (children + parent) must be atomic: a failure halfway
	// through cascade would otherwise leave half the subtree deleted while
	// the parent (or vice versa) survives.
	var children []*domain.Task
	err = s.taskRepo.RunInTransaction(ctx, func(repo port.TaskRepository) error {
		switch mode {
		case domain.DeleteSubtaskModeCascade:
			// Fetch children before deleting so we can broadcast each
			// deletion after commit.
			kids, err := repo.ListSubtasks(ctx, orgID, id)
			if err != nil {
				return fmt.Errorf("list subtasks for cascade: %w", err)
			}
			children = kids
			if err := repo.DeleteSubtasks(ctx, orgID, id); err != nil {
				return fmt.Errorf("delete subtasks: %w", err)
			}
		case domain.DeleteSubtaskModePromote:
			kids, err := repo.ListSubtasks(ctx, orgID, id)
			if err != nil {
				return fmt.Errorf("list subtasks for promote: %w", err)
			}
			if err := repo.PromoteSubtasks(ctx, orgID, id); err != nil {
				return fmt.Errorf("promote subtasks: %w", err)
			}
			children = kids
		}

		// No task_activity entry for the deletion itself: task_activity rows
		// cascade-delete with their task, so the entry would be erased the
		// instant it commits. Task deletions are recorded in the audit log
		// instead (see TaskHandler.Delete), which survives.
		return repo.Delete(ctx, orgID, id, projectID)
	})
	if err != nil {
		return err
	}

	for _, c := range children {
		if mode == domain.DeleteSubtaskModePromote {
			c.ParentID = nil
			s.broadcastTask(domain.WsTypeTaskUpdated, orgID, projectID, c)
		} else {
			s.broadcastTask(domain.WsTypeTaskDeleted, orgID, projectID, &domain.Task{ID: c.ID, OrgID: orgID, ProjectID: projectID})
		}
	}
	s.broadcastTask(domain.WsTypeTaskDeleted, orgID, projectID, &domain.Task{ID: id, OrgID: orgID, ProjectID: projectID})
	return nil
}

func (s *TaskService) Move(ctx context.Context, actorID, orgID, id, projectID, statusID, positionKey string) error {
	status, err := s.statusRepo.GetByID(ctx, statusID)
	if err != nil {
		return fmt.Errorf("get status: %w", err)
	}
	if status.ProjectID != projectID {
		return apperr.InvalidInput("status does not belong to project")
	}
	if !lexorank.IsValidKey(positionKey) {
		return apperr.InvalidInput("invalid position key")
	}

	task, err := s.taskRepo.GetByID(ctx, orgID, id, projectID)
	if err != nil {
		return fmt.Errorf("get task: %w", err)
	}

	oldStatus, err := s.statusRepo.GetByID(ctx, task.StatusID)
	if err != nil {
		return fmt.Errorf("get old status: %w", err)
	}

	now := time.Now()
	task.StatusID = statusID
	task.PositionKey = positionKey
	task.UpdatedAt = now
	task.CompletedAt = updatedCompletedAt(task.CompletedAt, oldStatus.Category, status.Category, now)
	if err := s.taskRepo.Update(ctx, task); err != nil {
		return err
	}

	s.broadcastTask(domain.WsTypeTaskMoved, orgID, projectID, task)
	s.recordActivity(ctx, task, actorID, domain.ActivityMoved, "status", oldStatus.Name, status.Name)
	return nil
}

func (s *TaskService) Reorder(ctx context.Context, orgID, projectID string, ops []domain.ReorderOp) error {
	if len(ops) > maxBatchSize {
		return apperr.InvalidInput("too many operations: max 500")
	}

	for _, op := range ops {
		if !lexorank.IsValidKey(op.PositionKey) {
			return apperr.InvalidInput(fmt.Sprintf("operation %s: invalid position key", op.TaskID))
		}
	}

	// Apply every reorder op atomically so a mid-batch failure can't leave
	// tasks with partially applied ordering.
	moved := make([]*domain.Task, 0, len(ops))
	err := s.taskRepo.RunInTransaction(ctx, func(repo port.TaskRepository) error {
		moved = moved[:0]
		for _, op := range ops {
			task, err := repo.GetByID(ctx, orgID, op.TaskID, projectID)
			if err != nil {
				return fmt.Errorf("operation %s: get task: %w", op.TaskID, err)
			}
			task.PositionKey = op.PositionKey
			task.UpdatedAt = time.Now()
			if err := repo.Update(ctx, task); err != nil {
				return fmt.Errorf("operation %s: %w", op.TaskID, err)
			}
			moved = append(moved, task)
		}
		return nil
	})
	if err != nil {
		return err
	}
	for _, task := range moved {
		s.broadcastTask(domain.WsTypeTaskMoved, orgID, projectID, task)
	}
	return nil
}

// hydrateTasks resolves <@type:id> tokens in each task's Description into a
// Mentions payload via the shared mentionHydrator.
func (s *TaskService) hydrateTasks(ctx context.Context, orgID string, tasks []*domain.Task) {
	if len(tasks) == 0 {
		return
	}
	contents := make([]string, len(tasks))
	for i, t := range tasks {
		contents[i] = t.Description
	}
	resolved := s.mentions.hydrateMany(ctx, orgID, contents)
	for i, t := range tasks {
		t.Mentions = resolved[i]
	}
}

// BatchUpdate applies a partial update to many tasks in one project. Only the
// non-nil fields in params are applied. When StatusID changes, each affected
// task gets a fresh position key at the end of the target status so ordering
// stays consistent. Assignee changes respect AssigneeMode (replace/add/remove).
func (s *TaskService) BatchUpdate(ctx context.Context, orgID string, p domain.BatchUpdateParams, actorID string) ([]*domain.Task, error) {
	if len(p.TaskIDs) == 0 {
		return nil, apperr.InvalidInput("task_ids is required")
	}
	if len(p.TaskIDs) > maxBatchSize {
		return nil, apperr.InvalidInput("too many tasks: max 500")
	}
	if p.Priority != nil {
		if !validPriorities[*p.Priority] {
			return nil, apperr.InvalidInput("invalid priority")
		}
	}
	var targetStatus *domain.TaskStatus
	if p.StatusID != nil {
		var err error
		targetStatus, err = s.statusRepo.GetByID(ctx, *p.StatusID)
		if err != nil {
			return nil, apperr.InvalidInput("invalid status")
		}
		if targetStatus.ProjectID != p.ProjectID {
			return nil, apperr.InvalidInput("status does not belong to project")
		}
	}
	if p.CycleID != nil && *p.CycleID != "" {
		cycle, err := s.cycleRepo.GetByID(ctx, *p.CycleID, p.ProjectID)
		if err != nil {
			return nil, fmt.Errorf("get cycle: %w", err)
		}
		if cycle.ProjectID != p.ProjectID {
			return nil, apperr.InvalidInput("cycle does not belong to project")
		}
	}

	mode := p.AssigneeMode
	if mode == "" {
		mode = domain.AssigneeModeReplace
	}

	// Fetch all project statuses once so we can resolve each task's current
	// status category without N+1 queries, then sync completed_at.
	statuses, err := s.statusRepo.ListByProject(ctx, p.ProjectID)
	if err != nil {
		return nil, fmt.Errorf("list statuses: %w", err)
	}
	statusCategoryByID := make(map[string]string, len(statuses))
	statusNameByID := make(map[string]string, len(statuses))
	for _, st := range statuses {
		statusCategoryByID[st.ID] = st.Category
		statusNameByID[st.ID] = st.Name
	}
	if targetStatus != nil {
		statusCategoryByID[targetStatus.ID] = targetStatus.Category
		statusNameByID[targetStatus.ID] = targetStatus.Name
	}

	// Fetch all tasks up front and verify they belong to the caller's org +
	// the target project. ListByIDs is org-scoped, so foreign-org IDs simply
	// won't match; we then check project membership explicitly.
	// ListByIDsFull returns complete task rows (priority/status/etc.) so the
	// per-task Update below writes valid data; the minimal ListByIDs (for
	// mention hydration) would write empty priority and violate the CHECK.
	tasks, err := s.taskRepo.ListByIDsFull(ctx, orgID, p.TaskIDs)
	if err != nil {
		return nil, fmt.Errorf("list by ids: %w", err)
	}
	byID := make(map[string]*domain.Task, len(tasks))
	for _, t := range tasks {
		if t.ProjectID != p.ProjectID {
			return nil, apperr.InvalidInput("one or more tasks do not belong to this project")
		}
		byID[t.ID] = t
	}
	if len(byID) != len(p.TaskIDs) {
		// Some IDs didn't resolve (missing, foreign org, or duplicate). Fail
		// closed rather than silently skipping.
		return nil, apperr.InvalidInput("one or more tasks not found")
	}

	// Snapshot old per-task state before the mutation for activity recording.
	type batchSnapshot struct {
		StatusID  string
		Priority  string
		CycleID   *string
		Assignees map[string]domain.TaskAssignee
	}
	oldSnapshots := make(map[string]batchSnapshot, len(tasks))
	for _, t := range tasks {
		aMap := make(map[string]domain.TaskAssignee, len(t.Assignees))
		for _, a := range t.Assignees {
			aMap[a.ID] = a
		}
		oldSnapshots[t.ID] = batchSnapshot{
			StatusID:  t.StatusID,
			Priority:  t.Priority,
			CycleID:   t.CycleID,
			Assignees: aMap,
		}
	}

	now := time.Now()
	// Run all mutations inside a single transaction so a mid-loop failure
	// rolls back earlier updates, keeping BatchUpdate atomic.
	err = s.taskRepo.RunInTransaction(ctx, func(repo port.TaskRepository) error {
		for _, t := range tasks {
			oldCategory := statusCategoryByID[t.StatusID]
			if p.StatusID != nil && t.StatusID != *p.StatusID {
				pk, err := repo.GeneratePositionKey(ctx, orgID, p.ProjectID, *p.StatusID)
				if err != nil {
					return fmt.Errorf("generate position key: %w", err)
				}
				t.StatusID = *p.StatusID
				t.PositionKey = pk
			}
			if p.Priority != nil {
				t.Priority = *p.Priority
			}
			if p.CycleID != nil {
				t.CycleID = p.CycleID
			}
			t.UpdatedAt = now
			newCategory := statusCategoryByID[t.StatusID]
			t.CompletedAt = updatedCompletedAt(t.CompletedAt, oldCategory, newCategory, now)
			if err := repo.Update(ctx, t); err != nil {
				return fmt.Errorf("update task %s: %w", t.ID, err)
			}

			if len(p.AssigneeIDs) > 0 || mode == domain.AssigneeModeReplace {
				var newIDs []string
				switch mode {
				case domain.AssigneeModeAdd:
					existing := make(map[string]bool)
					for _, a := range t.Assignees {
						existing[a.ID] = true
						newIDs = append(newIDs, a.ID)
					}
					for _, id := range p.AssigneeIDs {
						if !existing[id] {
							newIDs = append(newIDs, id)
						}
					}
				case domain.AssigneeModeRemove:
					rm := make(map[string]bool)
					for _, id := range p.AssigneeIDs {
						rm[id] = true
					}
					for _, a := range t.Assignees {
						if !rm[a.ID] {
							newIDs = append(newIDs, a.ID)
						}
					}
				default: // AssigneeModeReplace
					newIDs = p.AssigneeIDs
				}
				if err := repo.SetAssignees(ctx, t.ID, newIDs); err != nil {
					return fmt.Errorf("set assignees %s: %w", t.ID, err)
				}
			}
		}
		return nil
	})
	if err != nil {
		return nil, err
	}

	// Return the updated tasks so the caller can render them without a refetch.
	// ListByIDsFull returns complete rows so the response DTO has status_id,
	// priority, etc. (the minimal ListByIDs would render empty fields).
	updated, err := s.taskRepo.ListByIDsFull(ctx, orgID, p.TaskIDs)
	if err != nil {
		return nil, err
	}

	// Record activity per task by comparing the snapshot with the final state.
	for _, t := range updated {
		oldSnap := oldSnapshots[t.ID]

		// Status change.
		if t.StatusID != oldSnap.StatusID {
			oldName := statusNameByID[oldSnap.StatusID]
			newName := statusNameByID[t.StatusID]
			s.recordActivity(ctx, t, actorID, domain.ActivityStatusChanged, "status", oldName, newName)
		}

		// Priority change.
		if t.Priority != oldSnap.Priority {
			s.recordActivity(ctx, t, actorID, domain.ActivityPriorityChanged, "priority", priorityDisplayName(oldSnap.Priority), priorityDisplayName(t.Priority))
		}

		// Cycle change.
		oldCycle := ptrOrEmpty(oldSnap.CycleID)
		newCycle := ptrOrEmpty(t.CycleID)
		if oldCycle != newCycle {
			oldCycleName := s.cycleNameOrID(ctx, oldCycle, p.ProjectID)
			newCycleName := s.cycleNameOrID(ctx, newCycle, p.ProjectID)
			s.recordActivity(ctx, t, actorID, domain.ActivityCycleChanged, "cycle", oldCycleName, newCycleName)
		}

		// Assignee changes: compute old set from snapshot and new set from current task.
		newAssignees := make(map[string]bool)
		for _, a := range t.Assignees {
			newAssignees[a.ID] = true
		}
		for _, a := range t.Assignees {
			if _, exists := oldSnap.Assignees[a.ID]; !exists {
				s.recordActivity(ctx, t, actorID, domain.ActivityAssigned, "assignee", "", a.Name)
			}
		}
		for _, a := range oldSnap.Assignees {
			if !newAssignees[a.ID] {
				s.recordActivity(ctx, t, actorID, domain.ActivityUnassigned, "assignee", a.Name, "")
			}
		}

		s.broadcastTask(domain.WsTypeTaskUpdated, orgID, p.ProjectID, t)
	}
	return updated, nil
}

// Duplicate clones a task into the same project + status with a fresh ID and
// a new position key. Assignees are copied; the copy's title gains a
// " (copy)" suffix so it's distinguishable in lists. When includeSubtasks is
// true, the task's direct children are also duplicated and parented to the
// new task (1-level nesting only).
func (s *TaskService) Duplicate(ctx context.Context, orgID, taskID, projectID string, includeSubtasks bool, actorID string) (*domain.Task, error) {
	src, err := s.taskRepo.GetByID(ctx, orgID, taskID, projectID)
	if err != nil {
		return nil, fmt.Errorf("get source task: %w", err)
	}

	assigneeIDs := make([]string, len(src.Assignees))
	for i, a := range src.Assignees {
		assigneeIDs[i] = a.ID
	}

	copy, err := s.Create(ctx, domain.CreateTaskParams{
		OrgID:       src.OrgID,
		ProjectID:   src.ProjectID,
		CreatedBy:   src.CreatedBy,
		Title:       src.Title + " (copy)",
		Description: src.Description,
		StatusID:    src.StatusID,
		Priority:    src.Priority,
		AssigneeIDs: assigneeIDs,
		CycleID:     src.CycleID,
		// ParentID is deliberately omitted: a duplicate is a top-level task, not
		// a child of the source's parent.
		Estimate:  src.Estimate,
		StartedAt: src.StartedAt,
		DueAt:     src.DueAt,
	})
	if err != nil {
		return nil, err
	}

	s.recordActivity(ctx, src, actorID, domain.ActivityDuplicated, "", "", copy.Title)

	var copiedSubtasks []*domain.Task
	if includeSubtasks {
		subtasks, err := s.taskRepo.ListSubtasks(ctx, orgID, taskID)
		if err != nil {
			return nil, fmt.Errorf("list subtasks for duplicate: %w", err)
		}
		// All subtask copies commit atomically: a failure halfway must not
		// leave a half-populated duplicate behind. Events fire after commit.
		err = s.taskRepo.RunInTransaction(ctx, func(repo port.TaskRepository) error {
			copiedSubtasks = copiedSubtasks[:0]
			for _, st := range subtasks {
				stAssigneeIDs := make([]string, len(st.Assignees))
				for i, a := range st.Assignees {
					stAssigneeIDs[i] = a.ID
				}
				created, err := s.createTaskRow(ctx, repo, domain.CreateTaskParams{
					OrgID:       st.OrgID,
					ProjectID:   st.ProjectID,
					CreatedBy:   st.CreatedBy,
					Title:       st.Title,
					Description: st.Description,
					StatusID:    st.StatusID,
					Priority:    st.Priority,
					AssigneeIDs: stAssigneeIDs,
					CycleID:     st.CycleID,
					ParentID:    &copy.ID,
					Estimate:    st.Estimate,
					StartedAt:   st.StartedAt,
					DueAt:       st.DueAt,
				})
				if err != nil {
					return fmt.Errorf("duplicate subtask %s: %w", st.ID, err)
				}
				copiedSubtasks = append(copiedSubtasks, created)
			}
			return nil
		})
		if err != nil {
			return nil, err
		}
		for _, created := range copiedSubtasks {
			s.broadcastTask(domain.WsTypeTaskCreated, orgID, created.ProjectID, created)
		}
	}

	return copy, nil
}

// MoveToProject re-homes a task into a different project. The target status
// must belong to the target project, and both projects must be in the caller's
// org. The task's cycle + parent are cleared (both are project-scoped). A
// fresh position key is generated for the target status.
func (s *TaskService) MoveToProject(ctx context.Context, orgID, taskID, fromProjectID, toProjectID, toStatusID string, actorID string) (*domain.Task, error) {
	if toProjectID == fromProjectID {
		return nil, apperr.InvalidInput("task is already in this project")
	}
	// Validate the source exists in the caller's org + source project.
	srcTask, err := s.taskRepo.GetByID(ctx, orgID, taskID, fromProjectID)
	if err != nil {
		return nil, fmt.Errorf("get source task: %w", err)
	}
	// Validate the target project is in the caller's org.
	toProject, err := s.projRepo.GetByID(ctx, orgID, toProjectID)
	if err != nil {
		return nil, fmt.Errorf("get target project: %w", err)
	}
	if toProject.OrgID != orgID {
		return nil, apperr.InvalidInput("target project does not belong to your organization")
	}
	// Resolve the target status: if none provided, pick the target project's
	// default (or first) status so the caller doesn't need to pre-load the
	// target project's statuses.
	if toStatusID == "" {
		statuses, err := s.statusRepo.ListByProject(ctx, toProjectID)
		if err != nil {
			return nil, fmt.Errorf("list target statuses: %w", err)
		}
		for _, st := range statuses {
			if st.Default {
				toStatusID = st.ID
				break
			}
		}
		if toStatusID == "" && len(statuses) > 0 {
			toStatusID = statuses[0].ID
		}
		if toStatusID == "" {
			return nil, apperr.InvalidInput("target project has no statuses")
		}
	} else {
		// Validate the provided status belongs to the target project.
		status, err := s.statusRepo.GetByID(ctx, toStatusID)
		if err != nil {
			return nil, fmt.Errorf("get target status: %w", err)
		}
		if status.ProjectID != toProjectID {
			return nil, apperr.InvalidInput("status does not belong to target project")
		}
	}

	// Load the source/target status categories so we can sync completed_at
	// to the target status category.
	srcStatus, err := s.statusRepo.GetByID(ctx, srcTask.StatusID)
	if err != nil {
		return nil, fmt.Errorf("get source status: %w", err)
	}
	targetStatus, err := s.statusRepo.GetByID(ctx, toStatusID)
	if err != nil {
		return nil, fmt.Errorf("get target status: %w", err)
	}
	completedAt := updatedCompletedAt(srcTask.CompletedAt, srcStatus.Category, targetStatus.Category, time.Now())

	// Resolve from-project name for activity logging.
	fromProjectName := fromProjectID
	if fromProject, err := s.projRepo.GetByID(ctx, orgID, fromProjectID); err == nil {
		fromProjectName = fromProject.Name
	}

	// The whole move (parent plus every descendant) must be atomic.
	// Subtask moves are two writes each (move re-links parent separately),
	// so a mid-loop failure without a transaction would strand some
	// subtasks in the source project with a dangling parent reference.
	err = s.taskRepo.RunInTransaction(ctx, func(repo port.TaskRepository) error {
		positionKey, err := repo.GeneratePositionKey(ctx, orgID, toProjectID, toStatusID)
		if err != nil {
			return fmt.Errorf("generate position key: %w", err)
		}
		if err := repo.MoveToProject(ctx, orgID, taskID, fromProjectID, toProjectID, toStatusID, positionKey, completedAt); err != nil {
			return fmt.Errorf("move task: %w", err)
		}
		// Recursively move all subtasks to the target project so they don't
		// get orphaned in the source project with a dangling parent_task_id.
		return s.moveSubtasks(ctx, repo, orgID, taskID, fromProjectID, toProjectID, toStatusID, targetStatus.Category)
	})
	if err != nil {
		return nil, err
	}

	moved, err := s.taskRepo.GetByID(ctx, orgID, taskID, toProjectID)
	if err != nil {
		return nil, err
	}
	// The task left the source project room and entered the target project room.
	s.broadcastTask(domain.WsTypeTaskDeleted, orgID, fromProjectID, &domain.Task{ID: taskID, OrgID: orgID, ProjectID: fromProjectID})
	s.broadcastTask(domain.WsTypeTaskCreated, orgID, toProjectID, moved)
	s.recordActivity(ctx, moved, actorID, domain.ActivityMovedToProject, "project", fromProjectName, toProject.Name)
	return moved, nil
}

// ReorderSubtasks re-keys the subtask_position of a task's direct children.
// Each op provides a task ID + a new position key (lexorank). Validates that
// every op references an actual child of parentID and that the keys are valid.
func (s *TaskService) ReorderSubtasks(ctx context.Context, orgID, projectID, parentID string, ops []domain.ReorderOp) error {
	if len(ops) > maxBatchSize {
		return apperr.InvalidInput("too many operations: max 500")
	}
	if _, err := s.taskRepo.GetByID(ctx, orgID, parentID, projectID); err != nil {
		return apperr.NotFound("task", err)
	}
	for _, op := range ops {
		if !lexorank.IsValidKey(op.PositionKey) {
			return apperr.InvalidInput(fmt.Sprintf("operation %s: invalid position key", op.TaskID))
		}
	}
	if err := s.taskRepo.ReorderSubtasks(ctx, orgID, parentID, ops); err != nil {
		return err
	}
	// Broadcast an update for each moved subtask so connected clients re-render.
	for _, op := range ops {
		if t, err := s.taskRepo.GetByIDAndOrg(ctx, orgID, op.TaskID); err == nil {
			s.broadcastTask(domain.WsTypeTaskUpdated, orgID, t.ProjectID, t)
		}
	}
	return nil
}

// moveSubtasks recursively moves all direct children of parentID from
// fromProjectID to toProjectID/toStatusID. Each subtask gets a fresh
// position_key at the end of the target status. The parent_task_id is
// preserved so the hierarchy survives the move. Runs against the supplied
// repository so callers can execute it inside their transaction.
func (s *TaskService) moveSubtasks(ctx context.Context, repo port.TaskRepository, orgID, parentID, fromProjectID, toProjectID, toStatusID, targetCategory string) error {
	subtasks, err := repo.ListSubtasks(ctx, orgID, parentID)
	if err != nil {
		return err
	}
	for _, st := range subtasks {
		lastKey, err := repo.GeneratePositionKey(ctx, orgID, toProjectID, toStatusID)
		if err != nil {
			return err
		}
		positionKey := lastKey

		// Look up the subtask's source status so we can sync completed_at
		// to the target status category (subtasks may be in a different
		// source status than the parent).
		srcSubStatus, err := s.statusRepo.GetByID(ctx, st.StatusID)
		if err != nil {
			return fmt.Errorf("get source status for subtask %s: %w", st.ID, err)
		}
		subCompletedAt := updatedCompletedAt(st.CompletedAt, srcSubStatus.Category, targetCategory, time.Now())

		// Move the subtask, preserving its parent_task_id (the parent was
		// already moved above). We use a dedicated query variant that keeps
		// parent_task_id; but the existing MoveTaskToProject nulls it. So we
		// move then re-link.
		if err := repo.MoveToProject(ctx, orgID, st.ID, fromProjectID, toProjectID, toStatusID, positionKey, subCompletedAt); err != nil {
			return err
		}
		// Re-link the subtask to its parent (MoveTaskToProject nulls parent_task_id).
		// Update st.CompletedAt so the subsequent Update persists the corrected value.
		st.CompletedAt = subCompletedAt
		st.ProjectID = toProjectID
		st.StatusID = toStatusID
		st.ParentID = &parentID
		if err := repo.Update(ctx, st); err != nil {
			return err
		}
		// Recurse into nested subtasks.
		if err := s.moveSubtasks(ctx, repo, orgID, st.ID, fromProjectID, toProjectID, toStatusID, targetCategory); err != nil {
			return err
		}
	}
	return nil
}
