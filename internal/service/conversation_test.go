package service

import (
	"context"
	"errors"
	"testing"

	"ipmanlk/plume/internal/domain"
)

func TestConversationService_CreateChannel(t *testing.T) {
	ctx := context.Background()
	convRepo := newMockConversationRepo()
	userRepo := newMockUserRepo()
	msgRepo := newMockMessageRepo()
	notifSvc := newMockNotificationService()
	broadcaster := newMockBroadcaster()

	svc := NewConversationService(ConversationServiceDeps{
		ConvRepo: convRepo,
		UserRepo: userRepo,
		MsgRepo:  msgRepo,
		PrefRepo: newMockUserPrefRepo(),

		LinkRepo:    newMockLinkRepo(),
		NotifSvc:    notifSvc,
		Broadcaster: broadcaster,
	})

	channel, err := svc.CreateChannel(ctx, domain.CreateConversationParams{
		OrgID:     "org-1",
		CreatedBy: "user-1",
		Name:      "general",
		Type:      domain.ConvChannel,
	})
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if channel.Name != "general" {
		t.Errorf("expected name 'general', got '%s'", channel.Name)
	}
	if channel.Type != domain.ConvChannel {
		t.Errorf("expected channel type, got '%s'", channel.Type)
	}
	if channel.OrgID != "org-1" {
		t.Errorf("expected org-1, got '%s'", channel.OrgID)
	}
	isMember, _ := convRepo.IsMember(ctx, channel.ID, "user-1")
	if !isMember {
		t.Error("expected creator to be a member of the channel")
	}
}

func TestConversationService_CreateChannel_EmptyName(t *testing.T) {
	ctx := context.Background()
	svc := NewConversationService(ConversationServiceDeps{
		ConvRepo: newMockConversationRepo(),
		UserRepo: newMockUserRepo(),
		MsgRepo:  newMockMessageRepo(),
		PrefRepo: newMockUserPrefRepo(),

		LinkRepo:    newMockLinkRepo(),
		NotifSvc:    newMockNotificationService(),
		Broadcaster: newMockBroadcaster(),
	})
	_, err := svc.CreateChannel(ctx, domain.CreateConversationParams{
		OrgID:     "org-1",
		CreatedBy: "user-1",
		Name:      "",
		Type:      domain.ConvChannel,
	})
	if err == nil {
		t.Fatal("expected error for empty channel name")
	}
}

func TestConversationService_CreateDM(t *testing.T) {
	ctx := context.Background()
	convRepo := newMockConversationRepo()
	userRepo := newMockUserRepo()
	seedUser(userRepo, "user-1", "org-1")
	seedUser(userRepo, "user-2", "org-1")
	svc := NewConversationService(ConversationServiceDeps{
		ConvRepo: convRepo,
		UserRepo: userRepo,
		MsgRepo:  newMockMessageRepo(),
		PrefRepo: newMockUserPrefRepo(),

		LinkRepo:    newMockLinkRepo(),
		NotifSvc:    newMockNotificationService(),
		Broadcaster: newMockBroadcaster(),
	})

	dm, err := svc.CreateDM(ctx, "org-1", "user-1", "user-2")
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if dm.Type != domain.ConvDirect {
		t.Errorf("expected direct type, got '%s'", dm.Type)
	}
	members, err := svc.GetMembers(ctx, dm.ID)
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if len(members) != 2 {
		t.Errorf("expected 2 members, got %d", len(members))
	}
}

func TestConversationService_CreateDM_SelfDM(t *testing.T) {
	ctx := context.Background()
	svc := NewConversationService(ConversationServiceDeps{
		ConvRepo: newMockConversationRepo(),
		UserRepo: newMockUserRepo(),
		MsgRepo:  newMockMessageRepo(),
		PrefRepo: newMockUserPrefRepo(),

		LinkRepo:    newMockLinkRepo(),
		NotifSvc:    newMockNotificationService(),
		Broadcaster: newMockBroadcaster(),
	})
	_, err := svc.CreateDM(ctx, "org-1", "user-1", "user-1")
	if err == nil {
		t.Fatal("expected error for self-DM")
	}
}

