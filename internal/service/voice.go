package service

import (
	"context"
	"errors"
	"log/slog"
	"sync"

	"github.com/google/uuid"
	"ipmanlk/breeze/internal/apperr"
	"ipmanlk/breeze/internal/domain"
	"ipmanlk/breeze/internal/port"
	"ipmanlk/breeze/internal/voice"
)

// VoiceServiceDeps holds dependencies for VoiceService.
type VoiceServiceDeps struct {
	ParticipantRepo port.VoiceParticipantRepository
	ConvRepo        port.ConversationRepository
	UserRepo        port.UserRepository
	PermService     port.ChannelPermissionService
	SFU             port.VoiceSFU
	Broadcaster     port.Broadcaster
	Log             *slog.Logger
}

// voiceService implements port.VoiceService.
type voiceService struct {
	participantRepo port.VoiceParticipantRepository
	convRepo        port.ConversationRepository
	userRepo        port.UserRepository
	permService     port.ChannelPermissionService
	sfu             port.VoiceSFU
	broadcaster     port.Broadcaster
	log             *slog.Logger

	// speakingStates tracks ephemeral speaking state per (convID, userID)
	// so that broadcastVoiceStateUpdate can include accurate speaking
	// indicators. The DB doesn't persist speaking; it's purely ephemeral,
	// driven by the SFU's audio-level detector.
	speakingStates   map[string]bool // key: convID+"|"+userID
	speakingStatesMu sync.RWMutex

	// joinMu serializes Join/Leave for a given (convID, userID). Concurrent
	// joins from two tabs of the same user would otherwise both observe "no
	// existing row" and race through takeover/insert, corrupting SFU state.
	joinMu   map[string]*sync.Mutex
	joinMuMx sync.Mutex

	// speakingEvents is drained by a dedicated dispatcher goroutine so
	// speaking transitions reach DB+broadcast in order without blocking
	// the RTP track-reader loop (the callback fires under the detector
	// lock; doing I/O there stalls media processing for everyone).
	speakingEvents chan domain.SpeakingEvent
}

// speakingEventQueueSize bounds the async dispatch queue. Overflow drops
// the OLDEST transitions are coalesced by the next state change anyway;
// dropping keeps the reader loop never-blocking, which is the priority.
const speakingEventQueueSize = 256

// NewVoiceService creates a new VoiceService.
func NewVoiceService(deps VoiceServiceDeps) port.VoiceService {
	svc := &voiceService{
		participantRepo: deps.ParticipantRepo,
		convRepo:        deps.ConvRepo,
		userRepo:        deps.UserRepo,
		permService:     deps.PermService,
		sfu:             deps.SFU,
		broadcaster:     deps.Broadcaster,
		log:             deps.Log,
		speakingStates:  make(map[string]bool),
		joinMu:          make(map[string]*sync.Mutex),
		speakingEvents:  make(chan domain.SpeakingEvent, speakingEventQueueSize),
	}
	go svc.dispatchSpeakingEvents()

	// Set up speaking and ICE callbacks on SFU
	if sfu, ok := deps.SFU.(interface {
		SetOnSpeaking(func(string, string, bool))
		SetOnSubscriberOffer(func(string, string, string, string, string, string))
		SetOnICECandidate(func(string, string, string, string, string))
		SetOnSubscriberICECandidate(func(string, string, string, string, string, string))
	}); ok {
		sfu.SetOnSpeaking(svc.handleSpeaking)
		sfu.SetOnSubscriberOffer(svc.handleSubscriberOffer)
		sfu.SetOnICECandidate(svc.handleICECandidate)
		sfu.SetOnSubscriberICECandidate(svc.handleSubscriberICECandidate)
	}

	return svc
}

// Join adds a user to a voice channel.
//
// Multi-tab semantics: one active voice session per user per
// channel. If the user is already joined (e.g. from another tab), the existing
// session is torn down and the new connection takes over. The old connection
// receives a voice_kick event so it can reset its UI.
// lockJoin returns the serialized mutex for a (orgID, convID, userID) key.
func (s *voiceService) lockJoin(orgID, convID, userID string) *sync.Mutex {
	key := orgID + "|" + convID + "|" + userID
	s.joinMuMx.Lock()
	defer s.joinMuMx.Unlock()
	mu, ok := s.joinMu[key]
	if !ok {
		mu = &sync.Mutex{}
		s.joinMu[key] = mu
	}
	return mu
}

