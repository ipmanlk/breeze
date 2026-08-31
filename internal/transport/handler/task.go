package handler

import (
	"log/slog"
	"net/http"
	"strconv"
	"time"

	"ipmanlk/plume/internal/domain"
	"ipmanlk/plume/internal/port"
	"ipmanlk/plume/internal/transport"
	"ipmanlk/plume/internal/transport/dto"

	"github.com/go-chi/chi/v5"
)

type TaskHandler struct {
	svc         port.TaskService
	accessSvc   port.AccessService
	templateSvc port.TaskTemplateService
	audit       port.AuditService
	log         *slog.Logger
}

func NewTaskHandler(svc port.TaskService, accessSvc port.AccessService, log *slog.Logger) *TaskHandler {
	return &TaskHandler{svc: svc, accessSvc: accessSvc, log: log}
}

// SetAuditService injects the audit service so key task events (create/delete)
// are recorded in the unified audit log. Optional: when nil, task
// events are not audited (only the granular task_activity feed is populated).
func (h *TaskHandler) SetAuditService(audit port.AuditService) {
	h.audit = audit
}

// SetTemplateService injects the template service for lazy recurring task processing.
func (h *TaskHandler) SetTemplateService(svc port.TaskTemplateService) {
	h.templateSvc = svc
}

// @Summary		List tasks
// @Description	Returns tasks for a project, optionally filtered
// @Tags			tasks
// @Produce		json
// @Param			id			path	string	true	"Project ID"
// @Param			status_id	query	string	false	"Filter by status ID"
// @Param			assignee_id	query	string	false	"Filter by assignee ID"
// @Param			priority	query	string	false	"Filter by priority"
// @Param			cycle_id	query	string	false	"Filter by cycle ID"
// @Param			label_ids	query	string	false	"Comma-separated label IDs to filter by"
// @Param			q			query	string	false	"Search by title"
// @Param			include_subtasks	query	bool	false	"Include subtasks in the result (default false - top-level only)"
// @Success		200			{array}	dto.TaskResponse
// @Router			/projects/{id}/tasks [get]
func (h *TaskHandler) List(w http.ResponseWriter, r *http.Request) {
	// Lazily process any due recurring templates so new task instances
	// appear without needing a background job.
	if h.templateSvc != nil {
		if err := h.templateSvc.ProcessDueRecurring(r.Context()); err != nil {
			h.log.Error("recurring template processing failed", "error", err)
		}
	}
	orgID, _ := transport.OrgIDFromContext(r.Context())
	projectID := chi.URLParam(r, "id")

	if err := EnsureProjectAccess(r.Context(), h.accessSvc, projectID); err != nil {
		transport.RespondWithError(w, r, h.log, err)
		return
	}

	var cycleID *string
	if v := r.URL.Query().Get("cycle_id"); v != "" {
		cycleID = &v
	}
	var statusID *string
	if v := r.URL.Query().Get("status_id"); v != "" {
		statusID = &v
	}
	var assigneeID *string
	if v := r.URL.Query().Get("assignee_id"); v != "" {
		assigneeID = &v
	}

	filter := domain.TaskFilter{
		StatusID:        statusID,
		AssigneeID:      assigneeID,
		CycleID:         cycleID,
		Priority:        r.URL.Query().Get("priority"),
		Search:          r.URL.Query().Get("q"),
		LabelIDs:        parseCSVQueryParam(r.URL.Query().Get("label_ids")),
		IncludeSubtasks: r.URL.Query().Get("include_subtasks") == "true" || r.URL.Query().Get("include_subtasks") == "1",
	}

	tasks, err := h.svc.List(r.Context(), orgID, projectID, filter)
	if err != nil {
		transport.ServerError(w, r, h.log, err)
		return
	}
	resp := make([]*dto.TaskResponse, len(tasks))
	for i, t := range tasks {
		resp[i] = dto.NewTaskResponse(t)
	}
	transport.JSON(w, r, http.StatusOK, resp)
}

