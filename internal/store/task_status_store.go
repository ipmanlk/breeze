package store

import (
	"context"

	"ipmanlk/plume/internal/domain"
	"ipmanlk/plume/internal/port"
	"ipmanlk/plume/internal/store/sqlc"
)

type TaskStatusStore struct {
	q *sqlc.Queries
}

func NewTaskStatusStore(q *sqlc.Queries) *TaskStatusStore {
	return &TaskStatusStore{q: q}
}

var _ port.TaskStatusRepository = (*TaskStatusStore)(nil)

func (s *TaskStatusStore) toDomain(st sqlc.TaskStatus) domain.TaskStatus {
	return domain.TaskStatus{
		ID:        st.ID,
		ProjectID: st.ProjectID,
		Name:      st.Name,
		Color:     st.Color,
		Position:  int(st.Position),
		Category:  st.Category,
		Default:   st.IsDefault,
	}
}

func (s *TaskStatusStore) ListByProject(ctx context.Context, projectID string) ([]*domain.TaskStatus, error) {
	rows, err := s.q.ListStatusesByProject(ctx, projectID)
	if err != nil {
		return nil, err
	}
	statuses := make([]*domain.TaskStatus, len(rows))
	for i, row := range rows {
		d := s.toDomain(row)
		statuses[i] = &d
	}
	return statuses, nil
}

func (s *TaskStatusStore) GetByID(ctx context.Context, id string) (*domain.TaskStatus, error) {
	st, err := s.q.GetStatusByID(ctx, id)
	if err != nil {
		return nil, mapScanErr(err)
	}
	d := s.toDomain(st)
	return &d, nil
}

func (s *TaskStatusStore) Create(ctx context.Context, st *domain.TaskStatus) error {
	return s.q.CreateStatus(ctx, sqlc.CreateStatusParams{
		ID:        st.ID,
		ProjectID: st.ProjectID,
		Name:      st.Name,
		Color:     st.Color,
		Position:  int64(st.Position),
		Category:  st.Category,
		IsDefault: st.Default,
	})
}

func (s *TaskStatusStore) Update(ctx context.Context, st *domain.TaskStatus) error {
	return s.q.UpdateStatus(ctx, sqlc.UpdateStatusParams{
		Name:      st.Name,
		Color:     st.Color,
		Position:  int64(st.Position),
		Category:  st.Category,
		ID:        st.ID,
		ProjectID: st.ProjectID,
	})
}

func (s *TaskStatusStore) Delete(ctx context.Context, id, projectID string) error {
	return s.q.DeleteStatus(ctx, sqlc.DeleteStatusParams{ID: id, ProjectID: projectID})
}

func (s *TaskStatusStore) CountTasksByStatus(ctx context.Context, statusID, projectID string) (int64, error) {
	return s.q.CountTasksByStatus(ctx, sqlc.CountTasksByStatusParams{
		StatusID:  statusID,
		ProjectID: projectID,
	})
}

func (s *TaskStatusStore) ReassignTasks(ctx context.Context, toStatusID, fromStatusID, projectID string) error {
	return s.q.ReassignTasksOnStatusDelete(ctx, sqlc.ReassignTasksOnStatusDeleteParams{
		NewStatusID: toStatusID,
		OldStatusID: fromStatusID,
		ProjectID:   projectID,
	})
}
