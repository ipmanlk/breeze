package voice

import (
	"context"
	"log/slog"
	"os"
	"strings"
	"testing"
	"time"

	"github.com/pion/webrtc/v4"
)

func newTestSFU(t *testing.T) *SFU {
	t.Helper()
	engine, err := NewEngine(Config{}, slog.Default())
	if err != nil {
		t.Fatalf("failed to create engine: %v", err)
	}
	return NewSFU(engine, slog.Default())
}

func TestSFU_CreatePublisher_ReturnsValidSDPOffer(t *testing.T) {
	sfu := newTestSFU(t)

	sdp, err := sfu.CreatePublisher(context.Background(), "org-1", "user-1", "conn-1", "conv-1")
	if err != nil {
		t.Fatalf("CreatePublisher failed: %v", err)
	}

	if sdp == "" {
		t.Fatal("expected non-empty SDP")
	}

	// Check for key SDP attributes
	checks := []string{
		"m=audio",
		"a=rtpmap:111 opus",
		"a=recvonly",
		"a=ice-ufrag",
		"a=fingerprint",
	}
	for _, check := range checks {
		if !strings.Contains(sdp, check) {
			t.Errorf("SDP missing expected attribute: %s", check)
		}
	}
}

func TestSFU_RoomIsolation(t *testing.T) {
	sfu := newTestSFU(t)

	_, err := sfu.CreatePublisher(context.Background(), "org-1", "user-1", "conn-1", "conv-1")
	if err != nil {
		t.Fatalf("CreatePublisher conv-1 failed: %v", err)
	}

	_, err = sfu.CreatePublisher(context.Background(), "org-1", "user-2", "conn-2", "conv-2")
	if err != nil {
		t.Fatalf("CreatePublisher conv-2 failed: %v", err)
	}

	// Two rooms should exist
	sfu.mu.RLock()
	roomCount := len(sfu.rooms)
	sfu.mu.RUnlock()
	if roomCount != 2 {
		t.Errorf("expected 2 rooms, got %d", roomCount)
	}

	// Remove from conv-1, conv-2 should survive
	if err := sfu.RemoveParticipant(context.Background(), "user-1", "conv-1"); err != nil {
		t.Fatalf("RemoveParticipant failed: %v", err)
	}

	sfu.mu.RLock()
	roomCount = len(sfu.rooms)
	sfu.mu.RUnlock()
	if roomCount != 1 {
		t.Errorf("expected 1 room after removal, got %d", roomCount)
	}
}

func TestSFU_RemoveParticipant_CleansUpPeerConnections(t *testing.T) {
	sfu := newTestSFU(t)

	_, err := sfu.CreatePublisher(context.Background(), "org-1", "user-1", "conn-1", "conv-1")
	if err != nil {
		t.Fatalf("CreatePublisher failed: %v", err)
	}

	if err := sfu.RemoveParticipant(context.Background(), "user-1", "conv-1"); err != nil {
		t.Fatalf("RemoveParticipant failed: %v", err)
	}

	// Room should be empty (auto-removed when empty)
	sfu.mu.RLock()
	room, exists := sfu.rooms["conv-1"]
	sfu.mu.RUnlock()
	if exists {
		participants := room.getParticipants()
		if len(participants) != 0 {
			t.Errorf("expected 0 participants after removal, got %d", len(participants))
		}
	}
}

func TestSFU_SetMuted_UpdatesState(t *testing.T) {
	sfu := newTestSFU(t)

	_, err := sfu.CreatePublisher(context.Background(), "org-1", "user-1", "conn-1", "conv-1")
	if err != nil {
		t.Fatalf("CreatePublisher failed: %v", err)
	}

	if err := sfu.SetMuted(context.Background(), "user-1", "conv-1", true); err != nil {
		t.Fatalf("SetMuted failed: %v", err)
	}

	sfu.mu.RLock()
	room := sfu.rooms["conv-1"]
	sfu.mu.RUnlock()

	p, ok := room.getParticipant("user-1")
	if !ok {
		t.Fatal("expected participant to exist")
	}
	if !p.isMuted() {
		t.Error("expected participant to be muted")
	}
}

