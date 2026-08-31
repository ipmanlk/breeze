package service

import (
	"context"
	"errors"
	"strings"
	"testing"

	"ipmanlk/plume/internal/apperr"
	"ipmanlk/plume/internal/domain"
)

func TestTaskStatusService_Delete_NoTasks_Succeeds(t *testing.T) {
	repo := newMockTaskStatusRepo()
	svc := NewTaskStatusService(repo, nil)

	// Seed a status with zero tasks (default count is 0).
	st := &domain.TaskStatus{
		ID:        "status-1",
		ProjectID: "proj-1",
		Name:      "Todo",
		Color:     "#000",
		Position:  0,
	}
	if err := repo.Create(context.Background(), st); err != nil {
		t.Fatalf("create status: %v", err)
	}

	if err := svc.Delete(context.Background(), "", "org-1", "status-1", "proj-1"); err != nil {
		t.Errorf("expected no error, got: %v", err)
	}

	// Verify the status was actually deleted.
	if _, err := repo.GetByID(context.Background(), "status-1"); err == nil {
		t.Error("expected status to be deleted, but GetByID succeeded")
	}
}

func TestTaskStatusService_Delete_HasTasks_ReturnsInvalidInput(t *testing.T) {
	repo := newMockTaskStatusRepo()
	svc := NewTaskStatusService(repo, nil)

	// Seed a status with 3 referencing tasks.
	st := &domain.TaskStatus{
		ID:        "status-2",
		ProjectID: "proj-1",
		Name:      "In Progress",
		Color:     "#00f",
		Position:  0,
	}
	if err := repo.Create(context.Background(), st); err != nil {
		t.Fatalf("create status: %v", err)
	}
	repo.taskCountByStatus["status-2"] = 3

	err := svc.Delete(context.Background(), "", "org-1", "status-2", "proj-1")
	if err == nil {
		t.Fatal("expected error, got nil")
	}
	if !errors.Is(err, apperr.ErrInvalidInput) {
		t.Errorf("expected ErrInvalidInput, got: %v", err)
	}
	if !strings.Contains(err.Error(), "3") {
		t.Errorf("error message should include the task count '3', got: %v", err)
	}

	// Verify the status still exists (repo.Delete was never called).
	if _, err := repo.GetByID(context.Background(), "status-2"); err != nil {
		t.Errorf("expected status to exist (Delete not called), but GetByID failed: %v", err)
	}
}
