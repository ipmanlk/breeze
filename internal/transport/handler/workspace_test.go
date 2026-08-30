package handler

import (
	"context"
	"encoding/json"
	"errors"
	"net/http"
	"net/http/httptest"
	"testing"

	"github.com/go-chi/chi/v5"

	"ipmanlk/breeze/internal/apperr"
	"ipmanlk/breeze/internal/domain"
	"ipmanlk/breeze/internal/port"
	"log/slog"
)

// mockUserRepoForWorkspace is a minimal port.UserRepository for the workspace
// handler tests (only GetByID is exercised by resolveAccountID).
type mockUserRepoForWorkspace struct {
	port.UserRepository
	user *domain.User
	err  error
}

func (m *mockUserRepoForWorkspace) GetByID(ctx context.Context, orgID, id string) (*domain.User, error) {
	return m.user, m.err
}
func (m *mockUserRepoForWorkspace) GetByEmail(ctx context.Context, orgID, email string) (*domain.User, error) {
	return nil, errors.New("not found")
}
func (m *mockUserRepoForWorkspace) ListUsers(ctx context.Context, orgID string, filter domain.UserFilter) (*domain.UserListResult, error) {
	return &domain.UserListResult{}, nil
}
func (m *mockUserRepoForWorkspace) ListByIDs(ctx context.Context, ids []string) ([]*domain.User, error) {
	return nil, nil
}
func (m *mockUserRepoForWorkspace) ListByAccount(ctx context.Context, accountID string) ([]*domain.User, error) {
	return nil, nil
}
func (m *mockUserRepoForWorkspace) GetByOrgAndAccount(ctx context.Context, orgID, accountID string) (*domain.User, error) {
	return nil, errors.New("not found")
}
func (m *mockUserRepoForWorkspace) Create(ctx context.Context, user *domain.User) error { return nil }
func (m *mockUserRepoForWorkspace) Update(ctx context.Context, user *domain.User) error { return nil }
func (m *mockUserRepoForWorkspace) UpdateRole(ctx context.Context, orgID, id string, role domain.Role) error {
	return nil
}
func (m *mockUserRepoForWorkspace) UpdateActive(ctx context.Context, orgID, id string, active bool) error {
	return nil
}
func (m *mockUserRepoForWorkspace) UpdateProfileByAccount(ctx context.Context, accountID, name string, avatarURL *string) error {
	return nil
}
func (m *mockUserRepoForWorkspace) CountOwners(ctx context.Context, orgID string) (int, error) {
	return 0, nil
}

// mockOrgServiceForWorkspace captures the workspace-list/switch calls.
type mockOrgServiceForWorkspace struct {
	listForAccountFn  func(ctx context.Context, accountID string) ([]*domain.Workspace, error)
	createWorkspaceFn func(ctx context.Context, accountID, name, displayName, email string, avatarURL *string) (*domain.Organization, *domain.User, error)
	switchWorkspaceFn func(ctx context.Context, accountID, orgID, currentSessionID string) (*domain.Session, string, error)
	getByIDFn         func(ctx context.Context, id string) (*domain.Organization, error)
	updateFn          func(ctx context.Context, orgID, name string, messageEditWindowMinute int) (*domain.Organization, error)
	deleteFn          func(ctx context.Context, orgID string) error
}

func (m *mockOrgServiceForWorkspace) GetByID(ctx context.Context, id string) (*domain.Organization, error) {
	if m.getByIDFn != nil {
		return m.getByIDFn(ctx, id)
	}
	return &domain.Organization{ID: id, Name: "Org", Slug: "org"}, nil
}
func (m *mockOrgServiceForWorkspace) Create(ctx context.Context, name, adminName, adminEmail, adminPassword string) (*domain.Organization, *domain.User, error) {
	return nil, nil, nil
}
func (m *mockOrgServiceForWorkspace) Exists(ctx context.Context) (bool, error) {
	return false, nil
}
func (m *mockOrgServiceForWorkspace) ListForAccount(ctx context.Context, accountID string) ([]*domain.Workspace, error) {
	return m.listForAccountFn(ctx, accountID)
}
func (m *mockOrgServiceForWorkspace) CreateWorkspace(ctx context.Context, accountID, name, displayName, email string, avatarURL *string) (*domain.Organization, *domain.User, error) {
	if m.createWorkspaceFn != nil {
		return m.createWorkspaceFn(ctx, accountID, name, displayName, email, avatarURL)
	}
	return &domain.Organization{ID: "org-new", Name: name, Slug: "new"}, &domain.User{ID: "u", AccountID: accountID, Role: domain.RoleOwner}, nil
}
func (m *mockOrgServiceForWorkspace) SwitchWorkspace(ctx context.Context, accountID, orgID, currentSessionID string) (*domain.Session, string, error) {
	return m.switchWorkspaceFn(ctx, accountID, orgID, currentSessionID)
}
func (m *mockOrgServiceForWorkspace) Update(ctx context.Context, orgID, name string, messageEditWindowMinute int) (*domain.Organization, error) {
	if m.updateFn != nil {
		return m.updateFn(ctx, orgID, name, messageEditWindowMinute)
	}
	return nil, nil
}
func (m *mockOrgServiceForWorkspace) Delete(ctx context.Context, orgID, confirmName string) error {
	if m.deleteFn != nil {
		return m.deleteFn(ctx, orgID)
	}
	return nil
}

