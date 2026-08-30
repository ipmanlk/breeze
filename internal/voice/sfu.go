package voice

import (
	"context"
	"encoding/json"
	"fmt"
	"log/slog"
	"sync"
	"time"

	"github.com/pion/webrtc/v4"

	"ipmanlk/breeze/internal/domain"
)

// SFU implements the Selective Forwarding Unit for voice channels.
//
// Multi-tab correctness: each voice session is owned by a single WS
// connection (connID). Signaling callbacks carry the owning connID so the
// service layer routes SDP/ICE to RoomKeyConnection (one tab) rather than
// RoomKeyUser (every tab). The participant stores its connID so subscriber
// offers and ICE candidates reach the exact browser tab that owns the PC.
type SFU struct {
	engine                   *Engine
	rooms                    map[string]*Room // convID -> Room
	mu                       sync.RWMutex
	log                      *slog.Logger
	onSpeaking               func(userID, orgID string, speaking bool)
	onSubscriberOffer        func(subscriberID, subscriberConnID, publisherID, convID, orgID, sdp string)
	onICECandidate           func(userID, connID, convID, orgID, candidateJSON string)
	onSubscriberICECandidate func(subscriberID, subscriberConnID, publisherID, convID, orgID, candidateJSON string)
}

// NewSFU creates a new SFU instance.
func NewSFU(engine *Engine, log *slog.Logger) *SFU {
	return &SFU{
		engine:                   engine,
		rooms:                    make(map[string]*Room),
		log:                      log,
		onSpeaking:               func(string, string, bool) {},
		onSubscriberOffer:        func(string, string, string, string, string, string) {},
		onICECandidate:           func(string, string, string, string, string) {},
		onSubscriberICECandidate: func(string, string, string, string, string, string) {},
	}
}

// SetOnSpeaking sets the callback for speaking state changes.
func (s *SFU) SetOnSpeaking(fn func(userID, orgID string, speaking bool)) {
	s.onSpeaking = fn
}

// SetOnSubscriberOffer sets the callback for when a subscriber SDP offer is created.
func (s *SFU) SetOnSubscriberOffer(fn func(subscriberID, subscriberConnID, publisherID, convID, orgID, sdp string)) {
	s.onSubscriberOffer = fn
}

// SetOnICECandidate sets the callback for when a publisher PC generates an ICE candidate.
func (s *SFU) SetOnICECandidate(fn func(userID, connID, convID, orgID, candidateJSON string)) {
	s.onICECandidate = fn
}

// SetOnSubscriberICECandidate sets the callback for when a subscriber PC generates an ICE candidate.
func (s *SFU) SetOnSubscriberICECandidate(fn func(subscriberID, subscriberConnID, publisherID, convID, orgID, candidateJSON string)) {
	s.onSubscriberICECandidate = fn
}

// ICEServers returns the static ICE servers as domain types for inclusion in join results.
func (s *SFU) ICEServers() []domain.ICEServer {
	return webrtcToDomainICEServers(s.engine.ICEServers())
}

// ICEServersForUser returns ICE servers for a specific join, generating
// ephemeral TURN REST credentials when configured.
func (s *SFU) ICEServersForUser(userID string) []domain.ICEServer {
	return webrtcToDomainICEServers(s.engine.ICEServersForUser(userID))
}

// MaxParticipants returns the per-channel participant cap.
func (s *SFU) MaxParticipants() int {
	return s.engine.MaxParticipants()
}

func webrtcToDomainICEServers(servers []webrtc.ICEServer) []domain.ICEServer {
	result := make([]domain.ICEServer, len(servers))
	for i, srv := range servers {
		cred := ""
		if c, ok := srv.Credential.(string); ok {
			cred = c
		}
		result[i] = domain.ICEServer{
			URLs:       srv.URLs,
			Username:   srv.Username,
			Credential: cred,
		}
	}
	return result
}

