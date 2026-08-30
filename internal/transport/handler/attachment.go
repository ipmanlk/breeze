package handler

import (
	"io"
	"log/slog"
	"mime"
	"net/http"
	"path/filepath"
	"strings"

	"ipmanlk/breeze/internal/domain"
	"ipmanlk/breeze/internal/port"
	"ipmanlk/breeze/internal/transport"
	"ipmanlk/breeze/internal/transport/dto"

	"github.com/go-chi/chi/v5"
)

// blockedAttachmentTypes are content types that must never be served inline on
// the Breeze origin because a browser would execute them (stored XSS). Uploads
// of these types are rejected outright; if one ever reaches storage it is
// forced to application/octet-stream on download.
var blockedAttachmentTypes = map[string]bool{
	"text/html":                   true,
	"application/xhtml+xml":       true,
	"application/javascript":      true,
	"text/javascript":             true,
	"application/ecmascript":      true,
	"application/x-httpd-php":     true,
	"application/x-sh":            true,
	"application/x-csh":           true,
	"application/x-msdos-program": true,
	"application/x-executable":    true,
	"application/x-msdownload":    true,
	"application/x-httpd-cgi":     true,
}

// isBlockedAttachmentType reports whether contentType is a scriptable /
// executable type that must not be stored or served inline.
func isBlockedAttachmentType(contentType string) bool {
	contentType = strings.ToLower(strings.TrimSpace(strings.Split(contentType, ";")[0]))
	return blockedAttachmentTypes[contentType]
}

type AttachmentHandler struct {
	svc       port.AttachmentService
	accessSvc port.AccessService

	log *slog.Logger
}

func NewAttachmentHandler(svc port.AttachmentService, accessSvc port.AccessService, log *slog.Logger) *AttachmentHandler {
	return &AttachmentHandler{svc: svc, accessSvc: accessSvc, log: log}
}

// @Summary		List attachments
// @Description	Returns all attachments for a task
// @Tags			attachments
// @Produce		json
// @Param			id		path	string	true	"Project ID"
// @Param			taskId	path	string	true	"Task ID"
// @Success		200		{array}	dto.AttachmentResponse
// @Router			/projects/{id}/tasks/{taskId}/attachments [get]
func (h *AttachmentHandler) List(w http.ResponseWriter, r *http.Request) {
	orgID, _ := transport.OrgIDFromContext(r.Context())
	taskID := chi.URLParam(r, "taskId")
	projectID := chi.URLParam(r, "id")

	if err := EnsureProjectAccess(r.Context(), h.accessSvc, projectID); err != nil {
		transport.RespondWithError(w, r, h.log, err)
		return
	}

	attachments, err := h.svc.List(r.Context(), orgID, taskID, projectID)
	if err != nil {
		transport.ServerError(w, r, h.log, err)
		return
	}
	resp := make([]*dto.AttachmentResponse, len(attachments))
	for i, a := range attachments {
		resp[i] = dto.NewAttachmentResponse(a)
	}
	transport.JSON(w, r, http.StatusOK, resp)
}

// @Summary		Upload an attachment
// @Description	Uploads a file as an attachment to a task
// @Tags			attachments
// @Accept			mpfd
// @Produce		json
// @Param			id		path		string	true	"Project ID"
// @Param			taskId	path		string	true	"Task ID"
// @Param			file	formData	file	true	"File to upload"
// @Success		201		{object}	dto.AttachmentResponse
// @Failure		400		{object}	transport.ErrorResponse
// @Router			/projects/{id}/tasks/{taskId}/attachments [post]
func (h *AttachmentHandler) Upload(w http.ResponseWriter, r *http.Request) {
	orgID, _ := transport.OrgIDFromContext(r.Context())
	taskID := chi.URLParam(r, "taskId")
	projectID := chi.URLParam(r, "id")

	if err := EnsureProjectAccess(r.Context(), h.accessSvc, projectID); err != nil {
		transport.RespondWithError(w, r, h.log, err)
		return
	}

	userID, _ := transport.UserIDFromContext(r.Context())

	// The route-level LimitRequestBody middleware (driven by MAX_UPLOAD_SIZE)
	// already caps the request body, so we only need to bound in-memory
	// multipart parsing here.
	if err := r.ParseMultipartForm(10 << 20); err != nil {
		transport.LocalizedErrorJSON(w, r, http.StatusBadRequest, "validation_error", "ErrFileTooLarge")
		return
	}

	file, header, err := r.FormFile("file")
	if err != nil {
		transport.LocalizedErrorJSON(w, r, http.StatusBadRequest, "validation_error", "ErrFileRequired")
		return
	}
	defer file.Close()

	contentType := header.Header.Get("Content-Type")
	if contentType == "" {
		// Sniff from the extension so common types (images, PDFs, docs) keep a
		// useful content type without trusting the client header for empty
		// cases.
		contentType = mime.TypeByExtension(filepath.Ext(header.Filename))
	}
	if contentType == "" {
		contentType = "application/octet-stream"
	}

	if isBlockedAttachmentType(contentType) {
		transport.LocalizedErrorJSON(w, r, http.StatusBadRequest, "validation_error", "ErrFileTypeNotAllowed")
		return
	}

	att, err := h.svc.Create(r.Context(), domain.CreateAttachmentParams{
		OrgID:       orgID,
		TaskID:      taskID,
		ProjectID:   projectID,
		CreatedBy:   userID,
		File:        file,
		Filename:    header.Filename,
		ContentType: contentType,
		Size:        header.Size,
	})
	if err != nil {
		transport.RespondWithError(w, r, h.log, err)
		return
	}

	transport.JSON(w, r, http.StatusCreated, dto.NewAttachmentResponse(att))
}

