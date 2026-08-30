package handler

import (
	"io"
	"log/slog"
	"net/http"

	"ipmanlk/breeze/internal/port"
	"ipmanlk/breeze/internal/transport"
	"ipmanlk/breeze/internal/transport/dto"
)

type BackupHandler struct {
	svc port.BackupService
	log *slog.Logger
}

func NewBackupHandler(svc port.BackupService, log *slog.Logger) *BackupHandler {
	return &BackupHandler{svc: svc, log: log}
}

// @Summary		Download database backup
// @Description	Creates a VACUUM INTO snapshot of the SQLite database and streams it as a downloadable .db file.
// @Tags			backup
// @Produce		octet-stream
// @Success		200		{file}		binary
// @Failure		403		{object}	transport.ErrorResponse
// @Failure		500		{object}	transport.ErrorResponse
// @Router			/backup/download [get]
func (h *BackupHandler) Download(w http.ResponseWriter, r *http.Request) {
	reader, filename, err := h.svc.DownloadBackup(r.Context())
	if err != nil {
		transport.ServerError(w, r, h.log, err)
		return
	}
	defer reader.Close()

	w.Header().Set("Content-Type", "application/vnd.sqlite3")
	w.Header().Set("Content-Disposition", `attachment; filename="`+filename+`"`)
	w.Header().Set("X-Content-Type-Options", "nosniff")
	if _, err := io.Copy(w, reader); err != nil {
		h.log.Warn("backup download stream interrupted", "error", err)
	}
}

// @Summary		Stage a database backup for restore
// @Description	Accepts an uploaded .db backup file, validates it, and stages it for restore. The server must be restarted to apply the restore.
// @Tags			backup
// @Accept			multipart/form-data
// @Param			file	formData	file	true	"Uploaded .db backup file"
// @Success		200		{object}	dto.BackupRestoreResponse
// @Failure		400		{object}	transport.ErrorResponse
// @Failure		403		{object}	transport.ErrorResponse
// @Router			/backup/restore [post]
func (h *BackupHandler) StageRestore(w http.ResponseWriter, r *http.Request) {
	// 200MB max; SQLite DBs for self-hosted PM are well under this.
	if err := r.ParseMultipartForm(200 << 20); err != nil {
		transport.LocalizedErrorJSON(w, r, http.StatusBadRequest, "invalid_upload", "ErrFileTooLarge")
		return
	}
	file, _, err := r.FormFile("file")
	if err != nil {
		transport.LocalizedErrorJSON(w, r, http.StatusBadRequest, "missing_file", "ErrNoFileUploaded")
		return
	}
	defer file.Close()

	path, err := h.svc.StageRestore(r.Context(), file)
	if err != nil {
		transport.RespondWithError(w, r, h.log, err)
		return
	}
	h.log.Info("backup staged for restore", "path", path)
	transport.JSON(w, r, http.StatusOK, dto.BackupRestoreResponse{
		Success: true,
		Message: "Backup staged. Restart the server to apply.",
	})
}

// @Summary		Check for a pending staged restore
// @Description	Returns whether a staged restore file exists (awaiting server restart).
// @Tags			backup
// @Produce		json
// @Success		200	{object}	dto.BackupRestorePendingResponse
// @Router			/backup/restore/pending [get]
func (h *BackupHandler) CheckPendingRestore(w http.ResponseWriter, r *http.Request) {
	path, size, ok := h.svc.PendingRestoreInfo()
	transport.JSON(w, r, http.StatusOK, dto.BackupRestorePendingResponse{
		Pending: ok,
		Path:    path,
		Size:    size,
	})
}

// @Summary		Cancel a staged restore
// @Description	Removes a staged restore file so it will not be applied on next restart.
// @Tags			backup
// @Success		204
// @Failure		500	{object}	transport.ErrorResponse
// @Router			/backup/restore/pending [delete]
func (h *BackupHandler) ClearPendingRestore(w http.ResponseWriter, r *http.Request) {
	if err := h.svc.ClearPendingRestore(); err != nil {
		transport.ServerError(w, r, h.log, err)
		return
	}
	w.WriteHeader(http.StatusNoContent)
}
