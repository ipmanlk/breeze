package domain

import "time"

// VoiceParticipant represents a user in a voice channel (DB entity).
type VoiceParticipant struct {
	ID             string
	ConversationID string
	OrgID          string
	UserID         string
	Muted          bool
	Deafened       bool
	Speaking       bool   // ephemeral, not persisted
	ConnectionID   string // WS connection that owns this voice session
	JoinedAt       time.Time
}

// VoiceParticipantInfo is a VoiceParticipant hydrated with user display info.
// Built by the service layer for use by transport (HTTP response / WS broadcast).
type VoiceParticipantInfo struct {
	ID        string
	UserID    string
	Name      string
	AvatarURL string
	Muted     bool
	Deafened  bool
	Speaking  bool
	JoinedAt  time.Time
}

// VoiceJoinResult is the service-layer result of joining a voice channel.
// Contains the current participants, ICE server config, and the publisher SDP offer.
type VoiceJoinResult struct {
	Participants    []VoiceParticipantInfo
	ICEServers      []ICEServer
	SDPOffer        string
	MaxParticipants int
}

// ICEServer represents a STUN/TURN server configuration.
type ICEServer struct {
	URLs       []string
	Username   string
	Credential string
}

// VoiceSignalMsg carries a WebRTC signaling message (answer, ICE candidate,
// or subscriber answer). Used as a domain-level parameter to VoiceService.HandleSignal.
type VoiceSignalMsg struct {
	Type         string // "answer", "ice", "subscriber_answer"
	SDP          string
	Candidate    string
	TargetUserID string
}

// SpeakingEvent is an audio-level speaking transition emitted by the SFU's
// detector. Queued internally by VoiceService so the RTP reader loop never
// blocks on DB I/O or broadcasts.
type SpeakingEvent struct {
	OrgID    string
	UserID   string
	Speaking bool
}
