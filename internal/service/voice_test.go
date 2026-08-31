package service

import (
	"context"
	"sync"
	"testing"

	"ipmanlk/plume/internal/domain"
)

func newVoiceServiceForTest(t *testing.T) (*voiceService, *mockVoiceParticipantRepo, *mockVoiceSFU, *mockBroadcaster) {
	t.Helper()
	convRepo := newMockConversationRepo()
	convRepo.convsByID["conv-voice"] = &domain.Conversation{
		ID:    "conv-voice",
		OrgID: "org-1",
		Type:  domain.ConvVoice,
		Name:  "General Voice",
	}
	convRepo.convsByID["conv-text"] = &domain.Conversation{
		ID:    "conv-text",
		OrgID: "org-1",
		Type:  domain.ConvChannel,
		Name:  "general",
	}

	userRepo := newMockUserRepo()
	userRepo.usersByID["user-1"] = &domain.User{ID: "user-1", OrgID: "org-1", Name: "Alice"}

	participantRepo := newMockVoiceParticipantRepo()
	participantRepo.userNames["user-1"] = "Alice"
	sfu := newMockVoiceSFU()
	broadcaster := newMockBroadcaster()
	permService := newMockPermService()

	svc := &voiceService{
		participantRepo: participantRepo,
		convRepo:        convRepo,
		userRepo:        userRepo,
		permService:     permService,
		sfu:             sfu,
		broadcaster:     broadcaster,
		log:             testLogger,
		speakingStates:  make(map[string]bool),
		joinMu:          make(map[string]*sync.Mutex),
		speakingEvents:  make(chan domain.SpeakingEvent, speakingEventQueueSize),
	}
	return svc, participantRepo, sfu, broadcaster
}

func TestVoiceService_Join_Success(t *testing.T) {
	svc, repo, sfu, broadcaster := newVoiceServiceForTest(t)

	result, err := svc.Join(context.Background(), "org-1", "user-1", domain.RoleAdmin, "conn-1", "conv-voice")
	if err != nil {
		t.Fatalf("Join failed: %v", err)
	}

	// Participant row inserted
	if len(repo.participants) != 1 {
		t.Errorf("expected 1 participant in repo, got %d", len(repo.participants))
	}

	// SFU CreatePublisher called
	if len(sfu.createPublisherCalls) != 1 {
		t.Errorf("expected 1 CreatePublisher call, got %d", len(sfu.createPublisherCalls))
	}

	// voice_state_update broadcast to conversation room
	foundStateUpdate := false
	for _, msg := range broadcaster.messages {
		if msg.eventType == "voice_state_update" &&
			msg.roomKey == domain.RoomKeyConversation("org-1", "conv-voice") {
			foundStateUpdate = true
		}
	}
	if !foundStateUpdate {
		t.Error("expected voice_state_update broadcast to conversation room")
	}

	// ICE servers populated
	if len(result.ICEServers) == 0 {
		t.Error("expected ICE servers in result")
	}

	// Participants list includes the joiner
	if len(result.Participants) != 1 {
		t.Errorf("expected 1 participant in result, got %d", len(result.Participants))
	}
	if result.Participants[0].Name != "Alice" {
		t.Errorf("expected participant name 'Alice', got '%s'", result.Participants[0].Name)
	}

	// SDP offer included
	if result.SDPOffer == "" {
		t.Error("expected SDP offer in result")
	}
}

func TestVoiceService_Join_AlreadyJoined_Idempotent(t *testing.T) {
	svc, repo, sfu, _ := newVoiceServiceForTest(t)

	// First join
	_, err := svc.Join(context.Background(), "org-1", "user-1", domain.RoleAdmin, "conn-1", "conv-voice")
	if err != nil {
		t.Fatalf("first Join failed: %v", err)
	}

	// Second join should be idempotent
	_, err = svc.Join(context.Background(), "org-1", "user-1", domain.RoleAdmin, "conn-1", "conv-voice")
	if err != nil {
		t.Fatalf("second Join failed: %v", err)
	}

	// Still only 1 participant
	if len(repo.participants) != 1 {
		t.Errorf("expected 1 participant after double join, got %d", len(repo.participants))
	}

	// SFU CreatePublisher called once (not twice)
	if len(sfu.createPublisherCalls) != 1 {
		t.Errorf("expected 1 CreatePublisher call, got %d", len(sfu.createPublisherCalls))
	}
}

