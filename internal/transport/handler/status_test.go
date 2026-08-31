package handler

import (
	"context"
	"encoding/json"
	"log/slog"
	"net/http"
	"net/http/httptest"
	"testing"

	"ipmanlk/plume/internal/apperr"
	"ipmanlk/plume/internal/domain"
	"ipmanlk/plume/internal/port"
)

// mockTaskStatusService implements port.TaskStatusService for handler tests.
type mockTaskStatusService struct {
	byID    map[string]*domain.TaskStatus
	updated *domain.TaskStatus // last status passed to Update
}

func newMockTaskStatusService() *mockTaskStatusService {
	return &mockTaskStatusService{byID: make(map[string]*domain.TaskStatus)}
}

func (m *mockTaskStatusService) List(ctx context.Context, projectID string) ([]*domain.TaskStatus, error) {
	out := make([]*domain.TaskStatus, 0, len(m.byID))
	for _, s := range m.byID {
		if s.ProjectID == projectID {
			out = append(out, s)
		}
	}
	return out, nil
}
func (m *mockTaskStatusService) GetByID(ctx context.Context, id string) (*domain.TaskStatus, error) {
	if s, ok := m.byID[id]; ok {
		return s, nil
	}
	return nil, apperr.ErrNotFound
}
func (m *mockTaskStatusService) Create(ctx context.Context, userID, orgID string, params domain.CreateTaskStatusParams) (*domain.TaskStatus, error) {
	s := &domain.TaskStatus{ID: "s-new", ProjectID: params.ProjectID, Name: params.Name, Color: params.Color, Position: params.Position, Category: params.Category}
	m.byID[s.ID] = s
	return s, nil
}
func (m *mockTaskStatusService) Update(ctx context.Context, userID, orgID string, s *domain.TaskStatus) error {
	m.updated = s
	m.byID[s.ID] = s
	return nil
}
func (m *mockTaskStatusService) Delete(ctx context.Context, userID, orgID, id, projectID string) error {
	delete(m.byID, id)
	return nil
}

// compile-time check that the mock satisfies the port interface.
var _ port.TaskStatusService = (*mockTaskStatusService)(nil)

type mockProjectRepo struct{}

func (m *mockProjectRepo) List(ctx context.Context, orgID string) ([]*domain.Project, error) {
	return nil, nil
}
func (m *mockProjectRepo) ListForUser(ctx context.Context, orgID, userID string) ([]*domain.Project, error) {
	return nil, nil
}
func (m *mockProjectRepo) ListIncludingArchived(ctx context.Context, orgID string) ([]*domain.Project, error) {
	return nil, nil
}
func (m *mockProjectRepo) ListForUserIncludingArchived(ctx context.Context, orgID, userID string) ([]*domain.Project, error) {
	return nil, nil
}
func (m *mockProjectRepo) ListByIDs(ctx context.Context, orgID string, ids []string) ([]*domain.Project, error) {
	return nil, nil
}
func (m *mockProjectRepo) GetByID(ctx context.Context, orgID, id string) (*domain.Project, error) {
	return &domain.Project{ID: id, OrgID: orgID}, nil
}
func (m *mockProjectRepo) GetBySlug(ctx context.Context, orgID, slug string) (*domain.Project, error) {
	return nil, nil
}
func (m *mockProjectRepo) Create(ctx context.Context, p *domain.Project) error { return nil }
func (m *mockProjectRepo) Update(ctx context.Context, p *domain.Project) error { return nil }
func (m *mockProjectRepo) Delete(ctx context.Context, orgID, id string) error  { return nil }
func (m *mockProjectRepo) SetArchived(_ context.Context, _ string, _ string, _ bool) error {
	return nil
}

func (m *mockProjectRepo) CreateWithStatuses(_ context.Context, _ *domain.Project, _ []*domain.TaskStatus) error {
	return nil
}

var _ port.ProjectRepository = (*mockProjectRepo)(nil)

// TestTaskStatusHandler_Update_PositionZero is a regression test for the bug
// where `if req.Position != 0` silently dropped moves to position 0 (the
// first status), breaking reorders. Position 0 must be applied.
func TestTaskStatusHandler_Update_PositionZero(t *testing.T) {
	svc := newMockTaskStatusService()
	svc.byID["s1"] = &domain.TaskStatus{ID: "s1", ProjectID: "p1", Name: "Todo", Color: "gray", Position: 2, Category: "todo"}
	h := NewTaskStatusHandler(svc, &mockAccessService{}, slog.Default())

	body := jsonBody(map[string]any{"position": 0})
	r := httptest.NewRequest(http.MethodPut, "/api/projects/p1/statuses/s1", body)
	w := httptest.NewRecorder()
	r = addChiParams(r, "id", "p1", "statusId", "s1")
	r = r.WithContext(context.WithValue(r.Context(), domain.CtxRole, "owner"))
	r = r.WithContext(context.WithValue(r.Context(), domain.CtxOrgID, "org-1"))
	h.Update(w, r)

	if w.Code != http.StatusOK {
		t.Fatalf("status = %d, want %d; body: %s", w.Code, http.StatusOK, w.Body.String())
	}
	if svc.updated == nil {
		t.Fatal("Update was not called on the service")
	}
	if svc.updated.Position != 0 {
		t.Errorf("Position = %d, want 0 (position 0 must be applicable, not treated as 'unset')", svc.updated.Position)
	}

	// The response should also report position 0.
	var resp map[string]any
	_ = json.NewDecoder(w.Body).Decode(&resp)
	if pos, _ := resp["position"].(float64); pos != 0 {
		t.Errorf("response position = %v, want 0", resp["position"])
	}
}

// TestTaskStatusHandler_Update_KeepsPositionWhenOmitted ensures a partial
// update (e.g. renaming) without a position field leaves the position alone.
func TestTaskStatusHandler_Update_KeepsPositionWhenOmitted(t *testing.T) {
	svc := newMockTaskStatusService()
	svc.byID["s1"] = &domain.TaskStatus{ID: "s1", ProjectID: "p1", Name: "Todo", Color: "gray", Position: 3, Category: "todo"}
	h := NewTaskStatusHandler(svc, &mockAccessService{}, slog.Default())

	body := jsonBody(map[string]any{"name": "Renamed"})
	r := httptest.NewRequest(http.MethodPut, "/api/projects/p1/statuses/s1", body)
	w := httptest.NewRecorder()
	r = addChiParams(r, "id", "p1", "statusId", "s1")
	r = r.WithContext(context.WithValue(r.Context(), domain.CtxRole, "owner"))
	r = r.WithContext(context.WithValue(r.Context(), domain.CtxOrgID, "org-1"))
	h.Update(w, r)

	if w.Code != http.StatusOK {
		t.Fatalf("status = %d, want %d; body: %s", w.Code, http.StatusOK, w.Body.String())
	}
	if svc.updated == nil {
		t.Fatal("Update was not called on the service")
	}
	if svc.updated.Position != 3 {
		t.Errorf("Position = %d, want 3 (omitted position must be preserved)", svc.updated.Position)
	}
	if svc.updated.Name != "Renamed" {
		t.Errorf("Name = %q, want Renamed", svc.updated.Name)
	}
}
