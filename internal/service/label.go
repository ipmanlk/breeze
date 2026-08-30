package service

import (
	"context"
	"fmt"
	"regexp"
	"strings"

	"ipmanlk/breeze/internal/apperr"
	"ipmanlk/breeze/internal/domain"
	"ipmanlk/breeze/internal/port"

	"github.com/google/uuid"
)

var labelColorRegex = regexp.MustCompile(`^#[0-9a-fA-F]{6}$`)

type LabelService struct {
	labelRepo    port.LabelRepository
	access       port.AccessChecker
	activityRepo port.TaskActivityRepository
	taskRepo     port.TaskRepository
	broadcaster  port.Broadcaster
}

var _ port.LabelService = (*LabelService)(nil)

func NewLabelService(labelRepo port.LabelRepository, access port.AccessChecker, activityRepo port.TaskActivityRepository, taskRepo port.TaskRepository, broadcaster port.Broadcaster) *LabelService {
	return &LabelService{labelRepo: labelRepo, access: access, activityRepo: activityRepo, taskRepo: taskRepo, broadcaster: broadcaster}
}

func (s *LabelService) List(ctx context.Context, orgID string) ([]*domain.Label, error) {
	return s.labelRepo.ListByOrg(ctx, orgID)
}

func (s *LabelService) Create(ctx context.Context, userID, orgID, name, color string) (*domain.Label, error) {
	name = strings.TrimSpace(name)
	if len(name) < 1 || len(name) > 32 {
		return nil, apperr.InvalidInput("label name must be between 1 and 32 characters")
	}
	if !labelColorRegex.MatchString(color) {
		color = "#6366f1"
	}
	if s.access != nil {
		if err := s.access.RequireOrgAccess(ctx, userID, orgID, domain.PermTaskCreate); err != nil {
			return nil, err
		}
	}

	label := &domain.Label{
		ID:    uuid.New().String(),
		OrgID: orgID,
		Name:  name,
		Color: color,
	}
	if err := s.labelRepo.Create(ctx, label); err != nil {
		return nil, fmt.Errorf("create label: %w", err)
	}
	return s.labelRepo.GetByID(ctx, orgID, label.ID)
}

func (s *LabelService) Update(ctx context.Context, userID, orgID, id, name, color string) (*domain.Label, error) {
	name = strings.TrimSpace(name)
	if len(name) < 1 || len(name) > 32 {
		return nil, apperr.InvalidInput("label name must be between 1 and 32 characters")
	}
	if !labelColorRegex.MatchString(color) {
		color = "#6366f1"
	}
	if s.access != nil {
		if err := s.access.RequireOrgAccess(ctx, userID, orgID, domain.PermTaskCreate); err != nil {
			return nil, err
		}
	}
	label, err := s.labelRepo.GetByID(ctx, orgID, id)
	if err != nil {
		return nil, err
	}
	label.Name = name
	label.Color = color
	if err := s.labelRepo.Update(ctx, label); err != nil {
		return nil, err
	}
	return s.labelRepo.GetByID(ctx, orgID, id)
}

func (s *LabelService) Delete(ctx context.Context, userID, orgID, id string) error {
	if s.access != nil {
		if err := s.access.RequireOrgAccess(ctx, userID, orgID, domain.PermTaskCreate); err != nil {
			return err
		}
	}
	_, err := s.labelRepo.GetByID(ctx, orgID, id)
	if err != nil {
		return err
	}
	return s.labelRepo.Delete(ctx, orgID, id)
}

func (s *LabelService) SetTaskLabels(ctx context.Context, userID, orgID, taskID string, labelIDs []string) error {
	if s.access != nil {
		if err := s.access.RequireTaskAccess(ctx, userID, orgID, taskID, domain.PermTaskEdit); err != nil {
			return err
		}
	}
	// Validate every label belongs to the caller's org before attaching;
	// prevents cross-org label leakage via the join table.
	for _, lid := range labelIDs {
		if _, err := s.labelRepo.GetByID(ctx, orgID, lid); err != nil {
			return apperr.NotFound("label", nil)
		}
	}
	// Capture the current label set before replacing so we can record a
	// labels_changed activity entry (best-effort: never blocks the update).
	var oldLabels []*domain.Label
	if s.activityRepo != nil {
		if cur, err := s.labelRepo.GetTaskLabels(ctx, taskID); err == nil {
			oldLabels = cur
		}
	}
	if err := s.labelRepo.SetTaskLabels(ctx, taskID, labelIDs); err != nil {
		return err
	}
	if s.activityRepo != nil && s.taskRepo != nil {
		s.recordLabelsActivity(ctx, userID, orgID, taskID, oldLabels, labelIDs)
	}
	return nil
}

// recordLabelsActivity persists a labels_changed entry comparing the old and
// new label sets. Best-effort: errors are ignored (same pattern as
// TaskService.recordActivity). Uses comma-joined label names for readability.
func (s *LabelService) recordLabelsActivity(ctx context.Context, actorID, orgID, taskID string, oldLabels []*domain.Label, newLabelIDs []string) {
	oldNames := make([]string, 0, len(oldLabels))
	oldIDs := make(map[string]bool, len(oldLabels))
	for _, l := range oldLabels {
		oldNames = append(oldNames, l.Name)
		oldIDs[l.ID] = true
	}
	newNames := make([]string, 0, len(newLabelIDs))
	for _, id := range newLabelIDs {
		if l, err := s.labelRepo.GetByID(ctx, orgID, id); err == nil {
			newNames = append(newNames, l.Name)
		}
	}
	// Only record when the set actually changed.
	if strings.Join(oldNames, ", ") == strings.Join(newNames, ", ") {
		return
	}
	task, err := s.taskRepo.GetByIDAndOrg(ctx, orgID, taskID)
	if err != nil {
		return
	}
	_ = s.activityRepo.Create(ctx, &domain.TaskActivity{
		ID:        uuid.New().String(),
		TaskID:    taskID,
		OrgID:     orgID,
		ProjectID: task.ProjectID,
		ActorID:   actorID,
		Action:    domain.ActivityLabelsChanged,
		Field:     "labels",
		OldValue:  strings.Join(oldNames, ", "),
		NewValue:  strings.Join(newNames, ", "),
	})
	s.broadcastActivity(ctx, orgID, task.ProjectID, taskID)
}

// broadcastActivity broadcasts a task_activity_recorded WS event to the
// project room so open dialogs refresh their activity feed. Best-effort.
func (s *LabelService) broadcastActivity(ctx context.Context, orgID, projectID, taskID string) {
	if s.broadcaster == nil {
		return
	}
	_ = s.broadcaster.Broadcast(
		domain.RoomKeyProject(orgID, projectID),
		string(domain.WsTypeTaskActivityRecorded),
		map[string]any{"task_id": taskID},
	)
}

func (s *LabelService) GetTaskLabels(ctx context.Context, userID, orgID, taskID string) ([]*domain.Label, error) {
	// Read path must enforce the same task-level access as the write path:
	// resolve the caller's project role before returning any labels.
	if s.access != nil {
		if err := s.access.RequireTaskAccess(ctx, userID, orgID, taskID, domain.PermTaskView); err != nil {
			return nil, err
		}
	}
	return s.labelRepo.GetTaskLabels(ctx, taskID)
}

func (s *LabelService) ListLabelsByTaskIDs(ctx context.Context, taskIDs []string) (map[string][]*domain.Label, error) {
	return s.labelRepo.ListLabelsByTaskIDs(ctx, taskIDs)
}
