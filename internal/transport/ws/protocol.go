package ws

import (
	"encoding/json"

	"ipmanlk/breeze/internal/domain"
)

type wireEnvelope struct {
	Type    string          `json:"type"`
	Payload json.RawMessage `json:"payload"`
}

type pingPayload struct {
	Timestamp int64 `json:"timestamp"`
}

type pongPayload struct {
	Timestamp int64 `json:"timestamp"`
}

type connectedPayload struct {
	UserID    string `json:"user_id"`
	SessionID string `json:"session_id"`
}

type errorPayload struct {
	Code    string `json:"code"`
	Message string `json:"message"`
	ReqType string `json:"req_type,omitempty"`
}

func MarshalEvent(eventType string, payload any) ([]byte, error) {
	raw, err := json.Marshal(payload)
	if err != nil {
		return nil, err
	}
	return json.Marshal(wireEnvelope{Type: eventType, Payload: raw})
}

func UnmarshalWire(data []byte) (t string, raw json.RawMessage, err error) {
	var env wireEnvelope
	if err := json.Unmarshal(data, &env); err != nil {
		return "", nil, err
	}
	return env.Type, env.Payload, nil
}

func ParsePayload[T any](payload json.RawMessage) (T, error) {
	var v T
	if err := json.Unmarshal(payload, &v); err != nil {
		return v, err
	}
	return v, nil
}

func NewConnectedMessage(userID, sessionID string) []byte {
	msg, _ := MarshalEvent(string(domain.WsTypeConnected), connectedPayload{UserID: userID, SessionID: sessionID})
	return msg
}

func NewPongMessage(ts int64) []byte {
	msg, _ := MarshalEvent(string(domain.WsTypePong), pongPayload{Timestamp: ts})
	return msg
}

func NewErrorMessage(code, message, reqType string) []byte {
	msg, _ := MarshalEvent(string(domain.WsTypeError), errorPayload{Code: code, Message: message, ReqType: reqType})
	return msg
}

type conversationSubscribePayload struct {
	ConversationID string `json:"conversation_id"`
}

type conversationUnsubscribePayload struct {
	ConversationID string `json:"conversation_id"`
}

type projectSubscribePayload struct {
	ProjectID string `json:"project_id"`
}

type projectUnsubscribePayload struct {
	ProjectID string `json:"project_id"`
}

type typingStartPayload struct {
	ConversationID string `json:"conversation_id"`
}

type typingStopPayload struct {
	ConversationID string `json:"conversation_id"`
}

// Voice channel payloads
type voiceJoinPayload struct {
	ConversationID string `json:"conversation_id"`
}

type voiceLeavePayload struct {
	ConversationID string `json:"conversation_id"`
}

type voiceSignalPayload struct {
	ConversationID string `json:"conversation_id"`
	Type           string `json:"type"` // "answer", "ice", "subscriber_answer"
	SDP            string `json:"sdp,omitempty"`
	Candidate      string `json:"candidate,omitempty"`
	TargetUserID   string `json:"target_user_id,omitempty"`
}

type voiceMutePayload struct {
	ConversationID string `json:"conversation_id"`
	Muted          bool   `json:"muted"`
}

type voiceDeafenPayload struct {
	ConversationID string `json:"conversation_id"`
	Deafened       bool   `json:"deafened"`
}

// Outbound voice payloads (server → client).

type voiceParticipantPayload struct {
	ID        string `json:"id"`
	UserID    string `json:"user_id"`
	Name      string `json:"name"`
	AvatarURL string `json:"avatar_url,omitempty"`
	Muted     bool   `json:"muted"`
	Deafened  bool   `json:"deafened"`
	Speaking  bool   `json:"speaking"`
	JoinedAt  string `json:"joined_at"`
}

type voiceJoinResultPayload struct {
	Participants    []voiceParticipantPayload `json:"participants"`
	ICEServers      []voiceICEServerPayload   `json:"ice_servers"`
	SDPOffer        string                    `json:"sdp_offer,omitempty"`
	MaxParticipants int                       `json:"max_participants,omitempty"`
}