func TestConversationService_CreateDM_ReusesExisting(t *testing.T) {
	ctx := context.Background()
	convRepo := newMockConversationRepo()
	userRepo := newMockUserRepo()
	seedUser(userRepo, "user-1", "org-1")
	seedUser(userRepo, "user-2", "org-1")
	svc := NewConversationService(ConversationServiceDeps{
		ConvRepo: convRepo,
		UserRepo: userRepo,
		MsgRepo:  newMockMessageRepo(),
		PrefRepo: newMockUserPrefRepo(),

		LinkRepo:    newMockLinkRepo(),
		NotifSvc:    newMockNotificationService(),
		Broadcaster: newMockBroadcaster(),
	})

	dm1, _ := svc.CreateDM(ctx, "org-1", "user-1", "user-2")
	dm2, err := svc.CreateDM(ctx, "org-1", "user-1", "user-2")
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if dm1.ID != dm2.ID {
		t.Error("expected same DM to be reused")
	}
}

func TestConversationService_CreateGroupDM(t *testing.T) {
	ctx := context.Background()
	convRepo := newMockConversationRepo()
	userRepo := newMockUserRepo()
	seedUser(userRepo, "user-1", "org-1")
	seedUser(userRepo, "user-2", "org-1")
	seedUser(userRepo, "user-3", "org-1")
	svc := NewConversationService(ConversationServiceDeps{
		ConvRepo: convRepo,
		UserRepo: userRepo,
		MsgRepo:  newMockMessageRepo(),
		PrefRepo: newMockUserPrefRepo(),

		LinkRepo:    newMockLinkRepo(),
		NotifSvc:    newMockNotificationService(),
		Broadcaster: newMockBroadcaster(),
	})

	dm, err := svc.CreateGroupDM(ctx, "org-1", "user-1", []string{"user-2", "user-3"})
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if dm.Type != domain.ConvGroup {
		t.Errorf("expected group type, got '%s'", dm.Type)
	}
	members, err := svc.GetMembers(ctx, dm.ID)
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if len(members) != 3 {
		t.Errorf("expected 3 members, got %d", len(members))
	}
}

func TestConversationService_CreateGroupDM_TooFew(t *testing.T) {
	ctx := context.Background()
	svc := NewConversationService(ConversationServiceDeps{
		ConvRepo: newMockConversationRepo(),
		UserRepo: newMockUserRepo(),
		MsgRepo:  newMockMessageRepo(),
		PrefRepo: newMockUserPrefRepo(),

		LinkRepo:    newMockLinkRepo(),
		NotifSvc:    newMockNotificationService(),
		Broadcaster: newMockBroadcaster(),
	})
	_, err := svc.CreateGroupDM(ctx, "org-1", "user-1", []string{"user-2"})
	if err == nil {
		t.Fatal("expected error for group DM with only 1 other member")
	}
}

func TestConversationService_ListMyConversations(t *testing.T) {
	ctx := context.Background()
	convRepo := newMockConversationRepo()
	userRepo := newMockUserRepo()
	seedUser(userRepo, "user-1", "org-1")
	seedUser(userRepo, "user-2", "org-1")
	svc := NewConversationService(ConversationServiceDeps{
		ConvRepo: convRepo,
		UserRepo: userRepo,
		MsgRepo:  newMockMessageRepo(),
		PrefRepo: newMockUserPrefRepo(),

		LinkRepo:    newMockLinkRepo(),
		NotifSvc:    newMockNotificationService(),
		Broadcaster: newMockBroadcaster(),
	})

	svc.CreateChannel(ctx, domain.CreateConversationParams{OrgID: "org-1", CreatedBy: "user-1", Name: "general", Type: domain.ConvChannel})
	svc.CreateDM(ctx, "org-1", "user-1", "user-2")

	result, err := svc.ListMyConversations(ctx, "org-1", "user-1", domain.ConversationFilter{Limit: 20})
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if len(result.Items) != 2 {
		t.Errorf("expected 2 conversations, got %d", len(result.Items))
	}
}