func TestVoiceService_Join_TextChannel_Rejected(t *testing.T) {
	svc, repo, sfu, broadcaster := newVoiceServiceForTest(t)

	_, err := svc.Join(context.Background(), "org-1", "user-1", domain.RoleAdmin, "conn-1", "conv-text")
	if err == nil {
		t.Error("expected error when joining a text channel as voice")
	}

	// No DB insert
	if len(repo.participants) != 0 {
		t.Errorf("expected 0 participants, got %d", len(repo.participants))
	}

	// No broadcast
	if len(broadcaster.messages) != 0 {
		t.Errorf("expected 0 broadcasts, got %d", len(broadcaster.messages))
	}

	// No SFU call
	if len(sfu.createPublisherCalls) != 0 {
		t.Errorf("expected 0 SFU calls, got %d", len(sfu.createPublisherCalls))
	}
}

func TestVoiceService_Join_Forbidden_NoViewPermission(t *testing.T) {
	svc, repo, _, broadcaster := newVoiceServiceForTest(t)

	// Override perm service to deny view
	svc.permService = &mockPermService{
		resolvePermissionsFn: func(ctx context.Context, orgID, channelID, userID string, userRole domain.Role) (*domain.ChannelPermissions, error) {
			return &domain.ChannelPermissions{CanView: false}, nil
		},
	}

	_, err := svc.Join(context.Background(), "org-1", "user-1", domain.RoleAdmin, "conn-1", "conv-voice")
	if err == nil {
		t.Error("expected error when joining without view permission")
	}

	// No DB insert
	if len(repo.participants) != 0 {
		t.Errorf("expected 0 participants, got %d", len(repo.participants))
	}

	// No broadcast
	if len(broadcaster.messages) != 0 {
		t.Errorf("expected 0 broadcasts, got %d", len(broadcaster.messages))
	}
}

func TestVoiceService_Leave_RemovesParticipant_BroadcastsAndTearsDown(t *testing.T) {
	svc, repo, sfu, broadcaster := newVoiceServiceForTest(t)

	// Join first
	_, _ = svc.Join(context.Background(), "org-1", "user-1", domain.RoleAdmin, "conn-1", "conv-voice")
	broadcaster.messages = nil // clear broadcasts

	// Leave
	err := svc.Leave(context.Background(), "org-1", "user-1", "conv-voice")
	if err != nil {
		t.Fatalf("Leave failed: %v", err)
	}

	// Row deleted
	if len(repo.participants) != 0 {
		t.Errorf("expected 0 participants after leave, got %d", len(repo.participants))
	}

	// SFU RemoveParticipant called
	if len(sfu.removeParticipantCalls) != 1 {
		t.Errorf("expected 1 RemoveParticipant call, got %d", len(sfu.removeParticipantCalls))
	}
	if sfu.removeParticipantCalls[0].userID != "user-1" || sfu.removeParticipantCalls[0].convID != "conv-voice" {
		t.Errorf("RemoveParticipant called with wrong args: %+v", sfu.removeParticipantCalls[0])
	}

	// voice_state_update broadcast
	foundStateUpdate := false
	for _, msg := range broadcaster.messages {
		if msg.eventType == "voice_state_update" {
			foundStateUpdate = true
		}
	}
	if !foundStateUpdate {
		t.Error("expected voice_state_update broadcast after leave")
	}
}

