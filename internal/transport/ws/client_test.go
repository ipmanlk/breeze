package ws

import (
	"context"
	"log/slog"
	"testing"
	"time"

	"ipmanlk/plume/internal/domain"
)

// TestClient_ShouldBroadcastTyping_Debounce verifies the per-conversation
// typing_start debounce. The first event for a conversation
// broadcasts; subsequent events within the window are dropped; events for
// different conversations are independent.
func TestClient_ShouldBroadcastTyping_Debounce(t *testing.T) {
	c := newTestClient("u1")
	c.SetTypingDebounce(50 * time.Millisecond)

	// First event for conv-1: allowed.
	if !c.shouldBroadcastTyping("conv-1") {
		t.Error("first typing_start for conv-1 should be broadcast")
	}
	// Immediate second event for conv-1: dropped (within window).
	if c.shouldBroadcastTyping("conv-1") {
		t.Error("second typing_start for conv-1 within debounce window should be dropped")
	}
	// Different conversation: independent, allowed.
	if !c.shouldBroadcastTyping("conv-2") {
		t.Error("first typing_start for conv-2 should be broadcast")
	}

	// After the window elapses, conv-1 is allowed again.
	time.Sleep(60 * time.Millisecond)
	if !c.shouldBroadcastTyping("conv-1") {
		t.Error("typing_start for conv-1 after debounce window should be broadcast")
	}
}

// TestClient_ShouldBroadcastTyping_NoDebounce verifies that a debounce of 0
// (disabled) broadcasts every event.
func TestClient_ShouldBroadcastTyping_NoDebounce(t *testing.T) {
	c := newTestClient("u1")
	c.SetTypingDebounce(0)

	for i := 0; i < 10; i++ {
		if !c.shouldBroadcastTyping("conv-1") {
			t.Errorf("with debounce disabled, event %d should be broadcast", i)
		}
	}
}

// TestHub_RejectedClientReceivesNoBroadcast verifies that a client rejected by
// the connection limit is never subscribed to rooms and so never receives
// broadcasts (handler-level teardown correctness).
func TestHub_RejectedClientReceivesNoBroadcast(t *testing.T) {
	hub := NewHubWithLimits(slog.Default(), 1, 0) // max 1 per user
	go hub.Run()

	accepted := newTestClient("u1")
	rejected := newTestClient("u1")

	if !hub.Register(accepted) {
		t.Fatal("first connection should be accepted")
	}
	if hub.Register(rejected) {
		t.Fatal("second connection should be rejected")
	}

	// The handler would have returned without subscribing a rejected client.
	// Only the accepted client subscribes:
	hub.Subscribe(accepted, "room:chat")
	time.Sleep(10 * time.Millisecond)

	hub.Broadcast("room:chat", "typing", map[string]string{"user_id": "u2"})

	// accepted gets it
	awaitMsg(t, accepted, 100*time.Millisecond)
	// rejected never subscribed, so it must not receive anything
	assertNoMsg(t, rejected, 50*time.Millisecond)
}

// allowAllAccessChecker is a RoomAccessChecker stub that permits everything,
// used to exercise the typing handler paths.
type allowAllAccessChecker struct{}

func (allowAllAccessChecker) CanAccessConversation(context.Context, string, string, string, domain.Role) bool {
	return true
}
func (allowAllAccessChecker) CanAccessProject(context.Context, string, string, string, domain.Role) bool {
	return true
}
func (allowAllAccessChecker) CanSendInConversation(context.Context, string, string, string, domain.Role) bool {
	return true
}

// TestClient_TypingStopNotDebounced verifies that typing_stop is NOT subject
// to the debounce (unlike typing_start): a state-change event must always
// propagate. We drive both handlers directly and confirm stop broadcasts even
// immediately after a start that would otherwise be in the debounce window.
func TestClient_TypingStopNotDebounced(t *testing.T) {
	hub := newTestHub()
	go hub.Run()

	sender := newTestClient("sender")
	sender.orgID = "o1"
	sender.hub = hub
	sender.SetAccessChecker(allowAllAccessChecker{}, domain.RoleMember)
	sender.SetTypingDebounce(5 * time.Second) // long window

	// A second client in the conversation room observes typing events.
	observer := newTestClient("observer")
	hub.Register(observer)
	hub.Subscribe(observer, "org:o1:conversation:conv-1")
	time.Sleep(10 * time.Millisecond)

	// First typing_start: broadcasts (observer gets it).
	sender.handleTypingStart("typing_start", []byte(`{"conversation_id":"conv-1"}`))
	awaitMsg(t, observer, 100*time.Millisecond)

	// A second typing_start within the window is dropped (debounced): observer
	// should NOT receive a second event.
	sender.handleTypingStart("typing_start", []byte(`{"conversation_id":"conv-1"}`))
	assertNoMsg(t, observer, 50*time.Millisecond)

	// typing_stop must NOT be debounced: it broadcasts immediately despite
	// being within the start-debounce window.
	sender.handleTypingStop("typing_stop", []byte(`{"conversation_id":"conv-1"}`))
	msg := awaitMsg(t, observer, 100*time.Millisecond)
	if len(msg) == 0 {
		t.Fatal("expected typing_stop to broadcast even within start debounce window")
	}
}