func TestConversationService_DeleteConversation(t *testing.T) {
	ctx := context.Background()
	convRepo := newMockConversationRepo()
	svc := NewConversationService(ConversationServiceDeps{
		ConvRepo: convRepo,
		UserRepo: newMockUserRepo(),
		MsgRepo:  newMockMessageRepo(),
		PrefRepo: newMockUserPrefRepo(),

		LinkRepo:    newMockLinkRepo(),
		NotifSvc:    newMockNotificationService(),
		Broadcaster: newMockBroadcaster(),
	})

	channel, _ := svc.CreateChannel(ctx, domain.CreateConversationParams{OrgID: "org-1", CreatedBy: "user-1", Name: "general", Type: domain.ConvChannel})

	err := svc.DeleteConversation(ctx, "org-1", channel.ID, "user-1", domain.RoleMember)
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	deletedConv, err := svc.GetByID(ctx, "org-1", channel.ID, "user-1")
	if err != nil {
		t.Fatal("conversation should still be retrievable after soft delete")
	}
	if deletedConv.DeletedAt == nil {
		t.Error("expected deleted_at to be set after soft delete")
	}
}

func TestConversationService_AddMembers_ToDirect(t *testing.T) {
	ctx := context.Background()
	convRepo := newMockConversationRepo()
	userRepo := newMockUserRepo()
	seedUser(userRepo, "user-1", "org-1")
	seedUser(userRepo, "user-2", "org-1")
	svc := NewConversationService(ConversationServiceDeps{
		ConvRepo: convRepo,
		UserRepo: userRepo,
		MsgRepo:  newMockMessageRepo(),
		PrefRepo: newMockUserPrefRepo(),

		LinkRepo:    newMockLinkRepo(),
		NotifSvc:    newMockNotificationService(),
		Broadcaster: newMockBroadcaster(),
	})

	dm, _ := svc.CreateDM(ctx, "org-1", "user-1", "user-2")
	err := svc.AddMembers(ctx, "org-1", dm.ID, "user-1", []string{"user-3"})
	if err == nil {
		t.Fatal("expected error adding members to DM")
	}
}

func TestConversationService_EnsureGeneralChannel_Creates(t *testing.T) {
	ctx := context.Background()
	convRepo := newMockConversationRepo()
	svc := NewConversationService(ConversationServiceDeps{
		ConvRepo: convRepo,
		UserRepo: newMockUserRepo(),
		MsgRepo:  newMockMessageRepo(),
		PrefRepo: newMockUserPrefRepo(),

		LinkRepo:    newMockLinkRepo(),
		NotifSvc:    newMockNotificationService(),
		Broadcaster: newMockBroadcaster(),
	})
	if err := svc.EnsureGeneralChannel(ctx, "org-1", "user-1"); err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	convs, _ := convRepo.ListByUser(ctx, "org-1", "user-1", domain.ConversationFilter{})
	if len(convs.Items) != 1 {
		t.Fatalf("expected 1 conv, got %d", len(convs.Items))
	}
	if convs.Items[0].Name != "general" {
		t.Errorf("expected #general, got %s", convs.Items[0].Name)
	}
}

