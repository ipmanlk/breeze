package service

import (
	"context"
	"fmt"

	"ipmanlk/plume/internal/domain"
	"ipmanlk/plume/internal/port"
)

type ChannelPermissionService struct {
	permRepo port.ChannelPermissionRepository
	convRepo port.ConversationRepository
	linkRepo port.ChannelProjectLinkRepository
	userRepo port.UserRepository
}

var _ port.ChannelPermissionService = (*ChannelPermissionService)(nil)

func NewChannelPermissionService(
	permRepo port.ChannelPermissionRepository,
	convRepo port.ConversationRepository,
	linkRepo port.ChannelProjectLinkRepository,
	userRepo port.UserRepository,
) *ChannelPermissionService {
	return &ChannelPermissionService{
		permRepo: permRepo,
		convRepo: convRepo,
		linkRepo: linkRepo,
		userRepo: userRepo,
	}
}

func (s *ChannelPermissionService) ResolvePermissions(ctx context.Context, orgID, channelID, userID string, userRole domain.Role) (*domain.ChannelPermissions, error) {
	conv, err := s.convRepo.GetByID(ctx, orgID, channelID)
	if err != nil {
		return nil, fmt.Errorf("get conversation: %w", err)
	}

	result := &domain.ChannelPermissions{}

	// Org owners and admins are immune to channel-level restrictions: they
	// always hold every channel permission regardless of rules or overrides.
	if userRole == domain.RoleOwner || userRole == domain.RoleAdmin {
		result.CanView = true
		result.CanSend = true
		result.CanManage = true
		result.CanPermissions = true
		return result, nil
	}

	perms := []domain.Permission{
		domain.PermChannelView,
		domain.PermChannelSend,
		domain.PermChannelManage,
		domain.PermChannelPermissions,
	}

	for _, perm := range perms {
		allowed := s.resolveSinglePermission(ctx, conv, userID, userRole, perm)
		switch perm {
		case domain.PermChannelView:
			result.CanView = allowed
		case domain.PermChannelSend:
			result.CanSend = allowed
		case domain.PermChannelManage:
			result.CanManage = allowed
		case domain.PermChannelPermissions:
			result.CanPermissions = allowed
		}
	}

	return result, nil
}

func (s *ChannelPermissionService) ResolveRolePermissions(ctx context.Context, orgID, channelID string) ([]*domain.EffectivePermission, error) {
	conv, err := s.convRepo.GetByID(ctx, orgID, channelID)
	if err != nil {
		return nil, fmt.Errorf("get conversation: %w", err)
	}

	roles := []domain.Role{domain.RoleEveryone, domain.RoleMember, domain.RoleViewer, domain.RoleGuest}
	perms := []domain.Permission{
		domain.PermChannelView,
		domain.PermChannelSend,
		domain.PermChannelManage,
		domain.PermChannelPermissions,
	}

	// Build a lookup of explicit rules set at THIS channel level.
	explicitRules, err := s.permRepo.GetPermissions(ctx, channelID)
	if err != nil {
		return nil, err
	}
	explicitMap := make(map[string]*domain.PermissionRule)
	for _, r := range explicitRules {
		key := string(r.Role) + ":" + string(r.Permission)
		explicitMap[key] = r
	}

	var result []*domain.EffectivePermission

	for _, role := range roles {
		for _, perm := range perms {
			key := string(role) + ":" + string(perm)
			explicitRule := explicitMap[key]

			// The inherited value is what the permission would resolve to if
			// there were NO explicit rule at this channel level. We compute it
			// by walking the parent chain (excluding this channel's own rules).
			inherited := s.resolveRoleEffective(ctx, conv, role, perm, false)

			effective := inherited
			if explicitRule != nil {
				effective = explicitRule.Allow
			}

			result = append(result, &domain.EffectivePermission{
				Role:       role,
				Permission: perm,
				Allow:      effective,
				Explicit:   explicitRule != nil,
			})
		}
	}

	return result, nil
}

