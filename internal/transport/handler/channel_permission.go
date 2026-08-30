package handler

import (
	"log/slog"
	"net/http"

	"github.com/go-chi/chi/v5"

	"ipmanlk/breeze/internal/domain"
	"ipmanlk/breeze/internal/port"
	"ipmanlk/breeze/internal/transport"
	"ipmanlk/breeze/internal/transport/dto"
)

type ChannelPermissionHandler struct {
	svc       port.ChannelPermissionService
	permSvc   port.ChannelPermissionService
	accessSvc port.AccessService
	log       *slog.Logger
}

func NewChannelPermissionHandler(svc port.ChannelPermissionService, accessSvc port.AccessService, log *slog.Logger) *ChannelPermissionHandler {
	return &ChannelPermissionHandler{svc: svc, permSvc: svc, accessSvc: accessSvc, log: log}
}

// @Summary	Get channel permissions
// @Tags		chat-permissions
// @Produce	json
// @Param		id	path	string	true	"Channel ID"
// @Success	200	{array}	dto.EffectivePermissionResponse
// @Failure		500		{object}	transport.ErrorResponse
// @Router		/conversations/{id}/permissions [get]
func (h *ChannelPermissionHandler) GetPermissions(w http.ResponseWriter, r *http.Request) {
	id := chi.URLParam(r, "id")
	orgID, _ := transport.OrgIDFromContext(r.Context())

	if !EnsureConversationAccess(w, r, h.accessSvc, id) {
		return
	}

	effective, err := h.svc.ResolveRolePermissions(r.Context(), orgID, id)
	if err != nil {
		transport.ServerError(w, r, h.log, err)
		return
	}

	resp := make([]*dto.EffectivePermissionResponse, len(effective))
	for i, e := range effective {
		resp[i] = &dto.EffectivePermissionResponse{
			Role:       string(e.Role),
			Permission: string(e.Permission),
			Allow:      e.Allow,
			Explicit:   e.Explicit,
		}
	}
	transport.JSON(w, r, http.StatusOK, resp)
}

// @Summary	Set channel permissions
// @Tags		chat-permissions
// @Accept		json
// @Produce	json
// @Param		id		path	string						true	"Channel ID"
// @Param		body	body	dto.SetPermissionsRequest	true	"Permission rules"
// @Success	200		{object}	transport.SuccessResponse
// @Failure		400		{object}	transport.ErrorResponse
// @Failure		500		{object}	transport.ErrorResponse
// @Router		/conversations/{id}/permissions [put]
func (h *ChannelPermissionHandler) SetPermissions(w http.ResponseWriter, r *http.Request) {
	id := chi.URLParam(r, "id")

	var req dto.SetPermissionsRequest
	if err := dto.BindAndValidate(r, &req); err != nil {
		transport.ErrorJSON(w, r, http.StatusBadRequest, "validation_error", err.Error())
		return
	}

	roleStr, _ := transport.RoleFromContext(r.Context())
	requesterRole := domain.Role(roleStr)

	// Admin can only configure permissions for roles below admin (member,
	// viewer, guest). They cannot set rules for owner or admin roles.
	if requesterRole == domain.RoleAdmin {
		for _, rule := range req.Rules {
			if rule.Role == "owner" || rule.Role == "admin" {
				transport.LocalizedErrorJSON(w, r, http.StatusForbidden, "forbidden", "ErrCannotSetOwnerAdminPerms")
				return
			}
		}
	}

	rules := make([]*domain.PermissionRule, len(req.Rules))
	for i, r := range req.Rules {
		rules[i] = &domain.PermissionRule{
			ChannelID:  &id,
			Role:       domain.Role(r.Role),
			Permission: domain.Permission(r.Permission),
			Allow:      r.Allow,
		}
	}

	// Requires resolved channel:manage permission. Org owners/admins always
	// pass (they are immune to channel-level rules), so they can configure
	// permissions on channels they have not joined.
	if !EnsureConversationManageAccess(w, r, h.accessSvc, id) {
		return
	}

	if err := h.svc.SetPermissions(r.Context(), id, rules); err != nil {
		transport.ServerError(w, r, h.log, err)
		return
	}
	transport.JSON(w, r, http.StatusOK, transport.SuccessResponse{Success: true})
}

