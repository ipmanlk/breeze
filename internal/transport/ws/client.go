package ws

import (
	"context"
	"encoding/json"
	"log/slog"
	"sync"
	"time"

	"github.com/coder/websocket"
	"github.com/google/uuid"

	"ipmanlk/breeze/internal/domain"
	"ipmanlk/breeze/internal/port"
)

const (
	writeTimeout   = 10 * time.Second
	readTimeout    = 5 * time.Minute
	sendBufferSize = 256
	wsReadLimit    = 32768
	wsPingInterval = 30 * time.Second
	// How often a live connection re-checks that its session is still
	// active, the user is still active, and their role hasn't changed.
	// Role changes revoke HTTP sessions, but nothing else reaches an
	// already-upgraded socket; without this, a demoted or deactivated user
	// keeps receiving events until they disconnect.
	sessionRevalidateInterval = time.Minute
)

// SessionValidator re-checks that a live connection's session is still
// valid. Implemented by AuthService.ValidateSessionByID.
type SessionValidator interface {
	ValidateSessionByID(ctx context.Context, sessionID string) (*domain.Session, error)
}

type Client struct {
	hub        *Hub
	conn       *websocket.Conn
	userID     string
	orgID      string
	sessionID  string
	connID     string // unique per WS connection (for voice signaling routing)
	send       chan []byte
	log        *slog.Logger
	voiceSvc   port.VoiceService
	accessChk  RoomAccessChecker
	sessionVal SessionValidator
	orgRole    domain.Role
	ctx        context.Context
	ctxCancel  context.CancelFunc
	closeOnce  sync.Once

	// typingMu guards lastTyping. typingDebounce is the minimum interval
	// between re-broadcasting typing_start for the same conversation; 0 = no
	// debounce. Prevents N x M broadcast amplification from chatty clients.
	typingMu       sync.Mutex
	lastTyping     map[string]time.Time
	typingDebounce time.Duration
}

func NewClient(hub *Hub, conn *websocket.Conn, userID, orgID, sessionID string, log *slog.Logger, voiceSvc port.VoiceService) *Client {
	ctx, cancel := context.WithCancel(context.Background())
	return &Client{
		hub:        hub,
		conn:       conn,
		userID:     userID,
		orgID:      orgID,
		sessionID:  sessionID,
		connID:     uuid.NewString(),
		send:       make(chan []byte, sendBufferSize),
		log:        log,
		voiceSvc:   voiceSvc,
		ctx:        ctx,
		ctxCancel:  cancel,
		lastTyping: make(map[string]time.Time),
	}
}

// SetAccessChecker injects the room-access checker used to authorize
// client-driven subscribe/typing messages. When set, the client refuses to
// subscribe to a conversation/project room the user has no access to.
func (c *Client) SetAccessChecker(chk RoomAccessChecker, orgRole domain.Role) {
	c.accessChk = chk
	c.orgRole = orgRole
}

// SetSessionValidator injects the periodic session re-checker. When set, the
// client closes itself if its session is revoked, the user is deactivated,
// or their role changed.
func (c *Client) SetSessionValidator(v SessionValidator) {
	c.sessionVal = v
}

// revalidateLoop periodically re-checks the session behind this connection
// and closes the socket when it is no longer valid. Runs until the client
// context is cancelled (i.e. the connection ends for any reason).
func (c *Client) revalidateLoop() {
	ticker := time.NewTicker(sessionRevalidateInterval)
	defer ticker.Stop()
	for {
		select {
		case <-c.ctx.Done():
			return
		case <-ticker.C:
			if _, err := c.sessionVal.ValidateSessionByID(context.Background(), c.sessionID); err != nil {
				c.log.Info("closing websocket: session no longer valid", "user_id", c.userID, "error", err)
				c.conn.Close(websocket.StatusPolicyViolation, "session invalid")
				return
			}
		}
	}
}

// SetTypingDebounce sets the minimum interval between re-broadcasting
// typing_start for the same conversation. 0 disables debouncing (every
// typing_start is broadcast). Configured from WS_TYPING_DEBOUNCE.
func (c *Client) SetTypingDebounce(d time.Duration) {
	c.typingDebounce = d
}

// shouldBroadcastTyping reports whether a typing_start broadcast for the
// given conversation should proceed under the debounce window. The first
// event always proceeds; subsequent events within the window are dropped.
func (c *Client) shouldBroadcastTyping(conversationID string) bool {
	if c.typingDebounce <= 0 {
		return true
	}
	c.typingMu.Lock()
	defer c.typingMu.Unlock()
	if c.lastTyping == nil {
		c.lastTyping = make(map[string]time.Time)
	}
	now := time.Now()
	if last, ok := c.lastTyping[conversationID]; ok && now.Sub(last) < c.typingDebounce {
		return false
	}
	c.lastTyping[conversationID] = now
	return true
}

