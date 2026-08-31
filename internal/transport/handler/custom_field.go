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

type CustomFieldHandler struct {
	svc port.CustomFieldService
	log *slog.Logger
}

func NewCustomFieldHandler(svc port.CustomFieldService, log *slog.Logger) *CustomFieldHandler {
	return &CustomFieldHandler{svc: svc, log: log}
}

// @Summary		List custom fields
// @Description	Returns all custom field definitions for a project
// @Tags			custom-fields
// @Produce		json
// @Param			id	path	string	true	"Project ID"
// @Success		200	{array}	dto.CustomFieldResponse
// @Router			/projects/{id}/custom-fields [get]
func (h *CustomFieldHandler) List(w http.ResponseWriter, r *http.Request) {
	orgID, _ := transport.OrgIDFromContext(r.Context())
	projectID := chi.URLParam(r, "id")

	fields, err := h.svc.List(r.Context(), orgID, projectID)
	if err != nil {
		transport.ServerError(w, r, h.log, err)
		return
	}
	resp := make([]*dto.CustomFieldResponse, len(fields))
	for i, f := range fields {
		resp[i] = dto.NewCustomFieldResponse(f)
	}
	transport.JSON(w, r, http.StatusOK, resp)
}

// @Summary		Create a custom field
// @Tags			custom-fields
// @Accept			json
// @Produce		json
// @Param			id		path	string						true	"Project ID"
// @Param			body	body	dto.CreateCustomFieldRequest	true	"Field details"
// @Success		201		{object}	dto.CustomFieldResponse
// @Router			/projects/{id}/custom-fields [post]
func (h *CustomFieldHandler) Create(w http.ResponseWriter, r *http.Request) {
	orgID, _ := transport.OrgIDFromContext(r.Context())
	userID, _ := transport.UserIDFromContext(r.Context())
	projectID := chi.URLParam(r, "id")

	var req dto.CreateCustomFieldRequest
	if err := dto.BindAndValidate(r, &req); err != nil {
		transport.ErrorJSON(w, r, http.StatusBadRequest, "validation_error", err.Error())
		return
	}

	options := req.Options
	if options == nil {
		options = []string{}
	}

	field, err := h.svc.Create(r.Context(), userID, domain.CreateCustomFieldParams{
		OrgID:     orgID,
		ProjectID: projectID,
		Name:      req.Name,
		FieldType: req.FieldType,
		Options:   options,
		Position:  req.Position,
	})
	if err != nil {
		transport.RespondWithError(w, r, h.log, err)
		return
	}
	transport.JSON(w, r, http.StatusCreated, dto.NewCustomFieldResponse(field))
}

// @Summary		Update a custom field
// @Tags			custom-fields
// @Accept			json
// @Produce		json
// @Param			id		path	string						true	"Project ID"
// @Param			fieldId	path	string						true	"Field ID"
// @Param			body	body	dto.UpdateCustomFieldRequest	true	"Field updates"
// @Success		200		{object}	dto.CustomFieldResponse
// @Router			/projects/{id}/custom-fields/{fieldId} [patch]
func (h *CustomFieldHandler) Update(w http.ResponseWriter, r *http.Request) {
	orgID, _ := transport.OrgIDFromContext(r.Context())
	userID, _ := transport.UserIDFromContext(r.Context())
	id := chi.URLParam(r, "fieldId")

	var req dto.UpdateCustomFieldRequest
	if err := dto.BindAndValidate(r, &req); err != nil {
		transport.ErrorJSON(w, r, http.StatusBadRequest, "validation_error", err.Error())
		return
	}

	options := req.Options
	if options == nil {
		options = []string{}
	}

	field, err := h.svc.Update(r.Context(), userID, domain.UpdateCustomFieldParams{
		ID:        id,
		OrgID:     orgID,
		ProjectID: chi.URLParam(r, "id"),
		Name:      req.Name,
		Options:   options,
		Position:  req.Position,
	})
	if err != nil {
		transport.RespondWithError(w, r, h.log, err)
		return
	}
	transport.JSON(w, r, http.StatusOK, dto.NewCustomFieldResponse(field))
}

// @Summary		Delete a custom field
// @Tags			custom-fields
// @Param			id		path	string	true	"Project ID"
// @Param			fieldId	path	string	true	"Field ID"
// @Success		204
// @Router			/projects/{id}/custom-fields/{fieldId} [delete]
func (h *CustomFieldHandler) Delete(w http.ResponseWriter, r *http.Request) {
	orgID, _ := transport.OrgIDFromContext(r.Context())
	userID, _ := transport.UserIDFromContext(r.Context())
	projectID := chi.URLParam(r, "id")
	id := chi.URLParam(r, "fieldId")

	if err := h.svc.Delete(r.Context(), userID, orgID, projectID, id); err != nil {
		transport.RespondWithError(w, r, h.log, err)
		return
	}
	w.WriteHeader(http.StatusNoContent)
}

// @Summary		Get custom field values for a task
// @Tags			custom-fields
// @Produce		json
// @Param			id		path	string	true	"Project ID"
// @Param			taskId	path	string	true	"Task ID"
// @Success		200		{object}	map[string]string
// @Router			/projects/{id}/tasks/{taskId}/custom-fields [get]
func (h *CustomFieldHandler) GetTaskValues(w http.ResponseWriter, r *http.Request) {
	orgID, _ := transport.OrgIDFromContext(r.Context())
	userID, _ := transport.UserIDFromContext(r.Context())
	taskID := chi.URLParam(r, "taskId")

	values, err := h.svc.GetTaskValues(r.Context(), userID, orgID, taskID)
	if err != nil {
		transport.ServerError(w, r, h.log, err)
		return
	}
	transport.JSON(w, r, http.StatusOK, values)
}

// @Summary		Set a custom field value for a task
// @Tags			custom-fields
// @Accept			json
// @Param			id		path	string							true	"Project ID"
// @Param			taskId	path	string							true	"Task ID"
// @Param			fieldId	path	string							true	"Field ID"
// @Param			body	body	dto.SetCustomFieldValueRequest	true	"Value"
// @Success		204
// @Router			/projects/{id}/tasks/{taskId}/custom-fields/{fieldId} [put]
func (h *CustomFieldHandler) SetValue(w http.ResponseWriter, r *http.Request) {
	orgID, _ := transport.OrgIDFromContext(r.Context())
	userID, _ := transport.UserIDFromContext(r.Context())
	taskID := chi.URLParam(r, "taskId")
	fieldID := chi.URLParam(r, "fieldId")

	var req dto.SetCustomFieldValueRequest
	if err := dto.BindAndValidate(r, &req); err != nil {
		transport.ErrorJSON(w, r, http.StatusBadRequest, "validation_error", err.Error())
		return
	}

	if err := h.svc.SetTaskValue(r.Context(), userID, orgID, taskID, fieldID, req.Value); err != nil {
		transport.RespondWithError(w, r, h.log, err)
		return
	}
	w.WriteHeader(http.StatusNoContent)
}