func (s *voiceService) Join(ctx context.Context, orgID, userID string, callerRole domain.Role, connID, convID string) (*domain.VoiceJoinResult, error) {
	// Serialize concurrent joins by the same user into this channel so the
	// exists-check → takeover → insert sequence can't interleave (two tabs
	// joining simultaneously would otherwise both see "fresh join").
	mu := s.lockJoin(orgID, convID, userID)
	mu.Lock()
	defer mu.Unlock()

	// Get conversation
	conv, err := s.convRepo.GetByID(ctx, orgID, convID)
	if err != nil {
		return nil, apperr.NotFound("conversation", err)
	}

	if conv.Type != domain.ConvVoice {
		return nil, apperr.NotFound("voice channel", nil)
	}

	// Check permission using the caller's real org role so role rules,
	// everyone rules, and elevated-role immunity resolve correctly.
	perms, err := s.permService.ResolvePermissions(ctx, orgID, convID, userID, callerRole)
	if err != nil {
		return nil, err
	}

	if !perms.CanView {
		return nil, apperr.Forbidden("no permission to join voice channel")
	}

	// Enforce participant cap (practical limit to bound SFU cost).
	if max := s.maxParticipants(); max > 0 {
		// Check existing participant first; a returning user doesn't count
		// against the cap even when full (takeover, not a new slot).
		existing, err := s.participantRepo.Get(ctx, orgID, convID, userID)
		if err != nil || existing == nil {
			count, err := s.participantRepo.Count(ctx, orgID, convID)
			if err != nil {
				return nil, apperr.Internal("failed to count voice participants", err)
			}
			if count >= max {
				return nil, apperr.Conflict("voice channel is full")
			}
		}
	}

	// Tab takeover / idempotent rejoin:
	// - Same connection rejoining (duplicate voice_join) → return current
	//   state without recreating the publisher (idempotent).
	// - Different connection (another tab) → tear down the old SFU session,
	//   reassign the DB row to the new connection, and kick the old tab.
	oldConnID := ""
	if existing, err := s.participantRepo.Get(ctx, orgID, convID, userID); err == nil && existing != nil {
		if existing.ConnectionID == connID {
			// Same tab rejoining; return current state, no SFU churn.
			result, err := s.buildJoinResult(ctx, orgID, convID, userID)
			if err != nil {
				return nil, err
			}
			return result, nil
		}
		oldConnID = existing.ConnectionID
		// Remove the old SFU participant (closes its peer connections).
		if err := s.sfu.RemoveParticipant(ctx, userID, convID); err != nil {
			s.log.Warn("failed to remove old SFU participant during takeover", "error", err, "user_id", userID, "conv_id", convID)
		}
		// Reassign the DB row to the new connection.
		if err := s.participantRepo.UpdateConnection(ctx, orgID, convID, userID, connID); err != nil {
			s.log.Error("failed to reassign voice connection during takeover", "error", err)
		}
		// Reset flags on takeover.
		if err := s.participantRepo.UpdateFlags(ctx, orgID, convID, userID, false, false); err != nil {
			s.log.Warn("failed to reset voice flags during takeover", "error", err)
		}
	}

	// Create publisher in SFU. If a stale SFU participant lingered (race
	// with takeover), CreatePublisher returns errParticipantExists; in that
	// case force-remove and retry once.
	sdp, err := s.sfu.CreatePublisher(ctx, orgID, userID, connID, convID)
	if err != nil {
		if errors.Is(err, voice.ErrParticipantExists) {
			_ = s.sfu.RemoveParticipant(ctx, userID, convID)
			sdp, err = s.sfu.CreatePublisher(ctx, orgID, userID, connID, convID)
		}
		if err != nil {
			return nil, apperr.Internal("failed to create publisher", err)
		}
	}

	// Insert DB row if this is a fresh join (not a takeover).
	if oldConnID == "" {
		p := &domain.VoiceParticipant{
			ID:             uuid.New().String(),
			ConversationID: convID,
			OrgID:          orgID,
			UserID:         userID,
			ConnectionID:   connID,
		}
		if err := s.participantRepo.Join(ctx, p); err != nil {
			// Rollback SFU
			_ = s.sfu.RemoveParticipant(ctx, userID, convID)
			return nil, apperr.Internal("failed to join voice channel", err)
		}
	}

	// Build result
	result, err := s.buildJoinResult(ctx, orgID, convID, userID)
	if err != nil {
		return nil, err
	}

	// Include the offer SDP
	result.SDPOffer = sdp

	// Kick the old connection so it resets (it was taken over).
	if oldConnID != "" {
		s.broadcaster.Broadcast(
			domain.RoomKeyConnection(orgID, oldConnID),
			string(domain.WsTypeVoiceKick),
			map[string]any{
				"conversation_id": convID,
				"reason":          "taken_over",
			},
		)
	}

	// Broadcast state update
	s.broadcastVoiceStateUpdate(orgID, convID)

	s.log.Info("user joined voice channel", "user_id", userID, "conv_id", convID, "conn_id", connID, "takeover", oldConnID != "")

	return result, nil
}

