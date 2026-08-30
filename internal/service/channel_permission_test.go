package service

import (
	"context"
	"testing"

	"ipmanlk/breeze/internal/domain"
)

func TestChannelPermissionService_ResolvePermissions_ChannelOverrideTakesPriority(t *testing.T) {
	ctx := context.Background()
	convRepo := newMockConversationRepo()
	convRepo.Create(ctx, &domain.Conversation{ID: "ch-1", OrgID: "org-1", Name: "general", Type: domain.ConvChannel})
	permRepo := newMockPermRepo()
	permRepo.perms["cat-1"] = []*domain.PermissionRule{
		{Role: domain.RoleMember, Permission: domain.PermChannelView, Allow: true},
	}
	permRepo.perms["ch-1"] = []*domain.PermissionRule{
		{Role: domain.RoleMember, Permission: domain.PermChannelView, Allow: false},
	}

	svc := NewChannelPermissionService(permRepo, convRepo, newMockLinkRepo(), newMockUserRepo())
	perms, err := svc.ResolvePermissions(ctx, "org-1", "ch-1", "user-1", domain.RoleMember)
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if perms.CanView {
		t.Error("expected CanView=false (channel override denies), got true")
	}
}

func TestChannelPermissionService_ResolvePermissions_ParentDefaultFallback(t *testing.T) {
	ctx := context.Background()
	parentID := "cat-1"
	convRepo := newMockConversationRepo()
	convRepo.Create(ctx, &domain.Conversation{
		ID: "cat-1", OrgID: "org-1", Name: "General", Type: domain.ConvCategory,
	})
	convRepo.Create(ctx, &domain.Conversation{
		ID: "ch-1", OrgID: "org-1", Name: "general", Type: domain.ConvChannel, ParentID: &parentID,
	})
	permRepo := newMockPermRepo()
	permRepo.perms["cat-1"] = []*domain.PermissionRule{
		{Role: domain.RoleMember, Permission: domain.PermChannelView, Allow: true},
		{Role: domain.RoleMember, Permission: domain.PermChannelSend, Allow: false},
	}

	svc := NewChannelPermissionService(permRepo, convRepo, newMockLinkRepo(), newMockUserRepo())
	perms, err := svc.ResolvePermissions(ctx, "org-1", "ch-1", "user-1", domain.RoleMember)
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if !perms.CanView {
		t.Error("expected CanView=true (parent default), got false")
	}
	if perms.CanSend {
		t.Error("expected CanSend=false (parent default denies), got true")
	}
}

func TestChannelPermissionService_ResolvePermissions_FallbackDefaultByRole(t *testing.T) {
	ctx := context.Background()
	convRepo := newMockConversationRepo()
	convRepo.Create(ctx, &domain.Conversation{ID: "ch-1", OrgID: "org-1", Name: "general", Type: domain.ConvChannel})
	permRepo := newMockPermRepo()

	svc := NewChannelPermissionService(permRepo, convRepo, newMockLinkRepo(), newMockUserRepo())

	t.Run("owner gets all", func(t *testing.T) {
		perms, err := svc.ResolvePermissions(ctx, "org-1", "ch-1", "user-1", domain.RoleOwner)
		if err != nil {
			t.Fatalf("unexpected error: %v", err)
		}
		if !perms.CanView || !perms.CanSend || !perms.CanManage || !perms.CanPermissions {
			t.Error("expected owner to have all permissions")
		}
	})

	t.Run("member gets view+send", func(t *testing.T) {
		perms, err := svc.ResolvePermissions(ctx, "org-1", "ch-1", "user-1", domain.RoleMember)
		if err != nil {
			t.Fatalf("unexpected error: %v", err)
		}
		if !perms.CanView {
			t.Error("expected member CanView=true")
		}
		if !perms.CanSend {
			t.Error("expected member CanSend=true")
		}
		if perms.CanManage {
			t.Error("expected member CanManage=false")
		}
		if perms.CanPermissions {
			t.Error("expected member CanPermissions=false")
		}
	})

	t.Run("viewer gets view only", func(t *testing.T) {
		perms, err := svc.ResolvePermissions(ctx, "org-1", "ch-1", "user-1", domain.RoleViewer)
		if err != nil {
			t.Fatalf("unexpected error: %v", err)
		}
		if !perms.CanView {
			t.Error("expected viewer CanView=true")
		}
		if perms.CanSend {
			t.Error("expected viewer CanSend=false")
		}
	})
}

