package service

import (
	"context"
	"encoding/json"
	"errors"
	"log/slog"
	"strings"
	"testing"
	"time"

	"ipmanlk/plume/internal/apperr"
	"ipmanlk/plume/internal/domain"
	"ipmanlk/plume/internal/i18n"
)

var testBundle = i18n.NewBundle()

func TestMessageService_ListMessages_Empty(t *testing.T) {
	ctx := context.Background()
	svc := NewMessageService(MessageServiceDeps{
		MsgRepo:            newMockMessageRepo(),
		ConvRepo:           newMockConversationRepo(),
		OrgRepo:            newMockOrgRepo(true),
		ProjectRepo:        newMockProjectRepo(),
		TaskRepo:           newMockTaskRepo(),
		UserRepo:           newMockUserRepo(),
		AttRepo:            newMockMessageAttachmentRepo(),
		PendingAttRepo:     newMockPendingAttachmentRepo(),
		ReactionRepo:       newMockReactionRepo(),
		PrefRepo:           newMockUserPrefRepo(),
		NotifSvc:           newMockNotificationService(),
		ChannelPermService: newMockChannelPermissionService(),
		Broadcaster:        newMockBroadcaster(),
		UserPrefRepo:       nil,
		I18n:               testBundle,
		Log:                slog.Default(),
	})
	result, err := svc.ListMessages(ctx, "org-1", "conv-1", domain.MessageFilter{Limit: 50})
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if len(result.Items) != 0 {
		t.Errorf("expected 0 messages, got %d", len(result.Items))
	}
}

func TestMessageService_ListMessages_Pagination(t *testing.T) {
	ctx := context.Background()
	msgRepo := newMockMessageRepo()
	convRepo := newMockConversationRepo()
	convRepo.Create(ctx, &domain.Conversation{ID: "conv-1", OrgID: "org-1", Type: domain.ConvChannel, Name: "general"})
	convRepo.AddMember(ctx, "org-1", "conv-1", "user-1")

	svc := NewMessageService(MessageServiceDeps{
		MsgRepo:            msgRepo,
		ConvRepo:           convRepo,
		OrgRepo:            newMockOrgRepo(true),
		ProjectRepo:        newMockProjectRepo(),
		TaskRepo:           newMockTaskRepo(),
		UserRepo:           newMockUserRepo(),
		AttRepo:            newMockMessageAttachmentRepo(),
		PendingAttRepo:     newMockPendingAttachmentRepo(),
		ReactionRepo:       newMockReactionRepo(),
		PrefRepo:           newMockUserPrefRepo(),
		NotifSvc:           newMockNotificationService(),
		ChannelPermService: newMockChannelPermissionService(),
		Broadcaster:        newMockBroadcaster(),
		UserPrefRepo:       nil,
		I18n:               testBundle,
		Log:                slog.Default(),
	})

	// Create 5 messages
	for i := 0; i < 5; i++ {
		svc.SendMessage(ctx, domain.CreateMessageParams{
			ConversationID: "conv-1",
			OrgID:          "org-1",
			SenderID:       "user-1",
			Content:        "msg " + strings.Repeat("x", i+1),
		})
	}

	result, err := svc.ListMessages(ctx, "org-1", "conv-1", domain.MessageFilter{Limit: 3})
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if len(result.Items) > 3 {
		t.Errorf("expected at most 3 messages, got %d", len(result.Items))
	}
}

func TestMessageService_SendMessage_RequiresContent(t *testing.T) {
	ctx := context.Background()
	svc := NewMessageService(MessageServiceDeps{
		MsgRepo:            newMockMessageRepo(),
		ConvRepo:           newMockConversationRepo(),
		OrgRepo:            newMockOrgRepo(true),
		ProjectRepo:        newMockProjectRepo(),
		TaskRepo:           newMockTaskRepo(),
		UserRepo:           newMockUserRepo(),
		AttRepo:            newMockMessageAttachmentRepo(),
		PendingAttRepo:     newMockPendingAttachmentRepo(),
		ReactionRepo:       newMockReactionRepo(),
		PrefRepo:           newMockUserPrefRepo(),
		NotifSvc:           newMockNotificationService(),
		ChannelPermService: newMockChannelPermissionService(),
		Broadcaster:        newMockBroadcaster(),
		UserPrefRepo:       nil,
		I18n:               testBundle,
		Log:                slog.Default(),
	})
	_, err := svc.SendMessage(ctx, domain.CreateMessageParams{
		ConversationID: "conv-1",
		OrgID:          "org-1",
		SenderID:       "user-1",
		Content:        "",
	})
	if err == nil {
		t.Fatal("expected error for empty content")
	}
	if !strings.Contains(err.Error(), "required") {
		t.Errorf("expected 'required' error, got: %v", err)
	}
}

func TestMessageService_SendMessage_RequiresMembership(t *testing.T) {
	ctx := context.Background()
	convRepo := newMockConversationRepo()
	convRepo.Create(ctx, &domain.Conversation{ID: "conv-1", OrgID: "org-1", Type: domain.ConvChannel, Name: "general"})

	svc := NewMessageService(MessageServiceDeps{
		MsgRepo:            newMockMessageRepo(),
		ConvRepo:           convRepo,
		OrgRepo:            newMockOrgRepo(true),
		ProjectRepo:        newMockProjectRepo(),
		TaskRepo:           newMockTaskRepo(),
		UserRepo:           newMockUserRepo(),
		AttRepo:            newMockMessageAttachmentRepo(),
		PendingAttRepo:     newMockPendingAttachmentRepo(),
		ReactionRepo:       newMockReactionRepo(),
		PrefRepo:           newMockUserPrefRepo(),
		NotifSvc:           newMockNotificationService(),
		ChannelPermService: newMockChannelPermissionService(),
		Broadcaster:        newMockBroadcaster(),
		UserPrefRepo:       nil,
		I18n:               testBundle,
		Log:                slog.Default(),
	})
	_, err := svc.SendMessage(ctx, domain.CreateMessageParams{
		ConversationID: "conv-1",
		OrgID:          "org-1",
		SenderID:       "user-1",
		Content:        "hello",
	})
	if err == nil {
		t.Fatal("expected error for non-member")
	}
}