// ConnID returns the unique connection ID (for voice signaling routing).
func (c *Client) ConnID() string { return c.connID }

// closeSend closes the client's send channel safely, using sync.Once to
// ensure it is closed at most once. This prevents a panic when both the
// unregister and disconnect hub paths fire for the same client.
func (c *Client) closeSend() {
	c.closeOnce.Do(func() {
		close(c.send)
	})
}

func (c *Client) ReadPump() {
	defer func() {
		c.ctxCancel()
		c.conn.Close(websocket.StatusNormalClosure, "connection closed")
	}()

	// Session revalidation runs alongside the read loop; it terminates when
	// the client context is cancelled here on disconnect.
	if c.sessionVal != nil {
		go c.revalidateLoop()
	}

	c.conn.SetReadLimit(wsReadLimit)

	for {
		ctx, cancel := context.WithTimeout(context.Background(), readTimeout)
		_, msg, err := c.conn.Read(ctx)
		cancel()
		if err != nil {
			if websocket.CloseStatus(err) == websocket.StatusNormalClosure ||
				websocket.CloseStatus(err) == websocket.StatusGoingAway {
				return
			}
			c.log.Debug("read error", "user_id", c.userID, "error", err)
			return
		}

		msgType, payload, err := UnmarshalWire(msg)
		if err != nil {
			c.Send(NewErrorMessage("invalid_message", "failed to parse message", ""))
			continue
		}

		c.handleMessage(msgType, payload)
	}
}

func (c *Client) WritePump() {
	ticker := time.NewTicker(wsPingInterval)
	defer func() {
		ticker.Stop()
		c.conn.Close(websocket.StatusNormalClosure, "connection closed")
	}()

	for {
		select {
		case msg, ok := <-c.send:
			if !ok {
				return
			}

			ctx, cancel := context.WithTimeout(context.Background(), writeTimeout)
			err := c.conn.Write(ctx, websocket.MessageText, msg)
			cancel()
			if err != nil {
				c.log.Debug("write error", "user_id", c.userID, "error", err)
				return
			}

		case <-ticker.C:
			ctx, cancel := context.WithTimeout(context.Background(), writeTimeout)
			err := c.conn.Write(ctx, websocket.MessageText, NewPongMessage(time.Now().UnixMilli()))
			cancel()
			if err != nil {
				c.log.Debug("ping write error", "user_id", c.userID, "error", err)
				return
			}
		}
	}
}

func (c *Client) Send(data []byte) {
	select {
	case c.send <- data:
	default:
		c.log.Warn("client send buffer full", "user_id", c.userID)
	}
}

// canAccessConversation reports whether the user may view the conversation.
// When no access checker is wired (e.g. in unit tests) it fails closed and
// denies the subscription, which is the safe default.
//
// Uses context.Background() because the WS connection has no HTTP request to
// derive from. This is acceptable because access checks are fast DB reads
// that don't need cancellation, and defense-in-depth requires them to
// complete even if the client disconnects mid-check.
func (c *Client) canAccessConversation(conversationID string) bool {
	if c.accessChk == nil {
		return false
	}
	return c.accessChk.CanAccessConversation(context.Background(), c.orgID, conversationID, c.userID, c.orgRole)
}

// canAccessProject reports whether the user may view the project room.
// Uses context.Background(); see canAccessConversation for rationale.
func (c *Client) canAccessProject(projectID string) bool {
	if c.accessChk == nil {
		return false
	}
	return c.accessChk.CanAccessProject(context.Background(), c.orgID, projectID, c.userID, c.orgRole)
}

// canSendInConversation reports whether the user has send permission in the
// conversation. Used to gate typing indicators so a user can't spam typing
// into channels they can't even post in.
// Uses context.Background(); see canAccessConversation for rationale.
func (c *Client) canSendInConversation(conversationID string) bool {
	if c.accessChk == nil {
		return false
	}
	return c.accessChk.CanSendInConversation(context.Background(), c.orgID, conversationID, c.userID, c.orgRole)
}

func (c *Client) handleMessage(msgType string, payload json.RawMessage) {
	switch msgType {
	case string(domain.WsTypePing):
		c.handlePing(msgType, payload)
	case string(domain.WsTypeConversationSubscribe):
		c.handleConvSubscribe(msgType, payload)
	case string(domain.WsTypeConversationUnsubscribe):
		c.handleConvUnsubscribe(msgType, payload)
	case string(domain.WsTypeProjectSubscribe):
		c.handleProjectSubscribe(msgType, payload)
	case string(domain.WsTypeProjectUnsubscribe):
		c.handleProjectUnsubscribe(msgType, payload)
	case string(domain.WsTypeTypingStart):
		c.handleTypingStart(msgType, payload)
	case string(domain.WsTypeTypingStop):
		c.handleTypingStop(msgType, payload)
	case string(domain.WsTypeVoiceJoin):
		c.handleVoiceJoin(msgType, payload)
	case string(domain.WsTypeVoiceLeave):
		c.handleVoiceLeave(msgType, payload)
	case string(domain.WsTypeVoiceSignal):
		c.handleVoiceSignal(msgType, payload)
	case string(domain.WsTypeVoiceMute):
		c.handleVoiceMute(msgType, payload)
	case string(domain.WsTypeVoiceDeafen):
		c.handleVoiceDeafen(msgType, payload)
	default:
		c.Send(NewErrorMessage("unknown_type", "unknown message type: "+msgType, msgType))
	}
}

