package handler

import (
	"log/slog"
	"net/http"

	"ipmanlk/plume/internal/domain"
	"ipmanlk/plume/internal/port"
	"ipmanlk/plume/internal/transport"
	"ipmanlk/plume/internal/transport/dto"

	"github.com/go-chi/chi/v5"
)

type TimeEntryHandler struct {
	svc       port.TimeEntryService
	accessSvc port.AccessService

	log *slog.Logger
}

func NewTimeEntryHandler(svc port.TimeEntryService, accessSvc port.AccessService, log *slog.Logger) *TimeEntryHandler {
	return &TimeEntryHandler{svc: svc, accessSvc: accessSvc, log: log}
}

// @Summary		List time entries
// @Description	Returns all time entries for a task
// @Tags			time_entries
// @Produce		json
// @Param			id		path	string	true	"Project ID"
// @Param			taskId	path	string	true	"Task ID"
// @Success		200		{array}	dto.TimeEntryResponse
// @Router			/projects/{id}/tasks/{taskId}/time-entries [get]
func (h *TimeEntryHandler) List(w http.ResponseWriter, r *http.Request) {
	orgID, _ := transport.OrgIDFromContext(r.Context())
	taskID := chi.URLParam(r, "taskId")
	projectID := chi.URLParam(r, "id")

	if err := EnsureProjectAccess(r.Context(), h.accessSvc, projectID); err != nil {
		transport.RespondWithError(w, r, h.log, err)
		return
	}

	entries, err := h.svc.List(r.Context(), orgID, taskID, projectID)
	if err != nil {
		transport.ServerError(w, r, h.log, err)
		return
	}
	resp := make([]*dto.TimeEntryResponse, len(entries))
	for i, e := range entries {
		resp[i] = dto.NewTimeEntryResponse(e)
	}
	transport.JSON(w, r, http.StatusOK, resp)
}

// @Summary		Start a timer
// @Description	Starts a running time entry for the current user on a task
// @Tags			time_entries
// @Accept			json
// @Produce		json
// @Param			id		path		string					true	"Project ID"
// @Param			taskId	path		string					true	"Task ID"
// @Param			body	body		dto.StartTimerRequest	true	"Optional description"
// @Success		200		{array}		dto.TimeEntryResponse
// @Failure		400		{object}	transport.ErrorResponse
// @Router			/projects/{id}/tasks/{taskId}/time-entries/start [post]
func (h *TimeEntryHandler) Start(w http.ResponseWriter, r *http.Request) {
	orgID, _ := transport.OrgIDFromContext(r.Context())
	taskID := chi.URLParam(r, "taskId")
	projectID := chi.URLParam(r, "id")

	if err := EnsureProjectAccess(r.Context(), h.accessSvc, projectID); err != nil {
		transport.RespondWithError(w, r, h.log, err)
		return
	}

	userID, _ := transport.UserIDFromContext(r.Context())

	var req dto.StartTimerRequest
	if err := dto.BindAndValidate(r, &req); err != nil {
		transport.ErrorJSON(w, r, http.StatusBadRequest, "validation_error", err.Error())
		return
	}

	entries, err := h.svc.Start(r.Context(), orgID, taskID, projectID, userID, req.Description)
	if err != nil {
		transport.RespondWithError(w, r, h.log, err)
		return
	}

	resp := make([]*dto.TimeEntryResponse, len(entries))
	for i, e := range entries {
		resp[i] = dto.NewTimeEntryResponse(e)
	}
	transport.JSON(w, r, http.StatusOK, resp)
}

// @Summary		Stop the active timer
// @Description	Stops the current user's running timer on a task
// @Tags			time_entries
// @Produce		json
// @Param			id		path		string	true	"Project ID"
// @Param			taskId	path		string	true	"Task ID"
// @Success		200		{array}		dto.TimeEntryResponse
// @Failure		400		{object}	transport.ErrorResponse
// @Router			/projects/{id}/tasks/{taskId}/time-entries/stop [post]
func (h *TimeEntryHandler) Stop(w http.ResponseWriter, r *http.Request) {
	orgID, _ := transport.OrgIDFromContext(r.Context())
	taskID := chi.URLParam(r, "taskId")
	projectID := chi.URLParam(r, "id")

	if err := EnsureProjectAccess(r.Context(), h.accessSvc, projectID); err != nil {
		transport.RespondWithError(w, r, h.log, err)
		return
	}

	userID, _ := transport.UserIDFromContext(r.Context())

	entries, err := h.svc.Stop(r.Context(), orgID, taskID, projectID, userID)
	if err != nil {
		transport.RespondWithError(w, r, h.log, err)
		return
	}

	resp := make([]*dto.TimeEntryResponse, len(entries))
	for i, e := range entries {
		resp[i] = dto.NewTimeEntryResponse(e)
	}
	transport.JSON(w, r, http.StatusOK, resp)
}