// @Summary		Delete an attachment
// @Description	Permanently deletes an attachment
// @Tags			attachments
// @Produce		json
// @Param			id				path	string	true	"Project ID"
// @Param			taskId			path	string	true	"Task ID"
// @Param			attachmentId	path	string	true	"Attachment ID"
// @Success		204				"No Content"
// @Failure		404				{object}	transport.ErrorResponse
// @Router			/projects/{id}/tasks/{taskId}/attachments/{attachmentId} [delete]
func (h *AttachmentHandler) Delete(w http.ResponseWriter, r *http.Request) {
	orgID, _ := transport.OrgIDFromContext(r.Context())
	userID, _ := transport.UserIDFromContext(r.Context())
	taskID := chi.URLParam(r, "taskId")
	projectID := chi.URLParam(r, "id")
	attachmentID := chi.URLParam(r, "attachmentId")

	if err := EnsureProjectAccess(r.Context(), h.accessSvc, projectID); err != nil {
		transport.RespondWithError(w, r, h.log, err)
		return
	}

	if err := h.svc.Delete(r.Context(), userID, orgID, attachmentID, taskID, projectID); err != nil {
		transport.LocalizedErrorJSON(w, r, http.StatusNotFound, "not_found", "ErrAttachmentNotFound")
		return
	}

	transport.JSON(w, r, http.StatusNoContent, nil)
}

// @Summary		Download an attachment
// @Description	Streams the file content for download
// @Tags			attachments
// @Produce		application/octet-stream
// @Param			attachmentId	path		string	true	"Attachment ID"
// @Success		200				{file}		binary
// @Failure		404				{object}	transport.ErrorResponse
// @Router			/attachments/{attachmentId}/download [get]
func (h *AttachmentHandler) Download(w http.ResponseWriter, r *http.Request) {
	orgID, _ := transport.OrgIDFromContext(r.Context())
	attachmentID := chi.URLParam(r, "attachmentId")

	reader, contentType, projectID, filename, err := h.svc.Download(r.Context(), orgID, attachmentID)
	if err != nil {
		transport.LocalizedErrorJSON(w, r, http.StatusNotFound, "not_found", "ErrAttachmentNotFound")
		return
	}
	defer reader.Close()

	// Check project access for the attachment's task. The service already
	// verified the task belongs to the caller's org (org-scoped lookup), so
	// this enforces per-project membership for viewer/guest roles. Bytes are
	// not streamed until this passes.
	if err := EnsureProjectAccess(r.Context(), h.accessSvc, projectID); err != nil {
		transport.RespondWithError(w, r, h.log, err)
		return
	}

	// Force a download and prevent inline rendering: serving user-uploaded
	// files inline on the Breeze origin would allow stored XSS (e.g. an
	// uploaded HTML file executing with the victim's cookies). nosniff stops
	// browsers from interpreting the bytes as a different, dangerous type.
	if isBlockedAttachmentType(contentType) {
		contentType = "application/octet-stream"
	}
	dispFilename := filename
	if dispFilename == "" {
		dispFilename = "attachment"
	}
	w.Header().Set("Content-Type", contentType)
	w.Header().Set("Content-Disposition", mime.FormatMediaType("attachment", map[string]string{"filename": dispFilename}))
	w.Header().Set("X-Content-Type-Options", "nosniff")
	io.Copy(w, reader)
}