func TestMessageService_SendAndRetrieve(t *testing.T) {
	ctx := context.Background()
	msgRepo := newMockMessageRepo()
	convRepo := newMockConversationRepo()
	convRepo.Create(ctx, &domain.Conversation{ID: "conv-1", OrgID: "org-1", Type: domain.ConvChannel, Name: "general"})
	convRepo.AddMember(ctx, "org-1", "conv-1", "user-1")

	svc := NewMessageService(MessageServiceDeps{
		MsgRepo:            msgRepo,
		ConvRepo:           convRepo,
		OrgRepo:            newMockOrgRepo(true),
		ProjectRepo:        newMockProjectRepo(),
		TaskRepo:           newMockTaskRepo(),
		UserRepo:           newMockUserRepo(),
		AttRepo:            newMockMessageAttachmentRepo(),
		PendingAttRepo:     newMockPendingAttachmentRepo(),
		ReactionRepo:       newMockReactionRepo(),
		PrefRepo:           newMockUserPrefRepo(),
		NotifSvc:           newMockNotificationService(),
		ChannelPermService: newMockChannelPermissionService(),
		Broadcaster:        newMockBroadcaster(),
		UserPrefRepo:       nil,
		I18n:               testBundle,
		Log:                slog.Default(),
	})

	sent, err := svc.SendMessage(ctx, domain.CreateMessageParams{
		ConversationID: "conv-1",
		OrgID:          "org-1",
		SenderID:       "user-1",
		Content:        "Hello, world!",
	})
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if sent.Content != "Hello, world!" {
		t.Errorf("expected content 'Hello, world!', got '%s'", sent.Content)
	}
	if sent.SenderID != "user-1" {
		t.Errorf("expected sender user-1, got '%s'", sent.SenderID)
	}
	if sent.ConversationID != "conv-1" {
		t.Errorf("expected conversation conv-1, got '%s'", sent.ConversationID)
	}
	// Verify via list
	result, _ := svc.ListMessages(ctx, "org-1", "conv-1", domain.MessageFilter{Limit: 50})
	if len(result.Items) != 1 {
		t.Errorf("expected 1 message, got %d", len(result.Items))
	}
}

func TestMessageService_SendReply(t *testing.T) {
	ctx := context.Background()
	msgRepo := newMockMessageRepo()
	convRepo := newMockConversationRepo()
	convRepo.Create(ctx, &domain.Conversation{ID: "conv-1", OrgID: "org-1", Type: domain.ConvChannel, Name: "general"})
	convRepo.AddMember(ctx, "org-1", "conv-1", "user-1")
	convRepo.AddMember(ctx, "org-1", "conv-1", "user-2")

	svc := NewMessageService(MessageServiceDeps{
		MsgRepo:            msgRepo,
		ConvRepo:           convRepo,
		OrgRepo:            newMockOrgRepo(true),
		ProjectRepo:        newMockProjectRepo(),
		TaskRepo:           newMockTaskRepo(),
		UserRepo:           newMockUserRepo(),
		AttRepo:            newMockMessageAttachmentRepo(),
		PendingAttRepo:     newMockPendingAttachmentRepo(),
		ReactionRepo:       newMockReactionRepo(),
		PrefRepo:           newMockUserPrefRepo(),
		NotifSvc:           newMockNotificationService(),
		ChannelPermService: newMockChannelPermissionService(),
		Broadcaster:        newMockBroadcaster(),
		UserPrefRepo:       nil,
		I18n:               testBundle,
		Log:                slog.Default(),
	})

	parent, _ := svc.SendMessage(ctx, domain.CreateMessageParams{
		ConversationID: "conv-1",
		OrgID:          "org-1",
		SenderID:       "user-1",
		Content:        "parent",
	})
	reply, err := svc.SendMessage(ctx, domain.CreateMessageParams{
		ConversationID: "conv-1",
		OrgID:          "org-1",
		SenderID:       "user-2",
		Content:        "reply",
		ParentID:       &parent.ID,
	})
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if reply.ParentID == nil || *reply.ParentID != parent.ID {
		t.Errorf("expected parent ID %s, got %v", parent.ID, reply.ParentID)
	}
}

func TestMessageService_EditOwnMessage(t *testing.T) {
	ctx := context.Background()
	convRepo := newMockConversationRepo()
	orgRepo := newMockOrgRepo(true)
	convRepo.Create(ctx, &domain.Conversation{ID: "conv-1", OrgID: "org-1", Type: domain.ConvChannel, Name: "general"})
	convRepo.AddMember(ctx, "org-1", "conv-1", "user-1")

	svc := NewMessageService(MessageServiceDeps{
		MsgRepo:            newMockMessageRepo(),
		ConvRepo:           convRepo,
		OrgRepo:            orgRepo,
		ProjectRepo:        newMockProjectRepo(),
		TaskRepo:           newMockTaskRepo(),
		UserRepo:           newMockUserRepo(),
		AttRepo:            newMockMessageAttachmentRepo(),
		PendingAttRepo:     newMockPendingAttachmentRepo(),
		ReactionRepo:       newMockReactionRepo(),
		PrefRepo:           newMockUserPrefRepo(),
		NotifSvc:           newMockNotificationService(),
		ChannelPermService: newMockChannelPermissionService(),
		Broadcaster:        newMockBroadcaster(),
		UserPrefRepo:       nil,
		I18n:               testBundle,
		Log:                slog.Default(),
	})

	sent, _ := svc.SendMessage(ctx, domain.CreateMessageParams{
		ConversationID: "conv-1",
		OrgID:          "org-1",
		SenderID:       "user-1",
		Content:        "original",
	})
	edited, err := svc.EditMessage(ctx, domain.EditMessageParams{
		MsgID:    sent.ID,
		ConvID:   "conv-1",
		OrgID:    "org-1",
		EditorID: "user-1",
		Content:  "edited",
	})
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if edited.Content != "edited" {
		t.Errorf("expected 'edited', got '%s'", edited.Content)
	}
	if edited.EditedAt == nil {
		t.Error("expected edited_at to be set")
	}
}

func TestMessageService_EditMessage_WrongSender(t *testing.T) {
	ctx := context.Background()
	convRepo := newMockConversationRepo()
	convRepo.Create(ctx, &domain.Conversation{ID: "conv-1", OrgID: "org-1", Type: domain.ConvChannel, Name: "general"})
	convRepo.AddMember(ctx, "org-1", "conv-1", "user-1")
	convRepo.AddMember(ctx, "org-1", "conv-1", "user-2")

	svc := NewMessageService(MessageServiceDeps{
		MsgRepo:            newMockMessageRepo(),
		ConvRepo:           convRepo,
		OrgRepo:            newMockOrgRepo(true),
		ProjectRepo:        newMockProjectRepo(),
		TaskRepo:           newMockTaskRepo(),
		UserRepo:           newMockUserRepo(),
		AttRepo:            newMockMessageAttachmentRepo(),
		PendingAttRepo:     newMockPendingAttachmentRepo(),
		ReactionRepo:       newMockReactionRepo(),
		PrefRepo:           newMockUserPrefRepo(),
		NotifSvc:           newMockNotificationService(),
		ChannelPermService: newMockChannelPermissionService(),
		Broadcaster:        newMockBroadcaster(),
		UserPrefRepo:       nil,
		I18n:               testBundle,
		Log:                slog.Default(),
	})

	sent, _ := svc.SendMessage(ctx, domain.CreateMessageParams{
		ConversationID: "conv-1",
		OrgID:          "org-1",
		SenderID:       "user-1",
		Content:        "original",
	})
	_, err := svc.EditMessage(ctx, domain.EditMessageParams{
		MsgID:    sent.ID,
		ConvID:   "conv-1",
		OrgID:    "org-1",
		EditorID: "user-2",
		Content:  "hacked",
	})
	if err == nil {
		t.Fatal("expected error for editing another user's message")
	}
}

