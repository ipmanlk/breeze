package middleware

import (
	"log/slog"
	"net/http"
	"path"
	"strings"

	"ipmanlk/breeze/internal/port"
	"ipmanlk/breeze/internal/transport"
)

func RequireSetup(orgRepo port.OrganizationRepository, logger *slog.Logger) func(http.Handler) http.Handler {
	publicPaths := map[string]bool{
		"/api/setup":      true,
		"/api/auth/login": true,
		"/setup":          true,
		"/healthz":        true,
	}

	return func(next http.Handler) http.Handler {
		return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
			if publicPaths[r.URL.Path] {
				next.ServeHTTP(w, r)
				return
			}

			exists, err := orgRepo.Exists(r.Context())
			if err != nil {
				logger.Error("check org exists", "error", err)
				transport.ErrorJSON(w, r, http.StatusInternalServerError, "internal_error", "internal error")
				return
			}

			if !exists {
				if strings.HasPrefix(r.URL.Path, "/api") {
					transport.JSON(w, r, http.StatusPreconditionFailed, map[string]bool{"needs_setup": true})
					return
				}
				if path.Ext(r.URL.Path) != "" {
					next.ServeHTTP(w, r)
					return
				}
				http.Redirect(w, r, "/setup", http.StatusFound)
				return
			}

			next.ServeHTTP(w, r)
		})
	}
}
