package handler

import (
	"log/slog"
	"net/http"
	"strconv"

	"ipmanlk/plume/internal/port"
	"ipmanlk/plume/internal/transport"
	"ipmanlk/plume/internal/transport/dto"

	"github.com/go-chi/chi/v5"
)

type CommentHandler struct {
	svc       port.CommentService
	accessSvc port.AccessService

	log *slog.Logger
}

func NewCommentHandler(svc port.CommentService, accessSvc port.AccessService, log *slog.Logger) *CommentHandler {
	return &CommentHandler{svc: svc, accessSvc: accessSvc, log: log}
}

// @Summary		List task comments
// @Description	Returns comments for a task (oldest first within the loaded page). Cursor-paginated: pass `before` (the oldest loaded created_at) + `limit` to load older comments.
// @Tags			comments
// @Produce		json
// @Param			id		path	string	true	"Project ID"
// @Param			taskId	path	string	true	"Task ID"
// @Param			before	query	string	false	"Cursor (created_at of oldest loaded comment)"
// @Param			limit	query	int		false	"Page size (default 50, max 100)"
// @Success		200		{object}	dto.CommentListResponse
// @Router			/projects/{id}/tasks/{taskId}/comments [get]
func (h *CommentHandler) List(w http.ResponseWriter, r *http.Request) {
	orgID, _ := transport.OrgIDFromContext(r.Context())
	projectID := chi.URLParam(r, "id")
	taskID := chi.URLParam(r, "taskId")

	// Enforce project membership for viewer/guest roles. Previously List only
	// relied on the route's RequireProjectPermission middleware (org role),
	// and the service never verified the task belonged to the URL's project.
	if err := EnsureProjectAccess(r.Context(), h.accessSvc, projectID); err != nil {
		transport.RespondWithError(w, r, h.log, err)
		return
	}

	before := r.URL.Query().Get("before")
	limit := 50
	if v := r.URL.Query().Get("limit"); v != "" {
		if n, err := strconv.Atoi(v); err == nil && n > 0 && n <= 100 {
			limit = n
		}
	}

	result, err := h.svc.ListByTask(r.Context(), orgID, taskID, projectID, before, limit)
	if err != nil {
		transport.ServerError(w, r, h.log, err)
		return
	}
	resp := &dto.CommentListResponse{
		Items:      make([]*dto.CommentResponse, len(result.Items)),
		NextCursor: result.NextCursor,
		HasMore:    result.HasMore,
	}
	for i, c := range result.Items {
		resp.Items[i] = dto.NewCommentResponse(c)
	}
	transport.JSON(w, r, http.StatusOK, resp)
}

// @Summary		Create a comment
// @Description	Adds a comment to a task. Supports @mentions (users, @everyone, channels, projects, tasks) and optional parent_id for threaded replies. Notifies mentioned users, task assignees, and the parent comment author.
// @Tags			comments
// @Accept			json
// @Produce		json
// @Param			id		path		string						true	"Project ID"
// @Param			taskId	path		string						true	"Task ID"
// @Param			body	body		dto.CreateCommentRequest	true	"Comment content"
// @Success		201		{object}	dto.CommentResponse
// @Failure		400		{object}	transport.ErrorResponse
// @Router			/projects/{id}/tasks/{taskId}/comments [post]
func (h *CommentHandler) Create(w http.ResponseWriter, r *http.Request) {
	orgID, _ := transport.OrgIDFromContext(r.Context())
	projectID := chi.URLParam(r, "id")
	taskID := chi.URLParam(r, "taskId")

	if err := EnsureProjectAccess(r.Context(), h.accessSvc, projectID); err != nil {
		transport.RespondWithError(w, r, h.log, err)
		return
	}

	var req dto.CreateCommentRequest
	if err := dto.BindAndValidate(r, &req); err != nil {
		transport.ErrorJSON(w, r, http.StatusBadRequest, "validation_error", err.Error())
		return
	}

	userID, _ := transport.UserIDFromContext(r.Context())
	comment, err := h.svc.Create(r.Context(), orgID, taskID, userID, req.Content, req.ParentID)
	if err != nil {
		transport.RespondWithError(w, r, h.log, err)
		return
	}

	transport.JSON(w, r, http.StatusCreated, dto.NewCommentResponse(comment))
}

// @Summary		Update a comment
// @Description	Edits a comment's content. Only the comment author may edit. Sets the edited_at timestamp.
// @Tags			comments
// @Accept			json
// @Produce		json
// @Param			id			path		string						true	"Project ID"
// @Param			taskId		path		string						true	"Task ID"
// @Param			commentId	path		string						true	"Comment ID"
// @Param			body		body		dto.UpdateCommentRequest	true	"Updated content"
// @Success		200			{object}	dto.CommentResponse
// @Failure		400			{object}	transport.ErrorResponse
// @Failure		403			{object}	transport.ErrorResponse
// @Failure		404			{object}	transport.ErrorResponse
// @Router			/projects/{id}/tasks/{taskId}/comments/{commentId} [patch]
func (h *CommentHandler) Update(w http.ResponseWriter, r *http.Request) {
	orgID, _ := transport.OrgIDFromContext(r.Context())

	var req dto.UpdateCommentRequest
	if err := dto.BindAndValidate(r, &req); err != nil {
		transport.ErrorJSON(w, r, http.StatusBadRequest, "validation_error", err.Error())
		return
	}

	userID, _ := transport.UserIDFromContext(r.Context())
	commentID := chi.URLParam(r, "commentId")
	comment, err := h.svc.Update(r.Context(), orgID, commentID, userID, req.Content)
	if err != nil {
		transport.RespondWithError(w, r, h.log, err)
		return
	}

	transport.JSON(w, r, http.StatusOK, dto.NewCommentResponse(comment))
}

// @Summary		Delete a comment
// @Description	Soft-deletes a comment. Only the comment author may delete.
// @Tags			comments
// @Produce		json
// @Param			id			path	string	true	"Project ID"
// @Param			taskId		path	string	true	"Task ID"
// @Param			commentId	path	string	true	"Comment ID"
// @Success		204			"No Content"
// @Failure		403			{object}	transport.ErrorResponse
// @Failure		404			{object}	transport.ErrorResponse
// @Router			/projects/{id}/tasks/{taskId}/comments/{commentId} [delete]
func (h *CommentHandler) Delete(w http.ResponseWriter, r *http.Request) {
	orgID, _ := transport.OrgIDFromContext(r.Context())
	userID, _ := transport.UserIDFromContext(r.Context())
	commentID := chi.URLParam(r, "commentId")

	if err := h.svc.Delete(r.Context(), orgID, commentID, userID); err != nil {
		transport.RespondWithError(w, r, h.log, err)
		return
	}

	transport.JSON(w, r, http.StatusNoContent, nil)
}