func (c *Client) handlePing(msgType string, payload json.RawMessage) {
	p, err := ParsePayload[pingPayload](payload)
	if err != nil {
		c.Send(NewErrorMessage("invalid_payload", "invalid ping payload", string(domain.WsTypePing)))
		return
	}
	c.Send(NewPongMessage(p.Timestamp))
}

func (c *Client) handleConvSubscribe(msgType string, payload json.RawMessage) {
	p, err := ParsePayload[conversationSubscribePayload](payload)
	if err != nil {
		c.Send(NewErrorMessage("invalid_payload", "invalid subscribe payload", msgType))
		return
	}
	if !c.canAccessConversation(p.ConversationID) {
		c.Send(NewErrorMessage("forbidden", "no access to conversation", msgType))
		return
	}
	c.hub.Subscribe(c, domain.RoomKeyConversation(c.orgID, p.ConversationID))
}

func (c *Client) handleConvUnsubscribe(msgType string, payload json.RawMessage) {
	p, err := ParsePayload[conversationUnsubscribePayload](payload)
	if err != nil {
		c.Send(NewErrorMessage("invalid_payload", "invalid unsubscribe payload", msgType))
		return
	}
	c.hub.Unsubscribe(c, domain.RoomKeyConversation(c.orgID, p.ConversationID))
}

func (c *Client) handleProjectSubscribe(msgType string, payload json.RawMessage) {
	p, err := ParsePayload[projectSubscribePayload](payload)
	if err != nil {
		c.Send(NewErrorMessage("invalid_payload", "invalid project subscribe payload", msgType))
		return
	}
	if !c.canAccessProject(p.ProjectID) {
		c.Send(NewErrorMessage("forbidden", "no access to project", msgType))
		return
	}
	c.hub.Subscribe(c, domain.RoomKeyProject(c.orgID, p.ProjectID))
}

func (c *Client) handleProjectUnsubscribe(msgType string, payload json.RawMessage) {
	p, err := ParsePayload[projectUnsubscribePayload](payload)
	if err != nil {
		c.Send(NewErrorMessage("invalid_payload", "invalid project unsubscribe payload", msgType))
		return
	}
	c.hub.Unsubscribe(c, domain.RoomKeyProject(c.orgID, p.ProjectID))
}

func (c *Client) handleTypingStart(msgType string, payload json.RawMessage) {
	p, err := ParsePayload[typingStartPayload](payload)
	if err != nil {
		c.Send(NewErrorMessage("invalid_payload", "invalid typing_start payload", msgType))
		return
	}
	if !c.canAccessConversation(p.ConversationID) {
		c.Send(NewErrorMessage("forbidden", "no access to conversation", msgType))
		return
	}
	if !c.canSendInConversation(p.ConversationID) {
		c.Send(NewErrorMessage("forbidden", "no send permission in conversation", msgType))
		return
	}
	// Debounce: a chatty client may send typing_start on every keystroke;
	// drop duplicates within the window to avoid N x M broadcast
	// amplification to every room member.
	if !c.shouldBroadcastTyping(p.ConversationID) {
		return
	}
	c.hub.BroadcastExcept(domain.RoomKeyConversation(c.orgID, p.ConversationID), string(domain.WsTypeTyping), map[string]any{
		"conversation_id": p.ConversationID,
		"user_id":         c.userID,
		"is_typing":       true,
	}, c.userID)
}

func (c *Client) handleTypingStop(msgType string, payload json.RawMessage) {
	p, err := ParsePayload[typingStopPayload](payload)
	if err != nil {
		c.Send(NewErrorMessage("invalid_payload", "invalid typing_stop payload", msgType))
		return
	}
	if !c.canAccessConversation(p.ConversationID) {
		c.Send(NewErrorMessage("forbidden", "no access to conversation", msgType))
		return
	}
	if !c.canSendInConversation(p.ConversationID) {
		c.Send(NewErrorMessage("forbidden", "no send permission in conversation", msgType))
		return
	}
	c.hub.BroadcastExcept(domain.RoomKeyConversation(c.orgID, p.ConversationID), string(domain.WsTypeTyping), map[string]any{
		"conversation_id": p.ConversationID,
		"user_id":         c.userID,
		"is_typing":       false,
	}, c.userID)
}