// @Summary		Create a task
// @Description	Creates a new task in a project
// @Tags			tasks
// @Accept			json
// @Produce		json
// @Param			id		path		string					true	"Project ID"
// @Param			body	body		dto.CreateTaskRequest	true	"Task details"
// @Success		201		{object}	dto.TaskResponse
// @Failure		400		{object}	transport.ErrorResponse
// @Router			/projects/{id}/tasks [post]
func (h *TaskHandler) Create(w http.ResponseWriter, r *http.Request) {
	var req dto.CreateTaskRequest
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

	var startedAt, dueAt *time.Time
	if req.StartedAt != nil {
		t, err := time.Parse(time.RFC3339, *req.StartedAt)
		if err != nil {
			transport.LocalizedErrorJSON(w, r, http.StatusBadRequest, "validation_error", "ErrInvalidStartsAt")
			return
		}
		startedAt = &t
	}
	if req.DueAt != nil {
		t, err := time.Parse(time.RFC3339, *req.DueAt)
		if err != nil {
			transport.LocalizedErrorJSON(w, r, http.StatusBadRequest, "validation_error", "ErrInvalidEndsAt")
			return
		}
		dueAt = &t
	}

	task, err := h.svc.Create(r.Context(), domain.CreateTaskParams{
		OrgID:       orgID,
		ProjectID:   projectID,
		CreatedBy:   userID,
		Title:       req.Title,
		Description: req.Description,
		StatusID:    req.StatusID,
		Priority:    req.Priority,
		AssigneeIDs: req.AssigneeIDs,
		CycleID:     req.CycleID,
		ParentID:    req.ParentID,
		Estimate:    req.Estimate,
		StartedAt:   startedAt,
		DueAt:       dueAt,
	})
	if err != nil {
		transport.RespondWithError(w, r, h.log, err)
		return
	}

	if h.audit != nil {
		h.audit.Record(r.Context(), orgID, userID, domain.AuditActionTaskCreated, "task", task.ID, map[string]string{"title": task.Title, "project_id": projectID})
	}

	transport.JSON(w, r, http.StatusCreated, dto.NewTaskResponse(task))
}

// @Summary		Get a task
// @Description	Returns a single task by ID
// @Tags			tasks
// @Produce		json
// @Param			id		path	string	true	"Project ID"
// @Param			taskId	path	string	true	"Task ID"
// @Success		200		{object}	dto.TaskResponse
// @Failure		404		{object}	transport.ErrorResponse
// @Router			/projects/{id}/tasks/{taskId} [get]
func (h *TaskHandler) Get(w http.ResponseWriter, r *http.Request) {
	orgID, _ := transport.OrgIDFromContext(r.Context())
	projectID := chi.URLParam(r, "id")
	taskID := chi.URLParam(r, "taskId")

	if err := EnsureProjectAccess(r.Context(), h.accessSvc, projectID); err != nil {
		transport.RespondWithError(w, r, h.log, err)
		return
	}

	task, err := h.svc.GetByID(r.Context(), orgID, taskID, projectID)
	if err != nil {
		transport.RespondWithError(w, r, h.log, err)
		return
	}
	transport.JSON(w, r, http.StatusOK, dto.NewTaskResponse(task))
}

// @Summary		List subtasks
// @Description	Returns the direct children (subtasks) of a task
// @Tags			tasks
// @Produce		json
// @Param			id		path	string	true	"Project ID"
// @Param			taskId	path	string	true	"Task ID"
// @Success		200		{array}	dto.TaskResponse
// @Failure		404		{object}	transport.ErrorResponse
// @Router			/projects/{id}/tasks/{taskId}/subtasks [get]
func (h *TaskHandler) ListSubtasks(w http.ResponseWriter, r *http.Request) {
	orgID, _ := transport.OrgIDFromContext(r.Context())
	projectID := chi.URLParam(r, "id")
	taskID := chi.URLParam(r, "taskId")

	if err := EnsureProjectAccess(r.Context(), h.accessSvc, projectID); err != nil {
		transport.RespondWithError(w, r, h.log, err)
		return
	}

	subtasks, err := h.svc.ListSubtasks(r.Context(), orgID, projectID, taskID)
	if err != nil {
		transport.RespondWithError(w, r, h.log, err)
		return
	}
	resp := make([]*dto.TaskResponse, len(subtasks))
	for i, t := range subtasks {
		resp[i] = dto.NewTaskResponse(t)
	}
	transport.JSON(w, r, http.StatusOK, resp)
}

