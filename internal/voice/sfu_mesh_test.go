package voice

import (
	"os"
	"testing"
	"time"
)

// skipIfNoNetwork skips integration tests that require real UDP for ICE.
// Mirrors the guards on the existing loopback test.
func skipIfNoNetwork(t *testing.T) {
	t.Helper()
	if testing.Short() {
		t.Skip("skipping network integration test in short mode")
	}
	if os.Getenv("CI") != "" {
		t.Skip("skipping network integration test in CI (UDP may be blocked)")
	}
}

// TestSFU_AudioFlows_PublisherToSubscriber verifies the core SFU data path:
// RTP packets written to a publisher's mic track arrive at a subscriber's
// TrackRemote via OnTrack. This is the single most important property of an
// SFU and was previously unverified: the existing loopback test only checked
// ICE connection + participant count, never actual media flow.
//
// Strategy (pion peerconnection_media_test.go / LiveKit loopback pattern):
//  1. Two testPeers join a room via the SFU's public callback API.
//  2. Wait for the SFU to create subscriber offers in both directions
//     (driven by each publisher's OnTrack firing).
//  3. Publisher writes 50 Opus RTP packets.
//  4. Subscriber's OnTrack fires and counts received packets.
//  5. Assert the subscriber received > 0 packets (transport + media verified).
func TestSFU_AudioFlows_PublisherToSubscriber(t *testing.T) {
	skipIfNoNetwork(t)
	mesh := newTestMesh(t)

	pub := mesh.join("alice", "conn-a")
	mesh.join("bob", "conn-b")

	// Alice sends an initial burst. The SFU's OnTrack for alice fires on the
	// first packet, which triggers handleIncomingTrack → a subscriber offer to
	// bob. Bob's testPeer answers it, establishing the subscriber PC. These
	// early packets may be lost while the subscriber PC + ICE come up.
	pub.sendAudio(40)

	// Give the subscriber PC ICE a moment to connect after bob answers.
	time.Sleep(300 * time.Millisecond)

	// Second burst: now the forwarder is attached and ICE is up, so packets
	// should flow end-to-end.
	pub.sendAudio(80)

	// Wait for packets to drain through the SFU + subscriber jitter buffer.
	deadline := time.Now().Add(5 * time.Second)
	for time.Now().Before(deadline) {
		if mesh.countReceived("bob", "alice") > 0 {
			break
		}
		time.Sleep(50 * time.Millisecond)
	}

	if n := mesh.countReceived("bob", "alice"); n == 0 {
		t.Error("expected bob to receive packets from alice, got 0")
	}
}

// TestSFU_ThreeParticipantMesh_AudioReachesAll verifies the full-mesh fanout:
// when 3 participants join, audio from each publisher reaches BOTH other
// participants. This exercises handleIncomingTrack's bidirectional
// spawnSubscriber goroutines and catches any direction-asymmetry or
// missed-fanout bug: the existing tests never covered multi-party delivery.
func TestSFU_ThreeParticipantMesh_AudioReachesAll(t *testing.T) {
	skipIfNoNetwork(t)
	mesh := newTestMesh(t)

	alice := mesh.join("alice", "conn-a")
	bob := mesh.join("bob", "conn-b")
	carol := mesh.join("carol", "conn-c")

	// Bootstrap: each publisher sends a short burst. The SFU's OnTrack fires on
	// the first packet, triggering handleIncomingTrack → subscriber offers in
	// both directions for every other participant. Without this, no subscriber
	// PCs exist (OnTrack only fires once the SFU receives audio).
	alice.sendAudio(20)
	bob.sendAudio(20)
	carol.sendAudio(20)

	// Wait for the full subscriber mesh to be wired.
	waitForAllSubscriberPCs(t, mesh, "alice", "bob", "carol")

	// Give subscriber ICE a moment to connect.
	time.Sleep(300 * time.Millisecond)

	// Alice sends audio. Both Bob and Carol must receive it.
	alice.sendAudio(60)
	assertReceived(t, mesh, "alice->bob", "bob", "alice")
	assertReceived(t, mesh, "alice->carol", "carol", "alice")

	// Bob sends audio. Both Alice and Carol must receive it.
	bob.sendAudio(60)
	assertReceived(t, mesh, "bob->alice", "alice", "bob")
	assertReceived(t, mesh, "bob->carol", "carol", "bob")

	// Carol sends audio. Both Alice and Bob must receive it.
	carol.sendAudio(60)
	assertReceived(t, mesh, "carol->alice", "alice", "carol")
	assertReceived(t, mesh, "carol->bob", "bob", "carol")
}