func TestSFU_PublishAndSubscribe_Loopback(t *testing.T) {
	if testing.Short() {
		t.Skip("skipping loopback test in short mode")
	}

	// This test requires UDP ports to be available for ICE.
	// Skip if the environment doesn't allow UDP.
	if os.Getenv("CI") != "" {
		t.Skip("skipping loopback test in CI (UDP may be blocked)")
	}

	sfu := newTestSFU(t)

	// Create publisher
	pubSDP, err := sfu.CreatePublisher(context.Background(), "org-1", "user-1", "conn-1", "conv-1")
	if err != nil {
		t.Fatalf("CreatePublisher failed: %v", err)
	}

	// Get the SFU publisher PC so we can exchange ICE candidates
	sfu.mu.RLock()
	participant, ok := sfu.rooms["conv-1"].getParticipant("user-1")
	sfu.mu.RUnlock()
	if !ok {
		t.Fatal("participant not found")
	}
	sfuPubPC := participant.pubPC

	// Create a fake "browser" publisher PeerConnection
	pubEngine, err := NewEngine(Config{}, slog.Default())
	if err != nil {
		t.Fatalf("failed to create pub engine: %v", err)
	}
	pubPC, err := pubEngine.newPeerConnection()
	if err != nil {
		t.Fatalf("failed to create pub PC: %v", err)
	}
	defer pubPC.Close()

	// Exchange ICE candidates between the two peer connections
	pubPC.OnICECandidate(func(candidate *webrtc.ICECandidate) {
		if candidate == nil {
			return
		}
		init := candidate.ToJSON()
		if err := sfuPubPC.AddICECandidate(init); err != nil {
			t.Logf("failed to add ICE candidate from pubPC to SFU: %v", err)
		}
	})
	sfuPubPC.OnICECandidate(func(candidate *webrtc.ICECandidate) {
		if candidate == nil {
			return
		}
		init := candidate.ToJSON()
		if err := pubPC.AddICECandidate(init); err != nil {
			t.Logf("failed to add ICE candidate from SFU to pubPC: %v", err)
		}
	})

	// Add an audio track for sending
	_, err = createSilenceTrack(pubPC)
	if err != nil {
		t.Fatalf("failed to create audio track: %v", err)
	}

	// Set SFU offer as remote description
	if err := pubPC.SetRemoteDescription(parseOffer(pubSDP)); err != nil {
		t.Fatalf("SetRemoteDescription failed: %v", err)
	}

	// Create answer
	answer, err := pubPC.CreateAnswer(nil)
	if err != nil {
		t.Fatalf("CreateAnswer failed: %v", err)
	}
	if err := pubPC.SetLocalDescription(answer); err != nil {
		t.Fatalf("SetLocalDescription failed: %v", err)
	}

	// Send answer to SFU
	if err := sfu.HandleAnswer(context.Background(), "user-1", "conv-1", answer.SDP); err != nil {
		t.Fatalf("HandleAnswer failed: %v", err)
	}

	// Wait for ICE to connect
	connected := waitForICE(pubPC, 5*time.Second)
	if !connected {
		t.Fatal("publisher ICE connection did not establish")
	}

	// Wait for SFU to receive the track
	time.Sleep(500 * time.Millisecond)

	// Create subscriber for a second user
	_, err = sfu.CreatePublisher(context.Background(), "org-1", "user-2", "conn-2", "conv-1")
	if err != nil {
		t.Fatalf("CreatePublisher for user-2 failed: %v", err)
	}

	// Verify both participants exist
	sfu.mu.RLock()
	room := sfu.rooms["conv-1"]
	sfu.mu.RUnlock()
	participants := room.getParticipants()
	if len(participants) != 2 {
		t.Errorf("expected 2 participants, got %d", len(participants))
	}
}

// Helper functions for loopback test

func parseOffer(sdp string) webrtc.SessionDescription {
	return webrtc.SessionDescription{
		Type: webrtc.SDPTypeOffer,
		SDP:  sdp,
	}
}

func createSilenceTrack(pc *webrtc.PeerConnection) (*webrtc.TrackLocalStaticRTP, error) {
	track, err := webrtc.NewTrackLocalStaticRTP(
		webrtc.RTPCodecCapability{MimeType: webrtc.MimeTypeOpus, ClockRate: 48000, Channels: 2},
		"audio",
		"user-1",
	)
	if err != nil {
		return nil, err
	}
	if _, err := pc.AddTrack(track); err != nil {
		return nil, err
	}
	return track, nil
}

func waitForICE(pc *webrtc.PeerConnection, timeout time.Duration) bool {
	deadline := time.Now().Add(timeout)
	for time.Now().Before(deadline) {
		if pc.ICEConnectionState() == webrtc.ICEConnectionStateConnected ||
			pc.ICEConnectionState() == webrtc.ICEConnectionStateCompleted {
			return true
		}
		time.Sleep(50 * time.Millisecond)
	}
	return false
}