// Leave removes a user from a voice channel.
func (s *voiceService) Leave(ctx context.Context, orgID, userID, convID string) error {
	mu := s.lockJoin(orgID, convID, userID)
	mu.Lock()
	defer mu.Unlock()

	return s.leaveLocked(ctx, orgID, userID, convID)
}

// leaveLocked performs the teardown. Callers must hold the join mutex.
func (s *voiceService) leaveLocked(ctx context.Context, orgID, userID, convID string) error {
	// Guard: only tear down when this user actually holds a session here.
	// A client can send voice_leave for arbitrary conversation IDs; without
	// this check each one triggers SFU churn and an org-wide state broadcast.
	existing, err := s.participantRepo.Get(ctx, orgID, convID, userID)
	if err != nil || existing == nil {
		return nil
	}

	// Remove from database
	if err := s.participantRepo.Leave(ctx, orgID, convID, userID); err != nil {
		s.log.Error("failed to leave voice channel", "error", err, "user_id", userID, "conv_id", convID)
	}

	// Remove from SFU
	if err := s.sfu.RemoveParticipant(ctx, userID, convID); err != nil {
		s.log.Error("failed to remove participant from SFU", "error", err, "user_id", userID, "conv_id", convID)
	}

	// Clear ephemeral speaking state
	s.clearSpeakingState(convID, userID)

	// Broadcast state update
	s.broadcastVoiceStateUpdate(orgID, convID)

	s.log.Info("user left voice channel", "user_id", userID, "conv_id", convID)

	return nil
}

// LeaveByConnection removes all voice sessions owned by a specific WS
// connection. Called on WS disconnect so a closed tab exits voice without
// affecting the user's other tabs (which own different connections).
func (s *voiceService) LeaveByConnection(ctx context.Context, orgID, userID, connID string) error {
	participants, err := s.participantRepo.ListActiveVoiceForUser(ctx, orgID, userID)
	if err != nil {
		return err
	}
	for _, p := range participants {
		if p.OrgID != orgID || p.ConnectionID != connID {
			continue
		}
		if err := s.Leave(ctx, orgID, p.UserID, p.ConversationID); err != nil {
			s.log.Error("failed to leave voice channel on disconnect", "error", err, "user_id", p.UserID, "conv_id", p.ConversationID)
		}
	}
	return nil
}

// SetMute sets the muted state for a participant.
// Preserves the deafened flag; toggling mute doesn't affect deafen state.
func (s *voiceService) SetMute(ctx context.Context, orgID, userID, convID string, muted bool) error {
	// Get current state to preserve the deafened flag
	p, err := s.participantRepo.Get(ctx, orgID, convID, userID)
	if err != nil {
		return apperr.NotFound("participant", err)
	}

	// Update only the muted flag
	if err := s.participantRepo.UpdateFlags(ctx, orgID, convID, userID, muted, p.Deafened); err != nil {
		return apperr.Internal("failed to update mute state", err)
	}

	// Update SFU
	if err := s.sfu.SetMuted(ctx, userID, convID, muted); err != nil {
		return apperr.Internal("failed to set muted in SFU", err)
	}

	// Broadcast mute event
	s.broadcaster.Broadcast(
		domain.RoomKeyConversation(orgID, convID),
		string(domain.WsTypeVoiceMute),
		map[string]any{
			"conversation_id": convID,
			"user_id":         userID,
			"muted":           muted,
		},
	)

	return nil
}

