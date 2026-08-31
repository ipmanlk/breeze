package handler

import (
	"log/slog"
	"net/http"

	"ipmanlk/plume/internal/port"
	"ipmanlk/plume/internal/transport"
	"ipmanlk/plume/internal/transport/dto"
)

// AccountHandler exposes self-service account endpoints: edit display name,
// upload an avatar, and change password. All routes are available to any
// authenticated user for their own account (no org permission required).
type AccountHandler struct {
	userSvc port.UserService
	log     *slog.Logger
}

func NewAccountHandler(userSvc port.UserService, log *slog.Logger) *AccountHandler {
	return &AccountHandler{userSvc: userSvc, log: log}
}

// @Summary		Update own profile
// @Description	Changes the caller's display name (synced across all workspaces)
// @Tags			account
// @Accept			json
// @Produce		json
// @Param			body	body		dto.UpdateProfileRequest	true	"Profile update"
// @Success		200		{object}	dto.UserResponse
// @Failure		400		{object}	transport.ErrorResponse
// @Failure		401		{object}	transport.ErrorResponse
// @Router			/account [patch]
func (h *AccountHandler) UpdateProfile(w http.ResponseWriter, r *http.Request) {
	orgID, ok := transport.OrgIDFromContext(r.Context())
	if !ok || orgID == "" {
		transport.LocalizedErrorJSON(w, r, http.StatusUnauthorized, "auth_error", "ErrUnauthorized")
		return
	}
	userID, ok := transport.UserIDFromContext(r.Context())
	if !ok || userID == "" {
		transport.LocalizedErrorJSON(w, r, http.StatusUnauthorized, "auth_error", "ErrUnauthorized")
		return
	}

	var req dto.UpdateProfileRequest
	if err := dto.BindAndValidate(r, &req); err != nil {
		transport.ErrorJSON(w, r, http.StatusBadRequest, "validation_error", err.Error())
		return
	}

	user, err := h.userSvc.UpdateProfile(r.Context(), orgID, userID, req.Name, nil)
	if err != nil {
		transport.RespondWithError(w, r, h.log, err)
		return
	}

	transport.JSON(w, r, http.StatusOK, dto.NewUserResponse(user))
}

// @Summary		Upload avatar
// @Description	Uploads an avatar image for the caller's account (synced across all workspaces)
// @Tags			account
// @Accept			mpfd
// @Produce		json
// @Param			file	formData	file	true	"Image file to upload"
// @Success		200		{object}	dto.UserResponse
// @Failure		400		{object}	transport.ErrorResponse
// @Failure		401		{object}	transport.ErrorResponse
// @Router			/account/avatar [post]
func (h *AccountHandler) UploadAvatar(w http.ResponseWriter, r *http.Request) {
	orgID, ok := transport.OrgIDFromContext(r.Context())
	if !ok || orgID == "" {
		transport.LocalizedErrorJSON(w, r, http.StatusUnauthorized, "auth_error", "ErrUnauthorized")
		return
	}
	userID, ok := transport.UserIDFromContext(r.Context())
	if !ok || userID == "" {
		transport.LocalizedErrorJSON(w, r, http.StatusUnauthorized, "auth_error", "ErrUnauthorized")
		return
	}

	// The route-level LimitRequestBody(5MB) middleware wraps r.Body with
	// MaxBytesReader, so ParseMultipartForm enforces the cap.
	if err := r.ParseMultipartForm(5 << 20); err != nil {
		transport.LocalizedErrorJSON(w, r, http.StatusBadRequest, "validation_error", "ErrInvalidMultipart")
		return
	}

	file, header, err := r.FormFile("file")
	if err != nil {
		transport.LocalizedErrorJSON(w, r, http.StatusBadRequest, "validation_error", "ErrFileRequired")
		return
	}
	defer file.Close()

	contentType := header.Header.Get("Content-Type")
	if contentType == "" {
		contentType = "application/octet-stream"
	}

	user, err := h.userSvc.UploadAvatar(r.Context(), orgID, userID, file, header.Filename, contentType, header.Size)
	if err != nil {
		transport.RespondWithError(w, r, h.log, err)
		return
	}

	transport.JSON(w, r, http.StatusOK, dto.NewUserResponse(user))
}

// @Summary		Change password
// @Description	Changes the caller's password (current password required); revokes sessions on success
// @Tags			account
// @Accept			json
// @Produce		json
// @Param			body	body		dto.ChangePasswordRequest	true	"Password change"
// @Success		200		{object}	transport.SuccessResponse
// @Failure		400		{object}	transport.ErrorResponse
// @Failure		401		{object}	transport.ErrorResponse
// @Router			/account/change-password [post]
func (h *AccountHandler) ChangePassword(w http.ResponseWriter, r *http.Request) {
	orgID, ok := transport.OrgIDFromContext(r.Context())
	if !ok || orgID == "" {
		transport.LocalizedErrorJSON(w, r, http.StatusUnauthorized, "auth_error", "ErrUnauthorized")
		return
	}
	userID, ok := transport.UserIDFromContext(r.Context())
	if !ok || userID == "" {
		transport.LocalizedErrorJSON(w, r, http.StatusUnauthorized, "auth_error", "ErrUnauthorized")
		return
	}

	var req dto.ChangePasswordRequest
	if err := dto.BindAndValidate(r, &req); err != nil {
		transport.ErrorJSON(w, r, http.StatusBadRequest, "validation_error", err.Error())
		return
	}

	if err := h.userSvc.ChangePassword(r.Context(), orgID, userID, req.CurrentPassword, req.NewPassword); err != nil {
		transport.RespondWithError(w, r, h.log, err)
		return
	}

	transport.JSON(w, r, http.StatusOK, transport.SuccessResponse{Success: true})
}
