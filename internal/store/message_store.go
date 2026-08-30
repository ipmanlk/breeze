package store

import (
	"context"
	"encoding/base64"
	"encoding/json"
	"fmt"
	"time"

	"ipmanlk/breeze/internal/domain"
	"ipmanlk/breeze/internal/port"
	"ipmanlk/breeze/internal/store/sqlc"
)

var _ port.MessageRepository = (*MessageStore)(nil)
var _ port.ReactionRepository = (*ReactionStore)(nil)

type MessageStore struct {
	q *sqlc.Queries
}

func NewMessageStore(q *sqlc.Queries) *MessageStore {
	return &MessageStore{q: q}
}

type msgCursor struct {
	C string `json:"c"`
	I string `json:"i"`
}

func encodeMsgCursor(createdAt time.Time, id string) string {
	b, _ := json.Marshal(msgCursor{C: createdAt.Format("2006-01-02 15:04:05"), I: id})
	return base64.URLEncoding.EncodeToString(b)
}

func decodeMsgCursor(s string) (createdAt, id string, err error) {
	if s == "" {
		return "", "", nil
	}
	b, err := base64.URLEncoding.DecodeString(s)
	if err != nil {
		return "", "", err
	}
	var c msgCursor
	if err := json.Unmarshal(b, &c); err != nil {
		return "", "", err
	}
	return c.C, c.I, nil
}

func (s *MessageStore) ListByConversation(ctx context.Context, orgID, convID string, filter domain.MessageFilter) (*domain.MessageListResult, error) {
	cursorCreatedAt, cursorID, err := decodeMsgCursor(filter.Before)
	if err != nil {
		return nil, err
	}
	limit := filter.Limit
	if limit <= 0 {
		limit = 50
	}
	rows, err := s.q.ListMessages(ctx, sqlc.ListMessagesParams{
		ConversationID:  convID,
		OrgID:           orgID,
		CursorCreatedAt: cursorCreatedAt,
		CursorID:        cursorID,
		LimitVal:        int64(limit + 1),
	})
	if err != nil {
		return nil, err
	}
	hasMore := len(rows) > limit
	if hasMore {
		rows = rows[:limit]
	}
	// Reverse DESC query result into chronological order (oldest first)
	items := make([]*domain.Message, 0, len(rows))
	for i := len(rows) - 1; i >= 0; i-- {
		items = append(items, s.listRowToDomain(rows[i]))
	}
	nextCursor := ""
	if len(items) > 0 {
		oldest := items[0]
		nextCursor = encodeMsgCursor(oldest.CreatedAt, oldest.ID)
	}
	return &domain.MessageListResult{Items: items, NextCursor: nextCursor, HasMore: hasMore}, nil
}

func (s *MessageStore) ListReplies(ctx context.Context, orgID, convID, parentID string, filter domain.MessageFilter) (*domain.MessageListResult, error) {
	cursorCreatedAt, cursorID, err := decodeMsgCursor(filter.Before)
	if err != nil {
		return nil, err
	}
	limit := filter.Limit
	if limit <= 0 {
		limit = 50
	}
	rows, err := s.q.ListReplies(ctx, sqlc.ListRepliesParams{
		ConversationID:  convID,
		ParentID:        &parentID,
		OrgID:           orgID,
		CursorCreatedAt: cursorCreatedAt,
		CursorID:        cursorID,
		LimitVal:        int64(limit + 1),
	})
	if err != nil {
		return nil, err
	}
	hasMore := len(rows) > limit
	if hasMore {
		rows = rows[:limit]
	}
	// Reverse DESC query result into chronological order (oldest first)
	items := make([]*domain.Message, 0, len(rows))
	for i := len(rows) - 1; i >= 0; i-- {
		items = append(items, s.repliesRowToDomain(rows[i]))
	}
	nextCursor := ""
	if len(items) > 0 {
		oldest := items[0]
		nextCursor = encodeMsgCursor(oldest.CreatedAt, oldest.ID)
	}
	return &domain.MessageListResult{Items: items, NextCursor: nextCursor, HasMore: hasMore}, nil
}