func TestVoiceService_LeaveByConnection_OnDisconnect(t *testing.T) {
	svc, repo, sfu, _ := newVoiceServiceForTest(t)

	// Join two voice channels from the same connection
	convRepo := svc.convRepo.(*mockConversationRepo)
	convRepo.convsByID["conv-voice-2"] = &domain.Conversation{
		ID:    "conv-voice-2",
		OrgID: "org-1",
		Type:  domain.ConvVoice,
		Name:  "Another Voice",
	}

	_, _ = svc.Join(context.Background(), "org-1", "user-1", domain.RoleAdmin, "conn-1", "conv-voice")
	_, _ = svc.Join(context.Background(), "org-1", "user-1", domain.RoleAdmin, "conn-1", "conv-voice-2")

	// LeaveByConnection for conn-1 should remove both
	err := svc.LeaveByConnection(context.Background(), "org-1", "user-1", "conn-1")
	if err != nil {
		t.Fatalf("LeaveByConnection failed: %v", err)
	}

	// Both participants removed
	if len(repo.participants) != 0 {
		t.Errorf("expected 0 participants after LeaveByConnection, got %d", len(repo.participants))
	}

	// SFU RemoveParticipant called twice
	if len(sfu.removeParticipantCalls) != 2 {
		t.Errorf("expected 2 RemoveParticipant calls, got %d", len(sfu.removeParticipantCalls))
	}
}

func TestVoiceService_SetMute_BroadcastsAndUpdatesDB(t *testing.T) {
	svc, repo, sfu, broadcaster := newVoiceServiceForTest(t)

	// Join first
	_, _ = svc.Join(context.Background(), "org-1", "user-1", domain.RoleAdmin, "conn-1", "conv-voice")
	broadcaster.messages = nil

	// Mute
	err := svc.SetMute(context.Background(), "org-1", "user-1", "conv-voice", true)
	if err != nil {
		t.Fatalf("SetMute failed: %v", err)
	}

	// DB row muted=true
	p := repo.participants[key("conv-voice", "user-1")]
	if !p.Muted {
		t.Error("expected muted=true in DB")
	}

	// SFU SetMuted called with true
	if len(sfu.setMutedCalls) != 1 || !sfu.setMutedCalls[0].muted {
		t.Errorf("expected SFU SetMuted(true), got %+v", sfu.setMutedCalls)
	}

	// voice_mute broadcast
	foundMute := false
	for _, msg := range broadcaster.messages {
		if msg.eventType == "voice_mute" {
			foundMute = true
		}
	}
	if !foundMute {
		t.Error("expected voice_mute broadcast")
	}
}

func TestVoiceService_SetDeafen_StopsDownstream_Broadcasts(t *testing.T) {
	svc, repo, _, broadcaster := newVoiceServiceForTest(t)

	// Join first
	_, _ = svc.Join(context.Background(), "org-1", "user-1", domain.RoleAdmin, "conn-1", "conv-voice")
	broadcaster.messages = nil

	// Deafen
	err := svc.SetDeafen(context.Background(), "org-1", "user-1", "conv-voice", true)
	if err != nil {
		t.Fatalf("SetDeafen failed: %v", err)
	}

	// DB row deafened=true
	p := repo.participants[key("conv-voice", "user-1")]
	if !p.Deafened {
		t.Error("expected deafened=true in DB")
	}

	// voice_deafen broadcast
	foundDeafen := false
	for _, msg := range broadcaster.messages {
		if msg.eventType == "voice_deafen" {
			foundDeafen = true
		}
	}
	if !foundDeafen {
		t.Error("expected voice_deafen broadcast")
	}
}

