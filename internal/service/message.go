package service

import (
	"context"
	"fmt"
	"log/slog"
	"strings"
	"time"

	"ipmanlk/plume/internal/apperr"
	"ipmanlk/plume/internal/domain"
	"ipmanlk/plume/internal/i18n"
	"ipmanlk/plume/internal/port"

	"github.com/google/uuid"
)

const maxMessageContentLength = 50000

// MessageServiceDeps contains all dependencies for MessageService.
// Use this struct to avoid long parameter lists when constructing MessageService.
type MessageServiceDeps struct {
	MsgRepo            port.MessageRepository
	ConvRepo           port.ConversationRepository
	OrgRepo            port.OrganizationRepository
	ProjectRepo        port.ProjectRepository
	TaskRepo           port.TaskRepository
	UserRepo           port.UserRepository
	AttRepo            port.MessageAttachmentRepository
	PendingAttRepo     port.PendingAttachmentRepository
	ReactionRepo       port.ReactionRepository
	PrefRepo           port.UserChannelPreferenceRepository
	NotifSvc           port.NotificationService
	ChannelPermService port.ChannelPermissionService
	Broadcaster        port.Broadcaster
	UserPrefRepo       port.UserPreferencesRepository
	I18n               *i18n.Bundle
	Log                *slog.Logger
}

type MessageService struct {
	msgRepo            port.MessageRepository
	convRepo           port.ConversationRepository
	orgRepo            port.OrganizationRepository
	projectRepo        port.ProjectRepository
	taskRepo           port.TaskRepository
	userRepo           port.UserRepository
	attRepo            port.MessageAttachmentRepository
	pendingAttRepo     port.PendingAttachmentRepository
	reactionRepo       port.ReactionRepository
	prefRepo           port.UserChannelPreferenceRepository
	notifSvc           port.NotificationService
	channelPermService port.ChannelPermissionService
	broadcaster        port.Broadcaster
	userPrefRepo       port.UserPreferencesRepository
	i18n               *i18n.Bundle
	log                *slog.Logger
	mentions           mentionHydrator
}

// localize is a nil-safe wrapper around i18n.Bundle.MustLocalize. When the
// bundle is nil (e.g. in tests), returns the messageID as a no-op fallback.
func (s *MessageService) localize(locale, messageID string, data map[string]any, pluralCount any) string {
	if s.i18n == nil {
		return messageID
	}
	return s.i18n.MustLocalize(locale, messageID, data, pluralCount)
}

var _ port.MessageService = (*MessageService)(nil)

// localeForUser returns the recipient's preferred locale, falling back to
// the source locale if the preference can't be read or the repo is not
// configured (e.g. in tests).
func (s *MessageService) localeForUser(ctx context.Context, userID string) string {
	if s.userPrefRepo == nil {
		return i18n.SourceLocale
	}
	prefs, err := s.userPrefRepo.Get(ctx, userID)
	if err != nil || prefs == nil {
		return i18n.SourceLocale
	}
	return i18n.Normalize(prefs.Language)
}

// NewMessageService creates a new MessageService with the provided dependencies.
func NewMessageService(deps MessageServiceDeps) *MessageService {
	return &MessageService{
		msgRepo:            deps.MsgRepo,
		convRepo:           deps.ConvRepo,
		orgRepo:            deps.OrgRepo,
		projectRepo:        deps.ProjectRepo,
		taskRepo:           deps.TaskRepo,
		userRepo:           deps.UserRepo,
		attRepo:            deps.AttRepo,
		pendingAttRepo:     deps.PendingAttRepo,
		reactionRepo:       deps.ReactionRepo,
		prefRepo:           deps.PrefRepo,
		notifSvc:           deps.NotifSvc,
		channelPermService: deps.ChannelPermService,
		broadcaster:        deps.Broadcaster,
		userPrefRepo:       deps.UserPrefRepo,
		i18n:               deps.I18n,
		log:                deps.Log,
		mentions:           newMentionHydrator(deps.UserRepo, deps.ProjectRepo, deps.TaskRepo, deps.ConvRepo),
	}
}

func (s *MessageService) ListMessages(ctx context.Context, orgID, convID string, filter domain.MessageFilter) (*domain.MessageListResult, error) {
	if filter.Limit <= 0 || filter.Limit > 100 {
		filter.Limit = 50
	}
	result, err := s.msgRepo.ListByConversation(ctx, orgID, convID, filter)
	if err != nil {
		return nil, err
	}
	for _, m := range result.Items {
		s.resolveForwardedMessage(ctx, m)
	}
	if err := s.hydrateMessageAttachments(ctx, result.Items); err != nil {
		s.log.Error("list messages: hydrate attachments", "error", err)
	}
	s.hydrateMentionsList(ctx, result.Items)
	return result, nil
}

