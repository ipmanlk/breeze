package handler

import (
	"bytes"
	"context"
	"encoding/json"
	"log/slog"
	"net/http"
	"net/http/httptest"
	"testing"
	"time"

	"ipmanlk/plume/internal/domain"

	"github.com/go-chi/chi/v5"
)

type mockTaskService struct {
	getByIDFn func(ctx context.Context, orgID, id, projectID string) (*domain.Task, error)
	updateFn  func(ctx context.Context, t *domain.Task) error
}

func (m *mockTaskService) List(ctx context.Context, orgID, projectID string, filter domain.TaskFilter) ([]*domain.Task, error) {
	return nil, nil
}
func (m *mockTaskService) GetByID(ctx context.Context, orgID, id, projectID string) (*domain.Task, error) {
	return m.getByIDFn(ctx, orgID, id, projectID)
}
func (m *mockTaskService) Create(ctx context.Context, params domain.CreateTaskParams) (*domain.Task, error) {
	return nil, nil
}
func (m *mockTaskService) Update(ctx context.Context, actorID string, t *domain.Task) error {
	return m.updateFn(ctx, t)
}
func (m *mockTaskService) Delete(ctx context.Context, orgID, id, projectID string, _ domain.DeleteSubtaskMode, _ string) error {
	return nil
}
func (m *mockTaskService) Move(ctx context.Context, actorID, orgID, id, projectID, statusID, positionKey string) error {
	return nil
}

func (m *mockTaskService) ListActivity(ctx context.Context, orgID, projectID, taskID string, filter domain.TaskActivityFilter) (*domain.TaskActivityResult, error) {
	return &domain.TaskActivityResult{Items: nil}, nil
}

func (m *mockTaskService) ListTasks(ctx context.Context, orgID, userID string, role domain.Role, filter domain.TaskListFilter) (*domain.TaskListResult, error) {
	return &domain.TaskListResult{}, nil
}
func (m *mockTaskService) Reorder(ctx context.Context, orgID, projectID string, ops []domain.ReorderOp) error {
	return nil
}

func (m *mockTaskService) ListSubtasks(ctx context.Context, orgID, projectID, parentID string) ([]*domain.Task, error) {
	return nil, nil
}

func (m *mockTaskService) ReorderSubtasks(ctx context.Context, orgID, projectID, parentID string, ops []domain.ReorderOp) error {
	return nil
}

func (m *mockTaskService) BatchUpdate(ctx context.Context, orgID string, params domain.BatchUpdateParams, _ string) ([]*domain.Task, error) {
	return nil, nil
}

func (m *mockTaskService) Duplicate(ctx context.Context, orgID, taskID, projectID string, _ bool, _ string) (*domain.Task, error) {
	return nil, nil
}

func (m *mockTaskService) MoveToProject(ctx context.Context, orgID, taskID, fromProjectID, toProjectID, toStatusID string, _ string) (*domain.Task, error) {
	return nil, nil
}

func TestTaskHandler_Update_AppliesStartedAtAndDueAt(t *testing.T) {
	existing := &domain.Task{
		ID:          "task-1",
		OrgID:       "org-1",
		ProjectID:   "proj-1",
		CreatedBy:   "user-1",
		Title:       "Original title",
		Description: "Original desc",
		StatusID:    "status-1",
		Priority:    "none",
		PositionKey: "a0",
	}

	startedAt := "2024-06-15T00:00:00Z"
	dueAt := "2024-06-20T00:00:00Z"

	var captured *domain.Task
	mock := &mockTaskService{
		getByIDFn: func(ctx context.Context, orgID, id, projectID string) (*domain.Task, error) {
			return existing, nil
		},
		updateFn: func(ctx context.Context, t *domain.Task) error {
			captured = t
			return nil
		},
	}
	log := slog.Default()
	h := NewTaskHandler(mock, &mockAccessService{}, log)

	body, _ := json.Marshal(map[string]any{
		"started_at": startedAt,
		"due_at":     dueAt,
	})
	req := httptest.NewRequest(
		http.MethodPut,
		"/projects/proj-1/tasks/task-1",
		bytes.NewReader(body),
	)
	req.Header.Set("Content-Type", "application/json")

	rctx := chi.NewRouteContext()
	rctx.URLParams.Add("id", "proj-1")
	rctx.URLParams.Add("taskId", "task-1")
	ctx := context.WithValue(context.Background(), chi.RouteCtxKey, rctx)
	ctx = context.WithValue(ctx, domain.CtxRole, "owner")
	ctx = context.WithValue(ctx, domain.CtxOrgID, "org-1")
	req = req.WithContext(ctx)

	rr := httptest.NewRecorder()
	h.Update(rr, req)

	if rr.Code != http.StatusOK {
		t.Fatalf("expected 200, got %d: %s", rr.Code, rr.Body.String())
	}
	if captured == nil {
		t.Fatal("expected service.Update to be called")
	}
	if captured.StartedAt == nil || !captured.StartedAt.Equal(parseTime(t, startedAt)) {
		t.Errorf("started_at not applied: %+v", captured.StartedAt)
	}
	if captured.DueAt == nil || !captured.DueAt.Equal(parseTime(t, dueAt)) {
		t.Errorf("due_at not applied: %+v", captured.DueAt)
	}
}

