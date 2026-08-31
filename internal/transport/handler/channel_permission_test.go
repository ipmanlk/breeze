package handler

import (
	"context"
	"encoding/json"
	"errors"
	"log/slog"
	"net/http"
	"net/http/httptest"
	"testing"

	"github.com/go-chi/chi/v5"

	"ipmanlk/plume/internal/domain"
)

type mockConvService struct {
	getByIDFn func(ctx context.Context, orgID, id, userID string) (*domain.Conversation, error)
}

func (m *mockConvService) GetByID(ctx context.Context, orgID, id, userID string) (*domain.Conversation, error) {
	if m.getByIDFn != nil {
		return m.getByIDFn(ctx, orgID, id, userID)
	}
	return &domain.Conversation{}, nil
}
func (m *mockConvService) ListMyConversations(ctx context.Context, orgID, userID string, filter domain.ConversationFilter) (*domain.ConversationListResult, error) {
	return nil, nil
}
func (m *mockConvService) ListByParent(ctx context.Context, orgID, parentID, userID string, role domain.Role, includeProjectLinked bool) ([]*domain.Conversation, error) {
	return nil, nil
}
func (m *mockConvService) CreateChannel(ctx context.Context, params domain.CreateConversationParams) (*domain.Conversation, error) {
	return nil, nil
}
func (m *mockConvService) UpdateConversation(ctx context.Context, conv *domain.Conversation) error {
	return nil
}
func (m *mockConvService) UpdateChannelParent(ctx context.Context, orgID, id string, parentID *string, positionKey string) error {
	return nil
}
func (m *mockConvService) DeleteConversation(ctx context.Context, orgID, id, userID string) error {
	return nil
}
func (m *mockConvService) AddMembers(ctx context.Context, orgID, convID, adderID string, memberIDs []string) error {
	return nil
}
func (m *mockConvService) RemoveMember(ctx context.Context, orgID, convID, removerID, targetID string) error {
	return nil
}
func (m *mockConvService) GetMembers(ctx context.Context, convID string) ([]*domain.ConversationMember, error) {
	return nil, nil
}
func (m *mockConvService) MarkRead(ctx context.Context, convID, userID string) error {
	return nil
}
func (m *mockConvService) SetMuted(ctx context.Context, orgID, convID, userID string, muted bool) error {
	return nil
}
func (m *mockConvService) SetNotificationLevel(ctx context.Context, orgID, convID, userID string, level domain.NotificationLevel) error {
	return nil
}
func (m *mockConvService) GetPinnedMessages(ctx context.Context, convID string, limit int) ([]*domain.Message, error) {
	return nil, nil
}
func (m *mockConvService) EnsureGeneralChannel(ctx context.Context, orgID, userID string) error {
	return nil
}
func (m *mockConvService) GetChannelProjectLinks(ctx context.Context, channelID string) ([]string, error) {
	return nil, nil
}
func (m *mockConvService) SetChannelProjectLinks(ctx context.Context, channelID string, projectIDs []string) error {
	return nil
}
func (m *mockConvService) ListAccess(ctx context.Context, orgID, channelID string) ([]*domain.ChannelAccessEntry, error) {
	return nil, nil
}
func (m *mockConvService) CreateDM(ctx context.Context, orgID, createdBy, targetUserID string) (*domain.Conversation, error) {
	return nil, nil
}
func (m *mockConvService) CreateGroupDM(ctx context.Context, orgID, createdBy string, memberIDs []string) (*domain.Conversation, error) {
	return nil, nil
}

type mockChannelPermissionService struct {
	resolvePermissionsFn     func(ctx context.Context, orgID, channelID, userID string, userRole domain.Role) (*domain.ChannelPermissions, error)
	resolveRolePermissionsFn func(ctx context.Context, orgID, channelID string) ([]*domain.EffectivePermission, error)
	getPermissionsFn         func(ctx context.Context, channelID string) ([]*domain.PermissionRule, error)
	setPermissionsFn         func(ctx context.Context, channelID string, rules []*domain.PermissionRule) error
	getUserOverridesFn       func(ctx context.Context, channelID string) ([]*domain.UserPermissionOverride, error)
	setUserOverridesFn       func(ctx context.Context, channelID string, overrides []*domain.UserPermissionOverride) error
	userHasAccessFn          func(ctx context.Context, orgID, channelID, userID string, userRole domain.Role) (bool, error)
	getUsersWithProjectFn    func(ctx context.Context, orgID, projectID string) ([]*domain.User, error)
}

