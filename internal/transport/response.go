package transport

import (
	"context"
	"errors"
	"log/slog"
	"net/http"

	"ipmanlk/breeze/internal/apperr"
	"ipmanlk/breeze/internal/domain"
	"ipmanlk/breeze/internal/i18n"

	"github.com/go-chi/render"
)

// ErrorResponse is the standard API error shape for swagger docs.
type ErrorResponse struct {
	Error struct {
		Code    string `json:"code"`
		Message string `json:"message"`
	} `json:"error"`
}

// SuccessResponse is a generic success wrapper.
type SuccessResponse struct {
	Success bool `json:"success"`
}

func JSON(w http.ResponseWriter, r *http.Request, status int, data any) {
	render.Status(r, status)
	render.JSON(w, r, data)
}

func ErrorJSON(w http.ResponseWriter, r *http.Request, status int, code, message string) {
	JSON(w, r, status, map[string]any{
		"error": map[string]string{"code": code, "message": message},
	})
}

// DefaultSameSite controls the SameSite attribute on auth cookies.
// Defaults to Lax; cross-origin deployments should set it to None.
var DefaultSameSite = http.SameSiteLaxMode

func SetAuthCookie(w http.ResponseWriter, token string, maxAge int) {
	http.SetCookie(w, &http.Cookie{
		Name:     "__Host-token",
		Value:    token,
		Path:     "/",
		HttpOnly: true,
		Secure:   true,
		SameSite: DefaultSameSite,
		MaxAge:   maxAge,
	})
}

func UserIDFromContext(ctx context.Context) (string, bool) {
	id, ok := ctx.Value(domain.CtxUserID).(string)
	return id, ok
}

func OrgIDFromContext(ctx context.Context) (string, bool) {
	id, ok := ctx.Value(domain.CtxOrgID).(string)
	return id, ok
}

func RoleFromContext(ctx context.Context) (string, bool) {
	r, ok := ctx.Value(domain.CtxRole).(string)
	return r, ok
}

// WithEffectiveRole stashes the resolved project-effective role in the
// request context. Set by the RequireProjectPermission middleware after it
// resolves the role (org role for owner/admin/member, project role for
// viewer/guest) so handlers don't need to re-query project_members.
func WithEffectiveRole(ctx context.Context, role domain.Role) context.Context {
	return context.WithValue(ctx, domain.CtxEffectiveRole, role)
}

// EffectiveRoleFromContext returns the effective role previously stashed by
// RequireProjectPermission. The second return is false when the request did
// not go through that middleware (e.g. org-level or project-delete routes).
func EffectiveRoleFromContext(ctx context.Context) (domain.Role, bool) {
	role, ok := ctx.Value(domain.CtxEffectiveRole).(domain.Role)
	return role, ok
}

// WithResolvedProject records which project the stashed effective role was
// resolved for. Set alongside WithEffectiveRole by RequireProjectPermission.
func WithResolvedProject(ctx context.Context, projectID string) context.Context {
	return context.WithValue(ctx, domain.CtxEffectiveRoleProjectID, projectID)
}

// ResolvedProjectFromContext returns the project ID the stashed effective
// role belongs to (false when no project-scoped middleware ran).
func ResolvedProjectFromContext(ctx context.Context) (string, bool) {
	id, ok := ctx.Value(domain.CtxEffectiveRoleProjectID).(string)
	return id, ok
}

func SessionIDFromContext(ctx context.Context) (string, bool) {
	id, ok := ctx.Value(domain.CtxSessionID).(string)
	return id, ok
}

// i18nBundle is set once at startup by SetI18nBundle. RespondWithError uses
// it to localize user-facing error messages via the request's resolved locale.
var i18nBundle *i18n.Bundle

// SetI18nBundle configures the transport package with the i18n bundle for
// localized error responses. Must be called during startup, before any
// requests arrive.
func SetI18nBundle(b *i18n.Bundle) {
	i18nBundle = b
}