// SetDeafen sets the deafened state for a participant.
// Deafening implies muting the user.
func (s *voiceService) SetDeafen(ctx context.Context, orgID, userID, convID string, deafened bool) error {
	// Get current state to determine mute change
	p, err := s.participantRepo.Get(ctx, orgID, convID, userID)
	if err != nil {
		return apperr.NotFound("participant", err)
	}

	// When deafening, also mute the user; when undeafening, keep current mute state
	newMuted := p.Muted
	if deafened {
		newMuted = true
	}

	// Update both flags
	if err := s.participantRepo.UpdateFlags(ctx, orgID, convID, userID, newMuted, deafened); err != nil {
		return apperr.Internal("failed to update deafen state", err)
	}

	// Update SFU muted state
	if err := s.sfu.SetMuted(ctx, userID, convID, newMuted); err != nil {
		return apperr.Internal("failed to set muted in SFU", err)
	}

	// Broadcast mute event (deafen implies mute)
	s.broadcaster.Broadcast(
		domain.RoomKeyConversation(orgID, convID),
		string(domain.WsTypeVoiceMute),
		map[string]any{
			"conversation_id": convID,
			"user_id":         userID,
			"muted":           newMuted,
		},
	)

	// Broadcast deafen event
	s.broadcaster.Broadcast(
		domain.RoomKeyConversation(orgID, convID),
		string(domain.WsTypeVoiceDeafen),
		map[string]any{
			"conversation_id": convID,
			"user_id":         userID,
			"deafened":        deafened,
		},
	)

	return nil
}

// Kick removes a user from a voice channel (admin only).
func (s *voiceService) Kick(ctx context.Context, orgID, callerUserID string, callerRole domain.Role, convID, targetUserID string) error {
	// Check if caller has permission
	perms, err := s.permService.ResolvePermissions(ctx, orgID, convID, callerUserID, callerRole)
	if err != nil {
		return err
	}

	if !perms.CanManage {
		return apperr.Forbidden("no permission to kick users")
	}

	// Get the target's owning connection so we can kick just that tab.
	target, err := s.participantRepo.Get(ctx, orgID, convID, targetUserID)
	targetConnID := ""
	if err == nil && target != nil {
		targetConnID = target.ConnectionID
	}

	// Kick the target
	if err := s.Leave(ctx, orgID, targetUserID, convID); err != nil {
		return err
	}

	// Send kick event to the target's connection (not all their tabs).
	roomKey := domain.RoomKeyUser(orgID, targetUserID)
	if targetConnID != "" {
		roomKey = domain.RoomKeyConnection(orgID, targetConnID)
	}
	s.broadcaster.Broadcast(
		roomKey,
		string(domain.WsTypeVoiceKick),
		map[string]any{
			"conversation_id": convID,
		},
	)

	s.log.Info("user kicked from voice channel", "caller", callerUserID, "target", targetUserID, "conv_id", convID)

	return nil
}

// ListParticipants returns all participants in a voice channel.
func (s *voiceService) ListParticipants(ctx context.Context, orgID, convID string) ([]domain.VoiceParticipantInfo, error) {
	infos, err := s.participantRepo.ListByConversationWithUser(ctx, orgID, convID)
	if err != nil {
		return nil, apperr.Internal("failed to list participants", err)
	}

	// Fill in ephemeral speaking state from the in-memory cache.
	for i := range infos {
		infos[i].Speaking = s.getSpeakingState(convID, infos[i].UserID)
	}

	return infos, nil
}