func TestVoiceService_Kick_AdminCanKick(t *testing.T) {
	svc, repo, sfu, broadcaster := newVoiceServiceForTest(t)

	// Override perm service to allow manage
	svc.permService = &mockPermService{
		resolvePermissionsFn: func(ctx context.Context, orgID, channelID, userID string, userRole domain.Role) (*domain.ChannelPermissions, error) {
			return &domain.ChannelPermissions{CanView: true, CanManage: true}, nil
		},
	}

	// Target joins
	_, _ = svc.Join(context.Background(), "org-1", "user-1", domain.RoleAdmin, "conn-1", "conv-voice")
	broadcaster.messages = nil

	// Admin kicks
	err := svc.Kick(context.Background(), "org-1", "admin-1", domain.RoleAdmin, "conv-voice", "user-1")
	if err != nil {
		t.Fatalf("Kick failed: %v", err)
	}

	// Target removed from DB
	if len(repo.participants) != 0 {
		t.Errorf("expected 0 participants after kick, got %d", len(repo.participants))
	}

	// SFU RemoveParticipant called for target
	if len(sfu.removeParticipantCalls) != 1 || sfu.removeParticipantCalls[0].userID != "user-1" {
		t.Errorf("expected RemoveParticipant for user-1, got %+v", sfu.removeParticipantCalls)
	}

	// voice_kick sent to target's connection room (not all tabs)
	foundKick := false
	for _, msg := range broadcaster.messages {
		if msg.eventType == "voice_kick" && msg.roomKey == domain.RoomKeyConnection("org-1", "conn-1") {
			foundKick = true
		}
	}
	if !foundKick {
		t.Error("expected voice_kick broadcast to target connection room")
	}
}

func TestVoiceService_Kick_MemberCannotKick(t *testing.T) {
	svc, repo, _, _ := newVoiceServiceForTest(t)

	// Override perm service to deny manage
	svc.permService = &mockPermService{
		resolvePermissionsFn: func(ctx context.Context, orgID, channelID, userID string, userRole domain.Role) (*domain.ChannelPermissions, error) {
			return &domain.ChannelPermissions{CanView: true, CanManage: false}, nil
		},
	}

	// Target joins
	_, _ = svc.Join(context.Background(), "org-1", "user-1", domain.RoleAdmin, "conn-1", "conv-voice")

	// Member tries to kick
	err := svc.Kick(context.Background(), "org-1", "member-1", domain.RoleMember, "conv-voice", "user-1")
	if err == nil {
		t.Error("expected error when member kicks")
	}

	// Target NOT removed
	if len(repo.participants) != 1 {
		t.Errorf("expected 1 participant (not kicked), got %d", len(repo.participants))
	}
}

func TestVoiceService_HandleSignal_Answer_RoutesToSFU(t *testing.T) {
	svc, repo, sfu, _ := newVoiceServiceForTest(t)

	// The caller must be a participant of the voice channel before signaling.
	repo.participants[key("conv-voice", "user-1")] = &domain.VoiceParticipant{
		ID:             "p1",
		ConversationID: "conv-voice",
		OrgID:          "org-1",
		UserID:         "user-1",
	}

	err := svc.HandleSignal(context.Background(), "org-1", "user-1", "", "conv-voice", domain.VoiceSignalMsg{
		Type: "answer",
		SDP:  "v=0\no=- test",
	})
	if err != nil {
		t.Fatalf("HandleSignal failed: %v", err)
	}

	if len(sfu.handleAnswerCalls) != 1 {
		t.Errorf("expected 1 HandleAnswer call, got %d", len(sfu.handleAnswerCalls))
	}
	if sfu.handleAnswerCalls[0].sdp != "v=0\no=- test" {
		t.Errorf("expected SDP 'v=0\\no=- test', got '%s'", sfu.handleAnswerCalls[0].sdp)
	}
}

func TestVoiceService_HandleSignal_RejectsNonParticipant(t *testing.T) {
	svc, _, sfu, _ := newVoiceServiceForTest(t)

	// user-1 is NOT a participant of conv-voice, so signaling must be
	// rejected before reaching the SFU.
	err := svc.HandleSignal(context.Background(), "org-1", "user-1", "", "conv-voice", domain.VoiceSignalMsg{
		Type: "answer",
		SDP:  "v=0\no=- test",
	})
	if err == nil {
		t.Fatal("expected error for non-participant signal, got nil")
	}
	if len(sfu.handleAnswerCalls) != 0 {
		t.Errorf("expected 0 HandleAnswer calls, got %d", len(sfu.handleAnswerCalls))
	}
}

