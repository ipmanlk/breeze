package voice

import (
	"context"
	"encoding/json"
	"log/slog"
	"sync"
	"testing"
	"time"

	"github.com/pion/rtp"
	"github.com/pion/webrtc/v4"
)

// This file contains integration-test scaffolding that drives the SFU the way
// a real fleet of browser clients would: each testPeer owns a publisher
// PeerConnection (sends audio to the SFU) and a subscriber PeerConnection per
// other participant (receives audio from the SFU). All signaling is routed
// through the SFU's *public* callback API: SetOnICECandidate,
// SetOnSubscriberOffer, SetOnSubscriberICECandidate: exactly as the production
// VoiceService does, instead of reaching into the SFU's private room/participant
// state. This is the pattern used by pion's peerconnection_media_test.go and
// LiveKit's test peer factories.
//
// These tests are gated behind the same UDP/network skip guards as the
// existing loopback test, since they open real ICE candidate pairs.

// testPeer simulates a single browser tab connected to the SFU.
type testPeer struct {
	userID   string
	connID   string
	engine   *Engine
	pubPC    *webrtc.PeerConnection
	micTrack *webrtc.TrackLocalStaticRTP

	mu     sync.Mutex
	subPCs map[string]*webrtc.PeerConnection // publisherID -> subscriber PC

	// receivedPackets records packets received per publisher userID.
	receivedMu sync.Mutex
	received   map[string]int

	t *testing.T
}

func newTestPeer(t *testing.T, userID, connID string) *testPeer {
	t.Helper()
	engine, err := NewEngine(Config{}, slog.Default())
	if err != nil {
		t.Fatalf("new engine for %s: %v", userID, err)
	}
	pubPC, err := engine.newPeerConnection()
	if err != nil {
		t.Fatalf("new pub PC for %s: %v", userID, err)
	}
	t.Cleanup(func() { pubPC.Close() })

	micTrack, err := webrtc.NewTrackLocalStaticRTP(
		webrtc.RTPCodecCapability{MimeType: webrtc.MimeTypeOpus, ClockRate: 48000, Channels: 2},
		"audio", userID,
	)
	if err != nil {
		t.Fatalf("new mic track for %s: %v", userID, err)
	}
	if _, err := pubPC.AddTrack(micTrack); err != nil {
		t.Fatalf("add mic track for %s: %v", userID, err)
	}

	return &testPeer{
		userID:   userID,
		connID:   connID,
		engine:   engine,
		pubPC:    pubPC,
		micTrack: micTrack,
		subPCs:   make(map[string]*webrtc.PeerConnection),
		received: make(map[string]int),
		t:        t,
	}
}

// countReceived returns the number of packets this peer has received from the
// given publisher.
func (p *testPeer) countReceived(publisherID string) int {
	p.receivedMu.Lock()
	defer p.receivedMu.Unlock()
	return p.received[publisherID]
}

// close tears down all subscriber PCs (the publisher PC is cleaned via t.Cleanup).
func (p *testPeer) close() {
	p.mu.Lock()
	for _, pc := range p.subPCs {
		pc.Close()
	}
	p.subPCs = make(map[string]*webrtc.PeerConnection)
	p.mu.Unlock()
}

// createSubscriberPC handles an incoming subscriber SDP offer from the SFU for
// audio from publisherID. It creates a subscriber PC, wires OnTrack to count
// received packets, exchanges ICE, and sends the answer back to the SFU.
func (p *testPeer) createSubscriberPC(sfu *SFU, publisherID, sdp string) {
	p.mu.Lock()
	if existing, ok := p.subPCs[publisherID]; ok {
		existing.Close()
		delete(p.subPCs, publisherID)
	}
	p.mu.Unlock()

	pc, err := p.engine.newPeerConnection()
	if err != nil {
		p.t.Errorf("new sub PC for %s: %v", p.userID, err)
		return
	}

	pc.OnTrack(func(track *webrtc.TrackRemote, _ *webrtc.RTPReceiver) {
		buf := make([]byte, 1500)
		for {
			n, _, err := track.Read(buf)
			if err != nil {
				return
			}
			pkt := &rtp.Packet{}
			if err := pkt.Unmarshal(buf[:n]); err != nil {
				continue
			}
			p.receivedMu.Lock()
			p.received[publisherID]++
			p.receivedMu.Unlock()
		}
	})

	pc.OnICECandidate(func(candidate *webrtc.ICECandidate) {
		if candidate == nil {
			return
		}
		// Subscriber PC ICE candidates flow to the SFU via the subscriber
		// ICE handler, scoped to this publisher.
		if err := sfu.HandleSubscriberICECandidate(
			context.Background(), p.userID, "conv-test", publisherID,
			candidateJSON(candidate),
		); err != nil {
			p.t.Logf("sub ICE to SFU (%s): %v", p.userID, err)
		}
	})

	if err := pc.SetRemoteDescription(webrtc.SessionDescription{
		Type: webrtc.SDPTypeOffer, SDP: sdp,
	}); err != nil {
		p.t.Errorf("sub SetRemoteDescription for %s: %v", p.userID, err)
		pc.Close()
		return
	}
	answer, err := pc.CreateAnswer(nil)
	if err != nil {
		p.t.Errorf("sub CreateAnswer for %s: %v", p.userID, err)
		pc.Close()
		return
	}
	if err := pc.SetLocalDescription(answer); err != nil {
		p.t.Errorf("sub SetLocalDescription for %s: %v", p.userID, err)
		pc.Close()
		return
	}

	p.mu.Lock()
	p.subPCs[publisherID] = pc
	p.mu.Unlock()
	p.t.Cleanup(func() { pc.Close() })

	// Send the subscriber answer back to the SFU.
	if err := sfu.HandleSubscriberAnswer(
		context.Background(), p.userID, publisherID, "conv-test", answer.SDP,
	); err != nil {
		p.t.Errorf("HandleSubscriberAnswer for %s: %v", p.userID, err)
	}
}

