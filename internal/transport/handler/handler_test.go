package handler

import (
	"bytes"
	"context"
	"encoding/json"
	"errors"
	"io"
	"log/slog"
	"net/http"
	"net/http/httptest"
	"strings"
	"testing"

	"ipmanlk/plume/internal/apperr"
	"ipmanlk/plume/internal/domain"
	"ipmanlk/plume/internal/port"
)

// mockAccessService is a test double for port.AccessService. By default it
// allows every access check (returns nil). Tests that need to assert denial
// can override the function fields. It lets handler unit tests avoid
// importing the service package while still exercising the access-guard
// call sites.
type mockAccessService struct {
	ensureProjectAccessFn            func(ctx context.Context, orgID, userID string, role domain.Role, projectID string) error
	resolveProjectEffectiveRoleFn    func(ctx context.Context, orgID, userID string, role domain.Role, projectID string) (domain.Role, error)
	ensureConversationAccessFn       func(ctx context.Context, orgID, userID string, role domain.Role, convID string) error
	ensureConversationSendAccessFn   func(ctx context.Context, orgID, userID string, role domain.Role, convID string) error
	ensureConversationManageAccessFn func(ctx context.Context, orgID, userID string, role domain.Role, convID string) error
}

var _ port.AccessService = (*mockAccessService)(nil)

func (m *mockAccessService) EnsureProjectAccess(ctx context.Context, orgID, userID string, role domain.Role, projectID string) error {
	if m.ensureProjectAccessFn != nil {
		return m.ensureProjectAccessFn(ctx, orgID, userID, role, projectID)
	}
	return nil
}
func (m *mockAccessService) ResolveProjectEffectiveRole(ctx context.Context, orgID, userID string, role domain.Role, projectID string) (domain.Role, error) {
	if m.resolveProjectEffectiveRoleFn != nil {
		return m.resolveProjectEffectiveRoleFn(ctx, orgID, userID, role, projectID)
	}
	return role, nil
}
func (m *mockAccessService) EnsureConversationAccess(ctx context.Context, orgID, userID string, role domain.Role, convID string) error {
	if m.ensureConversationAccessFn != nil {
		return m.ensureConversationAccessFn(ctx, orgID, userID, role, convID)
	}
	return nil
}
func (m *mockAccessService) EnsureConversationSendAccess(ctx context.Context, orgID, userID string, role domain.Role, convID string) error {
	if m.ensureConversationSendAccessFn != nil {
		return m.ensureConversationSendAccessFn(ctx, orgID, userID, role, convID)
	}
	return nil
}
func (m *mockAccessService) EnsureConversationManageAccess(ctx context.Context, orgID, userID string, role domain.Role, convID string) error {
	if m.ensureConversationManageAccessFn != nil {
		return m.ensureConversationManageAccessFn(ctx, orgID, userID, role, convID)
	}
	return nil
}

// accessSvcFromPerm builds an AccessService whose conversation checks
// delegate to the given ChannelPermissionService mock, mirroring the
// production wiring. Project checks default to allow.
func accessSvcFromPerm(perm port.ChannelPermissionService) port.AccessService {
	return &mockAccessService{
		ensureConversationAccessFn: func(ctx context.Context, orgID, userID string, role domain.Role, convID string) error {
			if perm == nil {
				return nil
			}
			ok, err := perm.UserHasAccess(ctx, orgID, convID, userID, role)
			if err != nil || !ok {
				return apperr.ErrForbidden
			}
			return nil
		},
		ensureConversationSendAccessFn: func(ctx context.Context, orgID, userID string, role domain.Role, convID string) error {
			if perm == nil {
				return nil
			}
			p, err := perm.ResolvePermissions(ctx, orgID, convID, userID, role)
			if err != nil || p == nil || !p.CanSend {
				return apperr.ErrForbidden
			}
			return nil
		},
		ensureConversationManageAccessFn: func(ctx context.Context, orgID, userID string, role domain.Role, convID string) error {
			if perm == nil {
				return nil
			}
			p, err := perm.ResolvePermissions(ctx, orgID, convID, userID, role)
			if err != nil || p == nil || !p.CanManage {
				return apperr.ErrForbidden
			}
			return nil
		},
	}
}

type mockAuthService struct {
	loginFn           func(ctx context.Context, p domain.LoginParams) (*domain.Account, []*domain.User, string, error)
	validateSessionFn func(ctx context.Context, token string) (*domain.Session, error)
	logoutFn          func(ctx context.Context, sessionID string) error
	listSessionsFn    func(ctx context.Context, userID string) ([]*domain.Session, error)
	revokeSessionFn   func(ctx context.Context, userID, sessionID string) error
}