func TestChannelPermissionService_UserHasAccess_ExplicitMember(t *testing.T) {
	ctx := context.Background()
	convRepo := newMockConversationRepo()
	convRepo.Create(ctx, &domain.Conversation{ID: "ch-1", OrgID: "org-1", Name: "general", Type: domain.ConvChannel})
	convRepo.AddMember(ctx, "org-1", "ch-1", "user-1")
	permRepo := newMockPermRepo()

	svc := NewChannelPermissionService(permRepo, convRepo, newMockLinkRepo(), newMockUserRepo())
	hasAccess, err := svc.UserHasAccess(ctx, "org-1", "ch-1", "user-1", domain.RoleMember)
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if !hasAccess {
		t.Error("expected explicit member to have access")
	}
}

func TestChannelPermissionService_UserHasAccess_ExplicitMemberDeniedByOverride(t *testing.T) {
	ctx := context.Background()
	convRepo := newMockConversationRepo()
	convRepo.Create(ctx, &domain.Conversation{ID: "ch-1", OrgID: "org-1", Name: "general", Type: domain.ConvChannel})
	convRepo.AddMember(ctx, "org-1", "ch-1", "user-1")
	permRepo := newMockPermRepo()
	permRepo.perms["ch-1"] = []*domain.PermissionRule{
		{Role: domain.RoleMember, Permission: domain.PermChannelView, Allow: false},
	}

	svc := NewChannelPermissionService(permRepo, convRepo, newMockLinkRepo(), newMockUserRepo())
	hasAccess, err := svc.UserHasAccess(ctx, "org-1", "ch-1", "user-1", domain.RoleMember)
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if hasAccess {
		t.Error("expected member to be denied by channel override")
	}
}

func TestChannelPermissionService_UserHasAccess_ProjectLink(t *testing.T) {
	ctx := context.Background()
	convRepo := newMockConversationRepo()
	convRepo.Create(ctx, &domain.Conversation{ID: "ch-1", OrgID: "org-1", Name: "general", Type: domain.ConvChannel})
	linkRepo := newMockLinkRepo()
	linkRepo.Create(ctx, "ch-1", "proj-1")

	svc := NewChannelPermissionService(newMockPermRepo(), convRepo, linkRepo, newMockUserRepo())
	hasAccess, err := svc.UserHasAccess(ctx, "org-1", "ch-1", "user-1", domain.RoleMember)
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if !hasAccess {
		t.Error("expected member to have access via project link")
	}
}

func TestChannelPermissionService_UserHasAccess_AdminBypassesChannelRules(t *testing.T) {
	ctx := context.Background()
	convRepo := newMockConversationRepo()
	convRepo.Create(ctx, &domain.Conversation{ID: "ch-1", OrgID: "org-1", Name: "general", Type: domain.ConvChannel})
	permRepo := newMockPermRepo()
	permRepo.perms["ch-1"] = []*domain.PermissionRule{
		{Role: domain.RoleAdmin, Permission: domain.PermChannelView, Allow: false},
	}

	svc := NewChannelPermissionService(permRepo, convRepo, newMockLinkRepo(), newMockUserRepo())
	for _, role := range []domain.Role{domain.RoleOwner, domain.RoleAdmin} {
		hasAccess, err := svc.UserHasAccess(ctx, "org-1", "ch-1", "user-1", role)
		if err != nil {
			t.Fatalf("%s: unexpected error: %v", role, err)
		}
		if !hasAccess {
			t.Errorf("expected %s to access any channel in their org (elevated-role immunity)", role)
		}
	}
}

func TestChannelPermissionService_UserHasAccess_ProjectLinkDeniedByRule(t *testing.T) {
	ctx := context.Background()
	convRepo := newMockConversationRepo()
	convRepo.Create(ctx, &domain.Conversation{ID: "ch-1", OrgID: "org-1", Name: "general", Type: domain.ConvChannel})
	linkRepo := newMockLinkRepo()
	linkRepo.Create(ctx, "ch-1", "proj-1")
	permRepo := newMockPermRepo()
	// A per-user deny must not be bypassed just because the caller can reach
	// the channel through a linked project.
	permRepo.overrides["ch-1"] = []*domain.UserPermissionOverride{
		{ChannelID: "ch-1", UserID: "user-1", Permission: domain.PermChannelView, Allow: false},
	}

	svc := NewChannelPermissionService(permRepo, convRepo, linkRepo, newMockUserRepo())
	hasAccess, err := svc.UserHasAccess(ctx, "org-1", "ch-1", "user-1", domain.RoleMember)
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if hasAccess {
		t.Error("expected project-linked member to be denied by explicit view override")
	}
}

