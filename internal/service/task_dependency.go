package service

import (
	"context"
	"fmt"
	"time"

	"ipmanlk/plume/internal/apperr"
	"ipmanlk/plume/internal/domain"
	"ipmanlk/plume/internal/port"

	"github.com/google/uuid"
)

// TaskDependencyService manages task blocking relationships.
//
// Semantics: Add(taskID, blocksTaskID) records that taskID is blocked by
// blocksTaskID (blocksTaskID must complete first). The service prevents
// self-dependencies and direct cycles (adding B→A when A→B already exists).
// Deep cycle detection is intentionally bounded to one hop; longer cycles
// are rare in practice and full graph traversal is out of scope for the
// single-edge add operation.
type TaskDependencyService struct {
	depRepo      port.TaskDependencyRepository
	taskRepo     port.TaskRepository
	access       port.AccessChecker
	activityRepo port.TaskActivityRepository
	broadcaster  port.Broadcaster
}

func NewTaskDependencyService(depRepo port.TaskDependencyRepository, taskRepo port.TaskRepository, access port.AccessChecker, activityRepo port.TaskActivityRepository, broadcaster port.Broadcaster) *TaskDependencyService {
	return &TaskDependencyService{depRepo: depRepo, taskRepo: taskRepo, access: access, activityRepo: activityRepo, broadcaster: broadcaster}
}

var _ port.TaskDependencyService = (*TaskDependencyService)(nil)

// Add records that taskID is blocked by blocksTaskID. Both tasks must exist
// in the caller's org; a direct reverse edge (blocksTaskID already blocked
// by taskID) is rejected to prevent trivial cycles.
func (s *TaskDependencyService) Add(ctx context.Context, userID, orgID, taskID, blocksTaskID string) error {
	if s.access != nil {
		if err := s.access.RequireTaskAccess(ctx, userID, orgID, taskID, domain.PermTaskEdit); err != nil {
			return err
		}
	}
	if taskID == blocksTaskID {
		return apperr.InvalidInput("a task cannot block itself")
	}
	// Verify both tasks exist in the caller's org.
	if _, err := s.taskRepo.GetByIDAndOrg(ctx, orgID, taskID); err != nil {
		return apperr.NotFound("task", err)
	}
	blockingTask, err := s.taskRepo.GetByIDAndOrg(ctx, orgID, blocksTaskID)
	if err != nil {
		return apperr.NotFound("blocking task", err)
	}
	// Reject a direct cycle: if blocksTaskID is already blocked by taskID,
	// adding the reverse would create an A↔B deadlock.
	blocking, err := s.depRepo.ListBlocking(ctx, blocksTaskID)
	if err != nil {
		return fmt.Errorf("check cycle: %w", err)
	}
	for _, b := range blocking {
		if b.ID == taskID {
			return apperr.InvalidInput("this would create a circular dependency")
		}
	}
	if err := s.depRepo.Add(ctx, taskID, blocksTaskID); err != nil {
		return err
	}

	// Record activity best-effort.
	if s.activityRepo != nil {
		task, err := s.taskRepo.GetByIDAndOrg(ctx, orgID, taskID)
		if err == nil {
			_ = s.activityRepo.Create(ctx, &domain.TaskActivity{
				ID:        uuid.New().String(),
				TaskID:    taskID,
				OrgID:     orgID,
				ProjectID: task.ProjectID,
				ActorID:   userID,
				Action:    domain.ActivityDependencyAdded,
				Field:     "dependency",
				OldValue:  "",
				NewValue:  blockingTask.Title,
				CreatedAt: time.Now(),
			})
			s.broadcastTaskActivity(ctx, orgID, task.ProjectID, taskID)
		}
	}

	return nil
}

// Remove drops a blocking edge.
func (s *TaskDependencyService) Remove(ctx context.Context, userID, orgID, taskID, blocksTaskID string) error {
	if s.access != nil {
		if err := s.access.RequireTaskAccess(ctx, userID, orgID, taskID, domain.PermTaskEdit); err != nil {
			return err
		}
	}
	if _, err := s.taskRepo.GetByIDAndOrg(ctx, orgID, taskID); err != nil {
		return apperr.NotFound("task", err)
	}

	// Fetch the blocking task title before removing the edge.
	blockingTitle := ""
	if blockTask, err := s.taskRepo.GetByIDAndOrg(ctx, orgID, blocksTaskID); err == nil {
		blockingTitle = blockTask.Title
	}

	if err := s.depRepo.Remove(ctx, taskID, blocksTaskID); err != nil {
		return err
	}

	// Record activity best-effort.
	if s.activityRepo != nil && blockingTitle != "" {
		task, err := s.taskRepo.GetByIDAndOrg(ctx, orgID, taskID)
		if err == nil {
			_ = s.activityRepo.Create(ctx, &domain.TaskActivity{
				ID:        uuid.New().String(),
				TaskID:    taskID,
				OrgID:     orgID,
				ProjectID: task.ProjectID,
				ActorID:   userID,
				Action:    domain.ActivityDependencyRemoved,
				Field:     "dependency",
				OldValue:  blockingTitle,
				NewValue:  "",
				CreatedAt: time.Now(),
			})
			s.broadcastTaskActivity(ctx, orgID, task.ProjectID, taskID)
		}
	}

	return nil
}

func (s *TaskDependencyService) broadcastTaskActivity(ctx context.Context, orgID, projectID, taskID string) {
	if s.broadcaster == nil {
		return
	}
	_ = s.broadcaster.Broadcast(
		domain.RoomKeyProject(orgID, projectID),
		string(domain.WsTypeTaskActivityRecorded),
		map[string]any{"task_id": taskID},
	)
}

// ListBlocking returns the tasks blocking the given task. Viewers and guests
// must hold a project membership granting task:view on the task's project.
func (s *TaskDependencyService) ListBlocking(ctx context.Context, userID, orgID, taskID string) ([]*domain.Task, error) {
	if _, err := s.taskRepo.GetByIDAndOrg(ctx, orgID, taskID); err != nil {
		return nil, apperr.NotFound("task", err)
	}
	if s.access != nil {
		if err := s.access.RequireTaskAccess(ctx, userID, orgID, taskID, domain.PermTaskView); err != nil {
			return nil, err
		}
	}
	return s.depRepo.ListBlocking(ctx, taskID)
}

// ListBlocked returns the tasks that the given task is blocking. Viewers and
// guests must hold a project membership granting task:view.
func (s *TaskDependencyService) ListBlocked(ctx context.Context, userID, orgID, taskID string) ([]*domain.Task, error) {
	if _, err := s.taskRepo.GetByIDAndOrg(ctx, orgID, taskID); err != nil {
		return nil, apperr.NotFound("task", err)
	}
	if s.access != nil {
		if err := s.access.RequireTaskAccess(ctx, userID, orgID, taskID, domain.PermTaskView); err != nil {
			return nil, err
		}
	}
	return s.depRepo.ListBlocked(ctx, taskID)
}
