package handler

import (
	"log/slog"
	"net/http"
	"time"

	"ipmanlk/plume/internal/domain"
	"ipmanlk/plume/internal/port"
	"ipmanlk/plume/internal/transport"
	"ipmanlk/plume/internal/transport/dto"

	"github.com/go-chi/chi/v5"
)

type CycleHandler struct {
	svc       port.CycleService
	accessSvc port.AccessService

	log *slog.Logger
}

func NewCycleHandler(svc port.CycleService, accessSvc port.AccessService, log *slog.Logger) *CycleHandler {
	return &CycleHandler{svc: svc, accessSvc: accessSvc, log: log}
}

// @Summary		List cycles
// @Description	Returns all cycles for a project
// @Tags			cycles
// @Produce		json
// @Param			id	path	string	true	"Project ID"
// @Success		200	{array}	dto.CycleResponse
// @Router			/projects/{id}/cycles [get]
func (h *CycleHandler) List(w http.ResponseWriter, r *http.Request) {
	projectID := chi.URLParam(r, "id")

	if err := EnsureProjectAccess(r.Context(), h.accessSvc, projectID); err != nil {
		transport.RespondWithError(w, r, h.log, err)
		return
	}

	cycles, err := h.svc.List(r.Context(), projectID)
	if err != nil {
		transport.ServerError(w, r, h.log, err)
		return
	}
	resp := make([]*dto.CycleResponse, len(cycles))
	for i, c := range cycles {
		resp[i] = dto.NewCycleResponse(c)
	}
	transport.JSON(w, r, http.StatusOK, resp)
}

// @Summary		Get active cycle
// @Description	Returns the active cycle for a project
// @Tags			cycles
// @Produce		json
// @Param			id	path		string	true	"Project ID"
// @Success		200	{object}	dto.CycleResponse
// @Failure		404	{object}	transport.ErrorResponse
// @Router			/projects/{id}/cycles/active [get]
func (h *CycleHandler) GetActive(w http.ResponseWriter, r *http.Request) {
	projectID := chi.URLParam(r, "id")

	if err := EnsureProjectAccess(r.Context(), h.accessSvc, projectID); err != nil {
		transport.RespondWithError(w, r, h.log, err)
		return
	}

	cycle, err := h.svc.GetActive(r.Context(), projectID)
	if err != nil {
		transport.RespondWithError(w, r, h.log, err)
		return
	}
	transport.JSON(w, r, http.StatusOK, dto.NewCycleResponse(cycle))
}

// @Summary		Create a cycle
// @Description	Creates a new cycle in a project
// @Tags			cycles
// @Accept			json
// @Produce		json
// @Param			id		path		string					true	"Project ID"
// @Param			body	body		dto.CreateCycleRequest	true	"Cycle details"
// @Success		201		{object}	dto.CycleResponse
// @Failure		400		{object}	transport.ErrorResponse
// @Router			/projects/{id}/cycles [post]
func (h *CycleHandler) Create(w http.ResponseWriter, r *http.Request) {
	var req dto.CreateCycleRequest
	if err := dto.BindAndValidate(r, &req); err != nil {
		transport.ErrorJSON(w, r, http.StatusBadRequest, "validation_error", err.Error())
		return
	}

	orgID, _ := transport.OrgIDFromContext(r.Context())
	userID, _ := transport.UserIDFromContext(r.Context())
	projectID := chi.URLParam(r, "id")

	if err := EnsureProjectAccess(r.Context(), h.accessSvc, projectID); err != nil {
		transport.RespondWithError(w, r, h.log, err)
		return
	}

	startsAt, err := time.Parse(time.RFC3339, req.StartsAt)
	if err != nil {
		transport.LocalizedErrorJSON(w, r, http.StatusBadRequest, "validation_error", "ErrInvalidStartsAt")
		return
	}
	endsAt, err := time.Parse(time.RFC3339, req.EndsAt)
	if err != nil {
		transport.LocalizedErrorJSON(w, r, http.StatusBadRequest, "validation_error", "ErrInvalidEndsAt")
		return
	}

	cycle, err := h.svc.Create(r.Context(), domain.CreateCycleParams{
		OrgID:     orgID,
		ProjectID: projectID,
		CreatedBy: userID,
		Name:      req.Name,
		Goal:      req.Goal,
		StartsAt:  startsAt,
		EndsAt:    endsAt,
	})
	if err != nil {
		transport.RespondWithError(w, r, h.log, err)
		return
	}

	transport.JSON(w, r, http.StatusCreated, dto.NewCycleResponse(cycle))
}