func TestMessageService_DeleteMessage_WrongSender(t *testing.T) {
	ctx := context.Background()
	msgRepo := newMockMessageRepo()
	convRepo := newMockConversationRepo()
	convRepo.Create(ctx, &domain.Conversation{ID: "conv-1", OrgID: "org-1", Type: domain.ConvChannel, Name: "general"})
	convRepo.AddMember(ctx, "org-1", "conv-1", "user-1")
	convRepo.AddMember(ctx, "org-1", "conv-1", "user-2")

	svc := NewMessageService(MessageServiceDeps{
		MsgRepo:            msgRepo,
		ConvRepo:           convRepo,
		OrgRepo:            newMockOrgRepo(true),
		ProjectRepo:        newMockProjectRepo(),
		TaskRepo:           newMockTaskRepo(),
		UserRepo:           newMockUserRepo(),
		AttRepo:            newMockMessageAttachmentRepo(),
		PendingAttRepo:     newMockPendingAttachmentRepo(),
		ReactionRepo:       newMockReactionRepo(),
		PrefRepo:           newMockUserPrefRepo(),
		NotifSvc:           newMockNotificationService(),
		ChannelPermService: newMockChannelPermissionService(),
		Broadcaster:        newMockBroadcaster(),
		UserPrefRepo:       nil,
		I18n:               testBundle,
		Log:                slog.Default(),
	})

	sent, _ := svc.SendMessage(ctx, domain.CreateMessageParams{
		ConversationID: "conv-1",
		OrgID:          "org-1",
		SenderID:       "user-1",
		Content:        "delete me",
	})
	err := svc.DeleteMessage(ctx, "org-1", sent.ID, "conv-1", "user-2")
	if err == nil {
		t.Fatal("expected error for deleting another user's message")
	}
}

// TestMessageService_EditMessage_AlreadyDeleted verifies that editing a
// soft-deleted message fails (regression guard). GetMessageByID
// excludes deleted_at rows, so EditMessage returns an error and never
// resurrects or re-broadcasts the deleted message.
func TestMessageService_EditMessage_AlreadyDeleted(t *testing.T) {
	ctx := context.Background()
	msgRepo := newMockMessageRepo()
	convRepo := newMockConversationRepo()
	convRepo.Create(ctx, &domain.Conversation{ID: "conv-1", OrgID: "org-1", Type: domain.ConvChannel, Name: "general"})
	convRepo.AddMember(ctx, "org-1", "conv-1", "user-1")
	bc := newMockBroadcaster()
	svc := NewMessageService(MessageServiceDeps{
		MsgRepo:            msgRepo,
		ConvRepo:           convRepo,
		OrgRepo:            newMockOrgRepo(true),
		ProjectRepo:        newMockProjectRepo(),
		TaskRepo:           newMockTaskRepo(),
		UserRepo:           newMockUserRepo(),
		AttRepo:            newMockMessageAttachmentRepo(),
		PendingAttRepo:     newMockPendingAttachmentRepo(),
		ReactionRepo:       newMockReactionRepo(),
		PrefRepo:           newMockUserPrefRepo(),
		NotifSvc:           newMockNotificationService(),
		ChannelPermService: newMockChannelPermissionService(),
		Broadcaster:        bc,
		UserPrefRepo:       nil,
		I18n:               testBundle,
		Log:                slog.Default(),
	})

	sent, _ := svc.SendMessage(ctx, domain.CreateMessageParams{
		ConversationID: "conv-1",
		OrgID:          "org-1",
		SenderID:       "user-1",
		Content:        "original",
	})
	if err := svc.DeleteMessage(ctx, "org-1", sent.ID, "conv-1", "user-1"); err != nil {
		t.Fatalf("initial delete: %v", err)
	}

	bc.reset()
	_, err := svc.EditMessage(ctx, domain.EditMessageParams{
		MsgID:    sent.ID,
		ConvID:   "conv-1",
		OrgID:    "org-1",
		EditorID: "user-1",
		Content:  "resurrect attempt",
	})
	if err == nil {
		t.Fatal("expected error when editing a deleted message; got nil (message would be resurrected)")
	}
	if bc.broadcastCount() != 0 {
		t.Errorf("expected no broadcast when editing deleted message, got %d", bc.broadcastCount())
	}
}

// TestMessageService_DeleteMessage_AlreadyDeleted verifies that deleting an
// already-deleted message fails fast without a spurious message_deleted
// broadcast (regression guard).
func TestMessageService_DeleteMessage_AlreadyDeleted(t *testing.T) {
	ctx := context.Background()
	msgRepo := newMockMessageRepo()
	convRepo := newMockConversationRepo()
	convRepo.Create(ctx, &domain.Conversation{ID: "conv-1", OrgID: "org-1", Type: domain.ConvChannel, Name: "general"})
	convRepo.AddMember(ctx, "org-1", "conv-1", "user-1")
	bc := newMockBroadcaster()
	svc := NewMessageService(MessageServiceDeps{
		MsgRepo:            msgRepo,
		ConvRepo:           convRepo,
		OrgRepo:            newMockOrgRepo(true),
		ProjectRepo:        newMockProjectRepo(),
		TaskRepo:           newMockTaskRepo(),
		UserRepo:           newMockUserRepo(),
		AttRepo:            newMockMessageAttachmentRepo(),
		PendingAttRepo:     newMockPendingAttachmentRepo(),
		ReactionRepo:       newMockReactionRepo(),
		PrefRepo:           newMockUserPrefRepo(),
		NotifSvc:           newMockNotificationService(),
		ChannelPermService: newMockChannelPermissionService(),
		Broadcaster:        bc,
		UserPrefRepo:       nil,
		I18n:               testBundle,
		Log:                slog.Default(),
	})

	sent, _ := svc.SendMessage(ctx, domain.CreateMessageParams{
		ConversationID: "conv-1",
		OrgID:          "org-1",
		SenderID:       "user-1",
		Content:        "original",
	})
	if err := svc.DeleteMessage(ctx, "org-1", sent.ID, "conv-1", "user-1"); err != nil {
		t.Fatalf("initial delete: %v", err)
	}

	bc.reset()
	err := svc.DeleteMessage(ctx, "org-1", sent.ID, "conv-1", "user-1")
	if err == nil {
		t.Fatal("expected error when deleting an already-deleted message; got nil")
	}
	if bc.broadcastCount() != 0 {
		t.Errorf("expected no broadcast on second delete, got %d", bc.broadcastCount())
	}
}

