package handler

import (
	"context"
	"encoding/json"
	"log/slog"
	"net/http"
	"net/http/httptest"
	"testing"

	"github.com/go-chi/chi/v5"
	"ipmanlk/plume/internal/apperr"
	"ipmanlk/plume/internal/domain"
)

type mockViewService struct {
	viewsByID    map[string]*domain.View
	globalViews  map[string]*domain.View
	projectViews map[string]map[string]*domain.View // projectID -> viewID -> view
	pins         map[string]map[string]bool         // userID -> viewID -> bool
}

func newMockViewService() *mockViewService {
	return &mockViewService{
		viewsByID:    make(map[string]*domain.View),
		globalViews:  make(map[string]*domain.View),
		projectViews: make(map[string]map[string]*domain.View),
		pins:         make(map[string]map[string]bool),
	}
}

func (m *mockViewService) Create(ctx context.Context, params domain.CreateViewParams) (*domain.View, error) {
	v := &domain.View{
		ID:        "v-new",
		OrgID:     params.OrgID,
		ProjectID: params.ProjectID,
		CreatedBy: params.CreatedBy,
		Name:      params.Name,
		Layout:    params.Layout,
		Filters:   params.Filters,
	}
	m.viewsByID[v.ID] = v
	if v.ProjectID == nil {
		m.globalViews[v.ID] = v
	} else {
		if m.projectViews[*v.ProjectID] == nil {
			m.projectViews[*v.ProjectID] = make(map[string]*domain.View)
		}
		m.projectViews[*v.ProjectID][v.ID] = v
	}
	return v, nil
}

func (m *mockViewService) Update(ctx context.Context, userID string, params domain.UpdateViewParams) (*domain.View, error) {
	v, ok := m.viewsByID[params.ID]
	if !ok || v.OrgID != params.OrgID {
		return nil, apperr.ErrNotFound
	}
	if params.Name != nil {
		v.Name = *params.Name
	}
	if params.Layout != nil {
		v.Layout = *params.Layout
	}
	if params.Filters != nil {
		v.Filters = *params.Filters
	}
	return v, nil
}

func (m *mockViewService) Delete(ctx context.Context, userID, orgID, id string) error {
	v, ok := m.viewsByID[id]
	if !ok || v.OrgID != orgID {
		return apperr.ErrNotFound
	}
	delete(m.viewsByID, id)
	return nil
}

func (m *mockViewService) GetByID(ctx context.Context, orgID, id string) (*domain.View, error) {
	v, ok := m.viewsByID[id]
	if !ok || v.OrgID != orgID {
		return nil, apperr.ErrNotFound
	}
	return v, nil
}

func (m *mockViewService) ListByProject(ctx context.Context, orgID, projectID string) ([]*domain.View, error) {
	var views []*domain.View
	for _, v := range m.projectViews[projectID] {
		if v.OrgID == orgID {
			views = append(views, v)
		}
	}
	return views, nil
}

func (m *mockViewService) ListGlobal(ctx context.Context, orgID string) ([]*domain.View, error) {
	var views []*domain.View
	for _, v := range m.globalViews {
		if v.OrgID == orgID {
			views = append(views, v)
		}
	}
	return views, nil
}

func (m *mockViewService) ListPinned(ctx context.Context, userID string) ([]*domain.View, error) {
	var views []*domain.View
	for vid := range m.pins[userID] {
		if v, ok := m.viewsByID[vid]; ok {
			views = append(views, v)
		}
	}
	return views, nil
}

func (m *mockViewService) Pin(ctx context.Context, orgID, viewID, userID string) error {
	if m.pins[userID] == nil {
		m.pins[userID] = make(map[string]bool)
	}
	m.pins[userID][viewID] = true
	return nil
}

func (m *mockViewService) Unpin(ctx context.Context, orgID, viewID, userID string) error {
	if m.pins[userID] != nil {
		delete(m.pins[userID], viewID)
	}
	return nil
}

