package handler

import (
	"context"
	"encoding/json"
	"log/slog"
	"net/http/httptest"
	"testing"

	"ipmanlk/breeze/internal/apperr"
	"ipmanlk/breeze/internal/domain"
	"ipmanlk/breeze/internal/port"
	"ipmanlk/breeze/internal/transport/dto"

	"github.com/go-chi/chi/v5"
)

type mockNotificationHandlerService struct {
	listFn        func(ctx context.Context, orgID, userID string, filter domain.NotificationFilter) (*domain.NotificationListResult, error)
	countUnreadFn func(ctx context.Context, userID string) (int, error)
	markReadFn    func(ctx context.Context, id, userID string) error
	markAllReadFn func(ctx context.Context, userID string) error
	getPrefsFn    func(ctx context.Context, userID string) ([]*domain.NotificationPreference, error)
	setPrefFn     func(ctx context.Context, userID string, notifType domain.NotificationType, enabled bool) error
	notifyFn      func(ctx context.Context, orgID, recipientID string, notifType domain.NotificationType, title, body, link, entityType, entityID, actorID string) error
	processFn     func(ctx context.Context) error
}

func (m *mockNotificationHandlerService) List(ctx context.Context, orgID, userID string, filter domain.NotificationFilter) (*domain.NotificationListResult, error) {
	if m.listFn != nil {
		return m.listFn(ctx, orgID, userID, filter)
	}
	return &domain.NotificationListResult{}, nil
}

func (m *mockNotificationHandlerService) CountUnread(ctx context.Context, userID string) (int, error) {
	if m.countUnreadFn != nil {
		return m.countUnreadFn(ctx, userID)
	}
	return 0, nil
}

func (m *mockNotificationHandlerService) MarkRead(ctx context.Context, id, userID string) error {
	if m.markReadFn != nil {
		return m.markReadFn(ctx, id, userID)
	}
	return nil
}

func (m *mockNotificationHandlerService) MarkAllRead(ctx context.Context, userID string) error {
	if m.markAllReadFn != nil {
		return m.markAllReadFn(ctx, userID)
	}
	return nil
}

func (m *mockNotificationHandlerService) GetPreferences(ctx context.Context, userID string) ([]*domain.NotificationPreference, error) {
	if m.getPrefsFn != nil {
		return m.getPrefsFn(ctx, userID)
	}
	return nil, nil
}

func (m *mockNotificationHandlerService) SetPreference(ctx context.Context, userID string, notifType domain.NotificationType, enabled bool) error {
	if m.setPrefFn != nil {
		return m.setPrefFn(ctx, userID, notifType, enabled)
	}
	return nil
}

func (m *mockNotificationHandlerService) Notify(ctx context.Context, orgID, recipientID string, notifType domain.NotificationType, title, body, link, entityType, entityID, actorID string) error {
	if m.notifyFn != nil {
		return m.notifyFn(ctx, orgID, recipientID, notifType, title, body, link, entityType, entityID, actorID)
	}
	return nil
}

func (m *mockNotificationHandlerService) ProcessDueNotifications(ctx context.Context) error {
	if m.processFn != nil {
		return m.processFn(ctx)
	}
	return nil
}

var _ port.NotificationService = (*mockNotificationHandlerService)(nil)

func TestNotificationHandler_List(t *testing.T) {
	svc := &mockNotificationHandlerService{
		listFn: func(ctx context.Context, orgID, userID string, filter domain.NotificationFilter) (*domain.NotificationListResult, error) {
			items := []*domain.Notification{
				{
					ID: "n1", OrgID: "org-1", UserID: "user-1",
					Type: domain.NotifTaskAssigned, Title: "Test",
					Body: "Body", Link: "/projects/p1?task=t1",
					EntityType: "task", EntityID: "t1",
					IsRead: false,
					Actor:  &domain.User{ID: "a1", Name: "Alice", Email: "alice@test.com"},
				},
			}
			return &domain.NotificationListResult{Items: items, HasMore: false}, nil
		},
	}
	h := NewNotificationHandler(svc, slog.Default())

	r := httptest.NewRequest("GET", "/api/notifications", nil)
	ctx := context.WithValue(context.Background(), domain.CtxUserID, "user-1")
	ctx = context.WithValue(ctx, domain.CtxOrgID, "org-1")
	r = r.WithContext(ctx)
	w := httptest.NewRecorder()
	h.List(w, r)

	if w.Code != 200 {
		t.Errorf("expected 200, got %d", w.Code)
	}
	var resp dto.PaginatedNotificationsResponse
	if err := json.NewDecoder(w.Body).Decode(&resp); err != nil {
		t.Fatal(err)
	}
	if len(resp.Items) != 1 {
		t.Fatalf("expected 1 item, got %d", len(resp.Items))
	}
	if resp.Items[0].ID != "n1" {
		t.Errorf("expected n1, got %s", resp.Items[0].ID)
	}
	if resp.Items[0].Actor.Name != "Alice" {
		t.Errorf("expected Alice, got %s", resp.Items[0].Actor.Name)
	}
}