func TestMessageService_PinAndUnpin(t *testing.T) {
	ctx := context.Background()
	msgRepo := newMockMessageRepo()
	convRepo := newMockConversationRepo()
	convRepo.Create(ctx, &domain.Conversation{ID: "conv-1", OrgID: "org-1", Type: domain.ConvChannel, Name: "general"})
	convRepo.AddMember(ctx, "org-1", "conv-1", "user-1")

	svc := NewMessageService(MessageServiceDeps{
		MsgRepo:            msgRepo,
		ConvRepo:           convRepo,
		OrgRepo:            newMockOrgRepo(true),
		ProjectRepo:        newMockProjectRepo(),
		TaskRepo:           newMockTaskRepo(),
		UserRepo:           newMockUserRepo(),
		AttRepo:            newMockMessageAttachmentRepo(),
		PendingAttRepo:     newMockPendingAttachmentRepo(),
		ReactionRepo:       newMockReactionRepo(),
		PrefRepo:           newMockUserPrefRepo(),
		NotifSvc:           newMockNotificationService(),
		ChannelPermService: newMockChannelPermissionService(),
		Broadcaster:        newMockBroadcaster(),
		UserPrefRepo:       nil,
		I18n:               testBundle,
		Log:                slog.Default(),
	})

	sent, _ := svc.SendMessage(ctx, domain.CreateMessageParams{
		ConversationID: "conv-1",
		OrgID:          "org-1",
		SenderID:       "user-1",
		Content:        "pin this",
	})
	err := svc.PinMessage(ctx, "org-1", sent.ID, "conv-1", "user-1")
	if err != nil {
		t.Fatalf("unexpected error pinning: %v", err)
	}
	msg, _ := msgRepo.GetByID(ctx, sent.ID, "conv-1")
	if !msg.Pinned {
		t.Error("expected message to be pinned")
	}

	err = svc.UnpinMessage(ctx, "org-1", sent.ID, "conv-1")
	if err != nil {
		t.Fatalf("unexpected error unpinning: %v", err)
	}
	msg, _ = msgRepo.GetByID(ctx, sent.ID, "conv-1")
	if msg.Pinned {
		t.Error("expected message to be unpinned")
	}
}

func TestMessageService_BroadcasterCalledOnSend(t *testing.T) {
	ctx := context.Background()
	convRepo := newMockConversationRepo()
	convRepo.Create(ctx, &domain.Conversation{ID: "conv-1", OrgID: "org-1", Type: domain.ConvChannel, Name: "general"})
	convRepo.AddMember(ctx, "org-1", "conv-1", "user-1")

	broadcaster := newMockBroadcaster()
	svc := NewMessageService(MessageServiceDeps{
		MsgRepo:            newMockMessageRepo(),
		ConvRepo:           convRepo,
		OrgRepo:            newMockOrgRepo(true),
		ProjectRepo:        newMockProjectRepo(),
		TaskRepo:           newMockTaskRepo(),
		UserRepo:           newMockUserRepo(),
		AttRepo:            newMockMessageAttachmentRepo(),
		PendingAttRepo:     newMockPendingAttachmentRepo(),
		ReactionRepo:       newMockReactionRepo(),
		PrefRepo:           newMockUserPrefRepo(),
		NotifSvc:           newMockNotificationService(),
		ChannelPermService: newMockChannelPermissionService(),
		Broadcaster:        broadcaster,
	})

	svc.SendMessage(ctx, domain.CreateMessageParams{
		ConversationID: "conv-1",
		OrgID:          "org-1",
		SenderID:       "user-1",
		Content:        "hello",
	})
	if len(broadcaster.messages) < 1 {
		t.Fatal("expected at least 1 broadcast")
	}
	lastMsg := broadcaster.messages[len(broadcaster.messages)-1]
	if lastMsg.eventType != string(domain.WsTypeMessageNew) {
		t.Errorf("expected message_new, got '%s'", lastMsg.eventType)
	}
	if lastMsg.roomKey != "org:org-1:conversation:conv-1" {
		t.Errorf("expected conversation room, got '%s'", lastMsg.roomKey)
	}
}

func TestMessageService_NotificationSentForDM(t *testing.T) {
	ctx := context.Background()
	convRepo := newMockConversationRepo()
	convRepo.Create(ctx, &domain.Conversation{ID: "dm-1", OrgID: "org-1", Type: domain.ConvDirect})
	convRepo.AddMember(ctx, "org-1", "dm-1", "user-1")
	convRepo.AddMember(ctx, "org-1", "dm-1", "user-2")

	notifSvc := newMockNotificationService()
	svc := NewMessageService(MessageServiceDeps{
		MsgRepo:        newMockMessageRepo(),
		ConvRepo:       convRepo,
		OrgRepo:        newMockOrgRepo(true),
		ProjectRepo:    newMockProjectRepo(),
		TaskRepo:       newMockTaskRepo(),
		UserRepo:       newMockUserRepo(),
		AttRepo:        newMockMessageAttachmentRepo(),
		PendingAttRepo: newMockPendingAttachmentRepo(),
		ReactionRepo:   newMockReactionRepo(),
		PrefRepo:       newMockUserPrefRepo(),
		NotifSvc:       notifSvc,
		Broadcaster:    newMockBroadcaster(),
		UserPrefRepo:   nil,
		I18n:           nil,
		Log:            slog.Default(),
	})

	svc.SendMessage(ctx, domain.CreateMessageParams{
		ConversationID: "dm-1",
		OrgID:          "org-1",
		SenderID:       "user-1",
		Content:        "hey user-2",
	})
	if len(notifSvc.notifications) != 1 {
		t.Fatalf("expected 1 notification, got %d", len(notifSvc.notifications))
	}
	if notifSvc.notifications[0].UserID != "user-2" {
		t.Errorf("expected notification for user-2, got '%s'", notifSvc.notifications[0].UserID)
	}
	if notifSvc.notifications[0].Type != domain.NotifChatDM {
		t.Errorf("expected NotifChatDM, got '%s'", notifSvc.notifications[0].Type)
	}
}

func TestMessageService_NotificationsNotSentForMuted(t *testing.T) {
	ctx := context.Background()
	convRepo := newMockConversationRepo()
	convRepo.Create(ctx, &domain.Conversation{ID: "dm-1", OrgID: "org-1", Type: domain.ConvDirect})
	convRepo.AddMember(ctx, "org-1", "dm-1", "user-1")
	convRepo.AddMember(ctx, "org-1", "dm-1", "user-2")
	prefRepo := newMockUserPrefRepo()
	_ = prefRepo.SetMuted(ctx, "user-2", "dm-1", "org-1", true)

	notifSvc := newMockNotificationService()
	svc := NewMessageService(MessageServiceDeps{
		MsgRepo:        newMockMessageRepo(),
		ConvRepo:       convRepo,
		OrgRepo:        newMockOrgRepo(true),
		ProjectRepo:    newMockProjectRepo(),
		TaskRepo:       newMockTaskRepo(),
		UserRepo:       newMockUserRepo(),
		AttRepo:        newMockMessageAttachmentRepo(),
		PendingAttRepo: newMockPendingAttachmentRepo(),
		ReactionRepo:   newMockReactionRepo(),
		PrefRepo:       prefRepo,
		NotifSvc:       notifSvc,
		Broadcaster:    newMockBroadcaster(),
		UserPrefRepo:   nil,
		I18n:           nil,
		Log:            slog.Default(),
	})

	svc.SendMessage(ctx, domain.CreateMessageParams{
		ConversationID: "dm-1",
		OrgID:          "org-1",
		SenderID:       "user-1",
		Content:        "hey",
	})
	if len(notifSvc.notifications) != 0 {
		t.Errorf("expected 0 notifications for muted user, got %d", len(notifSvc.notifications))
	}
}

