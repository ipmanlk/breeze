package store

import (
	"context"
	"database/sql"
	"encoding/json"
	"time"

	"ipmanlk/breeze/internal/domain"
	"ipmanlk/breeze/internal/port"
	"ipmanlk/breeze/internal/store/sqlc"
)

type TaskTemplateStore struct {
	q  *sqlc.Queries
	db *sql.DB
}

func NewTaskTemplateStore(q *sqlc.Queries, db *sql.DB) *TaskTemplateStore {
	return &TaskTemplateStore{q: q, db: db}
}

var _ port.TaskTemplateRepository = (*TaskTemplateStore)(nil)

func (s *TaskTemplateStore) toDomain(r sqlc.TaskTemplate) (*domain.TaskTemplate, error) {
	var assigneeIDs []string
	if err := json.Unmarshal([]byte(r.AssigneeIds), &assigneeIDs); err != nil {
		assigneeIDs = nil
	}
	var nextRunAt *time.Time
	if r.NextRunAt != nil && *r.NextRunAt != "" {
		if t, err := time.Parse(time.RFC3339, *r.NextRunAt); err == nil {
			nextRunAt = &t
		}
	}
	var estimate *int
	if r.Estimate != nil {
		e := int(*r.Estimate)
		estimate = &e
	}
	return &domain.TaskTemplate{
		ID:                r.ID,
		OrgID:             r.OrgID,
		ProjectID:         r.ProjectID,
		Name:              r.Name,
		Description:       r.Description,
		Priority:          r.Priority,
		StatusID:          r.StatusID,
		AssigneeIDs:       assigneeIDs,
		Estimate:          estimate,
		RecurrencePattern: r.RecurrencePattern,
		RecurrenceDays:    r.RecurrenceDays,
		NextRunAt:         nextRunAt,
		LastError:         derefStr(r.LastError),
		LastErrorAt:       parseTimePtr(r.LastErrorAt),
		CreatedBy:         r.CreatedBy,
		CreatedAt:         parseTime(r.CreatedAt),
		UpdatedAt:         parseTime(r.UpdatedAt),
	}, nil
}

func (s *TaskTemplateStore) Create(ctx context.Context, t *domain.TaskTemplate) error {
	assigneeJSON, _ := json.Marshal(t.AssigneeIDs)
	var nextRunStr *string
	if t.NextRunAt != nil {
		ts := t.NextRunAt.Format(time.RFC3339)
		nextRunStr = &ts
	}
	return s.q.CreateTaskTemplate(ctx, sqlc.CreateTaskTemplateParams{
		ID:                t.ID,
		OrgID:             t.OrgID,
		ProjectID:         t.ProjectID,
		Name:              t.Name,
		Description:       t.Description,
		Priority:          t.Priority,
		StatusID:          t.StatusID,
		AssigneeIds:       string(assigneeJSON),
		Estimate:          int64Ptr(t.Estimate),
		RecurrencePattern: t.RecurrencePattern,
		RecurrenceDays:    t.RecurrenceDays,
		NextRunAt:         nextRunStr,
		CreatedBy:         t.CreatedBy,
	})
}

func (s *TaskTemplateStore) GetByID(ctx context.Context, orgID, id string) (*domain.TaskTemplate, error) {
	r, err := s.q.GetTaskTemplateByID(ctx, sqlc.GetTaskTemplateByIDParams{ID: id, OrgID: orgID})
	if err != nil {
		return nil, mapScanErr(err)
	}
	return s.toDomain(r)
}

func (s *TaskTemplateStore) ListByProject(ctx context.Context, orgID, projectID string) ([]*domain.TaskTemplate, error) {
	rows, err := s.q.ListTaskTemplatesByProject(ctx, sqlc.ListTaskTemplatesByProjectParams{ProjectID: projectID, OrgID: orgID})
	if err != nil {
		return nil, err
	}
	out := make([]*domain.TaskTemplate, len(rows))
	for i, r := range rows {
		t, err := s.toDomain(r)
		if err != nil {
			return nil, err
		}
		out[i] = t
	}
	return out, nil
}

func (s *TaskTemplateStore) ListDueRecurring(ctx context.Context, before time.Time) ([]*domain.TaskTemplate, error) {
	beforeStr := before.Format(time.RFC3339)
	rows, err := s.q.ListDueRecurringTemplates(ctx, &beforeStr)
	if err != nil {
		return nil, err
	}
	out := make([]*domain.TaskTemplate, len(rows))
	for i, r := range rows {
		t, err := s.toDomain(r)
		if err != nil {
			return nil, err
		}
		out[i] = t
	}
	return out, nil
}

func (s *TaskTemplateStore) Update(ctx context.Context, t *domain.TaskTemplate) error {
	assigneeJSON, _ := json.Marshal(t.AssigneeIDs)
	var nextRunStr *string
	if t.NextRunAt != nil {
		ts := t.NextRunAt.Format(time.RFC3339)
		nextRunStr = &ts
	}
	return s.q.UpdateTaskTemplate(ctx, sqlc.UpdateTaskTemplateParams{
		Name:              t.Name,
		Description:       t.Description,
		Priority:          t.Priority,
		StatusID:          t.StatusID,
		AssigneeIds:       string(assigneeJSON),
		Estimate:          int64Ptr(t.Estimate),
		RecurrencePattern: t.RecurrencePattern,
		RecurrenceDays:    t.RecurrenceDays,
		NextRunAt:         nextRunStr,
		ID:                t.ID,
		OrgID:             t.OrgID,
	})
}

func (s *TaskTemplateStore) UpdateNextRun(ctx context.Context, orgID, id string, nextRun *time.Time) error {
	var nextRunStr *string
	if nextRun != nil {
		ts := nextRun.Format(time.RFC3339)
		nextRunStr = &ts
	}
	return s.q.UpdateTaskTemplateNextRun(ctx, sqlc.UpdateTaskTemplateNextRunParams{
		NextRunAt: nextRunStr,
		ID:        id,
		OrgID:     orgID,
	})
}

func (s *TaskTemplateStore) ClaimDueRecurring(ctx context.Context, orgID, id string, currentNextRun, newNextRun *time.Time) (bool, error) {
	var curStr, newStr *string
	if currentNextRun != nil {
		s := currentNextRun.Format(time.RFC3339)
		curStr = &s
	}
	if newNextRun != nil {
		s := newNextRun.Format(time.RFC3339)
		newStr = &s
	}
	n, err := s.q.ClaimDueRecurringTemplate(ctx, sqlc.ClaimDueRecurringTemplateParams{
		NextRunAt:   newStr,
		ID:          id,
		OrgID:       orgID,
		NextRunAt_2: curStr,
	})
	if err != nil {
		return false, err
	}
	return n > 0, nil
}

// SetLastError records (or clears, when msg is empty) the last instantiation
// error for a recurring template. Best-effort visibility for silent failures.
func (s *TaskTemplateStore) SetLastError(ctx context.Context, orgID, id, msg string) error {
	if msg == "" {
		return s.q.ClearTemplateLastError(ctx, sqlc.ClearTemplateLastErrorParams{ID: id, OrgID: orgID})
	}
	return s.q.SetTemplateLastError(ctx, sqlc.SetTemplateLastErrorParams{LastError: &msg, ID: id, OrgID: orgID})
}

func (s *TaskTemplateStore) Delete(ctx context.Context, orgID, id string) error {
	return s.q.DeleteTaskTemplate(ctx, sqlc.DeleteTaskTemplateParams{ID: id, OrgID: orgID})
}