// HandleSignal handles WebRTC signaling messages.
//
// connID is the WS connection the signal arrived on. It must match the
// ConnectionID of the active voice participant for (userID, convID): after a
// tab takeover the old connection is no longer the active session, and
// accepting its signaling would disrupt the new tab's peer connection. The
// caller must already be a participant of the voice channel; this also
// prevents a client from injecting SDP/ICE into rooms they never joined.
func (s *voiceService) HandleSignal(ctx context.Context, orgID, userID, connID, convID string, msg domain.VoiceSignalMsg) error {
	p, err := s.participantRepo.Get(ctx, orgID, convID, userID)
	if err != nil {
		return apperr.Forbidden("not a participant of this voice channel")
	}
	// Reject signals from a connection that no longer owns the voice session
	// (stale tab after takeover). connID is empty only for non-WS callers
	// (e.g. tests), which are allowed through to preserve the participant check.
	if connID != "" && p.ConnectionID != "" && p.ConnectionID != connID {
		return apperr.Forbidden("voice session owned by another connection")
	}

	switch msg.Type {
	case "answer":
		return s.sfu.HandleAnswer(ctx, userID, convID, msg.SDP)
	case "subscriber_answer":
		return s.sfu.HandleSubscriberAnswer(ctx, userID, msg.TargetUserID, convID, msg.SDP)
	case "ice":
		if msg.TargetUserID != "" {
			return s.sfu.HandleSubscriberICECandidate(ctx, userID, convID, msg.TargetUserID, msg.Candidate)
		}
		return s.sfu.HandleICECandidate(ctx, userID, convID, msg.Candidate)
	default:
		return apperr.InvalidInput("unknown signal type")
	}
}

// handleSpeaking is called by the SFU when a participant starts/stops speaking.
// Uses context.Background() because this is an event-driven callback from the
// SFU (not an HTTP request handler); there's no request context to propagate.
// handleSpeaking is invoked by the SFU's audio-level detector; synchronously
// under the detector lock inside the RTP track-reader loop. It must not do
// I/O: enqueue for the dispatcher instead.
func (s *voiceService) handleSpeaking(userID, orgID string, speaking bool) {
	select {
	case s.speakingEvents <- domain.SpeakingEvent{UserID: userID, OrgID: orgID, Speaking: speaking}:
	default:
		// Queue full: drop rather than block media processing. The next
		// transition re-establishes correct state; speaking indicators are
		// ephemeral UI hints, not durable data.
	}
}

// dispatchSpeakingEvents processes queued speaking transitions in order on a
// background goroutine, doing the DB lookup and broadcast off the reader loop.
func (s *voiceService) dispatchSpeakingEvents() {
	ctx := context.Background()
	for ev := range s.speakingEvents {
		s.processSpeakingEvent(ctx, ev)
	}
}

// drainSpeakingEvents processes every queued transition synchronously.
// Used by tests that build voiceService without the dispatcher goroutine.
func (s *voiceService) drainSpeakingEvents() {
	ctx := context.Background()
	for {
		select {
		case ev := <-s.speakingEvents:
			s.processSpeakingEvent(ctx, ev)
		default:
			return
		}
	}
}

func (s *voiceService) processSpeakingEvent(ctx context.Context, ev domain.SpeakingEvent) {
	participants, err := s.participantRepo.ListActiveVoiceForUser(ctx, ev.OrgID, ev.UserID)
	if err != nil {
		s.log.Error("handleSpeaking: failed to list active voice", "error", err, "user_id", ev.UserID)
		return
	}
	for _, p := range participants {
		// Track ephemeral speaking state so state updates include it.
		s.setSpeakingState(p.ConversationID, ev.UserID, ev.Speaking)
		s.broadcaster.Broadcast(
			domain.RoomKeyConversation(p.OrgID, p.ConversationID),
			string(domain.WsTypeVoiceSpeaking),
			map[string]any{
				"conversation_id": p.ConversationID,
				"user_id":         ev.UserID,
				"speaking":        ev.Speaking,
			},
		)
	}
}

// handleSubscriberOffer is called by the SFU when a subscriber SDP offer is
// created. Routes to the subscriber's owning connection (not all tabs).
func (s *voiceService) handleSubscriberOffer(subscriberID, subscriberConnID, publisherID, convID, orgID, sdp string) {
	roomKey := domain.RoomKeyUser(orgID, subscriberID)
	if subscriberConnID != "" {
		roomKey = domain.RoomKeyConnection(orgID, subscriberConnID)
	}
	s.broadcaster.Broadcast(
		roomKey,
		string(domain.WsTypeVoiceSignal),
		map[string]any{
			"type":            "offer",
			"sdp":             sdp,
			"target_user_id":  publisherID,
			"conversation_id": convID,
		},
	)
}

