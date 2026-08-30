package handler

import (
	"context"
	"encoding/json"
	"log/slog"
	"net/http"
	"net/http/httptest"
	"strings"
	"testing"
	"time"

	"ipmanlk/breeze/internal/domain"
	"ipmanlk/breeze/internal/transport/ws"

	"github.com/coder/websocket"
)

func newTestWsServer(t *testing.T) (*httptest.Server, *ws.Hub) {
	t.Helper()
	hub := ws.NewHub(slog.Default())
	go hub.Run()
	wsHandler := NewWsHandler(hub, nil, nil, nil, []string{"*"}, slog.Default())
	// Allow-all access checker so the room-subscription tests (which don't
	// exercise authorization) can still subscribe. Access enforcement is
	// covered by the dedicated guard tests.
	wsHandler.SetAccessChecker(testAllowAllAccessChecker{})

	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		ctx := context.WithValue(r.Context(), domain.CtxUserID, "user-1")
		ctx = context.WithValue(ctx, domain.CtxOrgID, "org-1")
		ctx = context.WithValue(ctx, domain.CtxSessionID, "session-1")
		wsHandler.Upgrade(w, r.WithContext(ctx))
	}))
	return server, hub
}

// testAllowAllAccessChecker permits every room subscription. Used in WS handler
// tests that don't exercise authorization (room isolation, ping/pong). The
// dedicated ws_access behavior is covered by service/handler tests.
type testAllowAllAccessChecker struct{}

func (testAllowAllAccessChecker) CanAccessConversation(context.Context, string, string, string, domain.Role) bool {
	return true
}
func (testAllowAllAccessChecker) CanAccessProject(context.Context, string, string, string, domain.Role) bool {
	return true
}
func (testAllowAllAccessChecker) CanSendInConversation(context.Context, string, string, string, domain.Role) bool {
	return true
}

func dialWS(t *testing.T, server *httptest.Server) *websocket.Conn {
	t.Helper()
	wsURL := "ws" + strings.TrimPrefix(server.URL, "http") + "/api/ws"
	conn, _, err := websocket.Dial(context.Background(), wsURL, nil)
	if err != nil {
		t.Fatalf("failed to dial websocket: %v", err)
	}
	return conn
}

func readEnvelope(t *testing.T, conn *websocket.Conn) (string, json.RawMessage) {
	t.Helper()
	_, msg, err := conn.Read(context.Background())
	if err != nil {
		t.Fatalf("failed to read message: %v", err)
	}
	var env struct {
		Type    string          `json:"type"`
		Payload json.RawMessage `json:"payload"`
	}
	if err := json.Unmarshal(msg, &env); err != nil {
		t.Fatalf("failed to unmarshal envelope: %v", err)
	}
	return env.Type, env.Payload
}

func sendPing(t *testing.T, conn *websocket.Conn, ts int64) {
	t.Helper()
	data, err := ws.MarshalEvent(string(domain.WsTypePing),
		struct {
			Timestamp int64 `json:"timestamp"`
		}{Timestamp: ts},
	)
	if err != nil {
		t.Fatalf("failed to marshal ping: %v", err)
	}
	if err := conn.Write(context.Background(), websocket.MessageText, data); err != nil {
		t.Fatalf("failed to write ping: %v", err)
	}
}

func TestWsHandler_UpgradeAndReadConnected(t *testing.T) {
	server, _ := newTestWsServer(t)
	defer server.Close()

	conn := dialWS(t, server)
	defer conn.CloseNow()

	msgType, payload := readEnvelope(t, conn)

	if msgType != "connected" {
		t.Errorf("expected 'connected', got '%s'", msgType)
	}

	var connected struct {
		UserID    string `json:"user_id"`
		SessionID string `json:"session_id"`
	}
	if err := json.Unmarshal(payload, &connected); err != nil {
		t.Fatalf("failed to unmarshal connected payload: %v", err)
	}
	if connected.UserID != "user-1" {
		t.Errorf("expected user-1, got '%s'", connected.UserID)
	}
	if connected.SessionID != "session-1" {
		t.Errorf("expected session-1, got '%s'", connected.SessionID)
	}
}