func TestMessageService_EveryoneMentionNotifiesChannel(t *testing.T) {
	ctx := context.Background()
	convRepo := newMockConversationRepo()
	convRepo.Create(ctx, &domain.Conversation{ID: "ch-1", OrgID: "org-1", Type: domain.ConvChannel, Name: "general"})
	convRepo.AddMember(ctx, "org-1", "ch-1", "user-1")
	convRepo.AddMember(ctx, "org-1", "ch-1", "user-2")
	convRepo.AddMember(ctx, "org-1", "ch-1", "user-3")

	notifSvc := newMockNotificationService()
	svc := NewMessageService(MessageServiceDeps{
		MsgRepo:        newMockMessageRepo(),
		ConvRepo:       convRepo,
		OrgRepo:        newMockOrgRepo(true),
		ProjectRepo:    newMockProjectRepo(),
		TaskRepo:       newMockTaskRepo(),
		UserRepo:       newMockUserRepo(),
		AttRepo:        newMockMessageAttachmentRepo(),
		PendingAttRepo: newMockPendingAttachmentRepo(),
		ReactionRepo:   newMockReactionRepo(),
		PrefRepo:       newMockUserPrefRepo(),
		NotifSvc:       notifSvc,
		Broadcaster:    newMockBroadcaster(),
		UserPrefRepo:   nil,
		I18n:           testBundle,
		Log:            slog.Default(),
	})

	svc.SendMessage(ctx, domain.CreateMessageParams{
		ConversationID: "ch-1",
		OrgID:          "org-1",
		SenderID:       "user-1",
		Content:        "<p>Hello <@everyone>!</p>",
	})
	if len(notifSvc.notifications) != 2 {
		t.Errorf("expected 2 @everyone notifications, got %d", len(notifSvc.notifications))
	}
	for _, n := range notifSvc.notifications {
		if n.Type != domain.NotifChatMention {
			t.Errorf("expected chat_mention type, got '%s'", n.Type)
		}
		if n.Title != "You were mentioned" {
			t.Errorf("expected 'You were mentioned' title, got '%s'", n.Title)
		}
		if !strings.Contains(n.Body, "Someone") || !strings.Contains(n.Body, "#general") {
			t.Errorf("expected body to mention sender and channel, got: %s", n.Body)
		}
	}
}

func TestMessageService_ChannelMessageNoNotificationsWithoutMention(t *testing.T) {
	ctx := context.Background()
	convRepo := newMockConversationRepo()
	convRepo.Create(ctx, &domain.Conversation{ID: "ch-1", OrgID: "org-1", Type: domain.ConvChannel, Name: "general"})
	convRepo.AddMember(ctx, "org-1", "ch-1", "user-1")
	convRepo.AddMember(ctx, "org-1", "ch-1", "user-2")

	notifSvc := newMockNotificationService()
	svc := NewMessageService(MessageServiceDeps{
		MsgRepo:        newMockMessageRepo(),
		ConvRepo:       convRepo,
		OrgRepo:        newMockOrgRepo(true),
		ProjectRepo:    newMockProjectRepo(),
		TaskRepo:       newMockTaskRepo(),
		UserRepo:       newMockUserRepo(),
		AttRepo:        newMockMessageAttachmentRepo(),
		PendingAttRepo: newMockPendingAttachmentRepo(),
		ReactionRepo:   newMockReactionRepo(),
		PrefRepo:       newMockUserPrefRepo(),
		NotifSvc:       notifSvc,
		Broadcaster:    newMockBroadcaster(),
		UserPrefRepo:   nil,
		I18n:           nil,
		Log:            slog.Default(),
	})

	svc.SendMessage(ctx, domain.CreateMessageParams{
		ConversationID: "ch-1",
		OrgID:          "org-1",
		SenderID:       "user-1",
		Content:        "<p>Just a regular message</p>",
	})
	if len(notifSvc.notifications) != 0 {
		t.Errorf("expected 0 notifications for channel message without @everyone, got %d", len(notifSvc.notifications))
	}
}

func TestMessageService_ReplyNotifiesParentSender(t *testing.T) {
	ctx := context.Background()
	convRepo := newMockConversationRepo()
	convRepo.Create(ctx, &domain.Conversation{ID: "ch-1", OrgID: "org-1", Type: domain.ConvChannel, Name: "general"})
	convRepo.AddMember(ctx, "org-1", "ch-1", "user-1")
	convRepo.AddMember(ctx, "org-1", "ch-1", "user-2")

	notifSvc := newMockNotificationService()
	svc := NewMessageService(MessageServiceDeps{
		MsgRepo:        newMockMessageRepo(),
		ConvRepo:       convRepo,
		OrgRepo:        newMockOrgRepo(true),
		ProjectRepo:    newMockProjectRepo(),
		TaskRepo:       newMockTaskRepo(),
		UserRepo:       newMockUserRepo(),
		AttRepo:        newMockMessageAttachmentRepo(),
		PendingAttRepo: newMockPendingAttachmentRepo(),
		ReactionRepo:   newMockReactionRepo(),
		PrefRepo:       newMockUserPrefRepo(),
		NotifSvc:       notifSvc,
		Broadcaster:    newMockBroadcaster(),
		UserPrefRepo:   nil,
		I18n:           nil,
		Log:            slog.Default(),
	})

	parent, _ := svc.SendMessage(ctx, domain.CreateMessageParams{
		ConversationID: "ch-1",
		OrgID:          "org-1",
		SenderID:       "user-1",
		Content:        "Original message",
	})
	notifSvc.notifications = nil

	svc.SendMessage(ctx, domain.CreateMessageParams{
		ConversationID: "ch-1",
		OrgID:          "org-1",
		SenderID:       "user-2",
		Content:        "Reply to parent",
		ParentID:       &parent.ID,
	})
	hasReplyNotif := false
	for _, n := range notifSvc.notifications {
		if n.UserID == "user-1" && n.Type == domain.NotifChatMention {
			hasReplyNotif = true
			break
		}
	}
	if !hasReplyNotif {
		t.Error("expected reply notification to original message sender (user-1)")
	}
}