// TestCreateOfferWithTimeout_CancelledContext_ReturnsError verifies that
// createOfferWithTimeout returns ctx.Err() when the context is already
// cancelled, without blocking on the Pion CreateOffer call.
func TestCreateOfferWithTimeout_CancelledContext_ReturnsError(t *testing.T) {
	sfu := newTestSFU(t)
	pc, err := sfu.engine.newPeerConnection()
	if err != nil {
		t.Fatalf("newPeerConnection failed: %v", err)
	}
	defer pc.Close()

	ctx, cancel := context.WithCancel(context.Background())
	cancel() // cancel immediately

	_, err = createOfferWithTimeout(ctx, pc)
	if err == nil {
		t.Fatal("expected error on cancelled context")
	}
	if err != context.Canceled {
		t.Errorf("expected context.Canceled, got %v", err)
	}
}

// TestCreateOfferWithTimeout_Success verifies that createOfferWithTimeout
// succeeds with a valid context and returns a valid SDP offer.
func TestCreateOfferWithTimeout_Success(t *testing.T) {
	sfu := newTestSFU(t)
	pc, err := sfu.engine.newPeerConnection()
	if err != nil {
		t.Fatalf("newPeerConnection failed: %v", err)
	}
	defer pc.Close()

	// Add a recvonly transceiver so CreateOffer has something to negotiate.
	if _, err := pc.AddTransceiverFromKind(webrtc.RTPCodecTypeAudio, webrtc.RTPTransceiverInit{
		Direction: webrtc.RTPTransceiverDirectionRecvonly,
	}); err != nil {
		t.Fatalf("AddTransceiverFromKind failed: %v", err)
	}

	ctx := context.Background()
	offer, err := createOfferWithTimeout(ctx, pc)
	if err != nil {
		t.Fatalf("createOfferWithTimeout failed: %v", err)
	}
	if offer.SDP == "" {
		t.Fatal("expected non-empty SDP")
	}
	if offer.Type != webrtc.SDPTypeOffer {
		t.Errorf("expected SDP type offer, got %v", offer.Type)
	}
}

// TestSetLocalDescriptionWithTimeout_CancelledContext_ReturnsError verifies
// that setLocalDescriptionWithTimeout returns ctx.Err() when the context is
// already cancelled.
func TestSetLocalDescriptionWithTimeout_CancelledContext_ReturnsError(t *testing.T) {
	sfu := newTestSFU(t)
	pc, err := sfu.engine.newPeerConnection()
	if err != nil {
		t.Fatalf("newPeerConnection failed: %v", err)
	}
	defer pc.Close()

	// Add a transceiver so CreateOffer works.
	if _, err := pc.AddTransceiverFromKind(webrtc.RTPCodecTypeAudio, webrtc.RTPTransceiverInit{
		Direction: webrtc.RTPTransceiverDirectionRecvonly,
	}); err != nil {
		t.Fatalf("AddTransceiverFromKind failed: %v", err)
	}

	offer, err := pc.CreateOffer(nil)
	if err != nil {
		t.Fatalf("CreateOffer failed: %v", err)
	}

	ctx, cancel := context.WithCancel(context.Background())
	cancel()

	err = setLocalDescriptionWithTimeout(ctx, pc, offer)
	if err == nil {
		t.Fatal("expected error on cancelled context")
	}
	if err != context.Canceled {
		t.Errorf("expected context.Canceled, got %v", err)
	}
}

// TestSFU_SpawnSubscriber_CancelledContext_ReturnsEarly verifies that
// spawnSubscriber with an already-cancelled participant context returns
// before calling CreateSubscriber (which would attempt WebRTC SDP ops).
// The test asserts the subscriber offer callback is NOT triggered.
func TestSFU_SpawnSubscriber_CancelledContext_ReturnsEarly(t *testing.T) {
	sfu := newTestSFU(t)

	// Create two participants so the room and participants exist.
	_, err := sfu.CreatePublisher(context.Background(), "org-1", "user-1", "conn-1", "conv-1")
	if err != nil {
		t.Fatalf("CreatePublisher user-1 failed: %v", err)
	}
	_, err = sfu.CreatePublisher(context.Background(), "org-1", "user-2", "conn-2", "conv-1")
	if err != nil {
		t.Fatalf("CreatePublisher user-2 failed: %v", err)
	}

	// Set up a channel to detect if subscriber offer callback was triggered.
	offerCh := make(chan string, 1)
	sfu.SetOnSubscriberOffer(func(subscriberID, subscriberConnID, publisherID, convID, orgID, sdp string) {
		offerCh <- subscriberID
	})

	// Create a cancelled participant context.
	ctx, cancel := context.WithCancel(context.Background())
	cancel()

	// Call spawnSubscriber with the cancelled context: it should return
	// before calling CreateSubscriber.
	sfu.spawnSubscriber(ctx, "org-1", "user-2", "conn-2", "user-1", "conv-1")

	// The subscriber offer should NOT have been triggered.
	select {
	case <-offerCh:
		t.Fatal("spawnSubscriber should not have created a subscriber with cancelled context")
	default:
		// Good: no offer was sent.
	}
}
