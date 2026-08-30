// Package access resolves the effective role a user has on a given project.
//
// The model:
//   - Org owner/admin/member have implicit, org-wide access to every project
//     within their own org. Their effective role on any project is their org
//     role. A per-project role override is NOT consulted for them.
//   - Org viewer/guest have no implicit project access. Their effective role
//     on a project is the role stored in project_members; if they have no
//     row there, they get nothing (ErrNotProjectMember).
//
// ResolveEffectiveRole is the single source of truth used both by the
// RequireProjectPermission middleware (to gate project-scoped routes on the
// effective role) and by handler guards.
package access

import (
	"context"

	"ipmanlk/breeze/internal/apperr"
	"ipmanlk/breeze/internal/domain"
	"ipmanlk/breeze/internal/port"
	"ipmanlk/breeze/internal/transport"
)

var (
	ErrNotProjectMember = apperr.ErrNotFound
	ErrForbidden        = apperr.ErrForbidden
)

// IsOrgElevated reports whether an org role carries implicit access to every
// project (owner/admin/member). Viewers and guests are project-scoped.
// Delegates to domain.IsOrgElevatedRole (the canonical definition).
func IsOrgElevated(role domain.Role) bool {
	return domain.IsOrgElevatedRole(role)
}

// EffectiveRoleFor derives the effective project role from the user's org role
// and their stored project-membership role, without a database lookup. Used
// when both roles are already known (e.g. listing members).
func EffectiveRoleFor(orgRole, projectRole domain.Role) domain.Role {
	if IsOrgElevated(orgRole) {
		return orgRole
	}
	return projectRole
}

// ResolveEffectiveRole returns the effective role the authenticated user has
// on projectID. See the package doc for the resolution rules.
//
// For elevated org roles (owner/admin/member), it verifies the project
// belongs to the caller's org before returning the org role. This prevents
// cross-org data leaks where an admin of org A could access org B's projects
// by UUID.
func ResolveEffectiveRole(
	ctx context.Context,
	pmRepo port.ProjectMemberRepository,
	projRepo port.ProjectRepository,
	projectID string,
) (domain.Role, error) {
	roleStr, ok := transport.RoleFromContext(ctx)
	if !ok {
		return "", ErrForbidden
	}
	orgRole := domain.Role(roleStr)

	orgID, ok := transport.OrgIDFromContext(ctx)
	if !ok {
		return "", ErrForbidden
	}

	if IsOrgElevated(orgRole) {
		if projRepo == nil {
			return "", ErrForbidden
		}
		if _, err := projRepo.GetByID(ctx, orgID, projectID); err != nil {
			return "", ErrForbidden
		}
		return orgRole, nil
	}

	userID, ok := transport.UserIDFromContext(ctx)
	if !ok {
		return "", ErrForbidden
	}

	if pmRepo == nil {
		return "", ErrNotProjectMember
	}

	pm, err := pmRepo.Get(ctx, orgID, projectID, userID)
	if err != nil {
		return "", ErrNotProjectMember
	}
	if pm == nil || pm.Role == "" {
		return "", ErrNotProjectMember
	}
	return pm.Role, nil
}