func (m *mockChannelPermissionService) ResolvePermissions(ctx context.Context, orgID, channelID, userID string, userRole domain.Role) (*domain.ChannelPermissions, error) {
	if m.resolvePermissionsFn != nil {
		return m.resolvePermissionsFn(ctx, orgID, channelID, userID, userRole)
	}
	return &domain.ChannelPermissions{}, nil
}

func (m *mockChannelPermissionService) GetPermissions(ctx context.Context, channelID string) ([]*domain.PermissionRule, error) {
	if m.getPermissionsFn != nil {
		return m.getPermissionsFn(ctx, channelID)
	}
	return nil, nil
}

func (m *mockChannelPermissionService) SetPermissions(ctx context.Context, channelID string, rules []*domain.PermissionRule) error {
	if m.setPermissionsFn != nil {
		return m.setPermissionsFn(ctx, channelID, rules)
	}
	return nil
}

func (m *mockChannelPermissionService) GetUserOverrides(ctx context.Context, channelID string) ([]*domain.UserPermissionOverride, error) {
	if m.getUserOverridesFn != nil {
		return m.getUserOverridesFn(ctx, channelID)
	}
	return nil, nil
}

func (m *mockChannelPermissionService) SetUserOverrides(ctx context.Context, channelID string, overrides []*domain.UserPermissionOverride) error {
	if m.setUserOverridesFn != nil {
		return m.setUserOverridesFn(ctx, channelID, overrides)
	}
	return nil
}

func (m *mockChannelPermissionService) UserHasAccess(ctx context.Context, orgID, channelID, userID string, userRole domain.Role) (bool, error) {
	if m.userHasAccessFn != nil {
		return m.userHasAccessFn(ctx, orgID, channelID, userID, userRole)
	}
	return true, nil
}

func (m *mockChannelPermissionService) GetUsersWithProjectAccess(ctx context.Context, orgID, projectID string) ([]*domain.User, error) {
	if m.getUsersWithProjectFn != nil {
		return m.getUsersWithProjectFn(ctx, orgID, projectID)
	}
	return nil, nil
}

func (m *mockChannelPermissionService) ResolveRolePermissions(ctx context.Context, orgID, channelID string) ([]*domain.EffectivePermission, error) {
	if m.resolveRolePermissionsFn != nil {
		return m.resolveRolePermissionsFn(ctx, orgID, channelID)
	}
	return nil, nil
}

func (m *mockChannelPermissionService) ListAccess(ctx context.Context, orgID, channelID string) ([]*domain.ChannelAccessEntry, error) {
	return nil, nil
}

