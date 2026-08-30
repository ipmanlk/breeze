package service

import (
	"context"
	"fmt"
	"strings"
	"time"

	"ipmanlk/breeze/internal/apperr"
	"ipmanlk/breeze/internal/domain"
	"ipmanlk/breeze/internal/port"

	"github.com/google/uuid"
)

type TimeEntryService struct {
	repo         port.TimeEntryRepository
	taskRepo     port.TaskRepository
	access       port.AccessChecker
	activityRepo port.TaskActivityRepository
	broadcaster  port.Broadcaster
}

var _ port.TimeEntryService = (*TimeEntryService)(nil)

func NewTimeEntryService(repo port.TimeEntryRepository, taskRepo port.TaskRepository, access port.AccessChecker, activityRepo port.TaskActivityRepository, broadcaster port.Broadcaster) *TimeEntryService {
	return &TimeEntryService{repo: repo, taskRepo: taskRepo, access: access, activityRepo: activityRepo, broadcaster: broadcaster}
}

func (s *TimeEntryService) List(ctx context.Context, orgID, taskID, projectID string) ([]*domain.TimeEntry, error) {
	_, err := s.taskRepo.GetByID(ctx, orgID, taskID, projectID)
	if err != nil {
		return nil, fmt.Errorf("task not found: %w", err)
	}
	return s.repo.ListByTask(ctx, taskID)
}

func (s *TimeEntryService) Start(ctx context.Context, orgID, taskID, projectID, userID, description string) ([]*domain.TimeEntry, error) {
	if s.access != nil {
		if err := s.access.RequireProjectAccess(ctx, userID, orgID, projectID, domain.PermTimeCreate); err != nil {
			return nil, err
		}
	}
	_, err := s.taskRepo.GetByID(ctx, orgID, taskID, projectID)
	if err != nil {
		return nil, fmt.Errorf("task not found: %w", err)
	}

	id := uuid.New().String()
	if err := s.repo.StartTimerAtomic(ctx, id, taskID, userID, description); err != nil {
		// The partial unique index (idx_time_entries_active_user) prevents a
		// second concurrent start. Map the raw SQL constraint error to a
		// user-friendly conflict so the API returns 409, not 500.
		if strings.Contains(strings.ToLower(err.Error()), "unique constraint failed") {
			return nil, apperr.Conflict("a timer is already running")
		}
		return nil, err
	}

	return s.repo.ListByTask(ctx, taskID)
}

func (s *TimeEntryService) Stop(ctx context.Context, orgID, taskID, projectID, userID string) ([]*domain.TimeEntry, error) {
	if s.access != nil {
		if err := s.access.RequireProjectAccess(ctx, userID, orgID, projectID, domain.PermTimeCreate); err != nil {
			return nil, err
		}
	}
	_, err := s.taskRepo.GetByID(ctx, orgID, taskID, projectID)
	if err != nil {
		return nil, fmt.Errorf("task not found: %w", err)
	}

	active, err := s.repo.GetActiveTimer(ctx, taskID, userID)
	if err != nil {
		return nil, fmt.Errorf("no active timer: %w", err)
	}

	if err := s.repo.StopTimer(ctx, active.ID, userID); err != nil {
		return nil, err
	}

	return s.repo.ListByTask(ctx, taskID)
}

