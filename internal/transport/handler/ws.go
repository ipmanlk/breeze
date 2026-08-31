package handler

import (
	"context"
	"log/slog"
	"net/http"
	"time"

	"ipmanlk/plume/internal/domain"
	"ipmanlk/plume/internal/port"
	"ipmanlk/plume/internal/transport"
	"ipmanlk/plume/internal/transport/ws"

	"github.com/coder/websocket"
)

type WsHandler struct {
	hub            *ws.Hub
	presenceSvc    port.PresenceService
	presenceRepo   port.PresenceRepository
	voiceSvc       port.VoiceService
	accessChk      ws.RoomAccessChecker
	sessionVal     ws.SessionValidator
	corsOrigins    []string
	typingDebounce time.Duration
	log            *slog.Logger
}

func NewWsHandler(hub *ws.Hub, presenceSvc port.PresenceService, presenceRepo port.PresenceRepository, voiceSvc port.VoiceService, corsOrigins []string, log *slog.Logger) *WsHandler {
	return &WsHandler{hub: hub, presenceSvc: presenceSvc, presenceRepo: presenceRepo, voiceSvc: voiceSvc, corsOrigins: corsOrigins, log: log}
}

// SetTypingDebounce configures the per-conversation typing_start debounce
// applied to every upgraded client. Called during wiring from the loaded
// WebSocket config.
func (h *WsHandler) SetTypingDebounce(d time.Duration) {
	h.typingDebounce = d
}

// SetAccessChecker injects the room-access checker used to authorize
// client-driven WS subscribe/typing messages. When set, the upgraded client
// refuses to subscribe to rooms the user has no access to.
func (h *WsHandler) SetAccessChecker(chk ws.RoomAccessChecker) {
	h.accessChk = chk
}

// SetSessionValidator injects the periodic session re-checker applied to
// every upgraded client, so demotions and deactivations reach live sockets.
func (h *WsHandler) SetSessionValidator(v ws.SessionValidator) {
	h.sessionVal = v
}