func (m *mockAuthService) Login(ctx context.Context, p domain.LoginParams) (*domain.Account, []*domain.User, string, error) {
	return m.loginFn(ctx, p)
}
func (m *mockAuthService) ValidateSession(ctx context.Context, token string) (*domain.Session, error) {
	return m.validateSessionFn(ctx, token)
}
func (m *mockAuthService) ValidateSessionByID(ctx context.Context, sessionID string) (*domain.Session, error) {
	return nil, apperr.ErrSessionNotFound
}
func (m *mockAuthService) Logout(ctx context.Context, sessionID string) error {
	return m.logoutFn(ctx, sessionID)
}
func (m *mockAuthService) ListSessions(ctx context.Context, userID string) ([]*domain.Session, error) {
	if m.listSessionsFn == nil {
		return nil, nil
	}
	return m.listSessionsFn(ctx, userID)
}
func (m *mockAuthService) RevokeSession(ctx context.Context, userID, sessionID string) error {
	if m.revokeSessionFn == nil {
		return nil
	}
	return m.revokeSessionFn(ctx, userID, sessionID)
}
func (m *mockAuthService) HashPassword(password string) (string, error)    { return "", nil }
func (m *mockAuthService) CheckPassword(password, encodedHash string) bool { return true }
func (m *mockAuthService) RequestPasswordReset(ctx context.Context, email string) (string, error) {
	return "mock-token-123", nil
}
func (m *mockAuthService) ConfirmPasswordReset(ctx context.Context, token, newPassword string) error {
	return nil
}
func (m *mockAuthService) ValidateResetToken(ctx context.Context, token string) error {
	return nil
}

type mockUserService struct {
	getByIDFn        func(ctx context.Context, orgID, id string) (*domain.User, error)
	listUsersFn      func(ctx context.Context, orgID string, filter domain.UserFilter) (*domain.UserListResult, error)
	updateProfileFn  func(ctx context.Context, orgID, userID, name string, avatarURL *string) (*domain.User, error)
	changePasswordFn func(ctx context.Context, orgID, userID, currentPassword, newPassword string) error
}

func (m *mockUserService) GetByID(ctx context.Context, orgID, id string) (*domain.User, error) {
	return m.getByIDFn(ctx, orgID, id)
}
func (m *mockUserService) ListUsers(ctx context.Context, orgID string, filter domain.UserFilter) (*domain.UserListResult, error) {
	if m.listUsersFn != nil {
		return m.listUsersFn(ctx, orgID, filter)
	}
	return &domain.UserListResult{Users: nil, HasMore: false}, nil
}
func (m *mockUserService) UpdateRole(ctx context.Context, orgID, id string, role domain.Role, callerRole domain.Role, callerID string) error {
	return nil
}
func (m *mockUserService) UpdateActive(ctx context.Context, orgID, id string, active bool, callerID string) error {
	return nil
}
func (m *mockUserService) UpdateProfile(ctx context.Context, orgID, userID, name string, avatarURL *string) (*domain.User, error) {
	if m.updateProfileFn != nil {
		return m.updateProfileFn(ctx, orgID, userID, name, avatarURL)
	}
	return m.getByIDFn(ctx, orgID, userID)
}
func (m *mockUserService) UploadAvatar(ctx context.Context, orgID, userID string, file io.Reader, filename, contentType string, size int64) (*domain.User, error) {
	return m.getByIDFn(ctx, orgID, userID)
}
func (m *mockUserService) ChangePassword(ctx context.Context, orgID, userID, currentPassword, newPassword string) error {
	if m.changePasswordFn != nil {
		return m.changePasswordFn(ctx, orgID, userID, currentPassword, newPassword)
	}
	return nil
}

type mockOrgService struct {
	getByIDFn func(ctx context.Context, id string) (*domain.Organization, error)
	createFn  func(ctx context.Context, name, adminName, adminEmail, adminPassword string) (*domain.Organization, *domain.User, error)
	existsFn  func(ctx context.Context) (bool, error)
	updateFn  func(ctx context.Context, orgID, name string, messageEditWindowMinute int) (*domain.Organization, error)
	deleteFn  func(ctx context.Context, orgID, confirmName string) error
}

type mockProjectMemberService struct {
	listByUserFn     func(ctx context.Context, orgID, userID string) ([]*domain.UserProjectMembership, error)
	setMembershipsFn func(ctx context.Context, orgID, userID string, assignments []domain.ProjectAssignment) error
}