type voiceICEServerPayload struct {
	URLs       []string `json:"urls"`
	Username   string   `json:"username,omitempty"`
	Credential string   `json:"credential,omitempty"`
}

type voiceStateUpdatePayload struct {
	ConversationID string                    `json:"conversation_id"`
	Participants   []voiceParticipantPayload `json:"participants"`
}

type voiceSpeakingPayload struct {
	ConversationID string `json:"conversation_id"`
	UserID         string `json:"user_id"`
	Speaking       bool   `json:"speaking"`
}

type voiceMuteEventPayload struct {
	ConversationID string `json:"conversation_id"`
	UserID         string `json:"user_id"`
	Muted          bool   `json:"muted"`
}

type voiceDeafenEventPayload struct {
	ConversationID string `json:"conversation_id"`
	UserID         string `json:"user_id"`
	Deafened       bool   `json:"deafened"`
}

type voiceKickPayload struct {
	ConversationID string `json:"conversation_id"`
}

// voiceSignalOutbound is a server→client WebRTC signaling message.
type voiceSignalOutbound struct {
	ConversationID string `json:"conversation_id"`
	Type           string `json:"type"` // "offer", "ice"
	SDP            string `json:"sdp,omitempty"`
	Candidate      string `json:"candidate,omitempty"`
	TargetUserID   string `json:"target_user_id,omitempty"`
}

// NewVoiceJoinResultMessage builds a voice_join_result WS event from domain types.
func NewVoiceJoinResultMessage(participants []voiceParticipantPayload, iceServers []voiceICEServerPayload, sdpOffer string, maxParticipants int) []byte {
	msg, _ := MarshalEvent(string(domain.WsTypeVoiceJoinResult), voiceJoinResultPayload{
		Participants:    participants,
		ICEServers:      iceServers,
		SDPOffer:        sdpOffer,
		MaxParticipants: maxParticipants,
	})
	return msg
}

// NewVoiceStateUpdateMessage builds a voice_state_update WS event.
func NewVoiceStateUpdateMessage(convID string, participants []voiceParticipantPayload) []byte {
	msg, _ := MarshalEvent(string(domain.WsTypeVoiceStateUpdate), voiceStateUpdatePayload{
		ConversationID: convID,
		Participants:   participants,
	})
	return msg
}

// NewVoiceSpeakingMessage builds a voice_speaking WS event.
func NewVoiceSpeakingMessage(convID, userID string, speaking bool) []byte {
	msg, _ := MarshalEvent(string(domain.WsTypeVoiceSpeaking), voiceSpeakingPayload{
		ConversationID: convID,
		UserID:         userID,
		Speaking:       speaking,
	})
	return msg
}

// NewVoiceMuteMessage builds a voice_mute WS event.
func NewVoiceMuteMessage(convID, userID string, muted bool) []byte {
	msg, _ := MarshalEvent(string(domain.WsTypeVoiceMute), voiceMuteEventPayload{
		ConversationID: convID,
		UserID:         userID,
		Muted:          muted,
	})
	return msg
}

// NewVoiceDeafenMessage builds a voice_deafen WS event.
func NewVoiceDeafenMessage(convID, userID string, deafened bool) []byte {
	msg, _ := MarshalEvent(string(domain.WsTypeVoiceDeafen), voiceDeafenEventPayload{
		ConversationID: convID,
		UserID:         userID,
		Deafened:       deafened,
	})
	return msg
}

// NewVoiceKickMessage builds a voice_kick WS event.
func NewVoiceKickMessage(convID string) []byte {
	msg, _ := MarshalEvent(string(domain.WsTypeVoiceKick), voiceKickPayload{
		ConversationID: convID,
	})
	return msg
}

// --- Comment event wire types ---

func NewVoiceSignalOutboundMessage(convID, signalType, sdp, candidate, targetUserID string) []byte {
	msg, _ := MarshalEvent(string(domain.WsTypeVoiceSignal), voiceSignalOutbound{
		ConversationID: convID,
		Type:           signalType,
		SDP:            sdp,
		Candidate:      candidate,
		TargetUserID:   targetUserID,
	})
	return msg
}
