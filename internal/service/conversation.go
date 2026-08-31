package service

import (
	"context"
	"fmt"
	"log/slog"
	"slices"
	"strings"
	"time"

	"ipmanlk/plume/internal/apperr"
	"ipmanlk/plume/internal/domain"
	"ipmanlk/plume/internal/lexorank"
	"ipmanlk/plume/internal/port"

	"github.com/google/uuid"
)

type ConversationServiceDeps struct {
	ConvRepo       port.ConversationRepository
	UserRepo       port.UserRepository
	MsgRepo        port.MessageRepository
	PrefRepo       port.UserChannelPreferenceRepository
	LinkRepo       port.ChannelProjectLinkRepository
	PermRepo       port.ChannelPermissionRepository
	NotifSvc       port.NotificationService
	ChannelPermSvc port.ChannelPermissionService
	Broadcaster    port.Broadcaster
	Logger         *slog.Logger
}

type ConversationService struct {
	convRepo       port.ConversationRepository
	userRepo       port.UserRepository
	msgRepo        port.MessageRepository
	prefRepo       port.UserChannelPreferenceRepository
	linkRepo       port.ChannelProjectLinkRepository
	permRepo       port.ChannelPermissionRepository
	notifSvc       port.NotificationService
	channelPermSvc port.ChannelPermissionService
	broadcaster    port.Broadcaster
	log            *slog.Logger
}

var _ port.ConversationService = (*ConversationService)(nil)

func NewConversationService(deps ConversationServiceDeps) *ConversationService {
	log := deps.Logger
	if log == nil {
		log = slog.Default()
	}
	return &ConversationService{
		convRepo:       deps.ConvRepo,
		userRepo:       deps.UserRepo,
		msgRepo:        deps.MsgRepo,
		prefRepo:       deps.PrefRepo,
		linkRepo:       deps.LinkRepo,
		permRepo:       deps.PermRepo,
		notifSvc:       deps.NotifSvc,
		channelPermSvc: deps.ChannelPermSvc,
		broadcaster:    deps.Broadcaster,
		log:            log,
	}
}

func (s *ConversationService) ListMyConversations(ctx context.Context, orgID, userID string, filter domain.ConversationFilter) (*domain.ConversationListResult, error) {
	if filter.Limit <= 0 || filter.Limit > 50 {
		filter.Limit = 20
	}
	result, err := s.convRepo.ListByUser(ctx, orgID, userID, filter)
	if err != nil {
		return nil, err
	}
	s.enrichConversations(ctx, orgID, userID, result.Items)
	return result, nil
}

func (s *ConversationService) ListByParent(ctx context.Context, orgID, parentID, userID string, role domain.Role, includeProjectLinked bool) ([]*domain.Conversation, error) {
	convs, err := s.convRepo.ListByParent(ctx, orgID, parentID, userID, includeProjectLinked)
	if err != nil {
		return nil, err
	}

	// Filter out children the caller is explicitly denied access to via
	// channel-level permission overrides (e.g. a viewer who was denied
	// channel:view on a specific child channel).
	filtered := make([]*domain.Conversation, 0, len(convs))
	for _, c := range convs {
		ok, err := s.channelPermSvc.UserHasAccess(ctx, orgID, c.ID, userID, role)
		if err != nil {
			s.log.Warn("list by parent: UserHasAccess check failed, skipping child", "channel_id", c.ID, "error", err)
			continue
		}
		if ok {
			filtered = append(filtered, c)
		}
	}

	s.enrichConversations(ctx, orgID, userID, filtered)
	return filtered, nil
}

