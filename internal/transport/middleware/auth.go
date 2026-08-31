package middleware

import (
	"context"
	"log/slog"
	"net/http"

	"ipmanlk/plume/internal/domain"
	"ipmanlk/plume/internal/i18n"
	"ipmanlk/plume/internal/port"
	"ipmanlk/plume/internal/transport"
)

func RequireAuth(authService port.AuthService, userPrefsRepo port.UserPreferencesRepository, logger *slog.Logger) func(http.Handler) http.Handler {
	return func(next http.Handler) http.Handler {
		return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
			token := extractToken(r)
			if token == "" {
				transport.LocalizedErrorJSON(w, r, http.StatusUnauthorized, "unauthorized", "ErrMissingToken")
				return
			}

			session, err := authService.ValidateSession(r.Context(), token)
			if err != nil {
				logger.Warn("invalid session", "error", err)
				transport.LocalizedErrorJSON(w, r, http.StatusUnauthorized, "unauthorized", "ErrSessionNotFound")
				return
			}

			ctx := context.WithValue(r.Context(), domain.CtxUserID, session.UserID)
			ctx = context.WithValue(ctx, domain.CtxOrgID, session.OrgID)
			ctx = context.WithValue(ctx, domain.CtxRole, string(session.Role))
			ctx = context.WithValue(ctx, domain.CtxSessionID, session.ID)

			// Override the locale (set by LocaleMiddleware from Accept-Language)
			// with the authenticated user's stored language preference, if set.
			// This ensures error responses and any requester-locale lookups honor
			// the user's explicit choice rather than their browser default.
			if userPrefsRepo != nil {
				if prefs, perr := userPrefsRepo.Get(ctx, session.UserID); perr == nil && prefs != nil && prefs.Language != "" {
					ctx = i18n.WithLocale(ctx, i18n.Normalize(prefs.Language))
				}
			}

			next.ServeHTTP(w, r.WithContext(ctx))
		})
	}
}

func extractToken(r *http.Request) string {
	if cookie, err := r.Cookie("__Host-token"); err == nil {
		return cookie.Value
	}
	return ""
}