// TestVoiceService_HandleSignal_RejectsStaleConnection verifies that a signal
// from a connection that no longer owns the voice session is rejected. This
// prevents a stale tab (after takeover) from disrupting the new tab's peer
// connection.
func TestVoiceService_HandleSignal_RejectsStaleConnection(t *testing.T) {
	svc, repo, sfu, _ := newVoiceServiceForTest(t)

	// user-1 joined from conn-active; conn-stale is a takeover victim.
	repo.participants[key("conv-voice", "user-1")] = &domain.VoiceParticipant{
		ID:             "p1",
		ConversationID: "conv-voice",
		OrgID:          "org-1",
		UserID:         "user-1",
		ConnectionID:   "conn-active",
	}

	err := svc.HandleSignal(context.Background(), "org-1", "user-1", "conn-stale", "conv-voice", domain.VoiceSignalMsg{
		Type: "answer",
		SDP:  "v=0\no=- test",
	})
	if err == nil {
		t.Fatal("expected error for stale connection signal, got nil")
	}
	if len(sfu.handleAnswerCalls) != 0 {
		t.Errorf("expected 0 HandleAnswer calls for stale connection, got %d", len(sfu.handleAnswerCalls))
	}
}

// TestVoiceService_HandleSignal_AcceptsActiveConnection verifies that a
// signal from the connection that owns the voice session is accepted.
func TestVoiceService_HandleSignal_AcceptsActiveConnection(t *testing.T) {
	svc, repo, sfu, _ := newVoiceServiceForTest(t)

	repo.participants[key("conv-voice", "user-1")] = &domain.VoiceParticipant{
		ID:             "p1",
		ConversationID: "conv-voice",
		OrgID:          "org-1",
		UserID:         "user-1",
		ConnectionID:   "conn-active",
	}

	err := svc.HandleSignal(context.Background(), "org-1", "user-1", "conn-active", "conv-voice", domain.VoiceSignalMsg{
		Type: "answer",
		SDP:  "v=0\no=- test",
	})
	if err != nil {
		t.Fatalf("HandleSignal failed for active connection: %v", err)
	}
	if len(sfu.handleAnswerCalls) != 1 {
		t.Errorf("expected 1 HandleAnswer call, got %d", len(sfu.handleAnswerCalls))
	}
}

func TestVoiceService_ListParticipants_HydratesUserInfo(t *testing.T) {
	svc, repo, _, _ := newVoiceServiceForTest(t)

	// Add two participants
	repo.participants[key("conv-voice", "user-1")] = &domain.VoiceParticipant{
		ID: "p1", ConversationID: "conv-voice", OrgID: "org-1", UserID: "user-1",
	}
	repo.participants[key("conv-voice", "user-2")] = &domain.VoiceParticipant{
		ID: "p2", ConversationID: "conv-voice", OrgID: "org-1", UserID: "user-2",
	}

	// Set user names for the mock JOIN
	repo.userNames["user-1"] = "Alice"
	repo.userNames["user-2"] = "Bob"

	dtos, err := svc.ListParticipants(context.Background(), "org-1", "conv-voice")
	if err != nil {
		t.Fatalf("ListParticipants failed: %v", err)
	}

	if len(dtos) != 2 {
		t.Fatalf("expected 2 participants, got %d", len(dtos))
	}

	names := map[string]bool{}
	for _, dto := range dtos {
		names[dto.Name] = true
	}
	if !names["Alice"] || !names["Bob"] {
		t.Errorf("expected names Alice and Bob, got %v", names)
	}
}

