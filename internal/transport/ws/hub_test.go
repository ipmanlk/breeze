package ws

import (
	"log/slog"
	"testing"
	"time"
)

func newTestHub() *Hub {
	return NewHub(slog.Default())
}

func newTestClient(userID string) *Client {
	return &Client{userID: userID, send: make(chan []byte, 256)}
}

func awaitMsg(t *testing.T, c *Client, timeout time.Duration) []byte {
	t.Helper()
	select {
	case msg := <-c.send:
		return msg
	case <-time.After(timeout):
		t.Fatal("timed out waiting for message")
		return nil
	}
}

func assertNoMsg(t *testing.T, c *Client, timeout time.Duration) {
	t.Helper()
	select {
	case <-c.send:
		t.Fatal("unexpected message received")
	case <-time.After(timeout):
	}
}

func TestHub_ClientSubscribedToRoomReceivesBroadcast(t *testing.T) {
	hub := newTestHub()
	go hub.Run()

	client := newTestClient("u1")
	hub.Register(client)
	hub.Subscribe(client, "room:a")
	time.Sleep(10 * time.Millisecond)

	hub.Broadcast("room:a", "pong", map[string]int64{"timestamp": 1})

	msg := awaitMsg(t, client, 100*time.Millisecond)
	if len(msg) == 0 {
		t.Error("expected non-empty broadcast message")
	}
}

func TestHub_ClientNotSubscribedDoesNotReceive(t *testing.T) {
	hub := newTestHub()
	go hub.Run()

	client := newTestClient("u1")
	hub.Register(client)
	hub.Subscribe(client, "room:a")
	time.Sleep(10 * time.Millisecond)

	hub.Broadcast("room:b", "pong", map[string]int64{"timestamp": 1})

	assertNoMsg(t, client, 50*time.Millisecond)
}

func TestHub_MultipleClientsAllReceive(t *testing.T) {
	hub := newTestHub()
	go hub.Run()

	c1 := newTestClient("u1")
	c2 := newTestClient("u1")
	c3 := newTestClient("u1")

	hub.Register(c1)
	hub.Register(c2)
	hub.Register(c3)
	hub.Subscribe(c1, "room:a")
	hub.Subscribe(c2, "room:a")
	hub.Subscribe(c3, "room:a")
	time.Sleep(10 * time.Millisecond)

	hub.Broadcast("room:a", "pong", map[string]int64{"timestamp": 1})

	awaitMsg(t, c1, 100*time.Millisecond)
	awaitMsg(t, c2, 100*time.Millisecond)
	awaitMsg(t, c3, 100*time.Millisecond)
}

func TestHub_UnsubscribeStopsDelivery(t *testing.T) {
	hub := newTestHub()
	go hub.Run()

	client := newTestClient("u1")
	hub.Register(client)
	hub.Subscribe(client, "room:a")
	time.Sleep(10 * time.Millisecond)

	hub.Unsubscribe(client, "room:a")
	time.Sleep(10 * time.Millisecond)

	hub.Broadcast("room:a", "pong", map[string]int64{"timestamp": 1})

	assertNoMsg(t, client, 50*time.Millisecond)
}

func TestHub_DisconnectUserRemovesAllRoomSubscriptions(t *testing.T) {
	hub := newTestHub()
	go hub.Run()

	client := newTestClient("u1")
	hub.Register(client)
	hub.Subscribe(client, "room:a")
	hub.Subscribe(client, "room:b")
	time.Sleep(10 * time.Millisecond)

	hub.DisconnectUser("u1")
	time.Sleep(20 * time.Millisecond)

	// send channel should be closed after disconnect
	_, ok := <-client.send
	if ok {
		t.Error("expected client send channel to be closed after disconnect")
	}

	// broadcasting to previously subscribed rooms should not panic
	hub.Broadcast("room:a", "pong", map[string]int64{"timestamp": 1})
	hub.Broadcast("room:b", "pong", map[string]int64{"timestamp": 1})

	// remaining client in room gets broadcast
	other := newTestClient("u2")
	hub.Register(other)
	hub.Subscribe(other, "room:a")
	time.Sleep(10 * time.Millisecond)

	hub.Broadcast("room:a", "pong", map[string]int64{"timestamp": 1})
	awaitMsg(t, other, 100*time.Millisecond)
}

