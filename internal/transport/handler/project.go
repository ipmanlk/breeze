package handler

import (
	"log/slog"
	"net/http"
	"time"

	"github.com/go-chi/chi/v5"

	"ipmanlk/breeze/internal/domain"
	"ipmanlk/breeze/internal/port"
	"ipmanlk/breeze/internal/transport"
	"ipmanlk/breeze/internal/transport/dto"
)

type ProjectHandler struct {
	svc       port.ProjectService
	accessSvc port.AccessService
	audit     port.AuditService
	log       *slog.Logger
}

func NewProjectHandler(svc port.ProjectService, accessSvc port.AccessService, audit port.AuditService, log *slog.Logger) *ProjectHandler {
	return &ProjectHandler{svc: svc, accessSvc: accessSvc, audit: audit, log: log}
}

// @Summary		List projects
// @Description	Returns all projects for the user's organization. Pass archived=true to include archived projects.
// @Tags			projects
// @Produce		json
// @Param			archived	query		boolean	false	"Include archived projects (default false)"
// @Success		200		{array}	dto.ProjectResponse
// @Router			/projects [get]
func (h *ProjectHandler) List(w http.ResponseWriter, r *http.Request) {
	orgID, _ := transport.OrgIDFromContext(r.Context())
	roleStr, _ := transport.RoleFromContext(r.Context())
	userID, _ := transport.UserIDFromContext(r.Context())

	includeArchived := r.URL.Query().Get("archived") == "true"

	// Org owner/admin/member see every project in the org. Viewers/guests are
	// project-scoped: only the projects they have a project_members row in.
	projects, err := h.svc.ListForCaller(r.Context(), orgID, userID, domain.Role(roleStr), includeArchived)
	if err != nil {
		transport.ServerError(w, r, h.log, err)
		return
	}
	resp := make([]*dto.ProjectResponse, len(projects))
	for i, p := range projects {
		resp[i] = dto.NewProjectResponse(p)
	}
	transport.JSON(w, r, http.StatusOK, resp)
}

// @Summary		Create a project
// @Description	Creates a new project in the organization
// @Tags			projects
// @Accept			json
// @Produce		json
// @Param			body	body		dto.CreateProjectRequest	true	"Project details"
// @Success		201		{object}	dto.ProjectResponse
// @Failure		400		{object}	transport.ErrorResponse
// @Router			/projects [post]
func (h *ProjectHandler) Create(w http.ResponseWriter, r *http.Request) {
	var req dto.CreateProjectRequest
	if err := dto.BindAndValidate(r, &req); err != nil {
		transport.ErrorJSON(w, r, http.StatusBadRequest, "validation_error", err.Error())
		return
	}

	orgID, _ := transport.OrgIDFromContext(r.Context())
	userID, _ := transport.UserIDFromContext(r.Context())

	var startsAt, endsAt *time.Time
	if req.StartsAt != nil {
		t, err := time.Parse(time.RFC3339, *req.StartsAt)
		if err == nil {
			startsAt = &t
		}
	}
	if req.EndsAt != nil {
		t, err := time.Parse(time.RFC3339, *req.EndsAt)
		if err == nil {
			endsAt = &t
		}
	}

	var autoGenerateCycles bool
	var incompleteTaskHandling domain.CycleCompletionHandling
	if req.AutoGenerateCycles != nil {
		autoGenerateCycles = *req.AutoGenerateCycles
	}
	if req.IncompleteTaskHandling != nil {
		incompleteTaskHandling = domain.CycleCompletionHandling(*req.IncompleteTaskHandling)
	} else {
		incompleteTaskHandling = domain.CycleHandlingNextCycle
	}

	project, err := h.svc.Create(r.Context(), orgID, req.Name, userID, req.CycleDuration, autoGenerateCycles, incompleteTaskHandling, startsAt, endsAt)
	if err != nil {
		transport.RespondWithError(w, r, h.log, err)
		return
	}

	transport.JSON(w, r, http.StatusCreated, dto.NewProjectResponse(project))
}