func TestMessageService_AddRemoveReaction(t *testing.T) {
	ctx := context.Background()
	msgRepo := newMockMessageRepo()
	convRepo := newMockConversationRepo()
	convRepo.Create(ctx, &domain.Conversation{ID: "conv-1", OrgID: "org-1", Type: domain.ConvChannel, Name: "general"})
	convRepo.AddMember(ctx, "org-1", "conv-1", "user-1")
	convRepo.AddMember(ctx, "org-1", "conv-1", "user-2")
	convRepo.AddMember(ctx, "org-1", "conv-1", "user-3")
	reactionRepo := newMockReactionRepo()
	prefRepo := newMockUserPrefRepo()

	svc := NewMessageService(MessageServiceDeps{
		MsgRepo:            msgRepo,
		ConvRepo:           convRepo,
		OrgRepo:            newMockOrgRepo(true),
		ProjectRepo:        newMockProjectRepo(),
		TaskRepo:           newMockTaskRepo(),
		UserRepo:           newMockUserRepo(),
		AttRepo:            newMockMessageAttachmentRepo(),
		PendingAttRepo:     newMockPendingAttachmentRepo(),
		ReactionRepo:       reactionRepo,
		PrefRepo:           prefRepo,
		NotifSvc:           newMockNotificationService(),
		ChannelPermService: newMockChannelPermissionService(),
		Broadcaster:        newMockBroadcaster(),
		UserPrefRepo:       nil,
		I18n:               testBundle,
		Log:                slog.Default(),
	})

	sent, _ := svc.SendMessage(ctx, domain.CreateMessageParams{
		ConversationID: "conv-1",
		OrgID:          "org-1",
		SenderID:       "user-1",
		Content:        "react to this",
	})

	if err := svc.AddReaction(ctx, domain.AddReactionParams{
		MsgID:  sent.ID,
		ConvID: "conv-1",
		UserID: "user-2",
		OrgID:  "org-1",
		Emoji:  "🎉",
	}); err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if err := svc.AddReaction(ctx, domain.AddReactionParams{
		MsgID:  sent.ID,
		ConvID: "conv-1",
		UserID: "user-3",
		OrgID:  "org-1",
		Emoji:  "🎉",
	}); err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	reactions, _ := reactionRepo.ListForMessages(ctx, []string{sent.ID})
	if len(reactions) != 2 {
		t.Errorf("expected 2 reactions, got %d", len(reactions))
	}

	if err := svc.RemoveReaction(ctx, domain.RemoveReactionParams{
		MsgID:  sent.ID,
		ConvID: "conv-1",
		UserID: "user-2",
		OrgID:  "org-1",
		Emoji:  "🎉",
	}); err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	reactions, _ = reactionRepo.ListForMessages(ctx, []string{sent.ID})
	if len(reactions) != 1 {
		t.Errorf("expected 1 reaction after remove, got %d", len(reactions))
	}
}

func TestMessageService_NotificationLevelRespected(t *testing.T) {
	ctx := context.Background()
	convRepo := newMockConversationRepo()
	convRepo.Create(ctx, &domain.Conversation{ID: "ch-1", OrgID: "org-1", Type: domain.ConvChannel, Name: "general"})
	convRepo.AddMember(ctx, "org-1", "ch-1", "user-1")
	convRepo.AddMember(ctx, "org-1", "ch-1", "user-2")
	prefRepo := newMockUserPrefRepo()
	_ = prefRepo.SetNotificationLevel(ctx, "user-2", "ch-1", "org-1", domain.NotifLevelNothing)

	notifSvc := newMockNotificationService()
	svc := NewMessageService(MessageServiceDeps{
		MsgRepo:        newMockMessageRepo(),
		ConvRepo:       convRepo,
		OrgRepo:        newMockOrgRepo(true),
		ProjectRepo:    newMockProjectRepo(),
		TaskRepo:       newMockTaskRepo(),
		UserRepo:       newMockUserRepo(),
		AttRepo:        newMockMessageAttachmentRepo(),
		PendingAttRepo: newMockPendingAttachmentRepo(),
		ReactionRepo:   newMockReactionRepo(),
		PrefRepo:       prefRepo,
		NotifSvc:       notifSvc,
		Broadcaster:    newMockBroadcaster(),
		UserPrefRepo:   nil,
		I18n:           nil,
		Log:            slog.Default(),
	})

	// Channel message with @everyone should NOT notify user-2 (level=nothing)
	svc.SendMessage(ctx, domain.CreateMessageParams{
		ConversationID: "ch-1",
		OrgID:          "org-1",
		SenderID:       "user-1",
		Content:        "<p>Hello @everyone!</p>",
	})
	for _, n := range notifSvc.notifications {
		if n.UserID == "user-2" {
			t.Errorf("user-2 should not be notified (level=nothing), but got notification: %+v", n)
		}
	}
}

func TestMessageService_MentionUserTriggersNotification(t *testing.T) {
	ctx := context.Background()
	convRepo := newMockConversationRepo()
	convRepo.Create(ctx, &domain.Conversation{ID: "ch-1", OrgID: "org-1", Type: domain.ConvChannel, Name: "general"})
	convRepo.AddMember(ctx, "org-1", "ch-1", "user-1")
	convRepo.AddMember(ctx, "org-1", "ch-1", "user-2")
	notifSvc := newMockNotificationService()
	svc := NewMessageService(MessageServiceDeps{
		MsgRepo:        newMockMessageRepo(),
		ConvRepo:       convRepo,
		OrgRepo:        newMockOrgRepo(true),
		ProjectRepo:    newMockProjectRepo(),
		TaskRepo:       newMockTaskRepo(),
		UserRepo:       newMockUserRepo(),
		AttRepo:        newMockMessageAttachmentRepo(),
		PendingAttRepo: newMockPendingAttachmentRepo(),
		ReactionRepo:   newMockReactionRepo(),
		PrefRepo:       newMockUserPrefRepo(),
		NotifSvc:       notifSvc,
		Broadcaster:    newMockBroadcaster(),
		UserPrefRepo:   nil,
		I18n:           nil,
		Log:            slog.Default(),
	})

	// Channel message with user mention token (ID-only)
	svc.SendMessage(ctx, domain.CreateMessageParams{
		ConversationID: "ch-1",
		OrgID:          "org-1",
		SenderID:       "user-1",
		Content:        "Hey <@user:user-2>",
	})
	hasUserNotif := false
	for _, n := range notifSvc.notifications {
		if n.UserID == "user-2" {
			hasUserNotif = true
		}
	}
	if !hasUserNotif {
		t.Error("expected user-2 to be notified by @user mention")
	}
}