// @Summary		Reorder subtasks
// @Description	Re-orders a task's direct children by updating their subtask_position keys
// @Tags			tasks
// @Accept		json
// @Produce		json
// @Param			id		path	string			true	"Project ID"
// @Param			taskId	path	string			true	"Task ID"
// @Param			body	body	dto.ReorderRequest	true	"Reorder operations"
// @Success		200		{object}	map[string]bool
// @Failure		400		{object}	transport.ErrorResponse
// @Router			/projects/{id}/tasks/{taskId}/subtasks/reorder [post]
func (h *TaskHandler) ReorderSubtasks(w http.ResponseWriter, r *http.Request) {
	orgID, _ := transport.OrgIDFromContext(r.Context())
	projectID := chi.URLParam(r, "id")
	taskID := chi.URLParam(r, "taskId")

	if err := EnsureProjectAccess(r.Context(), h.accessSvc, projectID); err != nil {
		transport.RespondWithError(w, r, h.log, err)
		return
	}

	var req dto.ReorderRequest
	if err := dto.BindAndValidate(r, &req); err != nil {
		transport.ErrorJSON(w, r, http.StatusBadRequest, "validation_error", err.Error())
		return
	}

	ops := make([]domain.ReorderOp, len(req.Operations))
	for i, op := range req.Operations {
		ops[i] = domain.ReorderOp{
			TaskID:      op.TaskID,
			PositionKey: op.PositionKey,
		}
	}

	if err := h.svc.ReorderSubtasks(r.Context(), orgID, projectID, taskID, ops); err != nil {
		transport.RespondWithError(w, r, h.log, err)
		return
	}
	transport.JSON(w, r, http.StatusOK, map[string]bool{"success": true})
}

// @Summary		Update a task
// @Description	Updates task fields
// @Tags			tasks
// @Accept			json
// @Produce		json
// @Param			id		path		string					true	"Project ID"
// @Param			taskId	path		string					true	"Task ID"
// @Param			body	body		dto.UpdateTaskRequest	true	"Updated fields"
// @Success		200		{object}	dto.TaskResponse
// @Failure		400		{object}	transport.ErrorResponse
// @Router			/projects/{id}/tasks/{taskId} [put]
func (h *TaskHandler) Update(w http.ResponseWriter, r *http.Request) {
	orgID, _ := transport.OrgIDFromContext(r.Context())
	userID, _ := transport.UserIDFromContext(r.Context())
	projectID := chi.URLParam(r, "id")
	taskID := chi.URLParam(r, "taskId")

	if err := EnsureProjectAccess(r.Context(), h.accessSvc, projectID); err != nil {
		transport.RespondWithError(w, r, h.log, err)
		return
	}

	existing, err := h.svc.GetByID(r.Context(), orgID, taskID, projectID)
	if err != nil {
		transport.RespondWithError(w, r, h.log, err)
		return
	}

	var req dto.UpdateTaskRequest
	if err := dto.BindAndValidate(r, &req); err != nil {
		transport.ErrorJSON(w, r, http.StatusBadRequest, "validation_error", err.Error())
		return
	}

	if req.Title != nil {
		existing.Title = *req.Title
	}
	if req.Description != nil {
		existing.Description = *req.Description
	}
	if req.StatusID != nil {
		existing.StatusID = *req.StatusID
	}
	if req.Priority != nil {
		existing.Priority = *req.Priority
	}
	if req.CycleID != nil {
		existing.CycleID = req.CycleID
	}
	if req.Estimate != nil {
		existing.Estimate = req.Estimate
	}

	if req.StartedAt != nil {
		if *req.StartedAt == "" {
			existing.StartedAt = nil
		} else {
			t, err := time.Parse(time.RFC3339, *req.StartedAt)
			if err != nil {
				transport.LocalizedErrorJSON(w, r, http.StatusBadRequest, "validation_error", "ErrInvalidStartsAt")
				return
			}
			existing.StartedAt = &t
		}
	}
	if req.DueAt != nil {
		if *req.DueAt == "" {
			existing.DueAt = nil
		} else {
			t, err := time.Parse(time.RFC3339, *req.DueAt)
			if err != nil {
				transport.LocalizedErrorJSON(w, r, http.StatusBadRequest, "validation_error", "ErrInvalidEndsAt")
				return
			}
			existing.DueAt = &t
		}
	}

	if req.AssigneeIDs != nil {
		assignees := make([]domain.TaskAssignee, len(req.AssigneeIDs))
		for i, id := range req.AssigneeIDs {
			assignees[i] = domain.TaskAssignee{ID: id}
		}
		existing.Assignees = assignees
	}
	// Reparent: nil/omitted leaves the parent unchanged; empty string clears
	// it (promotes to top-level); a non-empty value sets/changes the parent.
	if req.ParentID != nil {
		if *req.ParentID == "" {
			existing.ParentID = nil
		} else {
			existing.ParentID = req.ParentID
		}
	}

	if err := h.svc.Update(r.Context(), userID, existing); err != nil {
		transport.RespondWithError(w, r, h.log, err)
		return
	}

	task, err := h.svc.GetByID(r.Context(), orgID, taskID, projectID)
	if err != nil {
		transport.RespondWithError(w, r, h.log, err)
		return
	}
	transport.JSON(w, r, http.StatusOK, dto.NewTaskResponse(task))
}

