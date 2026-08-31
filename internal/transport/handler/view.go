package handler

import (
	"context"
	"log/slog"
	"net/http"

	"github.com/go-chi/chi/v5"
	"ipmanlk/plume/internal/domain"
	"ipmanlk/plume/internal/port"
	"ipmanlk/plume/internal/transport"
	"ipmanlk/plume/internal/transport/dto"
)

type ViewHandler struct {
	svc       port.ViewService
	projRepo  port.ProjectRepository
	accessSvc port.AccessService
	log       *slog.Logger
}

func NewViewHandler(svc port.ViewService, projRepo port.ProjectRepository, accessSvc port.AccessService, log *slog.Logger) *ViewHandler {
	return &ViewHandler{svc: svc, projRepo: projRepo, accessSvc: accessSvc, log: log}
}

func filtersFromDTO(f dto.ViewFilters) domain.ViewFilters {
	return domain.ViewFilters{
		Search:     f.Search,
		Priority:   f.Priority,
		StatusID:   f.StatusID,
		AssigneeID: f.AssigneeID,
		CycleID:    f.CycleID,
	}
}

func (h *ViewHandler) projectMap(orgID string) map[string]*domain.Project {
	if h.projRepo == nil {
		return nil
	}
	projects, err := h.projRepo.List(context.Background(), orgID)
	if err != nil {
		h.log.Error("list projects for view enrichment", "error", err)
		return nil
	}
	m := make(map[string]*domain.Project, len(projects))
	for _, p := range projects {
		m[p.ID] = p
	}
	return m
}

func enrichViewResponse(r *dto.ViewResponse, v *domain.View, projMap map[string]*domain.Project) {
	if v.ProjectID == nil || projMap == nil {
		return
	}
	if p, ok := projMap[*v.ProjectID]; ok {
		r.ProjectSlug = &p.Slug
		r.ProjectName = &p.Name
	}
}

func respondWithView(w http.ResponseWriter, r *http.Request, h *ViewHandler, v *domain.View) {
	resp := dto.NewViewResponse(v)
	orgID, _ := transport.OrgIDFromContext(r.Context())
	projMap := h.projectMap(orgID)
	enrichViewResponse(resp, v, projMap)
	transport.JSON(w, r, http.StatusOK, resp)
}

// @Summary		Create a view
// @Description	Creates a new global or project view
// @Tags			views
// @Accept			json
// @Produce			json
// @Param			body	body		dto.CreateViewRequest	true	"View details"
// @Success			201		{object}	dto.ViewResponse
// @Failure			400		{object}	transport.ErrorResponse
// @Router			/views [post]
func (h *ViewHandler) Create(w http.ResponseWriter, r *http.Request) {
	var req dto.CreateViewRequest
	if err := dto.BindAndValidate(r, &req); err != nil {
		transport.ErrorJSON(w, r, http.StatusBadRequest, "validation_error", err.Error())
		return
	}

	orgID, _ := transport.OrgIDFromContext(r.Context())
	userID, _ := transport.UserIDFromContext(r.Context())

	layout := domain.ViewLayoutBoard
	if req.Layout == "list" {
		layout = domain.ViewLayoutList
	}

	view, err := h.svc.Create(r.Context(), domain.CreateViewParams{
		OrgID:     orgID,
		ProjectID: req.ProjectID,
		CreatedBy: userID,
		Name:      req.Name,
		Layout:    layout,
		Filters:   filtersFromDTO(req.Filters),
	})
	if err != nil {
		transport.RespondWithError(w, r, h.log, err)
		return
	}

	resp := dto.NewViewResponse(view)
	projMap := h.projectMap(orgID)
	enrichViewResponse(resp, view, projMap)
	transport.JSON(w, r, http.StatusCreated, resp)
}

// @Summary		List global views
// @Description	Returns all global views for the organization
// @Tags			views
// @Produce			json
// @Success			200	{array}	dto.ViewResponse
// @Router			/views [get]
func (h *ViewHandler) ListGlobal(w http.ResponseWriter, r *http.Request) {
	orgID, _ := transport.OrgIDFromContext(r.Context())

	views, err := h.svc.ListGlobal(r.Context(), orgID)
	if err != nil {
		transport.ServerError(w, r, h.log, err)
		return
	}

	projMap := h.projectMap(orgID)

	resp := make([]*dto.ViewResponse, len(views))
	for i, v := range views {
		resp[i] = dto.NewViewResponse(v)
		enrichViewResponse(resp[i], v, projMap)
	}
	transport.JSON(w, r, http.StatusOK, resp)
}

// @Summary		List pinned views
// @Description	Returns pinned views for the current user
// @Tags			views
// @Produce			json
// @Success			200	{array}	dto.ViewResponse
// @Router			/views/pins [get]
func (h *ViewHandler) ListPinned(w http.ResponseWriter, r *http.Request) {
	userID, _ := transport.UserIDFromContext(r.Context())

	views, err := h.svc.ListPinned(r.Context(), userID)
	if err != nil {
		transport.ServerError(w, r, h.log, err)
		return
	}

	oid, _ := transport.OrgIDFromContext(r.Context())
	projMap := h.projectMap(oid)

	resp := make([]*dto.ViewResponse, len(views))
	for i, v := range views {
		resp[i] = dto.NewViewResponse(v)
		enrichViewResponse(resp[i], v, projMap)
	}
	transport.JSON(w, r, http.StatusOK, resp)
}

