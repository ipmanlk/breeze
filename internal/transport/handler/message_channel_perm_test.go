package handler

import (
	"bytes"
	"context"
	"encoding/json"
	"log/slog"
	"net/http"
	"net/http/httptest"
	"testing"

	"ipmanlk/breeze/internal/domain"
	"ipmanlk/breeze/internal/port"
	"ipmanlk/breeze/internal/transport/dto"
)

// newMessageHandlerWithPerm builds a MessageHandler wired with a real
// ChannelPermissionService mock, so the send/manage guards consult it instead
// of falling back to conversation membership.
func newMessageHandlerWithPerm(svc port.MessageService, perm port.ChannelPermissionService) *MessageHandler {
	return NewMessageHandler(MessageHandlerDeps{
		SVC:            svc,
		MentionSvc:     &mockMentionSVC{},
		AttRepo:        &mockMsgAttRepo{},
		PendingAttRepo: &mockPendingAttRepo{},
		ReactionRepo:   &mockReactionRepoHandler{},
		AccessSvc:      accessSvcFromPerm(perm),
		StoreBack:      &mockStore{},
		Log:            slog.Default(),
	})
}

// permSvcReturning builds a ChannelPermissionService mock that always
// resolves the given permission set for the conversation.
func permSvcReturning(perms domain.ChannelPermissions) port.ChannelPermissionService {
	return &mockChannelPermissionService{
		resolvePermissionsFn: func(ctx context.Context, orgID, channelID, userID string, userRole domain.Role) (*domain.ChannelPermissions, error) {
			cp := perms
			return &cp, nil
		},
		// UserHasAccess is consulted by EnsureConversationAccess for read
		// routes; the send/manage guards go straight to ResolvePermissions.
		userHasAccessFn: func(ctx context.Context, orgID, channelID, userID string, userRole domain.Role) (bool, error) {
			return perms.CanView, nil
		},
	}
}

// TestMessageHandler_SendMessage_ChannelSendDenied verifies that when
// the channel-permission service resolves CanSend=false (e.g. an admin set an
// "everyone: send = deny" rule or a per-user override), SendMessage must
// return 403 instead of posting the message. Previously SendMessage only
// checked org PermChatSend + conversation view access, so the override was
// ignored on the HTTP path.
func TestMessageHandler_SendMessage_ChannelSendDenied(t *testing.T) {
	called := false
	svc := &mockMessageSVC{
		sendMessageFn: func(ctx context.Context, params domain.CreateMessageParams) (*domain.Message, error) {
			called = true
			return &domain.Message{ID: "msg-1"}, nil
		},
	}
	// CanView=true (so view access passes), CanSend=false (the override).
	perm := permSvcReturning(domain.ChannelPermissions{CanView: true, CanSend: false})
	h := newMessageHandlerWithPerm(svc, perm)

	body := dto.SendMessageRequest{Content: "hello"}
	b, _ := json.Marshal(body)
	r := httptest.NewRequest(http.MethodPost, "/api/conversations/conv-1/messages", bytes.NewReader(b))
	r.Header.Set("Content-Type", "application/json")
	r = addAuthCtx(r, "user-1", "org-1")
	r = addChiURLParam(r, "id", "conv-1")
	w := httptest.NewRecorder()
	h.SendMessage(w, r)

	if w.Code != http.StatusForbidden {
		t.Fatalf("expected 403 (channel:send denied), got %d: %s", w.Code, w.Body.String())
	}
	if called {
		t.Error("SendMessage service was called despite channel:send being denied")
	}
}

// TestMessageHandler_SendMessage_ChannelSendAllowed confirms the happy path
// still works when CanSend=true (no regression from adding the guard).
func TestMessageHandler_SendMessage_ChannelSendAllowed(t *testing.T) {
	svc := &mockMessageSVC{
		sendMessageFn: func(ctx context.Context, params domain.CreateMessageParams) (*domain.Message, error) {
			return &domain.Message{ID: "msg-1", ConversationID: "conv-1", Content: "hello"}, nil
		},
	}
	perm := permSvcReturning(domain.ChannelPermissions{CanView: true, CanSend: true})
	h := newMessageHandlerWithPerm(svc, perm)

	body := dto.SendMessageRequest{Content: "hello"}
	b, _ := json.Marshal(body)
	r := httptest.NewRequest(http.MethodPost, "/api/conversations/conv-1/messages", bytes.NewReader(b))
	r.Header.Set("Content-Type", "application/json")
	r = addAuthCtx(r, "user-1", "org-1")
	r = addChiURLParam(r, "id", "conv-1")
	w := httptest.NewRecorder()
	h.SendMessage(w, r)

	if w.Code != http.StatusCreated {
		t.Fatalf("expected 201 (channel:send allowed), got %d: %s", w.Code, w.Body.String())
	}
}

// TestMessageHandler_PinMessage_ChannelManageDenied verifies that pinning is a
// moderation action that requires channel:manage. When CanManage=false, PinMessage
// returns 403 even though the user can view the conversation.
func TestMessageHandler_PinMessage_ChannelManageDenied(t *testing.T) {
	called := false
	svc := &mockMessageSVC{
		pinMessageFn: func(ctx context.Context, orgID, msgID, convID, pinnerID string) error {
			called = true
			return nil
		},
	}
	perm := permSvcReturning(domain.ChannelPermissions{CanView: true, CanSend: true, CanManage: false})
	h := newMessageHandlerWithPerm(svc, perm)

	r := httptest.NewRequest(http.MethodPost, "/api/conversations/conv-1/messages/m-1/pin", nil)
	r = addAuthCtx(r, "user-1", "org-1")
	r = addChiURLParam(r, "id", "conv-1")
	r = addChiURLParam(r, "msg_id", "m-1")
	w := httptest.NewRecorder()
	h.PinMessage(w, r)

	if w.Code != http.StatusForbidden {
		t.Fatalf("expected 403 (channel:manage denied), got %d: %s", w.Code, w.Body.String())
	}
	if called {
		t.Error("PinMessage service was called despite channel:manage being denied")
	}
}

// TestMessageHandler_UnpinMessage_ChannelManageDenied mirrors the above for unpin.
func TestMessageHandler_UnpinMessage_ChannelManageDenied(t *testing.T) {
	called := false
	svc := &mockMessageSVC{
		unpinMessageFn: func(ctx context.Context, orgID, msgID, convID string) error {
			called = true
			return nil
		},
	}
	perm := permSvcReturning(domain.ChannelPermissions{CanView: true, CanSend: true, CanManage: false})
	h := newMessageHandlerWithPerm(svc, perm)

	r := httptest.NewRequest(http.MethodDelete, "/api/conversations/conv-1/messages/m-1/pin", nil)
	r = addAuthCtx(r, "user-1", "org-1")
	r = addChiURLParam(r, "id", "conv-1")
	r = addChiURLParam(r, "msg_id", "m-1")
	w := httptest.NewRecorder()
	h.UnpinMessage(w, r)

	if w.Code != http.StatusForbidden {
		t.Fatalf("expected 403 (channel:manage denied), got %d: %s", w.Code, w.Body.String())
	}
	if called {
		t.Error("UnpinMessage service was called despite channel:manage being denied")
	}
}

// ensure context import is used when helpers reference it
var _ = context.Background