func TestHub_UnregisterRemovesClientFromAllRooms(t *testing.T) {
	hub := newTestHub()
	go hub.Run()

	client := newTestClient("u1")
	hub.Register(client)
	hub.Subscribe(client, "room:a")
	hub.Subscribe(client, "room:b")
	time.Sleep(10 * time.Millisecond)

	hub.Unregister(client)
	time.Sleep(20 * time.Millisecond)

	// send channel should be closed after unregister
	_, ok := <-client.send
	if ok {
		t.Error("expected client send channel to be closed after unregister")
	}

	// broadcasting should not panic
	hub.Broadcast("room:a", "pong", map[string]int64{"timestamp": 1})
	hub.Broadcast("room:b", "pong", map[string]int64{"timestamp": 1})

	// remaining client in room gets broadcast
	other := newTestClient("u2")
	hub.Register(other)
	hub.Subscribe(other, "room:a")
	time.Sleep(10 * time.Millisecond)

	hub.Broadcast("room:a", "pong", map[string]int64{"timestamp": 1})
	awaitMsg(t, other, 100*time.Millisecond)
}

func TestHub_BroadcastToEmptyRoomDoesNotPanic(t *testing.T) {
	hub := newTestHub()
	go hub.Run()

	hub.Broadcast("nonexistent", "pong", map[string]int64{"timestamp": 1})
	time.Sleep(10 * time.Millisecond)
}

func TestHub_ConcurrentRegisterSubscribeAndBroadcast(t *testing.T) {
	hub := newTestHub()
	go hub.Run()

	const count = 10
	clients := make([]*Client, count)
	for i := range clients {
		clients[i] = newTestClient("u1")
	}

	for _, c := range clients {
		go hub.Register(c)
	}
	time.Sleep(20 * time.Millisecond)

	for i, c := range clients {
		hub.Subscribe(c, "room:concurrent")
		_ = i
	}
	time.Sleep(10 * time.Millisecond)

	hub.Broadcast("room:concurrent", "pong", map[string]int64{"timestamp": 1})

	for _, c := range clients {
		awaitMsg(t, c, 200*time.Millisecond)
	}
}

func TestHub_ClientInMultipleRoomsReceivesFromAll(t *testing.T) {
	hub := newTestHub()
	go hub.Run()

	client := newTestClient("u1")
	hub.Register(client)
	hub.Subscribe(client, "room:a")
	hub.Subscribe(client, "room:b")
	time.Sleep(10 * time.Millisecond)

	hub.Broadcast("room:a", "pong", map[string]int64{"timestamp": 1})
	awaitMsg(t, client, 100*time.Millisecond)

	hub.Broadcast("room:b", "pong", map[string]int64{"timestamp": 1})
	awaitMsg(t, client, 100*time.Millisecond)
}

// Voice-specific hub tests: proving targeted delivery via RoomKeyUser
// and broadcast delivery via RoomKeyConversation work for voice signaling.

func TestHub_BroadcastToUserRoom_DeliversOnlyToThatUser(t *testing.T) {
	hub := newTestHub()
	go hub.Run()
	defer hub.Unregister(newTestClient("stop"))

	c1 := newTestClient("u1")
	c2 := newTestClient("u2")
	hub.Register(c1)
	hub.Register(c2)
	hub.Subscribe(c1, "org:org-1:user:u1")
	hub.Subscribe(c2, "org:org-1:user:u2")
	time.Sleep(10 * time.Millisecond)

	hub.Broadcast("org:org-1:user:u1", "voice_signal", map[string]any{"type": "offer"})

	msg := awaitMsg(t, c1, 100*time.Millisecond)
	if msg == nil {
		t.Fatal("expected u1 to receive voice_signal")
	}
	assertNoMsg(t, c2, 50*time.Millisecond)
}

func TestHub_BroadcastToConversationRoom_DeliversToAllInRoom(t *testing.T) {
	hub := newTestHub()
	go hub.Run()
	defer hub.Unregister(newTestClient("stop"))

	c1 := newTestClient("u1")
	c2 := newTestClient("u2")
	c3 := newTestClient("u3")
	hub.Register(c1)
	hub.Register(c2)
	hub.Register(c3)
	hub.Subscribe(c1, "org:org-1:conversation:conv-1")
	hub.Subscribe(c2, "org:org-1:conversation:conv-1")
	hub.Subscribe(c3, "org:org-1:conversation:conv-1")
	time.Sleep(10 * time.Millisecond)

	hub.Broadcast("org:org-1:conversation:conv-1", "voice_state_update", map[string]any{})

	for _, c := range []*Client{c1, c2, c3} {
		msg := awaitMsg(t, c, 100*time.Millisecond)
		if msg == nil {
			t.Fatalf("expected client %s to receive voice_state_update", c.userID)
		}
	}
}