func authCtx() context.Context {
	ctx := context.WithValue(context.Background(), domain.CtxUserID, "user-1")
	ctx = context.WithValue(ctx, domain.CtxOrgID, "org-1")
	ctx = context.WithValue(ctx, domain.CtxSessionID, "sess-1")
	return ctx
}

func newCallerUserRepo() *mockUserRepoForWorkspace {
	return &mockUserRepoForWorkspace{
		user: &domain.User{ID: "user-1", AccountID: "acct-1", OrgID: "org-1", Email: "a@x.com", Name: "A", Role: domain.RoleOwner, IsActive: true},
	}
}

func TestWorkspaceHandler_List(t *testing.T) {
	orgSvc := &mockOrgServiceForWorkspace{
		listForAccountFn: func(_ context.Context, _ string) ([]*domain.Workspace, error) {
			return []*domain.Workspace{
				{Organization: domain.Organization{ID: "org-1", Name: "First", Slug: "first"}, Role: domain.RoleOwner, IsOwner: true},
			}, nil
		},
	}
	h := NewWorkspaceHandler(orgSvc, newCallerUserRepo(), slog.Default())

	w := httptest.NewRecorder()
	r := httptest.NewRequest("GET", "/api/workspaces", nil).WithContext(authCtx())
	h.List(w, r)

	if w.Code != http.StatusOK {
		t.Fatalf("status = %d, want %d; body: %s", w.Code, http.StatusOK, w.Body.String())
	}
	var list []map[string]any
	json.NewDecoder(w.Body).Decode(&list)
	if len(list) != 1 {
		t.Fatalf("len = %d, want 1", len(list))
	}
	if list[0]["name"] != "First" {
		t.Errorf("name = %v, want First", list[0]["name"])
	}
	if list[0]["is_owner"] != true {
		t.Errorf("is_owner = %v, want true", list[0]["is_owner"])
	}
}

func TestWorkspaceHandler_Create(t *testing.T) {
	orgSvc := &mockOrgServiceForWorkspace{}
	h := NewWorkspaceHandler(orgSvc, newCallerUserRepo(), slog.Default())

	body := jsonBody(map[string]string{"name": "Acme"})
	w := httptest.NewRecorder()
	r := httptest.NewRequest("POST", "/api/workspaces", body).WithContext(authCtx())
	r.Header.Set("Content-Type", "application/json")
	h.Create(w, r)

	if w.Code != http.StatusCreated {
		t.Fatalf("status = %d, want %d; body: %s", w.Code, http.StatusCreated, w.Body.String())
	}
	var resp map[string]any
	json.NewDecoder(w.Body).Decode(&resp)
	if resp["name"] != "Acme" {
		t.Errorf("name = %v, want Acme", resp["name"])
	}
	if resp["is_owner"] != true {
		t.Errorf("is_owner = %v, want true", resp["is_owner"])
	}
}

func TestWorkspaceHandler_Switch_NotMember(t *testing.T) {
	orgSvc := &mockOrgServiceForWorkspace{
		switchWorkspaceFn: func(_ context.Context, _, _, _ string) (*domain.Session, string, error) {
			return nil, "", apperr.ErrNotFound
		},
		listForAccountFn: func(_ context.Context, _ string) ([]*domain.Workspace, error) {
			return nil, nil
		},
	}
	h := NewWorkspaceHandler(orgSvc, newCallerUserRepo(), slog.Default())

	w := httptest.NewRecorder()
	rctx := chi.NewRouteContext()
	rctx.URLParams.Add("id", "org-x")
	r := httptest.NewRequest("POST", "/api/workspaces/org-x/switch", nil).
		WithContext(context.WithValue(authCtx(), chi.RouteCtxKey, rctx))
	h.Switch(w, r)

	if w.Code != http.StatusNotFound {
		t.Errorf("status = %d, want %d; body: %s", w.Code, http.StatusNotFound, w.Body.String())
	}
}

func TestWorkspaceHandler_Switch_Success(t *testing.T) {
	orgSvc := &mockOrgServiceForWorkspace{
		switchWorkspaceFn: func(_ context.Context, _, orgID, _ string) (*domain.Session, string, error) {
			return &domain.Session{ID: "sess-2", UserID: "user-1", OrgID: orgID, Role: domain.RoleOwner}, "new-token", nil
		},
		listForAccountFn: func(_ context.Context, _ string) ([]*domain.Workspace, error) {
			return []*domain.Workspace{{Organization: domain.Organization{ID: "org-2", Name: "Second"}, Role: domain.RoleOwner, IsOwner: true}}, nil
		},
	}
	h := NewWorkspaceHandler(orgSvc, newCallerUserRepo(), slog.Default())

	w := httptest.NewRecorder()
	rctx := chi.NewRouteContext()
	rctx.URLParams.Add("id", "org-2")
	r := httptest.NewRequest("POST", "/api/workspaces/org-2/switch", nil).
		WithContext(context.WithValue(authCtx(), chi.RouteCtxKey, rctx))
	h.Switch(w, r)

	if w.Code != http.StatusOK {
		t.Fatalf("status = %d, want %d; body: %s", w.Code, http.StatusOK, w.Body.String())
	}
	// Cookie must be (re)set with the new token.
	var found bool
	for _, c := range w.Result().Cookies() {
		if c.Name == "__Host-token" && c.Value == "new-token" {
			found = true
		}
	}
	if !found {
		t.Error("token cookie was not set")
	}
}

func (m *mockUserRepoForWorkspace) RunInTransaction(ctx context.Context, fn func(port.UserRepository) error) error {
	return fn(m)
}