func (m *mockProjectMemberService) List(ctx context.Context, orgID, projectID string, filter domain.UserFilter) (*domain.ProjectMemberListResult, error) {
	return nil, nil
}
func (m *mockProjectMemberService) Add(ctx context.Context, orgID, projectID, userID string, role domain.Role) error {
	return nil
}
func (m *mockProjectMemberService) Remove(ctx context.Context, orgID, projectID, userID string) error {
	return nil
}
func (m *mockProjectMemberService) UpdateRole(ctx context.Context, orgID, projectID, userID string, role domain.Role) error {
	return nil
}
func (m *mockProjectMemberService) ListByUser(ctx context.Context, orgID, userID string) ([]*domain.UserProjectMembership, error) {
	if m.listByUserFn != nil {
		return m.listByUserFn(ctx, orgID, userID)
	}
	return nil, nil
}
func (m *mockProjectMemberService) SetMemberships(ctx context.Context, orgID, userID string, assignments []domain.ProjectAssignment) error {
	if m.setMembershipsFn != nil {
		return m.setMembershipsFn(ctx, orgID, userID, assignments)
	}
	return nil
}

func (m *mockOrgService) GetByID(ctx context.Context, id string) (*domain.Organization, error) {
	if m.getByIDFn != nil {
		return m.getByIDFn(ctx, id)
	}
	return nil, errors.New("not found")
}
func (m *mockOrgService) Create(ctx context.Context, name, adminName, adminEmail, adminPassword string) (*domain.Organization, *domain.User, error) {
	return m.createFn(ctx, name, adminName, adminEmail, adminPassword)
}
func (m *mockOrgService) Exists(ctx context.Context) (bool, error) {
	return m.existsFn(ctx)
}
func (m *mockOrgService) ListForAccount(ctx context.Context, accountID string) ([]*domain.Workspace, error) {
	return nil, nil
}
func (m *mockOrgService) CreateWorkspace(ctx context.Context, accountID, name, displayName, email string, avatarURL *string) (*domain.Organization, *domain.User, error) {
	return nil, nil, nil
}
func (m *mockOrgService) SwitchWorkspace(ctx context.Context, accountID, orgID, currentSessionID string) (*domain.Session, string, error) {
	return nil, "", nil
}
func (m *mockOrgService) Update(ctx context.Context, orgID, name string, messageEditWindowMinute int) (*domain.Organization, error) {
	if m.updateFn != nil {
		return m.updateFn(ctx, orgID, name, messageEditWindowMinute)
	}
	return nil, nil
}
func (m *mockOrgService) Delete(ctx context.Context, orgID, confirmName string) error {
	if m.deleteFn != nil {
		return m.deleteFn(ctx, orgID, confirmName)
	}
	return nil
}

func jsonBody(v any) *bytes.Buffer {
	b, _ := json.Marshal(v)
	return bytes.NewBuffer(b)
}

func TestSetupHandler_Check_NeedsSetup(t *testing.T) {
	auth := &mockAuthService{}
	org := &mockOrgService{existsFn: func(_ context.Context) (bool, error) { return false, nil }}
	h := NewSetupHandler(org, auth, slog.Default())

	w := httptest.NewRecorder()
	r := httptest.NewRequest("GET", "/api/setup", nil)
	h.Check(w, r)

	if w.Code != http.StatusOK {
		t.Errorf("status = %d, want %d", w.Code, http.StatusOK)
	}
	var resp map[string]bool
	json.NewDecoder(w.Body).Decode(&resp)
	if resp["needs_setup"] != true {
		t.Error("needs_setup should be true")
	}
}

func TestSetupHandler_Check_AlreadySetup(t *testing.T) {
	auth := &mockAuthService{}
	org := &mockOrgService{existsFn: func(_ context.Context) (bool, error) { return true, nil }}
	h := NewSetupHandler(org, auth, slog.Default())

	w := httptest.NewRecorder()
	r := httptest.NewRequest("GET", "/api/setup", nil)
	h.Check(w, r)

	if w.Code != http.StatusOK {
		t.Errorf("status = %d, want %d", w.Code, http.StatusOK)
	}
	var resp map[string]bool
	json.NewDecoder(w.Body).Decode(&resp)
	if resp["needs_setup"] != false {
		t.Error("needs_setup should be false")
	}
}

func TestSetupHandler_Check_Error(t *testing.T) {
	auth := &mockAuthService{}
	org := &mockOrgService{existsFn: func(_ context.Context) (bool, error) { return false, errors.New("db error") }}
	h := NewSetupHandler(org, auth, slog.Default())

	w := httptest.NewRecorder()
	r := httptest.NewRequest("GET", "/api/setup", nil)
	h.Check(w, r)

	if w.Code != http.StatusInternalServerError {
		t.Errorf("status = %d, want %d", w.Code, http.StatusInternalServerError)
	}
}