func TestViewHandler_Create(t *testing.T) {
	svc := newMockViewService()
	h := NewViewHandler(svc, &mockProjectRepo{}, &mockAccessService{}, slog.Default())

	body := jsonBody(map[string]any{
		"name":    "All Tasks",
		"layout":  "board",
		"filters": map[string]string{"search": "bug"},
	})
	ctx := context.WithValue(context.Background(), domain.CtxOrgID, "org-1")
	ctx = context.WithValue(ctx, domain.CtxUserID, "user-1")
	r := httptest.NewRequest("POST", "/api/views", body).WithContext(ctx)
	w := httptest.NewRecorder()
	h.Create(w, r)

	if w.Code != http.StatusCreated {
		t.Fatalf("status = %d, want %d; body: %s", w.Code, http.StatusCreated, w.Body.String())
	}
	var resp map[string]any
	json.NewDecoder(w.Body).Decode(&resp)
	if resp["name"] != "All Tasks" {
		t.Errorf("name = %v, want All Tasks", resp["name"])
	}
	if resp["layout"] != "board" {
		t.Errorf("layout = %v, want board", resp["layout"])
	}
	filters, _ := resp["filters"].(map[string]any)
	if filters["search"] != "bug" {
		t.Errorf("filters.search = %v, want bug", filters["search"])
	}
}

func TestViewHandler_Create_Validation(t *testing.T) {
	svc := newMockViewService()
	h := NewViewHandler(svc, &mockProjectRepo{}, &mockAccessService{}, slog.Default())

	body := jsonBody(map[string]string{"name": ""})
	ctx := context.WithValue(context.Background(), domain.CtxOrgID, "org-1")
	ctx = context.WithValue(ctx, domain.CtxUserID, "user-1")
	r := httptest.NewRequest("POST", "/api/views", body).WithContext(ctx)
	w := httptest.NewRecorder()
	h.Create(w, r)

	if w.Code != http.StatusBadRequest {
		t.Errorf("status = %d, want %d", w.Code, http.StatusBadRequest)
	}
}

func TestViewHandler_Get(t *testing.T) {
	svc := newMockViewService()
	svc.viewsByID["v1"] = &domain.View{ID: "v1", OrgID: "org-1", Name: "Board", Layout: domain.ViewLayoutBoard}
	h := NewViewHandler(svc, &mockProjectRepo{}, &mockAccessService{}, slog.Default())

	ctx := context.WithValue(context.Background(), domain.CtxOrgID, "org-1")
	r := httptest.NewRequest("GET", "/api/views/v1", nil).WithContext(ctx)
	w := httptest.NewRecorder()
	r = addChiParam(r, "id", "v1")
	h.Get(w, r)

	if w.Code != http.StatusOK {
		t.Fatalf("status = %d, want %d", w.Code, http.StatusOK)
	}
	var resp map[string]any
	json.NewDecoder(w.Body).Decode(&resp)
	if resp["name"] != "Board" {
		t.Errorf("name = %v, want Board", resp["name"])
	}
}

func TestViewHandler_Get_NotFound(t *testing.T) {
	svc := newMockViewService()
	h := NewViewHandler(svc, &mockProjectRepo{}, &mockAccessService{}, slog.Default())

	ctx := context.WithValue(context.Background(), domain.CtxOrgID, "org-1")
	r := httptest.NewRequest("GET", "/api/views/missing", nil).WithContext(ctx)
	w := httptest.NewRecorder()
	r = addChiParam(r, "id", "missing")
	h.Get(w, r)

	if w.Code != http.StatusNotFound {
		t.Errorf("status = %d, want %d", w.Code, http.StatusNotFound)
	}
}

func TestViewHandler_Update(t *testing.T) {
	svc := newMockViewService()
	svc.viewsByID["v1"] = &domain.View{ID: "v1", OrgID: "org-1", Name: "Board", Layout: domain.ViewLayoutBoard, Filters: domain.ViewFilters{}}
	h := NewViewHandler(svc, &mockProjectRepo{}, &mockAccessService{}, slog.Default())

	body := jsonBody(map[string]any{
		"name":    "Updated",
		"layout":  "list",
		"filters": map[string]string{"priority": "urgent"},
	})
	ctx := context.WithValue(context.Background(), domain.CtxOrgID, "org-1")
	r := httptest.NewRequest("PATCH", "/api/views/v1", body).WithContext(ctx)
	w := httptest.NewRecorder()
	r = addChiParam(r, "id", "v1")
	h.Update(w, r)

	if w.Code != http.StatusOK {
		t.Fatalf("status = %d, want %d; body: %s", w.Code, http.StatusOK, w.Body.String())
	}
	var resp map[string]any
	json.NewDecoder(w.Body).Decode(&resp)
	if resp["name"] != "Updated" {
		t.Errorf("name = %v, want Updated", resp["name"])
	}
	if resp["layout"] != "list" {
		t.Errorf("layout = %v, want list", resp["layout"])
	}
	filters, _ := resp["filters"].(map[string]any)
	if filters["priority"] != "urgent" {
		t.Errorf("filters.priority = %v, want urgent", filters["priority"])
	}
}

