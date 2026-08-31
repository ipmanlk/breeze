package handler

import (
	"log/slog"
	"net/http"

	"ipmanlk/plume/internal/domain"
	"ipmanlk/plume/internal/port"
	"ipmanlk/plume/internal/transport"
	"ipmanlk/plume/internal/transport/dto"
)

type UserPreferencesHandler struct {
	svc port.UserPreferencesService
	log *slog.Logger
}

func NewUserPreferencesHandler(svc port.UserPreferencesService, log *slog.Logger) *UserPreferencesHandler {
	return &UserPreferencesHandler{svc: svc, log: log}
}

// @Summary		Get user preferences
// @Description	Returns the current user's account preferences
// @Tags			settings
// @Produce		json
// @Success		200	{object}	dto.UserPreferencesResponse
// @Failure		401	{object}	transport.ErrorResponse
// @Router			/settings/preferences [get]
func (h *UserPreferencesHandler) Get(w http.ResponseWriter, r *http.Request) {
	userID, ok := transport.UserIDFromContext(r.Context())
	if !ok || userID == "" {
		transport.LocalizedErrorJSON(w, r, http.StatusUnauthorized, "auth_error", "ErrUnauthorized")
		return
	}

	prefs, err := h.svc.Get(r.Context(), userID)
	if err != nil {
		h.log.Error("get user preferences", "error", err)
		transport.ErrorJSON(w, r, http.StatusInternalServerError, "internal_error", "internal error")
		return
	}

	transport.JSON(w, r, http.StatusOK, dto.UserPreferencesResponse{
		Theme:                prefs.Theme,
		Language:             prefs.Language,
		Timezone:             prefs.Timezone,
		EmailNotifications:   prefs.EmailNotifications,
		DesktopNotifications: prefs.DesktopNotifications,
		NotificationSounds:   prefs.NotificationSounds,
		SidebarCollapsed:     prefs.SidebarCollapsed,
		MotionSettings:       prefs.MotionSettings,
	})
}

// @Summary		Update user preferences
// @Description	Partially updates the current user's account preferences
// @Tags			settings
// @Accept			json
// @Produce		json
// @Param			body	body		dto.UpdateUserPreferencesRequest	true	"Preference updates"
// @Success		200		{object}	dto.UserPreferencesResponse
// @Failure		400		{object}	transport.ErrorResponse
// @Failure		401		{object}	transport.ErrorResponse
// @Router			/settings/preferences [patch]
func (h *UserPreferencesHandler) Update(w http.ResponseWriter, r *http.Request) {
	userID, ok := transport.UserIDFromContext(r.Context())
	if !ok || userID == "" {
		transport.LocalizedErrorJSON(w, r, http.StatusUnauthorized, "auth_error", "ErrUnauthorized")
		return
	}

	var req dto.UpdateUserPreferencesRequest
	if err := dto.BindAndValidate(r, &req); err != nil {
		transport.ErrorJSON(w, r, http.StatusBadRequest, "validation_error", err.Error())
		return
	}

	params := domain.UpdateUserPreferencesParams{
		Theme:                req.Theme,
		Language:             req.Language,
		Timezone:             req.Timezone,
		EmailNotifications:   req.EmailNotifications,
		DesktopNotifications: req.DesktopNotifications,
		NotificationSounds:   req.NotificationSounds,
		SidebarCollapsed:     req.SidebarCollapsed,
		MotionSettings:       req.MotionSettings,
	}

	prefs, err := h.svc.Update(r.Context(), userID, params)
	if err != nil {
		transport.RespondWithError(w, r, h.log, err)
		return
	}

	transport.JSON(w, r, http.StatusOK, dto.UserPreferencesResponse{
		Theme:                prefs.Theme,
		Language:             prefs.Language,
		Timezone:             prefs.Timezone,
		EmailNotifications:   prefs.EmailNotifications,
		DesktopNotifications: prefs.DesktopNotifications,
		NotificationSounds:   prefs.NotificationSounds,
		SidebarCollapsed:     prefs.SidebarCollapsed,
		MotionSettings:       prefs.MotionSettings,
	})
}
