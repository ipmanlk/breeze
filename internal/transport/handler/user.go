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

type UserHandler struct {
	svc   port.UserService
	pmSvc port.ProjectMemberService
	audit port.AuditService
	log   *slog.Logger
}

func NewUserHandler(svc port.UserService, pmSvc port.ProjectMemberService, audit port.AuditService, log *slog.Logger) *UserHandler {
	return &UserHandler{svc: svc, pmSvc: pmSvc, audit: audit, log: log}
}

// @Summary		List users
// @Description	Returns paginated users in the organization, optionally filtered by search, role, and active status
// @Tags			users
// @Produce		json
// @Param			cursor			query		string	false	"Pagination cursor"
// @Param			search			query		string	false	"Search by name"
// @Param			role			query		string	false	"Filter by role"
// @Param			include_inactive	query		bool	false	"Include deactivated users"
// @Param			limit			query		int		false	"Items per page" default(20)
// @Success		200				{object}	dto.PaginatedUsersResponse
// @Router			/users [get]
func (h *UserHandler) List(w http.ResponseWriter, r *http.Request) {
	orgID, _ := transport.OrgIDFromContext(r.Context())

	filter := domain.UserFilter{
		Cursor: r.URL.Query().Get("cursor"),
		Search: r.URL.Query().Get("search"),
		Role:   r.URL.Query().Get("role"),
		Limit:  20,
	}
	if l, err := strconv.Atoi(r.URL.Query().Get("limit")); err == nil && l > 0 && l <= 100 {
		filter.Limit = l
	}
	if r.URL.Query().Get("include_inactive") == "true" {
		filter.IncludeInactive = true
	}

	result, err := h.svc.ListUsers(r.Context(), orgID, filter)
	if err != nil {
		transport.ServerError(w, r, h.log, err)
		return
	}

	resp := dto.PaginatedUsersResponse{
		Items:      make([]*dto.UserResponse, len(result.Users)),
		NextCursor: result.NextCursor,
		HasMore:    result.HasMore,
	}
	for i, u := range result.Users {
		resp.Items[i] = dto.NewUserResponse(u)
	}
	transport.JSON(w, r, http.StatusOK, resp)
}

// @Summary		Get user
// @Description	Returns a single user by ID
// @Tags			users
// @Produce		json
// @Param			id	path	string	true	"User ID"
// @Success		200	{object}	dto.UserResponse
// @Failure		404	{object}	transport.ErrorResponse
// @Router			/users/{id} [get]
func (h *UserHandler) Get(w http.ResponseWriter, r *http.Request) {
	id := chi.URLParam(r, "id")
	if id == "" {
		transport.LocalizedErrorJSON(w, r, http.StatusBadRequest, "validation_error", "ErrIDRequired")
		return
	}

	orgID, _ := transport.OrgIDFromContext(r.Context())
	user, err := h.svc.GetByID(r.Context(), orgID, id)
	if err != nil {
		transport.RespondWithError(w, r, h.log, err)
		return
	}

	transport.JSON(w, r, http.StatusOK, dto.NewUserResponse(user))
}

// @Summary		Update user role
// @Description	Changes the organization role of a user
// @Tags			users
// @Accept			json
// @Produce		json
// @Param			id		path		string						true	"User ID"
// @Param			body	body		dto.UpdateUserRoleRequest	true	"Role update"
// @Success		200		{object}	dto.UserResponse
// @Failure		400		{object}	transport.ErrorResponse
// @Failure		403		{object}	transport.ErrorResponse
// @Router			/users/{id}/role [put]
func (h *UserHandler) UpdateRole(w http.ResponseWriter, r *http.Request) {
	id := chi.URLParam(r, "id")
	orgID, _ := transport.OrgIDFromContext(r.Context())
	callerID, _ := transport.UserIDFromContext(r.Context())
	callerRoleStr, _ := transport.RoleFromContext(r.Context())
	callerRole := domain.Role(callerRoleStr)

	var req dto.UpdateUserRoleRequest
	if err := dto.BindAndValidate(r, &req); err != nil {
		transport.ErrorJSON(w, r, http.StatusBadRequest, "validation_error", err.Error())
		return
	}

	newRole := domain.Role(req.Role)
	if err := h.svc.UpdateRole(r.Context(), orgID, id, newRole, callerRole, callerID); err != nil {
		transport.RespondWithError(w, r, h.log, err)
		return
	}

	h.audit.Record(r.Context(), orgID, callerID, domain.AuditActionRoleChanged, "user", id,
		map[string]string{"new_role": string(newRole)})

	user, err := h.svc.GetByID(r.Context(), orgID, id)
	if err != nil {
		transport.RespondWithError(w, r, h.log, err)
		return
	}

	transport.JSON(w, r, http.StatusOK, dto.NewUserResponse(user))
}

