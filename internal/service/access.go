package service

import (
	"context"

	"ipmanlk/breeze/internal/apperr"
	"ipmanlk/breeze/internal/domain"
	"ipmanlk/breeze/internal/port"
)

// accessService implements port.AccessService by wrapping the same repository
// lookups that the old guard.go functions and transport/access helpers used.
// Identity (orgID, userID, role) is passed explicitly so the service layer
// stays free of transport imports.
type accessService struct {
	pmRepo   port.ProjectMemberRepository
	projRepo port.ProjectRepository
	permSvc  port.ChannelPermissionService
	convRepo port.ConversationRepository
}

// NewAccessService creates a new AccessService. Repositories and the optional
// ChannelPermissionService are injected at construction time; see app.go for
// the wiring.
func NewAccessService(
	pmRepo port.ProjectMemberRepository,
	projRepo port.ProjectRepository,
	permSvc port.ChannelPermissionService,
	convRepo port.ConversationRepository,
) port.AccessService {
	return &accessService{
		pmRepo:   pmRepo,
		projRepo: projRepo,
		permSvc:  permSvc,
		convRepo: convRepo,
	}
}

// ---------------------------------------------------------------------------
// Project access
// ---------------------------------------------------------------------------

func (a *accessService) EnsureProjectAccess(
	ctx context.Context, orgID, userID string, role domain.Role, projectID string,
) error {
	// Fast path: only when the middleware already resolved the role for THIS
	// exact project. Handlers may call this for a second, body-supplied
	// project ID (e.g. move-to-project), which must never be satisfied by a
	// role cached for the URL-param project.
	if cachedProject, ok := a.cachedEffectiveRoleProject(ctx); ok && cachedProject == projectID {
		return nil
	}
	_, err := a.resolveProjectRole(ctx, orgID, userID, role, projectID)
	return err
}

func (a *accessService) ResolveProjectEffectiveRole(
	ctx context.Context, orgID, userID string, role domain.Role, projectID string,
) (domain.Role, error) {
	return a.resolveProjectRole(ctx, orgID, userID, role, projectID)
}

// resolveProjectRole is the shared body of the two public methods above.
// It replicates the logic of internal/transport/access.ResolveEffectiveRole
// without importing transport.
func (a *accessService) resolveProjectRole(
	ctx context.Context, orgID, userID string, role domain.Role, projectID string,
) (domain.Role, error) {
	if domain.IsOrgElevatedRole(role) {
		// Elevated roles (owner/admin/member) have org-wide access. Verify
		// the project belongs to the caller's org (cross-org guard).
		if a.projRepo == nil {
			return "", apperr.ErrForbidden
		}
		if _, err := a.projRepo.GetByID(ctx, orgID, projectID); err != nil {
			return "", apperr.ErrForbidden
		}
		return role, nil
	}

	// Viewer/guest: must have a project_members entry.
	if a.pmRepo == nil {
		return "", apperr.ErrNotFound
	}
	pm, err := a.pmRepo.Get(ctx, orgID, projectID, userID)
	if err != nil {
		return "", apperr.ErrNotFound
	}
	if pm == nil || pm.Role == "" {
		return "", apperr.ErrNotFound
	}
	return pm.Role, nil
}

// ---------------------------------------------------------------------------
// Conversation access
// ---------------------------------------------------------------------------

func (a *accessService) EnsureConversationAccess(
	ctx context.Context, orgID, userID string, role domain.Role, convID string,
) error {
	return a.ensureConv(ctx, orgID, userID, role, convID, false, "")
}

func (a *accessService) EnsureConversationSendAccess(
	ctx context.Context, orgID, userID string, role domain.Role, convID string,
) error {
	return a.ensureConv(ctx, orgID, userID, role, convID, true, "send")
}

func (a *accessService) EnsureConversationManageAccess(
	ctx context.Context, orgID, userID string, role domain.Role, convID string,
) error {
	return a.ensureConv(ctx, orgID, userID, role, convID, true, "manage")
}

// ensureConv is the shared body of the conversation access checks.
//
// If usePermissions is true, it resolves channel permissions and checks the
// named permission (send/manage). Otherwise it checks view access via
// UserHasAccess (when a ChannelPermissionService is available) or falls back
// to explicit conversation membership.
func (a *accessService) ensureConv(
	ctx context.Context, orgID, userID string, role domain.Role, convID string,
	usePermissions bool, permName string,
) error {
	if a.permSvc != nil {
		if usePermissions {
			perms, err := a.permSvc.ResolvePermissions(ctx, orgID, convID, userID, role)
			if err != nil {
				return apperr.Forbidden("failed to resolve channel permissions")
			}
			switch permName {
			case "send":
				if !perms.CanSend {
					return apperr.Forbidden("you do not have send permission in this conversation")
				}
			case "manage":
				if !perms.CanManage {
					return apperr.Forbidden("you do not have manage permission in this conversation")
				}
			}
			return nil
		}

		hasAccess, err := a.permSvc.UserHasAccess(ctx, orgID, convID, userID, role)
		if err != nil {
			return apperr.Forbidden("failed to check conversation access")
		}
		if !hasAccess {
			return apperr.Forbidden("you do not have access to this conversation")
		}
		return nil
	}

	// No channel-permission service: fall back to explicit membership.
	if a.convRepo == nil {
		return apperr.Forbidden("no access checker available")
	}
	isMember, err := a.convRepo.IsMember(ctx, convID, userID)
	if err != nil {
		return apperr.Forbidden("failed to check membership")
	}
	if !isMember {
		return apperr.Forbidden("you are not a member of this conversation")
	}
	return nil
}

// ---------------------------------------------------------------------------
// Helpers
// ---------------------------------------------------------------------------

// cachedEffectiveRoleProject returns the project the RequireProjectPermission
// middleware resolved the stashed effective role for. The second return is
// false when no project-scoped middleware ran on this request.
func (a *accessService) cachedEffectiveRoleProject(ctx context.Context) (string, bool) {
	id, ok := ctx.Value(domain.CtxEffectiveRoleProjectID).(string)
	return id, ok && id != ""
}