func (s *MessageStore) GetByID(ctx context.Context, id, convID string) (*domain.Message, error) {
	row, err := s.q.GetMessageByID(ctx, sqlc.GetMessageByIDParams{ID: id, ConversationID: convID})
	if err != nil {
		return nil, mapScanErr(err)
	}
	return s.byIDRowToDomain(row), nil
}

func (s *MessageStore) GetByIDAnyConv(ctx context.Context, id string) (*domain.Message, error) {
	row, err := s.q.GetMessageByIDAnyConv(ctx, id)
	if err != nil {
		return nil, mapScanErr(err)
	}
	return s.byIDAnyConvRowToDomain(row), nil
}

func (s *MessageStore) Create(ctx context.Context, msg *domain.Message) error {
	row, err := s.q.CreateMessage(ctx, sqlc.CreateMessageParams{
		ID:                 msg.ID,
		ConversationID:     msg.ConversationID,
		OrgID:              msg.OrgID,
		SenderID:           msg.SenderID,
		Content:            msg.Content,
		SearchContent:      domain.StripMentionTokens(msg.Content),
		ParentID:           msg.ParentID,
		ForwardedMessageID: msg.ForwardedMessageID,
	})
	if err != nil {
		return err
	}
	msg.CreatedAt = parseTime(row.CreatedAt)
	return nil
}

func (s *MessageStore) Update(ctx context.Context, msg *domain.Message) error {
	return s.q.UpdateMessageContent(ctx, sqlc.UpdateMessageContentParams{
		Content:        msg.Content,
		SearchContent:  domain.StripMentionTokens(msg.Content),
		ID:             msg.ID,
		ConversationID: msg.ConversationID,
		SenderID:       msg.SenderID,
	})
}

func (s *MessageStore) SoftDelete(ctx context.Context, id, convID string) error {
	return s.q.SoftDeleteMessage(ctx, sqlc.SoftDeleteMessageParams{ID: id, ConversationID: convID})
}

func (s *MessageStore) Pin(ctx context.Context, id, convID, pinnedBy string) error {
	return s.q.PinMessage(ctx, sqlc.PinMessageParams{ID: id, ConversationID: convID, PinnedBy: &pinnedBy})
}

func (s *MessageStore) Unpin(ctx context.Context, id, convID string) error {
	return s.q.UnpinMessage(ctx, sqlc.UnpinMessageParams{ID: id, ConversationID: convID})
}

func (s *MessageStore) GetConversationLastMessage(ctx context.Context, convID string) (*domain.Message, error) {
	row, err := s.q.GetConversationLastMessage(ctx, convID)
	if err != nil {
		return nil, mapScanErr(err)
	}
	return s.lastMsgRowToDomain(row), nil
}

// GetLastMessagesForConversations returns the latest non-deleted message for
// each of the given conversation IDs in a single query (avoids the N+1 of
// calling GetConversationLastMessage per conversation).
func (s *MessageStore) GetLastMessagesForConversations(ctx context.Context, convIDs []string) (map[string]*domain.Message, error) {
	if len(convIDs) == 0 {
		return map[string]*domain.Message{}, nil
	}
	rows, err := s.q.GetLastMessagesForConversations(ctx, convIDs)
	if err != nil {
		return nil, err
	}
	out := make(map[string]*domain.Message, len(rows))
	for _, r := range rows {
		out[r.ConversationID] = messageFromFields(
			r.ID, r.ConversationID, r.OrgID, r.SenderID, r.Content,
			r.ParentID, r.ForwardedMessageID,
			r.Pinned, r.PinnedAt, r.PinnedBy, r.EditedAt, r.DeletedAt, r.DeletedAt, r.CreatedAt,
			r.SenderName, r.SenderEmail, r.SenderAvatarUrl,
		)
	}
	return out, nil
}

func (s *MessageStore) Count(ctx context.Context, convID string) (int, error) {
	n, err := s.q.CountMessages(ctx, convID)
	if err != nil {
		return 0, err
	}
	return int(n), nil
}

