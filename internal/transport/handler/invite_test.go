package handler

import (
	"context"
	"encoding/json"
	"log/slog"
	"net/http"
	"net/http/httptest"
	"testing"
	"time"

	"ipmanlk/plume/internal/apperr"
	"ipmanlk/plume/internal/domain"

	"github.com/go-chi/chi/v5"
)

// --- mocks ---

type mockInviteService struct {
	createFn   func(ctx context.Context, params domain.CreateInviteParams, callerRole domain.Role) (*domain.UserInvite, string, error)
	listFn     func(ctx context.Context, orgID string, limit int) ([]*domain.UserInvite, error)
	revokeFn   func(ctx context.Context, orgID, id string) error
	validateFn func(ctx context.Context, token string) (*domain.UserInvite, error)
	acceptFn   func(ctx context.Context, params domain.AcceptInviteParams) (*domain.User, string, error)
}

func (m *mockInviteService) Create(ctx context.Context, params domain.CreateInviteParams, callerRole domain.Role) (*domain.UserInvite, string, error) {
	return m.createFn(ctx, params, callerRole)
}
func (m *mockInviteService) List(ctx context.Context, orgID string, limit int) ([]*domain.UserInvite, error) {
	return m.listFn(ctx, orgID, limit)
}
func (m *mockInviteService) Revoke(ctx context.Context, orgID, id string) error {
	return m.revokeFn(ctx, orgID, id)
}
func (m *mockInviteService) Validate(ctx context.Context, token string) (*domain.UserInvite, error) {
	return m.validateFn(ctx, token)
}
func (m *mockInviteService) Accept(ctx context.Context, params domain.AcceptInviteParams) (*domain.User, string, error) {
	return m.acceptFn(ctx, params)
}

// --- tests ---

func TestInviteHandler_Create_Success(t *testing.T) {
	svc := &mockInviteService{
		createFn: func(ctx context.Context, params domain.CreateInviteParams, callerRole domain.Role) (*domain.UserInvite, string, error) {
			return &domain.UserInvite{ID: "inv-1", OrgID: "org-1", Role: domain.RoleMember, ExpiresAt: time.Now().UTC().Add(7 * 24 * time.Hour)}, "raw-token", nil
		},
	}
	h := NewInviteHandler(svc, noopAuditService{}, slog.Default())

	body := jsonBody(map[string]string{"role": "member"})
	req := httptest.NewRequest(http.MethodPost, "/api/invites", body)
	req = req.WithContext(ctxWithRole("org-1", "u1", "owner"))
	req.Header.Set("Content-Type", "application/json")
	rr := httptest.NewRecorder()

	h.Create(rr, req)
	if rr.Code != http.StatusCreated {
		t.Fatalf("expected 201, got %d: %s", rr.Code, rr.Body.String())
	}
	var resp map[string]any
	json.NewDecoder(rr.Body).Decode(&resp)
	if resp["token"] == "" {
		t.Error("expected token in response")
	}
}

func TestInviteHandler_Create_Forbidden(t *testing.T) {
	svc := &mockInviteService{
		createFn: func(ctx context.Context, params domain.CreateInviteParams, callerRole domain.Role) (*domain.UserInvite, string, error) {
			return nil, "", apperr.ErrForbidden
		},
	}
	h := NewInviteHandler(svc, noopAuditService{}, slog.Default())

	body := jsonBody(map[string]string{"role": "admin"})
	req := httptest.NewRequest(http.MethodPost, "/api/invites", body)
	req = req.WithContext(ctxWithRole("org-1", "u1", "admin"))
	req.Header.Set("Content-Type", "application/json")
	rr := httptest.NewRecorder()

	h.Create(rr, req)
	if rr.Code != http.StatusForbidden {
		t.Errorf("expected 403, got %d", rr.Code)
	}
}

