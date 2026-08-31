package handler

import (
	"context"
	"fmt"
	"log/slog"
	"net/http"

	"ipmanlk/plume/internal/domain"
	"ipmanlk/plume/internal/port"
	"ipmanlk/plume/internal/transport"
	"ipmanlk/plume/internal/transport/dto"

	"github.com/go-chi/chi/v5"
)

type AuthHandler struct {
	auth port.AuthService
	user port.UserService
	org  port.OrganizationService
	log  *slog.Logger
}

func NewAuthHandler(auth port.AuthService, user port.UserService, org port.OrganizationService, log *slog.Logger) *AuthHandler {
	return &AuthHandler{auth: auth, user: user, org: org, log: log}
}

// @Summary		Login
// @Description	Authenticate with email and password, sets HttpOnly cookie
// @Tags			auth
// @Accept			json
// @Produce		json
// @Param			body	body		dto.LoginRequest	true	"Credentials"
// @Success		200		{object}	dto.UserResponse
// @Failure		401		{object}	transport.ErrorResponse
// @Router			/auth/login [post]
func (h *AuthHandler) Login(w http.ResponseWriter, r *http.Request) {
	var req dto.LoginRequest
	if err := dto.BindAndValidate(r, &req); err != nil {
		transport.ErrorJSON(w, r, http.StatusBadRequest, "validation_error", err.Error())
		return
	}

	_, memberships, token, err := h.auth.Login(r.Context(), domain.LoginParams{
		Email:     req.Email,
		Password:  req.Password,
		UserAgent: transport.UserAgent(r),
		IPAddress: transport.ClientIP(r),
	})
	if err != nil {
		transport.RespondWithError(w, r, h.log, err)
		return
	}

	transport.SetAuthCookie(w, token, 7*24*60*60)

	// The login session is scoped to the most-recent active membership; find
	// it to build the /auth/me-shaped response.
	var active *domain.User
	for _, m := range memberships {
		if m.IsActive {
			active = m
			break
		}
	}
	resp := dto.NewUserResponse(active)
	if org, err := h.org.GetByID(r.Context(), active.OrgID); err == nil && org != nil {
		resp.Org = &dto.OrgSummaryResponse{ID: org.ID, Name: org.Name, Slug: org.Slug}
	}
	resp.ActiveOrgID = active.OrgID
	resp.Workspaces = h.buildWorkspaces(r.Context(), memberships, active.OrgID)

	transport.JSON(w, r, http.StatusOK, resp)
}

// @Summary		Logout
// @Description	Clears the auth cookie and invalidates the session
// @Tags			auth
// @Produce		json
// @Success		200	{object}	transport.SuccessResponse
// @Router			/auth/logout [post]
func (h *AuthHandler) Logout(w http.ResponseWriter, r *http.Request) {
	// Prevent the browser from caching this response; a stale cached copy
	// could be served on pages that check authentication state before the
	// cookie removal is reflected.
	w.Header().Set("Cache-Control", "no-store")

	sessionID, ok := transport.SessionIDFromContext(r.Context())
	if ok && sessionID != "" {
		if err := h.auth.Logout(r.Context(), sessionID); err != nil {
			h.log.Error("logout: failed to revoke session", "error", err, "session_id", sessionID)
		}
	}
	transport.SetAuthCookie(w, "", -1)
	transport.JSON(w, r, http.StatusOK, map[string]bool{"success": true})
}