func TestNotificationHandler_List_Unauthenticated(t *testing.T) {
	h := NewNotificationHandler(&mockNotificationHandlerService{}, slog.Default())
	r := httptest.NewRequest("GET", "/api/notifications", nil)
	w := httptest.NewRecorder()
	h.List(w, r)
	if w.Code != 401 {
		t.Errorf("expected 401, got %d", w.Code)
	}
}

func TestNotificationHandler_CountUnread(t *testing.T) {
	svc := &mockNotificationHandlerService{
		countUnreadFn: func(ctx context.Context, userID string) (int, error) {
			return 5, nil
		},
	}
	h := NewNotificationHandler(svc, slog.Default())

	r := httptest.NewRequest("GET", "/api/notifications/unread-count", nil)
	r = r.WithContext(context.WithValue(context.Background(), domain.CtxUserID, "user-1"))
	w := httptest.NewRecorder()
	h.CountUnread(w, r)

	if w.Code != 200 {
		t.Errorf("expected 200, got %d", w.Code)
	}
	var resp dto.UnreadCountResponse
	if err := json.NewDecoder(w.Body).Decode(&resp); err != nil {
		t.Fatal(err)
	}
	if resp.Count != 5 {
		t.Errorf("expected 5, got %d", resp.Count)
	}
}

func TestNotificationHandler_MarkRead(t *testing.T) {
	var markedID, markedUser string
	svc := &mockNotificationHandlerService{
		markReadFn: func(ctx context.Context, id, userID string) error {
			markedID = id
			markedUser = userID
			return nil
		},
	}
	h := NewNotificationHandler(svc, slog.Default())

	r := httptest.NewRequest("PATCH", "/api/notifications/n1/read", nil)
	routeCtx := chi.NewRouteContext()
	routeCtx.URLParams.Add("id", "n1")
	ctx := context.WithValue(context.Background(), chi.RouteCtxKey, routeCtx)
	ctx = context.WithValue(ctx, domain.CtxUserID, "user-1")
	r = r.WithContext(ctx)
	w := httptest.NewRecorder()
	h.MarkRead(w, r)

	if w.Code != 200 {
		t.Errorf("expected 200, got %d", w.Code)
	}
	if markedID != "n1" {
		t.Errorf("expected n1, got %s", markedID)
	}
	if markedUser != "user-1" {
		t.Errorf("expected user-1, got %s", markedUser)
	}
}

func TestNotificationHandler_GetPreferences(t *testing.T) {
	svc := &mockNotificationHandlerService{
		getPrefsFn: func(ctx context.Context, userID string) ([]*domain.NotificationPreference, error) {
			return []*domain.NotificationPreference{
				{Type: domain.NotifTaskAssigned, Enabled: true},
				{Type: domain.NotifTaskStatusChanged, Enabled: false},
			}, nil
		},
	}
	h := NewNotificationHandler(svc, slog.Default())

	r := httptest.NewRequest("GET", "/api/settings/notifications", nil)
	r = r.WithContext(context.WithValue(context.Background(), domain.CtxUserID, "user-1"))
	w := httptest.NewRecorder()
	h.GetPreferences(w, r)

	if w.Code != 200 {
		t.Errorf("expected 200, got %d", w.Code)
	}
	var resp []dto.NotificationPreferenceResponse
	if err := json.NewDecoder(w.Body).Decode(&resp); err != nil {
		t.Fatal(err)
	}
	if len(resp) != 2 {
		t.Fatalf("expected 2, got %d", len(resp))
	}
	if resp[0].Type != string(domain.NotifTaskAssigned) {
		t.Errorf("expected task_assigned, got %s", resp[0].Type)
	}
	if !resp[0].Enabled {
		t.Errorf("expected enabled, got false")
	}
}

// TestNotificationHandler_MarkRead_NotFound verifies that when the
// service returns apperr.ErrNotFound (the notification is missing or belongs
// to another user), the handler returns 404: not 500. Previously the handler
// compared against errors.New("not found") (never matching) and fell through
// to ServerError (500).
func TestNotificationHandler_MarkRead_NotFound(t *testing.T) {
	svc := &mockNotificationHandlerService{
		markReadFn: func(ctx context.Context, id, userID string) error {
			return apperr.ErrNotFound
		},
	}
	h := NewNotificationHandler(svc, slog.Default())

	r := httptest.NewRequest("PATCH", "/api/notifications/n1/read", nil)
	routeCtx := chi.NewRouteContext()
	routeCtx.URLParams.Add("id", "n1")
	ctx := context.WithValue(context.Background(), chi.RouteCtxKey, routeCtx)
	ctx = context.WithValue(ctx, domain.CtxUserID, "user-1")
	r = r.WithContext(ctx)
	w := httptest.NewRecorder()
	h.MarkRead(w, r)

	if w.Code != 404 {
		t.Errorf("expected 404 for missing notification, got %d: %s", w.Code, w.Body.String())
	}
}