func TestChannelPermissionHandler_GetPermissions(t *testing.T) {
	svc := &mockChannelPermissionService{
		resolveRolePermissionsFn: func(_ context.Context, _ string, channelID string) ([]*domain.EffectivePermission, error) {
			if channelID != "ch-1" {
				t.Errorf("channelID = %s, want ch-1", channelID)
			}
			perms := []domain.Permission{domain.PermChannelView, domain.PermChannelSend, domain.PermChannelManage, domain.PermChannelPermissions}
			roles := []domain.Role{"everyone", "member", "viewer", "guest"}
			var result []*domain.EffectivePermission
			for _, role := range roles {
				for _, perm := range perms {
					explicit := role == "member" && perm == domain.PermChannelView
					result = append(result, &domain.EffectivePermission{
						Role:       role,
						Permission: perm,
						Allow:      explicit || (role == "everyone" || role == "member" && perm == domain.PermChannelSend),
						Explicit:   explicit,
					})
				}
			}
			return result, nil
		},
	}
	h := NewChannelPermissionHandler(svc, &mockAccessService{}, slog.Default())

	rctx := chi.NewRouteContext()
	rctx.URLParams.Add("id", "ch-1")
	r := httptest.NewRequest("GET", "/conversations/ch-1/permissions", nil)
	ctx := context.WithValue(r.Context(), chi.RouteCtxKey, rctx)
	ctx = context.WithValue(ctx, domain.CtxUserID, "user-1")
	ctx = context.WithValue(ctx, domain.CtxOrgID, "org-1")
	ctx = context.WithValue(ctx, domain.CtxRole, "member")
	r = r.WithContext(ctx)
	w := httptest.NewRecorder()

	h.GetPermissions(w, r)

	if w.Code != http.StatusOK {
		t.Fatalf("status = %d, want %d; body: %s", w.Code, http.StatusOK, w.Body.String())
	}
	var resp []map[string]any
	if err := json.NewDecoder(w.Body).Decode(&resp); err != nil {
		t.Fatalf("decode: %v", err)
	}
	if len(resp) != 16 {
		t.Fatalf("len = %d, want 16 (4 roles x 4 permissions)", len(resp))
	}
	// Find the explicit member+view rule.
	var found bool
	for _, item := range resp {
		if item["role"] == "member" && item["permission"] == "channel:view" {
			found = true
			if item["allow"] != true {
				t.Errorf("member+view allow = %v, want true", item["allow"])
			}
			break
		}
	}
	if !found {
		t.Error("member+channel:view not found in response")
	}
}

func TestChannelPermissionHandler_GetPermissions_Error(t *testing.T) {
	svc := &mockChannelPermissionService{
		resolveRolePermissionsFn: func(_ context.Context, _ string, _ string) ([]*domain.EffectivePermission, error) {
			return nil, errors.New("db error")
		},
	}
	h := NewChannelPermissionHandler(svc, &mockAccessService{}, slog.Default())

	rctx := chi.NewRouteContext()
	rctx.URLParams.Add("id", "ch-1")
	r := httptest.NewRequest("GET", "/conversations/ch-1/permissions", nil)
	ctx := context.WithValue(r.Context(), chi.RouteCtxKey, rctx)
	ctx = context.WithValue(ctx, domain.CtxUserID, "user-1")
	ctx = context.WithValue(ctx, domain.CtxOrgID, "org-1")
	ctx = context.WithValue(ctx, domain.CtxRole, "member")
	r = r.WithContext(ctx)
	w := httptest.NewRecorder()

	h.GetPermissions(w, r)

	if w.Code != http.StatusInternalServerError {
		t.Errorf("status = %d, want %d; body: %s", w.Code, http.StatusInternalServerError, w.Body.String())
	}
}

func TestChannelPermissionHandler_SetPermissions(t *testing.T) {
	var savedRules []*domain.PermissionRule
	svc := &mockChannelPermissionService{
		setPermissionsFn: func(_ context.Context, channelID string, rules []*domain.PermissionRule) error {
			if channelID != "ch-1" {
				t.Errorf("channelID = %s, want ch-1", channelID)
			}
			savedRules = rules
			return nil
		},
	}
	h := NewChannelPermissionHandler(svc, &mockAccessService{}, slog.Default())

	body := jsonBody(map[string]any{
		"rules": []map[string]any{
			{"role": "member", "permission": "channel:view", "allow": true},
		},
	})
	rctx := chi.NewRouteContext()
	rctx.URLParams.Add("id", "ch-1")
	r := httptest.NewRequest("PUT", "/conversations/ch-1/permissions", body)
	ctx := context.WithValue(context.Background(), chi.RouteCtxKey, rctx)
	ctx = context.WithValue(ctx, domain.CtxOrgID, "org-1")
	ctx = context.WithValue(ctx, domain.CtxUserID, "user-1")
	ctx = context.WithValue(ctx, domain.CtxRole, "admin")
	r = r.WithContext(ctx)
	r.Header.Set("Content-Type", "application/json")
	w := httptest.NewRecorder()

	h.SetPermissions(w, r)

	if w.Code != http.StatusOK {
		t.Fatalf("status = %d, want %d; body: %s", w.Code, http.StatusOK, w.Body.String())
	}
	if len(savedRules) != 1 || savedRules[0].Permission != "channel:view" {
		t.Error("expected saved rule with permission channel:view")
	}
}

