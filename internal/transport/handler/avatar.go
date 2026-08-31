package handler

import (
	"errors"
	"io"
	"log/slog"
	"mime"
	"net/http"
	"os"
	"path"
	"strings"

	"github.com/go-chi/chi/v5"

	"ipmanlk/plume/internal/apperr"
	"ipmanlk/plume/internal/port"
	"ipmanlk/plume/internal/storage"
	"ipmanlk/plume/internal/transport"
)

// AvatarHandler serves user avatars through an auth-protected API endpoint so
// the raw /uploads/* file server can be removed. The handler resolves the
// user by ID within the caller's org, reads the stored avatar path, and
// streams it with the correct content type.
type AvatarHandler struct {
	userRepo port.UserRepository
	storage  storage.Storage
	log      *slog.Logger
}

func NewAvatarHandler(userRepo port.UserRepository, storage storage.Storage, log *slog.Logger) *AvatarHandler {
	return &AvatarHandler{userRepo: userRepo, storage: storage, log: log}
}

// @Summary		Get user avatar
// @Description	Returns the avatar image for a user. Requires authentication.
// @Tags		users
// @Produce		json
// @Param		id	path	string	true	"User ID"
// @Success		200	file				"Avatar image"
// @Failure		404	{object} transport.ErrorResponse
// @Router		/avatars/{id} [get]
func (h *AvatarHandler) Get(w http.ResponseWriter, r *http.Request) {
	orgID, _ := transport.OrgIDFromContext(r.Context())
	userID := chi.URLParam(r, "id")
	if userID == "" {
		transport.LocalizedErrorJSON(w, r, http.StatusBadRequest, "validation_error", "ErrIDRequired")
		return
	}

	user, err := h.userRepo.GetByID(r.Context(), orgID, userID)
	if err != nil {
		transport.RespondWithError(w, r, h.log, err)
		return
	}
	if user == nil || user.AvatarURL == nil || *user.AvatarURL == "" {
		transport.LocalizedErrorJSON(w, r, http.StatusNotFound, "not_found", "ErrAvatarNotFound")
		return
	}

	// The stored AvatarURL may be the legacy public /uploads/ path or just a
	// relative storage path. Strip the /uploads/ prefix to get the storage key.
	storagePath := strings.TrimPrefix(*user.AvatarURL, "/uploads/")
	if storagePath == "" {
		transport.LocalizedErrorJSON(w, r, http.StatusNotFound, "not_found", "ErrAvatarNotFound")
		return
	}

	reader, err := h.storage.Get(r.Context(), storagePath)
	if err != nil {
		if errors.Is(err, apperr.ErrNotFound) || os.IsNotExist(err) {
			transport.LocalizedErrorJSON(w, r, http.StatusNotFound, "not_found", "ErrAvatarNotFound")
			return
		}
		transport.ServerError(w, r, h.log, err)
		return
	}
	defer reader.Close()

	contentType := mime.TypeByExtension(path.Ext(storagePath))
	if contentType == "" {
		contentType = "application/octet-stream"
	}
	w.Header().Set("Content-Type", contentType)
	w.Header().Set("Cache-Control", "public, max-age=86400")
	io.Copy(w, reader)
}
