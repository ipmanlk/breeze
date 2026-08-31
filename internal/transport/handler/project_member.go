package handler

import (
	"log/slog"
	"net/http"
	"strconv"

	"ipmanlk/plume/internal/domain"
	"ipmanlk/plume/internal/port"
	"ipmanlk/plume/internal/transport"
	"ipmanlk/plume/internal/transport/access"
	"ipmanlk/plume/internal/transport/dto"

	"github.com/go-chi/chi/v5"
)

type ProjectMemberHandler struct {
	svc       port.ProjectMemberService
	accessSvc port.AccessService

	audit port.AuditService
	log   *slog.Logger
}

func NewProjectMemberHandler(svc port.ProjectMemberService, accessSvc port.AccessService, audit port.AuditService, log *slog.Logger) *ProjectMemberHandler {
	return &ProjectMemberHandler{svc: svc, accessSvc: accessSvc, audit: audit, log: log}
}

// @Summary		List project members
// @Description	Returns paginated users who have access to this project (implicit + explicit)
// @Tags			project-members
// @Produce		json
// @Param			id		path		string	true	"Project ID"
// @Param			cursor	query		string	false	"Pagination cursor"
// @Param			search	query		string	false	"Search by name"
// @Param			limit	query		int		false	"Items per page"	default(20)
// @Success		200		{object}	dto.PaginatedProjectMembersResponse
// @Router			/projects/{id}/members [get]
func (h *ProjectMemberHandler) List(w http.ResponseWriter, r *http.Request) {
	orgID, _ := transport.OrgIDFromContext(r.Context())
	projectID := chi.URLParam(r, "id")

	if err := EnsureProjectAccess(r.Context(), h.accessSvc, projectID); err != nil {
		transport.RespondWithError(w, r, h.log, err)
		return
	}

	filter := domain.UserFilter{
		Cursor: r.URL.Query().Get("cursor"),
		Search: r.URL.Query().Get("search"),
		Limit:  20,
	}
	if l, err := strconv.Atoi(r.URL.Query().Get("limit")); err == nil && l > 0 && l <= 100 {
		filter.Limit = l
	}

	result, err := h.svc.List(r.Context(), orgID, projectID, filter)
	if err != nil {
		transport.ServerError(w, r, h.log, err)
		return
	}

	resp := dto.PaginatedProjectMembersResponse{
		Items:      make([]*dto.ProjectMemberResponse, len(result.Members)),
		NextCursor: result.NextCursor,
		HasMore:    result.HasMore,
	}
	for i, m := range result.Members {
		orgRole := m.User.Role
		projectRole := m.Role
		resp.Items[i] = &dto.ProjectMemberResponse{
			ID:              m.User.ID,
			Name:            m.User.Name,
			Email:           m.User.Email,
			AvatarURL:       m.User.AvatarURL,
			OrgRole:         string(orgRole),
			ProjectRole:     string(projectRole),
			Role:            string(access.EffectiveRoleFor(orgRole, projectRole)),
			RoleOverridable: !domain.IsOrgElevatedRole(orgRole),
		}
	}
	transport.JSON(w, r, http.StatusOK, resp)
}

// @Summary		Add a member to a project
// @Description	Grants a user access to a project with a specific role
// @Tags			project-members
// @Accept			json
// @Produce		json
// @Param			id		path	string						true	"Project ID"
// @Param			body	body	dto.AddProjectMemberRequest	true	"User ID and role"
// @Success		201		{object}	transport.SuccessResponse
// @Failure		400	{object}	transport.ErrorResponse
// @Router			/projects/{id}/members [post]
func (h *ProjectMemberHandler) Add(w http.ResponseWriter, r *http.Request) {
	orgID, _ := transport.OrgIDFromContext(r.Context())
	projectID := chi.URLParam(r, "id")

	if err := EnsureProjectAccess(r.Context(), h.accessSvc, projectID); err != nil {
		transport.RespondWithError(w, r, h.log, err)
		return
	}

	var req dto.AddProjectMemberRequest
	if err := dto.BindAndValidate(r, &req); err != nil {
		transport.ErrorJSON(w, r, http.StatusBadRequest, "validation_error", err.Error())
		return
	}

	if err := h.svc.Add(r.Context(), orgID, projectID, req.UserID, domain.Role(req.Role)); err != nil {
		transport.RespondWithError(w, r, h.log, err)
		return
	}

	transport.JSON(w, r, http.StatusCreated, transport.SuccessResponse{Success: true})
}

// @Summary		Remove a member from a project
// @Description	Revokes a user's access to a project
// @Tags			project-members
// @Produce		json
// @Param			id		path	string	true	"Project ID"
// @Param			userId	path	string	true	"User ID"
// @Success		204
// @Router			/projects/{id}/members/{userId} [delete]
func (h *ProjectMemberHandler) Remove(w http.ResponseWriter, r *http.Request) {
	orgID, _ := transport.OrgIDFromContext(r.Context())
	projectID := chi.URLParam(r, "id")
	userID := chi.URLParam(r, "userId")

	if err := EnsureProjectAccess(r.Context(), h.accessSvc, projectID); err != nil {
		transport.RespondWithError(w, r, h.log, err)
		return
	}

	if err := h.svc.Remove(r.Context(), orgID, projectID, userID); err != nil {
		transport.RespondWithError(w, r, h.log, err)
		return
	}

	callerID, _ := transport.UserIDFromContext(r.Context())
	h.audit.Record(r.Context(), orgID, callerID, domain.AuditActionMemberRemoved, "project_member", userID,
		map[string]string{"project_id": projectID})

	w.WriteHeader(http.StatusNoContent)
}

// @Summary		Update a member's role in a project
// @Description	Changes the role of an existing project member
// @Tags			project-members
// @Accept			json
// @Produce		json
// @Param			id		path		string								true	"Project ID"
// @Param			userId	path		string								true	"User ID"
// @Param			body	body		dto.UpdateProjectMemberRoleRequest	true	"New role"
// @Success		200		{object}	transport.SuccessResponse
// @Router			/projects/{id}/members/{userId} [put]
func (h *ProjectMemberHandler) UpdateRole(w http.ResponseWriter, r *http.Request) {
	orgID, _ := transport.OrgIDFromContext(r.Context())
	projectID := chi.URLParam(r, "id")
	userID := chi.URLParam(r, "userId")

	if err := EnsureProjectAccess(r.Context(), h.accessSvc, projectID); err != nil {
		transport.RespondWithError(w, r, h.log, err)
		return
	}

	var req dto.UpdateProjectMemberRoleRequest
	if err := dto.BindAndValidate(r, &req); err != nil {
		transport.ErrorJSON(w, r, http.StatusBadRequest, "validation_error", err.Error())
		return
	}

	if err := h.svc.UpdateRole(r.Context(), orgID, projectID, userID, domain.Role(req.Role)); err != nil {
		transport.RespondWithError(w, r, h.log, err)
		return
	}

	callerID, _ := transport.UserIDFromContext(r.Context())
	h.audit.Record(r.Context(), orgID, callerID, domain.AuditActionMemberRoleChanged, "project_member", userID,
		map[string]string{"project_id": projectID, "new_role": req.Role})

	transport.JSON(w, r, http.StatusOK, transport.SuccessResponse{Success: true})
}