// @Summary		Delete a task
// @Description	Deletes a task. When the task has subtasks, mode controls their fate: block (default, returns 409), cascade (delete children), or promote (children become top-level).
// @Tags			tasks
// @Produce		json
// @Param			id		path	string	true	"Project ID"
// @Param			taskId	path	string	true	"Task ID"
// @Param			mode	query	string	false	"Subtask handling: block (default), cascade, promote"
// @Success		204
// @Router			/projects/{id}/tasks/{taskId} [delete]
func (h *TaskHandler) Delete(w http.ResponseWriter, r *http.Request) {
	orgID, _ := transport.OrgIDFromContext(r.Context())
	userID, _ := transport.UserIDFromContext(r.Context())
	projectID := chi.URLParam(r, "id")
	taskID := chi.URLParam(r, "taskId")

	if err := EnsureProjectAccess(r.Context(), h.accessSvc, projectID); err != nil {
		transport.RespondWithError(w, r, h.log, err)
		return
	}

	mode := domain.DeleteSubtaskModeBlock
	switch r.URL.Query().Get("mode") {
	case "cascade":
		mode = domain.DeleteSubtaskModeCascade
	case "promote":
		mode = domain.DeleteSubtaskModePromote
	}
	if err := h.svc.Delete(r.Context(), orgID, taskID, projectID, mode, userID); err != nil {
		transport.RespondWithError(w, r, h.log, err)
		return
	}
	if h.audit != nil {
		userID, _ := transport.UserIDFromContext(r.Context())
		h.audit.Record(r.Context(), orgID, userID, domain.AuditActionTaskDeleted, "task", taskID, map[string]string{"project_id": projectID, "mode": r.URL.Query().Get("mode")})
	}
	w.WriteHeader(http.StatusNoContent)
}

// @Summary		Move a task
// @Description	Changes task status and position
// @Tags			tasks
// @Accept			json
// @Produce		json
// @Param			id		path		string					true	"Project ID"
// @Param			taskId	path		string					true	"Task ID"
// @Param			body	body		dto.MoveTaskRequest		true	"New status and position"
// @Success		200		{object}	dto.TaskResponse
// @Failure		400		{object}	transport.ErrorResponse
// @Router			/projects/{id}/tasks/{taskId}/position [patch]
func (h *TaskHandler) Move(w http.ResponseWriter, r *http.Request) {
	orgID, _ := transport.OrgIDFromContext(r.Context())
	userID, _ := transport.UserIDFromContext(r.Context())
	projectID := chi.URLParam(r, "id")
	taskID := chi.URLParam(r, "taskId")

	if err := EnsureProjectAccess(r.Context(), h.accessSvc, projectID); err != nil {
		transport.RespondWithError(w, r, h.log, err)
		return
	}

	var req dto.MoveTaskRequest
	if err := dto.BindAndValidate(r, &req); err != nil {
		transport.ErrorJSON(w, r, http.StatusBadRequest, "validation_error", err.Error())
		return
	}

	if err := h.svc.Move(r.Context(), userID, orgID, taskID, projectID, req.StatusID, req.PositionKey); err != nil {
		transport.RespondWithError(w, r, h.log, err)
		return
	}

	task, err := h.svc.GetByID(r.Context(), orgID, taskID, projectID)
	if err != nil {
		transport.RespondWithError(w, r, h.log, err)
		return
	}

	transport.JSON(w, r, http.StatusOK, dto.NewTaskResponse(task))
}