// testMesh wires a set of testPeers to an SFU via its public callback API,
// replicating the VoiceService's signaling routing in miniature.
type testMesh struct {
	t     *testing.T
	sfu   *SFU
	mu    sync.Mutex
	peers map[string]*testPeer // userID -> peer

	speakingMu sync.Mutex
	speaking   map[string]bool // userID -> speaking

	orgID  string
	convID string
}

func newTestMesh(t *testing.T) *testMesh {
	t.Helper()
	sfu := newTestSFU(t)
	m := &testMesh{
		t:        t,
		sfu:      sfu,
		peers:    make(map[string]*testPeer),
		speaking: make(map[string]bool),
		orgID:    "org-test",
		convID:   "conv-test",
	}

	// Wire the SFU callbacks exactly like VoiceService.NewVoiceService does.
	sfu.SetOnICECandidate(func(userID, connID, convID, orgID, candidateJSON string) {
		m.mu.Lock()
		peer, ok := m.peers[userID]
		m.mu.Unlock()
		if !ok {
			return
		}
		if err := peer.pubPC.AddICECandidate(parseCandidate(candidateJSON)); err != nil {
			m.t.Logf("pub ICE %s: %v", userID, err)
		}
	})

	sfu.SetOnSubscriberOffer(func(subscriberID, subscriberConnID, publisherID, convID, orgID, sdp string) {
		m.mu.Lock()
		peer, ok := m.peers[subscriberID]
		m.mu.Unlock()
		if !ok {
			return
		}
		peer.createSubscriberPC(m.sfu, publisherID, sdp)
	})

	sfu.SetOnSubscriberICECandidate(func(subscriberID, subscriberConnID, publisherID, convID, orgID, candidateJSON string) {
		m.mu.Lock()
		peer, ok := m.peers[subscriberID]
		m.mu.Unlock()
		if !ok {
			return
		}
		peer.mu.Lock()
		pc, ok := peer.subPCs[publisherID]
		peer.mu.Unlock()
		if !ok {
			return
		}
		if err := pc.AddICECandidate(parseCandidate(candidateJSON)); err != nil {
			m.t.Logf("sub ICE %s<- %s: %v", subscriberID, publisherID, err)
		}
	})

	sfu.SetOnSpeaking(func(userID, orgID string, speaking bool) {
		m.speakingMu.Lock()
		m.speaking[userID] = speaking
		m.speakingMu.Unlock()
	})

	return m
}