// @Summary		Create a manual time entry
// @Description	Creates a completed time entry with a specified duration
// @Tags			time_entries
// @Accept			json
// @Produce		json
// @Param			id		path		string						true	"Project ID"
// @Param			taskId	path		string						true	"Task ID"
// @Param			body	body		dto.CreateTimeEntryRequest	true	"Time entry details"
// @Success		201		{array}		dto.TimeEntryResponse
// @Failure		400		{object}	transport.ErrorResponse
// @Router			/projects/{id}/tasks/{taskId}/time-entries [post]
func (h *TimeEntryHandler) Create(w http.ResponseWriter, r *http.Request) {
	orgID, _ := transport.OrgIDFromContext(r.Context())
	taskID := chi.URLParam(r, "taskId")
	projectID := chi.URLParam(r, "id")

	if err := EnsureProjectAccess(r.Context(), h.accessSvc, projectID); err != nil {
		transport.RespondWithError(w, r, h.log, err)
		return
	}

	userID, _ := transport.UserIDFromContext(r.Context())

	var req dto.CreateTimeEntryRequest
	if err := dto.BindAndValidate(r, &req); err != nil {
		transport.ErrorJSON(w, r, http.StatusBadRequest, "validation_error", err.Error())
		return
	}

	entries, err := h.svc.Create(r.Context(), domain.CreateTimeEntryParams{
		OrgID:           orgID,
		TaskID:          taskID,
		ProjectID:       projectID,
		UserID:          userID,
		Description:     req.Description,
		DurationMinutes: req.DurationMinutes,
	})
	if err != nil {
		transport.RespondWithError(w, r, h.log, err)
		return
	}

	resp := make([]*dto.TimeEntryResponse, len(entries))
	for i, e := range entries {
		resp[i] = dto.NewTimeEntryResponse(e)
	}
	transport.JSON(w, r, http.StatusCreated, resp)
}

// @Summary		Update a time entry
// @Description	Updates a time entry's description and/or duration
// @Tags			time_entries
// @Accept			json
// @Produce		json
// @Param			id		path		string						true	"Project ID"
// @Param			taskId	path		string						true	"Task ID"
// @Param			entryId	path		string						true	"Time Entry ID"
// @Param			body	body		dto.UpdateTimeEntryRequest	true	"Fields to update"
// @Success		200		{array}		dto.TimeEntryResponse
// @Failure		400		{object}	transport.ErrorResponse
// @Router			/projects/{id}/tasks/{taskId}/time-entries/{entryId} [put]
func (h *TimeEntryHandler) Update(w http.ResponseWriter, r *http.Request) {
	orgID, _ := transport.OrgIDFromContext(r.Context())
	userID, _ := transport.UserIDFromContext(r.Context())
	roleStr, _ := transport.RoleFromContext(r.Context())
	role := domain.Role(roleStr)
	taskID := chi.URLParam(r, "taskId")
	projectID := chi.URLParam(r, "id")
	entryID := chi.URLParam(r, "entryId")

	if err := EnsureProjectAccess(r.Context(), h.accessSvc, projectID); err != nil {
		transport.RespondWithError(w, r, h.log, err)
		return
	}

	var req dto.UpdateTimeEntryRequest
	if err := dto.BindAndValidate(r, &req); err != nil {
		transport.ErrorJSON(w, r, http.StatusBadRequest, "validation_error", err.Error())
		return
	}

	entries, err := h.svc.Update(r.Context(), userID, role, domain.UpdateTimeEntryParams{
		ID:              entryID,
		OrgID:           orgID,
		TaskID:          taskID,
		ProjectID:       projectID,
		Description:     req.Description,
		DurationMinutes: req.DurationMinutes,
	})
	if err != nil {
		transport.RespondWithError(w, r, h.log, err)
		return
	}

	resp := make([]*dto.TimeEntryResponse, len(entries))
	for i, e := range entries {
		resp[i] = dto.NewTimeEntryResponse(e)
	}
	transport.JSON(w, r, http.StatusOK, resp)
}

// @Summary		Delete a time entry
// @Description	Permanently deletes a time entry
// @Tags			time_entries
// @Produce		json
// @Param			id		path	string	true	"Project ID"
// @Param			taskId	path	string	true	"Task ID"
// @Param			entryId	path	string	true	"Time Entry ID"
// @Success		204		"No Content"
// @Failure		404		{object}	transport.ErrorResponse
// @Router			/projects/{id}/tasks/{taskId}/time-entries/{entryId} [delete]
func (h *TimeEntryHandler) Delete(w http.ResponseWriter, r *http.Request) {
	orgID, _ := transport.OrgIDFromContext(r.Context())
	userID, _ := transport.UserIDFromContext(r.Context())
	roleStr, _ := transport.RoleFromContext(r.Context())
	role := domain.Role(roleStr)
	taskID := chi.URLParam(r, "taskId")
	projectID := chi.URLParam(r, "id")
	entryID := chi.URLParam(r, "entryId")

	if err := EnsureProjectAccess(r.Context(), h.accessSvc, projectID); err != nil {
		transport.RespondWithError(w, r, h.log, err)
		return
	}

	if err := h.svc.Delete(r.Context(), userID, role, orgID, entryID, taskID, projectID); err != nil {
		transport.RespondWithError(w, r, h.log, err)
		return
	}

	w.WriteHeader(http.StatusNoContent)
}