// @Summary		Reorder tasks
// @Description	Bulk-update positions for multiple tasks in a project
// @Tags			tasks
// @Accept			json
// @Produce		json
// @Param			id		path		string					true	"Project ID"
// @Param			body	body		dto.ReorderRequest		true	"Reorder operations"
// @Success		200		{array}		dto.TaskResponse
// @Failure		400		{object}	transport.ErrorResponse
// @Router			/projects/{id}/tasks/reorder [post]
func (h *TaskHandler) Reorder(w http.ResponseWriter, r *http.Request) {
	orgID, _ := transport.OrgIDFromContext(r.Context())
	projectID := chi.URLParam(r, "id")

	if err := EnsureProjectAccess(r.Context(), h.accessSvc, projectID); err != nil {
		transport.RespondWithError(w, r, h.log, err)
		return
	}

	var req dto.ReorderRequest
	if err := dto.BindAndValidate(r, &req); err != nil {
		transport.ErrorJSON(w, r, http.StatusBadRequest, "validation_error", err.Error())
		return
	}

	ops := make([]domain.ReorderOp, len(req.Operations))
	for i, op := range req.Operations {
		ops[i] = domain.ReorderOp{
			TaskID:      op.TaskID,
			PositionKey: op.PositionKey,
		}
	}

	if err := h.svc.Reorder(r.Context(), orgID, projectID, ops); err != nil {
		transport.RespondWithError(w, r, h.log, err)
		return
	}

	transport.JSON(w, r, http.StatusOK, map[string]bool{"success": true})
}

// @Summary		Bulk-update tasks
// @Description	Applies a partial update (status/priority/assignees/cycle) to many tasks in a project at once
// @Tags			tasks
// @Accept			json
// @Produce			json
// @Param			id		path	string				true	"Project ID"
// @Param			body	body	dto.BatchUpdateRequest	true	"Batch update"
// @Success		200		{array}	dto.TaskResponse
// @Failure		400		{object}	transport.ErrorResponse
// @Router			/projects/{id}/tasks/batch [post]
func (h *TaskHandler) BatchUpdate(w http.ResponseWriter, r *http.Request) {
	orgID, _ := transport.OrgIDFromContext(r.Context())
	userID, _ := transport.UserIDFromContext(r.Context())
	projectID := chi.URLParam(r, "id")

	if err := EnsureProjectAccess(r.Context(), h.accessSvc, projectID); err != nil {
		transport.RespondWithError(w, r, h.log, err)
		return
	}

	var req dto.BatchUpdateRequest
	if err := dto.BindAndValidate(r, &req); err != nil {
		transport.ErrorJSON(w, r, http.StatusBadRequest, "validation_error", err.Error())
		return
	}

	updated, err := h.svc.BatchUpdate(r.Context(), orgID, domain.BatchUpdateParams{
		TaskIDs:      req.TaskIDs,
		ProjectID:    projectID,
		StatusID:     req.StatusID,
		Priority:     req.Priority,
		AssigneeIDs:  req.AssigneeIDs,
		AssigneeMode: req.AssigneeMode,
		CycleID:      req.CycleID,
	}, userID)
	if err != nil {
		transport.RespondWithError(w, r, h.log, err)
		return
	}

	resp := make([]*dto.TaskResponse, len(updated))
	for i, t := range updated {
		resp[i] = dto.NewTaskResponse(t)
	}
	transport.JSON(w, r, http.StatusOK, resp)
}