// TestSFU_MuteStopsAudioReachingSubscribers verifies the privacy-critical
// property: SetMuted(true) pauses the publisher's forwarders so subscribers
// stop receiving audio, and SetMuted(false) resumes it. The existing test
// only checked the boolean flag: never that packets actually stop flowing.
// (mediasoup's producer.pause() / consumer-stop test is the direct analog.)
func TestSFU_MuteStopsAudioReachingSubscribers(t *testing.T) {
	skipIfNoNetwork(t)
	mesh := newTestMesh(t)

	alice := mesh.join("alice", "conn-a")
	mesh.join("bob", "conn-b")

	// Bootstrap: alice sends audio so the SFU's OnTrack fires and creates the
	// subscriber offer to bob (bob's subscriber PC for alice).
	alice.sendAudio(30)
	if !waitForSubscriberPC(mesh, "bob", "alice", 3*time.Second) {
		t.Fatal("bob's subscriber PC for alice was never created")
	}
	time.Sleep(200 * time.Millisecond) // let subscriber ICE connect

	// Baseline: audio flows before mute.
	alice.sendAudio(40)
	// Wait for the baseline burst to fully drain so the baseline count is stable.
	time.Sleep(300 * time.Millisecond)
	baseline := mesh.countReceived("bob", "alice")
	if baseline == 0 {
		t.Fatal("expected baseline packets before mute, got 0")
	}

	// Mute alice. Her forwarders to bob should pause.
	if err := mesh.sfu.SetMuted(testCtx(), "alice", "conv-test", true); err != nil {
		t.Fatalf("SetMuted(true): %v", err)
	}

	// Give the pause a beat to propagate, then send more audio.
	time.Sleep(100 * time.Millisecond)
	alice.sendAudio(30)

	// Bob must NOT receive additional packets while alice is muted.
	deadline := time.Now().Add(1 * time.Second)
	for time.Now().Before(deadline) {
		if mesh.countReceived("bob", "alice") > baseline {
			t.Fatalf("audio leaked while muted: baseline=%d now=%d", baseline, mesh.countReceived("bob", "alice"))
		}
		time.Sleep(50 * time.Millisecond)
	}

	// Unmute and verify flow resumes.
	if err := mesh.sfu.SetMuted(testCtx(), "alice", "conv-test", false); err != nil {
		t.Fatalf("SetMuted(false): %v", err)
	}
	time.Sleep(100 * time.Millisecond)
	alice.sendAudio(80)

	resumed := false
	deadline = time.Now().Add(5 * time.Second)
	for time.Now().Before(deadline) {
		if mesh.countReceived("bob", "alice") > baseline {
			resumed = true
			break
		}
		time.Sleep(50 * time.Millisecond)
	}
	if !resumed {
		t.Error("expected audio to resume after unmute, but no new packets received")
	}
}

// TestSFU_SpeakingDetection_EndToEnd verifies the full speaking-detection
// chain: real RTP packets carrying the RFC 6464 audio-level extension →
// trackReader → audioLevelDetector → onSpeaking callback. The existing
// audio_level_test.go tests the detector algorithm in isolation; this test
// verifies the SFU wiring (extension negotiation, detector creation, the
// isMuted closure passed to trackReader).
func TestSFU_SpeakingDetection_EndToEnd(t *testing.T) {
	skipIfNoNetwork(t)
	mesh := newTestMesh(t)

	alice := mesh.join("alice", "conn-a")

	// Bootstrap: send enough packets so the SFU's OnTrack reliably fires
	// (under -race, ICE + OnTrack take longer, so send a meaningful burst).
	alice.sendAudioWithLevel(30, 127) // quiet bootstrap, shouldn't trigger speaking
	if !waitForInboundTrack(mesh, "alice", 5*time.Second) {
		t.Fatal("alice's inbound track never arrived at the SFU")
	}

	// Send loud audio (RFC 6464 level=10, V=1) past the on-debounce (150ms).
	// Send a generous burst so the sustained-loud window clears the debounce
	// even under -race scheduling jitter (the detector debounces on wall-clock
	// time, so a descheduled sender could compress the burst).
	alice.sendAudioWithLevel(200, 10)
	if !waitForSpeaking(mesh, "alice", true, 2*time.Second) {
		t.Error("expected alice to be marked speaking after loud audio")
	}

	// Send silence (level=127) and wait for the off-debounce (500ms).
	// Need sustained silence past speakingOffDebounce (500ms) since the last
	// loud packet.
	alice.sendAudioWithLevel(400, 127)
	if !waitForSpeaking(mesh, "alice", false, 3*time.Second) {
		t.Error("expected alice to be marked not-speaking after silence")
	}
}

