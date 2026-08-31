package store

import (
	"context"
	"database/sql"
	"encoding/base64"
	"encoding/json"
	"errors"
	"time"

	"ipmanlk/plume/internal/domain"
	"ipmanlk/plume/internal/port"
	"ipmanlk/plume/internal/store/sqlc"
)

var _ port.ConversationRepository = (*ConversationStore)(nil)

type ConversationStore struct {
	q  *sqlc.Queries
	db *sql.DB
}

func NewConversationStore(q *sqlc.Queries, db *sql.DB) *ConversationStore {
	return &ConversationStore{q: q, db: db}
}

type convCursor struct {
	K string `json:"k"`
	I string `json:"i"`
}

func encodeConvCursor(positionKey, id string) string {
	b, _ := json.Marshal(convCursor{K: positionKey, I: id})
	return base64.URLEncoding.EncodeToString(b)
}

func decodeConvCursor(s string) (positionKey, id string, err error) {
	if s == "" {
		return "", "", nil
	}
	b, err := base64.URLEncoding.DecodeString(s)
	if err != nil {
		return "", "", err
	}
	var c convCursor
	if err := json.Unmarshal(b, &c); err != nil {
		return "", "", err
	}
	return c.K, c.I, nil
}

func (s *ConversationStore) ListByUser(ctx context.Context, orgID, userID string, filter domain.ConversationFilter) (*domain.ConversationListResult, error) {
	limit := filter.Limit
	if limit <= 0 {
		limit = 20
	}
	cursorKey, cursorID, err := decodeConvCursor(filter.Cursor)
	if err != nil {
		return nil, err
	}
	scope := ""
	if filter.Scope != nil {
		scope = *filter.Scope
	}
	includeProjectLinked := int64(0)
	if filter.IncludeProjectLinked {
		includeProjectLinked = 1
	}
	rows, err := s.q.ListConversationsByUser(ctx, sqlc.ListConversationsByUserParams{
		UserID:               userID,
		OrgID:                orgID,
		Scope:                scope,
		IncludeProjectLinked: includeProjectLinked,
		CursorKey:            cursorKey,
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
	items := make([]*domain.Conversation, 0, len(rows))
	for _, r := range rows {
		c, err := listRowToDomain(r)
		if err != nil {
			return nil, err
		}
		items = append(items, c)
	}
	nextCursor := ""
	if len(items) > 0 {
		last := items[len(items)-1]
		nextCursor = encodeConvCursor(last.PositionKey, last.ID)
	}
	return &domain.ConversationListResult{Items: items, NextCursor: nextCursor, HasMore: hasMore}, nil
}

func (s *ConversationStore) ListByParent(ctx context.Context, orgID, parentID, userID string, includeProjectLinked bool) ([]*domain.Conversation, error) {
	inclProjLinked := int64(0)
	if includeProjectLinked {
		inclProjLinked = 1
	}
	rows, err := s.q.ListConversationsByParent(ctx, sqlc.ListConversationsByParentParams{
		UserID:               userID,
		OrgID:                orgID,
		ParentID:             &parentID,
		IncludeProjectLinked: inclProjLinked,
	})
	if err != nil {
		return nil, err
	}
	items := make([]*domain.Conversation, 0, len(rows))
	for _, r := range rows {
		c, err := parentRowToDomain(r)
		if err != nil {
			return nil, err
		}
		items = append(items, c)
	}
	return items, nil
}

func (s *ConversationStore) GetByID(ctx context.Context, orgID, id string) (*domain.Conversation, error) {
	row, err := s.q.GetConversationByID(ctx, sqlc.GetConversationByIDParams{ID: id, OrgID: orgID})
	if err != nil {
		return nil, mapScanErr(err)
	}
	return singleToDomain(row)
}

func (s *ConversationStore) GetByIDWithMember(ctx context.Context, orgID, id, userID string) (*domain.Conversation, error) {
	conv, err := s.GetByID(ctx, orgID, id)
	if err != nil {
		return nil, err
	}
	_, err = s.q.GetConversationMember(ctx, sqlc.GetConversationMemberParams{ConversationID: id, UserID: userID})
	if err != nil {
		return nil, errors.New("not a member of this conversation")
	}
	return conv, nil
}

func (s *ConversationStore) GetDMByUsers(ctx context.Context, orgID, requesterID, recipientID string) (*domain.Conversation, error) {
	row, err := s.q.FindDMByUsers(ctx, sqlc.FindDMByUsersParams{
		RequesterID: requesterID,
		RecipientID: recipientID,
		OrgID:       orgID,
	})
	if err != nil {
		return nil, mapScanErr(err)
	}
	return singleToDomain(sqlc.Conversation{
		ID:          row.ID,
		OrgID:       row.OrgID,
		ParentID:    row.ParentID,
		Name:        row.Name,
		Topic:       row.Topic,
		Type:        row.Type,
		CreatedBy:   row.CreatedBy,
		PositionKey: row.PositionKey,
		CreatedAt:   row.CreatedAt,
		UpdatedAt:   row.UpdatedAt,
		DeletedAt:   row.DeletedAt,
	})
}

func (s *ConversationStore) Create(ctx context.Context, conv *domain.Conversation) error {
	row, err := s.q.CreateConversation(ctx, sqlc.CreateConversationParams{
		ID:          conv.ID,
		OrgID:       conv.OrgID,
		ParentID:    conv.ParentID,
		Name:        conv.Name,
		Topic:       conv.Topic,
		Type:        string(conv.Type),
		CreatedBy:   conv.CreatedBy,
		PositionKey: conv.PositionKey,
	})
	if err != nil {
		return err
	}
	created, err := singleToDomain(row)
	if err != nil {
		return err
	}
	*conv = *created
	return nil
}

// CreateWithMembers creates a conversation and adds the given member IDs
// atomically. Used by CreateChannel, CreateDM, CreateGroupDM to ensure the
// conversation is never orphaned without its creator as a member.
func (s *ConversationStore) CreateWithMembers(ctx context.Context, conv *domain.Conversation, memberIDs []string) error {
	tx, err := s.db.BeginTx(ctx, nil)
	if err != nil {
		return err
	}
	defer tx.Rollback()

	q := s.q.WithTx(tx)

	row, err := q.CreateConversation(ctx, sqlc.CreateConversationParams{
		ID:          conv.ID,
		OrgID:       conv.OrgID,
		ParentID:    conv.ParentID,
		Name:        conv.Name,
		Topic:       conv.Topic,
		Type:        string(conv.Type),
		CreatedBy:   conv.CreatedBy,
		PositionKey: conv.PositionKey,
	})
	if err != nil {
		return err
	}
	created, err := singleToDomain(row)
	if err != nil {
		return err
	}
	*conv = *created

	for _, memberID := range memberIDs {
		if err := q.AddConversationMember(ctx, sqlc.AddConversationMemberParams{
			ConversationID: conv.ID,
			UserID:         memberID,
			OrgID:          conv.OrgID,
		}); err != nil {
			return err
		}
	}

	return tx.Commit()
}

func (s *ConversationStore) Update(ctx context.Context, conv *domain.Conversation) error {
	return s.q.UpdateConversation(ctx, sqlc.UpdateConversationParams{
		Name:  conv.Name,
		Topic: conv.Topic,
		ID:    conv.ID,
		OrgID: conv.OrgID,
	})
}

func (s *ConversationStore) UpdateParent(ctx context.Context, orgID, id string, parentID *string, positionKey string) error {
	return s.q.UpdateConversationParent(ctx, sqlc.UpdateConversationParentParams{
		ParentID:    parentID,
		PositionKey: positionKey,
		ID:          id,
		OrgID:       orgID,
	})
}

func (s *ConversationStore) UpdatePositionKey(ctx context.Context, orgID, id string, positionKey string) error {
	return s.q.UpdateConversationPositionKey(ctx, sqlc.UpdateConversationPositionKeyParams{
		PositionKey: positionKey,
		ID:          id,
		OrgID:       orgID,
	})
}

func (s *ConversationStore) ListCategories(ctx context.Context, orgID string) ([]*domain.Conversation, error) {
	rows, err := s.q.ListCategoriesByOrg(ctx, orgID)
	if err != nil {
		return nil, err
	}
	items := make([]*domain.Conversation, 0, len(rows))
	for _, r := range rows {
		c, err := singleToDomain(r)
		if err != nil {
			return nil, err
		}
		items = append(items, c)
	}
	return items, nil
}

func (s *ConversationStore) ListSiblingPositionKeys(ctx context.Context, orgID string, parentID *string) ([]string, error) {
	if parentID == nil {
		cats, err := s.ListCategories(ctx, orgID)
		if err != nil {
			return nil, err
		}
		keys := make([]string, 0, len(cats))
		for _, c := range cats {
			keys = append(keys, c.PositionKey)
		}
		return keys, nil
	}
	return s.q.ListChildPositionKeys(ctx, sqlc.ListChildPositionKeysParams{
		OrgID:    orgID,
		ParentID: parentID,
	})
}

func (s *ConversationStore) Delete(ctx context.Context, orgID, id string) error {
	return s.q.DeleteConversation(ctx, sqlc.DeleteConversationParams{ID: id, OrgID: orgID})
}

func (s *ConversationStore) SoftDeleteByParent(ctx context.Context, orgID, parentID string) error {
	return s.q.DeleteConversationsByParent(ctx, sqlc.DeleteConversationsByParentParams{
		ParentID: &parentID,
		OrgID:    orgID,
	})
}

func (s *ConversationStore) AddMember(ctx context.Context, orgID, convID, userID string) error {
	return s.q.AddConversationMember(ctx, sqlc.AddConversationMemberParams{
		ConversationID: convID,
		UserID:         userID,
		OrgID:          orgID,
	})
}

func (s *ConversationStore) RemoveMember(ctx context.Context, convID, userID string) error {
	return s.q.RemoveConversationMember(ctx, sqlc.RemoveConversationMemberParams{
		ConversationID: convID,
		UserID:         userID,
	})
}

func (s *ConversationStore) GetMembers(ctx context.Context, convID string) ([]*domain.ConversationMember, error) {
	rows, err := s.q.ListConversationMembers(ctx, convID)
	if err != nil {
		return nil, err
	}
	items := make([]*domain.ConversationMember, 0, len(rows))
	for _, r := range rows {
		joined := parseTime(r.JoinedAt)
		lastRead := parseTime(r.LastReadAt)
		items = append(items, &domain.ConversationMember{
			ConversationID: r.ConversationID,
			UserID:         r.UserID,
			OrgID:          r.OrgID,
			JoinedAt:       joined,
			LastReadAt:     lastRead,
			Muted:          r.Muted == 1,
			User: &domain.User{
				ID:        r.UserID,
				Name:      r.UserName,
				Email:     r.UserEmail,
				AvatarURL: r.UserAvatarUrl,
				Role:      domain.Role(r.UserRole),
			},
		})
	}
	return items, nil
}

func (s *ConversationStore) IsMember(ctx context.Context, convID, userID string) (bool, error) {
	_, err := s.q.GetConversationMember(ctx, sqlc.GetConversationMemberParams{ConversationID: convID, UserID: userID})
	if err != nil {
		if errors.Is(err, sql.ErrNoRows) {
			return false, nil
		}
		return false, err
	}
	return true, nil
}

func (s *ConversationStore) UpdateReadState(ctx context.Context, convID, userID string) error {
	return s.q.UpdateLastRead(ctx, sqlc.UpdateLastReadParams{ConversationID: convID, UserID: userID})
}

func (s *ConversationStore) UnreadCount(ctx context.Context, convID, userID string) (int, error) {
	n, err := s.q.GetUnreadMessageCount(ctx, sqlc.GetUnreadMessageCountParams{
		ConversationID: convID,
		UserID:         userID,
	})
	if err != nil {
		return 0, err
	}
	return int(n), nil
}

// UnreadCounts returns a map of conversation_id → unread count for the given
// conversation IDs in a single query (avoids the N+1 of calling UnreadCount
// per conversation).
func (s *ConversationStore) UnreadCounts(ctx context.Context, userID string, convIDs []string) (map[string]int, error) {
	if len(convIDs) == 0 {
		return map[string]int{}, nil
	}
	rows, err := s.q.GetUnreadCounts(ctx, sqlc.GetUnreadCountsParams{
		UserID:          userID,
		ConversationIds: convIDs,
	})
	if err != nil {
		return nil, err
	}
	out := make(map[string]int, len(rows))
	for _, r := range rows {
		out[r.ConversationID] = int(r.Unread)
	}
	return out, nil
}

func (s *ConversationStore) GetLastMessage(ctx context.Context, convID string) (*domain.Message, error) {
	row, err := s.q.GetLastMessage(ctx, convID)
	if err != nil {
		return nil, err
	}
	return lastMsgToDomain(row)
}

func (s *ConversationStore) ListPinnedMessages(ctx context.Context, convID string, limit int) ([]*domain.Message, error) {
	_ = limit // query returns all pinned messages; limit reserved for future paging
	rows, err := s.q.ListPinnedMessages(ctx, convID)
	if err != nil {
		return nil, err
	}
	items := make([]*domain.Message, 0, len(rows))
	for _, r := range rows {
		items = append(items, pinnedMsgToDomain(r))
	}
	return items, nil
}

func (s *ConversationStore) CountMembers(ctx context.Context, convID string) (int, error) {
	n, err := s.q.CountConversationMembers(ctx, convID)
	if err != nil {
		return 0, err
	}
	return int(n), nil
}

func (s *ConversationStore) ListByIDs(ctx context.Context, orgID string, ids []string) ([]*domain.Conversation, error) {
	if len(ids) == 0 {
		return []*domain.Conversation{}, nil
	}
	rows, err := s.q.ListConversationsByIDs(ctx, sqlc.ListConversationsByIDsParams{
		OrgID: orgID,
		Ids:   ids,
	})
	if err != nil {
		return nil, err
	}
	items := make([]*domain.Conversation, len(rows))
	for i, r := range rows {
		c, err := singleToDomain(sqlc.Conversation{
			ID:          r.ID,
			OrgID:       r.OrgID,
			ParentID:    r.ParentID,
			Name:        r.Name,
			Topic:       r.Topic,
			Type:        r.Type,
			CreatedBy:   r.CreatedBy,
			PositionKey: r.PositionKey,
			CreatedAt:   r.CreatedAt,
			UpdatedAt:   r.UpdatedAt,
		})
		if err != nil {
			return nil, err
		}
		items[i] = c
	}
	return items, nil
}

func singleToDomain(r sqlc.Conversation) (*domain.Conversation, error) {
	createdAt := parseTime(r.CreatedAt)
	updatedAt := parseTime(r.UpdatedAt)
	return &domain.Conversation{
		ID:          r.ID,
		OrgID:       r.OrgID,
		ParentID:    r.ParentID,
		Name:        r.Name,
		Topic:       r.Topic,
		Type:        domain.ConversationType(r.Type),
		CreatedBy:   r.CreatedBy,
		PositionKey: r.PositionKey,
		CreatedAt:   createdAt,
		UpdatedAt:   updatedAt,
		DeletedAt:   parseTimePtr(r.DeletedAt),
	}, nil
}

func listRowToDomain(r sqlc.ListConversationsByUserRow) (*domain.Conversation, error) {
	c, err := singleToDomain(sqlc.Conversation{
		ID:          r.ID,
		OrgID:       r.OrgID,
		ParentID:    r.ParentID,
		Name:        r.Name,
		Topic:       r.Topic,
		Type:        r.Type,
		CreatedBy:   r.CreatedBy,
		PositionKey: r.PositionKey,
		CreatedAt:   r.CreatedAt,
		UpdatedAt:   r.UpdatedAt,
	})
	if err != nil {
		return nil, err
	}
	c.MemberCount = int(r.MemberCount)
	return c, nil
}

func parentRowToDomain(r sqlc.ListConversationsByParentRow) (*domain.Conversation, error) {
	c, err := singleToDomain(sqlc.Conversation{
		ID:          r.ID,
		OrgID:       r.OrgID,
		ParentID:    r.ParentID,
		Name:        r.Name,
		Topic:       r.Topic,
		Type:        r.Type,
		CreatedBy:   r.CreatedBy,
		PositionKey: r.PositionKey,
		CreatedAt:   r.CreatedAt,
		UpdatedAt:   r.UpdatedAt,
	})
	if err != nil {
		return nil, err
	}
	c.MemberCount = int(r.MemberCount)
	return c, nil
}

func lastMsgToDomain(r sqlc.Message) (*domain.Message, error) {
	createdAt := parseTime(r.CreatedAt)
	var editedAt, deletedAt, pinnedAt *time.Time
	if r.EditedAt != nil {
		t := parseTime(*r.EditedAt)
		editedAt = &t
	}
	if r.DeletedAt != nil {
		t := parseTime(*r.DeletedAt)
		deletedAt = &t
	}
	if r.PinnedAt != nil {
		t := parseTime(*r.PinnedAt)
		pinnedAt = &t
	}
	return &domain.Message{
		ID:                 r.ID,
		ConversationID:     r.ConversationID,
		OrgID:              r.OrgID,
		SenderID:           r.SenderID,
		Content:            r.Content,
		ParentID:           r.ParentID,
		ForwardedMessageID: r.ForwardedMessageID,
		Pinned:             r.Pinned == 1,
		PinnedAt:           pinnedAt,
		PinnedBy:           r.PinnedBy,
		EditedAt:           editedAt,
		DeletedAt:          deletedAt,
		CreatedAt:          createdAt,
	}, nil
}

func pinnedMsgToDomain(r sqlc.ListPinnedMessagesRow) *domain.Message {
	createdAt := parseTime(r.CreatedAt)
	var pinnedAt *time.Time
	if r.PinnedAt != nil {
		t := parseTime(*r.PinnedAt)
		pinnedAt = &t
	}
	return &domain.Message{
		ID:                 r.ID,
		ConversationID:     r.ConversationID,
		OrgID:              r.OrgID,
		SenderID:           r.SenderID,
		Content:            r.Content,
		ParentID:           r.ParentID,
		ForwardedMessageID: r.ForwardedMessageID,
		Pinned:             r.Pinned == 1,
		PinnedAt:           pinnedAt,
		PinnedBy:           r.PinnedBy,
		EditedAt:           parseTimePtr(r.EditedAt),
		CreatedAt:          createdAt,
		Sender: &domain.User{
			ID:        r.SenderID,
			Name:      r.SenderName,
			Email:     r.SenderEmail,
			AvatarURL: r.SenderAvatarUrl,
		},
	}
}
