package handler

import (
	"log/slog"
	"net/http"

	"ipmanlk/breeze/internal/domain"
	"ipmanlk/breeze/internal/port"
	"ipmanlk/breeze/internal/transport"
	"ipmanlk/breeze/internal/transport/dto"

	"github.com/go-chi/chi/v5"
)

type TaskStatusHandler struct {
	svc       port.TaskStatusService
	accessSvc port.AccessService

	log *slog.Logger
}

func NewTaskStatusHandler(svc port.TaskStatusService, accessSvc port.AccessService, log *slog.Logger) *TaskStatusHandler {
	return &TaskStatusHandler{svc: svc, accessSvc: accessSvc, log: log}
}

// @Summary		List statuses
// @Description	Returns all task statuses for a project
// @Tags			statuses
// @Produce		json
// @Param			id	path	string	true	"Project ID"
// @Success		200	{array}	dto.TaskStatusResponse
// @Router			/projects/{id}/statuses [get]
func (h *TaskStatusHandler) List(w http.ResponseWriter, r *http.Request) {
	projectID := chi.URLParam(r, "id")

	if err := EnsureProjectAccess(r.Context(), h.accessSvc, projectID); err != nil {
		transport.RespondWithError(w, r, h.log, err)
		return
	}

	statuses, err := h.svc.List(r.Context(), projectID)
	if err != nil {
		transport.ServerError(w, r, h.log, err)
		return
	}
	resp := make([]*dto.TaskStatusResponse, len(statuses))
	for i, s := range statuses {
		resp[i] = dto.NewTaskStatusResponse(s)
	}
	transport.JSON(w, r, http.StatusOK, resp)
}

// @Summary		Create a status
// @Description	Creates a new task status for a project
// @Tags			statuses
// @Accept			json
// @Produce		json
// @Param			id		path		string					true	"Project ID"
// @Param			body	body		dto.CreateStatusRequest	true	"Status details"
// @Success		201		{object}	dto.TaskStatusResponse
// @Failure		400		{object}	transport.ErrorResponse
// @Router			/projects/{id}/statuses [post]
func (h *TaskStatusHandler) Create(w http.ResponseWriter, r *http.Request) {
	var req dto.CreateStatusRequest
	if err := dto.BindAndValidate(r, &req); err != nil {
		transport.ErrorJSON(w, r, http.StatusBadRequest, "validation_error", err.Error())
		return
	}

	projectID := chi.URLParam(r, "id")
	userID, _ := transport.UserIDFromContext(r.Context())
	orgID, _ := transport.OrgIDFromContext(r.Context())

	if err := EnsureProjectAccess(r.Context(), h.accessSvc, projectID); err != nil {
		transport.RespondWithError(w, r, h.log, err)
		return
	}

	status, err := h.svc.Create(r.Context(), userID, orgID, domain.CreateTaskStatusParams{
		ProjectID: projectID,
		Name:      req.Name,
		Color:     req.Color,
		Position:  req.Position,
		Category:  req.Category,
	})
	if err != nil {
		transport.RespondWithError(w, r, h.log, err)
		return
	}

	transport.JSON(w, r, http.StatusCreated, dto.NewTaskStatusResponse(status))
}

// @Summary		Update a status
// @Description	Updates a task status
// @Tags			statuses
// @Accept			json
// @Produce		json
// @Param			id			path		string					true	"Project ID"
// @Param			statusId	path		string					true	"Status ID"
// @Param			body		body		dto.UpdateStatusRequest	true	"Fields to update"
// @Success		200			{object}	dto.TaskStatusResponse
// @Failure		400			{object}	transport.ErrorResponse
// @Failure		404			{object}	transport.ErrorResponse
// @Router			/projects/{id}/statuses/{statusId} [put]
func (h *TaskStatusHandler) Update(w http.ResponseWriter, r *http.Request) {
	projectID := chi.URLParam(r, "id")
	statusID := chi.URLParam(r, "statusId")
	userID, _ := transport.UserIDFromContext(r.Context())
	orgID, _ := transport.OrgIDFromContext(r.Context())

	if err := EnsureProjectAccess(r.Context(), h.accessSvc, projectID); err != nil {
		transport.RespondWithError(w, r, h.log, err)
		return
	}

	status, err := h.svc.GetByID(r.Context(), statusID)
	if err != nil {
		transport.RespondWithError(w, r, h.log, err)
		return
	}

	if status.ProjectID != projectID {
		transport.LocalizedErrorJSON(w, r, http.StatusForbidden, "forbidden", "ErrStatusNotInProject")
		return
	}

	var req dto.UpdateStatusRequest
	if err := dto.BindAndValidate(r, &req); err != nil {
		transport.ErrorJSON(w, r, http.StatusBadRequest, "validation_error", err.Error())
		return
	}

	if req.Name != "" {
		status.Name = req.Name
	}
	if req.Color != "" {
		status.Color = req.Color
	}
	// Position is a pointer so that 0 (a valid position; the first status) is
	// distinguishable from "not provided". The old `req.Position != 0` guard
	// silently dropped moves to position 0, breaking reorders.
	if req.Position != nil {
		status.Position = *req.Position
	}
	if req.Category != "" {
		status.Category = req.Category
	}

	if err := h.svc.Update(r.Context(), userID, orgID, status); err != nil {
		transport.RespondWithError(w, r, h.log, err)
		return
	}

	transport.JSON(w, r, http.StatusOK, dto.NewTaskStatusResponse(status))
}

// @Summary		Delete a status
// @Description	Permanently deletes a task status
// @Tags			statuses
// @Produce		json
// @Param			id			path	string	true	"Project ID"
// @Param			statusId	path	string	true	"Status ID"
// @Success		204			"No Content"
// @Failure		404			{object}	transport.ErrorResponse
// @Router			/projects/{id}/statuses/{statusId} [delete]
func (h *TaskStatusHandler) Delete(w http.ResponseWriter, r *http.Request) {
	projectID := chi.URLParam(r, "id")
	statusID := chi.URLParam(r, "statusId")
	userID, _ := transport.UserIDFromContext(r.Context())
	orgID, _ := transport.OrgIDFromContext(r.Context())

	if err := EnsureProjectAccess(r.Context(), h.accessSvc, projectID); err != nil {
		transport.RespondWithError(w, r, h.log, err)
		return
	}

	if err := h.svc.Delete(r.Context(), userID, orgID, statusID, projectID); err != nil {
		transport.RespondWithError(w, r, h.log, err)
		return
	}

	w.WriteHeader(http.StatusNoContent)
}