// enrichConversations batch-loads the per-conversation enrichment (last
// message, unread count, preference, members, project links) in a constant
// number of queries instead of one round-trip per conversation.
func (s *ConversationService) enrichConversations(ctx context.Context, orgID, userID string, convs []*domain.Conversation) {
	if len(convs) == 0 {
		return
	}
	convIDs := make([]string, len(convs))
	for i, c := range convs {
		convIDs[i] = c.ID
	}

	lastMsgs, _ := s.msgRepo.GetLastMessagesForConversations(ctx, convIDs)
	unreadMap, _ := s.convRepo.UnreadCounts(ctx, userID, convIDs)

	// DM/group member enrichment is still per-conversation (rare in pages and
	// requires partner resolution), but last-message + unread are the hot path.
	for _, c := range convs {
		if last, ok := lastMsgs[c.ID]; ok && last != nil {
			c.LastMessage = last
		} else {
			last, _ := s.msgRepo.GetConversationLastMessage(ctx, c.ID)
			if last != nil {
				c.LastMessage = last
			}
		}
		c.UnreadCount = unreadMap[c.ID]
		pref, _ := s.prefRepo.Get(ctx, userID, c.ID)
		if pref != nil {
			c.Muted = pref.Muted
			c.NotifLevel = pref.NotificationLevel
		}
		if c.Type == domain.ConvDirect || c.Type == domain.ConvGroup {
			members, _ := s.convRepo.GetMembers(ctx, c.ID)
			if c.Type == domain.ConvDirect {
				for _, m := range members {
					if m.UserID != userID {
						c.PartnerUserID = m.UserID
						if m.User != nil {
							c.PartnerName = m.User.Name
						}
						break
					}
				}
			} else {
				names := make([]string, 0, len(members)-1)
				for _, m := range members {
					if m.UserID != userID && m.User != nil {
						names = append(names, m.User.Name)
					}
				}
				if len(names) > 0 {
					c.PartnerName = truncateParticipantNames(names, 100)
				}
			}
		}
		if c.Type == domain.ConvChannel || c.Type == domain.ConvCategory || c.Type == domain.ConvVoice {
			projectIDs, _ := s.linkRepo.GetByChannel(ctx, c.ID)
			c.ProjectIDs = projectIDs
		}
	}
}

func (s *ConversationService) GetByID(ctx context.Context, orgID, id, userID string) (*domain.Conversation, error) {
	conv, err := s.convRepo.GetByIDWithMember(ctx, orgID, id, userID)
	if err != nil {
		return nil, err
	}
	pref, _ := s.prefRepo.Get(ctx, userID, id)
	if pref != nil {
		conv.Muted = pref.Muted
		conv.NotifLevel = pref.NotificationLevel
	}
	projectIDs, _ := s.linkRepo.GetByChannel(ctx, id)
	conv.ProjectIDs = projectIDs
	return conv, nil
}

func (s *ConversationService) CreateChannel(ctx context.Context, params domain.CreateConversationParams) (*domain.Conversation, error) {
	if params.Name == "" {
		return nil, fmt.Errorf("channel name is required")
	}
	if len(params.Name) > 100 {
		return nil, fmt.Errorf("channel name must be 100 characters or less")
	}
	positionKey := params.PositionKey
	if positionKey == "" {
		next, err := s.nextPositionKey(ctx, params.OrgID, params.ParentID)
		if err != nil {
			return nil, err
		}
		positionKey = next
	}
	conv := &domain.Conversation{
		ID:          uuid.New().String(),
		OrgID:       params.OrgID,
		ParentID:    params.ParentID,
		Name:        params.Name,
		Topic:       params.Topic,
		Type:        params.Type,
		CreatedBy:   params.CreatedBy,
		PositionKey: positionKey,
		CreatedAt:   time.Now(),
		UpdatedAt:   time.Now(),
	}
	if err := s.convRepo.CreateWithMembers(ctx, conv, []string{params.CreatedBy}); err != nil {
		return nil, err
	}
	for _, pid := range params.ProjectIDs {
		if err := s.linkRepo.Create(ctx, conv.ID, pid); err != nil {
			return nil, fmt.Errorf("create project link: %w", err)
		}
	}

	// When a channel is linked to projects, auto-configure role permissions
	// so viewers and guests are denied access by default. Only explicit
	// channel members and org members with project access (owner/admin/member)
	// will be able to view the channel via the UserHasAccess project-link check.
	if len(params.ProjectIDs) > 0 && s.permRepo != nil {
		_ = s.permRepo.SetPermissions(ctx, conv.ID, []*domain.PermissionRule{
			{Role: domain.RoleViewer, Permission: domain.PermChannelView, Allow: false},
			{Role: domain.RoleGuest, Permission: domain.PermChannelView, Allow: false},
		})
	}

	return conv, nil
}

func (s *ConversationService) nextPositionKey(ctx context.Context, orgID string, parentID *string) (string, error) {
	keys, err := s.convRepo.ListSiblingPositionKeys(ctx, orgID, parentID)
	if err != nil {
		return "", err
	}
	if len(keys) == 0 {
		return lexorank.FirstKey(), nil
	}
	return lexorank.GenerateKeyBetween(keys[len(keys)-1], "")
}