func TestConversationService_EnsureGeneralChannel_Idempotent(t *testing.T) {
	ctx := context.Background()
	convRepo := newMockConversationRepo()
	svc := NewConversationService(ConversationServiceDeps{
		ConvRepo: convRepo,
		UserRepo: newMockUserRepo(),
		MsgRepo:  newMockMessageRepo(),
		PrefRepo: newMockUserPrefRepo(),

		LinkRepo:    newMockLinkRepo(),
		NotifSvc:    newMockNotificationService(),
		Broadcaster: newMockBroadcaster(),
	})
	if err := svc.EnsureGeneralChannel(ctx, "org-1", "user-1"); err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if err := svc.EnsureGeneralChannel(ctx, "org-1", "user-1"); err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	convs, _ := convRepo.ListByUser(ctx, "org-1", "user-1", domain.ConversationFilter{})
	if len(convs.Items) != 1 {
		t.Errorf("expected 1 conv (no duplicate), got %d", len(convs.Items))
	}
}

// TestConversationService_CreateDM_RejectsCrossOrgUser verifies that a DM
// cannot be created with a user who belongs to a different org. The
// users table is global (one account → many org memberships), so without this
// check the conversation_members row would be created for a foreign-org user,
// leaking the DM's contents.
func TestConversationService_CreateDM_RejectsCrossOrgUser(t *testing.T) {
	ctx := context.Background()
	convRepo := newMockConversationRepo()
	userRepo := newMockUserRepo()
	seedUser(userRepo, "user-1", "org-1")
	seedUser(userRepo, "user-2", "org-2") // different org
	svc := NewConversationService(ConversationServiceDeps{
		ConvRepo:    convRepo,
		UserRepo:    userRepo,
		MsgRepo:     newMockMessageRepo(),
		PrefRepo:    newMockUserPrefRepo(),
		LinkRepo:    newMockLinkRepo(),
		NotifSvc:    newMockNotificationService(),
		Broadcaster: newMockBroadcaster(),
	})

	_, err := svc.CreateDM(ctx, "org-1", "user-1", "user-2")
	if err == nil {
		t.Fatal("expected error creating DM with cross-org user, got nil")
	}
	// No conversation should have been created.
	if len(convRepo.convsByID) != 0 {
		t.Errorf("expected 0 conversations, got %d", len(convRepo.convsByID))
	}
}

// TestConversationService_CreateGroupDM_RejectsCrossOrgUser verifies that
// group DMs reject any member ID not in the org before the
// conversation is created.
func TestConversationService_CreateGroupDM_RejectsCrossOrgUser(t *testing.T) {
	ctx := context.Background()
	convRepo := newMockConversationRepo()
	userRepo := newMockUserRepo()
	seedUser(userRepo, "user-1", "org-1")
	seedUser(userRepo, "user-2", "org-1")
	seedUser(userRepo, "user-3", "org-2") // foreign org
	svc := NewConversationService(ConversationServiceDeps{
		ConvRepo:    convRepo,
		UserRepo:    userRepo,
		MsgRepo:     newMockMessageRepo(),
		PrefRepo:    newMockUserPrefRepo(),
		LinkRepo:    newMockLinkRepo(),
		NotifSvc:    newMockNotificationService(),
		Broadcaster: newMockBroadcaster(),
	})

	_, err := svc.CreateGroupDM(ctx, "org-1", "user-1", []string{"user-2", "user-3"})
	if err == nil {
		t.Fatal("expected error creating group DM with cross-org user, got nil")
	}
	if len(convRepo.convsByID) != 0 {
		t.Errorf("expected 0 conversations, got %d", len(convRepo.convsByID))
	}
}