func TestSetupHandler_Setup_Success(t *testing.T) {
	auth := &mockAuthService{
		loginFn: func(_ context.Context, _ domain.LoginParams) (*domain.Account, []*domain.User, string, error) {
			return &domain.Account{ID: "acct-1", Email: "admin@test.com"}, nil, "token-123", nil
		},
	}
	org := &mockOrgService{
		createFn: func(_ context.Context, name, _, _, _ string) (*domain.Organization, *domain.User, error) {
			return &domain.Organization{ID: "org-1", Name: name, Slug: strings.ToLower(name)}, &domain.User{ID: "user-1", Email: "admin@test.com"}, nil
		},
		existsFn: func(_ context.Context) (bool, error) { return false, nil },
	}
	h := NewSetupHandler(org, auth, slog.Default())

	body := jsonBody(map[string]string{"org_name": "MyOrg", "name": "Admin", "email": "admin@test.com", "password": "password123"})
	w := httptest.NewRecorder()
	r := httptest.NewRequest("POST", "/api/setup", body)
	r.Header.Set("Content-Type", "application/json")
	h.Setup(w, r)

	if w.Code != http.StatusOK {
		t.Errorf("status = %d, want %d; body: %s", w.Code, http.StatusOK, w.Body.String())
	}
	var resp map[string]bool
	json.NewDecoder(w.Body).Decode(&resp)
	if resp["success"] != true {
		t.Error("success should be true")
	}
}

func TestSetupHandler_Setup_AlreadyComplete(t *testing.T) {
	auth := &mockAuthService{}
	org := &mockOrgService{
		createFn: func(_ context.Context, _, _, _, _ string) (*domain.Organization, *domain.User, error) {
			return nil, nil, apperr.ErrSetupComplete
		},
	}
	h := NewSetupHandler(org, auth, slog.Default())

	body := jsonBody(map[string]string{"org_name": "MyOrg", "name": "Admin", "email": "admin@test.com", "password": "password123"})
	w := httptest.NewRecorder()
	r := httptest.NewRequest("POST", "/api/setup", body)
	r.Header.Set("Content-Type", "application/json")
	h.Setup(w, r)

	if w.Code != http.StatusConflict {
		t.Errorf("status = %d, want %d", w.Code, http.StatusConflict)
	}
}

func TestAuthHandler_Login_Success(t *testing.T) {
	auth := &mockAuthService{
		loginFn: func(_ context.Context, p domain.LoginParams) (*domain.Account, []*domain.User, string, error) {
			return &domain.Account{ID: "acct-1", Email: p.Email}, []*domain.User{{ID: "user-1", AccountID: "acct-1", OrgID: "org-1", Email: p.Email, Name: "Alice", Role: domain.RoleMember, IsActive: true}}, "session-token", nil
		},
	}
	user := &mockUserService{}
	org := &mockOrgService{}
	h := NewAuthHandler(auth, user, org, slog.Default())

	body := jsonBody(map[string]string{"email": "alice@test.com", "password": "password"})
	w := httptest.NewRecorder()
	r := httptest.NewRequest("POST", "/api/auth/login", body)
	r.Header.Set("Content-Type", "application/json")
	h.Login(w, r)

	if w.Code != http.StatusOK {
		t.Errorf("status = %d, want %d; body: %s", w.Code, http.StatusOK, w.Body.String())
	}
	var resp map[string]any
	json.NewDecoder(w.Body).Decode(&resp)
	if resp["email"] != "alice@test.com" {
		t.Errorf("email = %v, want alice@test.com", resp["email"])
	}
	// Verify cookie was set
	cookies := w.Result().Cookies()
	var found bool
	for _, c := range cookies {
		if c.Name == "__Host-token" && c.Value == "session-token" {
			found = true
			break
		}
	}
	if !found {
		t.Error("token cookie was not set")
	}
}

func TestAuthHandler_Login_InvalidCreds(t *testing.T) {
	auth := &mockAuthService{
		loginFn: func(_ context.Context, _ domain.LoginParams) (*domain.Account, []*domain.User, string, error) {
			return nil, nil, "", apperr.ErrInvalidCreds
		},
	}
	h := NewAuthHandler(auth, &mockUserService{}, &mockOrgService{}, slog.Default())

	body := jsonBody(map[string]string{"email": "alice@test.com", "password": "wrong"})
	w := httptest.NewRecorder()
	r := httptest.NewRequest("POST", "/api/auth/login", body)
	r.Header.Set("Content-Type", "application/json")
	h.Login(w, r)

	if w.Code != http.StatusUnauthorized {
		t.Errorf("status = %d, want %d", w.Code, http.StatusUnauthorized)
	}
}