func (s *TimeEntryService) Create(ctx context.Context, p domain.CreateTimeEntryParams) ([]*domain.TimeEntry, error) {
	if p.DurationMinutes <= 0 {
		return nil, apperr.InvalidInput("duration must be positive")
	}
	if s.access != nil {
		if err := s.access.RequireProjectAccess(ctx, p.UserID, p.OrgID, p.ProjectID, domain.PermTimeCreate); err != nil {
			return nil, err
		}
	}
	_, err := s.taskRepo.GetByID(ctx, p.OrgID, p.TaskID, p.ProjectID)
	if err != nil {
		return nil, fmt.Errorf("task not found: %w", err)
	}

	now := time.Now()
	end := now.Add(time.Duration(p.DurationMinutes) * time.Minute)
	entry := &domain.TimeEntry{
		ID:              uuid.New().String(),
		TaskID:          p.TaskID,
		UserID:          p.UserID,
		Description:     p.Description,
		StartedAt:       now,
		EndedAt:         &end,
		DurationMinutes: &p.DurationMinutes,
	}

	if err := s.repo.Create(ctx, entry); err != nil {
		return nil, err
	}

	// Record activity best-effort for manual time entry.
	if s.activityRepo != nil {
		durationStr := fmt.Sprintf("%dh %dm", p.DurationMinutes/60, p.DurationMinutes%60)
		if p.DurationMinutes < 60 {
			durationStr = fmt.Sprintf("%dm", p.DurationMinutes)
		}
		_ = s.activityRepo.Create(ctx, &domain.TaskActivity{
			ID:        uuid.New().String(),
			TaskID:    p.TaskID,
			OrgID:     p.OrgID,
			ProjectID: p.ProjectID,
			ActorID:   p.UserID,
			Action:    domain.ActivityTimeLogged,
			Field:     "time",
			OldValue:  "",
			NewValue:  durationStr,
			CreatedAt: time.Now(),
		})
		s.broadcastTaskActivity(ctx, p.OrgID, p.ProjectID, p.TaskID)
	}

	return s.repo.ListByTask(ctx, p.TaskID)
}

func (s *TimeEntryService) broadcastTaskActivity(ctx context.Context, orgID, projectID, taskID string) {
	if s.broadcaster == nil {
		return
	}
	_ = s.broadcaster.Broadcast(
		domain.RoomKeyProject(orgID, projectID),
		string(domain.WsTypeTaskActivityRecorded),
		map[string]any{"task_id": taskID},
	)
}

func (s *TimeEntryService) Update(ctx context.Context, callerUserID string, callerRole domain.Role, p domain.UpdateTimeEntryParams) ([]*domain.TimeEntry, error) {
	if s.access != nil {
		if err := s.access.RequireProjectAccess(ctx, callerUserID, p.OrgID, p.ProjectID, domain.PermTimeCreate); err != nil {
			return nil, err
		}
	}
	_, err := s.taskRepo.GetByID(ctx, p.OrgID, p.TaskID, p.ProjectID)
	if err != nil {
		return nil, fmt.Errorf("task not found: %w", err)
	}

	entries, err := s.repo.ListByTask(ctx, p.TaskID)
	if err != nil {
		return nil, err
	}

	var entry *domain.TimeEntry
	for _, e := range entries {
		if e.ID == p.ID {
			entry = e
			break
		}
	}
	if entry == nil {
		return nil, apperr.ErrNotFound
	}
	if !canManageTimeEntry(entry, callerUserID, callerRole) {
		return nil, apperr.ErrForbidden
	}

	if p.Description != nil {
		entry.Description = *p.Description
	}
	if p.DurationMinutes != nil {
		entry.DurationMinutes = p.DurationMinutes
	}

	if err := s.repo.Update(ctx, entry); err != nil {
		return nil, err
	}

	return s.repo.ListByTask(ctx, p.TaskID)
}

func (s *TimeEntryService) Delete(ctx context.Context, callerUserID string, callerRole domain.Role, orgID, id, taskID, projectID string) error {
	if s.access != nil {
		if err := s.access.RequireProjectAccess(ctx, callerUserID, orgID, projectID, domain.PermTimeDelete); err != nil {
			return err
		}
	}
	_, err := s.taskRepo.GetByID(ctx, orgID, taskID, projectID)
	if err != nil {
		return fmt.Errorf("task not found: %w", err)
	}

	entries, err := s.repo.ListByTask(ctx, taskID)
	if err != nil {
		return err
	}
	for _, e := range entries {
		if e.ID == id {
			if !canManageTimeEntry(e, callerUserID, callerRole) {
				return apperr.ErrForbidden
			}
			return s.repo.Delete(ctx, id, taskID)
		}
	}
	return apperr.ErrNotFound
}

// canManageTimeEntry returns true when the caller owns the entry or holds an
// elevated org role (owner/admin) that can manage others' time entries.
func canManageTimeEntry(entry *domain.TimeEntry, callerUserID string, callerRole domain.Role) bool {
	if entry.UserID == callerUserID {
		return true
	}
	return callerRole == domain.RoleOwner || callerRole == domain.RoleAdmin
}