func TestChannelPermissionService_UserHasAccess_CrossOrgDenied(t *testing.T) {
	ctx := context.Background()
	convRepo := newMockConversationRepo()
	convRepo.Create(ctx, &domain.Conversation{ID: "ch-2", OrgID: "org-2", Name: "foreign", Type: domain.ConvChannel})
	linkRepo := newMockLinkRepo()
	linkRepo.Create(ctx, "ch-2", "proj-foreign")

	svc := NewChannelPermissionService(newMockPermRepo(), convRepo, linkRepo, newMockUserRepo())

	t.Run("member of another org is denied", func(t *testing.T) {
		hasAccess, err := svc.UserHasAccess(ctx, "org-1", "ch-2", "user-1", domain.RoleMember)
		if err != nil {
			t.Fatalf("unexpected error: %v", err)
		}
		if hasAccess {
			t.Error("expected cross-org channel access to be denied")
		}
	})

	t.Run("even an elevated foreign role is denied", func(t *testing.T) {
		hasAccess, err := svc.UserHasAccess(ctx, "org-1", "ch-2", "user-1", domain.RoleAdmin)
		if err != nil {
			t.Fatalf("unexpected error: %v", err)
		}
		if hasAccess {
			t.Error("elevated-role immunity must stop at the org boundary")
		}
	})
}

func TestChannelPermissionService_ResolvePermissions_AdminImmuneToDenyRules(t *testing.T) {
	ctx := context.Background()
	convRepo := newMockConversationRepo()
	convRepo.Create(ctx, &domain.Conversation{ID: "ch-1", OrgID: "org-1", Name: "general", Type: domain.ConvChannel})
	permRepo := newMockPermRepo()
	permRepo.perms["ch-1"] = []*domain.PermissionRule{
		{Role: domain.RoleEveryone, Permission: domain.PermChannelView, Allow: false},
	}

	svc := NewChannelPermissionService(permRepo, convRepo, newMockLinkRepo(), newMockUserRepo())
	perms, err := svc.ResolvePermissions(ctx, "org-1", "ch-1", "user-1", domain.RoleAdmin)
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if !perms.CanManage || !perms.CanPermissions || !perms.CanSend || !perms.CanView {
		t.Error("expected admin to hold all channel permissions despite everyone-deny rules")
	}
}

func TestChannelPermissionService_UserHasAccess_NoAccess(t *testing.T) {
	ctx := context.Background()
	convRepo := newMockConversationRepo()
	convRepo.Create(ctx, &domain.Conversation{ID: "ch-1", OrgID: "org-1", Name: "general", Type: domain.ConvChannel})

	svc := NewChannelPermissionService(newMockPermRepo(), convRepo, newMockLinkRepo(), newMockUserRepo())
	hasAccess, err := svc.UserHasAccess(ctx, "org-1", "ch-1", "user-1", domain.RoleViewer)
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if hasAccess {
		t.Error("expected viewer to have no access (no membership, no project links)")
	}
}

func TestChannelPermissionService_GetSetPermissions(t *testing.T) {
	ctx := context.Background()
	permRepo := newMockPermRepo()
	svc := NewChannelPermissionService(permRepo, newMockConversationRepo(), newMockLinkRepo(), newMockUserRepo())

	rules := []*domain.PermissionRule{
		{Role: domain.RoleMember, Permission: domain.PermChannelView, Allow: true},
	}
	if err := svc.SetPermissions(ctx, "ch-1", rules); err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	got, err := svc.GetPermissions(ctx, "ch-1")
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if len(got) != 1 {
		t.Fatalf("expected 1 rule, got %d", len(got))
	}
	if got[0].Permission != domain.PermChannelView {
		t.Errorf("expected permission channel:view, got %s", got[0].Permission)
	}
}

func TestChannelPermissionService_GetUsersWithProjectAccess(t *testing.T) {
	ctx := context.Background()
	userRepo := newMockUserRepo()
	userRepo.usersByID["user-1"] = &domain.User{ID: "user-1", Name: "Alice", Email: "alice@test.com", OrgID: "org-1", Role: domain.RoleOwner}

	linkRepo := newMockLinkRepo()
	linkRepo.users["proj-1"] = []*domain.User{{ID: "user-1", Name: "Alice", Email: "alice@test.com", OrgID: "org-1", Role: domain.RoleOwner}}

	svc := NewChannelPermissionService(newMockPermRepo(), newMockConversationRepo(), linkRepo, userRepo)
	users, err := svc.GetUsersWithProjectAccess(ctx, "org-1", "proj-1")
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if len(users) != 1 {
		t.Fatalf("expected 1 user, got %d", len(users))
	}
	if users[0].ID != "user-1" {
		t.Errorf("expected user-1, got %s", users[0].ID)
	}
}