func TestViewHandler_Update_NotFound(t *testing.T) {
	svc := newMockViewService()
	h := NewViewHandler(svc, &mockProjectRepo{}, &mockAccessService{}, slog.Default())

	body := jsonBody(map[string]any{"name": "Updated"})
	ctx := context.WithValue(context.Background(), domain.CtxOrgID, "org-1")
	r := httptest.NewRequest("PATCH", "/api/views/missing", body).WithContext(ctx)
	w := httptest.NewRecorder()
	r = addChiParam(r, "id", "missing")
	h.Update(w, r)

	if w.Code != http.StatusNotFound {
		t.Errorf("status = %d, want %d", w.Code, http.StatusNotFound)
	}
}

func TestViewHandler_Delete(t *testing.T) {
	svc := newMockViewService()
	svc.viewsByID["v1"] = &domain.View{ID: "v1", OrgID: "org-1", Name: "Custom", Layout: domain.ViewLayoutBoard}
	h := NewViewHandler(svc, &mockProjectRepo{}, &mockAccessService{}, slog.Default())

	ctx := context.WithValue(context.Background(), domain.CtxOrgID, "org-1")
	r := httptest.NewRequest("DELETE", "/api/views/v1", nil).WithContext(ctx)
	w := httptest.NewRecorder()
	r = addChiParam(r, "id", "v1")
	h.Delete(w, r)

	if w.Code != http.StatusNoContent {
		t.Errorf("status = %d, want %d", w.Code, http.StatusNoContent)
	}
	if _, ok := svc.viewsByID["v1"]; ok {
		t.Error("expected view to be deleted")
	}
}

func TestViewHandler_Delete_NotFound(t *testing.T) {
	svc := newMockViewService()
	h := NewViewHandler(svc, &mockProjectRepo{}, &mockAccessService{}, slog.Default())

	ctx := context.WithValue(context.Background(), domain.CtxOrgID, "org-1")
	r := httptest.NewRequest("DELETE", "/api/views/missing", nil).WithContext(ctx)
	w := httptest.NewRecorder()
	r = addChiParam(r, "id", "missing")
	h.Delete(w, r)

	if w.Code != http.StatusNotFound {
		t.Errorf("status = %d, want %d", w.Code, http.StatusNotFound)
	}
}

func TestViewHandler_Pin(t *testing.T) {
	svc := newMockViewService()
	svc.viewsByID["v1"] = &domain.View{ID: "v1", OrgID: "org-1", Name: "Board", Layout: domain.ViewLayoutBoard}
	h := NewViewHandler(svc, &mockProjectRepo{}, &mockAccessService{}, slog.Default())

	ctx := context.WithValue(context.Background(), domain.CtxUserID, "user-1")
	r := httptest.NewRequest("POST", "/api/views/v1/pin", nil).WithContext(ctx)
	w := httptest.NewRecorder()
	r = addChiParam(r, "id", "v1")
	h.Pin(w, r)

	if w.Code != http.StatusNoContent {
		t.Errorf("status = %d, want %d", w.Code, http.StatusNoContent)
	}
	if !svc.pins["user-1"]["v1"] {
		t.Error("expected view to be pinned")
	}
}

