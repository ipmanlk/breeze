package store

import (
	"context"
	"encoding/json"

	"ipmanlk/breeze/internal/domain"
	"ipmanlk/breeze/internal/port"
	"ipmanlk/breeze/internal/store/sqlc"
)

type CustomFieldStore struct {
	q *sqlc.Queries
}

func NewCustomFieldStore(q *sqlc.Queries) *CustomFieldStore {
	return &CustomFieldStore{q: q}
}

var _ port.CustomFieldRepository = (*CustomFieldStore)(nil)

func fieldRowToDomain(r sqlc.CustomField) (*domain.CustomField, error) {
	var options []string
	if err := json.Unmarshal([]byte(r.Options), &options); err != nil {
		options = nil
	}
	return &domain.CustomField{
		ID:        r.ID,
		OrgID:     r.OrgID,
		ProjectID: r.ProjectID,
		Name:      r.Name,
		FieldType: r.FieldType,
		Options:   options,
		Position:  int(r.Position),
		CreatedAt: parseTime(r.CreatedAt),
		UpdatedAt: parseTime(r.UpdatedAt),
	}, nil
}

func (s *CustomFieldStore) Create(ctx context.Context, f *domain.CustomField) error {
	optionsJSON, _ := json.Marshal(f.Options)
	return s.q.CreateCustomField(ctx, sqlc.CreateCustomFieldParams{
		ID:        f.ID,
		OrgID:     f.OrgID,
		ProjectID: f.ProjectID,
		Name:      f.Name,
		FieldType: f.FieldType,
		Options:   string(optionsJSON),
		Position:  int64(f.Position),
	})
}

func (s *CustomFieldStore) GetByID(ctx context.Context, orgID, id string) (*domain.CustomField, error) {
	r, err := s.q.GetCustomFieldByID(ctx, sqlc.GetCustomFieldByIDParams{ID: id, OrgID: orgID})
	if err != nil {
		return nil, mapScanErr(err)
	}
	return fieldRowToDomain(r)
}

func (s *CustomFieldStore) ListByProject(ctx context.Context, orgID, projectID string) ([]*domain.CustomField, error) {
	rows, err := s.q.ListCustomFieldsByProject(ctx, sqlc.ListCustomFieldsByProjectParams{ProjectID: projectID, OrgID: orgID})
	if err != nil {
		return nil, err
	}
	out := make([]*domain.CustomField, len(rows))
	for i, r := range rows {
		f, err := fieldRowToDomain(r)
		if err != nil {
			return nil, err
		}
		out[i] = f
	}
	return out, nil
}

func (s *CustomFieldStore) Update(ctx context.Context, f *domain.CustomField) error {
	optionsJSON, _ := json.Marshal(f.Options)
	return s.q.UpdateCustomField(ctx, sqlc.UpdateCustomFieldParams{
		Name:     f.Name,
		Options:  string(optionsJSON),
		Position: int64(f.Position),
		ID:       f.ID,
		OrgID:    f.OrgID,
	})
}

func (s *CustomFieldStore) Delete(ctx context.Context, orgID, id string) error {
	return s.q.DeleteCustomField(ctx, sqlc.DeleteCustomFieldParams{ID: id, OrgID: orgID})
}

func (s *CustomFieldStore) SetValue(ctx context.Context, taskID, fieldID, value string) error {
	return s.q.SetTaskCustomFieldValue(ctx, sqlc.SetTaskCustomFieldValueParams{
		TaskID:        taskID,
		CustomFieldID: fieldID,
		Value:         value,
	})
}

func (s *CustomFieldStore) GetValuesByTask(ctx context.Context, taskID string) (map[string]string, error) {
	rows, err := s.q.GetTaskCustomFieldValues(ctx, taskID)
	if err != nil {
		return nil, err
	}
	out := make(map[string]string, len(rows))
	for _, r := range rows {
		out[r.CustomFieldID] = r.Value
	}
	return out, nil
}

func (s *CustomFieldStore) ListValuesByTaskIDs(ctx context.Context, taskIDs []string) (map[string]map[string]string, error) {
	if len(taskIDs) == 0 {
		return nil, nil
	}
	rows, err := s.q.ListTaskCustomFieldValuesByTaskIDs(ctx, taskIDs)
	if err != nil {
		return nil, err
	}
	out := make(map[string]map[string]string)
	for _, r := range rows {
		if out[r.TaskID] == nil {
			out[r.TaskID] = make(map[string]string)
		}
		out[r.TaskID][r.CustomFieldID] = r.Value
	}
	return out, nil
}

func (s *CustomFieldStore) DeleteValuesForTask(ctx context.Context, taskID string) error {
	return s.q.DeleteTaskCustomFieldValues(ctx, taskID)
}