func TestInviteHandler_Validate_Success(t *testing.T) {
	svc := &mockInviteService{
		validateFn: func(ctx context.Context, token string) (*domain.UserInvite, error) {
			return &domain.UserInvite{ID: "inv-1", Role: domain.RoleMember, ExpiresAt: time.Now().UTC().Add(24 * time.Hour)}, nil
		},
	}
	h := NewInviteHandler(svc, noopAuditService{}, slog.Default())

	req := httptest.NewRequest(http.MethodGet, "/api/invites/abc/validate", nil)
	req = withChiParam(req, "token", "abc")
	rr := httptest.NewRecorder()

	h.Validate(rr, req)
	if rr.Code != http.StatusOK {
		t.Fatalf("expected 200, got %d: %s", rr.Code, rr.Body.String())
	}
}

func TestInviteHandler_Validate_Expired(t *testing.T) {
	svc := &mockInviteService{
		validateFn: func(ctx context.Context, token string) (*domain.UserInvite, error) {
			return nil, apperr.ErrSessionExpired
		},
	}
	h := NewInviteHandler(svc, noopAuditService{}, slog.Default())

	req := httptest.NewRequest(http.MethodGet, "/api/invites/abc/validate", nil)
	req = withChiParam(req, "token", "abc")
	rr := httptest.NewRecorder()

	h.Validate(rr, req)
	if rr.Code != http.StatusNotFound {
		t.Errorf("expected 404, got %d", rr.Code)
	}
}

func TestInviteHandler_Accept_Success(t *testing.T) {
	svc := &mockInviteService{
		acceptFn: func(ctx context.Context, params domain.AcceptInviteParams) (*domain.User, string, error) {
			return &domain.User{ID: "u-new", Name: "Jane", Email: "jane@test.com", Role: domain.RoleMember, IsActive: true}, "", nil
		},
	}
	h := NewInviteHandler(svc, noopAuditService{}, slog.Default())

	body := jsonBody(map[string]any{"name": "Jane", "email": "jane@test.com", "password": "password123"})
	req := httptest.NewRequest(http.MethodPost, "/api/invites/abc/accept", body)
	req = withChiParam(req, "token", "abc")
	req.Header.Set("Content-Type", "application/json")
	rr := httptest.NewRecorder()

	h.Accept(rr, req)
	if rr.Code != http.StatusCreated {
		t.Fatalf("expected 201, got %d: %s", rr.Code, rr.Body.String())
	}
	var resp map[string]any
	json.NewDecoder(rr.Body).Decode(&resp)
	if resp["email"] != "jane@test.com" {
		t.Errorf("email = %v, want jane@test.com", resp["email"])
	}
}

func TestInviteHandler_Accept_AlreadyExists(t *testing.T) {
	svc := &mockInviteService{
		acceptFn: func(ctx context.Context, params domain.AcceptInviteParams) (*domain.User, string, error) {
			return nil, "", apperr.ErrAlreadyExists
		},
	}
	h := NewInviteHandler(svc, noopAuditService{}, slog.Default())

	body := jsonBody(map[string]any{"name": "Jane", "email": "jane@test.com", "password": "password123"})
	req := httptest.NewRequest(http.MethodPost, "/api/invites/abc/accept", body)
	req = withChiParam(req, "token", "abc")
	req.Header.Set("Content-Type", "application/json")
	rr := httptest.NewRecorder()

	h.Accept(rr, req)
	if rr.Code != http.StatusConflict {
		t.Errorf("expected 409, got %d", rr.Code)
	}
}

func TestUserHandler_Get_Success(t *testing.T) {
	svc := &mockUserService{
		getByIDFn: func(ctx context.Context, _, id string) (*domain.User, error) {
			return &domain.User{ID: "u1", Name: "Alice", Email: "a@test.com", Role: domain.RoleAdmin, IsActive: true}, nil
		},
	}
	h := NewUserHandler(svc, &mockProjectMemberService{}, noopAuditService{}, slog.Default())

	req := httptest.NewRequest(http.MethodGet, "/api/users/u1", nil)
	req = req.WithContext(ctxWithRole("org-1", "u1", "member"))
	req = withChiParam(req, "id", "u1")
	rr := httptest.NewRecorder()

	h.Get(rr, req)
	if rr.Code != http.StatusOK {
		t.Fatalf("expected 200, got %d: %s", rr.Code, rr.Body.String())
	}
	var resp map[string]any
	json.NewDecoder(rr.Body).Decode(&resp)
	if resp["name"] != "Alice" {
		t.Errorf("name = %v, want Alice", resp["name"])
	}
}