// resolveRoleEffective walks the parent chain and returns the effective
// permission for the given role+perm, WITHOUT considering rules at the current
// conv level when checkSelf is false. This is used to compute the "inherited"
// value that would apply if no rule existed at the current level.
func (s *ChannelPermissionService) resolveRoleEffective(ctx context.Context, conv *domain.Conversation, role domain.Role, perm domain.Permission, checkSelf bool) bool {
	// Owners and admins are immune to channel-level rules.
	if role == domain.RoleOwner || role == domain.RoleAdmin {
		return true
	}

	current := conv
	first := true
	for current != nil {
		if !first || checkSelf {
			rules, err := s.permRepo.GetPermissions(ctx, current.ID)
			if err == nil {
				// Role-specific rule (highest priority at this level)
				for _, rule := range rules {
					if rule.Role == role && rule.Permission == perm {
						return rule.Allow
					}
				}
				// "everyone" fallback at this level. Admins never reach this
				// loop (early return above), so no admin exemption is needed.
				for _, rule := range rules {
					if rule.Role == domain.RoleEveryone && rule.Permission == perm {
						return rule.Allow
					}
				}
			}
		}
		first = false

		if current.ParentID == nil {
			break
		}
		parent, err := s.convRepo.GetByID(ctx, current.OrgID, *current.ParentID)
		if err != nil {
			break
		}
		current = parent
	}

	return s.fallbackPermission(role, perm)
}

func (s *ChannelPermissionService) resolveSinglePermission(ctx context.Context, conv *domain.Conversation, userID string, userRole domain.Role, perm domain.Permission) bool {
	// Owners and admins are immune to all channel-level restrictions.
	if userRole == domain.RoleOwner || userRole == domain.RoleAdmin {
		return true
	}

	// Walk up the parent chain (conv → parent → grandparent → ...), checking
	// per-user overrides first, then role-specific rules, then "everyone" rules.
	current := conv
	for current != nil {
		// Per-user override (highest priority)
		userOverrides, err := s.permRepo.GetUserOverrides(ctx, current.ID)
		if err == nil {
			for _, o := range userOverrides {
				if o.UserID == userID && o.Permission == perm {
					return o.Allow
				}
			}
		}

		// Role-specific rules
		rules, err := s.permRepo.GetPermissions(ctx, current.ID)
		if err == nil {
			for _, rule := range rules {
				if rule.Role == userRole && rule.Permission == perm {
					return rule.Allow
				}
			}
			// "everyone" rules. Admins never reach this loop (early return
			// above), so no admin exemption is needed here.
			for _, rule := range rules {
				if rule.Role == domain.RoleEveryone && rule.Permission == perm {
					return rule.Allow
				}
			}
		}

		// Move to parent, or stop if no parent
		if current.ParentID == nil {
			break
		}
		parent, err := s.convRepo.GetByID(ctx, current.OrgID, *current.ParentID)
		if err != nil {
			break
		}
		current = parent
	}

	// Fallback default based on org role
	return s.fallbackPermission(userRole, perm)
}

func (s *ChannelPermissionService) fallbackPermission(role domain.Role, perm domain.Permission) bool {
	switch role {
	case domain.RoleOwner, domain.RoleAdmin:
		return true
	case domain.RoleMember:
		switch perm {
		case domain.PermChannelView, domain.PermChannelSend:
			return true
		default:
			return false
		}
	case domain.RoleViewer:
		return perm == domain.PermChannelView
	case domain.RoleGuest:
		return false
	default:
		return false
	}
}

func (s *ChannelPermissionService) UserHasAccess(ctx context.Context, orgID, channelID, userID string, userRole domain.Role) (bool, error) {
	// Verify the channel exists in the caller's org before anything else.
	// Every downstream lookup keys on raw IDs, so this is the one place
	// cross-org access is stopped. Unknown ID = no access (fail closed).
	conv, err := s.convRepo.GetByID(ctx, orgID, channelID)
	if err != nil {
		return false, nil
	}

	// Owners and admins have full access to every channel in their org.
	if userRole == domain.RoleOwner || userRole == domain.RoleAdmin {
		return true, nil
	}

	isMember, err := s.convRepo.IsMember(ctx, channelID, userID)
	if err != nil {
		return false, fmt.Errorf("check membership: %w", err)
	}
	if isMember {
		perms, err := s.ResolvePermissions(ctx, orgID, channelID, userID, userRole)
		if err != nil {
			return false, err
		}
		return perms.CanView, nil
	}

	// Non-members reach a channel only through a linked project; and even
	// then, channel:view rules and per-user overrides still apply so an
	// explicit deny cannot be bypassed via a project link. Viewers and guests
	// are hard-denied here: they only see channels they were explicitly added to.
	projectIDs, err := s.collectProjectIDs(ctx, conv)
	if err != nil {
		return false, fmt.Errorf("get project links: %w", err)
	}

	if userRole == domain.RoleMember && len(projectIDs) > 0 {
		perms, err := s.ResolvePermissions(ctx, orgID, channelID, userID, userRole)
		if err != nil {
			return false, err
		}
		return perms.CanView, nil
	}

	return false, nil
}