// TestHub_SurvivesPanicAndContinuesProcessing verifies that after a panic
// inside the hub select loop, the hub recovers via the for+recover pattern
// and continues processing register/subscribe/broadcast events normally.
func TestHub_SurvivesPanicAndContinuesProcessing(t *testing.T) {
	hub := newTestHub()
	go hub.Run()

	// Register a baseline client so we can prove pre-panic state is intact.
	c1 := newTestClient("u1")
	hub.Register(c1)
	hub.Subscribe(c1, "room:panic-test")
	time.Sleep(10 * time.Millisecond)

	// Inject a nil *roomMessage directly into the broadcast channel.
	// The hub's select receives it, tries msg.roomKey → nil pointer dereference,
	// the defer recover() catches the panic, and the for loop continues.
	hub.broadcast <- nil

	time.Sleep(20 * time.Millisecond)

	// Register a new client after the panic and verify it can subscribe
	// and receive a broadcast: proving the hub loop survived.
	survivor := newTestClient("u2")
	hub.Register(survivor)
	hub.Subscribe(survivor, "room:panic-test")
	time.Sleep(10 * time.Millisecond)

	hub.Broadcast("room:panic-test", "pong", map[string]int64{"seq": 1})
	msg := awaitMsg(t, survivor, 200*time.Millisecond)
	if len(msg) == 0 {
		t.Fatal("expected survivor to receive broadcast after hub panic recovery")
	}

	// Also verify the pre-panic client is unaffected (its send channel is open,
	// and the hub still delivers to it if it's still in the room).
	// c1 was registered before the panic and never unregistered, so it should
	// still be in the room.
	msg2 := awaitMsg(t, c1, 100*time.Millisecond)
	if len(msg2) == 0 {
		t.Fatal("expected pre-panic client to also receive broadcast after recovery")
	}
}

func TestHub_UnregisterAndDisconnectUserNoDoubleClosePanic(t *testing.T) {
	hub := newTestHub()
	go hub.Run()

	// Register a client and subscribe to a room.
	client := newTestClient("u1")
	hub.Register(client)
	hub.Subscribe(client, "room:a")
	time.Sleep(10 * time.Millisecond)

	// Trigger both code paths that close the send channel:
	// unregister (fires from ReadPump defer) and DisconnectUser (force removal).
	// Call them sequentially: in production a race between ReadPump exit and
	// DisconnectUser could fire both for the same client. The second close must
	// NOT panic.
	hub.Unregister(client)
	time.Sleep(5 * time.Millisecond)

	// If closeSend is idempotent, this second close is a safe no-op.
	hub.DisconnectUser("u1")
	time.Sleep(10 * time.Millisecond)

	// Verify the send channel is closed exactly once.
	_, ok := <-client.send
	if ok {
		t.Error("expected client send channel to be closed after unregister+disconnect")
	}

	// The hub must still be alive and processing events.
	survivor := newTestClient("u2")
	hub.Register(survivor)
	hub.Subscribe(survivor, "room:a")
	time.Sleep(10 * time.Millisecond)

	hub.Broadcast("room:a", "pong", map[string]int64{"after": 1})
	msg := awaitMsg(t, survivor, 100*time.Millisecond)
	if len(msg) == 0 {
		t.Fatal("expected hub to still process broadcasts after unregister+disconnect")
	}
}

