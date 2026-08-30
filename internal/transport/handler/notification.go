package handler

import (
	"log/slog"
	"net/http"
	"strconv"

	"ipmanlk/breeze/internal/domain"
	"ipmanlk/breeze/internal/port"
	"ipmanlk/breeze/internal/transport"
	"ipmanlk/breeze/internal/transport/dto"

	"github.com/go-chi/chi/v5"
)

type NotificationHandler struct {
	svc port.NotificationService
	log *slog.Logger
}

func NewNotificationHandler(svc port.NotificationService, log *slog.Logger) *NotificationHandler {
	return &NotificationHandler{svc: svc, log: log}
}

// @Summary		List notifications
// @Description	Cursor-paginated list of notifications for the current user
// @Tags			notifications
// @Param			cursor		query		string	false	"Cursor for pagination"
// @Param			unread_only	query		bool	false	"Filter to unread only"
// @Param			limit		query		int		false	"Page size (max 100)"
// @Success		200			{object}	dto.PaginatedNotificationsResponse
// @Failure		401			{object}	transport.ErrorResponse
// @Router			/notifications [get]
func (h *NotificationHandler) List(w http.ResponseWriter, r *http.Request) {
	userID, ok := transport.UserIDFromContext(r.Context())
	if !ok {
		transport.LocalizedErrorJSON(w, r, http.StatusUnauthorized, "unauthorized", "ErrNotAuthenticated")
		return
	}
	orgID, ok := transport.OrgIDFromContext(r.Context())
	if !ok {
		transport.LocalizedErrorJSON(w, r, http.StatusUnauthorized, "unauthorized", "ErrNotAuthenticated")
		return
	}

	filter := domain.NotificationFilter{
		Cursor: r.URL.Query().Get("cursor"),
		Limit:  20,
	}
	if r.URL.Query().Get("unread_only") == "true" {
		filter.UnreadOnly = true
	}
	if l, err := strconv.Atoi(r.URL.Query().Get("limit")); err == nil && l > 0 && l <= 100 {
		filter.Limit = l
	}

	result, err := h.svc.List(r.Context(), orgID, userID, filter)
	if err != nil {
		transport.ServerError(w, r, h.log, err)
		return
	}

	items := make([]*dto.NotificationResponse, len(result.Items))
	for i, n := range result.Items {
		items[i] = dto.NewNotificationResponse(n)
	}

	transport.JSON(w, r, http.StatusOK, dto.PaginatedNotificationsResponse{
		Items:      items,
		NextCursor: result.NextCursor,
		HasMore:    result.HasMore,
	})
}

// @Summary		Get unread notification count
// @Description	Returns the number of unread notifications for the current user
// @Tags			notifications
// @Success		200	{object}	dto.UnreadCountResponse
// @Failure		401	{object}	transport.ErrorResponse
// @Router			/notifications/unread-count [get]
func (h *NotificationHandler) CountUnread(w http.ResponseWriter, r *http.Request) {
	userID, ok := transport.UserIDFromContext(r.Context())
	if !ok {
		transport.LocalizedErrorJSON(w, r, http.StatusUnauthorized, "unauthorized", "ErrNotAuthenticated")
		return
	}

	count, err := h.svc.CountUnread(r.Context(), userID)
	if err != nil {
		transport.ServerError(w, r, h.log, err)
		return
	}

	transport.JSON(w, r, http.StatusOK, dto.UnreadCountResponse{Count: count})
}

