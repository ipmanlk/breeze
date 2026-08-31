package handler

import (
	"log/slog"
	"net/http"

	"ipmanlk/plume/internal/port"
	"ipmanlk/plume/internal/transport"
	"ipmanlk/plume/internal/transport/dto"

	"github.com/go-chi/chi/v5"
)

type TaskDependencyHandler struct {
	svc port.TaskDependencyService
	log *slog.Logger
}

func NewTaskDependencyHandler(svc port.TaskDependencyService, log *slog.Logger) *TaskDependencyHandler {
	return &TaskDependencyHandler{svc: svc, log: log}
}

// @Summary		List blocking tasks
// @Description	Returns the tasks blocking the given task (the task is blocked by these)
// @Tags			task-dependencies
// @Produce		json
// @Param			id		path	string	true	"Project ID"
// @Param			taskId	path	string	true	"Task ID"
// @Success		200		{array}	dto.TaskResponse
// @Router			/projects/{id}/tasks/{taskId}/dependencies/blocking [get]
func (h *TaskDependencyHandler) ListBlocking(w http.ResponseWriter, r *http.Request) {
	orgID, _ := transport.OrgIDFromContext(r.Context())
	userID, _ := transport.UserIDFromContext(r.Context())
	taskID := chi.URLParam(r, "taskId")
	tasks, err := h.svc.ListBlocking(r.Context(), userID, orgID, taskID)
	if err != nil {
		transport.RespondWithError(w, r, h.log, err)
		return
	}
	resp := make([]*dto.TaskResponse, len(tasks))
	for i, t := range tasks {
		resp[i] = dto.NewTaskResponse(t)
	}
	transport.JSON(w, r, http.StatusOK, resp)
}

// @Summary		List blocked tasks
// @Description	Returns the tasks the given task is blocking (these wait on the task)
// @Tags			task-dependencies
// @Produce		json
// @Param			id		path	string	true	"Project ID"
// @Param			taskId	path	string	true	"Task ID"
// @Success		200		{array}	dto.TaskResponse
// @Router			/projects/{id}/tasks/{taskId}/dependencies/blocked [get]
func (h *TaskDependencyHandler) ListBlocked(w http.ResponseWriter, r *http.Request) {
	orgID, _ := transport.OrgIDFromContext(r.Context())
	userID, _ := transport.UserIDFromContext(r.Context())
	taskID := chi.URLParam(r, "taskId")
	tasks, err := h.svc.ListBlocked(r.Context(), userID, orgID, taskID)
	if err != nil {
		transport.RespondWithError(w, r, h.log, err)
		return
	}
	resp := make([]*dto.TaskResponse, len(tasks))
	for i, t := range tasks {
		resp[i] = dto.NewTaskResponse(t)
	}
	transport.JSON(w, r, http.StatusOK, resp)
}

// @Summary		Add a blocking dependency
// @Description	Records that the task is blocked by the given blocking task
// @Tags			task-dependencies
// @Accept			json
// @Produce		json
// @Param			id		path	string						true	"Project ID"
// @Param			taskId	path	string						true	"Task ID"
// @Param			body	body	dto.AddDependencyRequest	true	"Blocking task"
// @Success		201		{object}	transport.SuccessResponse
// @Failure		400		{object}	transport.ErrorResponse
// @Router			/projects/{id}/tasks/{taskId}/dependencies [post]
func (h *TaskDependencyHandler) Add(w http.ResponseWriter, r *http.Request) {
	orgID, _ := transport.OrgIDFromContext(r.Context())
	userID, _ := transport.UserIDFromContext(r.Context())
	taskID := chi.URLParam(r, "taskId")
	var req dto.AddDependencyRequest
	if err := dto.BindAndValidate(r, &req); err != nil {
		transport.ErrorJSON(w, r, http.StatusBadRequest, "validation_error", err.Error())
		return
	}
	if err := h.svc.Add(r.Context(), userID, orgID, taskID, req.BlocksTaskID); err != nil {
		transport.RespondWithError(w, r, h.log, err)
		return
	}
	transport.JSON(w, r, http.StatusCreated, transport.SuccessResponse{Success: true})
}

// @Summary		Remove a blocking dependency
// @Description	Removes the blocking edge between two tasks
// @Tags			task-dependencies
// @Produce		json
// @Param			id			path	string	true	"Project ID"
// @Param			taskId		path	string	true	"Task ID"
// @Param			blocksTaskId	path	string	true	"Blocking task ID"
// @Success		204	"No Content"
// @Router			/projects/{id}/tasks/{taskId}/dependencies/{blocksTaskId} [delete]
func (h *TaskDependencyHandler) Remove(w http.ResponseWriter, r *http.Request) {
	orgID, _ := transport.OrgIDFromContext(r.Context())
	userID, _ := transport.UserIDFromContext(r.Context())
	taskID := chi.URLParam(r, "taskId")
	blocksTaskID := chi.URLParam(r, "blocksTaskId")
	if err := h.svc.Remove(r.Context(), userID, orgID, taskID, blocksTaskID); err != nil {
		transport.RespondWithError(w, r, h.log, err)
		return
	}
	transport.JSON(w, r, http.StatusNoContent, nil)
}