func (s *MessageService) ListReplies(ctx context.Context, orgID, convID, parentID string, filter domain.MessageFilter) (*domain.MessageListResult, error) {
	if filter.Limit <= 0 || filter.Limit > 100 {
		filter.Limit = 50
	}
	result, err := s.msgRepo.ListReplies(ctx, orgID, convID, parentID, filter)
	if err != nil {
		return nil, err
	}
	for _, m := range result.Items {
		s.resolveForwardedMessage(ctx, m)
	}
	if err := s.hydrateMessageAttachments(ctx, result.Items); err != nil {
		s.log.Error("list replies: hydrate attachments", "error", err)
	}
	s.hydrateMentionsList(ctx, result.Items)
	return result, nil
}

// hydrateMentionsList resolves <@type:id> tokens in each message's Content
// into a Mentions payload via the shared mentionHydrator.
func (s *MessageService) hydrateMentionsList(ctx context.Context, messages []*domain.Message) {
	if len(messages) == 0 {
		return
	}
	contents := make([]string, len(messages))
	for i, m := range messages {
		contents[i] = m.Content
	}
	orgID := messages[0].OrgID
	resolved := s.mentions.hydrateMany(ctx, orgID, contents)
	for i, m := range messages {
		m.Mentions = resolved[i]
	}
}

func (s *MessageService) hydrateMessageAttachments(ctx context.Context, messages []*domain.Message) error {
	if len(messages) == 0 {
		return nil
	}
	msgIDs := make([]string, 0, len(messages))
	for _, m := range messages {
		msgIDs = append(msgIDs, m.ID)
	}
	groups, err := s.attRepo.ListByMessages(ctx, msgIDs)
	if err != nil {
		return err
	}
	for _, m := range messages {
		ptrs := groups[m.ID]
		atts := make([]domain.MessageAttachment, 0, len(ptrs))
		for _, a := range ptrs {
			if a != nil {
				atts = append(atts, *a)
			}
		}
		m.Attachments = atts
	}
	return nil
}

func (s *MessageService) resolveForwardedMessage(ctx context.Context, msg *domain.Message) {
	if msg.ForwardedMessageID == nil || *msg.ForwardedMessageID == "" {
		return
	}
	fwd, err := s.msgRepo.GetByID(ctx, *msg.ForwardedMessageID, msg.ConversationID)
	if err != nil {
		fwd, err = s.msgRepo.GetByIDAnyConv(ctx, *msg.ForwardedMessageID)
		if err != nil {
			return
		}
	}
	msg.ForwardedMessage = fwd
	fwdConv, err := s.convRepo.GetByID(ctx, msg.OrgID, fwd.ConversationID)
	if err != nil {
		return
	}
	msg.ForwardedMessageConvName = fwdConv.Name
	msg.ForwardedMessageConvType = fwdConv.Type
	// Check project links for forwarded message
	if len(fwdConv.ProjectIDs) > 0 {
		project, err := s.projectRepo.GetByID(ctx, msg.OrgID, fwdConv.ProjectIDs[0])
		if err == nil && project != nil {
			msg.ForwardedMessageConvProjectName = project.Name
		}
	}
	if msg.ForwardedMessageConvName == "" && msg.ForwardedMessageConvType == domain.ConvDirect {
		members, err := s.convRepo.GetMembers(ctx, fwd.ConversationID)
		if err == nil {
			for _, mem := range members {
				if mem.UserID != fwd.SenderID {
					msg.ForwardedMessageConvName = mem.User.Name
					break
				}
			}
		}
	}
}

// canViewConversation reports whether userID may view the given conversation.
// Prefers channel-permission resolution (which needs the user's org role);
// falls back to explicit membership.
func (s *MessageService) canViewConversation(ctx context.Context, orgID, convID, userID string) bool {
	role := domain.Role("")
	if u, err := s.userRepo.GetByID(ctx, orgID, userID); err == nil && u != nil {
		role = u.Role
	}
	if s.channelPermService != nil && role != "" {
		ok, err := s.channelPermService.UserHasAccess(ctx, orgID, convID, userID, role)
		if err == nil {
			return ok
		}
	}
	isMember, err := s.convRepo.IsMember(ctx, convID, userID)
	return err == nil && isMember
}