// createOfferWithTimeout wraps pc.CreateOffer with a context timeout.
// The goroutine that performs CreateOffer runs until completion (no leak)
// even if the context fires, but the result is discarded on timeout.
func createOfferWithTimeout(ctx context.Context, pc *webrtc.PeerConnection) (webrtc.SessionDescription, error) {
	type offerResult struct {
		sd  webrtc.SessionDescription
		err error
	}
	ch := make(chan offerResult, 1)
	go func() {
		sd, err := pc.CreateOffer(nil)
		ch <- offerResult{sd, err}
	}()
	select {
	case <-ctx.Done():
		return webrtc.SessionDescription{}, ctx.Err()
	case r := <-ch:
		return r.sd, r.err
	}
}

// setLocalDescriptionWithTimeout wraps pc.SetLocalDescription with a context timeout.
// Cancellation takes precedence: if the context is done, the context error is
// returned even when the underlying SetLocalDescription raced to completion —
// a bare two-case select would pick randomly between two ready cases, making
// the outcome non-deterministic. Callers abort (and close the pc) on error, so
// reporting cancellation is always safe.
func setLocalDescriptionWithTimeout(ctx context.Context, pc *webrtc.PeerConnection, desc webrtc.SessionDescription) error {
	ch := make(chan error, 1)
	go func() {
		ch <- pc.SetLocalDescription(desc)
	}()
	select {
	case <-ctx.Done():
		return ctx.Err()
	case err := <-ch:
		if err != nil {
			return err
		}
		// The call succeeded, but honor a cancellation that landed in the
		// same instant rather than racing it.
		select {
		case <-ctx.Done():
			return ctx.Err()
		default:
		}
		return nil
	}
}

// CreatePublisher creates a publisher peer connection for a user.
func (s *SFU) CreatePublisher(ctx context.Context, orgID, userID, connID, convID string) (string, error) {
	room := s.getOrCreateRoom(convID, orgID)

	if existing, exists := room.getParticipant(userID); exists {
		// Tab takeover: the new connection takes over the
		// existing voice session. The service layer is responsible for
		// kicking the old connection; here we just reject the duplicate so
		// the service can detect the conflict and handle it.
		_ = existing
		return "", ErrParticipantExists
	}

	pc, err := s.engine.newPeerConnection()
	if err != nil {
		return "", fmt.Errorf("create peer connection: %w", err)
	}

	if _, err := pc.AddTransceiverFromKind(webrtc.RTPCodecTypeAudio, webrtc.RTPTransceiverInit{
		Direction: webrtc.RTPTransceiverDirectionRecvonly,
	}); err != nil {
		pc.Close()
		return "", fmt.Errorf("add transceiver: %w", err)
	}

	pc.OnTrack(func(track *webrtc.TrackRemote, receiver *webrtc.RTPReceiver) {
		s.handleIncomingTrack(userID, convID, track, receiver)
	})

	pc.OnICECandidate(func(candidate *webrtc.ICECandidate) {
		if candidate == nil {
			return
		}
		init := candidate.ToJSON()
		data, err := json.Marshal(init)
		if err != nil {
			s.log.Error("failed to marshal ICE candidate", "error", err)
			return
		}
		s.onICECandidate(userID, connID, convID, room.orgID, string(data))
	})

	// SDP offer creation with timeout; if the participant context is
	// cancelled or the WebRTC op hangs, this fails fast instead of leaking
	// the goroutine.
	sdpCtx, cancel := context.WithTimeout(ctx, 10*time.Second)
	defer cancel()

	offer, err := createOfferWithTimeout(sdpCtx, pc)
	if err != nil {
		pc.Close()
		return "", fmt.Errorf("create offer: %w", err)
	}

	if err := setLocalDescriptionWithTimeout(sdpCtx, pc, offer); err != nil {
		pc.Close()
		return "", fmt.Errorf("set local description: %w", err)
	}

	p := newParticipant(userID, connID, pc)
	room.addParticipant(p)

	return offer.SDP, nil
}