func TestUserHandler_UpdateRole_Success(t *testing.T) {
	svc := &mockUserService{
		getByIDFn: func(ctx context.Context, _, id string) (*domain.User, error) {
			return &domain.User{ID: "u2", Name: "Bob", Email: "b@test.com", Role: domain.RoleAdmin, IsActive: true}, nil
		},
	}
	h := NewUserHandler(svc, &mockProjectMemberService{}, noopAuditService{}, slog.Default())

	body := jsonBody(map[string]string{"role": "admin"})
	req := httptest.NewRequest(http.MethodPut, "/api/users/u2/role", body)
	req = withChiParam(req, "id", "u2")
	req = req.WithContext(ctxWithRole("org-1", "u1", "owner"))
	req.Header.Set("Content-Type", "application/json")
	rr := httptest.NewRecorder()

	h.UpdateRole(rr, req)
	if rr.Code != http.StatusOK {
		t.Fatalf("expected 200, got %d: %s", rr.Code, rr.Body.String())
	}
}

func TestUserHandler_UpdateActive_Success(t *testing.T) {
	svc := &mockUserService{
		getByIDFn: func(ctx context.Context, _, id string) (*domain.User, error) {
			return &domain.User{ID: "u2", Name: "Bob", Email: "b@test.com", Role: domain.RoleMember, IsActive: false}, nil
		},
	}
	h := NewUserHandler(svc, &mockProjectMemberService{}, noopAuditService{}, slog.Default())

	body := jsonBody(map[string]bool{"is_active": false})
	req := httptest.NewRequest(http.MethodPut, "/api/users/u2/active", body)
	req = withChiParam(req, "id", "u2")
	req = req.WithContext(ctxWithRole("org-1", "u1", "admin"))
	req.Header.Set("Content-Type", "application/json")
	rr := httptest.NewRecorder()

	h.UpdateActive(rr, req)
	if rr.Code != http.StatusOK {
		t.Fatalf("expected 200, got %d: %s", rr.Code, rr.Body.String())
	}
	var resp map[string]any
	json.NewDecoder(rr.Body).Decode(&resp)
	if resp["is_active"] != false {
		t.Errorf("is_active = %v, want false", resp["is_active"])
	}
}

func TestUserHandler_List_WithFilters(t *testing.T) {
	svc := &mockUserService{
		listUsersFn: func(ctx context.Context, orgID string, filter domain.UserFilter) (*domain.UserListResult, error) {
			if filter.Role != "admin" {
				t.Errorf("Role filter = %q, want admin", filter.Role)
			}
			if filter.IncludeInactive {
				t.Error("IncludeInactive should be false")
			}
			return &domain.UserListResult{Users: []*domain.User{
				{ID: "u1", Name: "Alice", Role: domain.RoleAdmin},
			}}, nil
		},
	}
	h := NewUserHandler(svc, &mockProjectMemberService{}, noopAuditService{}, slog.Default())

	req := httptest.NewRequest(http.MethodGet, "/api/users?role=admin", nil)
	req = req.WithContext(ctxWithRole("org-1", "u1", "member"))
	rr := httptest.NewRecorder()

	h.List(rr, req)
	if rr.Code != http.StatusOK {
		t.Fatalf("expected 200, got %d: %s", rr.Code, rr.Body.String())
	}
	var resp map[string]any
	json.NewDecoder(rr.Body).Decode(&resp)
	items := resp["items"].([]any)
	if len(items) != 1 {
		t.Errorf("len(items) = %d, want 1", len(items))
	}
}