// TestVoiceService_Join_TabTakeover verifies the multi-tab takeover model:
// when a second connection joins a channel the user is already in, the old
// connection's SFU session is torn down, the DB row is reassigned to the new
// connection, and the old connection receives a voice_kick.
func TestVoiceService_Join_TabTakeover(t *testing.T) {
	svc, repo, sfu, broadcaster := newVoiceServiceForTest(t)

	// Tab 1 joins
	_, err := svc.Join(context.Background(), "org-1", "user-1", domain.RoleAdmin, "conn-1", "conv-voice")
	if err != nil {
		t.Fatalf("first join failed: %v", err)
	}
	broadcaster.messages = nil // clear

	// Tab 2 (different connection) joins the same channel
	_, err = svc.Join(context.Background(), "org-1", "user-1", domain.RoleAdmin, "conn-2", "conv-voice")
	if err != nil {
		t.Fatalf("takeover join failed: %v", err)
	}

	// Still exactly 1 participant row (reassigned, not duplicated)
	if len(repo.participants) != 1 {
		t.Fatalf("expected 1 participant after takeover, got %d", len(repo.participants))
	}
	p := repo.participants[key("conv-voice", "user-1")]
	if p.ConnectionID != "conn-2" {
		t.Errorf("expected connection reassigned to conn-2, got %s", p.ConnectionID)
	}

	// SFU: old participant removed, new publisher created
	if len(sfu.removeParticipantCalls) != 1 || sfu.removeParticipantCalls[0].userID != "user-1" {
		t.Errorf("expected old SFU participant removed, got %+v", sfu.removeParticipantCalls)
	}
	if len(sfu.createPublisherCalls) != 2 {
		t.Errorf("expected 2 CreatePublisher calls (initial + takeover), got %d", len(sfu.createPublisherCalls))
	}
	if sfu.createPublisherCalls[1].connID != "conn-2" {
		t.Errorf("expected takeover publisher on conn-2, got %s", sfu.createPublisherCalls[1].connID)
	}

	// Old connection receives voice_kick
	foundKick := false
	for _, msg := range broadcaster.messages {
		if msg.eventType == "voice_kick" && msg.roomKey == domain.RoomKeyConnection("org-1", "conn-1") {
			foundKick = true
		}
	}
	if !foundKick {
		t.Error("expected voice_kick to old connection conn-1")
	}
}

// TestVoiceService_Join_SameConnectionRejoin_Idempotent verifies that a
// duplicate voice_join from the same connection (e.g. client retry) does NOT
// tear down and recreate the SFU session.
func TestVoiceService_Join_SameConnectionRejoin_Idempotent(t *testing.T) {
	svc, repo, sfu, _ := newVoiceServiceForTest(t)

	_, err := svc.Join(context.Background(), "org-1", "user-1", domain.RoleAdmin, "conn-1", "conv-voice")
	if err != nil {
		t.Fatalf("first join failed: %v", err)
	}
	firstPublisherCalls := len(sfu.createPublisherCalls)

	// Same connection joins again
	_, err = svc.Join(context.Background(), "org-1", "user-1", domain.RoleAdmin, "conn-1", "conv-voice")
	if err != nil {
		t.Fatalf("rejoin failed: %v", err)
	}

	// No additional CreatePublisher or RemoveParticipant calls
	if len(sfu.createPublisherCalls) != firstPublisherCalls {
		t.Errorf("expected no new CreatePublisher on same-conn rejoin, got %d", len(sfu.createPublisherCalls)-firstPublisherCalls)
	}
	if len(sfu.removeParticipantCalls) != 0 {
		t.Errorf("expected no RemoveParticipant on same-conn rejoin, got %d", len(sfu.removeParticipantCalls))
	}
	if len(repo.participants) != 1 {
		t.Errorf("expected 1 participant, got %d", len(repo.participants))
	}
}

