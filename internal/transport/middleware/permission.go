package middleware

import (
	"net/http"

	"github.com/go-chi/chi/v5"

	"ipmanlk/plume/internal/domain"
	"ipmanlk/plume/internal/port"
	"ipmanlk/plume/internal/transport"
	"ipmanlk/plume/internal/transport/access"
)

// RequirePermission gates a route on the user's ORG role. Use this for
// org-level routes (users, invites, org settings) and for project-delete
// (an org-owner/admin action). No database calls.
func RequirePermission(perms ...domain.Permission) func(http.Handler) http.Handler {
	return func(next http.Handler) http.Handler {
		return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
			roleStr, ok := transport.RoleFromContext(r.Context())
			if !ok {
				transport.LocalizedErrorJSON(w, r, http.StatusForbidden, "forbidden", "ErrNoRoleInContext")
				return
			}
			role := domain.Role(roleStr)
			for _, perm := range perms {
				if !domain.HasPermission(role, perm) {
					transport.LocalizedErrorJSON(w, r, http.StatusForbidden, "forbidden", "ErrForbidden")
					return
				}
			}
			next.ServeHTTP(w, r)
		})
	}
}

// RequireProjectPermission gates a project-scoped route on the user's
// EFFECTIVE role for the project in the {id} route param.
//
// Effective role = org role for owner/admin/member (implicit access), or the
// project-membership role for viewer/guest (403 if not a member). This is what
// makes per-project role overrides actually take effect: a viewer promoted to
// project "admin" gains admin permissions on that project only.
//
// The resolved role is stashed in the request context so handlers can read it
// via transport.EffectiveRoleFromContext and skip a second membership lookup.
func RequireProjectPermission(
	pmRepo port.ProjectMemberRepository,
	projRepo port.ProjectRepository,
	perms ...domain.Permission,
) func(http.Handler) http.Handler {
	return func(next http.Handler) http.Handler {
		return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
			projectID := chi.URLParam(r, "id")
			if projectID == "" {
				transport.LocalizedErrorJSON(w, r, http.StatusForbidden, "forbidden", "ErrMissingProjectID")
				return
			}

			role, err := access.ResolveEffectiveRole(r.Context(), pmRepo, projRepo, projectID)
			if err != nil {
				transport.LocalizedErrorJSON(w, r, http.StatusForbidden, "forbidden", "ErrNoAccessToProject")
				return
			}

			for _, perm := range perms {
				if !domain.HasPermission(role, perm) {
					transport.LocalizedErrorJSON(w, r, http.StatusForbidden, "forbidden", "ErrForbidden")
					return
				}
			}

			ctx := transport.WithEffectiveRole(r.Context(), role)
			ctx = transport.WithResolvedProject(ctx, projectID)
			next.ServeHTTP(w, r.WithContext(ctx))
		})
	}
}