// @Summary		Get current user
// @Description	Returns the authenticated user's profile with org info
// @Tags			auth
// @Produce		json
// @Success		200	{object}	dto.UserResponse
// @Failure		401	{object}	transport.ErrorResponse
// @Router			/auth/me [get]
func (h *AuthHandler) Me(w http.ResponseWriter, r *http.Request) {
	userID, ok := transport.UserIDFromContext(r.Context())
	if !ok || userID == "" {
		transport.LocalizedErrorJSON(w, r, http.StatusUnauthorized, "auth_error", "ErrUnauthorized")
		return
	}

	orgID, _ := transport.OrgIDFromContext(r.Context())
	user, err := h.user.GetByID(r.Context(), orgID, userID)
	if err != nil {
		h.log.Error("get user", "error", err)
		transport.LocalizedErrorJSON(w, r, http.StatusNotFound, "not_found", "ErrUserNotFound")
		return
	}

	resp := dto.NewUserResponse(user)
	org, err := h.org.GetByID(r.Context(), user.OrgID)
	if err == nil && org != nil {
		resp.Org = &dto.OrgSummaryResponse{
			ID:   org.ID,
			Name: org.Name,
			Slug: org.Slug,
		}
	}
	resp.ActiveOrgID = user.OrgID

	// Include the caller's full workspace list so the switcher works on reload.
	if workspaces, err := h.org.ListForAccount(r.Context(), user.AccountID); err == nil {
		resp.Workspaces = make([]*dto.WorkspaceResponse, len(workspaces))
		for i, ws := range workspaces {
			resp.Workspaces[i] = dto.NewWorkspaceResponse(
				ws.Organization.ID, ws.Organization.Name, ws.Organization.Slug,
				string(ws.Role), ws.IsOwner, ws.Organization.CreatedAt,
			)
		}
	}

	transport.JSON(w, r, http.StatusOK, resp)
}

// buildWorkspaces maps an account's memberships into the switcher list DTO,
// marking the active workspace. Used by Login (which already has the
// memberships in memory) to avoid a second ListForAccount round-trip.
func (h *AuthHandler) buildWorkspaces(ctx context.Context, memberships []*domain.User, activeOrgID string) []*dto.WorkspaceResponse {
	out := make([]*dto.WorkspaceResponse, 0, len(memberships))
	for _, m := range memberships {
		org, err := h.org.GetByID(ctx, m.OrgID)
		if err != nil || org == nil {
			continue
		}
		out = append(out, dto.NewWorkspaceResponse(
			org.ID, org.Name, org.Slug,
			string(m.Role), m.Role == domain.RoleOwner, org.CreatedAt,
		))
	}
	return out
}

// @Summary		Request password reset
// @Description	Generates a password reset token (logged server-side; no SMTP configured for self-hosters)
// @Tags			auth
// @Accept			json
// @Produce		json
// @Param			body	body		dto.RequestPasswordResetRequest	true	"Email"
// @Success		200		{object}	transport.SuccessResponse
// @Router			/auth/password-reset/request [post]
func (h *AuthHandler) RequestPasswordReset(w http.ResponseWriter, r *http.Request) {
	var req dto.RequestPasswordResetRequest
	if err := dto.BindAndValidate(r, &req); err != nil {
		transport.ErrorJSON(w, r, http.StatusBadRequest, "validation_error", err.Error())
		return
	}

	token, err := h.auth.RequestPasswordReset(r.Context(), req.Email)
	if err != nil {
		h.log.Error("request password reset", "error", err)
	}

	// When SMTP is configured the service emails the reset link directly and
	// returns an empty token, so this log is skipped. In air-gapped mode (no
	// SMTP) the token is returned and logged here as the fallback.
	if token != "" {
		scheme := "http"
		if r.TLS != nil {
			scheme = "https"
		}
		// Respect the X-Forwarded-Proto header set by reverse proxies.
		if xfp := r.Header.Get("X-Forwarded-Proto"); xfp != "" {
			scheme = xfp
		}
		h.log.Info("password reset link", "url", fmt.Sprintf("%s://%s/reset-password?token=%s", scheme, r.Host, token))
	}

	// Always return success to avoid leaking whether the email exists.
	transport.JSON(w, r, http.StatusOK, transport.SuccessResponse{Success: true})
}

// @Summary		Confirm password reset
// @Description	Completes a password reset with the token from the reset link and a new password
// @Tags			auth
// @Accept			json
// @Produce		json
// @Param			body	body		dto.ConfirmPasswordResetRequest	true	"Token + new password"
// @Success		200		{object}	transport.SuccessResponse
// @Failure		400		{object}	transport.ErrorResponse
// @Router			/auth/password-reset/confirm [post]
func (h *AuthHandler) ConfirmPasswordReset(w http.ResponseWriter, r *http.Request) {
	var req dto.ConfirmPasswordResetRequest
	if err := dto.BindAndValidate(r, &req); err != nil {
		transport.ErrorJSON(w, r, http.StatusBadRequest, "validation_error", err.Error())
		return
	}

	if err := h.auth.ConfirmPasswordReset(r.Context(), req.Token, req.NewPassword); err != nil {
		transport.RespondWithError(w, r, h.log, err)
		return
	}

	transport.JSON(w, r, http.StatusOK, transport.SuccessResponse{Success: true})
}