func (c *Client) handleVoiceJoin(msgType string, payload json.RawMessage) {
	if c.voiceSvc == nil {
		c.Send(NewErrorMessage("service_unavailable", "voice service not available", msgType))
		return
	}
	p, err := ParsePayload[voiceJoinPayload](payload)
	if err != nil {
		c.Send(NewErrorMessage("invalid_payload", "invalid voice_join payload", msgType))
		return
	}
	if !c.canAccessConversation(p.ConversationID) {
		c.Send(NewErrorMessage("forbidden", "no access to conversation", msgType))
		return
	}
	result, err := c.voiceSvc.Join(c.ctx, c.orgID, c.userID, c.orgRole, c.connID, p.ConversationID)
	if err != nil {
		c.Send(NewErrorMessage("join_failed", err.Error(), msgType))
		return
	}
	participantPayloads := make([]voiceParticipantPayload, 0, len(result.Participants))
	for _, p := range result.Participants {
		participantPayloads = append(participantPayloads, voiceParticipantPayload{
			ID:        p.ID,
			UserID:    p.UserID,
			Name:      p.Name,
			AvatarURL: p.AvatarURL,
			Muted:     p.Muted,
			Deafened:  p.Deafened,
			Speaking:  p.Speaking,
			JoinedAt:  p.JoinedAt.Format(time.RFC3339),
		})
	}
	icePayloads := make([]voiceICEServerPayload, 0, len(result.ICEServers))
	for _, s := range result.ICEServers {
		icePayloads = append(icePayloads, voiceICEServerPayload{
			URLs:       s.URLs,
			Username:   s.Username,
			Credential: s.Credential,
		})
	}
	c.Send(NewVoiceJoinResultMessage(participantPayloads, icePayloads, result.SDPOffer, result.MaxParticipants))
}

func (c *Client) handleVoiceLeave(msgType string, payload json.RawMessage) {
	if c.voiceSvc == nil {
		c.Send(NewErrorMessage("service_unavailable", "voice service not available", msgType))
		return
	}
	p, err := ParsePayload[voiceLeavePayload](payload)
	if err != nil {
		c.Send(NewErrorMessage("invalid_payload", "invalid voice_leave payload", msgType))
		return
	}
	if err := c.voiceSvc.Leave(c.ctx, c.orgID, c.userID, p.ConversationID); err != nil {
		c.Send(NewErrorMessage("leave_failed", err.Error(), msgType))
	}
}

func (c *Client) handleVoiceSignal(msgType string, payload json.RawMessage) {
	if c.voiceSvc == nil {
		c.Send(NewErrorMessage("service_unavailable", "voice service not available", msgType))
		return
	}
	p, err := ParsePayload[voiceSignalPayload](payload)
	if err != nil {
		c.Send(NewErrorMessage("invalid_payload", "invalid voice_signal payload", msgType))
		return
	}
	msg := domain.VoiceSignalMsg{
		Type:         p.Type,
		SDP:          p.SDP,
		Candidate:    p.Candidate,
		TargetUserID: p.TargetUserID,
	}
	if err := c.voiceSvc.HandleSignal(c.ctx, c.orgID, c.userID, c.connID, p.ConversationID, msg); err != nil {
		c.Send(NewErrorMessage("signal_failed", err.Error(), msgType))
	}
}

func (c *Client) handleVoiceMute(msgType string, payload json.RawMessage) {
	if c.voiceSvc == nil {
		c.Send(NewErrorMessage("service_unavailable", "voice service not available", msgType))
		return
	}
	p, err := ParsePayload[voiceMutePayload](payload)
	if err != nil {
		c.Send(NewErrorMessage("invalid_payload", "invalid voice_mute payload", msgType))
		return
	}
	if err := c.voiceSvc.SetMute(c.ctx, c.orgID, c.userID, p.ConversationID, p.Muted); err != nil {
		c.Send(NewErrorMessage("mute_failed", err.Error(), msgType))
	}
}

func (c *Client) handleVoiceDeafen(msgType string, payload json.RawMessage) {
	if c.voiceSvc == nil {
		c.Send(NewErrorMessage("service_unavailable", "voice service not available", msgType))
		return
	}
	p, err := ParsePayload[voiceDeafenPayload](payload)
	if err != nil {
		c.Send(NewErrorMessage("invalid_payload", "invalid voice_deafen payload", msgType))
		return
	}
	if err := c.voiceSvc.SetDeafen(c.ctx, c.orgID, c.userID, p.ConversationID, p.Deafened); err != nil {
		c.Send(NewErrorMessage("deafen_failed", err.Error(), msgType))
	}
}