// requireConversationMember enforces the same explicit-membership rule that
// SendMessage applies, on the other mutating paths (edit, delete, reactions).
// Without it, a user removed from a channel could keep editing their old
// messages there or react to content they can no longer see.
func (s *MessageService) requireConversationMember(ctx context.Context, convID, userID string) error {
	isMember, err := s.convRepo.IsMember(ctx, convID, userID)
	if err != nil || !isMember {
		return fmt.Errorf("you are not a member of this conversation")
	}
	return nil
}

func (s *MessageService) SendMessage(ctx context.Context, params domain.CreateMessageParams) (*domain.Message, error) {
	if params.Content == "" && len(params.AttachmentIDs) == 0 {
		return nil, apperr.InvalidInput("message content or attachment is required")
	}
	if len(params.Content) > maxMessageContentLength {
		return nil, apperr.InvalidInput("message content must be 50000 characters or less")
	}
	conv, err := s.convRepo.GetByID(ctx, params.OrgID, params.ConversationID)
	if err != nil {
		return nil, fmt.Errorf("conversation not found: %w", err)
	}
	isMember, err := s.convRepo.IsMember(ctx, params.ConversationID, params.SenderID)
	if err != nil || !isMember {
		return nil, fmt.Errorf("you are not a member of this conversation")
	}
	if params.ParentID != nil && *params.ParentID != "" {
		parent, err := s.msgRepo.GetByID(ctx, *params.ParentID, params.ConversationID)
		if err != nil {
			return nil, fmt.Errorf("parent message not found: %w", err)
		}
		params.ParentID = &parent.ID
	}
	if params.ForwardedMessageID != nil && *params.ForwardedMessageID != "" {
		if _, err := s.msgRepo.GetByID(ctx, *params.ForwardedMessageID, params.ConversationID); err != nil {
			fwdSrc, ferr := s.msgRepo.GetByIDAnyConv(ctx, *params.ForwardedMessageID)
			if ferr != nil {
				return nil, fmt.Errorf("forwarded message not found: %w", ferr)
			}
			// Tenant guard: the source must live in the sender's org AND in
			// a conversation the sender can actually view. Without this,
			// anyone who learns a foreign message UUID could copy its full
			// content into their own channel.
			if fwdSrc.OrgID != params.OrgID {
				return nil, fmt.Errorf("forwarded message not found")
			}
			if !s.canViewConversation(ctx, params.OrgID, fwdSrc.ConversationID, params.SenderID) {
				return nil, fmt.Errorf("forwarded message not found")
			}
		}
	}
	now := time.Now()
	msg := &domain.Message{
		ID:                 uuid.New().String(),
		ConversationID:     params.ConversationID,
		OrgID:              params.OrgID,
		SenderID:           params.SenderID,
		Content:            params.Content,
		ParentID:           params.ParentID,
		ForwardedMessageID: params.ForwardedMessageID,
		CreatedAt:          now,
	}
	if err := s.msgRepo.Create(ctx, msg); err != nil {
		return nil, err
	}
	msgAttachments := make([]domain.MessageAttachment, 0, len(params.AttachmentIDs))
	for _, attID := range params.AttachmentIDs {
		// Scoped by uploader: a sender can only attach pending uploads they
		// created themselves, never another user's (or another
		// conversation's) upload ID.
		pending, err := s.pendingAttRepo.GetByID(ctx, attID, params.SenderID)
		if err != nil {
			s.log.Error("pending attachment not found, skipping", "attachment_id", attID, "error", err)
			continue
		}
		att := &domain.MessageAttachment{
			ID:          pending.ID,
			MessageID:   msg.ID,
			FileName:    pending.FileName,
			FileSize:    pending.FileSize,
			ContentType: pending.ContentType,
			StoragePath: pending.StoragePath,
		}
		if err := s.attRepo.Create(ctx, att); err != nil {
			s.log.Error("failed to persist attachment, skipping", "attachment_id", attID, "error", err)
			continue
		}
		if err := s.pendingAttRepo.Delete(ctx, attID); err != nil {
			s.log.Error("failed to delete pending attachment", "attachment_id", attID, "error", err)
		}
		att.CreatedAt = time.Now()
		msgAttachments = append(msgAttachments, *att)
	}
	msg.Attachments = msgAttachments
	msg.Sender, _ = s.userRepo.GetByID(ctx, params.OrgID, params.SenderID)
	s.hydrateMentionsList(ctx, []*domain.Message{msg})
	// Bump conversation updated_at so it rises in the sidebar list.
	// conv already has correct Name/Topic from the earlier GetByID call.
	_ = s.convRepo.Update(ctx, conv)
	m := buildWireMessage(msg)
	s.broadcaster.Broadcast(
		domain.RoomKeyConversation(params.OrgID, params.ConversationID),
		string(domain.WsTypeMessageNew),
		messageCreatedPayload{Message: m, ConversationID: params.ConversationID},
	)
	s.sendNotifications(ctx, msg, conv)
	return msg, nil
}