// @Summary		Get a project
// @Description	Returns a single project by ID
// @Tags			projects
// @Produce		json
// @Param			id	path		string	true	"Project ID"
// @Success		200	{object}	dto.ProjectResponse
// @Failure		404	{object}	transport.ErrorResponse
// @Router			/projects/{id} [get]
func (h *ProjectHandler) Get(w http.ResponseWriter, r *http.Request) {
	orgID, _ := transport.OrgIDFromContext(r.Context())
	id := chi.URLParam(r, "id")

	if err := EnsureProjectAccess(r.Context(), h.accessSvc, id); err != nil {
		transport.RespondWithError(w, r, h.log, err)
		return
	}

	project, err := h.svc.GetByID(r.Context(), orgID, id)
	if err != nil {
		transport.RespondWithError(w, r, h.log, err)
		return
	}

	transport.JSON(w, r, http.StatusOK, dto.NewProjectResponse(project))
}

// @Summary		Get a project by slug
// @Description	Returns a single project by its slug
// @Tags			projects
// @Produce		json
// @Param			slug	path		string	true	"Project slug"
// @Success		200		{object}	dto.ProjectResponse
// @Failure		404		{object}	transport.ErrorResponse
// @Router			/projects/by-slug/{slug} [get]
func (h *ProjectHandler) GetBySlug(w http.ResponseWriter, r *http.Request) {
	orgID, _ := transport.OrgIDFromContext(r.Context())
	slug := chi.URLParam(r, "slug")

	project, err := h.svc.GetBySlug(r.Context(), orgID, slug)
	if err != nil {
		transport.RespondWithError(w, r, h.log, err)
		return
	}

	// Enforce project membership for viewer/guest roles. GetBySlug is
	// org-scoped (so cross-org access is impossible), but the permission model
	// says project-scoped roles should only see projects they're an explicit
	// member of. Without this check a viewer could read any project's metadata
	// (name, description, config) by guessing slugs.
	if err := EnsureProjectAccess(r.Context(), h.accessSvc, project.ID); err != nil {
		transport.RespondWithError(w, r, h.log, err)
		return
	}

	transport.JSON(w, r, http.StatusOK, dto.NewProjectResponse(project))
}

// @Summary		Get the current user's project access
// @Description	Returns the authenticated user's effective role and permission
// @Description	set for the given project. Org owner/admin/member use their org
// @Description	role; viewer/guest use their per-project role (403 if not a member).
// @Tags			projects
// @Produce		json
// @Param			id	path		string	true	"Project ID"
// @Success		200	{object}	dto.ProjectAccessResponse
// @Failure		403	{object}	transport.ErrorResponse
// @Router			/projects/{id}/my-access [get]
func (h *ProjectHandler) MyAccess(w http.ResponseWriter, r *http.Request) {
	projectID := chi.URLParam(r, "id")

	// RequireProjectPermission already resolved + stashed the effective role.
	role, ok := transport.EffectiveRoleFromContext(r.Context())
	if !ok {
		var err error
		role, err = ResolveProjectEffectiveRole(r.Context(), h.accessSvc, projectID)
		if err != nil {
			transport.LocalizedErrorJSON(w, r, http.StatusForbidden, "forbidden", "ErrNoAccessToProject")
			return
		}
	}

	perms := domain.PermissionsForRole(role)
	out := make([]string, len(perms))
	for i, p := range perms {
		out[i] = string(p)
	}
	transport.JSON(w, r, http.StatusOK, dto.ProjectAccessResponse{
		Role:        string(role),
		Permissions: out,
	})
}