// join creates a publisher for the given user, exchanges SDP/ICE with the SFU,
// and waits for ICE to connect. This mirrors the full client join flow.
func (m *testMesh) join(userID, connID string) *testPeer {
	m.t.Helper()
	peer := newTestPeer(m.t, userID, connID)

	m.mu.Lock()
	m.peers[userID] = peer
	m.mu.Unlock()
	m.t.Cleanup(func() {
		peer.close()
		m.mu.Lock()
		delete(m.peers, userID)
		m.mu.Unlock()
	})

	// SFU creates publisher offer.
	offer, err := m.sfu.CreatePublisher(context.Background(), m.orgID, userID, connID, m.convID)
	if err != nil {
		m.t.Fatalf("CreatePublisher %s: %v", userID, err)
	}

	// Client wires its publisher PC's ICE candidates to the SFU.
	peer.pubPC.OnICECandidate(func(candidate *webrtc.ICECandidate) {
		if candidate == nil {
			return
		}
		if err := m.sfu.HandleICECandidate(
			context.Background(), userID, m.convID, candidateJSON(candidate),
		); err != nil {
			m.t.Logf("pub ICE %s to SFU: %v", userID, err)
		}
	})

	// Client applies the offer and answers.
	if err := peer.pubPC.SetRemoteDescription(webrtc.SessionDescription{
		Type: webrtc.SDPTypeOffer, SDP: offer,
	}); err != nil {
		m.t.Fatalf("pub SetRemoteDescription %s: %v", userID, err)
	}
	answer, err := peer.pubPC.CreateAnswer(nil)
	if err != nil {
		m.t.Fatalf("pub CreateAnswer %s: %v", userID, err)
	}
	if err := peer.pubPC.SetLocalDescription(answer); err != nil {
		m.t.Fatalf("pub SetLocalDescription %s: %v", userID, err)
	}
	if err := m.sfu.HandleAnswer(context.Background(), userID, m.convID, answer.SDP); err != nil {
		m.t.Fatalf("HandleAnswer %s: %v", userID, err)
	}

	// Wait for ICE to connect on the publisher leg.
	if !waitForICE(peer.pubPC, 5*time.Second) {
		m.t.Fatalf("publisher ICE did not connect for %s", userID)
	}
	return peer
}

// sendAudio writes n Opus RTP packets from the given publisher's mic track,
// spacing them ~2ms apart (50 pps, matching real Opus framing).
func (p *testPeer) sendAudio(n int) {
	p.t.Helper()
	seq := uint16(0)
	ts := uint32(0)
	for i := 0; i < n; i++ {
		pkt := &rtp.Packet{
			Header: rtp.Header{
				Version:        2,
				PayloadType:    111,
				SequenceNumber: seq,
				Timestamp:      ts,
				SSRC:           0xC0FFEE,
			},
			Payload: []byte{0x00, 0x00}, // 2 bytes of silence-ish Opus
		}
		if err := p.micTrack.WriteRTP(pkt); err != nil {
			p.t.Logf("WriteRTP %s: %v", p.userID, err)
			return
		}
		seq++
		ts += 960 // 20ms at 48kHz
		time.Sleep(2 * time.Millisecond)
	}
}

// candidateJSON marshals an ICE candidate to the JSON string the SFU expects.
func candidateJSON(c *webrtc.ICECandidate) string {
	init := c.ToJSON()
	data, _ := json.Marshal(init)
	return string(data)
}

// parseCandidate unmarshals a candidate JSON string into an ICECandidateInit.
func parseCandidate(s string) webrtc.ICECandidateInit {
	var c webrtc.ICECandidateInit
	_ = json.Unmarshal([]byte(s), &c)
	return c
}

// speakingState returns the latest speaking state for a user.
func (m *testMesh) speakingState(userID string) bool {
	m.speakingMu.Lock()
	defer m.speakingMu.Unlock()
	return m.speaking[userID]
}

// countReceived returns how many packets userID received from publisherID.
func (m *testMesh) countReceived(userID, publisherID string) int {
	m.mu.Lock()
	peer, ok := m.peers[userID]
	m.mu.Unlock()
	if !ok {
		return 0
	}
	return peer.countReceived(publisherID)
}

// testCtx returns a context with a generous deadline for SFU calls in tests.
func testCtx() context.Context {
	ctx, cancel := context.WithTimeout(context.Background(), 5*time.Second)
	// The caller is a short test op; leak the cancel to keep the linter quiet;
	// tests don't outlive the process.
	_ = cancel
	return ctx
}

// sendAudioWithLevel writes n Opus RTP packets carrying the RFC 6464 audio-level
// header extension (extID=1) with the given dBov level (0=loudest, 127=silent).
// The V bit (0x80) is set so the level is treated as voice-active.
func (p *testPeer) sendAudioWithLevel(n int, level byte) {
	p.t.Helper()
	const audioLevelExtID = 1
	seq := uint16(0)
	ts := uint32(0)
	for i := 0; i < n; i++ {
		pkt := &rtp.Packet{
			Header: rtp.Header{
				Version:        2,
				PayloadType:    111,
				SequenceNumber: seq,
				Timestamp:      ts,
				SSRC:           0xC0FFEE,
			},
			Payload: []byte{0x00, 0x00},
		}
		// 1-byte RFC 6464 extension: V(1) | level(7). V=0x80, level masked.
		pkt.Header.SetExtension(audioLevelExtID, []byte{0x80 | (level & 0x7F)})
		if err := p.micTrack.WriteRTP(pkt); err != nil {
			p.t.Logf("WriteRTP %s: %v", p.userID, err)
			return
		}
		seq++
		ts += 960
		time.Sleep(2 * time.Millisecond)
	}
}