func TestWsHandler_Upgrade_Unauthenticated(t *testing.T) {
	hub := ws.NewHub(slog.Default())
	go hub.Run()
	wsHandler := NewWsHandler(hub, nil, nil, nil, []string{"*"}, slog.Default())

	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		wsHandler.Upgrade(w, r)
	}))
	defer server.Close()

	wsURL := "ws" + strings.TrimPrefix(server.URL, "http") + "/api/ws"
	_, _, err := websocket.Dial(context.Background(), wsURL, nil)
	if err == nil {
		t.Fatal("expected error for unauthenticated request")
	}
}

func TestWsHandler_PingPongRoundTrip(t *testing.T) {
	server, _ := newTestWsServer(t)
	defer server.Close()

	conn := dialWS(t, server)
	defer conn.CloseNow()

	readEnvelope(t, conn)

	sendPing(t, conn, 42)

	msgType, payload := readEnvelope(t, conn)
	if msgType != "pong" {
		t.Errorf("expected 'pong', got '%s'", msgType)
	}

	var pong struct {
		Timestamp int64 `json:"timestamp"`
	}
	if err := json.Unmarshal(payload, &pong); err != nil {
		t.Fatalf("failed to unmarshal pong payload: %v", err)
	}
	if pong.Timestamp != 42 {
		t.Errorf("expected timestamp 42, got %d", pong.Timestamp)
	}
}

func TestWsHandler_InvalidMessageReturnsError(t *testing.T) {
	server, _ := newTestWsServer(t)
	defer server.Close()

	conn := dialWS(t, server)
	defer conn.CloseNow()

	readEnvelope(t, conn)

	if err := conn.Write(context.Background(), websocket.MessageText, []byte("{invalid json")); err != nil {
		t.Fatalf("failed to write: %v", err)
	}

	msgType, payload := readEnvelope(t, conn)
	if msgType != "error" {
		t.Errorf("expected 'error', got '%s'", msgType)
	}

	var errPayload struct {
		Code    string `json:"code"`
		Message string `json:"message"`
	}
	if err := json.Unmarshal(payload, &errPayload); err != nil {
		t.Fatalf("failed to unmarshal error payload: %v", err)
	}
	if errPayload.Code != "invalid_message" {
		t.Errorf("expected 'invalid_message', got '%s'", errPayload.Code)
	}
}

func TestWsHandler_UnknownMessageTypeReturnsError(t *testing.T) {
	server, _ := newTestWsServer(t)
	defer server.Close()

	conn := dialWS(t, server)
	defer conn.CloseNow()

	readEnvelope(t, conn)

	data, err := ws.MarshalEvent("unknown_type", map[string]string{"foo": "bar"})
	if err != nil {
		t.Fatalf("failed to marshal: %v", err)
	}
	if err := conn.Write(context.Background(), websocket.MessageText, data); err != nil {
		t.Fatalf("failed to write: %v", err)
	}

	msgType, payload := readEnvelope(t, conn)
	if msgType != "error" {
		t.Errorf("expected 'error', got '%s'", msgType)
	}

	var errPayload struct {
		Code string `json:"code"`
	}
	if err := json.Unmarshal(payload, &errPayload); err != nil {
		t.Fatalf("failed to unmarshal error payload: %v", err)
	}
	if errPayload.Code != "unknown_type" {
		t.Errorf("expected 'unknown_type', got '%s'", errPayload.Code)
	}
}

func TestWsHandler_MultiplePingsMaintainOrder(t *testing.T) {
	server, _ := newTestWsServer(t)
	defer server.Close()

	conn := dialWS(t, server)
	defer conn.CloseNow()

	readEnvelope(t, conn)

	for i := int64(1); i <= 5; i++ {
		sendPing(t, conn, i)
	}

	for i := int64(1); i <= 5; i++ {
		msgType, payload := readEnvelope(t, conn)
		if msgType != "pong" {
			t.Errorf("message %d: expected 'pong', got '%s'", i, msgType)
		}
		var pong struct {
			Timestamp int64 `json:"timestamp"`
		}
		if err := json.Unmarshal(payload, &pong); err != nil {
			t.Fatalf("failed to unmarshal pong %d: %v", i, err)
		}
		if pong.Timestamp != i {
			t.Errorf("message %d: expected timestamp %d, got %d", i, i, pong.Timestamp)
		}
	}
}

