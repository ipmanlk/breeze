package handler

import (
	"fmt"
	"log/slog"
	"net/http"

	"ipmanlk/plume/internal/apperr"
	"ipmanlk/plume/internal/domain"
	"ipmanlk/plume/internal/port"
	"ipmanlk/plume/internal/transport"
	"ipmanlk/plume/internal/transport/dto"

	"github.com/go-chi/chi/v5"
)

type InviteHandler struct {
	svc   port.UserInviteService
	audit port.AuditService
	log   *slog.Logger
}

func NewInviteHandler(svc port.UserInviteService, audit port.AuditService, log *slog.Logger) *InviteHandler {
	return &InviteHandler{svc: svc, audit: audit, log: log}
}

// @Summary		Create invite
// @Description	Creates a time-limited invite token for a new user
// @Tags			invites
// @Accept			json
// @Produce		json
// @Param			body	body		dto.CreateInviteRequest	true	"Invite details"
// @Success		201		{object}	dto.CreateInviteResponse
// @Failure		400		{object}	transport.ErrorResponse
// @Failure		403		{object}	transport.ErrorResponse
// @Router			/invites [post]
func (h *InviteHandler) Create(w http.ResponseWriter, r *http.Request) {
	var req dto.CreateInviteRequest
	if err := dto.BindAndValidate(r, &req); err != nil {
		transport.ErrorJSON(w, r, http.StatusBadRequest, "validation_error", err.Error())
		return
	}

	orgID, _ := transport.OrgIDFromContext(r.Context())
	callerID, _ := transport.UserIDFromContext(r.Context())
	callerRoleStr, _ := transport.RoleFromContext(r.Context())
	callerRole := domain.Role(callerRoleStr)

	role := domain.Role(req.Role)
	var emailPtr *string
	if req.Email != nil && *req.Email != "" {
		emailPtr = req.Email
	}

	projectAssignments := make([]domain.InviteProjectAssignment, len(req.ProjectAssignments))
	for i, a := range req.ProjectAssignments {
		projectAssignments[i] = domain.InviteProjectAssignment{
			ProjectID: a.ProjectID,
			Role:      domain.Role(a.Role),
		}
	}

	invite, token, err := h.svc.Create(r.Context(), domain.CreateInviteParams{
		OrgID:              orgID,
		InvitedBy:          callerID,
		Role:               role,
		Email:              emailPtr,
		ProjectAssignments: projectAssignments,
	}, callerRole)
	if err != nil {
		if err == apperr.ErrForbidden {
			transport.LocalizedErrorJSON(w, r, http.StatusForbidden, "forbidden", "ErrInvitePermissionDenied")
			return
		}
		h.log.Error("create invite", "error", err)
		transport.ErrorJSON(w, r, http.StatusInternalServerError, "internal_error", "internal error")
		return
	}

	resp := dto.CreateInviteResponse{
		ID:        invite.ID,
		Token:     token,
		URL:       fmt.Sprintf("/join?token=%s", token),
		Role:      string(invite.Role),
		ExpiresAt: invite.ExpiresAt,
	}
	transport.JSON(w, r, http.StatusCreated, resp)
}

// @Summary		List invites
// @Description	Returns pending invite tokens for the organization
// @Tags			invites
// @Produce		json
// @Success		200	{object}	[]dto.InviteResponse
// @Router			/invites [get]
func (h *InviteHandler) List(w http.ResponseWriter, r *http.Request) {
	orgID, _ := transport.OrgIDFromContext(r.Context())

	invites, err := h.svc.List(r.Context(), orgID, 50)
	if err != nil {
		h.log.Error("list invites", "error", err)
		transport.ErrorJSON(w, r, http.StatusInternalServerError, "internal_error", "internal error")
		return
	}

	resp := make([]*dto.InviteResponse, len(invites))
	for i, inv := range invites {
		resp[i] = dto.NewInviteResponse(inv, "")
	}
	transport.JSON(w, r, http.StatusOK, resp)
}