func TestViewHandler_Unpin(t *testing.T) {
	svc := newMockViewService()
	svc.pins["user-1"] = map[string]bool{"v1": true}
	h := NewViewHandler(svc, &mockProjectRepo{}, &mockAccessService{}, slog.Default())

	ctx := context.WithValue(context.Background(), domain.CtxUserID, "user-1")
	r := httptest.NewRequest("DELETE", "/api/views/v1/pin", nil).WithContext(ctx)
	w := httptest.NewRecorder()
	r = addChiParam(r, "id", "v1")
	h.Unpin(w, r)

	if w.Code != http.StatusNoContent {
		t.Errorf("status = %d, want %d", w.Code, http.StatusNoContent)
	}
	if svc.pins["user-1"]["v1"] {
		t.Error("expected view to be unpinned")
	}
}

func TestViewHandler_ListGlobal(t *testing.T) {
	svc := newMockViewService()
	svc.globalViews["v1"] = &domain.View{ID: "v1", OrgID: "org-1", Name: "Global Board", Layout: domain.ViewLayoutBoard}
	h := NewViewHandler(svc, &mockProjectRepo{}, &mockAccessService{}, slog.Default())

	ctx := context.WithValue(context.Background(), domain.CtxOrgID, "org-1")
	r := httptest.NewRequest("GET", "/api/views", nil).WithContext(ctx)
	w := httptest.NewRecorder()
	h.ListGlobal(w, r)

	if w.Code != http.StatusOK {
		t.Fatalf("status = %d, want %d", w.Code, http.StatusOK)
	}
	var resp []map[string]any
	json.NewDecoder(w.Body).Decode(&resp)
	if len(resp) != 1 {
		t.Errorf("got %d items, want 1", len(resp))
	}
}

func TestViewHandler_ListPinned(t *testing.T) {
	svc := newMockViewService()
	svc.viewsByID["v1"] = &domain.View{ID: "v1", OrgID: "org-1", Name: "Board", Layout: domain.ViewLayoutBoard}
	svc.pins["user-1"] = map[string]bool{"v1": true}
	h := NewViewHandler(svc, &mockProjectRepo{}, &mockAccessService{}, slog.Default())

	ctx := context.WithValue(context.Background(), domain.CtxUserID, "user-1")
	r := httptest.NewRequest("GET", "/api/views/pins", nil).WithContext(ctx)
	w := httptest.NewRecorder()
	h.ListPinned(w, r)

	if w.Code != http.StatusOK {
		t.Fatalf("status = %d, want %d", w.Code, http.StatusOK)
	}
	var resp []map[string]any
	json.NewDecoder(w.Body).Decode(&resp)
	if len(resp) != 1 {
		t.Errorf("got %d items, want 1", len(resp))
	}
}

func TestViewHandler_ListByProject(t *testing.T) {
	svc := newMockViewService()
	pid := "proj-1"
	svc.projectViews[pid] = map[string]*domain.View{
		"v1": {ID: "v1", OrgID: "org-1", ProjectID: &pid, Name: "Board", Layout: domain.ViewLayoutBoard},
	}
	h := NewViewHandler(svc, &mockProjectRepo{}, &mockAccessService{}, slog.Default())

	ctx := context.WithValue(context.Background(), domain.CtxOrgID, "org-1")
	ctx = context.WithValue(ctx, domain.CtxRole, "owner")
	r := httptest.NewRequest("GET", "/api/projects/proj-1/views", nil).WithContext(ctx)
	w := httptest.NewRecorder()
	r = addChiParam(r, "id", "proj-1")
	h.ListByProject(w, r)

	if w.Code != http.StatusOK {
		t.Fatalf("status = %d, want %d", w.Code, http.StatusOK)
	}
	var resp []map[string]any
	json.NewDecoder(w.Body).Decode(&resp)
	if len(resp) != 1 {
		t.Errorf("got %d items, want 1", len(resp))
	}
}

// addChiParam injects chi URL parameters into the request context.
func addChiParam(r *http.Request, key, value string) *http.Request {
	rctx := chi.NewRouteContext()
	rctx.URLParams.Add(key, value)
	return r.WithContext(context.WithValue(r.Context(), chi.RouteCtxKey, rctx))
}

func addChiParams(r *http.Request, keysAndValues ...string) *http.Request {
	rctx := chi.NewRouteContext()
	for i := 0; i+1 < len(keysAndValues); i += 2 {
		rctx.URLParams.Add(keysAndValues[i], keysAndValues[i+1])
	}
	return r.WithContext(context.WithValue(r.Context(), chi.RouteCtxKey, rctx))
}
