package service

import (
	"context"

	"ipmanlk/plume/internal/apperr"
	"ipmanlk/plume/internal/domain"
	"ipmanlk/plume/internal/port"
)

// accessCheckerImpl implements port.AccessChecker by resolving the effective
// role from the database, providing defense-in-depth authorization for
// service-level operations independently of HTTP middleware.
//
// IMPORTANT: nil-guards fail CLOSED (deny): if the checker or any required
// dependency repo is nil, ErrForbidden is returned. This ensures that a
// mis-wired constructor never silently opens access to everyone.
type accessCheckerImpl struct {
	projRepo port.ProjectRepository
	userRepo port.UserRepository
	pmRepo   port.ProjectMemberRepository
	taskRepo port.TaskRepository
}

// NewAccessChecker creates a new AccessChecker implementation.
func NewAccessChecker(projRepo port.ProjectRepository, userRepo port.UserRepository, pmRepo port.ProjectMemberRepository, taskRepo port.TaskRepository) port.AccessChecker {
	return &accessCheckerImpl{
		projRepo: projRepo,
		userRepo: userRepo,
		pmRepo:   pmRepo,
		taskRepo: taskRepo,
	}
}

func (a *accessCheckerImpl) RequireProjectAccess(ctx context.Context, userID, orgID, projectID string, perm domain.Permission) error {
	if a == nil || a.projRepo == nil || a.userRepo == nil {
		return apperr.ErrForbidden
	}

	// Verify the project exists and belongs to the caller's org.
	if _, err := a.projRepo.GetByID(ctx, orgID, projectID); err != nil {
		return apperr.ErrForbidden
	}

	// Look up the caller's org role.
	u, err := a.userRepo.GetByID(ctx, orgID, userID)
	if err != nil {
		return apperr.ErrForbidden
	}

	// Elevated roles (owner/admin/member) have org-wide access.
	if domain.IsOrgElevatedRole(u.Role) {
		return nil
	}

	// Viewers and guests need a project_members entry.
	if a.pmRepo == nil {
		return apperr.ErrForbidden
	}
	pm, err := a.pmRepo.Get(ctx, orgID, projectID, userID)
	if err != nil || pm == nil || pm.Role == "" {
		return apperr.ErrForbidden
	}
	if !domain.HasPermission(pm.Role, perm) {
		return apperr.ErrForbidden
	}
	return nil
}

func (a *accessCheckerImpl) RequireTaskAccess(ctx context.Context, userID, orgID, taskID string, perm domain.Permission) error {
	if a == nil || a.taskRepo == nil {
		return apperr.ErrForbidden
	}
	task, err := a.taskRepo.GetByIDAndOrg(ctx, orgID, taskID)
	if err != nil {
		return apperr.ErrForbidden
	}
	return a.RequireProjectAccess(ctx, userID, orgID, task.ProjectID, perm)
}

func (a *accessCheckerImpl) RequireOrgAccess(ctx context.Context, userID, orgID string, perm domain.Permission) error {
	if a == nil || a.userRepo == nil {
		return apperr.ErrForbidden
	}
	u, err := a.userRepo.GetByID(ctx, orgID, userID)
	if err != nil {
		return apperr.ErrForbidden
	}
	// No elevated-role shortcut here: RolePermissions for owner/admin are
	// supersets of every org permission, so HasPermission alone decides.
	// A shortcut on IsOrgElevatedRole would wrongly grant `member` every
	// org permission, including future org:* ones.
	if !domain.HasPermission(u.Role, perm) {
		return apperr.ErrForbidden
	}
	return nil
}