// @Summary	Get channel user overrides
// @Tags		chat-permissions
// @Produce	json
// @Param		id	path	string	true	"Channel ID"
// @Success	200	{array}	dto.UserOverrideResponse
// @Failure		500		{object}	transport.ErrorResponse
// @Router		/conversations/{id}/user-overrides [get]
func (h *ChannelPermissionHandler) GetUserOverrides(w http.ResponseWriter, r *http.Request) {
	id := chi.URLParam(r, "id")

	if !EnsureConversationAccess(w, r, h.accessSvc, id) {
		return
	}

	overrides, err := h.svc.GetUserOverrides(r.Context(), id)
	if err != nil {
		transport.ServerError(w, r, h.log, err)
		return
	}

	resp := make([]*dto.UserOverrideResponse, len(overrides))
	for i, o := range overrides {
		resp[i] = dto.NewUserOverrideResponse(o)
	}
	transport.JSON(w, r, http.StatusOK, resp)
}

// @Summary	Set channel user overrides
// @Tags		chat-permissions
// @Accept		json
// @Produce	json
// @Param		id		path	string						true	"Channel ID"
// @Param		body	body	dto.SetUserOverridesRequest	true	"User overrides"
// @Success	200		{object}	transport.SuccessResponse
// @Failure		400		{object}	transport.ErrorResponse
// @Failure		500		{object}	transport.ErrorResponse
// @Router		/conversations/{id}/user-overrides [put]
func (h *ChannelPermissionHandler) SetUserOverrides(w http.ResponseWriter, r *http.Request) {
	id := chi.URLParam(r, "id")

	var req dto.SetUserOverridesRequest
	if err := dto.BindAndValidate(r, &req); err != nil {
		transport.ErrorJSON(w, r, http.StatusBadRequest, "validation_error", err.Error())
		return
	}

	// Requires resolved channel:manage permission. Org owners/admins always
	// pass (they are immune to channel-level rules), so they can configure
	// overrides on channels they have not joined.
	if !EnsureConversationManageAccess(w, r, h.accessSvc, id) {
		return
	}

	overrides := make([]*domain.UserPermissionOverride, len(req.Overrides))
	for i, o := range req.Overrides {
		overrides[i] = &domain.UserPermissionOverride{
			ChannelID:  id,
			UserID:     o.UserID,
			Permission: domain.Permission(o.Permission),
			Allow:      o.Allow,
		}
	}

	if err := h.svc.SetUserOverrides(r.Context(), id, overrides); err != nil {
		transport.RespondWithError(w, r, h.log, err)
		return
	}
	transport.JSON(w, r, http.StatusOK, transport.SuccessResponse{Success: true})
}

// @Summary	Resolve current user's channel permissions
// @Tags		chat-permissions
// @Produce	json
// @Param		id	path	string	true	"Channel ID"
// @Success	200	{object}	dto.ChannelPermissionsResponse
// @Failure		500		{object}	transport.ErrorResponse
// @Router		/conversations/{id}/my-permissions [get]
func (h *ChannelPermissionHandler) ResolvePermissions(w http.ResponseWriter, r *http.Request) {
	userID, _ := transport.UserIDFromContext(r.Context())
	orgID, _ := transport.OrgIDFromContext(r.Context())
	roleStr, _ := transport.RoleFromContext(r.Context())
	id := chi.URLParam(r, "id")

	// Gate on view access so callers can't probe arbitrary channel IDs to
	// learn they exist and how their permissions resolve.
	if !EnsureConversationAccess(w, r, h.accessSvc, id) {
		return
	}

	perms, err := h.svc.ResolvePermissions(r.Context(), orgID, id, userID, domain.Role(roleStr))
	if err != nil {
		transport.RespondWithError(w, r, h.log, err)
		return
	}

	transport.JSON(w, r, http.StatusOK, dto.NewChannelPermissionsResponse(perms))
}