// @Summary		Duplicate a task
// @Description	Creates a copy of a task in the same project + status with a fresh ID
// @Tags			tasks
// @Produce			json
// @Param			id		path	string	true	"Project ID"
// @Param			taskId	path	string	true	"Task ID"
// @Param			include_subtasks	query	bool	false	"Also duplicate the task's subtasks (default false)"
// @Success		201		{object}	dto.TaskResponse
// @Failure		400		{object}	transport.ErrorResponse
// @Router			/projects/{id}/tasks/{taskId}/duplicate [post]
func (h *TaskHandler) Duplicate(w http.ResponseWriter, r *http.Request) {
	orgID, _ := transport.OrgIDFromContext(r.Context())
	userID, _ := transport.UserIDFromContext(r.Context())
	projectID := chi.URLParam(r, "id")
	taskID := chi.URLParam(r, "taskId")

	if err := EnsureProjectAccess(r.Context(), h.accessSvc, projectID); err != nil {
		transport.RespondWithError(w, r, h.log, err)
		return
	}

	includeSubtasks := r.URL.Query().Get("include_subtasks") == "true" || r.URL.Query().Get("include_subtasks") == "1"
	task, err := h.svc.Duplicate(r.Context(), orgID, taskID, projectID, includeSubtasks, userID)
	if err != nil {
		transport.RespondWithError(w, r, h.log, err)
		return
	}
	transport.JSON(w, r, http.StatusCreated, dto.NewTaskResponse(task))
}

// @Summary		Move a task to another project
// @Description	Re-homes a task into a different project + status (clears cycle + parent)
// @Tags			tasks
// @Accept			json
// @Produce			json
// @Param			id		path	string				true	"Source project ID"
// @Param			taskId	path	string				true	"Task ID"
// @Param			body	body	dto.MoveToProjectRequest	true	"Move target"
// @Success		200		{object}	dto.TaskResponse
// @Failure		400		{object}	transport.ErrorResponse
// @Router			/projects/{id}/tasks/{taskId}/move [post]
func (h *TaskHandler) MoveToProject(w http.ResponseWriter, r *http.Request) {
	orgID, _ := transport.OrgIDFromContext(r.Context())
	userID, _ := transport.UserIDFromContext(r.Context())
	fromProjectID := chi.URLParam(r, "id")
	taskID := chi.URLParam(r, "taskId")

	// Caller needs edit access to the source project to remove the task.
	if err := EnsureProjectAccess(r.Context(), h.accessSvc, fromProjectID); err != nil {
		transport.RespondWithError(w, r, h.log, err)
		return
	}

	var req dto.MoveToProjectRequest
	if err := dto.BindAndValidate(r, &req); err != nil {
		transport.ErrorJSON(w, r, http.StatusBadRequest, "validation_error", err.Error())
		return
	}

	// Caller also needs create access in the target project (it's gaining a task).
	if err := EnsureProjectAccess(r.Context(), h.accessSvc, req.ToProjectID); err != nil {
		transport.RespondWithError(w, r, h.log, err)
		return
	}

	task, err := h.svc.MoveToProject(r.Context(), orgID, taskID, fromProjectID, req.ToProjectID, req.ToStatusID, userID)
	if err != nil {
		transport.RespondWithError(w, r, h.log, err)
		return
	}
	transport.JSON(w, r, http.StatusOK, dto.NewTaskResponse(task))
}