// CreateSubscriber creates a subscriber peer connection from a subscriber to a publisher.
func (s *SFU) CreateSubscriber(ctx context.Context, orgID, subscriberID, subscriberConnID, publisherID, convID string) (string, error) {
	room := s.getOrCreateRoom(convID, orgID)

	subscriber, ok := room.getParticipant(subscriberID)
	if !ok {
		return "", fmt.Errorf("subscriber not found")
	}

	publisher, ok := room.getParticipant(publisherID)
	if !ok {
		return "", fmt.Errorf("publisher not found")
	}

	if _, exists := subscriber.getSubscriberPC(publisherID); exists {
		return "", fmt.Errorf("subscriber already has PC for publisher")
	}

	pc, err := s.engine.newPeerConnection()
	if err != nil {
		return "", fmt.Errorf("create peer connection: %w", err)
	}

	// Read the publisher's inbound track + codec safely (the inbound field
	// is written under the publisher's lock in handleIncomingTrack).
	_, inboundCodec, ok := publisher.getInboundTrack()
	if !ok {
		pc.Close()
		return "", fmt.Errorf("publisher has no inbound track")
	}

	localTrack, err := webrtc.NewTrackLocalStaticRTP(
		inboundCodec,
		"audio",
		publisherID,
	)
	if err != nil {
		pc.Close()
		return "", fmt.Errorf("create local track: %w", err)
	}

	if _, err := pc.AddTrack(localTrack); err != nil {
		pc.Close()
		return "", fmt.Errorf("add track: %w", err)
	}

	subscriber.addSubscriberPC(publisherID, pc)

	pc.OnICECandidate(func(candidate *webrtc.ICECandidate) {
		if candidate == nil {
			return
		}
		init := candidate.ToJSON()
		data, err := json.Marshal(init)
		if err != nil {
			s.log.Error("failed to marshal subscriber ICE candidate", "error", err)
			return
		}
		s.onSubscriberICECandidate(subscriberID, subscriberConnID, publisherID, convID, room.orgID, string(data))
	})

	fwd := newRTPForwarder(localTrack)
	subscriber.addForwarder(publisherID, fwd)

	// SDP offer creation with timeout; if the participant context is
	// cancelled or the WebRTC op hangs, this fails fast instead of leaking
	// the goroutine.
	sdpCtx, cancel := context.WithTimeout(ctx, 10*time.Second)
	defer cancel()

	offer, err := createOfferWithTimeout(sdpCtx, pc)
	if err != nil {
		subscriber.removeSubscriberPC(publisherID)
		return "", fmt.Errorf("create offer: %w", err)
	}

	if err := setLocalDescriptionWithTimeout(sdpCtx, pc, offer); err != nil {
		subscriber.removeSubscriberPC(publisherID)
		return "", fmt.Errorf("set local description: %w", err)
	}

	return offer.SDP, nil
}

// HandleAnswer handles an answer SDP from a client.
func (s *SFU) HandleAnswer(ctx context.Context, userID, convID, sdp string) error {
	room, ok := s.getRoom(convID)
	if !ok {
		return fmt.Errorf("room not found")
	}

	p, ok := room.getParticipant(userID)
	if !ok {
		return fmt.Errorf("participant not found")
	}

	answer := webrtc.SessionDescription{
		Type: webrtc.SDPTypeAnswer,
		SDP:  sdp,
	}

	if err := p.pubPC.SetRemoteDescription(answer); err != nil {
		return fmt.Errorf("set remote description: %w", err)
	}

	return nil
}

// HandleSubscriberAnswer handles an answer for a subscriber connection.
func (s *SFU) HandleSubscriberAnswer(ctx context.Context, subscriberID, publisherID, convID, sdp string) error {
	room, ok := s.getRoom(convID)
	if !ok {
		return fmt.Errorf("room not found")
	}

	p, ok := room.getParticipant(subscriberID)
	if !ok {
		return fmt.Errorf("subscriber not found")
	}

	pc, ok := p.getSubscriberPC(publisherID)
	if !ok {
		return fmt.Errorf("subscriber PC not found")
	}

	answer := webrtc.SessionDescription{
		Type: webrtc.SDPTypeAnswer,
		SDP:  sdp,
	}

	if err := pc.SetRemoteDescription(answer); err != nil {
		return fmt.Errorf("set remote description: %w", err)
	}

	return nil
}

