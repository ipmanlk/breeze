package store

import (
	"context"
	"database/sql"

	"ipmanlk/breeze/internal/domain"
	"ipmanlk/breeze/internal/port"
	"ipmanlk/breeze/internal/store/sqlc"
)

type LabelStore struct {
	q  *sqlc.Queries
	db *sql.DB
}

func NewLabelStore(q *sqlc.Queries, db *sql.DB) *LabelStore {
	return &LabelStore{q: q, db: db}
}

var _ port.LabelRepository = (*LabelStore)(nil)

func (s *LabelStore) GetByID(ctx context.Context, orgID, id string) (*domain.Label, error) {
	l, err := s.q.GetLabelByID(ctx, sqlc.GetLabelByIDParams{ID: id, OrgID: orgID})
	if err != nil {
		return nil, mapScanErr(err)
	}
	return &domain.Label{
		ID:    l.ID,
		OrgID: l.OrgID,
		Name:  l.Name,
		Color: l.Color,
	}, nil
}

func (s *LabelStore) ListByOrg(ctx context.Context, orgID string) ([]*domain.Label, error) {
	rows, err := s.q.ListLabelsByOrg(ctx, orgID)
	if err != nil {
		return nil, err
	}
	out := make([]*domain.Label, len(rows))
	for i, r := range rows {
		out[i] = &domain.Label{
			ID:    r.ID,
			OrgID: r.OrgID,
			Name:  r.Name,
			Color: r.Color,
		}
	}
	return out, nil
}

func (s *LabelStore) Create(ctx context.Context, label *domain.Label) error {
	return s.q.CreateLabel(ctx, sqlc.CreateLabelParams{
		ID:    label.ID,
		OrgID: label.OrgID,
		Name:  label.Name,
		Color: label.Color,
	})
}

func (s *LabelStore) Update(ctx context.Context, label *domain.Label) error {
	return s.q.UpdateLabel(ctx, sqlc.UpdateLabelParams{
		ID:    label.ID,
		OrgID: label.OrgID,
		Name:  label.Name,
		Color: label.Color,
	})
}

func (s *LabelStore) Delete(ctx context.Context, orgID, id string) error {
	return s.q.DeleteLabel(ctx, sqlc.DeleteLabelParams{ID: id, OrgID: orgID})
}

func (s *LabelStore) ClearTaskLabels(ctx context.Context, taskID string) error {
	return s.q.ClearTaskLabels(ctx, taskID)
}

func (s *LabelStore) AddTaskLabel(ctx context.Context, taskID, labelID string) error {
	return s.q.AddTaskLabel(ctx, sqlc.AddTaskLabelParams{TaskID: taskID, LabelID: labelID})
}

func (s *LabelStore) GetTaskLabels(ctx context.Context, taskID string) ([]*domain.Label, error) {
	rows, err := s.q.GetTaskLabels(ctx, taskID)
	if err != nil {
		return nil, err
	}
	out := make([]*domain.Label, len(rows))
	for i, r := range rows {
		out[i] = &domain.Label{
			ID:    r.ID,
			OrgID: r.OrgID,
			Name:  r.Name,
			Color: r.Color,
		}
	}
	return out, nil
}

func (s *LabelStore) ListLabelsByTaskIDs(ctx context.Context, taskIDs []string) (map[string][]*domain.Label, error) {
	rows, err := s.q.ListLabelsByTaskIDs(ctx, taskIDs)
	if err != nil {
		return nil, err
	}
	out := make(map[string][]*domain.Label)
	for _, r := range rows {
		l := &domain.Label{
			ID:    r.ID,
			OrgID: r.OrgID,
			Name:  r.Name,
			Color: r.Color,
		}
		out[r.TaskID] = append(out[r.TaskID], l)
	}
	return out, nil
}

// SetTaskLabels atomically replaces all labels for a task within a single
// transaction. Deletes existing associations then inserts the new set.
func (s *LabelStore) SetTaskLabels(ctx context.Context, taskID string, labelIDs []string) error {
	tx, err := s.db.BeginTx(ctx, nil)
	if err != nil {
		return err
	}
	defer tx.Rollback()

	q := s.q.WithTx(tx)

	if err := q.ClearTaskLabels(ctx, taskID); err != nil {
		return err
	}

	for _, labelID := range labelIDs {
		if err := q.AddTaskLabel(ctx, sqlc.AddTaskLabelParams{
			TaskID:  taskID,
			LabelID: labelID,
		}); err != nil {
			return err
		}
	}

	return tx.Commit()
}