// TestVoiceService_Join_ParticipantCapEnforced verifies that joining beyond
// the configured max is rejected, and that a returning user (takeover) is
// allowed even when the channel is full.
func TestVoiceService_Join_ParticipantCapEnforced(t *testing.T) {
	svc, repo, sfu, _ := newVoiceServiceForTest(t)
	// Lower the cap to 2 for this test
	sfu.maxParticipants = 2

	// Two different users join
	svc.userRepo.(*mockUserRepo).usersByID["user-2"] = &domain.User{ID: "user-2", OrgID: "org-1", Name: "Bob"}
	if _, err := svc.Join(context.Background(), "org-1", "user-1", domain.RoleAdmin, "conn-1", "conv-voice"); err != nil {
		t.Fatalf("user-1 join failed: %v", err)
	}
	if _, err := svc.Join(context.Background(), "org-1", "user-2", domain.RoleAdmin, "conn-2", "conv-voice"); err != nil {
		t.Fatalf("user-2 join failed: %v", err)
	}

	// Third user should be rejected
	svc.userRepo.(*mockUserRepo).usersByID["user-3"] = &domain.User{ID: "user-3", OrgID: "org-1", Name: "Carol"}
	_, err := svc.Join(context.Background(), "org-1", "user-3", domain.RoleAdmin, "conn-3", "conv-voice")
	if err == nil {
		t.Fatal("expected error when channel is full, got nil")
	}

	// user-1's second tab (takeover) should still succeed when full
	_, err = svc.Join(context.Background(), "org-1", "user-1", domain.RoleAdmin, "conn-1b", "conv-voice")
	if err != nil {
		t.Fatalf("takeover should succeed when full: %v", err)
	}

	if len(repo.participants) != 2 {
		t.Errorf("expected 2 participants after cap+takeover, got %d", len(repo.participants))
	}
}

// TestVoiceService_LeaveByConnection_DoesNotAffectOtherTabs verifies that
// disconnecting one tab leaves the user's voice sessions on other tabs intact.
func TestVoiceService_LeaveByConnection_DoesNotAffectOtherTabs(t *testing.T) {
	svc, repo, _, _ := newVoiceServiceForTest(t)

	// conn-1 joins a channel
	_, _ = svc.Join(context.Background(), "org-1", "user-1", domain.RoleAdmin, "conn-1", "conv-voice")
	// Takeover: conn-2 takes over the same channel
	_, _ = svc.Join(context.Background(), "org-1", "user-1", domain.RoleAdmin, "conn-2", "conv-voice")

	// conn-1 disconnects (stale tab finally closing)
	if err := svc.LeaveByConnection(context.Background(), "org-1", "user-1", "conn-1"); err != nil {
		t.Fatalf("LeaveByConnection conn-1 failed: %v", err)
	}

	// conn-2's session survives (the active connection)
	if len(repo.participants) != 1 {
		t.Fatalf("expected conn-2 session to survive, got %d participants", len(repo.participants))
	}
	p := repo.participants[key("conv-voice", "user-1")]
	if p.ConnectionID != "conn-2" {
		t.Errorf("expected surviving session on conn-2, got %s", p.ConnectionID)
	}
}

// TestVoiceService_Join_EphemeralTurnCredentials verifies that when the SFU
// provides per-user ICE servers (ephemeral TURN), they are included in the
// join result instead of the static list.
func TestVoiceService_Join_EphemeralTurnCredentials(t *testing.T) {
	svc, _, sfu, _ := newVoiceServiceForTest(t)
	// Inject a per-user ICE server generator that returns a TURN server with
	// per-user credentials (simulating the RFC 5766 REST API).
	sfu.iceServersForUserFn = func(userID string) []domain.ICEServer {
		return []domain.ICEServer{
			{URLs: []string{"stun:stun.example.com:19302"}},
			{URLs: []string{"turn:turn.example.com:3478"}, Username: "1234:" + userID, Credential: "ephemeral-secret"},
		}
	}

	result, err := svc.Join(context.Background(), "org-1", "user-1", domain.RoleAdmin, "conn-1", "conv-voice")
	if err != nil {
		t.Fatalf("Join failed: %v", err)
	}
	if len(result.ICEServers) != 2 {
		t.Fatalf("expected 2 ICE servers, got %d", len(result.ICEServers))
	}
	var turn domain.ICEServer
	for _, s := range result.ICEServers {
		if len(s.URLs) > 0 && s.URLs[0] == "turn:turn.example.com:3478" {
			turn = s
		}
	}
	if turn.Username == "" {
		t.Fatal("expected ephemeral TURN username in join result")
	}
	if turn.Username != "1234:user-1" {
		t.Errorf("expected per-user TURN username '1234:user-1', got %q", turn.Username)
	}
	if turn.Credential != "ephemeral-secret" {
		t.Errorf("expected ephemeral TURN credential, got %q", turn.Credential)
	}
}