func TestChannelPermissionHandler_SetPermissions_ValidationError(t *testing.T) {
	h := NewChannelPermissionHandler(&mockChannelPermissionService{}, &mockAccessService{}, slog.Default())

	rctx := chi.NewRouteContext()
	rctx.URLParams.Add("id", "ch-1")
	r := httptest.NewRequest("PUT", "/conversations/ch-1/permissions", jsonBody(map[string]any{}))
	r = r.WithContext(context.WithValue(r.Context(), chi.RouteCtxKey, rctx))
	r.Header.Set("Content-Type", "application/json")
	w := httptest.NewRecorder()

	h.SetPermissions(w, r)

	if w.Code != http.StatusBadRequest {
		t.Errorf("status = %d, want %d; body: %s", w.Code, http.StatusBadRequest, w.Body.String())
	}
}

func TestChannelPermissionHandler_ResolvePermissions(t *testing.T) {
	svc := &mockChannelPermissionService{
		resolvePermissionsFn: func(_ context.Context, orgID, channelID, userID string, userRole domain.Role) (*domain.ChannelPermissions, error) {
			if channelID != "ch-1" {
				t.Errorf("channelID = %s, want ch-1", channelID)
			}
			if userID != "user-1" {
				t.Errorf("userID = %s, want user-1", userID)
			}
			if orgID != "org-1" {
				t.Errorf("orgID = %s, want org-1", orgID)
			}
			if userRole != domain.RoleMember {
				t.Errorf("userRole = %s, want member", userRole)
			}
			return &domain.ChannelPermissions{CanView: true, CanSend: true}, nil
		},
	}
	h := NewChannelPermissionHandler(svc, &mockAccessService{}, slog.Default())

	ctx := context.WithValue(context.Background(), domain.CtxUserID, "user-1")
	ctx = context.WithValue(ctx, domain.CtxOrgID, "org-1")
	ctx = context.WithValue(ctx, domain.CtxRole, "member")
	rctx := chi.NewRouteContext()
	rctx.URLParams.Add("id", "ch-1")
	ctx = context.WithValue(ctx, chi.RouteCtxKey, rctx)
	r := httptest.NewRequest("GET", "/conversations/ch-1/my-permissions", nil).WithContext(ctx)
	w := httptest.NewRecorder()

	h.ResolvePermissions(w, r)

	if w.Code != http.StatusOK {
		t.Fatalf("status = %d, want %d", w.Code, http.StatusOK)
	}
	var resp map[string]any
	json.NewDecoder(w.Body).Decode(&resp)
	if resp["can_view"] != true {
		t.Error("expected can_view=true")
	}
	if resp["can_send"] != true {
		t.Error("expected can_send=true")
	}
	if _, ok := resp["can_manage"]; !ok {
		t.Error("expected can_manage in response")
	}
}

func TestChannelPermissionHandler_ResolvePermissions_Error(t *testing.T) {
	svc := &mockChannelPermissionService{
		resolvePermissionsFn: func(_ context.Context, _, _, _ string, _ domain.Role) (*domain.ChannelPermissions, error) {
			return nil, errors.New("not found")
		},
	}
	h := NewChannelPermissionHandler(svc, &mockAccessService{}, slog.Default())

	ctx := context.WithValue(context.Background(), domain.CtxUserID, "user-1")
	ctx = context.WithValue(ctx, domain.CtxOrgID, "org-1")
	ctx = context.WithValue(ctx, domain.CtxRole, "member")
	rctx := chi.NewRouteContext()
	rctx.URLParams.Add("id", "ch-1")
	ctx = context.WithValue(ctx, chi.RouteCtxKey, rctx)
	r := httptest.NewRequest("GET", "/conversations/ch-1/my-permissions", nil).WithContext(ctx)
	w := httptest.NewRecorder()

	h.ResolvePermissions(w, r)

	if w.Code != http.StatusInternalServerError {
		t.Errorf("status = %d, want %d", w.Code, http.StatusInternalServerError)
	}
}