func TestTaskHandler_Update_ClearsStartedAtWhenEmptyString(t *testing.T) {
	started := time.Date(2024, 6, 15, 0, 0, 0, 0, time.UTC)
	existing := &domain.Task{
		ID:        "task-1",
		OrgID:     "org-1",
		ProjectID: "proj-1",
		Title:     "Original",
		StartedAt: &started,
	}

	var captured *domain.Task
	mock := &mockTaskService{
		getByIDFn: func(ctx context.Context, orgID, id, projectID string) (*domain.Task, error) {
			return existing, nil
		},
		updateFn: func(ctx context.Context, t *domain.Task) error {
			captured = t
			return nil
		},
	}
	h := NewTaskHandler(mock, &mockAccessService{}, slog.Default())

	body, _ := json.Marshal(map[string]any{
		"started_at": "",
	})
	req := httptest.NewRequest(
		http.MethodPut,
		"/projects/proj-1/tasks/task-1",
		bytes.NewReader(body),
	)
	req.Header.Set("Content-Type", "application/json")
	req = req.WithContext(context.WithValue(req.Context(), chi.RouteCtxKey, func() *chi.Context {
		rctx := chi.NewRouteContext()
		rctx.URLParams.Add("id", "proj-1")
		rctx.URLParams.Add("taskId", "task-1")
		return rctx
	}()))
	ctx := context.WithValue(req.Context(), domain.CtxRole, "owner")
	ctx = context.WithValue(ctx, domain.CtxOrgID, "org-1")
	req = req.WithContext(ctx)

	rr := httptest.NewRecorder()
	h.Update(rr, req)

	if rr.Code != http.StatusOK {
		t.Fatalf("expected 200, got %d: %s", rr.Code, rr.Body.String())
	}
	if captured == nil {
		t.Fatal("expected service.Update to be called")
	}
	if captured.StartedAt != nil {
		t.Errorf("expected started_at to be cleared, got %+v", captured.StartedAt)
	}
}

func parseTime(t *testing.T, s string) time.Time {
	t.Helper()
	v, err := time.Parse(time.RFC3339, s)
	if err != nil {
		t.Fatalf("parse %q: %v", s, err)
	}
	return v
}

func TestTaskHandler_ListTasks_InvalidLimitReturns400(t *testing.T) {
	log := slog.Default()
	h := NewTaskHandler(&mockTaskService{}, &mockAccessService{}, log)

	req := httptest.NewRequest(http.MethodGet, "/api/tasks?limit=abc", nil)
	rctx := chi.NewRouteContext()
	ctx := context.WithValue(context.Background(), chi.RouteCtxKey, rctx)
	ctx = context.WithValue(ctx, domain.CtxRole, "owner")
	ctx = context.WithValue(ctx, domain.CtxOrgID, "org-1")
	ctx = context.WithValue(ctx, domain.CtxUserID, "user-1")
	req = req.WithContext(ctx)

	rr := httptest.NewRecorder()
	h.ListTasks(rr, req)

	if rr.Code != http.StatusBadRequest {
		t.Fatalf("expected 400, got %d: %s", rr.Code, rr.Body.String())
	}

	// Also verify a valid limit still works
	req2 := httptest.NewRequest(http.MethodGet, "/api/tasks?limit=10", nil)
	rctx2 := chi.NewRouteContext()
	ctx2 := context.WithValue(context.Background(), chi.RouteCtxKey, rctx2)
	ctx2 = context.WithValue(ctx2, domain.CtxRole, "owner")
	ctx2 = context.WithValue(ctx2, domain.CtxOrgID, "org-1")
	ctx2 = context.WithValue(ctx2, domain.CtxUserID, "user-1")
	req2 = req2.WithContext(ctx2)

	rr2 := httptest.NewRecorder()
	h.ListTasks(rr2, req2)

	if rr2.Code != http.StatusOK {
		t.Fatalf("expected 200, got %d: %s", rr2.Code, rr2.Body.String())
	}
}