// @Summary		Update user active status
// @Description	Activates or deactivates a user account
// @Tags			users
// @Accept			json
// @Produce		json
// @Param			id		path		string						true	"User ID"
// @Param			body	body		dto.UpdateUserActiveRequest	true	"Active status"
// @Success		200		{object}	dto.UserResponse
// @Failure		400		{object}	transport.ErrorResponse
// @Failure		403		{object}	transport.ErrorResponse
// @Router			/users/{id}/active [put]
func (h *UserHandler) UpdateActive(w http.ResponseWriter, r *http.Request) {
	id := chi.URLParam(r, "id")
	orgID, _ := transport.OrgIDFromContext(r.Context())
	callerID, _ := transport.UserIDFromContext(r.Context())

	var req dto.UpdateUserActiveRequest
	if err := dto.BindAndValidate(r, &req); err != nil {
		transport.ErrorJSON(w, r, http.StatusBadRequest, "validation_error", err.Error())
		return
	}

	if err := h.svc.UpdateActive(r.Context(), orgID, id, req.IsActive, callerID); err != nil {
		transport.RespondWithError(w, r, h.log, err)
		return
	}

	action := domain.AuditActionUserActivated
	if !req.IsActive {
		action = domain.AuditActionUserDeactivated
	}
	h.audit.Record(r.Context(), orgID, callerID, action, "user", id, nil)

	user, err := h.svc.GetByID(r.Context(), orgID, id)
	if err != nil {
		transport.RespondWithError(w, r, h.log, err)
		return
	}

	transport.JSON(w, r, http.StatusOK, dto.NewUserResponse(user))
}

// @Summary		List user's project memberships
// @Description	Returns all projects the user is a member of
// @Tags			users
// @Produce		json
// @Param			id	path	string	true	"User ID"
// @Success		200	{array}	dto.UserProjectMembershipResponse
// @Failure		403	{object}	transport.ErrorResponse
// @Router			/users/{id}/project-memberships [get]
func (h *UserHandler) ListProjectMemberships(w http.ResponseWriter, r *http.Request) {
	orgID, _ := transport.OrgIDFromContext(r.Context())
	userID := chi.URLParam(r, "id")

	memberships, err := h.pmSvc.ListByUser(r.Context(), orgID, userID)
	if err != nil {
		transport.ServerError(w, r, h.log, err)
		return
	}

	resp := make([]*dto.UserProjectMembershipResponse, len(memberships))
	for i, m := range memberships {
		resp[i] = &dto.UserProjectMembershipResponse{
			ProjectID: m.ProjectID,
			Name:      m.Name,
			Color:     m.Color,
			Icon:      m.Icon,
			Role:      string(m.Role),
		}
	}
	transport.JSON(w, r, http.StatusOK, resp)
}

// @Summary		Update user's project memberships
// @Description	Batch-set which projects a user belongs to and their role in each
// @Tags			users
// @Accept			json
// @Produce		json
// @Param			id		path	string								true	"User ID"
// @Param			body	body	dto.UpdateUserProjectMembershipsRequest	true	"Project assignments"
// @Success		200		{object}	transport.SuccessResponse
// @Failure		400		{object}	transport.ErrorResponse
// @Failure		403		{object}	transport.ErrorResponse
// @Router			/users/{id}/project-memberships [put]
func (h *UserHandler) UpdateProjectMemberships(w http.ResponseWriter, r *http.Request) {
	orgID, _ := transport.OrgIDFromContext(r.Context())
	userID := chi.URLParam(r, "id")

	var req dto.UpdateUserProjectMembershipsRequest
	if err := dto.BindAndValidate(r, &req); err != nil {
		transport.ErrorJSON(w, r, http.StatusBadRequest, "validation_error", err.Error())
		return
	}

	assignments := make([]domain.ProjectAssignment, len(req.Assignments))
	for i, a := range req.Assignments {
		assignments[i] = domain.ProjectAssignment{
			ProjectID: a.ProjectID,
			Role:      domain.Role(a.Role),
		}
	}

	if err := h.pmSvc.SetMemberships(r.Context(), orgID, userID, assignments); err != nil {
		transport.RespondWithError(w, r, h.log, err)
		return
	}

	transport.JSON(w, r, http.StatusOK, transport.SuccessResponse{Success: true})
}