// @Summary		Revoke invite
// @Description	Deletes an invite token, preventing further use
// @Tags			invites
// @Produce		json
// @Param			id	path	string	true	"Invite ID"
// @Success		204
// @Failure		400	{object}	transport.ErrorResponse
// @Router			/invites/{id} [delete]
func (h *InviteHandler) Revoke(w http.ResponseWriter, r *http.Request) {
	orgID, _ := transport.OrgIDFromContext(r.Context())
	id := chi.URLParam(r, "id")

	if err := h.svc.Revoke(r.Context(), orgID, id); err != nil {
		h.log.Error("revoke invite", "error", err)
		transport.ErrorJSON(w, r, http.StatusInternalServerError, "internal_error", "internal error")
		return
	}
	callerID, _ := transport.UserIDFromContext(r.Context())
	h.audit.Record(r.Context(), orgID, callerID, domain.AuditActionInviteRevoked, "invite", id, nil)
	w.WriteHeader(http.StatusNoContent)
}

// @Summary		Validate invite
// @Description	Checks if an invite token is valid and not expired (public)
// @Tags			invites
// @Produce		json
// @Param			token	path	string	true	"Invite token"
// @Success		200		{object}	dto.InviteResponse
// @Failure		404		{object}	transport.ErrorResponse
// @Router			/invites/{token}/validate [get]
func (h *InviteHandler) Validate(w http.ResponseWriter, r *http.Request) {
	token := chi.URLParam(r, "token")

	invite, err := h.svc.Validate(r.Context(), token)
	if err != nil {
		if err == apperr.ErrNotFound || err == apperr.ErrSessionExpired {
			transport.LocalizedErrorJSON(w, r, http.StatusNotFound, "invalid_invite", "ErrInviteInvalid")
			return
		}
		h.log.Error("validate invite", "error", err)
		transport.ErrorJSON(w, r, http.StatusInternalServerError, "internal_error", "internal error")
		return
	}

	resp := dto.NewInviteResponse(invite, "")
	transport.JSON(w, r, http.StatusOK, resp)
}

// @Summary		Accept invite
// @Description	Registers a new user using an invite token (public)
// @Tags			invites
// @Accept			json
// @Produce		json
// @Param			token	path	string				true	"Invite token"
// @Param			body	body	dto.AcceptInviteRequest	true	"Registration details"
// @Success		201		{object}	dto.UserResponse
// @Failure		400		{object}	transport.ErrorResponse
// @Failure		404		{object}	transport.ErrorResponse
// @Failure		409		{object}	transport.ErrorResponse
// @Router			/invites/{token}/accept [post]
func (h *InviteHandler) Accept(w http.ResponseWriter, r *http.Request) {
	token := chi.URLParam(r, "token")

	var req dto.AcceptInviteRequest
	if err := dto.BindAndValidate(r, &req); err != nil {
		transport.ErrorJSON(w, r, http.StatusBadRequest, "validation_error", err.Error())
		return
	}

	user, _, err := h.svc.Accept(r.Context(), domain.AcceptInviteParams{
		Token:    token,
		Name:     req.Name,
		Email:    req.Email,
		Password: req.Password,
	})
	if err != nil {
		switch err {
		case apperr.ErrNotFound, apperr.ErrSessionExpired:
			transport.LocalizedErrorJSON(w, r, http.StatusNotFound, "invalid_invite", "ErrInviteInvalid")
			return
		case apperr.ErrForbidden:
			transport.LocalizedErrorJSON(w, r, http.StatusForbidden, "forbidden", "ErrInviteEmailMismatch")
			return
		case apperr.ErrAlreadyExists:
			transport.LocalizedErrorJSON(w, r, http.StatusConflict, "already_exists", "ErrUserAlreadyExists")
			return
		}
		h.log.Error("accept invite", "error", err)
		transport.ErrorJSON(w, r, http.StatusInternalServerError, "internal_error", "internal error")
		return
	}

	resp := dto.NewUserResponse(user)
	transport.JSON(w, r, http.StatusCreated, resp)
}