func TestUserHandler_ListProjectMemberships_Success(t *testing.T) {
	pmSvc := &mockProjectMemberService{
		listByUserFn: func(ctx context.Context, orgID, userID string) ([]*domain.UserProjectMembership, error) {
			return []*domain.UserProjectMembership{
				{ProjectID: "proj-1", Name: "Project A", Color: "#ff0000", Role: domain.RoleMember},
			}, nil
		},
	}
	h := NewUserHandler(&mockUserService{}, pmSvc, noopAuditService{}, slog.Default())

	req := httptest.NewRequest(http.MethodGet, "/api/users/u1/project-memberships", nil)
	req = withChiParam(req, "id", "u1")
	req = req.WithContext(ctxWithRole("org-1", "u1", "admin"))
	rr := httptest.NewRecorder()

	h.ListProjectMemberships(rr, req)
	if rr.Code != http.StatusOK {
		t.Fatalf("expected 200, got %d: %s", rr.Code, rr.Body.String())
	}

	var resp []map[string]any
	json.NewDecoder(rr.Body).Decode(&resp)
	if len(resp) != 1 {
		t.Fatalf("expected 1 membership, got %d", len(resp))
	}
	if resp[0]["project_id"] != "proj-1" {
		t.Errorf("project_id = %v, want proj-1", resp[0]["project_id"])
	}
}

func TestUserHandler_UpdateProjectMemberships_Success(t *testing.T) {
	pmSvc := &mockProjectMemberService{
		setMembershipsFn: func(ctx context.Context, orgID, userID string, assignments []domain.ProjectAssignment) error {
			if len(assignments) != 1 {
				t.Fatalf("expected 1 assignment, got %d", len(assignments))
			}
			if assignments[0].ProjectID != "proj-1" {
				t.Errorf("ProjectID = %s, want proj-1", assignments[0].ProjectID)
			}
			if assignments[0].Role != domain.RoleMember {
				t.Errorf("Role = %s, want member", assignments[0].Role)
			}
			return nil
		},
	}
	h := NewUserHandler(&mockUserService{}, pmSvc, noopAuditService{}, slog.Default())

	body := jsonBody(map[string]any{
		"assignments": []map[string]any{
			{"project_id": "proj-1", "role": "member"},
		},
	})
	req := httptest.NewRequest(http.MethodPut, "/api/users/u1/project-memberships", body)
	req = withChiParam(req, "id", "u1")
	req = req.WithContext(ctxWithRole("org-1", "u1", "admin"))
	req.Header.Set("Content-Type", "application/json")
	rr := httptest.NewRecorder()

	h.UpdateProjectMemberships(rr, req)
	if rr.Code != http.StatusOK {
		t.Fatalf("expected 200, got %d: %s", rr.Code, rr.Body.String())
	}
}

func TestUserHandler_UpdateProjectMemberships_ValidationError(t *testing.T) {
	pmSvc := &mockProjectMemberService{}
	h := NewUserHandler(&mockUserService{}, pmSvc, noopAuditService{}, slog.Default())

	// Empty body should trigger validation error
	body := jsonBody(map[string]any{})
	req := httptest.NewRequest(http.MethodPut, "/api/users/u1/project-memberships", body)
	req = withChiParam(req, "id", "u1")
	req = req.WithContext(ctxWithRole("org-1", "u1", "admin"))
	req.Header.Set("Content-Type", "application/json")
	rr := httptest.NewRecorder()

	h.UpdateProjectMemberships(rr, req)
	if rr.Code != http.StatusBadRequest {
		t.Fatalf("expected 400, got %d: %s", rr.Code, rr.Body.String())
	}
}

// --- helpers ---

func ctxWithRole(orgID, userID, role string) context.Context {
	ctx := context.Background()
	ctx = context.WithValue(ctx, domain.CtxOrgID, orgID)
	ctx = context.WithValue(ctx, domain.CtxUserID, userID)
	ctx = context.WithValue(ctx, domain.CtxRole, role)
	return ctx
}

func withChiParam(r *http.Request, key, value string) *http.Request {
	rctx := chi.NewRouteContext()
	rctx.URLParams.Add(key, value)
	return r.WithContext(context.WithValue(r.Context(), chi.RouteCtxKey, rctx))
}
