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

type TaskTemplateHandler struct {
	svc port.TaskTemplateService
	log *slog.Logger
}

func NewTaskTemplateHandler(svc port.TaskTemplateService, log *slog.Logger) *TaskTemplateHandler {
	return &TaskTemplateHandler{svc: svc, log: log}
}

// @Summary		List task templates
// @Description	Returns all task templates for a project
// @Tags			task-templates
// @Produce		json
// @Param			id	path	string	true	"Project ID"
// @Success		200	{array}	dto.TaskTemplateResponse
// @Router			/projects/{id}/templates [get]
func (h *TaskTemplateHandler) List(w http.ResponseWriter, r *http.Request) {
	orgID, _ := transport.OrgIDFromContext(r.Context())
	projectID := chi.URLParam(r, "id")

	templates, err := h.svc.List(r.Context(), orgID, projectID)
	if err != nil {
		transport.ServerError(w, r, h.log, err)
		return
	}
	resp := make([]*dto.TaskTemplateResponse, len(templates))
	for i, t := range templates {
		resp[i] = dto.NewTaskTemplateResponse(t)
	}
	transport.JSON(w, r, http.StatusOK, resp)
}

// @Summary		Create a task template
// @Description	Creates a new task template with optional recurrence
// @Tags			task-templates
// @Accept			json
// @Produce		json
// @Param			id		path	string						true	"Project ID"
// @Param			body	body	dto.CreateTaskTemplateRequest	true	"Template details"
// @Success		201		{object}	dto.TaskTemplateResponse
// @Failure		400		{object}	transport.ErrorResponse
// @Router			/projects/{id}/templates [post]
func (h *TaskTemplateHandler) Create(w http.ResponseWriter, r *http.Request) {
	orgID, _ := transport.OrgIDFromContext(r.Context())
	userID, _ := transport.UserIDFromContext(r.Context())
	projectID := chi.URLParam(r, "id")

	var req dto.CreateTaskTemplateRequest
	if err := dto.BindAndValidate(r, &req); err != nil {
		transport.ErrorJSON(w, r, http.StatusBadRequest, "validation_error", err.Error())
		return
	}

	assigneeIDs := req.AssigneeIDs
	if assigneeIDs == nil {
		assigneeIDs = []string{}
	}

	tmpl, err := h.svc.Create(r.Context(), domain.CreateTaskTemplateParams{
		OrgID:             orgID,
		ProjectID:         projectID,
		CreatedBy:         userID,
		Name:              req.Name,
		Description:       req.Description,
		Priority:          req.Priority,
		StatusID:          req.StatusID,
		AssigneeIDs:       assigneeIDs,
		Estimate:          req.Estimate,
		RecurrencePattern: req.RecurrencePattern,
		RecurrenceDays:    req.RecurrenceDays,
	})
	if err != nil {
		transport.RespondWithError(w, r, h.log, err)
		return
	}
	transport.JSON(w, r, http.StatusCreated, dto.NewTaskTemplateResponse(tmpl))
}

// @Summary		Update a task template
// @Tags			task-templates
// @Accept			json
// @Produce		json
// @Param			id			path	string						true	"Project ID"
// @Param			templateId	path	string						true	"Template ID"
// @Param			body		body	dto.UpdateTaskTemplateRequest	true	"Template updates"
// @Success		200			{object}	dto.TaskTemplateResponse
// @Router			/projects/{id}/templates/{templateId} [patch]
func (h *TaskTemplateHandler) Update(w http.ResponseWriter, r *http.Request) {
	orgID, _ := transport.OrgIDFromContext(r.Context())
	id := chi.URLParam(r, "templateId")

	var req dto.UpdateTaskTemplateRequest
	if err := dto.BindAndValidate(r, &req); err != nil {
		transport.ErrorJSON(w, r, http.StatusBadRequest, "validation_error", err.Error())
		return
	}

	assigneeIDs := req.AssigneeIDs
	if assigneeIDs == nil {
		assigneeIDs = []string{}
	}

	tmpl, err := h.svc.Update(r.Context(), domain.UpdateTaskTemplateParams{
		ID:                id,
		OrgID:             orgID,
		ProjectID:         chi.URLParam(r, "id"),
		Name:              req.Name,
		Description:       req.Description,
		Priority:          req.Priority,
		StatusID:          req.StatusID,
		AssigneeIDs:       assigneeIDs,
		Estimate:          req.Estimate,
		RecurrencePattern: req.RecurrencePattern,
		RecurrenceDays:    req.RecurrenceDays,
	})
	if err != nil {
		transport.RespondWithError(w, r, h.log, err)
		return
	}
	transport.JSON(w, r, http.StatusOK, dto.NewTaskTemplateResponse(tmpl))
}

// @Summary		Delete a task template
// @Tags			task-templates
// @Param			id			path	string	true	"Project ID"
// @Param			templateId	path	string	true	"Template ID"
// @Success		204
// @Router			/projects/{id}/templates/{templateId} [delete]
func (h *TaskTemplateHandler) Delete(w http.ResponseWriter, r *http.Request) {
	orgID, _ := transport.OrgIDFromContext(r.Context())
	projectID := chi.URLParam(r, "id")
	id := chi.URLParam(r, "templateId")

	if err := h.svc.Delete(r.Context(), orgID, projectID, id); err != nil {
		transport.RespondWithError(w, r, h.log, err)
		return
	}
	w.WriteHeader(http.StatusNoContent)
}

// @Summary		Instantiate a task from a template
// @Description	Creates a real task from a template. For recurring templates, advances next_run_at.
// @Tags			task-templates
// @Param			id			path	string	true	"Project ID"
// @Param			templateId	path	string	true	"Template ID"
// @Success		201			{object}	dto.TaskResponse
// @Router			/projects/{id}/templates/{templateId}/instantiate [post]
func (h *TaskTemplateHandler) Instantiate(w http.ResponseWriter, r *http.Request) {
	orgID, _ := transport.OrgIDFromContext(r.Context())
	userID, _ := transport.UserIDFromContext(r.Context())
	projectID := chi.URLParam(r, "id")
	id := chi.URLParam(r, "templateId")

	task, err := h.svc.Instantiate(r.Context(), orgID, projectID, id, userID)
	if err != nil {
		transport.RespondWithError(w, r, h.log, err)
		return
	}
	transport.JSON(w, r, http.StatusCreated, dto.NewTaskResponse(task))
}