func (s *MessageService) EditMessage(ctx context.Context, p domain.EditMessageParams) (*domain.Message, error) {
	if p.Content == "" {
		return nil, fmt.Errorf("message content is required")
	}
	if len(p.Content) > maxMessageContentLength {
		return nil, fmt.Errorf("message content must be 50000 characters or less")
	}
	msg, err := s.msgRepo.GetByID(ctx, p.MsgID, p.ConvID)
	if err != nil {
		return nil, fmt.Errorf("message not found: %w", err)
	}
	if msg.SenderID != p.EditorID {
		return nil, fmt.Errorf("you can only edit your own messages")
	}
	if err := s.requireConversationMember(ctx, p.ConvID, p.EditorID); err != nil {
		return nil, err
	}
	org, err := s.orgRepo.GetByID(ctx, p.OrgID)
	if err == nil && org.MessageEditWindowMinute > 0 {
		elapsed := time.Since(msg.CreatedAt)
		if elapsed > time.Duration(org.MessageEditWindowMinute)*time.Minute {
			return nil, fmt.Errorf("edit window of %d minutes has passed", org.MessageEditWindowMinute)
		}
	}
	now := time.Now()
	msg.Content = p.Content
	msg.EditedAt = &now
	if err := s.msgRepo.Update(ctx, msg); err != nil {
		return nil, err
	}
	msg, _ = s.msgRepo.GetByID(ctx, p.MsgID, p.ConvID)
	_ = s.hydrateMessageAttachments(ctx, []*domain.Message{msg})
	s.hydrateMentionsList(ctx, []*domain.Message{msg})
	m := buildWireMessage(msg)
	s.broadcaster.Broadcast(
		domain.RoomKeyConversation(p.OrgID, p.ConvID),
		string(domain.WsTypeMessageUpdated),
		messageCreatedPayload{Message: m, ConversationID: p.ConvID},
	)
	return msg, nil
}

func (s *MessageService) DeleteMessage(ctx context.Context, orgID, msgID, convID, deleterID string) error {
	msg, err := s.msgRepo.GetByID(ctx, msgID, convID)
	if err != nil {
		return fmt.Errorf("message not found: %w", err)
	}
	if msg.SenderID != deleterID {
		return fmt.Errorf("you can only delete your own messages")
	}
	if err := s.requireConversationMember(ctx, convID, deleterID); err != nil {
		return err
	}
	if err := s.msgRepo.SoftDelete(ctx, msgID, convID); err != nil {
		return err
	}
	if s.broadcaster != nil {
		s.broadcaster.Broadcast(
			domain.RoomKeyConversation(orgID, convID),
			string(domain.WsTypeMessageDeleted),
			map[string]any{
				"message_id":      msgID,
				"conversation_id": convID,
			},
		)
	}
	return nil
}

func (s *MessageService) PinMessage(ctx context.Context, orgID, msgID, convID, pinnerID string) error {
	msg, err := s.msgRepo.GetByID(ctx, msgID, convID)
	if err != nil {
		return fmt.Errorf("message not found: %w", err)
	}
	if err := s.msgRepo.Pin(ctx, msg.ID, convID, pinnerID); err != nil {
		return err
	}
	msg.Pinned = true
	m := buildWireMessage(msg)
	s.broadcaster.Broadcast(
		domain.RoomKeyConversation(orgID, convID),
		string(domain.WsTypeMessagePinned),
		messageCreatedPayload{Message: m, ConversationID: convID},
	)
	return nil
}

func (s *MessageService) UnpinMessage(ctx context.Context, orgID, msgID, convID string) error {
	if err := s.msgRepo.Unpin(ctx, msgID, convID); err != nil {
		return err
	}
	if s.broadcaster != nil {
		s.broadcaster.Broadcast(
			domain.RoomKeyConversation(orgID, convID),
			string(domain.WsTypeMessageUnpinned),
			map[string]any{
				"message_id":      msgID,
				"conversation_id": convID,
			},
		)
	}
	return nil
}