func (s *ConversationService) CreateDM(ctx context.Context, orgID, createdBy, targetUserID string) (*domain.Conversation, error) {
	if createdBy == targetUserID {
		return nil, fmt.Errorf("cannot create DM with yourself")
	}
	// Validate the target belongs to the caller's org. The users table is
	// global (an account can have memberships in multiple orgs), so the
	// conversation_members FK would otherwise accept a cross-org user ID and
	// leak the DM's contents to a foreign-org membership.
	if _, err := s.userRepo.GetByID(ctx, orgID, targetUserID); err != nil {
		return nil, apperr.InvalidInput("target user is not a member of this organization")
	}
	existing, err := s.convRepo.GetDMByUsers(ctx, orgID, createdBy, targetUserID)
	if err == nil && existing != nil {
		return existing, nil
	}
	conv := &domain.Conversation{
		ID:          uuid.New().String(),
		OrgID:       orgID,
		Type:        domain.ConvDirect,
		Name:        "",
		CreatedBy:   createdBy,
		PositionKey: lexorank.FirstKey(),
		CreatedAt:   time.Now(),
		UpdatedAt:   time.Now(),
	}
	if err := s.convRepo.CreateWithMembers(ctx, conv, []string{createdBy, targetUserID}); err != nil {
		return nil, err
	}
	return conv, nil
}

func (s *ConversationService) CreateGroupDM(ctx context.Context, orgID, createdBy string, memberIDs []string) (*domain.Conversation, error) {
	allMembers := append([]string{createdBy}, memberIDs...)
	allMembers = slices.Compact(allMembers)
	if len(allMembers) < 3 {
		return nil, fmt.Errorf("group DM requires at least 2 other members")
	}
	// Validate every member belongs to the caller's org before creating the
	// conversation. Same cross-org-leak concern as CreateDM/AddMembers.
	for _, uid := range memberIDs {
		if _, err := s.userRepo.GetByID(ctx, orgID, uid); err != nil {
			return nil, apperr.InvalidInput("user " + uid + " is not a member of this organization")
		}
	}
	conv := &domain.Conversation{
		ID:          uuid.New().String(),
		OrgID:       orgID,
		Type:        domain.ConvGroup,
		Name:        "",
		CreatedBy:   createdBy,
		PositionKey: lexorank.FirstKey(),
		CreatedAt:   time.Now(),
		UpdatedAt:   time.Now(),
	}
	if err := s.convRepo.CreateWithMembers(ctx, conv, allMembers); err != nil {
		return nil, err
	}
	return conv, nil
}

func (s *ConversationService) UpdateConversation(ctx context.Context, conv *domain.Conversation) error {
	if conv.Name == "" && conv.Type == domain.ConvChannel {
		return fmt.Errorf("channel name is required")
	}
	if len(conv.Name) > 100 {
		return fmt.Errorf("conversation name must be 100 characters or less")
	}
	if len(conv.Topic) > 500 {
		return fmt.Errorf("topic must be 500 characters or less")
	}
	return s.convRepo.Update(ctx, conv)
}

func (s *ConversationService) UpdateChannelParent(ctx context.Context, orgID, id string, parentID *string, positionKey string) error {
	return s.convRepo.UpdateParent(ctx, orgID, id, parentID, positionKey)
}

func (s *ConversationService) DeleteConversation(ctx context.Context, orgID, id, userID string, callerRole domain.Role) error {
	conv, err := s.convRepo.GetByID(ctx, orgID, id)
	if err != nil {
		return err
	}
	if conv.DeletedAt != nil {
		return fmt.Errorf("conversation already deleted")
	}
	// Channels can be deleted by their creator or by an elevated org role
	// (owner/admin). DMs and group DMs have no ownership model; any
	// participant may delete them.
	if conv.Type == domain.ConvChannel && conv.CreatedBy != userID {
		if callerRole != domain.RoleOwner && callerRole != domain.RoleAdmin {
			return apperr.ErrForbidden
		}
	}
	if conv.Type == domain.ConvCategory {
		if err := s.convRepo.SoftDeleteByParent(ctx, orgID, id); err != nil {
			return err
		}
	}
	return s.convRepo.Delete(ctx, orgID, id)
}

func (s *ConversationService) AddMembers(ctx context.Context, orgID, convID, adderID string, memberIDs []string) error {
	conv, err := s.convRepo.GetByID(ctx, orgID, convID)
	if err != nil {
		return err
	}
	if conv.Type == domain.ConvDirect {
		return fmt.Errorf("cannot add members to a direct message")
	}
	for _, uid := range memberIDs {
		// Validate the user belongs to this org before adding them. Without
		// this, a caller could add a user from another org (the users table is
		// global; the conversation_members FK only checks user existence, not
		// org membership), leaking the conversation's messages cross-org.
		if _, err := s.userRepo.GetByID(ctx, orgID, uid); err != nil {
			return apperr.InvalidInput("user " + uid + " is not a member of this organization")
		}
		if err := s.convRepo.AddMember(ctx, orgID, convID, uid); err != nil {
			return fmt.Errorf("add member %s: %w", uid, err)
		}
	}
	return nil
}

