package handler

import (
	"log/slog"
	"net/http"
	"strconv"

	"ipmanlk/breeze/internal/port"
	"ipmanlk/breeze/internal/transport"
	"ipmanlk/breeze/internal/transport/dto"
)

type AuditHandler struct {
	svc port.AuditService
	log *slog.Logger
}

func NewAuditHandler(svc port.AuditService, log *slog.Logger) *AuditHandler {
	return &AuditHandler{svc: svc, log: log}
}

// @Summary		List audit log entries
// @Description	Returns the org's audit log (admin actions), newest first. Filter by action or actor.
// @Tags			audit-log
// @Produce		json
// @Param			limit		query		int		false	"Max results per page (default 50, max 100)"
// @Param			offset		query		int		false	"Pagination offset (default 0)"
// @Param			action		query		string	false	"Filter by action type (e.g. role_changed)"
// @Param			actor_id	query		string	false	"Filter by actor user ID"
// @Success		200			{object}	dto.AuditLogResponse
// @Router			/audit-log [get]
func (h *AuditHandler) List(w http.ResponseWriter, r *http.Request) {
	orgID, _ := transport.OrgIDFromContext(r.Context())

	limit := 50
	offset := 0
	if v := r.URL.Query().Get("limit"); v != "" {
		if n, err := strconv.Atoi(v); err == nil && n > 0 {
			limit = n
		}
	}
	if v := r.URL.Query().Get("offset"); v != "" {
		if n, err := strconv.Atoi(v); err == nil && n >= 0 {
			offset = n
		}
	}

	// Optional filters: nil means "no filter".
	var action, actorID *string
	if v := r.URL.Query().Get("action"); v != "" {
		action = &v
	}
	if v := r.URL.Query().Get("actor_id"); v != "" {
		actorID = &v
	}

	entries, total, err := h.svc.List(r.Context(), orgID, limit, offset, action, actorID)
	if err != nil {
		transport.ServerError(w, r, h.log, err)
		return
	}

	resp := &dto.AuditLogResponse{
		Total:  total,
		Limit:  limit,
		Offset: offset,
		Items:  make([]*dto.AuditEntryResponse, len(entries)),
	}
	for i, e := range entries {
		resp.Items[i] = dto.NewAuditEntryResponse(e)
	}
	transport.JSON(w, r, http.StatusOK, resp)
}