func (s *MessageService) sendNotifications(ctx context.Context, msg *domain.Message, conv *domain.Conversation) {
	members, err := s.convRepo.GetMembers(ctx, msg.ConversationID)
	if err != nil {
		return
	}

	hasEveryoneMention := containsEveryoneMention(msg.Content)
	mentionedUserIDs := extractMentionedUserIDs(msg.Content)
	notifiedUsers := map[string]bool{msg.SenderID: true}

	senderName := "Someone"
	if msg.Sender != nil {
		senderName = msg.Sender.Name
	}

	convLabel := conv.Name
	if conv.Type == domain.ConvChannel && convLabel != "" {
		convLabel = "#" + convLabel
	}

	// For @everyone in channels, get all users with access (not just explicit members)
	var accessEntries []*domain.ChannelAccessEntry
	if conv.Type == domain.ConvChannel && hasEveryoneMention && s.channelPermService != nil {
		accessEntries, _ = s.channelPermService.ListAccess(ctx, conv.OrgID, msg.ConversationID)
	}

	// Build a unified list of users to check for notifications
	usersToCheck := make(map[string]bool)
	for _, m := range members {
		usersToCheck[m.UserID] = true
	}
	for _, entry := range accessEntries {
		if entry.User != nil {
			usersToCheck[entry.User.ID] = true
		}
	}

	for userID := range usersToCheck {
		if userID == msg.SenderID {
			continue
		}

		pref, _ := s.prefRepo.Get(ctx, userID, msg.ConversationID)
		if pref != nil {
			if pref.Muted {
				continue
			}
			if pref.NotificationLevel == domain.NotifLevelNothing {
				continue
			}
		}

		// DMs/groups notify all members (subject to per-user mute)
		if conv.Type == domain.ConvDirect || conv.Type == domain.ConvGroup {
			loc := s.localeForUser(ctx, userID)
			s.notifSvc.Notify(ctx, msg.OrgID, userID, domain.NotifChatDM,
				s.localize(loc, "ChatDMTitle", map[string]any{"Sender": senderName}, nil),
				msg.Content,
				fmt.Sprintf("/chat/%s", msg.ConversationID),
				"chat", msg.ConversationID,
				msg.SenderID,
			)
			notifiedUsers[userID] = true
			continue
		}

		// Channels: only notify on @user mentions, @everyone, or reply-to-me
		isMentioned := mentionedUserIDs[userID]
		if conv.Type == domain.ConvChannel {
			if hasEveryoneMention || isMentioned {
				loc := s.localeForUser(ctx, userID)
				s.notifSvc.Notify(ctx, msg.OrgID, userID, domain.NotifChatMention,
					s.localize(loc, "ChatMentionTitle", nil, nil),
					s.localize(loc, "ChatMentionBody", map[string]any{"Sender": senderName, "Conversation": convLabel}, nil),
					fmt.Sprintf("/chat/%s", msg.ConversationID),
					"chat", msg.ConversationID,
					msg.SenderID,
				)
				notifiedUsers[userID] = true
			}
		}
	}

	s.notifyReplyParent(ctx, msg, conv, senderName, convLabel, notifiedUsers)
}

// notifyReplyParent sends a notification to the parent message's author
// (if not already notified and not muted). Only for channel replies.
func (s *MessageService) notifyReplyParent(ctx context.Context, msg *domain.Message, conv *domain.Conversation, senderName, convLabel string, notifiedUsers map[string]bool) {
	if msg.ParentID == nil || *msg.ParentID == "" {
		return
	}
	parent, err := s.msgRepo.GetByID(ctx, *msg.ParentID, msg.ConversationID)
	if err != nil {
		return
	}
	if parent.SenderID == msg.SenderID || notifiedUsers[parent.SenderID] {
		return
	}
	pref, _ := s.prefRepo.Get(ctx, parent.SenderID, msg.ConversationID)
	if pref != nil && (pref.Muted || pref.NotificationLevel == domain.NotifLevelNothing) {
		return
	}
	loc := s.localeForUser(ctx, parent.SenderID)
	s.notifSvc.Notify(ctx, msg.OrgID, parent.SenderID, domain.NotifChatMention,
		s.localize(loc, "ChatMentionTitle", nil, nil),
		s.localize(loc, "ChatMentionBody", map[string]any{"Sender": senderName, "Conversation": convLabel}, nil),
		fmt.Sprintf("/chat/%s", msg.ConversationID),
		"chat", msg.ConversationID,
		msg.SenderID,
	)
}