// TestSFU_MutedParticipantNeverReportsSpeaking verifies that a muted
// participant's speaking indicator is suppressed even if the browser keeps
// sending the audio-level extension (V=1) after muting: the stale-indicator
// case the audioLevelDetector.processPacket muted branch handles.
func TestSFU_MutedParticipantNeverReportsSpeaking(t *testing.T) {
	skipIfNoNetwork(t)
	mesh := newTestMesh(t)

	alice := mesh.join("alice", "conn-a")
	// Bootstrap so the inbound track exists.
	alice.sendAudioWithLevel(30, 127)
	if !waitForInboundTrack(mesh, "alice", 5*time.Second) {
		t.Fatal("inbound track never arrived")
	}

	// Mute alice, then send loud audio (enough to clear speakingOnDebounce
	// were she not muted). Speaking must never go true.
	if err := mesh.sfu.SetMuted(testCtx(), "alice", "conv-test", true); err != nil {
		t.Fatalf("SetMuted: %v", err)
	}
	alice.sendAudioWithLevel(200, 10)

	// Give the detector a beat to process the muted packets.
	time.Sleep(300 * time.Millisecond)
	if mesh.speakingState("alice") {
		t.Error("muted participant should never report speaking=true")
	}
}

// waitForAllSubscriberPCs waits until every peer has a subscriber PC for
// every other peer, so the mesh is wired in all directions before audio is
// asserted. Caller must have already sent a bootstrap burst from each
// publisher so the SFU's OnTrack has fired for each.
func waitForAllSubscriberPCs(t *testing.T, mesh *testMesh, userIDs ...string) {
	t.Helper()
	deadline := time.Now().Add(8 * time.Second)
	for time.Now().Before(deadline) {
		allReady := true
		for _, sub := range userIDs {
			for _, pub := range userIDs {
				if sub == pub {
					continue
				}
				if !peerHasSubscriberPC(mesh, sub, pub) {
					allReady = false
				}
			}
		}
		if allReady {
			return
		}
		time.Sleep(50 * time.Millisecond)
	}
	t.Fatal("timed out waiting for all subscriber PCs to be created")
}

// waitForSubscriberPC polls until userID has a subscriber PC for publisherID.
func waitForSubscriberPC(mesh *testMesh, userID, publisherID string, timeout time.Duration) bool {
	deadline := time.Now().Add(timeout)
	for time.Now().Before(deadline) {
		if peerHasSubscriberPC(mesh, userID, publisherID) {
			return true
		}
		time.Sleep(20 * time.Millisecond)
	}
	return false
}

// peerHasSubscriberPC reports whether userID's testPeer has created a
// subscriber PC for publisherID (i.e. the SFU's subscriber offer was answered).
func peerHasSubscriberPC(mesh *testMesh, userID, publisherID string) bool {
	mesh.mu.Lock()
	peer, ok := mesh.peers[userID]
	mesh.mu.Unlock()
	if !ok {
		return false
	}
	peer.mu.Lock()
	defer peer.mu.Unlock()
	_, exists := peer.subPCs[publisherID]
	return exists
}

// assertReceived waits up to 5s for userID to have received at least one
// packet from publisherID, failing the test with label if not. The generous
// timeout accommodates -race overhead and CI load.
func assertReceived(t *testing.T, mesh *testMesh, label, userID, publisherID string) {
	t.Helper()
	deadline := time.Now().Add(5 * time.Second)
	for time.Now().Before(deadline) {
		if mesh.countReceived(userID, publisherID) > 0 {
			return
		}
		time.Sleep(50 * time.Millisecond)
	}
	t.Errorf("%s: %s received 0 packets from %s", label, userID, publisherID)
}

// waitForInboundTrack polls the SFU until the publisher's inbound track exists
// (i.e. handleIncomingTrack has fired and the trackReader + detector started).
func waitForInboundTrack(mesh *testMesh, userID string, timeout time.Duration) bool {
	deadline := time.Now().Add(timeout)
	for time.Now().Before(deadline) {
		mesh.sfu.mu.RLock()
		room, ok := mesh.sfu.rooms[mesh.convID]
		mesh.sfu.mu.RUnlock()
		if ok {
			if p, ok := room.getParticipant(userID); ok {
				if _, _, ready := p.getInboundTrack(); ready {
					return true
				}
			}
		}
		time.Sleep(20 * time.Millisecond)
	}
	return false
}

// waitForSpeaking polls the mesh's speaking state until it matches want or the
// timeout elapses.
func waitForSpeaking(mesh *testMesh, userID string, want bool, timeout time.Duration) bool {
	deadline := time.Now().Add(timeout)
	for time.Now().Before(deadline) {
		if mesh.speakingState(userID) == want {
			return true
		}
		time.Sleep(20 * time.Millisecond)
	}
	return mesh.speakingState(userID) == want
}
