package port

import (
	"context"

	"ipmanlk/breeze/internal/domain"
)

// AccessService performs authorization checks that were previously scattered
// across handler-injected repositories and guard.go. Handlers depend on this
// instead of directly receiving repository interfaces.
//
// Every method accepts identity explicitly (orgID, userID, role) so the
// implementation stays in the service layer without importing transport.
type AccessService interface {
	// EnsureProjectAccess returns nil if the caller may access the project.
	// When the effective role is already cached in context (set by
	// RequireProjectPermission middleware), this is a cheap lookup.
	// Otherwise it resolves membership and verifies org ownership.
	EnsureProjectAccess(ctx context.Context, orgID, userID string, role domain.Role, projectID string) error

	// ResolveProjectEffectiveRole returns the caller's effective role for a
	// project, resolving org-role vs project-membership.
	ResolveProjectEffectiveRole(ctx context.Context, orgID, userID string, role domain.Role, projectID string) (domain.Role, error)

	// EnsureConversationAccess returns nil if the caller may view the
	// conversation (honoring channel permission rules + project-link gating).
	EnsureConversationAccess(ctx context.Context, orgID, userID string, role domain.Role, convID string) error

	// EnsureConversationSendAccess returns nil if the caller may send in the
	// conversation (honoring channel:send overrides).
	EnsureConversationSendAccess(ctx context.Context, orgID, userID string, role domain.Role, convID string) error

	// EnsureConversationManageAccess returns nil if the caller has
	// channel:manage in the conversation.
	EnsureConversationManageAccess(ctx context.Context, orgID, userID string, role domain.Role, convID string) error
}