// HandleICECandidate handles a publisher ICE candidate from a client.
func (s *SFU) HandleICECandidate(ctx context.Context, userID, convID, candidateJSON string) error {
	room, ok := s.getRoom(convID)
	if !ok {
		return fmt.Errorf("room not found")
	}

	p, ok := room.getParticipant(userID)
	if !ok {
		return fmt.Errorf("participant not found")
	}

	var candidate webrtc.ICECandidateInit
	if err := json.Unmarshal([]byte(candidateJSON), &candidate); err != nil {
		return fmt.Errorf("unmarshal candidate: %w", err)
	}

	if err := p.pubPC.AddICECandidate(candidate); err != nil {
		return fmt.Errorf("add ICE candidate: %w", err)
	}

	return nil
}

// HandleSubscriberICECandidate handles an ICE candidate for a subscriber connection.
func (s *SFU) HandleSubscriberICECandidate(ctx context.Context, userID, convID, publisherID, candidateJSON string) error {
	room, ok := s.getRoom(convID)
	if !ok {
		return fmt.Errorf("room not found")
	}

	p, ok := room.getParticipant(userID)
	if !ok {
		return fmt.Errorf("participant not found")
	}

	pc, ok := p.getSubscriberPC(publisherID)
	if !ok {
		return fmt.Errorf("subscriber PC not found for publisher %s", publisherID)
	}

	var candidate webrtc.ICECandidateInit
	if err := json.Unmarshal([]byte(candidateJSON), &candidate); err != nil {
		return fmt.Errorf("unmarshal candidate: %w", err)
	}

	return pc.AddICECandidate(candidate)
}

// RemoveParticipant removes a participant from a room.
func (s *SFU) RemoveParticipant(ctx context.Context, userID, convID string) error {
	room, ok := s.getRoom(convID)
	if !ok {
		return fmt.Errorf("room not found")
	}

	room.removeParticipant(userID)

	if room.participantCount() == 0 {
		s.removeRoom(convID)
	}

	return nil
}

// SetMuted sets the muted state for a participant.
// When muted, all RTP forwarders carrying this participant's audio to
// other participants are paused (audio is not forwarded to others).
func (s *SFU) SetMuted(ctx context.Context, userID, convID string, muted bool) error {
	room, ok := s.getRoom(convID)
	if !ok {
		return fmt.Errorf("room not found")
	}

	p, ok := room.getParticipant(userID)
	if !ok {
		return fmt.Errorf("participant not found")
	}

	p.setMuted(muted)

	// Pause/resume all forwarders that carry this participant's audio.
	// These forwarders live on OTHER participants (subscribers), keyed by
	// publisherID == userID.
	for _, other := range room.getParticipants() {
		other.mu.RLock()
		if fwd, ok := other.forwarders[userID]; ok {
			fwd.setPaused(muted)
		}
		other.mu.RUnlock()
	}

	return nil
}

// getOrCreateRoom gets or creates a room.
func (s *SFU) getOrCreateRoom(convID, orgID string) *Room {
	s.mu.Lock()
	defer s.mu.Unlock()

	if room, ok := s.rooms[convID]; ok {
		return room
	}

	room := newRoom(convID, orgID)
	s.rooms[convID] = room
	return room
}

// getRoom gets a room by ID.
func (s *SFU) getRoom(convID string) (*Room, bool) {
	s.mu.RLock()
	defer s.mu.RUnlock()
	room, ok := s.rooms[convID]
	return room, ok
}