// LocalizedErrorJSON is like ErrorJSON but localizes the message via the
// request's resolved locale. messageID must match a TOML key in
// internal/i18n/messages/errors/. Falls back to messageID if the bundle
// is nil or the key is missing (safe for missing keys).
func LocalizedErrorJSON(w http.ResponseWriter, r *http.Request, status int, code, messageID string) {
	locale := i18n.LocaleFromContext(r.Context())
	localized := messageID
	if i18nBundle != nil {
		if l := i18nBundle.MustLocalize(locale, messageID, nil, nil); l != messageID {
			localized = l
		}
	}
	ErrorJSON(w, r, status, code, localized)
}

func ServerError(w http.ResponseWriter, r *http.Request, log *slog.Logger, err error) {
	log.Error("internal error", "error", err)
	ErrorJSON(w, r, http.StatusInternalServerError, "internal_error", "internal error")
}

func RespondWithError(w http.ResponseWriter, r *http.Request, log *slog.Logger, err error) {
	locale := i18n.LocaleFromContext(r.Context())
	localize := func(msg string) string {
		if i18nBundle != nil {
			if localized := i18nBundle.MustLocalize(locale, msg, nil, nil); localized != msg {
				return localized
			}
		}
		return msg
	}

	switch {
	case errors.Is(err, apperr.ErrNotFound):
		ErrorJSON(w, r, http.StatusNotFound, "not_found", localize("ErrNotFound"))
	case errors.Is(err, apperr.ErrForbiddenViewer):
		ErrorJSON(w, r, http.StatusForbidden, "forbidden", localize("ErrForbiddenViewer"))
	case errors.Is(err, apperr.ErrForbidden):
		ErrorJSON(w, r, http.StatusForbidden, "forbidden", localize("ErrForbidden"))
	case errors.Is(err, apperr.ErrInvalidInput):
		// Surface the contextual message; callers pass field/arg names, never
		// internal structure. Strip the wrapped sentinel suffix for cleanliness.
		ErrorJSON(w, r, http.StatusBadRequest, "validation_error", errorMessage(err))
	case errors.Is(err, apperr.ErrConflict):
		ErrorJSON(w, r, http.StatusConflict, "conflict", errorMessage(err))
	case errors.Is(err, apperr.ErrInvalidCreds):
		ErrorJSON(w, r, http.StatusUnauthorized, "auth_error", localize("ErrInvalidCreds"))
	case errors.Is(err, apperr.ErrUserDeactivated):
		ErrorJSON(w, r, http.StatusUnauthorized, "auth_error", localize("ErrUserDeactivated"))
	case errors.Is(err, apperr.ErrSessionExpired):
		ErrorJSON(w, r, http.StatusUnauthorized, "unauthorized", localize("ErrSessionExpired"))
	case errors.Is(err, apperr.ErrSessionNotFound):
		ErrorJSON(w, r, http.StatusUnauthorized, "unauthorized", localize("ErrSessionNotFound"))
	case errors.Is(err, apperr.ErrSetupComplete):
		ErrorJSON(w, r, http.StatusConflict, "setup_complete", localize("ErrSetupComplete"))
	case errors.Is(err, apperr.ErrAlreadyExists):
		ErrorJSON(w, r, http.StatusConflict, "conflict", localize("ErrAlreadyExists"))
	default:
		ServerError(w, r, log, err)
	}
}

// errorMessage returns err.Error() with the trailing ": <sentinel>" suffix
// stripped, so callers using apperr.InvalidInput("title is required") get
// "title is required" rather than "title is required: invalid input".
func errorMessage(err error) string {
	msg := err.Error()
	for _, sentinel := range []string{
		": " + apperr.ErrInvalidInput.Error(),
		": " + apperr.ErrConflict.Error(),
	} {
		if msg == sentinel[2:] {
			return "invalid request"
		}
		if len(msg) > len(sentinel) && msg[len(msg)-len(sentinel):] == sentinel {
			return msg[:len(msg)-len(sentinel)]
		}
	}
	return msg
}