func (s *ConversationService) RemoveMember(ctx context.Context, orgID, convID, removerID, targetID string) error {
	conv, err := s.convRepo.GetByID(ctx, orgID, convID)
	if err != nil {
		return err
	}
	if conv.Type == domain.ConvDirect {
		return fmt.Errorf("cannot remove members from a direct message")
	}
	if removerID != conv.CreatedBy && removerID != targetID {
		return apperr.ErrForbidden
	}
	return s.convRepo.RemoveMember(ctx, convID, targetID)
}

func (s *ConversationService) GetMembers(ctx context.Context, convID string) ([]*domain.ConversationMember, error) {
	return s.convRepo.GetMembers(ctx, convID)
}

func (s *ConversationService) MarkRead(ctx context.Context, convID, userID string) error {
	_ = s.prefRepo.UpdateLastRead(ctx, userID, convID)
	return s.convRepo.UpdateReadState(ctx, convID, userID)
}

func (s *ConversationService) SetMuted(ctx context.Context, orgID, convID, userID string, muted bool) error {
	conv, err := s.convRepo.GetByID(ctx, orgID, convID)
	if err != nil {
		return err
	}
	if err := s.prefRepo.SetMuted(ctx, userID, convID, conv.OrgID, muted); err != nil {
		return err
	}
	if err := s.convRepo.UpdateReadState(ctx, convID, userID); err != nil {
	}
	return nil
}

func (s *ConversationService) SetNotificationLevel(ctx context.Context, orgID, convID, userID string, level domain.NotificationLevel) error {
	conv, err := s.convRepo.GetByID(ctx, orgID, convID)
	if err != nil {
		return err
	}
	return s.prefRepo.SetNotificationLevel(ctx, userID, convID, conv.OrgID, level)
}

func (s *ConversationService) GetPinnedMessages(ctx context.Context, convID string, limit int) ([]*domain.Message, error) {
	if limit <= 0 || limit > 50 {
		limit = 10
	}
	return s.convRepo.ListPinnedMessages(ctx, convID, limit)
}

func (s *ConversationService) EnsureGeneralChannel(ctx context.Context, orgID, userID string) error {
	scope := "workspace"
	filter := domain.ConversationFilter{Scope: &scope, Limit: 1}
	existing, err := s.convRepo.ListByUser(ctx, orgID, userID, filter)
	if err == nil && existing != nil && len(existing.Items) > 0 {
		return nil
	}
	name := "general"
	_, err = s.CreateChannel(ctx, domain.CreateConversationParams{
		OrgID:     orgID,
		Name:      name,
		Type:      domain.ConvChannel,
		CreatedBy: userID,
	})
	return err
}

func (s *ConversationService) GetChannelProjectLinks(ctx context.Context, channelID string) ([]string, error) {
	return s.linkRepo.GetByChannel(ctx, channelID)
}

func (s *ConversationService) SetChannelProjectLinks(ctx context.Context, channelID string, projectIDs []string) error {
	return s.linkRepo.SetProjectLinks(ctx, channelID, projectIDs)
}

func (s *ConversationService) ListAccess(ctx context.Context, orgID, channelID string) ([]*domain.ChannelAccessEntry, error) {
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

	projectIDs, err := s.linkRepo.GetByChannel(ctx, channelID)
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

// truncateParticipantNames builds a participant display string that fits within maxLen.
// If the full joined list exceeds maxLen, it shows as many names as fit and appends
// " & N more" for the remainder.
func truncateParticipantNames(names []string, maxLen int) string {
	full := strings.Join(names, ", ")
	if len(full) <= maxLen {
		return full
	}
	suffix := " & %d more"
	prefix := ""
	for i, name := range names {
		candidate := prefix
		if i > 0 {
			candidate += ", "
		}
		candidate += name
		remaining := len(names) - i - 1
		withSuffix := fmt.Sprintf("%s"+suffix, candidate, remaining)
		if len(withSuffix) <= maxLen {
			prefix = candidate
		} else {
			return fmt.Sprintf("%s"+suffix, prefix, remaining+1)
		}
	}
	return full
}
