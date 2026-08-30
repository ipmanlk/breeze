package handler

import (
	"log/slog"
	"net/http"

	"github.com/go-chi/chi/v5"
	"ipmanlk/breeze/internal/port"
	"ipmanlk/breeze/internal/transport"
	"ipmanlk/breeze/internal/transport/dto"
)

// VoiceHandler handles voice channel HTTP requests.
type VoiceHandler struct {
	svc       port.VoiceService
	accessSvc port.AccessService
	log       *slog.Logger
}

// NewVoiceHandler creates a new VoiceHandler.
func NewVoiceHandler(svc port.VoiceService, accessSvc port.AccessService, log *slog.Logger) *VoiceHandler {
	return &VoiceHandler{svc: svc, accessSvc: accessSvc, log: log}
}

// ListParticipants handles GET /api/conversations/{id}/voice/participants
// @Summary		List voice channel participants
// @Description	Get all participants in a voice channel
// @Tags		voice
// @Produce		json
// @Param		id	path	string	true	"Conversation ID"
// @Success		200	{array}	dto.VoiceParticipantResponse
// @Failure		401	{object}	map[string]string	"Unauthorized"
// @Failure		403	{object}	map[string]string	"Forbidden"
// @Failure		404	{object}	map[string]string	"Not Found"
// @Router		/conversations/{id}/voice/participants [get]
func (h *VoiceHandler) ListParticipants(w http.ResponseWriter, r *http.Request) {
	convID := chi.URLParam(r, "id")
	if convID == "" {
		transport.LocalizedErrorJSON(w, r, http.StatusBadRequest, "validation_error", "ErrIDRequired")
		return
	}

	if !EnsureConversationAccess(w, r, h.accessSvc, convID) {
		return
	}

	orgID, _ := transport.OrgIDFromContext(r.Context())

	participants, err := h.svc.ListParticipants(r.Context(), orgID, convID)
	if err != nil {
		h.log.Error("list voice participants", "error", err, "conv_id", convID)
		transport.ErrorJSON(w, r, http.StatusInternalServerError, "internal_error", "failed to list participants")
		return
	}

	resp := make([]*dto.VoiceParticipantResponse, 0, len(participants))
	for _, p := range participants {
		resp = append(resp, dto.NewVoiceParticipantResponse(&p))
	}

	transport.JSON(w, r, http.StatusOK, resp)
}