// @Summary		Get a view
// @Description	Returns a single view by ID
// @Tags			views
// @Produce			json
// @Param			id	path		string	true	"View ID"
// @Success			200	{object}	dto.ViewResponse
// @Failure			404	{object}	transport.ErrorResponse
// @Router			/views/{id} [get]
func (h *ViewHandler) Get(w http.ResponseWriter, r *http.Request) {
	orgID, _ := transport.OrgIDFromContext(r.Context())
	id := chi.URLParam(r, "id")

	view, err := h.svc.GetByID(r.Context(), orgID, id)
	if err != nil {
		transport.RespondWithError(w, r, h.log, err)
		return
	}

	respondWithView(w, r, h, view)
}

// @Summary		Update a view
// @Description	Updates view fields
// @Tags			views
// @Accept			json
// @Produce			json
// @Param			id		path		string					true	"View ID"
// @Param			body	body		dto.UpdateViewRequest	true	"Fields to update"
// @Success			200		{object}	dto.ViewResponse
// @Failure			400		{object}	transport.ErrorResponse
// @Failure			404		{object}	transport.ErrorResponse
// @Router			/views/{id} [patch]
func (h *ViewHandler) Update(w http.ResponseWriter, r *http.Request) {
	orgID, _ := transport.OrgIDFromContext(r.Context())
	userID, _ := transport.UserIDFromContext(r.Context())
	id := chi.URLParam(r, "id")

	var req dto.UpdateViewRequest
	if err := dto.BindAndValidate(r, &req); err != nil {
		transport.ErrorJSON(w, r, http.StatusBadRequest, "validation_error", err.Error())
		return
	}

	var layout *domain.ViewLayout
	if req.Layout != nil {
		l := domain.ViewLayout(*req.Layout)
		layout = &l
	}

	var filters *domain.ViewFilters
	if req.Filters != nil {
		f := filtersFromDTO(*req.Filters)
		filters = &f
	}

	view, err := h.svc.Update(r.Context(), userID, domain.UpdateViewParams{
		ID:      id,
		OrgID:   orgID,
		Name:    req.Name,
		Layout:  layout,
		Filters: filters,
	})
	if err != nil {
		transport.RespondWithError(w, r, h.log, err)
		return
	}

	respondWithView(w, r, h, view)
}

// @Summary		Delete a view
// @Description	Deletes a view
// @Tags			views
// @Produce			json
// @Param			id	path	string	true	"View ID"
// @Success			204	"No Content"
// @Failure			404	{object}	transport.ErrorResponse
// @Router			/views/{id} [delete]
func (h *ViewHandler) Delete(w http.ResponseWriter, r *http.Request) {
	orgID, _ := transport.OrgIDFromContext(r.Context())
	userID, _ := transport.UserIDFromContext(r.Context())
	id := chi.URLParam(r, "id")

	if err := h.svc.Delete(r.Context(), userID, orgID, id); err != nil {
		transport.RespondWithError(w, r, h.log, err)
		return
	}

	w.WriteHeader(http.StatusNoContent)
}

// @Summary		Pin a view
// @Description	Pins a view to the sidebar for the current user
// @Tags			views
// @Produce			json
// @Param			id	path	string	true	"View ID"
// @Success			204	"No Content"
// @Failure			404	{object}	transport.ErrorResponse
// @Router			/views/{id}/pin [post]
func (h *ViewHandler) Pin(w http.ResponseWriter, r *http.Request) {
	orgID, _ := transport.OrgIDFromContext(r.Context())
	userID, _ := transport.UserIDFromContext(r.Context())
	id := chi.URLParam(r, "id")

	if err := h.svc.Pin(r.Context(), orgID, id, userID); err != nil {
		transport.RespondWithError(w, r, h.log, err)
		return
	}

	w.WriteHeader(http.StatusNoContent)
}

// @Summary		Unpin a view
// @Description	Unpins a view from the sidebar for the current user
// @Tags			views
// @Produce			json
// @Param			id	path	string	true	"View ID"
// @Success			204	"No Content"
// @Router			/views/{id}/pin [delete]
func (h *ViewHandler) Unpin(w http.ResponseWriter, r *http.Request) {
	orgID, _ := transport.OrgIDFromContext(r.Context())
	userID, _ := transport.UserIDFromContext(r.Context())
	id := chi.URLParam(r, "id")

	if err := h.svc.Unpin(r.Context(), orgID, id, userID); err != nil {
		transport.RespondWithError(w, r, h.log, err)
		return
	}

	w.WriteHeader(http.StatusNoContent)
}

// @Summary		List project views
// @Description	Returns all views for a specific project
// @Tags			views
// @Produce			json
// @Param			id		path		string	true	"Project ID"
// @Success			200		{array}	dto.ViewResponse
// @Router			/projects/{id}/views [get]
func (h *ViewHandler) ListByProject(w http.ResponseWriter, r *http.Request) {
	orgID, _ := transport.OrgIDFromContext(r.Context())
	projectID := chi.URLParam(r, "id")

	if err := EnsureProjectAccess(r.Context(), h.accessSvc, projectID); err != nil {
		transport.RespondWithError(w, r, h.log, err)
		return
	}

	views, err := h.svc.ListByProject(r.Context(), orgID, projectID)
	if err != nil {
		transport.ServerError(w, r, h.log, err)
		return
	}

	projMap := h.projectMap(orgID)

	resp := make([]*dto.ViewResponse, len(views))
	for i, v := range views {
		resp[i] = dto.NewViewResponse(v)
		enrichViewResponse(resp[i], v, projMap)
	}
	transport.JSON(w, r, http.StatusOK, resp)
}