func TestVoiceService_SpeakingState_TrackedAndClearedOnLeave(t *testing.T) {
	svc, repo, _, _ := newVoiceServiceForTest(t)

	// Join
	_, err := svc.Join(context.Background(), "org-1", "user-1", domain.RoleAdmin, "conn-1", "conv-voice")
	if err != nil {
		t.Fatalf("Join failed: %v", err)
	}

	// Simulate speaking (hand-built svc has no dispatcher goroutine;
	// drain the queue synchronously to emulate it)
	svc.handleSpeaking("user-1", "org-1", true)
	svc.drainSpeakingEvents()
	if !svc.getSpeakingState("conv-voice", "user-1") {
		t.Error("expected speaking state to be tracked as true")
	}

	// Simulate stop speaking
	svc.handleSpeaking("user-1", "org-1", false)
	svc.drainSpeakingEvents()
	if svc.getSpeakingState("conv-voice", "user-1") {
		t.Error("expected speaking state to be cleared after stop")
	}

	// Speaking again, then leave should clear it
	svc.handleSpeaking("user-1", "org-1", true)
	svc.drainSpeakingEvents()
	if !svc.getSpeakingState("conv-voice", "user-1") {
		t.Error("expected speaking state to be true before leave")
	}
	if err := svc.Leave(context.Background(), "org-1", "user-1", "conv-voice"); err != nil {
		t.Fatalf("Leave failed: %v", err)
	}
	if svc.getSpeakingState("conv-voice", "user-1") {
		t.Error("expected speaking state to be cleared after leave")
	}

	_ = repo // keep linter happy
}

// TestVoiceService_Join_RealPermissionResolution verifies Join against the
// real ChannelPermissionService (not the always-allow mock): the caller's org
// role must drive the resolution, so an owner can join a rule-free channel
// and a guest is denied by the role fallback.
func TestVoiceService_Join_RealPermissionResolution(t *testing.T) {
	ctx := context.Background()
	convRepo := newMockConversationRepo()
	convRepo.convsByID["conv-voice"] = &domain.Conversation{
		ID: "conv-voice", OrgID: "org-1", Type: domain.ConvVoice, Name: "General Voice",
	}
	userRepo := newMockUserRepo()
	userRepo.usersByID["user-owner"] = &domain.User{ID: "user-owner", OrgID: "org-1", Name: "Owner"}
	participantRepo := newMockVoiceParticipantRepo()
	participantRepo.userNames["user-owner"] = "Owner"
	participantRepo.userNames["user-guest"] = "Guest"

	permSvc := NewChannelPermissionService(newMockPermRepo(), convRepo, newMockLinkRepo(), userRepo)
	svc := &voiceService{
		participantRepo: participantRepo,
		convRepo:        convRepo,
		userRepo:        userRepo,
		permService:     permSvc,
		sfu:             newMockVoiceSFU(),
		broadcaster:     newMockBroadcaster(),
		log:             testLogger,
		speakingStates:  make(map[string]bool),
		joinMu:          make(map[string]*sync.Mutex),
		speakingEvents:  make(chan domain.SpeakingEvent, speakingEventQueueSize),
	}

	t.Run("owner joins with no explicit rules", func(t *testing.T) {
		if _, err := svc.Join(ctx, "org-1", "user-owner", domain.RoleOwner, "conn-1", "conv-voice"); err != nil {
			t.Fatalf("owner join failed: %v", err)
		}
	})

	t.Run("guest is denied by role fallback", func(t *testing.T) {
		if _, err := svc.Join(ctx, "org-1", "user-guest", domain.RoleGuest, "conn-2", "conv-voice"); err == nil {
			t.Fatal("expected guest join to be denied without explicit channel rules")
		}
	})
}

func (m *mockVoiceParticipantRepo) DeleteAll(ctx context.Context) (int64, error) {
	return 0, nil
}