// TestHub_UnregisterAndWait_AccurateRemainingCount verifies that
// UnregisterAndWait returns the correct remaining connection count for a user
// after removing exactly one client. This fixes the multi-tab presence
// race: the caller must know if ALL connections for a user are gone before
// setting presence offline.
func TestHub_UnregisterAndWait_AccurateRemainingCount(t *testing.T) {
	hub := newTestHub()
	go hub.Run()

	// Register two clients for the same user (simulates two browser tabs).
	c1 := newTestClient("u1")
	c2 := newTestClient("u1")
	hub.Register(c1)
	hub.Register(c2)
	time.Sleep(10 * time.Millisecond)

	// Remove c1: remaining should be 1 (c2 is still connected).
	remaining := hub.UnregisterAndWait(c1)
	if remaining != 1 {
		t.Errorf("expected remaining connections = 1 after removing one of two clients, got %d", remaining)
	}

	// Verify c1's send channel was closed.
	if _, ok := <-c1.send; ok {
		t.Error("expected c1 send channel to be closed after unregister")
	}

	// Remove c2: remaining should be 0 (last connection).
	remaining = hub.UnregisterAndWait(c2)
	if remaining != 0 {
		t.Errorf("expected remaining connections = 0 after removing last client, got %d", remaining)
	}

	// Hub must still be alive and processing events.
	c3 := newTestClient("u3")
	hub.Register(c3)
	hub.Subscribe(c3, "room:a")
	time.Sleep(10 * time.Millisecond)

	hub.Broadcast("room:a", "pong", map[string]int64{"seq": 1})
	msg := awaitMsg(t, c3, 100*time.Millisecond)
	if len(msg) == 0 {
		t.Fatal("expected hub to continue processing after UnregisterAndWait")
	}
}

// TestHub_UnregisterAndWait_MultipleConnectionsDisconnectUser verifies that
// when DisconnectUser removes a client, UnregisterAndWait for the same client
// returns the correct remaining count (the else-branch of the unregister case).
func TestHub_UnregisterAndWait_AfterDisconnectUser(t *testing.T) {
	hub := newTestHub()
	go hub.Run()

	// Register two clients for the same user.
	c1 := newTestClient("u1")
	c2 := newTestClient("u1")
	hub.Register(c1)
	hub.Register(c2)
	time.Sleep(10 * time.Millisecond)

	// Force-disconnect user: this removes both c1 and c2 via the
	// disconnect channel path, NOT the unregister channel.
	hub.DisconnectUser("u1")
	time.Sleep(20 * time.Millisecond)

	// Now attempt UnregisterAndWait for c1: client is already gone
	// (removed by DisconnectUser), so the hub's else-branch returns
	// the current tracker count (0).
	remaining := hub.UnregisterAndWait(c1)
	if remaining != 0 {
		t.Errorf("expected remaining connections = 0 after DisconnectUser, got %d", remaining)
	}

	// Second unregister for c2 should also return 0.
	remaining = hub.UnregisterAndWait(c2)
	if remaining != 0 {
		t.Errorf("expected remaining connections = 0 for second client after DisconnectUser, got %d", remaining)
	}

	// Hub must still be alive.
	survivor := newTestClient("u2")
	hub.Register(survivor)
	hub.Subscribe(survivor, "room:b")
	time.Sleep(10 * time.Millisecond)

	hub.Broadcast("room:b", "pong", map[string]int64{"seq": 1})
	msg := awaitMsg(t, survivor, 100*time.Millisecond)
	if len(msg) == 0 {
		t.Fatal("expected hub to continue processing after DisconnectUser + UnregisterAndWait")
	}
}

func TestHub_BroadcastExcept_SkipsExceptedUser(t *testing.T) {
	hub := newTestHub()
	go hub.Run()

	sender := newTestClient("sender-1")
	other := newTestClient("other-1")
	unrelated := newTestClient("unrelated-1")

	hub.Register(sender)
	hub.Register(other)
	hub.Register(unrelated)
	hub.Subscribe(sender, "room:chat")
	hub.Subscribe(other, "room:chat")
	hub.Subscribe(unrelated, "room:other")
	time.Sleep(10 * time.Millisecond)

	hub.BroadcastExcept("room:chat", "typing", map[string]string{"user_id": "sender-1"}, "sender-1")

	// Sender should NOT receive the typing event
	assertNoMsg(t, sender, 50*time.Millisecond)

	// Other client in the same room SHOULD receive it
	msg := awaitMsg(t, other, 100*time.Millisecond)
	if len(msg) == 0 {
		t.Fatal("expected other client to receive broadcastExcept message")
	}

	// Unrelated client in a different room should NOT receive it
	assertNoMsg(t, unrelated, 50*time.Millisecond)
}

func TestHub_BroadcastExcept_DifferentExceptedUserStillDelivers(t *testing.T) {
	hub := newTestHub()
	go hub.Run()

	alice := newTestClient("alice")
	bob := newTestClient("bob")

	hub.Register(alice)
	hub.Register(bob)
	hub.Subscribe(alice, "room:chat")
	hub.Subscribe(bob, "room:chat")
	time.Sleep(10 * time.Millisecond)

	// BroadcastExcept with alice as the excepted user
	hub.BroadcastExcept("room:chat", "typing", map[string]string{"user_id": "alice"}, "alice")

	// bob should receive it (bob != excepted)
	msg := awaitMsg(t, bob, 100*time.Millisecond)
	if len(msg) == 0 {
		t.Fatal("expected bob to receive broadcastExcept message")
	}

	// alice should NOT receive it
	assertNoMsg(t, alice, 50*time.Millisecond)
}