func TestAuthHandler_Me_Authenticated(t *testing.T) {
	auth := &mockAuthService{}
	user := &mockUserService{
		getByIDFn: func(_ context.Context, _, id string) (*domain.User, error) {
			return &domain.User{ID: id, OrgID: "org-1", Email: "alice@test.com", Name: "Alice", Role: domain.RoleMember}, nil
		},
	}
	org := &mockOrgService{
		getByIDFn: func(_ context.Context, id string) (*domain.Organization, error) {
			return &domain.Organization{ID: id, Name: "MyOrg", Slug: "myorg"}, nil
		},
	}
	h := NewAuthHandler(auth, user, org, slog.Default())

	ctx := context.WithValue(context.Background(), domain.CtxUserID, "user-1")
	ctx = context.WithValue(ctx, domain.CtxOrgID, "org-1")
	r := httptest.NewRequest("GET", "/api/auth/me", nil).WithContext(ctx)
	w := httptest.NewRecorder()
	h.Me(w, r)

	if w.Code != http.StatusOK {
		t.Errorf("status = %d, want %d; body: %s", w.Code, http.StatusOK, w.Body.String())
	}
	var resp map[string]any
	json.NewDecoder(w.Body).Decode(&resp)
	if resp["email"] != "alice@test.com" {
		t.Errorf("email = %v, want alice@test.com", resp["email"])
	}
}

func TestAuthHandler_Me_Unauthenticated(t *testing.T) {
	h := NewAuthHandler(&mockAuthService{}, &mockUserService{}, &mockOrgService{}, slog.Default())

	w := httptest.NewRecorder()
	r := httptest.NewRequest("GET", "/api/auth/me", nil)
	h.Me(w, r)

	if w.Code != http.StatusUnauthorized {
		t.Errorf("status = %d, want %d", w.Code, http.StatusUnauthorized)
	}
}

func TestAuthHandler_Logout(t *testing.T) {
	loggedOut := false
	auth := &mockAuthService{
		logoutFn: func(_ context.Context, _ string) error {
			loggedOut = true
			return nil
		},
	}
	h := NewAuthHandler(auth, &mockUserService{}, &mockOrgService{}, slog.Default())

	ctx := context.WithValue(context.Background(), domain.CtxSessionID, "session-1")
	r := httptest.NewRequest("POST", "/api/auth/logout", nil).WithContext(ctx)
	w := httptest.NewRecorder()
	h.Logout(w, r)

	if w.Code != http.StatusOK {
		t.Errorf("status = %d, want %d", w.Code, http.StatusOK)
	}
	if !loggedOut {
		t.Error("Logout() was not called")
	}
	if cache := w.Header().Get("Cache-Control"); cache != "no-store" {
		t.Errorf("Cache-Control = %q, want %q", cache, "no-store")
	}
}

func TestAuthHandler_Logout_ErrorLogged(t *testing.T) {
	var logBuf bytes.Buffer
	logger := slog.New(slog.NewTextHandler(&logBuf, &slog.HandlerOptions{Level: slog.LevelError}))

	called := false
	auth := &mockAuthService{
		logoutFn: func(_ context.Context, _ string) error {
			called = true
			return errors.New("db unavailable")
		},
	}
	h := NewAuthHandler(auth, &mockUserService{}, &mockOrgService{}, logger)

	ctx := context.WithValue(context.Background(), domain.CtxSessionID, "session-1")
	r := httptest.NewRequest("POST", "/api/auth/logout", nil).WithContext(ctx)
	w := httptest.NewRecorder()
	h.Logout(w, r)

	if w.Code != http.StatusOK {
		t.Errorf("status = %d, want %d", w.Code, http.StatusOK)
	}
	if !called {
		t.Error("Logout() was not called")
	}

	// Verify cookie is cleared
	setCookie := w.Result().Header.Get("Set-Cookie")
	if !strings.Contains(setCookie, "__Host-token=;") {
		t.Errorf("cookie not cleared: Set-Cookie = %q", setCookie)
	}

	// Verify the error was logged
	logOutput := logBuf.String()
	if !strings.Contains(logOutput, "db unavailable") {
		t.Errorf("expected error to be logged, got: %s", logOutput)
	}
	if !strings.Contains(logOutput, "failed to revoke session") {
		t.Errorf("expected log message about failed logout, got: %s", logOutput)
	}
}