// removeRoom removes a room.
func (s *SFU) removeRoom(convID string) {
	s.mu.Lock()
	defer s.mu.Unlock()

	if room, ok := s.rooms[convID]; ok {
		room.close()
		delete(s.rooms, convID)
	}
}

// handleIncomingTrack handles an incoming track from a publisher.
func (s *SFU) handleIncomingTrack(userID, convID string, track *webrtc.TrackRemote, receiver *webrtc.RTPReceiver) {
	room, ok := s.getRoom(convID)
	if !ok {
		s.log.Error("room not found for incoming track", "conv_id", convID)
		return
	}

	p, ok := room.getParticipant(userID)
	if !ok {
		s.log.Error("participant not found for incoming track", "user_id", userID)
		return
	}

	// Capture the owning connection ID for routing subscriber offers to the
	// correct tab (the subscriber is the one receiving the offer).
	subscriberConnID := p.connIDFor()

	p.mu.Lock()
	p.inbound = track

	// Detect audio level header extension ID from negotiated parameters
	var audioLevelID uint8 = 1 // fallback if not found
	if receiver != nil {
		for _, ext := range receiver.GetParameters().HeaderExtensions {
			if ext.URI == audioLevelExtURI {
				audioLevelID = uint8(ext.ID)
				break
			}
		}
	}

	// Start audio level detection; the detector runs inside a trackReader
	// (single goroutine that reads RTP packets, processes audio levels, and
	// dispatches to all active forwarders). This ensures detection works even
	// when the user is alone in the channel (no forwarders/subscribers).
	if s.onSpeaking != nil {
		p.detector = newAudioLevelDetector(userID, audioLevelID, func(uid string, speaking bool) {
			s.onSpeaking(uid, room.orgID, speaking)
		})
	}

	reader := newTrackReader(track, p.detector, p.isMuted, s.log)
	p.trackReader = reader
	// Capture the participant context under the lock so the subscriber
	// goroutines below can observe cancellation from close() (takeover).
	partCtx := p.ctx
	publisherID := userID
	p.mu.Unlock()

	// Dispatch inbound RTP to every subscriber's forwarder for this publisher.
	// The forwarders live on the SUBSCRIBER participants (keyed by publisherID),
	// not on the publisher itself; so we ask the room for the live set on each
	// packet. New subscribers added after the reader starts are picked up
	// automatically because the closure queries the room fresh each iteration.
	reader.start(func() []*rtpForwarder {
		return room.subscriberForwarders(publisherID)
	})

	// Create subscribers for all other participants
	participants := room.getParticipants()
	for otherID := range participants {
		if otherID == userID {
			continue
		}

		other, ok := room.getParticipant(otherID)
		if !ok {
			continue
		}
		otherConnID := other.connIDFor()

		// new participant subscribes to existing participant's audio
		go s.spawnSubscriber(partCtx, room.orgID, userID, subscriberConnID, otherID, convID)
		// existing participant subscribes to new participant's audio
		go s.spawnSubscriber(partCtx, room.orgID, otherID, otherConnID, userID, convID)
	}
}

// spawnSubscriber creates a WebRTC subscriber offer for one participant to
// receive another's audio track. Runs in a goroutine with panic recovery
// and respects the participant context (canceled on takeover/disconnect).
func (s *SFU) spawnSubscriber(partCtx context.Context, orgID, subscriberID, subConnID, publisherID, convID string) {
	defer func() {
		if r := recover(); r != nil {
			s.log.Error("subscriber goroutine panic recovered", "panic", r, "subscriber", subscriberID, "publisher", publisherID)
		}
	}()
	select {
	case <-partCtx.Done():
		return
	default:
	}
	sdp, err := s.CreateSubscriber(partCtx, orgID, subscriberID, subConnID, publisherID, convID)
	if err != nil {
		s.log.Error("failed to create subscriber", "error", err, "subscriber", subscriberID, "publisher", publisherID)
		return
	}
	s.onSubscriberOffer(subscriberID, subConnID, publisherID, convID, orgID, sdp)
}