// TestConversationService_AddMembers_RejectsCrossOrgUser verifies that
// AddMembers rejects a foreign-org user rather than adding it to an existing
// conversation in the caller's org.
func TestConversationService_AddMembers_RejectsCrossOrgUser(t *testing.T) {
	ctx := context.Background()
	convRepo := newMockConversationRepo()
	userRepo := newMockUserRepo()
	seedUser(userRepo, "user-1", "org-1")
	seedUser(userRepo, "user-2", "org-1")
	seedUser(userRepo, "user-3", "org-2") // foreign org
	svc := NewConversationService(ConversationServiceDeps{
		ConvRepo:    convRepo,
		UserRepo:    userRepo,
		MsgRepo:     newMockMessageRepo(),
		PrefRepo:    newMockUserPrefRepo(),
		LinkRepo:    newMockLinkRepo(),
		NotifSvc:    newMockNotificationService(),
		Broadcaster: newMockBroadcaster(),
	})

	// Create a channel (group-DMs can't have members added; channels can).
	conv, err := svc.CreateChannel(ctx, domain.CreateConversationParams{
		OrgID: "org-1", CreatedBy: "user-1", Name: "general", Type: domain.ConvChannel,
	})
	if err != nil {
		t.Fatalf("CreateChannel: %v", err)
	}

	// Attempt to add a cross-org user.
	err = svc.AddMembers(ctx, "org-1", conv.ID, "user-1", []string{"user-3"})
	if err == nil {
		t.Fatal("expected error adding cross-org user, got nil")
	}
}

func TestConversationService_ListByParent_FiltersDeniedChildren(t *testing.T) {
	ctx := context.Background()
	convRepo := newMockConversationRepo()
	userRepo := newMockUserRepo()
	seedUser(userRepo, "user-1", "org-1")
	msgRepo := newMockMessageRepo()

	// mockPermService that denies specific channels and allows everything else.
	deniedIDs := make(map[string]bool)
	permSvc := &mockPermService{
		resolvePermissionsFn: func(ctx context.Context, orgID, channelID, userID string, userRole domain.Role) (*domain.ChannelPermissions, error) {
			if deniedIDs[channelID] {
				return &domain.ChannelPermissions{CanView: false, CanSend: false, CanManage: false, CanPermissions: false}, nil
			}
			return &domain.ChannelPermissions{CanView: true, CanSend: true, CanManage: false, CanPermissions: false}, nil
		},
	}

	svc := NewConversationService(ConversationServiceDeps{
		ConvRepo:       convRepo,
		UserRepo:       userRepo,
		MsgRepo:        msgRepo,
		PrefRepo:       newMockUserPrefRepo(),
		LinkRepo:       newMockLinkRepo(),
		PermRepo:       newMockPermRepo(),
		NotifSvc:       newMockNotificationService(),
		ChannelPermSvc: permSvc,
		Broadcaster:    newMockBroadcaster(),
	})

	// Create a parent category.
	cat, err := svc.CreateChannel(ctx, domain.CreateConversationParams{
		OrgID: "org-1", CreatedBy: "user-1", Name: "category", Type: domain.ConvCategory,
	})
	if err != nil {
		t.Fatalf("CreateChannel category: %v", err)
	}

	// Create two child channels.
	allowedCh, err := svc.CreateChannel(ctx, domain.CreateConversationParams{
		OrgID: "org-1", CreatedBy: "user-1", Name: "allowed", Type: domain.ConvChannel, ParentID: &cat.ID,
	})
	if err != nil {
		t.Fatalf("CreateChannel allowed: %v", err)
	}
	deniedCh, err := svc.CreateChannel(ctx, domain.CreateConversationParams{
		OrgID: "org-1", CreatedBy: "user-1", Name: "denied", Type: domain.ConvChannel, ParentID: &cat.ID,
	})
	if err != nil {
		t.Fatalf("CreateChannel denied: %v", err)
	}

	// Register the denied channel so UserHasAccess returns false for it.
	deniedIDs[deniedCh.ID] = true

	t.Run("denied_child_filtered_out", func(t *testing.T) {
		convs, err := svc.ListByParent(ctx, "org-1", cat.ID, "user-1", domain.RoleMember, false)
		if err != nil {
			t.Fatalf("ListByParent: %v", err)
		}
		if len(convs) != 1 {
			t.Fatalf("expected 1 child (allowed), got %d", len(convs))
		}
		if convs[0].ID != allowedCh.ID {
			t.Errorf("expected allowed channel, got %s (%s)", convs[0].Name, convs[0].ID)
		}
	})

	t.Run("no_children_allowed_returns_empty", func(t *testing.T) {
		// Deny both children.
		deniedIDs[allowedCh.ID] = true
		convs, err := svc.ListByParent(ctx, "org-1", cat.ID, "user-1", domain.RoleMember, false)
		if err != nil {
			t.Fatalf("ListByParent: %v", err)
		}
		if len(convs) != 0 {
			t.Errorf("expected 0 children, got %d", len(convs))
		}
	})

	t.Run("error_in_UserHasAccess_skips_child", func(t *testing.T) {
		// Reset denied map and cause an error for the denied channel.
		delete(deniedIDs, allowedCh.ID)
		permSvc.resolvePermissionsFn = func(ctx context.Context, orgID, channelID, userID string, userRole domain.Role) (*domain.ChannelPermissions, error) {
			if channelID == deniedCh.ID {
				return nil, errors.New("unexpected error")
			}
			return &domain.ChannelPermissions{CanView: true, CanSend: true, CanManage: false, CanPermissions: false}, nil
		}

		convs, err := svc.ListByParent(ctx, "org-1", cat.ID, "user-1", domain.RoleMember, false)
		if err != nil {
			t.Fatalf("ListByParent: %v", err)
		}
		if len(convs) != 1 {
			t.Fatalf("expected 1 child (allowed), got %d", len(convs))
		}
		if convs[0].ID != allowedCh.ID {
			t.Errorf("expected allowed channel, got %s (%s)", convs[0].Name, convs[0].ID)
		}
	})
}