func TestMessageService_SendMessage_WithAttachments(t *testing.T) {
	ctx := context.Background()
	msgRepo := newMockMessageRepo()
	convRepo := newMockConversationRepo()
	convRepo.Create(ctx, &domain.Conversation{ID: "conv-1", OrgID: "org-1", Type: domain.ConvChannel, Name: "general"})
	convRepo.AddMember(ctx, "org-1", "conv-1", "user-1")

	pendingRepo := newMockPendingAttachmentRepo()
	pendingRepo.Create(ctx, &domain.PendingAttachment{ID: "att-1", ConversationID: "conv-1", FileName: "doc.pdf", FileSize: 123, ContentType: "application/pdf", StoragePath: "chat/conv-1/att-1", UploadedBy: "user-1"})

	svc := NewMessageService(MessageServiceDeps{
		MsgRepo:            msgRepo,
		ConvRepo:           convRepo,
		OrgRepo:            newMockOrgRepo(true),
		ProjectRepo:        newMockProjectRepo(),
		TaskRepo:           newMockTaskRepo(),
		UserRepo:           newMockUserRepo(),
		AttRepo:            newMockMessageAttachmentRepo(),
		PendingAttRepo:     pendingRepo,
		ReactionRepo:       newMockReactionRepo(),
		PrefRepo:           newMockUserPrefRepo(),
		NotifSvc:           newMockNotificationService(),
		ChannelPermService: newMockChannelPermissionService(),
		Broadcaster:        newMockBroadcaster(),
		UserPrefRepo:       nil,
		I18n:               testBundle,
		Log:                slog.Default(),
	})

	sent, err := svc.SendMessage(ctx, domain.CreateMessageParams{
		ConversationID: "conv-1",
		OrgID:          "org-1",
		SenderID:       "user-1",
		AttachmentIDs:  []string{"att-1"},
	})
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if len(sent.Attachments) != 1 {
		t.Fatalf("expected 1 attachment on sent message, got %d", len(sent.Attachments))
	}
	if sent.Attachments[0].ID != "att-1" {
		t.Errorf("expected attachment id att-1, got %s", sent.Attachments[0].ID)
	}

	result, err := svc.ListMessages(ctx, "org-1", "conv-1", domain.MessageFilter{Limit: 50})
	if err != nil {
		t.Fatalf("list messages error: %v", err)
	}
	if len(result.Items) != 1 {
		t.Fatalf("expected 1 message, got %d", len(result.Items))
	}
	if len(result.Items[0].Attachments) != 1 {
		t.Errorf("expected 1 attachment in list, got %d", len(result.Items[0].Attachments))
	}
}

func TestMessageService_SearchMessages_EmptyQuery(t *testing.T) {
	ctx := context.Background()
	svc := NewMessageService(MessageServiceDeps{
		MsgRepo:            newMockMessageRepo(),
		ConvRepo:           newMockConversationRepo(),
		OrgRepo:            newMockOrgRepo(true),
		ProjectRepo:        newMockProjectRepo(),
		TaskRepo:           newMockTaskRepo(),
		UserRepo:           newMockUserRepo(),
		AttRepo:            newMockMessageAttachmentRepo(),
		PendingAttRepo:     newMockPendingAttachmentRepo(),
		ReactionRepo:       newMockReactionRepo(),
		PrefRepo:           newMockUserPrefRepo(),
		NotifSvc:           newMockNotificationService(),
		ChannelPermService: newMockChannelPermissionService(),
		Broadcaster:        newMockBroadcaster(),
		UserPrefRepo:       nil,
		I18n:               testBundle,
		Log:                slog.Default(),
	})

	result, err := svc.SearchMessages(ctx, "org-1", "user-1", domain.MessageSearchFilter{Query: "   "})
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if len(result.Items) != 0 {
		t.Errorf("expected empty result for empty query, got %d", len(result.Items))
	}
}

func TestMessageService_SearchMessages_FTS5SyntaxError(t *testing.T) {
	ctx := context.Background()
	msgRepo := newMockMessageRepo()
	msgRepo.searchErr = errors.New("fts5: syntax error near '*'")
	svc := NewMessageService(MessageServiceDeps{
		MsgRepo:            msgRepo,
		ConvRepo:           newMockConversationRepo(),
		OrgRepo:            newMockOrgRepo(true),
		ProjectRepo:        newMockProjectRepo(),
		TaskRepo:           newMockTaskRepo(),
		UserRepo:           newMockUserRepo(),
		AttRepo:            newMockMessageAttachmentRepo(),
		PendingAttRepo:     newMockPendingAttachmentRepo(),
		ReactionRepo:       newMockReactionRepo(),
		PrefRepo:           newMockUserPrefRepo(),
		NotifSvc:           newMockNotificationService(),
		ChannelPermService: newMockChannelPermissionService(),
		Broadcaster:        newMockBroadcaster(),
		UserPrefRepo:       nil,
		I18n:               testBundle,
		Log:                slog.Default(),
	})

	result, err := svc.SearchMessages(ctx, "org-1", "user-1", domain.MessageSearchFilter{Query: "*"})
	if err == nil {
		t.Fatal("expected error for FTS5 syntax error, got nil")
	}
	if !errors.Is(err, apperr.ErrInvalidInput) {
		t.Errorf("expected ErrInvalidInput, got %v (%T)", err, err)
	}
	if !strings.Contains(err.Error(), "invalid search query syntax") {
		t.Errorf("expected 'invalid search query syntax' in error message, got %v", err)
	}
	if result != nil {
		t.Errorf("expected nil result on error, got %d items", len(result.Items))
	}
}

// TestMessageService_EditMessage_BroadcastsSnakeCase asserts that EditMessage
// broadcasts the updated message using the wire format (snake_case json tags),
// not the raw domain.Message struct (which has no json tags and would serialize
// to PascalCase).
func TestMessageService_EditMessage_BroadcastsSnakeCase(t *testing.T) {
	ctx := context.Background()
	msgRepo := newMockMessageRepo()
	orgRepo := newMockOrgRepo(true)
	bc := newMockBroadcaster()

	// Pre-create a message that will be "sent" so it exists for editing.
	msg := &domain.Message{
		ID:             "msg-1",
		ConversationID: "conv-1",
		OrgID:          "org-1",
		SenderID:       "user-1",
		Content:        "hello",
		CreatedAt:      time.Now(),
	}
	msgRepo.msgsByID[msg.ID] = msg

	convRepo := newMockConversationRepo()
	convRepo.AddMember(ctx, "org-1", "conv-1", "user-1")

	svc := NewMessageService(MessageServiceDeps{
		MsgRepo:            msgRepo,
		ConvRepo:           convRepo,
		OrgRepo:            orgRepo,
		ProjectRepo:        newMockProjectRepo(),
		TaskRepo:           newMockTaskRepo(),
		UserRepo:           newMockUserRepo(),
		AttRepo:            newMockMessageAttachmentRepo(),
		PendingAttRepo:     newMockPendingAttachmentRepo(),
		ReactionRepo:       newMockReactionRepo(),
		PrefRepo:           newMockUserPrefRepo(),
		NotifSvc:           newMockNotificationService(),
		ChannelPermService: newMockChannelPermissionService(),
		Broadcaster:        bc,
		Log:                slog.Default(),
	})

	_, err := svc.EditMessage(ctx, domain.EditMessageParams{
		OrgID:    "org-1",
		MsgID:    "msg-1",
		ConvID:   "conv-1",
		EditorID: "user-1",
		Content:  "hello edited",
	})
	if err != nil {
		t.Fatalf("EditMessage: %v", err)
	}

	if len(bc.messages) != 1 {
		t.Fatalf("expected 1 broadcast, got %d", len(bc.messages))
	}

	payloadJSON, err := json.Marshal(bc.messages[0].payload)
	if err != nil {
		t.Fatalf("marshal payload: %v", err)
	}

	payload := string(payloadJSON)

	// Ensure snake_case keys are present in the JSON output.
	if !strings.Contains(payload, "\"conversation_id\"") {
		t.Errorf("expected snake_case key 'conversation_id' in broadcast payload, got: %s", payload)
	}
	if !strings.Contains(payload, "\"sender_id\"") {
		t.Errorf("expected snake_case key 'sender_id' in broadcast payload, got: %s", payload)
	}
	if !strings.Contains(payload, "\"created_at\"") {
		t.Errorf("expected snake_case key 'created_at' in broadcast payload, got: %s", payload)
	}
	// Ensure PascalCase keys from raw domain.Message are NOT present.
	if strings.Contains(payload, "\"ConversationID\"") {
		t.Errorf("found PascalCase key 'ConversationID': raw domain.Message probably leaked")
	}
	if strings.Contains(payload, "\"SenderID\"") {
		t.Errorf("found PascalCase key 'SenderID': raw domain.Message probably leaked")
	}
	if string(domain.WsTypeMessageUpdated) != bc.messages[0].eventType {
		t.Errorf("expected event type %s, got %s", domain.WsTypeMessageUpdated, bc.messages[0].eventType)
	}
}