func (s *MessageStore) SearchMessages(ctx context.Context, orgID, userID string, filter domain.MessageSearchFilter) (*domain.MessageSearchListResult, error) {
	cursorCreatedAt, cursorID, err := decodeMsgCursor(filter.Cursor)
	if err != nil {
		return nil, err
	}
	limit := filter.Limit
	if limit <= 0 {
		limit = 20
	}
	var hasAttachment int64 = 0
	if filter.HasAttachment {
		hasAttachment = 1
	}
	var hasLink int64 = 0
	if filter.HasLink {
		hasLink = 1
	}
	var isPinned int64 = 0
	if filter.IsPinned {
		isPinned = 1
	}
	includeProjectLinked := int64(0)
	if filter.IncludeProjectLinked {
		includeProjectLinked = 1
	}
	rows, err := s.q.SearchMessages(ctx, sqlc.SearchMessagesParams{
		UserID:               userID,
		OrgID:                orgID,
		Query:                filter.Query,
		Scope:                string(filter.Scope),
		ConversationID:       filter.ConversationID,
		SenderID:             filter.SenderID,
		HasAttachment:        hasAttachment,
		HasLink:              hasLink,
		IsPinned:             isPinned,
		IncludeProjectLinked: includeProjectLinked,
		After:                filter.After,
		Before:               filter.Before,
		CursorCreatedAt:      cursorCreatedAt,
		CursorID:             cursorID,
		LimitVal:             int64(limit + 1),
	})
	if err != nil {
		return nil, err
	}
	hasMore := len(rows) > limit
	if hasMore {
		rows = rows[:limit]
	}
	items := make([]*domain.MessageSearchResult, 0, len(rows))
	for _, r := range rows {
		msg := messageFromFields(
			r.ID, r.ConversationID, r.OrgID, r.SenderID, r.Content,
			r.ParentID, r.ForwardedMessageID,
			r.Pinned, r.PinnedAt, r.PinnedBy, r.EditedAt, r.DeletedAt, r.DeletedAt, r.CreatedAt,
			r.SenderName, r.SenderEmail, r.SenderAvatarUrl,
		)
		items = append(items, &domain.MessageSearchResult{
			Message:          msg,
			Rank:             r.RankVal,
			Snippet:          r.Snippet,
			ConversationName: r.ConversationName,
			ConversationType: domain.ConversationType(r.ConversationType),
		})
	}
	nextCursor := ""
	if hasMore && len(items) > 0 {
		last := items[len(items)-1]
		nextCursor = encodeMsgCursor(last.Message.CreatedAt, last.Message.ID)
	}
	return &domain.MessageSearchListResult{
		Items:      items,
		NextCursor: nextCursor,
		HasMore:    hasMore,
	}, nil
}

func (s *MessageStore) listRowToDomain(r sqlc.ListMessagesRow) *domain.Message {
	return messageFromFields(
		r.ID, r.ConversationID, r.OrgID, r.SenderID, r.Content,
		r.ParentID, r.ForwardedMessageID,
		r.Pinned, r.PinnedAt, r.PinnedBy, r.EditedAt, r.DeletedAt, r.DeletedAt, r.CreatedAt,
		r.SenderName, r.SenderEmail, r.SenderAvatarUrl,
	)
}

func (s *MessageStore) repliesRowToDomain(r sqlc.ListRepliesRow) *domain.Message {
	return messageFromFields(
		r.ID, r.ConversationID, r.OrgID, r.SenderID, r.Content,
		r.ParentID, r.ForwardedMessageID,
		r.Pinned, r.PinnedAt, r.PinnedBy, r.EditedAt, r.DeletedAt, r.DeletedAt, r.CreatedAt,
		r.SenderName, r.SenderEmail, r.SenderAvatarUrl,
	)
}