// @Summary		List active sessions
// @Description	Returns the caller's active + recently-revoked sessions
// @Tags			auth
// @Produce		json
// @Success		200	{array}	dto.SessionResponse
// @Router			/auth/sessions [get]
func (h *AuthHandler) ListSessions(w http.ResponseWriter, r *http.Request) {
	userID, ok := transport.UserIDFromContext(r.Context())
	if !ok {
		transport.LocalizedErrorJSON(w, r, http.StatusUnauthorized, "unauthorized", "ErrNoUserInContext")
		return
	}
	sessions, err := h.auth.ListSessions(r.Context(), userID)
	if err != nil {
		transport.ServerError(w, r, h.log, err)
		return
	}
	currentSessionID, _ := transport.SessionIDFromContext(r.Context())
	resp := make([]*dto.SessionResponse, len(sessions))
	for i, s := range sessions {
		resp[i] = dto.NewSessionResponse(s)
		resp[i].IsCurrent = s.ID == currentSessionID
	}
	transport.JSON(w, r, http.StatusOK, resp)
}

// @Summary		Revoke a session
// @Description	Revokes one of the caller's other sessions ("log out" that device). The current session cannot be revoked here; use /auth/logout.
// @Tags			auth
// @Produce		json
// @Param			id	path	string	true	"Session ID"
// @Success		200	{object}	transport.SuccessResponse
// @Failure		400	{object}	transport.ErrorResponse
// @Failure		404	{object}	transport.ErrorResponse
// @Router			/auth/sessions/{id} [delete]
func (h *AuthHandler) RevokeSession(w http.ResponseWriter, r *http.Request) {
	userID, ok := transport.UserIDFromContext(r.Context())
	if !ok {
		transport.LocalizedErrorJSON(w, r, http.StatusUnauthorized, "unauthorized", "ErrNoUserInContext")
		return
	}
	sessionID := chi.URLParam(r, "id")
	if sessionID == "" {
		transport.LocalizedErrorJSON(w, r, http.StatusBadRequest, "validation_error", "ErrSessionIDRequired")
		return
	}
	// Guard: never let a user revoke the session this request is riding on
	//: that's what /auth/logout is for, and doing it here would leave the
	// caller's cookie valid but the row revoked (confusing UX).
	if current, ok := transport.SessionIDFromContext(r.Context()); ok && current == sessionID {
		transport.LocalizedErrorJSON(w, r, http.StatusBadRequest, "validation_error", "ErrSessionExists")
		return
	}
	if err := h.auth.RevokeSession(r.Context(), userID, sessionID); err != nil {
		transport.RespondWithError(w, r, h.log, err)
		return
	}
	transport.JSON(w, r, http.StatusOK, transport.SuccessResponse{Success: true})
}

// @Summary		Validate password reset token
// @Description	Checks if a password-reset token is valid (exists, not expired, not used) without consuming it. Returns 200 with valid=true or 400 with valid=false.
// @Tags			auth
// @Produce		json
// @Param			token	query	string	true	"Reset token from the email"
// @Success		200	{object}	map[string]bool	"{\"valid\": true}"
// @Failure		400	{object}	map[string]bool	"{\"valid\": false}"
// @Router			/auth/password-reset/validate [get]
func (h *AuthHandler) ValidateResetToken(w http.ResponseWriter, r *http.Request) {
	// Prefer POST body (prevents token leakage in URLs), fall back to query
	// param for backward compatibility with email links.
	var req struct {
		Token string `json:"token"`
	}
	token := ""
	if err := dto.BindAndValidate(r, &req); err == nil && req.Token != "" {
		token = req.Token
	} else {
		token = r.URL.Query().Get("token")
	}
	if token == "" {
		transport.JSON(w, r, http.StatusBadRequest, map[string]bool{"valid": false})
		return
	}

	if err := h.auth.ValidateResetToken(r.Context(), token); err != nil {
		transport.JSON(w, r, http.StatusBadRequest, map[string]bool{"valid": false})
		return
	}

	transport.JSON(w, r, http.StatusOK, map[string]bool{"valid": true})
}