// @Summary		Upgrade to WebSocket
// @Description	Upgrades the HTTP connection to a WebSocket for real-time events (chat, presence, typing, task updates).
// @Tags		websocket
// @Router		/ws [get]
func (h *WsHandler) Upgrade(w http.ResponseWriter, r *http.Request) {
	userID, ok := transport.UserIDFromContext(r.Context())
	if !ok {
		transport.LocalizedErrorJSON(w, r, http.StatusUnauthorized, "unauthorized", "ErrUnauthorized")
		return
	}
	orgID, ok := transport.OrgIDFromContext(r.Context())
	if !ok {
		transport.LocalizedErrorJSON(w, r, http.StatusUnauthorized, "unauthorized", "ErrUnauthorized")
		return
	}
	sessionID, ok := transport.SessionIDFromContext(r.Context())
	if !ok {
		transport.LocalizedErrorJSON(w, r, http.StatusUnauthorized, "unauthorized", "ErrUnauthorized")
		return
	}

	conn, err := websocket.Accept(w, r, &websocket.AcceptOptions{
		// OriginPatterns gates CROSS-ORIGIN WebSocket connections (CSRF
		// defense). coder/websocket always authorizes a request whose Origin
		// host equals the request Host (same-origin), so a production same-origin
		// deployment works even when corsOrigins defaults to localhost. Operators
		// running the SPA and API on different origins must list the SPA origin in
		// CORS_ORIGINS. The __Host- cookie + SameSite=Lax already prevent CSRF
		// for the HTTP path; this is the WS equivalent.
		OriginPatterns: h.corsOrigins,
	})
	if err != nil {
		h.log.Error("websocket upgrade failed", "error", err)
		return
	}

	// Register and subscribe client BEFORE setting presence status.
	// This ensures the client is in the org room when presence_updated
	// is broadcast, so they receive their own online event.
	client := ws.NewClient(h.hub, conn, userID, orgID, sessionID, h.log, h.voiceSvc)
	// Authorize client-driven room subscriptions against the channel
	// permission + project access layers so a user cannot eavesdrop on
	// conversations/projects they have no access to. The org role is recorded
	// on the client even when no checker is wired, so voice/permission
	// resolution always sees the real role instead of an empty one.
	roleStr, _ := transport.RoleFromContext(r.Context())
	client.SetAccessChecker(h.accessChk, domain.Role(roleStr))
	if h.sessionVal != nil {
		client.SetSessionValidator(h.sessionVal)
	}
	client.SetTypingDebounce(h.typingDebounce)
	// Register with the hub, enforcing per-user/global connection caps.
	// A rejected connection is torn down immediately.
	if !h.hub.Register(client) {
		h.log.Warn("websocket connection rejected: limit reached", "user_id", userID)
		conn.Close(websocket.StatusTryAgainLater, "too many connections")
		return
	}
	h.hub.Subscribe(client, domain.RoomKeyOrg(orgID))
	h.hub.Subscribe(client, domain.RoomKeyUser(orgID, userID))
	// Subscribe to a connection-specific room so voice signaling (ICE
	// candidates, subscriber offers) is delivered only to this tab, not
	// broadcast to every tab for the user (multi-tab correctness).
	h.hub.Subscribe(client, domain.RoomKeyConnection(orgID, client.ConnID()))

	// Set to online on connect (unless user explicitly set DND)
	if h.presenceSvc != nil {
		pres, err := h.presenceRepo.Get(r.Context(), orgID, userID)
		if err != nil {
			h.log.Error("presence get on connect", "error", err, "user_id", userID)
		}
		if pres != nil && pres.Status == domain.PresenceDnd {
			// keep DND, but broadcast to refresh last_seen
			if err := h.presenceSvc.SetStatus(r.Context(), orgID, userID, domain.PresenceDnd); err != nil {
				h.log.Error("presence set online (DND)", "error", err, "user_id", userID)
			}
		} else {
			if err := h.presenceSvc.SetStatus(r.Context(), orgID, userID, domain.PresenceOnline); err != nil {
				h.log.Error("presence set online", "error", err, "user_id", userID)
			}
		}
	}

	client.Send(ws.NewConnectedMessage(userID, sessionID))

	go client.WritePump()
	client.ReadPump()

	// Synchronously remove the client from the hub and get the remaining
	// connection count. This is used below to only set the user offline
	// when ALL their connections have been removed, fixing the multi-tab
	// presence race where a stale count captured at connect time would
	// incorrectly set presence to offline when another tab is still open.
	remaining := h.hub.UnregisterAndWait(client)

	h.log.Info("websocket disconnected", "user_id", userID, "session_id", sessionID, "remaining_connections", remaining)

	// Use a background context so presence writes always succeed even
	// if the original HTTP request context was cancelled.
	bg := context.Background()

	// Only leave voice channels owned by THIS connection on disconnect.
	// A user's other tabs (different connections) keep their sessions.
	if h.voiceSvc != nil {
		if err := h.voiceSvc.LeaveByConnection(bg, orgID, userID, client.ConnID()); err != nil {
			h.log.Error("voice leave-by-connection on disconnect", "error", err, "user_id", userID)
		}
	}

	if remaining == 0 && h.presenceSvc != nil {
		pres, err := h.presenceRepo.Get(bg, orgID, userID)
		if err != nil {
			h.log.Error("presence get on disconnect", "error", err, "user_id", userID)
		}
		if pres != nil && pres.Status == domain.PresenceDnd {
			// keep DND through disconnect
			if err := h.presenceSvc.SetStatus(bg, orgID, userID, domain.PresenceDnd); err != nil {
				h.log.Error("presence set offline (DND)", "error", err, "user_id", userID)
			}
		} else {
			if err := h.presenceSvc.SetStatus(bg, orgID, userID, domain.PresenceOffline); err != nil {
				h.log.Error("presence set offline", "error", err, "user_id", userID)
			}
		}
	}
}