// containsEveryoneMention matches the canonical <@everyone> token only.
// A bare "@everyone" substring in prose (e.g. "email me@everyone.example")
// must not page the whole channel.
func containsEveryoneMention(content string) bool {
	for _, t := range domain.ParseMentionTokens(content) {
		if t.Type == string(domain.MentionEveryone) {
			return true
		}
	}
	return false
}

func extractMentionedUserIDs(content string) map[string]bool {
	out := map[string]bool{}
	for _, t := range domain.ParseMentionTokens(content) {
		if t.Type == string(domain.MentionUser) && t.ID != "" {
			out[t.ID] = true
		}
	}
	return out
}

func (s *MessageService) AddReaction(ctx context.Context, p domain.AddReactionParams) error {
	// Reactions follow the same membership rule as sending: reacting to a
	// message in a conversation you were removed from (or were never part
	// of) is not allowed, even if you can still guess message UUIDs.
	if err := s.requireConversationMember(ctx, p.ConvID, p.UserID); err != nil {
		return err
	}
	// The reaction must land on a message that actually exists in THIS
	// conversation (which is org-scoped upstream). Without this check the
	// insert would bind an arbitrary foreign message ID to this org.
	if _, err := s.msgRepo.GetByID(ctx, p.MsgID, p.ConvID); err != nil {
		return fmt.Errorf("message not found: %w", err)
	}
	if err := s.reactionRepo.Add(ctx, p.OrgID, p.MsgID, p.UserID, p.Emoji); err != nil {
		return err
	}
	if s.broadcaster != nil {
		_ = s.broadcaster.Broadcast(
			domain.RoomKeyConversation(p.OrgID, p.ConvID),
			string(domain.WsTypeMessageReactionAdded),
			map[string]any{
				"message_id":      p.MsgID,
				"conversation_id": p.ConvID,
				"user_id":         p.UserID,
				"emoji":           p.Emoji,
			},
		)
	}
	return nil
}

func (s *MessageService) SearchMessages(ctx context.Context, orgID, userID string, filter domain.MessageSearchFilter) (*domain.MessageSearchListResult, error) {
	if filter.Limit <= 0 || filter.Limit > 100 {
		filter.Limit = 20
	}
	if strings.TrimSpace(filter.Query) == "" {
		return &domain.MessageSearchListResult{Items: []*domain.MessageSearchResult{}}, nil
	}
	result, err := s.msgRepo.SearchMessages(ctx, orgID, userID, filter)
	if err != nil {
		// FTS5 syntax errors are common with user input. Surface a user-facing
		// error so the frontend can inform the user their query is invalid,
		// rather than silently returning empty results.
		if strings.Contains(err.Error(), "fts5") && strings.Contains(err.Error(), "syntax error") {
			return nil, apperr.InvalidInput("invalid search query syntax")
		}
		return nil, err
	}
	for _, m := range result.Items {
		s.resolveForwardedMessage(ctx, m.Message)
	}
	searchMsgs := make([]*domain.Message, 0, len(result.Items))
	for _, r := range result.Items {
		searchMsgs = append(searchMsgs, r.Message)
	}
	if err := s.hydrateMessageAttachments(ctx, searchMsgs); err != nil {
		s.log.Error("search messages: hydrate attachments", "error", err)
	}
	return result, nil
}

func (s *MessageService) RemoveReaction(ctx context.Context, p domain.RemoveReactionParams) error {
	if err := s.requireConversationMember(ctx, p.ConvID, p.UserID); err != nil {
		return err
	}
	// Silently succeed if reaction doesn't exist (idempotent), but surface
	// unexpected DB failures instead of broadcasting a removal that never
	// happened.
	removed, err := s.reactionRepo.Remove(ctx, p.MsgID, p.UserID, p.Emoji)
	if err != nil {
		return err
	}
	if !removed {
		return nil
	}

	if s.broadcaster != nil {
		_ = s.broadcaster.Broadcast(
			domain.RoomKeyConversation(p.OrgID, p.ConvID),
			string(domain.WsTypeMessageReactionRemoved),
			map[string]any{
				"message_id":      p.MsgID,
				"conversation_id": p.ConvID,
				"user_id":         p.UserID,
				"emoji":           p.Emoji,
			},
		)
	}
	return nil
}
