package service

import (
	"context"
	"errors"
	"testing"

	"ipmanlk/breeze/internal/apperr"
	"ipmanlk/breeze/internal/domain"
)

func TestTimeEntryService_Update_RequiresOwnershipOrAdmin(t *testing.T) {
	taskRepo := newMockTaskRepo()
	timeRepo := newMockTimeEntryRepo()

	taskRepo.tasksByID["task-1"] = &domain.Task{ID: "task-1", OrgID: "org-1", ProjectID: "proj-1"}
	timeRepo.entriesByTask["task-1"] = []*domain.TimeEntry{
		{ID: "entry-1", TaskID: "task-1", UserID: "owner-1", Description: "Original", DurationMinutes: intPtr(60)},
	}

	svc := NewTimeEntryService(timeRepo, taskRepo, nil, nil, nil)

	desc := "Updated"
	// Owner updates own entry: OK.
	if _, err := svc.Update(context.Background(), "owner-1", domain.RoleMember, domain.UpdateTimeEntryParams{
		ID: "entry-1", OrgID: "org-1", TaskID: "task-1", ProjectID: "proj-1", Description: &desc,
	}); err != nil {
		t.Fatalf("owner update: %v", err)
	}

	// Another member tries to update: forbidden.
	if _, err := svc.Update(context.Background(), "other-1", domain.RoleMember, domain.UpdateTimeEntryParams{
		ID: "entry-1", OrgID: "org-1", TaskID: "task-1", ProjectID: "proj-1", Description: &desc,
	}); !isForbidden(err) {
		t.Fatalf("expected forbidden for other member, got %v", err)
	}

	// Admin can update on behalf.
	if _, err := svc.Update(context.Background(), "admin-1", domain.RoleAdmin, domain.UpdateTimeEntryParams{
		ID: "entry-1", OrgID: "org-1", TaskID: "task-1", ProjectID: "proj-1", Description: &desc,
	}); err != nil {
		t.Fatalf("admin update: %v", err)
	}
}

func TestTimeEntryService_Delete_RequiresOwnershipOrAdmin(t *testing.T) {
	taskRepo := newMockTaskRepo()
	timeRepo := newMockTimeEntryRepo()

	taskRepo.tasksByID["task-1"] = &domain.Task{ID: "task-1", OrgID: "org-1", ProjectID: "proj-1"}
	timeRepo.entriesByTask["task-1"] = []*domain.TimeEntry{
		{ID: "entry-1", TaskID: "task-1", UserID: "owner-1"},
	}

	svc := NewTimeEntryService(timeRepo, taskRepo, nil, nil, nil)

	if err := svc.Delete(context.Background(), "other-1", domain.RoleMember, "org-1", "entry-1", "task-1", "proj-1"); !isForbidden(err) {
		t.Fatalf("expected forbidden for other member, got %v", err)
	}
	if err := svc.Delete(context.Background(), "owner-1", domain.RoleMember, "org-1", "entry-1", "task-1", "proj-1"); err != nil {
		t.Fatalf("owner delete: %v", err)
	}
}

func intPtr(v int) *int {
	return &v
}

func isForbidden(err error) bool {
	return err != nil && (err == apperr.ErrForbidden || errors.Is(err, apperr.ErrForbidden))
}