// @Description  Returns tasks assigned to the current user across all projects, with filtering, searching, and status/project info.
// @Tags         tasks
// @Produce      json
// @Param        status_id       query    string  false  "Filter by status ID"
// @Param        assignee_id     query    string  false  "Filter by assignee ID (defaults to current user)"
// @Param        label_ids       query    string  false  "Comma-separated label IDs"
// @Param        cycle_id        query    string  false  "Filter by cycle ID"
// @Param        priority        query    string  false  "Filter by priority"
// @Param        q               query    string  false  "Search by title"
// @Param        show_completed  query    bool    false  "Include completed tasks"
// @Param        group_by        query    string  false  "Group by: status, priority, project"
// @Param        offset          query    int     false  "Pagination offset (default 0)"
// @Param        limit           query    int     false  "Max results per page (default 20, max 50)"
// @Success      200             {object}  dto.TaskListPageResponse
// @Router       /tasks [get]
func (h *TaskHandler) ListTasks(w http.ResponseWriter, r *http.Request) {
	orgID, _ := transport.OrgIDFromContext(r.Context())
	userID, _ := transport.UserIDFromContext(r.Context())
	roleStr, _ := transport.RoleFromContext(r.Context())

	var statusID, cycleID, assigneeID *string
	if v := r.URL.Query().Get("status_id"); v != "" {
		statusID = &v
	}
	if v := r.URL.Query().Get("cycle_id"); v != "" {
		cycleID = &v
	}
	if v := r.URL.Query().Get("assignee_id"); v != "" {
		assigneeID = &v
	}

	// The RequireProjectMembership business rule (viewer/guest must have
	// explicit project membership) is enforced inside the service.
	filter := domain.TaskListFilter{
		StatusID:   statusID,
		CycleID:    cycleID,
		AssigneeID: assigneeID,
		Priority:   r.URL.Query().Get("priority"),
		Search:     r.URL.Query().Get("q"),
		GroupBy:    r.URL.Query().Get("group_by"),
		Cursor:     r.URL.Query().Get("cursor"),
		LabelIDs:   parseCSVQueryParam(r.URL.Query().Get("label_ids")),
	}

	if v := r.URL.Query().Get("show_completed"); v == "true" || v == "1" {
		filter.ShowCompleted = true
	}
	if v := r.URL.Query().Get("limit"); v != "" {
		n, err := strconv.Atoi(v)
		if err != nil || n <= 0 {
			transport.LocalizedErrorJSON(w, r, http.StatusBadRequest, "validation_error", "ErrInvalidLimit")
			return
		}
		filter.Limit = n
	}
	if filter.Limit <= 0 || filter.Limit > 50 {
		filter.Limit = 20
	}

	result, err := h.svc.ListTasks(r.Context(), orgID, userID, domain.Role(roleStr), filter)
	if err != nil {
		transport.ServerError(w, r, h.log, err)
		return
	}

	items := make([]*dto.TaskListResponse, len(result.Items))
	for i, t := range result.Items {
		items[i] = dto.NewTaskListResponse(t)
	}
	transport.JSON(w, r, http.StatusOK, dto.TaskListPageResponse{
		Items:      items,
		NextCursor: result.NextCursor,
		HasMore:    result.HasMore,
	})
}

// @Summary		List task activity
// @Description	Returns the activity history for a task (status, assignee, priority, due date changes)
// @Tags			tasks
// @Produce		json
// @Param			id		path	string	true	"Project ID"
// @Param			taskId	path	string	true	"Task ID"
// @Param			cursor	query	string	false	"Pagination cursor"
// @Param			limit	query	int	false	"Page size (max 100)"
// @Success		200		{object}	dto.PaginatedTaskActivityResponse
// @Failure		404		{object}	transport.ErrorResponse
// @Router			/projects/{id}/tasks/{taskId}/activity [get]
func (h *TaskHandler) ListActivity(w http.ResponseWriter, r *http.Request) {
	orgID, _ := transport.OrgIDFromContext(r.Context())
	projectID := chi.URLParam(r, "id")
	taskID := chi.URLParam(r, "taskId")

	if err := EnsureProjectAccess(r.Context(), h.accessSvc, projectID); err != nil {
		transport.RespondWithError(w, r, h.log, err)
		return
	}

	filter := domain.TaskActivityFilter{
		Cursor: r.URL.Query().Get("cursor"),
		Limit:  20,
	}
	if l, err := strconv.Atoi(r.URL.Query().Get("limit")); err == nil && l > 0 && l <= 100 {
		filter.Limit = l
	}

	result, err := h.svc.ListActivity(r.Context(), orgID, projectID, taskID, filter)
	if err != nil {
		transport.RespondWithError(w, r, h.log, err)
		return
	}

	transport.JSON(w, r, http.StatusOK, dto.NewPaginatedTaskActivityResponse(result))
}