// @Summary		Mark notification as read
// @Description	Marks a single notification as read
// @Tags			notifications
// @Param			id	path		string	true	"Notification ID"
// @Success		200	{object}	map[string]bool
// @Failure		401	{object}	transport.ErrorResponse
// @Failure		404	{object}	transport.ErrorResponse
// @Router			/notifications/{id}/read [patch]
func (h *NotificationHandler) MarkRead(w http.ResponseWriter, r *http.Request) {
	userID, ok := transport.UserIDFromContext(r.Context())
	if !ok {
		transport.LocalizedErrorJSON(w, r, http.StatusUnauthorized, "unauthorized", "ErrNotAuthenticated")
		return
	}

	id := chi.URLParam(r, "id")
	if id == "" {
		transport.LocalizedErrorJSON(w, r, http.StatusBadRequest, "validation_error", "ErrIDRequired")
		return
	}

	if err := h.svc.MarkRead(r.Context(), id, userID); err != nil {
		// The store returns apperr.ErrNotFound when no row matched id+user_id
		// (missing or owned by another user). RespondWithError maps that to 404.
		transport.RespondWithError(w, r, h.log, err)
		return
	}

	transport.JSON(w, r, http.StatusOK, map[string]bool{"success": true})
}

// @Summary		Mark all notifications as read
// @Description	Marks all unread notifications as read
// @Tags			notifications
// @Success		200	{object}	map[string]bool
// @Failure		401	{object}	transport.ErrorResponse
// @Router			/notifications/read-all [patch]
func (h *NotificationHandler) MarkAllRead(w http.ResponseWriter, r *http.Request) {
	userID, ok := transport.UserIDFromContext(r.Context())
	if !ok {
		transport.LocalizedErrorJSON(w, r, http.StatusUnauthorized, "unauthorized", "ErrNotAuthenticated")
		return
	}

	if err := h.svc.MarkAllRead(r.Context(), userID); err != nil {
		transport.ServerError(w, r, h.log, err)
		return
	}

	transport.JSON(w, r, http.StatusOK, map[string]bool{"success": true})
}

// @Summary		Get notification preferences
// @Description	Returns the current user's notification preferences
// @Tags			notifications
// @Success		200	{array}		dto.NotificationPreferenceResponse
// @Failure		401	{object}	transport.ErrorResponse
// @Router			/settings/notifications [get]
func (h *NotificationHandler) GetPreferences(w http.ResponseWriter, r *http.Request) {
	userID, ok := transport.UserIDFromContext(r.Context())
	if !ok {
		transport.LocalizedErrorJSON(w, r, http.StatusUnauthorized, "unauthorized", "ErrNotAuthenticated")
		return
	}

	prefs, err := h.svc.GetPreferences(r.Context(), userID)
	if err != nil {
		transport.ServerError(w, r, h.log, err)
		return
	}

	resp := make([]dto.NotificationPreferenceResponse, len(prefs))
	for i, p := range prefs {
		resp[i] = dto.NotificationPreferenceResponse{
			Type:    string(p.Type),
			Enabled: p.Enabled,
		}
	}

	transport.JSON(w, r, http.StatusOK, resp)
}

// @Summary		Set notification preference
// @Description	Toggle a notification type preference
// @Tags			notifications
// @Param			type	path		string									true	"Notification type"
// @Param			body	body		dto.SetNotificationPreferenceRequest	true	"Preference state"
// @Success		200		{object}	map[string]bool
// @Failure		401		{object}	transport.ErrorResponse
// @Router			/settings/notifications/{type} [patch]
func (h *NotificationHandler) SetPreference(w http.ResponseWriter, r *http.Request) {
	userID, ok := transport.UserIDFromContext(r.Context())
	if !ok {
		transport.LocalizedErrorJSON(w, r, http.StatusUnauthorized, "unauthorized", "ErrNotAuthenticated")
		return
	}

	notifType := domain.NotificationType(chi.URLParam(r, "type"))
	if notifType == "" {
		transport.LocalizedErrorJSON(w, r, http.StatusBadRequest, "validation_error", "ErrTypeRequired")
		return
	}

	var req dto.SetNotificationPreferenceRequest
	if err := dto.BindAndValidate(r, &req); err != nil {
		transport.ErrorJSON(w, r, http.StatusBadRequest, "validation_error", err.Error())
		return
	}

	if err := h.svc.SetPreference(r.Context(), userID, notifType, *req.Enabled); err != nil {
		transport.ServerError(w, r, h.log, err)
		return
	}

	transport.JSON(w, r, http.StatusOK, map[string]bool{"success": true})
}