// TestMessageService_PinMessage_BroadcastsSnakeCase asserts that PinMessage
// broadcasts using the wire format (snake_case json tags).
func TestMessageService_PinMessage_BroadcastsSnakeCase(t *testing.T) {
	ctx := context.Background()
	msgRepo := newMockMessageRepo()
	bc := newMockBroadcaster()

	msg := &domain.Message{
		ID:             "msg-2",
		ConversationID: "conv-1",
		OrgID:          "org-1",
		SenderID:       "user-1",
		Content:        "hello",
		CreatedAt:      time.Now(),
	}
	msgRepo.msgsByID[msg.ID] = msg

	svc := NewMessageService(MessageServiceDeps{
		MsgRepo:            msgRepo,
		ConvRepo:           newMockConversationRepo(),
		OrgRepo:            newMockOrgRepo(true),
		ProjectRepo:        newMockProjectRepo(),
		TaskRepo:           newMockTaskRepo(),
		UserRepo:           newMockUserRepo(),
		AttRepo:            newMockMessageAttachmentRepo(),
		PendingAttRepo:     newMockPendingAttachmentRepo(),
		ReactionRepo:       newMockReactionRepo(),
		PrefRepo:           newMockUserPrefRepo(),
		NotifSvc:           newMockNotificationService(),
		ChannelPermService: newMockChannelPermissionService(),
		Broadcaster:        bc,
		Log:                slog.Default(),
	})

	err := svc.PinMessage(ctx, "org-1", "msg-2", "conv-1", "user-1")
	if err != nil {
		t.Fatalf("PinMessage: %v", err)
	}

	if len(bc.messages) != 1 {
		t.Fatalf("expected 1 broadcast, got %d", len(bc.messages))
	}

	payloadJSON, err := json.Marshal(bc.messages[0].payload)
	if err != nil {
		t.Fatalf("marshal payload: %v", err)
	}

	payload := string(payloadJSON)

	// Ensure snake_case keys are present.
	if !strings.Contains(payload, "\"conversation_id\"") {
		t.Errorf("expected snake_case key 'conversation_id' in broadcast payload, got: %s", payload)
	}
	if !strings.Contains(payload, "\"message\"") {
		t.Errorf("expected key 'message' wrapping the wireMessage, got: %s", payload)
	}
	// Ensure PascalCase keys from raw domain.Message are NOT present.
	if strings.Contains(payload, "\"ConversationID\"") {
		t.Errorf("found PascalCase key 'ConversationID': raw domain.Message probably leaked")
	}
	if strings.Contains(payload, "\"SenderID\"") {
		t.Errorf("found PascalCase key 'SenderID': raw domain.Message probably leaked")
	}
	if string(domain.WsTypeMessagePinned) != bc.messages[0].eventType {
		t.Errorf("expected event type %s, got %s", domain.WsTypeMessagePinned, bc.messages[0].eventType)
	}
}

// TestMessageService_MutationRequiresMembership asserts that reacting to and
// editing a message in a conversation you are not (or no longer) a member of
// is rejected, even when the message UUID is known.
func TestMessageService_MutationRequiresMembership(t *testing.T) {
	ctx := context.Background()
	msgRepo := newMockMessageRepo()
	convRepo := newMockConversationRepo()
	convRepo.Create(ctx, &domain.Conversation{ID: "conv-1", OrgID: "org-1", Type: domain.ConvChannel, Name: "general"})
	convRepo.AddMember(ctx, "org-1", "conv-1", "user-1")

	svc := NewMessageService(MessageServiceDeps{
		MsgRepo:            msgRepo,
		ConvRepo:           convRepo,
		OrgRepo:            newMockOrgRepo(true),
		ProjectRepo:        newMockProjectRepo(),
		TaskRepo:           newMockTaskRepo(),
		UserRepo:           newMockUserRepo(),
		AttRepo:            newMockMessageAttachmentRepo(),
		PendingAttRepo:     newMockPendingAttachmentRepo(),
		ReactionRepo:       newMockReactionRepo(),
		PrefRepo:           newMockUserPrefRepo(),
		NotifSvc:           newMockNotificationService(),
		ChannelPermService: newMockChannelPermissionService(),
		Broadcaster:        newMockBroadcaster(),
		Log:                slog.Default(),
	})

	sent, err := svc.SendMessage(ctx, domain.CreateMessageParams{
		ConversationID: "conv-1",
		OrgID:          "org-1",
		SenderID:       "user-1",
		Content:        "hello",
	})
	if err != nil {
		t.Fatalf("SendMessage: %v", err)
	}

	// user-2 knows the UUID but was never a member.
	if err := svc.AddReaction(ctx, domain.AddReactionParams{
		MsgID: sent.ID, ConvID: "conv-1", UserID: "user-2", OrgID: "org-1", Emoji: "🎉",
	}); err == nil {
		t.Fatal("AddReaction by non-member should fail")
	}
	_, err = svc.EditMessage(ctx, domain.EditMessageParams{
		OrgID: "org-1", MsgID: sent.ID, ConvID: "conv-1", EditorID: "user-2", Content: "hijack",
	})
	if err == nil {
		t.Fatal("EditMessage by non-member sender should fail")
	}

	// The sender who gets removed loses edit/react rights too.
	convRepo.RemoveMember(ctx, "conv-1", "user-1")
	if err := svc.AddReaction(ctx, domain.AddReactionParams{
		MsgID: sent.ID, ConvID: "conv-1", UserID: "user-1", OrgID: "org-1", Emoji: "🎉",
	}); err == nil {
		t.Fatal("AddReaction after removal should fail")
	}
	_, err = svc.EditMessage(ctx, domain.EditMessageParams{
		OrgID: "org-1", MsgID: sent.ID, ConvID: "conv-1", EditorID: "user-1", Content: "still here?",
	})
	if err == nil {
		t.Fatal("EditMessage after removal should fail")
	}
}
