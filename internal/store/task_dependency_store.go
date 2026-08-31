package store

import (
	"context"

	"ipmanlk/plume/internal/domain"
	"ipmanlk/plume/internal/port"
	"ipmanlk/plume/internal/store/sqlc"
)

type TaskDependencyStore struct {
	q *sqlc.Queries
}

func NewTaskDependencyStore(q *sqlc.Queries) *TaskDependencyStore {
	return &TaskDependencyStore{q: q}
}

var _ port.TaskDependencyRepository = (*TaskDependencyStore)(nil)

func (s *TaskDependencyStore) Add(ctx context.Context, taskID, blocksTaskID string) error {
	return s.q.AddTaskDependency(ctx, sqlc.AddTaskDependencyParams{
		TaskID:       taskID,
		BlocksTaskID: blocksTaskID,
	})
}

func (s *TaskDependencyStore) Remove(ctx context.Context, taskID, blocksTaskID string) error {
	return s.q.RemoveTaskDependency(ctx, sqlc.RemoveTaskDependencyParams{
		TaskID:       taskID,
		BlocksTaskID: blocksTaskID,
	})
}

// ListBlocking returns the tasks that block the given task.
func (s *TaskDependencyStore) ListBlocking(ctx context.Context, taskID string) ([]*domain.Task, error) {
	rows, err := s.q.ListBlockingTasks(ctx, taskID)
	if err != nil {
		return nil, err
	}
	out := make([]*domain.Task, len(rows))
	for i, r := range rows {
		d := depRowToTask(r.ID, r.OrgID, r.ProjectID, r.CycleID, r.ParentTaskID, r.CreatedBy,
			r.Title, r.Description, r.StatusID, r.Priority, r.PositionKey, r.Estimate,
			r.StartedAt, r.DueAt, r.CompletedAt, r.CreatedAt, r.UpdatedAt)
		out[i] = &d
	}
	return out, nil
}

// ListBlocked returns the tasks that the given task is blocking.
func (s *TaskDependencyStore) ListBlocked(ctx context.Context, taskID string) ([]*domain.Task, error) {
	rows, err := s.q.ListBlockedTasks(ctx, taskID)
	if err != nil {
		return nil, err
	}
	out := make([]*domain.Task, len(rows))
	for i, r := range rows {
		d := depRowToTask(r.ID, r.OrgID, r.ProjectID, r.CycleID, r.ParentTaskID, r.CreatedBy,
			r.Title, r.Description, r.StatusID, r.Priority, r.PositionKey, r.Estimate,
			r.StartedAt, r.DueAt, r.CompletedAt, r.CreatedAt, r.UpdatedAt)
		out[i] = &d
	}
	return out, nil
}

// depRowToTask mirrors TaskStore.toDomain but takes the raw column values so
// it can serve both the blocking and blocked row types (structurally
// identical, but distinct generated types).
func depRowToTask(
	id, orgID, projectID string,
	cycleID, parentTaskID *string,
	createdBy, title, description, statusID, priority, positionKey string,
	estimate *int64,
	startedAt, dueAt, completedAt *string,
	createdAt, updatedAt string,
) domain.Task {
	return domain.Task{
		ID:          id,
		OrgID:       orgID,
		ProjectID:   projectID,
		CycleID:     cycleID,
		ParentID:    parentTaskID,
		CreatedBy:   createdBy,
		Title:       title,
		Description: description,
		StatusID:    statusID,
		Priority:    priority,
		PositionKey: positionKey,
		Estimate:    intPtr(estimate),
		StartedAt:   parseTimePtr(startedAt),
		DueAt:       parseTimePtr(dueAt),
		CompletedAt: parseTimePtr(completedAt),
		CreatedAt:   parseTime(createdAt),
		UpdatedAt:   parseTime(updatedAt),
	}
}
