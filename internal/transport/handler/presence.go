package handler

import (
	"log/slog"
	"net/http"

	"ipmanlk/plume/internal/domain"
	"ipmanlk/plume/internal/port"
	"ipmanlk/plume/internal/transport"
	"ipmanlk/plume/internal/transport/dto"
)

type PresenceHandler struct {
	svc port.PresenceService
	log *slog.Logger
}

func NewPresenceHandler(svc port.PresenceService, log *slog.Logger) *PresenceHandler {
	return &PresenceHandler{svc: svc, log: log}
}

// @Summary	List presence for org
// @Tags		chat-presence
// @Produce	json
// @Success	200	{object}	dto.PresenceListResponse
// @Failure		500		{object}	transport.ErrorResponse
// @Router		/chat/presence [get]
func (h *PresenceHandler) List(w http.ResponseWriter, r *http.Request) {
	orgID, _ := transport.OrgIDFromContext(r.Context())
	items, err := h.svc.ListForOrg(r.Context(), orgID)
	if err != nil {
		transport.ServerError(w, r, h.log, err)
		return
	}
	out := &dto.PresenceListResponse{Items: make([]*dto.UserPresenceResponse, 0, len(items))}
	for _, p := range items {
		out.Items = append(out.Items, dto.NewUserPresenceResponse(p))
	}
	transport.JSON(w, r, http.StatusOK, out)
}

// @Summary	Set own presence
// @Tags		chat-presence
// @Accept		json
// @Produce	json
// @Param		body	body	dto.SetStatusRequest	true	"New status"
// @Success	200		{object}	transport.SuccessResponse
// @Failure		400		{object}	transport.ErrorResponse
// @Failure		500		{object}	transport.ErrorResponse
// @Router		/chat/presence/me [put]
func (h *PresenceHandler) SetMe(w http.ResponseWriter, r *http.Request) {
	userID, _ := transport.UserIDFromContext(r.Context())
	orgID, _ := transport.OrgIDFromContext(r.Context())
	var req dto.SetStatusRequest
	if err := dto.BindAndValidate(r, &req); err != nil {
		transport.ErrorJSON(w, r, http.StatusBadRequest, "validation_error", err.Error())
		return
	}
	if err := h.svc.SetStatus(r.Context(), orgID, userID, domain.PresenceStatus(req.Status)); err != nil {
		transport.ServerError(w, r, h.log, err)
		return
	}
	transport.JSON(w, r, http.StatusOK, transport.SuccessResponse{Success: true})
}
