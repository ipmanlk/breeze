package handler

import (
	"context"
	"log/slog"

	"ipmanlk/plume/internal/domain"
	"ipmanlk/plume/internal/port"
	"ipmanlk/plume/internal/transport/ws"
)

// wsRoomAccessChecker implements ws.RoomAccessChecker. It authorizes
// client-driven WS room subscriptions by consulting the same access layers
// the HTTP handlers use: channel permissions for conversations and the
// project effective-role resolver for projects. All checks fail closed
// (return false) on any error so a transient failure can never grant access.
type wsRoomAccessChecker struct {
	permSvc  port.ChannelPermissionService
	convRepo port.ConversationRepository
	pmRepo   port.ProjectMemberRepository
	projRepo port.ProjectRepository
	log      *slog.Logger
}

var _ ws.RoomAccessChecker = (*wsRoomAccessChecker)(nil)

// NewWSRoomAccessChecker builds the access checker used to authorize WS
// room subscriptions.
func NewWSRoomAccessChecker(
	permSvc port.ChannelPermissionService,
	convRepo port.ConversationRepository,
	pmRepo port.ProjectMemberRepository,
	projRepo port.ProjectRepository,
	log *slog.Logger,
) ws.RoomAccessChecker {
	return &wsRoomAccessChecker{
		permSvc:  permSvc,
		convRepo: convRepo,
		pmRepo:   pmRepo,
		projRepo: projRepo,
		log:      log,
	}
}

func (c *wsRoomAccessChecker) CanAccessConversation(ctx context.Context, orgID, conversationID, userID string, orgRole domain.Role) bool {
	if c.permSvc != nil {
		ok, err := c.permSvc.UserHasAccess(ctx, orgID, conversationID, userID, orgRole)
		if err != nil {
			c.log.Warn("ws access: conversation check", "error", err, "conversation_id", conversationID, "user_id", userID)
			return false
		}
		return ok
	}
	// No channel-permission service: fall back to explicit membership. This is
	// the pre-existing behavior for DMs/groups (which have no role rules) and
	// is still correct for channels when the perm service is unavailable.
	if c.convRepo != nil {
		ok, err := c.convRepo.IsMember(ctx, conversationID, userID)
		if err != nil {
			c.log.Warn("ws access: membership check", "error", err, "conversation_id", conversationID, "user_id", userID)
			return false
		}
		return ok
	}
	return false
}

func (c *wsRoomAccessChecker) CanAccessProject(ctx context.Context, orgID, projectID, userID string, orgRole domain.Role) bool {
	if c.projRepo == nil || orgID == "" || userID == "" {
		return false
	}
	// The WS client invokes this checker with a background context (it has no
	// HTTP request to derive context values from), so we cannot reuse
	// access.ResolveEffectiveRole; it reads userID/orgID/role from the
	// context and would always deny. Instead we mirror its logic using the
	// explicit args the client passes.
	//
	// Elevated org roles (owner/admin/member) have implicit access to every
	// project in their org; we just verify the project belongs to orgID so an
	// admin of org A cannot subscribe to org B's project room (the room key
	// already embeds orgID, but this is defense in depth). Project-scoped
	// roles (viewer/guest) must have an explicit project_members row.
	if domain.IsOrgElevatedRole(orgRole) {
		if _, err := c.projRepo.GetByID(ctx, orgID, projectID); err != nil {
			return false
		}
		return true
	}
	if c.pmRepo == nil {
		return false
	}
	pm, err := c.pmRepo.Get(ctx, orgID, projectID, userID)
	if err != nil || pm == nil || pm.Role == "" {
		return false
	}
	return true
}

func (c *wsRoomAccessChecker) CanSendInConversation(ctx context.Context, orgID, conversationID, userID string, orgRole domain.Role) bool {
	if c.permSvc != nil {
		perms, err := c.permSvc.ResolvePermissions(ctx, orgID, conversationID, userID, orgRole)
		if err != nil {
			c.log.Warn("ws access: send check", "error", err, "conversation_id", conversationID, "user_id", userID)
			return false
		}
		return perms.CanSend
	}
	// Fallback: explicit members can send.
	if c.convRepo != nil {
		ok, err := c.convRepo.IsMember(ctx, conversationID, userID)
		if err != nil {
			return false
		}
		return ok
	}
	return false
}
