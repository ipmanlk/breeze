package handler

import (
	"log/slog"
	"net/http"

	"ipmanlk/breeze/internal/port"
	"ipmanlk/breeze/internal/transport"
	"ipmanlk/breeze/internal/transport/dto"
)

type PushHandler struct {
	svc port.PushService
	log *slog.Logger
}

func NewPushHandler(svc port.PushService, log *slog.Logger) *PushHandler {
	return &PushHandler{svc: svc, log: log}
}

// @Summary		Get push public key
// @Description	Returns the VAPID public key for browser push subscription
// @Tags			push
// @Produce		json
// @Success		200	{object}	dto.PushPublicKeyResponse
// @Router			/push/vapid-public-key [get]
func (h *PushHandler) PublicKey(w http.ResponseWriter, r *http.Request) {
	transport.JSON(w, r, http.StatusOK, dto.PushPublicKeyResponse{
		PublicKey: h.svc.PublicKey(),
		Enabled:   h.svc.Enabled(),
	})
}

// @Summary		Subscribe to push notifications
// @Description	Registers a Web Push subscription endpoint for the current user
// @Tags			push
// @Accept			json
// @Produce		json
// @Param			body	body		dto.PushSubscribeRequest	true	"Push subscription"
// @Success		200		{object}	transport.SuccessResponse
// @Failure		400		{object}	transport.ErrorResponse
// @Failure		403		{object}	transport.ErrorResponse
// @Router			/push/subscribe [post]
func (h *PushHandler) Subscribe(w http.ResponseWriter, r *http.Request) {
	var req dto.PushSubscribeRequest
	if err := dto.BindAndValidate(r, &req); err != nil {
		transport.ErrorJSON(w, r, http.StatusBadRequest, "validation_error", err.Error())
		return
	}
	userID, ok := transport.UserIDFromContext(r.Context())
	if !ok {
		transport.LocalizedErrorJSON(w, r, http.StatusForbidden, "forbidden", "ErrNotAuthenticated")
		return
	}
	orgID, _ := transport.OrgIDFromContext(r.Context())
	if err := h.svc.Subscribe(r.Context(), userID, orgID, req.Endpoint, req.P256dh, req.Auth); err != nil {
		transport.ErrorJSON(w, r, http.StatusBadRequest, "push_error", err.Error())
		return
	}
	transport.JSON(w, r, http.StatusOK, transport.SuccessResponse{Success: true})
}

// @Summary		Unsubscribe from push notifications
// @Description	Removes a Web Push subscription endpoint for the current user
// @Tags			push
// @Accept			json
// @Produce		json
// @Param			body	body		dto.PushSubscribeRequest	true	"Push subscription (endpoint identifies the row)"
// @Success		204		{string}	string	"No Content"
// @Failure		403		{object}	transport.ErrorResponse
// @Router			/push/subscribe [delete]
func (h *PushHandler) Unsubscribe(w http.ResponseWriter, r *http.Request) {
	// Dual-parse strategy: first try to parse the JSON body for callers that
	// send a full subscription object (e.g. the frontend), then fall back to
	// a query-param-only endpoint for REST-minimal callers. Query-param-only
	// is preferred for DELETE idempotency, but the body variant is kept for
	// backward compatibility with the generated SDK.
	var req dto.PushSubscribeRequest
	if err := dto.BindAndValidate(r, &req); err != nil {
		// Allow query-param fallback for DELETE without a body.
		req.Endpoint = r.URL.Query().Get("endpoint")
	}
	userID, ok := transport.UserIDFromContext(r.Context())
	if !ok {
		transport.LocalizedErrorJSON(w, r, http.StatusForbidden, "forbidden", "ErrNotAuthenticated")
		return
	}
	if err := h.svc.Unsubscribe(r.Context(), userID, req.Endpoint); err != nil {
		h.log.Error("push unsubscribe", "error", err)
		transport.LocalizedErrorJSON(w, r, http.StatusInternalServerError, "push_error", "ErrFailedToUnsubscribe")
		return
	}
	w.WriteHeader(http.StatusNoContent)
}
