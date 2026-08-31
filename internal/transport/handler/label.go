package handler

import (
	"log/slog"
	"net/http"

	"ipmanlk/plume/internal/port"
	"ipmanlk/plume/internal/transport"
	"ipmanlk/plume/internal/transport/dto"

	"github.com/go-chi/chi/v5"
)

type LabelHandler struct {
	svc port.LabelService
	log *slog.Logger
}

func NewLabelHandler(svc port.LabelService, log *slog.Logger) *LabelHandler {
	return &LabelHandler{svc: svc, log: log}
}

// @Summary		List labels
// @Description	Returns all labels for the caller's organization
// @Tags			labels
// @Produce		json
// @Success		200	{array}	dto.LabelResponse
// @Router			/labels [get]
func (h *LabelHandler) List(w http.ResponseWriter, r *http.Request) {
	orgID, _ := transport.OrgIDFromContext(r.Context())
	labels, err := h.svc.List(r.Context(), orgID)
	if err != nil {
		transport.ServerError(w, r, h.log, err)
		return
	}
	resp := make([]*dto.LabelResponse, len(labels))
	for i, l := range labels {
		resp[i] = dto.NewLabelResponse(l)
	}
	transport.JSON(w, r, http.StatusOK, resp)
}

// @Summary		Create a label
// @Description	Creates a new label in the organization
// @Tags			labels
// @Accept			json
// @Produce		json
// @Param			body	body		dto.CreateLabelRequest	true	"Label details"
// @Success		201		{object}	dto.LabelResponse
// @Failure		400		{object}	transport.ErrorResponse
// @Router			/labels [post]
func (h *LabelHandler) Create(w http.ResponseWriter, r *http.Request) {
	orgID, _ := transport.OrgIDFromContext(r.Context())
	userID, _ := transport.UserIDFromContext(r.Context())
	var req dto.CreateLabelRequest
	if err := dto.BindAndValidate(r, &req); err != nil {
		transport.ErrorJSON(w, r, http.StatusBadRequest, "validation_error", err.Error())
		return
	}
	label, err := h.svc.Create(r.Context(), userID, orgID, req.Name, req.Color)
	if err != nil {
		transport.RespondWithError(w, r, h.log, err)
		return
	}
	transport.JSON(w, r, http.StatusCreated, dto.NewLabelResponse(label))
}

// @Summary		Update a label
// @Description	Updates a label's name and color
// @Tags			labels
// @Accept			json
// @Produce		json
// @Param			id		path		string						true	"Label ID"
// @Param			body	body		dto.UpdateLabelRequest		true	"Label updates"
// @Success		200		{object}	dto.LabelResponse
// @Failure		400		{object}	transport.ErrorResponse
// @Failure		404		{object}	transport.ErrorResponse
// @Router			/labels/{id} [patch]
func (h *LabelHandler) Update(w http.ResponseWriter, r *http.Request) {
	orgID, _ := transport.OrgIDFromContext(r.Context())
	userID, _ := transport.UserIDFromContext(r.Context())
	id := chi.URLParam(r, "id")
	if id == "" {
		transport.LocalizedErrorJSON(w, r, http.StatusBadRequest, "validation_error", "ErrIDRequired")
		return
	}
	var req dto.UpdateLabelRequest
	if err := dto.BindAndValidate(r, &req); err != nil {
		transport.ErrorJSON(w, r, http.StatusBadRequest, "validation_error", err.Error())
		return
	}
	label, err := h.svc.Update(r.Context(), userID, orgID, id, req.Name, req.Color)
	if err != nil {
		transport.RespondWithError(w, r, h.log, err)
		return
	}
	transport.JSON(w, r, http.StatusOK, dto.NewLabelResponse(label))
}

// @Summary		Delete a label
// @Description	Permanently deletes a label
// @Tags			labels
// @Produce		json
// @Param			id	path	string	true	"Label ID"
// @Success		204	"No Content"
// @Failure		404	{object}	transport.ErrorResponse
// @Router			/labels/{id} [delete]
func (h *LabelHandler) Delete(w http.ResponseWriter, r *http.Request) {
	orgID, _ := transport.OrgIDFromContext(r.Context())
	userID, _ := transport.UserIDFromContext(r.Context())
	id := chi.URLParam(r, "id")
	if id == "" {
		transport.LocalizedErrorJSON(w, r, http.StatusBadRequest, "validation_error", "ErrIDRequired")
		return
	}
	if err := h.svc.Delete(r.Context(), userID, orgID, id); err != nil {
		transport.RespondWithError(w, r, h.log, err)
		return
	}
	transport.JSON(w, r, http.StatusNoContent, nil)
}

// @Summary		Set task labels
// @Description	Replaces all labels on a task with the given set
// @Tags			labels
// @Accept			json
// @Produce		json
// @Param			id		path		string						true	"Project ID"
// @Param			taskId	path		string						true	"Task ID"
// @Param			body	body		dto.SetTaskLabelsRequest	true	"Label IDs"
// @Success		200		{array}		dto.LabelResponse
// @Router			/projects/{id}/tasks/{taskId}/labels [put]
func (h *LabelHandler) SetTaskLabels(w http.ResponseWriter, r *http.Request) {
	orgID, _ := transport.OrgIDFromContext(r.Context())
	userID, _ := transport.UserIDFromContext(r.Context())
	taskID := chi.URLParam(r, "taskId")

	var req dto.SetTaskLabelsRequest
	if err := dto.BindAndValidate(r, &req); err != nil {
		transport.ErrorJSON(w, r, http.StatusBadRequest, "validation_error", err.Error())
		return
	}
	if err := h.svc.SetTaskLabels(r.Context(), userID, orgID, taskID, req.LabelIDs); err != nil {
		transport.RespondWithError(w, r, h.log, err)
		return
	}
	// Return the actual labels now attached to the task. Access was already
	// enforced by SetTaskLabels above; pass the caller identity through for
	// the read's own check.
	labels, err := h.svc.GetTaskLabels(r.Context(), userID, orgID, taskID)
	if err != nil {
		transport.RespondWithError(w, r, h.log, err)
		return
	}
	resp := make([]*dto.LabelResponse, len(labels))
	for i, l := range labels {
		resp[i] = dto.NewLabelResponse(l)
	}
	transport.JSON(w, r, http.StatusOK, resp)
}

// @Summary		Get task labels
// @Description	Returns all labels attached to a task
// @Tags			labels
// @Produce		json
// @Param			id		path	string	true	"Project ID"
// @Param			taskId	path	string	true	"Task ID"
// @Success		200		{array}	dto.LabelResponse
// @Router			/projects/{id}/tasks/{taskId}/labels [get]
func (h *LabelHandler) GetTaskLabels(w http.ResponseWriter, r *http.Request) {
	orgID, _ := transport.OrgIDFromContext(r.Context())
	userID, _ := transport.UserIDFromContext(r.Context())
	taskID := chi.URLParam(r, "taskId")
	labels, err := h.svc.GetTaskLabels(r.Context(), userID, orgID, taskID)
	if err != nil {
		transport.ServerError(w, r, h.log, err)
		return
	}
	resp := make([]*dto.LabelResponse, len(labels))
	for i, l := range labels {
		resp[i] = dto.NewLabelResponse(l)
	}
	transport.JSON(w, r, http.StatusOK, resp)
}