// collectProjectIDs gathers project IDs from the channel and its parent chain.
// The conversation must already be verified to belong to the caller's org;
// parent lookups are org-scoped and any error walking the chain fails closed.
func (s *ChannelPermissionService) collectProjectIDs(ctx context.Context, conv *domain.Conversation) ([]string, error) {
	ids, err := s.linkRepo.GetByChannel(ctx, conv.ID)
	if err != nil {
		return nil, err
	}

	current := conv
	seen := make(map[string]bool)
	for _, id := range ids {
		seen[id] = true
	}

	for current != nil && current.ParentID != nil {
		parentIDs, err := s.linkRepo.GetByChannel(ctx, *current.ParentID)
		if err != nil {
			break
		}
		for _, id := range parentIDs {
			if !seen[id] {
				ids = append(ids, id)
				seen[id] = true
			}
		}

		parent, err := s.convRepo.GetByID(ctx, current.OrgID, *current.ParentID)
		if err != nil {
			break
		}
		current = parent
	}

	return ids, nil
}

func (s *ChannelPermissionService) GetPermissions(ctx context.Context, channelID string) ([]*domain.PermissionRule, error) {
	return s.permRepo.GetPermissions(ctx, channelID)
}

func (s *ChannelPermissionService) SetPermissions(ctx context.Context, channelID string, rules []*domain.PermissionRule) error {
	return s.permRepo.SetPermissions(ctx, channelID, rules)
}

func (s *ChannelPermissionService) GetUserOverrides(ctx context.Context, channelID string) ([]*domain.UserPermissionOverride, error) {
	return s.permRepo.GetUserOverrides(ctx, channelID)
}

func (s *ChannelPermissionService) SetUserOverrides(ctx context.Context, channelID string, overrides []*domain.UserPermissionOverride) error {
	return s.permRepo.SetUserOverrides(ctx, channelID, overrides)
}

func (s *ChannelPermissionService) GetUsersWithProjectAccess(ctx context.Context, orgID, projectID string) ([]*domain.User, error) {
	return s.linkRepo.GetUsersWithProjectAccess(ctx, orgID, projectID)
}

// ListAccess returns all users who have access to the channel (explicit members + project-linked users)
func (s *ChannelPermissionService) ListAccess(ctx context.Context, orgID, channelID string) ([]*domain.ChannelAccessEntry, error) {
	members, err := s.convRepo.GetMembers(ctx, channelID)
	if err != nil {
		return nil, err
	}

	conv, err := s.convRepo.GetByID(ctx, orgID, channelID)
	if err != nil {
		return nil, err
	}

	seen := make(map[string]bool)
	entries := make([]*domain.ChannelAccessEntry, 0, len(members))

	for _, m := range members {
		user, err := s.userRepo.GetByID(ctx, conv.OrgID, m.UserID)
		if err != nil {
			continue
		}
		entries = append(entries, &domain.ChannelAccessEntry{
			User:   user,
			Source: "explicit",
		})
		seen[user.ID] = true
	}

	projectIDs, err := s.collectProjectIDs(ctx, conv)
	if err != nil {
		return entries, nil
	}

	for _, pid := range projectIDs {
		users, err := s.linkRepo.GetUsersWithProjectAccess(ctx, conv.OrgID, pid)
		if err != nil {
			continue
		}
		for _, user := range users {
			if !seen[user.ID] {
				entries = append(entries, &domain.ChannelAccessEntry{
					User:   user,
					Source: "project",
				})
				seen[user.ID] = true
			}
		}
	}

	return entries, nil
}