// handleICECandidate is called by the SFU when a publisher PC generates an ICE
// candidate. Routes to the publisher's owning connection (not all tabs).
func (s *voiceService) handleICECandidate(userID, connID, convID, orgID, candidateJSON string) {
	roomKey := domain.RoomKeyUser(orgID, userID)
	if connID != "" {
		roomKey = domain.RoomKeyConnection(orgID, connID)
	}
	s.broadcaster.Broadcast(
		roomKey,
		string(domain.WsTypeVoiceSignal),
		map[string]any{
			"type":            "ice",
			"candidate":       candidateJSON,
			"conversation_id": convID,
		},
	)
}

// handleSubscriberICECandidate is called by the SFU when a subscriber PC
// generates an ICE candidate. Routes to the subscriber's owning connection.
func (s *voiceService) handleSubscriberICECandidate(subscriberID, subscriberConnID, publisherID, convID, orgID, candidateJSON string) {
	roomKey := domain.RoomKeyUser(orgID, subscriberID)
	if subscriberConnID != "" {
		roomKey = domain.RoomKeyConnection(orgID, subscriberConnID)
	}
	s.broadcaster.Broadcast(
		roomKey,
		string(domain.WsTypeVoiceSignal),
		map[string]any{
			"type":            "ice",
			"candidate":       candidateJSON,
			"target_user_id":  publisherID,
			"conversation_id": convID,
		},
	)
}

// maxParticipants returns the configured per-channel cap from the SFU.
func (s *voiceService) maxParticipants() int {
	if sfu, ok := s.sfu.(interface{ MaxParticipants() int }); ok {
		return sfu.MaxParticipants()
	}
	return 0
}

// buildJoinResult builds the VoiceJoinResult for a user.
// Generates per-user ephemeral TURN credentials when configured.
func (s *voiceService) buildJoinResult(ctx context.Context, orgID, convID, userID string) (*domain.VoiceJoinResult, error) {
	participants, err := s.ListParticipants(ctx, orgID, convID)
	if err != nil {
		return nil, err
	}

	iceServers := s.sfu.ICEServers()
	if sfu, ok := s.sfu.(interface {
		ICEServersForUser(userID string) []domain.ICEServer
	}); ok {
		iceServers = sfu.ICEServersForUser(userID)
	}

	return &domain.VoiceJoinResult{
		Participants:    participants,
		ICEServers:      iceServers,
		MaxParticipants: s.maxParticipants(),
	}, nil
}

// setSpeakingState records ephemeral speaking state for a participant.
func (s *voiceService) setSpeakingState(convID, userID string, speaking bool) {
	key := convID + "|" + userID
	s.speakingStatesMu.Lock()
	if speaking {
		s.speakingStates[key] = true
	} else {
		delete(s.speakingStates, key)
	}
	s.speakingStatesMu.Unlock()
}

// getSpeakingState returns the ephemeral speaking state for a participant.
func (s *voiceService) getSpeakingState(convID, userID string) bool {
	s.speakingStatesMu.RLock()
	defer s.speakingStatesMu.RUnlock()
	return s.speakingStates[convID+"|"+userID]
}

// clearSpeakingState removes ephemeral speaking state (on leave/kick).
func (s *voiceService) clearSpeakingState(convID, userID string) {
	s.speakingStatesMu.Lock()
	delete(s.speakingStates, convID+"|"+userID)
	s.speakingStatesMu.Unlock()
}

// broadcastVoiceStateUpdate broadcasts the current participant list to all users.
// Uses context.Background() because this is an internal broadcast (not request-scoped).
func (s *voiceService) broadcastVoiceStateUpdate(orgID, convID string) {
	ctx := context.Background()
	participants, err := s.ListParticipants(ctx, orgID, convID)
	if err != nil {
		s.log.Error("failed to list participants for broadcast", "error", err)
		return
	}

	payloads := make([]map[string]any, 0, len(participants))
	for _, p := range participants {
		payloads = append(payloads, map[string]any{
			"id":         p.ID,
			"user_id":    p.UserID,
			"name":       p.Name,
			"avatar_url": p.AvatarURL,
			"muted":      p.Muted,
			"deafened":   p.Deafened,
			"speaking":   p.Speaking,
			"joined_at":  p.JoinedAt,
		})
	}

	s.broadcaster.Broadcast(
		domain.RoomKeyConversation(orgID, convID),
		string(domain.WsTypeVoiceStateUpdate),
		map[string]any{
			"conversation_id": convID,
			"participants":    payloads,
		},
	)
}