// @Summary		Update a cycle
// @Description	Updates cycle fields
// @Tags			cycles
// @Accept			json
// @Produce		json
// @Param			id		path		string					true	"Project ID"
// @Param			cycleId	path		string					true	"Cycle ID"
// @Param			body	body		dto.UpdateCycleRequest	true	"Fields to update"
// @Success		200		{object}	dto.CycleResponse
// @Failure		400		{object}	transport.ErrorResponse
// @Failure		404		{object}	transport.ErrorResponse
// @Router			/projects/{id}/cycles/{cycleId} [put]
func (h *CycleHandler) Update(w http.ResponseWriter, r *http.Request) {
	projectID := chi.URLParam(r, "id")
	cycleID := chi.URLParam(r, "cycleId")
	userID, _ := transport.UserIDFromContext(r.Context())
	orgID, _ := transport.OrgIDFromContext(r.Context())

	if err := EnsureProjectAccess(r.Context(), h.accessSvc, projectID); err != nil {
		transport.RespondWithError(w, r, h.log, err)
		return
	}

	cycle, err := h.svc.GetByID(r.Context(), cycleID, projectID)
	if err != nil {
		transport.RespondWithError(w, r, h.log, err)
		return
	}

	var req dto.UpdateCycleRequest
	if err := dto.BindAndValidate(r, &req); err != nil {
		transport.ErrorJSON(w, r, http.StatusBadRequest, "validation_error", err.Error())
		return
	}

	if req.Name != "" {
		cycle.Name = req.Name
	}
	if req.Goal != "" {
		cycle.Goal = req.Goal
	}
	if req.StartsAt != "" {
		t, err := time.Parse(time.RFC3339, req.StartsAt)
		if err == nil {
			cycle.StartsAt = t
		}
	}
	if req.EndsAt != "" {
		t, err := time.Parse(time.RFC3339, req.EndsAt)
		if err == nil {
			cycle.EndsAt = t
		}
	}
	if req.IsCompleted != nil {
		cycle.IsCompleted = *req.IsCompleted
	}

	if err := h.svc.Update(r.Context(), userID, orgID, cycle); err != nil {
		transport.RespondWithError(w, r, h.log, err)
		return
	}

	transport.JSON(w, r, http.StatusOK, dto.NewCycleResponse(cycle))
}

// @Summary		Activate a cycle
// @Description	Sets a cycle as the active cycle for its project
// @Tags			cycles
// @Produce		json
// @Param			id		path		string	true	"Project ID"
// @Param			cycleId	path		string	true	"Cycle ID"
// @Success		200		{object}	dto.CycleResponse
// @Failure		404		{object}	transport.ErrorResponse
// @Router			/projects/{id}/cycles/{cycleId}/activate [post]
func (h *CycleHandler) Activate(w http.ResponseWriter, r *http.Request) {
	projectID := chi.URLParam(r, "id")
	cycleID := chi.URLParam(r, "cycleId")
	userID, _ := transport.UserIDFromContext(r.Context())
	orgID, _ := transport.OrgIDFromContext(r.Context())

	if err := EnsureProjectAccess(r.Context(), h.accessSvc, projectID); err != nil {
		transport.RespondWithError(w, r, h.log, err)
		return
	}

	cycle, err := h.svc.Activate(r.Context(), userID, orgID, cycleID, projectID)
	if err != nil {
		transport.RespondWithError(w, r, h.log, err)
		return
	}

	transport.JSON(w, r, http.StatusOK, dto.NewCycleResponse(cycle))
}

// @Summary		Complete a cycle
// @Description	Marks a cycle as completed and handles incomplete tasks
// @Tags			cycles
// @Accept			json
// @Produce		json
// @Param			id		path		string						true	"Project ID"
// @Param			cycleId	path		string						true	"Cycle ID"
// @Param			body	body		dto.CompleteCycleRequest	false	"Completion options"
// @Success		200		{object}	dto.CycleResponse
// @Failure		404		{object}	transport.ErrorResponse
// @Router			/projects/{id}/cycles/{cycleId}/complete [post]
func (h *CycleHandler) Complete(w http.ResponseWriter, r *http.Request) {
	projectID := chi.URLParam(r, "id")
	cycleID := chi.URLParam(r, "cycleId")
	userID, _ := transport.UserIDFromContext(r.Context())
	orgID, _ := transport.OrgIDFromContext(r.Context())

	if err := EnsureProjectAccess(r.Context(), h.accessSvc, projectID); err != nil {
		transport.RespondWithError(w, r, h.log, err)
		return
	}

	moveToCycleID := ""
	var req dto.CompleteCycleRequest
	if err := dto.BindAndValidate(r, &req); err == nil {
		moveToCycleID = req.MoveToCycleID
	}

	cycle, err := h.svc.Complete(r.Context(), userID, orgID, cycleID, projectID, moveToCycleID)
	if err != nil {
		transport.RespondWithError(w, r, h.log, err)
		return
	}

	transport.JSON(w, r, http.StatusOK, dto.NewCycleResponse(cycle))
}

// @Summary		Delete a cycle
// @Description	Permanently deletes a cycle
// @Tags			cycles
// @Produce		json
// @Param			id		path	string	true	"Project ID"
// @Param			cycleId	path	string	true	"Cycle ID"
// @Success		204		"No Content"
// @Failure		404		{object}	transport.ErrorResponse
// @Router			/projects/{id}/cycles/{cycleId} [delete]
func (h *CycleHandler) Delete(w http.ResponseWriter, r *http.Request) {
	projectID := chi.URLParam(r, "id")
	cycleID := chi.URLParam(r, "cycleId")
	userID, _ := transport.UserIDFromContext(r.Context())
	orgID, _ := transport.OrgIDFromContext(r.Context())

	if err := EnsureProjectAccess(r.Context(), h.accessSvc, projectID); err != nil {
		transport.RespondWithError(w, r, h.log, err)
		return
	}

	if err := h.svc.Delete(r.Context(), userID, orgID, cycleID, projectID); err != nil {
		transport.RespondWithError(w, r, h.log, err)
		return
	}

	w.WriteHeader(http.StatusNoContent)
}