func TestConversationService_ListByParent_AllowsAllWhenNoOverrides(t *testing.T) {
	ctx := context.Background()
	convRepo := newMockConversationRepo()
	userRepo := newMockUserRepo()
	seedUser(userRepo, "user-1", "org-1")
	msgRepo := newMockMessageRepo()

	// Default mockPermService: all channels have CanView=true.
	svc := NewConversationService(ConversationServiceDeps{
		ConvRepo:       convRepo,
		UserRepo:       userRepo,
		MsgRepo:        msgRepo,
		PrefRepo:       newMockUserPrefRepo(),
		LinkRepo:       newMockLinkRepo(),
		PermRepo:       newMockPermRepo(),
		NotifSvc:       newMockNotificationService(),
		ChannelPermSvc: newMockChannelPermissionService(),
		Broadcaster:    newMockBroadcaster(),
	})

	cat, err := svc.CreateChannel(ctx, domain.CreateConversationParams{
		OrgID: "org-1", CreatedBy: "user-1", Name: "category", Type: domain.ConvCategory,
	})
	if err != nil {
		t.Fatalf("CreateChannel category: %v", err)
	}

	ch1, _ := svc.CreateChannel(ctx, domain.CreateConversationParams{
		OrgID: "org-1", CreatedBy: "user-1", Name: "ch1", Type: domain.ConvChannel, ParentID: &cat.ID,
	})
	ch2, _ := svc.CreateChannel(ctx, domain.CreateConversationParams{
		OrgID: "org-1", CreatedBy: "user-1", Name: "ch2", Type: domain.ConvChannel, ParentID: &cat.ID,
	})

	convs, err := svc.ListByParent(ctx, "org-1", cat.ID, "user-1", domain.RoleMember, false)
	if err != nil {
		t.Fatalf("ListByParent: %v", err)
	}
	if len(convs) != 2 {
		t.Fatalf("expected 2 children, got %d", len(convs))
	}

	ids := map[string]bool{convs[0].ID: true, convs[1].ID: true}
	if !ids[ch1.ID] {
		t.Errorf("expected ch1 to be present")
	}
	if !ids[ch2.ID] {
		t.Errorf("expected ch2 to be present")
	}
}