func TestHub_BroadcastExcept_EmptyRoomDoesNotPanic(t *testing.T) {
	hub := newTestHub()
	go hub.Run()

	err := hub.BroadcastExcept("room:empty", "typing", map[string]string{"user_id": "nobody"}, "nobody")
	if err != nil {
		t.Fatalf("BroadcastExcept on empty room returned error: %v", err)
	}
}

// TestHub_RejectsConnectionOverPerUserLimit verifies that a connection is
// rejected once a user exceeds MaxConnectionsPerUser. Register
// returns false for the rejected client, which must be torn down by the caller.
func TestHub_RejectsConnectionOverPerUserLimit(t *testing.T) {
	hub := NewHubWithLimits(slog.Default(), 2, 0) // max 2 per user, unlimited global
	go hub.Run()

	c1 := newTestClient("u1")
	c2 := newTestClient("u1")
	c3 := newTestClient("u1")

	if !hub.Register(c1) {
		t.Error("first connection should be accepted")
	}
	if !hub.Register(c2) {
		t.Error("second connection should be accepted (at limit)")
	}
	if hub.Register(c3) {
		t.Error("third connection should be REJECTED (over per-user limit)")
	}

	// Rejected client must not be tracked or registered.
	if hub.GetConnectionCount("u1") != 2 {
		t.Errorf("expected 2 tracked connections, got %d", hub.GetConnectionCount("u1"))
	}
}

// TestHub_RejectsConnectionOverGlobalLimit verifies the global cap is enforced
// independent of per-user limits.
func TestHub_RejectsConnectionOverGlobalLimit(t *testing.T) {
	hub := NewHubWithLimits(slog.Default(), 0, 2) // unlimited per user, max 2 global
	go hub.Run()

	if !hub.Register(newTestClient("u1")) {
		t.Error("first connection should be accepted")
	}
	if !hub.Register(newTestClient("u2")) {
		t.Error("second connection should be accepted")
	}
	if hub.Register(newTestClient("u3")) {
		t.Error("third connection should be REJECTED (over global limit)")
	}
}

// TestHub_ZeroLimitsAcceptsAll verifies that 0 limits are unlimited (the
// default NewHub behavior used by tests and unconfigured deployments).
func TestHub_ZeroLimitsAcceptsAll(t *testing.T) {
	hub := NewHubWithLimits(slog.Default(), 0, 0)
	go hub.Run()

	for i := 0; i < 50; i++ {
		if !hub.Register(newTestClient("u1")) {
			t.Fatalf("connection %d should be accepted with no limits", i)
		}
	}
}

// TestHub_Shutdown_ClosesAllClients verifies graceful shutdown: Shutdown
// closes every client's send channel so their pumps exit, and Done() is
// signalled.
func TestHub_Shutdown_ClosesAllClients(t *testing.T) {
	hub := newTestHub()
	go hub.Run()

	c1 := newTestClient("u1")
	c2 := newTestClient("u2")
	c3 := newTestClient("u3")
	hub.Register(c1)
	hub.Register(c2)
	hub.Register(c3)
	time.Sleep(10 * time.Millisecond)

	hub.Shutdown()

	select {
	case <-hub.Done():
	case <-time.After(time.Second):
		t.Fatal("hub did not signal Done() within 1s of Shutdown")
	}

	// Every client's send channel must be closed.
	for _, c := range []*Client{c1, c2, c3} {
		if _, ok := <-c.send; ok {
			t.Errorf("expected client %s send channel closed after Shutdown", c.userID)
		}
	}
}

// TestHub_Shutdown_Idempotent verifies Shutdown can be called multiple times
// without panicking (the shutdown channel is guarded).
func TestHub_Shutdown_Idempotent(t *testing.T) {
	hub := newTestHub()
	go hub.Run()

	hub.Shutdown()
	select {
	case <-hub.Done():
	case <-time.After(time.Second):
		t.Fatal("first Shutdown did not complete")
	}

	hub.Shutdown() // must not panic on a closed channel
	hub.Shutdown()
}
