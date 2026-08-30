package handler

import (
	"context"
	"encoding/json"
	"log/slog"
	"net/http"
	"net/http/httptest"
	"testing"
	"time"

	"ipmanlk/breeze/internal/domain"
	"ipmanlk/breeze/internal/transport/ws"

	"github.com/coder/websocket"
)

// denyAllAccessChecker denies every room subscription. Used to verify the WS
// client fails closed when the access checker rejects a subscription.
type denyAllAccessChecker struct{}

func (denyAllAccessChecker) CanAccessConversation(context.Context, string, string, string, domain.Role) bool {
	return false
}
func (denyAllAccessChecker) CanAccessProject(context.Context, string, string, string, domain.Role) bool {
	return false
}
func (denyAllAccessChecker) CanSendInConversation(context.Context, string, string, string, domain.Role) bool {
	return false
}

// TestP0_WSRoomSubscribeDenied verifies that when the access checker denies a
// conversation subscription, the client receives a "forbidden" error and does
// NOT receive broadcasts to that conversation's room.
func TestP0_WSRoomSubscribeDenied(t *testing.T) {
	hub := ws.NewHub(slog.Default())
	go hub.Run()
	wsHandler := NewWsHandler(hub, nil, nil, nil, []string{"*"}, slog.Default())
	wsHandler.SetAccessChecker(denyAllAccessChecker{})

	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		ctx := context.WithValue(r.Context(), domain.CtxUserID, "user-1")
		ctx = context.WithValue(ctx, domain.CtxOrgID, "org-1")
		ctx = context.WithValue(ctx, domain.CtxSessionID, "session-1")
		ctx = context.WithValue(ctx, domain.CtxRole, "member")
		wsHandler.Upgrade(w, r.WithContext(ctx))
	}))
	defer server.Close()

	conn := dialWS(t, server)
	defer conn.CloseNow()

	readEnvelope(t, conn) // connected

	// Attempt to subscribe to a conversation room: must be denied.
	sub, err := ws.MarshalEvent(string(domain.WsTypeConversationSubscribe),
		struct {
			ConversationID string `json:"conversation_id"`
		}{ConversationID: "conv-1"},
	)
	if err != nil {
		t.Fatalf("marshal subscribe: %v", err)
	}
	if err := conn.Write(context.Background(), websocket.MessageText, sub); err != nil {
		t.Fatalf("write subscribe: %v", err)
	}

	// The client should receive an "error" event with code "forbidden".
	msgType, payload := readEnvelope(t, conn)
	if msgType != "error" {
		t.Fatalf("expected 'error' for denied subscription, got '%s'", msgType)
	}
	var errResp struct {
		Code string `json:"code"`
	}
	if err := json.Unmarshal(payload, &errResp); err != nil {
		t.Fatalf("unmarshal error payload: %v", err)
	}
	if errResp.Code != "forbidden" {
		t.Errorf("error code = %q, want 'forbidden'", errResp.Code)
	}

	// Broadcast to the conversation room: the denied client must NOT receive it.
	if err := hub.Broadcast(
		domain.RoomKeyConversation("org-1", "conv-1"),
		"message_new",
		map[string]any{"message_id": "m-1"},
	); err != nil {
		t.Fatalf("hub broadcast: %v", err)
	}

	// Read should time out / not deliver (the client was never subscribed).
	ctx, cancel := context.WithTimeout(context.Background(), 200*time.Millisecond)
	defer cancel()
	_, _, err = conn.Read(ctx)
	if err == nil {
		t.Error("unexpected leak: denied client received a broadcast to a conversation it has no access to")
	}
}