func (s *MessageStore) byIDRowToDomain(r sqlc.GetMessageByIDRow) *domain.Message {
	return messageFromFields(
		r.ID, r.ConversationID, r.OrgID, r.SenderID, r.Content,
		r.ParentID, r.ForwardedMessageID,
		r.Pinned, r.PinnedAt, r.PinnedBy, r.EditedAt, r.DeletedAt, r.DeletedAt, r.CreatedAt,
		r.SenderName, r.SenderEmail, r.SenderAvatarUrl,
	)
}

func (s *MessageStore) byIDAnyConvRowToDomain(r sqlc.GetMessageByIDAnyConvRow) *domain.Message {
	return messageFromFields(
		r.ID, r.ConversationID, r.OrgID, r.SenderID, r.Content,
		r.ParentID, r.ForwardedMessageID,
		r.Pinned, r.PinnedAt, r.PinnedBy, r.EditedAt, r.DeletedAt, r.DeletedAt, r.CreatedAt,
		r.SenderName, r.SenderEmail, r.SenderAvatarUrl,
	)
}

func (s *MessageStore) lastMsgRowToDomain(r sqlc.GetConversationLastMessageRow) *domain.Message {
	return messageFromFields(
		r.ID, r.ConversationID, r.OrgID, r.SenderID, r.Content,
		r.ParentID, r.ForwardedMessageID,
		r.Pinned, r.PinnedAt, r.PinnedBy, r.EditedAt, r.DeletedAt, r.DeletedAt, r.CreatedAt,
		r.SenderName, r.SenderEmail, r.SenderAvatarUrl,
	)
}

func messageFromFields(
	id, conversationID, orgID, senderID, content string,
	parentID, forwardedMessageID *string,
	pinned int64, pinnedAt, pinnedBy, editedAt, deletedAt, _ *string,
	createdAtStr string,
	senderName, senderEmail string, senderAvatarURL *string,
) *domain.Message {
	created := parseTime(createdAtStr)
	return &domain.Message{
		ID:                 id,
		ConversationID:     conversationID,
		OrgID:              orgID,
		SenderID:           senderID,
		Content:            content,
		ParentID:           parentID,
		ForwardedMessageID: forwardedMessageID,
		Pinned:             pinned == 1,
		PinnedAt:           parseTimePtr(pinnedAt),
		PinnedBy:           pinnedBy,
		EditedAt:           parseTimePtr(editedAt),
		DeletedAt:          parseTimePtr(deletedAt),
		CreatedAt:          created,
		Sender: &domain.User{
			ID:        senderID,
			Name:      senderName,
			Email:     senderEmail,
			AvatarURL: senderAvatarURL,
		},
	}
}

// Reactions

type ReactionStore struct {
	q *sqlc.Queries
}

func NewReactionStore(q *sqlc.Queries) *ReactionStore {
	return &ReactionStore{q: q}
}

func (s *ReactionStore) Add(ctx context.Context, orgID, messageID, userID, emoji string) error {
	return s.q.AddReaction(ctx, sqlc.AddReactionParams{
		MessageID: messageID,
		UserID:    userID,
		OrgID:     orgID,
		Emoji:     emoji,
	})
}

func (s *ReactionStore) Remove(ctx context.Context, messageID, userID, emoji string) (bool, error) {
	rows, err := s.q.RemoveReaction(ctx, sqlc.RemoveReactionParams{
		MessageID: messageID,
		UserID:    userID,
		Emoji:     emoji,
	})
	if err != nil {
		return false, err
	}
	return rows > 0, nil
}

func (s *ReactionStore) ListForMessages(ctx context.Context, messageIDs []string) ([]*domain.Reaction, error) {
	if len(messageIDs) == 0 {
		return nil, nil
	}
	rows, err := s.q.GetReactionsForMessages(ctx, messageIDs)
	if err != nil {
		return nil, fmt.Errorf("reactions: %w", err)
	}
	items := make([]*domain.Reaction, 0, len(rows))
	for _, r := range rows {
		createdAt := parseTime(r.CreatedAt)
		items = append(items, &domain.Reaction{
			MessageID: r.MessageID,
			UserID:    r.UserID,
			Emoji:     r.Emoji,
			CreatedAt: createdAt,
		})
	}
	return items, nil
}
