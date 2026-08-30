package service

import (
	"context"
	"errors"
	"testing"

	"ipmanlk/breeze/internal/apperr"
	"ipmanlk/breeze/internal/domain"
)

func TestTaskDependencyService_Add_RejectsSelfAndCycle(t *testing.T) {
	taskRepo := newMockTaskRepo()
	depRepo := newMockTaskDepRepo()
	svc := NewTaskDependencyService(depRepo, taskRepo, nil, nil, nil)

	taskRepo.tasksByID["t-a"] = &domain.Task{ID: "t-a", OrgID: "org-1", ProjectID: "proj-1"}
	taskRepo.tasksByID["t-b"] = &domain.Task{ID: "t-b", OrgID: "org-1", ProjectID: "proj-1"}

	// Self-dependency.
	if err := svc.Add(context.Background(), "", "org-1", "t-a", "t-a"); !errors.Is(err, apperr.ErrInvalidInput) {
		t.Errorf("self-add err = %v, want ErrInvalidInput", err)
	}

	// t-a is blocked by t-b (A waits on B).
	if err := svc.Add(context.Background(), "", "org-1", "t-a", "t-b"); err != nil {
		t.Fatalf("Add t-a<-t-b: %v", err)
	}
	// Now try the reverse (t-b blocked by t-a): must be rejected as a cycle.
	if err := svc.Add(context.Background(), "", "org-1", "t-b", "t-a"); !errors.Is(err, apperr.ErrInvalidInput) {
		t.Errorf("cycle add err = %v, want ErrInvalidInput", err)
	}
}

// mockTaskDepRepo is an in-memory TaskDependencyRepository for service tests.
type mockTaskDepRepo struct {
	// edges: taskID -> set of blocksTaskID
	edges map[string]map[string]bool
}

func newMockTaskDepRepo() *mockTaskDepRepo {
	return &mockTaskDepRepo{edges: make(map[string]map[string]bool)}
}

func (m *mockTaskDepRepo) Add(_ context.Context, taskID, blocksTaskID string) error {
	if m.edges[taskID] == nil {
		m.edges[taskID] = make(map[string]bool)
	}
	m.edges[taskID][blocksTaskID] = true
	return nil
}

func (m *mockTaskDepRepo) Remove(_ context.Context, taskID, blocksTaskID string) error {
	if s, ok := m.edges[taskID]; ok {
		delete(s, blocksTaskID)
	}
	return nil
}

func (m *mockTaskDepRepo) ListBlocking(_ context.Context, taskID string) ([]*domain.Task, error) {
	out := []*domain.Task{}
	for b := range m.edges[taskID] {
		out = append(out, &domain.Task{ID: b})
	}
	return out, nil
}

func (m *mockTaskDepRepo) ListBlocked(_ context.Context, taskID string) ([]*domain.Task, error) {
	out := []*domain.Task{}
	for taskID2, blocks := range m.edges {
		if blocks[taskID] {
			out = append(out, &domain.Task{ID: taskID2})
		}
	}
	return out, nil
}
