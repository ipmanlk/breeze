package service

import (
	"context"
	"errors"
	"testing"

	"ipmanlk/plume/internal/apperr"
	"ipmanlk/plume/internal/domain"
	"ipmanlk/plume/internal/port"
)

// stubAccessChecker is a configurable AccessChecker double for tests that
// exercise access-gated read paths.
type stubAccessChecker struct {
	deny bool
}

func (s *stubAccessChecker) RequireProjectAccess(ctx context.Context, userID, orgID, projectID string, perm domain.Permission) error {
	if s.deny {
		return apperr.ErrForbidden
	}
	return nil
}

func (s *stubAccessChecker) RequireTaskAccess(ctx context.Context, userID, orgID, taskID string, perm domain.Permission) error {
	if s.deny {
		return apperr.ErrForbidden
	}
	return nil
}

func (s *stubAccessChecker) RequireOrgAccess(ctx context.Context, userID, orgID string, perm domain.Permission) error {
	if s.deny {
		return apperr.ErrForbidden
	}
	return nil
}

type stubFieldRepo struct {
	values map[string]string
}

func (s *stubFieldRepo) Create(ctx context.Context, f *domain.CustomField) error {
	return errors.New("unused")
}
func (s *stubFieldRepo) Update(ctx context.Context, f *domain.CustomField) error {
	return errors.New("unused")
}
func (s *stubFieldRepo) Delete(ctx context.Context, orgID, id string) error {
	return errors.New("unused")
}
func (s *stubFieldRepo) GetByID(ctx context.Context, orgID, id string) (*domain.CustomField, error) {
	return nil, errors.New("unused")
}
func (s *stubFieldRepo) ListByProject(ctx context.Context, orgID, projectID string) ([]*domain.CustomField, error) {
	return nil, errors.New("unused")
}
func (s *stubFieldRepo) SetValue(ctx context.Context, taskID, fieldID, value string) error {
	return errors.New("unused")
}
func (s *stubFieldRepo) GetValuesByTask(ctx context.Context, taskID string) (map[string]string, error) {
	return s.values, nil
}
func (s *stubFieldRepo) ListValuesByTaskIDs(ctx context.Context, taskIDs []string) (map[string]map[string]string, error) {
	return nil, errors.New("unused")
}
func (s *stubFieldRepo) DeleteValuesForTask(ctx context.Context, taskID string) error {
	return errors.New("unused")
}

var _ port.CustomFieldRepository = (*stubFieldRepo)(nil)

// TestCustomFieldService_GetTaskValues_EnforcesTaskAccess guards the read
// path: fetching a task's custom-field values must resolve the caller's
// project role first, exactly like the write path does. Without this check a
// member of any project could read another project's (or org's) field values
// by supplying a raw task ID.
func TestCustomFieldService_GetTaskValues_EnforcesTaskAccess(t *testing.T) {
	repo := &stubFieldRepo{values: map[string]string{"field-1": "high"}}

	t.Run("denied when the access checker rejects the caller", func(t *testing.T) {
		svc := NewCustomFieldService(repo, nil, &stubAccessChecker{deny: true})
		if _, err := svc.GetTaskValues(context.Background(), "user-1", "org-1", "task-1"); !errors.Is(err, apperr.ErrForbidden) {
			t.Fatalf("expected ErrForbidden, got %v", err)
		}
	})

	t.Run("allowed when the caller has task:view", func(t *testing.T) {
		svc := NewCustomFieldService(repo, nil, &stubAccessChecker{deny: false})
		values, err := svc.GetTaskValues(context.Background(), "user-1", "org-1", "task-1")
		if err != nil {
			t.Fatalf("unexpected error: %v", err)
		}
		if values["field-1"] != "high" {
			t.Errorf("values = %v, want field-1=high", values)
		}
	})
}
