package middleware

import (
	"net/http"

	"github.com/go-chi/chi/v5"

	"ipmanlk/plume/internal/port"
	"ipmanlk/plume/internal/transport"
)

// RejectArchivedProject blocks mutating requests (non-GET/HEAD/OPTIONS) to
// project-scoped routes when the project identified by the {id} route param
// is archived. Archived projects are read-only references; writes return 409.
//
// Reads (GET/HEAD) and CORS preflight (OPTIONS) pass through so users can
// still view an archived project's tasks, comments, and history. This runs
// after RequireProjectPermission, so access has already been verified.
func RejectArchivedProject(
	projRepo port.ProjectRepository,
) func(http.Handler) http.Handler {
	return func(next http.Handler) http.Handler {
		return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
			if r.Method == http.MethodGet || r.Method == http.MethodHead || r.Method == http.MethodOptions {
				next.ServeHTTP(w, r)
				return
			}

			projectID := chi.URLParam(r, "id")
			if projectID == "" {
				next.ServeHTTP(w, r)
				return
			}

			orgID, ok := transport.OrgIDFromContext(r.Context())
			if !ok {
				transport.LocalizedErrorJSON(w, r, http.StatusForbidden, "forbidden", "ErrOrgContextMissing")
				return
			}

			project, err := projRepo.GetByID(r.Context(), orgID, projectID)
			if err != nil {
				// Can't confirm archived state; fail closed for writes.
				transport.LocalizedErrorJSON(w, r, http.StatusForbidden, "forbidden", "ErrNoAccessToProject")
				return
			}
			if project.IsArchived {
				transport.LocalizedErrorJSON(w, r, http.StatusConflict, "project_archived", "ErrProjectArchived")
				return
			}

			next.ServeHTTP(w, r)
		})
	}
}