func TestWsHandler_BroadcastToOrgRoom(t *testing.T) {
	server, hub := newTestWsServer(t)
	defer server.Close()

	conn := dialWS(t, server)
	defer conn.CloseNow()

	readEnvelope(t, conn)

	if err := hub.Broadcast(
		domain.RoomKeyOrg("org-1"),
		string(domain.WsTypePong),
		map[string]int64{"timestamp": 99},
	); err != nil {
		t.Fatalf("hub broadcast failed: %v", err)
	}

	msgType, _ := readEnvelope(t, conn)
	if msgType != "pong" {
		t.Errorf("expected 'pong' from broadcast, got '%s'", msgType)
	}
}

// TestWsHandler_ProjectRoomSubscribe verifies that a client can subscribe to
// a project room via the project_subscribe WS message and then receive
// broadcasts to that room (e.g. comment_new events for live task comments).
func TestWsHandler_ProjectRoomSubscribe(t *testing.T) {
	server, hub := newTestWsServer(t)
	defer server.Close()

	conn := dialWS(t, server)
	defer conn.CloseNow()

	readEnvelope(t, conn) // connected

	// Subscribe to the project room.
	sub, err := ws.MarshalEvent(string(domain.WsTypeProjectSubscribe),
		struct {
			ProjectID string `json:"project_id"`
		}{ProjectID: "proj-1"},
	)
	if err != nil {
		t.Fatalf("marshal subscribe: %v", err)
	}
	if err := conn.Write(context.Background(), websocket.MessageText, sub); err != nil {
		t.Fatalf("write subscribe: %v", err)
	}

	// Give the hub a moment to process the subscription.
	time.Sleep(50 * time.Millisecond)

	// Broadcast a comment_new event to the project room.
	if err := hub.Broadcast(
		domain.RoomKeyProject("org-1", "proj-1"),
		"comment_new",
		map[string]any{"comment_id": "c-1"},
	); err != nil {
		t.Fatalf("hub broadcast failed: %v", err)
	}

	msgType, _ := readEnvelope(t, conn)
	if msgType != "comment_new" {
		t.Errorf("expected 'comment_new' from project room broadcast, got '%s'", msgType)
	}

	// A different project room should NOT receive the broadcast.
	if err := hub.Broadcast(
		domain.RoomKeyProject("org-1", "proj-other"),
		"comment_new",
		map[string]any{"comment_id": "c-2"},
	); err != nil {
		t.Fatalf("hub broadcast failed: %v", err)
	}

	// The next message should be absent (read should time out / not deliver).
	ctx, cancel := context.WithTimeout(context.Background(), 200*time.Millisecond)
	defer cancel()
	_, _, err = conn.Read(ctx)
	if err == nil {
		t.Error("expected no broadcast to an unsubscribed project room, but received a message")
	}
}

func TestWsHandler_ClientDisconnectCleansUp(t *testing.T) {
	server, hub := newTestWsServer(t)
	defer server.Close()

	conn := dialWS(t, server)
	defer conn.CloseNow()

	readEnvelope(t, conn)

	if err := conn.CloseNow(); err != nil {
		t.Fatalf("failed to close connection: %v", err)
	}

	// client is now disconnected; client.send should be closed
	// broadcasting does not panic even if room is empty
	hub.Broadcast(domain.RoomKeyOrg("org-1"), string(domain.WsTypePong), map[string]int64{"timestamp": 0})

	// verify a new connection to same org works fine
	conn2 := dialWS(t, server)
	defer conn2.CloseNow()
	readEnvelope(t, conn2)
}

func TestWsHandler_ConcurrentPings(t *testing.T) {
	server, _ := newTestWsServer(t)
	defer server.Close()

	conn := dialWS(t, server)
	defer conn.CloseNow()

	readEnvelope(t, conn)

	const n = 50
	for i := int64(1); i <= n; i++ {
		sendPing(t, conn, i)
	}

	received := make(map[int64]bool)
	for i := int64(1); i <= n; i++ {
		_, payload := readEnvelope(t, conn)
		var pong struct {
			Timestamp int64 `json:"timestamp"`
		}
		if err := json.Unmarshal(payload, &pong); err != nil {
			t.Fatalf("failed to unmarshal pong %d: %v", i, err)
		}
		if received[pong.Timestamp] {
			t.Fatalf("duplicate timestamp: %d", pong.Timestamp)
		}
		received[pong.Timestamp] = true
	}
	if len(received) != n {
		t.Errorf("expected %d unique pongs, got %d", n, len(received))
	}
}