// @Summary		Update a project
// @Description	Updates project fields
// @Tags			projects
// @Accept			json
// @Produce		json
// @Param			id		path		string						true	"Project ID"
// @Param			body	body		dto.UpdateProjectRequest	true	"Fields to update"
// @Success		200		{object}	dto.ProjectResponse
// @Failure		400		{object}	transport.ErrorResponse
// @Failure		404		{object}	transport.ErrorResponse
// @Router			/projects/{id} [put]
func (h *ProjectHandler) Update(w http.ResponseWriter, r *http.Request) {
	orgID, _ := transport.OrgIDFromContext(r.Context())
	id := chi.URLParam(r, "id")

	if err := EnsureProjectAccess(r.Context(), h.accessSvc, id); err != nil {
		transport.RespondWithError(w, r, h.log, err)
		return
	}

	project, err := h.svc.GetByID(r.Context(), orgID, id)
	if err != nil {
		transport.RespondWithError(w, r, h.log, err)
		return
	}

	var req dto.UpdateProjectRequest
	if err := dto.BindAndValidate(r, &req); err != nil {
		transport.ErrorJSON(w, r, http.StatusBadRequest, "validation_error", err.Error())
		return
	}

	if req.Name != "" {
		project.Name = req.Name
	}
	if req.Description != "" {
		project.Description = req.Description
	}
	if req.Color != "" {
		project.Color = req.Color
	}
	if req.Icon != "" {
		project.Icon = req.Icon
	}
	if req.CycleDuration != nil {
		project.CycleDuration = req.CycleDuration
	}
	if req.AutoGenerateCycles != nil {
		project.AutoGenerateCycles = *req.AutoGenerateCycles
	}
	if req.IncompleteTaskHandling != nil {
		project.IncompleteTaskHandling = domain.CycleCompletionHandling(*req.IncompleteTaskHandling)
	}

	if err := h.svc.Update(r.Context(), project); err != nil {
		transport.RespondWithError(w, r, h.log, err)
		return
	}

	transport.JSON(w, r, http.StatusOK, dto.NewProjectResponse(project))
}

// @Summary		Delete a project
// @Description	Permanently deletes a project
// @Tags			projects
// @Produce		json
// @Param			id	path	string	true	"Project ID"
// @Success		204	"No Content"
// @Failure		404	{object}	transport.ErrorResponse
// @Router			/projects/{id} [delete]
func (h *ProjectHandler) Delete(w http.ResponseWriter, r *http.Request) {
	orgID, _ := transport.OrgIDFromContext(r.Context())
	id := chi.URLParam(r, "id")

	if err := EnsureProjectAccess(r.Context(), h.accessSvc, id); err != nil {
		transport.RespondWithError(w, r, h.log, err)
		return
	}

	if err := h.svc.Delete(r.Context(), orgID, id); err != nil {
		transport.RespondWithError(w, r, h.log, err)
		return
	}

	callerID, _ := transport.UserIDFromContext(r.Context())
	h.audit.Record(r.Context(), orgID, callerID, domain.AuditActionProjectDeleted, "project", id, nil)

	w.WriteHeader(http.StatusNoContent)
}

// @Summary		Archive a project
// @Description	Marks a project as archived (hidden from default list, read-only)
// @Tags			projects
// @Produce		json
// @Param			id	path	string	true	"Project ID"
// @Success		204	"No Content"
// @Failure		404	{object}	transport.ErrorResponse
// @Router			/projects/{id}/archive [post]
func (h *ProjectHandler) Archive(w http.ResponseWriter, r *http.Request) {
	orgID, _ := transport.OrgIDFromContext(r.Context())
	id := chi.URLParam(r, "id")

	if err := EnsureProjectAccess(r.Context(), h.accessSvc, id); err != nil {
		transport.RespondWithError(w, r, h.log, err)
		return
	}

	if err := h.svc.Archive(r.Context(), orgID, id); err != nil {
		transport.RespondWithError(w, r, h.log, err)
		return
	}
	w.WriteHeader(http.StatusNoContent)
}

// @Summary		Unarchive a project
// @Description	Restores an archived project to the active list
// @Tags			projects
// @Produce		json
// @Param			id	path	string	true	"Project ID"
// @Success		204	"No Content"
// @Failure		404	{object}	transport.ErrorResponse
// @Router			/projects/{id}/unarchive [post]
func (h *ProjectHandler) Unarchive(w http.ResponseWriter, r *http.Request) {
	orgID, _ := transport.OrgIDFromContext(r.Context())
	id := chi.URLParam(r, "id")

	if err := EnsureProjectAccess(r.Context(), h.accessSvc, id); err != nil {
		transport.RespondWithError(w, r, h.log, err)
		return
	}

	if err := h.svc.Unarchive(r.Context(), orgID, id); err != nil {
		transport.RespondWithError(w, r, h.log, err)
		return
	}
	w.WriteHeader(http.StatusNoContent)
}
