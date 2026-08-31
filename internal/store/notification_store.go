package store

import (
	"context"
	"encoding/base64"
	"encoding/json"
	"fmt"
	"time"

	"ipmanlk/plume/internal/apperr"
	"ipmanlk/plume/internal/domain"
	"ipmanlk/plume/internal/port"
	"ipmanlk/plume/internal/store/sqlc"
)

type NotificationStore struct {
	q *sqlc.Queries
}

func NewNotificationStore(q *sqlc.Queries) *NotificationStore {
	return &NotificationStore{q: q}
}

var _ port.NotificationRepository = (*NotificationStore)(nil)

type notifCursor struct {
	C string `json:"c"`
	I string `json:"i"`
}

func encodeNotifCursor(createdAt, id string) string {
	data, _ := json.Marshal(notifCursor{C: createdAt, I: id})
	return base64.StdEncoding.EncodeToString(data)
}

func decodeNotifCursor(cursor string) (createdAt, id string, err error) {
	data, err := base64.StdEncoding.DecodeString(cursor)
	if err != nil {
		return "", "", fmt.Errorf("invalid cursor: %w", err)
	}
	var c notifCursor
	if err := json.Unmarshal(data, &c); err != nil {
		return "", "", fmt.Errorf("invalid cursor: %w", err)
	}
	return c.C, c.I, nil
}

func (s *NotificationStore) toNotification(row sqlc.ListNotificationsRow) *domain.Notification {
	n := &domain.Notification{
		ID:          row.ID,
		OrgID:       row.OrgID,
		UserID:      row.UserID,
		Type:        domain.NotificationType(row.Type),
		Title:       row.Title,
		Body:        row.Body,
		Link:        row.Link,
		EntityType:  row.EntityType,
		EntityID:    row.EntityID,
		ProjectSlug: row.ProjectSlug,
		IsRead:      row.IsRead == 1,
		CreatedAt:   parseTime(row.CreatedAt),
	}
	if row.ReadAt != nil {
		t := parseTime(*row.ReadAt)
		n.ReadAt = &t
	}
	if row.ActorID != nil && *row.ActorID != "" {
		n.ActorID = *row.ActorID
		n.Actor = &domain.User{
			ID:        *row.ActorUserID,
			Name:      *row.ActorName,
			Email:     *row.ActorEmail,
			AvatarURL: row.ActorAvatarUrl,
		}
	}
	return n
}

func (s *NotificationStore) toNotificationFromGet(row sqlc.GetNotificationByIDRow) *domain.Notification {
	n := &domain.Notification{
		ID:          row.ID,
		OrgID:       row.OrgID,
		UserID:      row.UserID,
		Type:        domain.NotificationType(row.Type),
		Title:       row.Title,
		Body:        row.Body,
		Link:        row.Link,
		EntityType:  row.EntityType,
		EntityID:    row.EntityID,
		ProjectSlug: row.ProjectSlug,
		IsRead:      row.IsRead == 1,
		CreatedAt:   parseTime(row.CreatedAt),
	}
	if row.ReadAt != nil {
		t := parseTime(*row.ReadAt)
		n.ReadAt = &t
	}
	if row.ActorID != nil && *row.ActorID != "" {
		n.ActorID = *row.ActorID
		n.Actor = &domain.User{
			ID:        *row.ActorUserID,
			Name:      *row.ActorName,
			Email:     *row.ActorEmail,
			AvatarURL: row.ActorAvatarUrl,
		}
	}
	return n
}

func (s *NotificationStore) Create(ctx context.Context, n *domain.Notification) error {
	createdAt := n.CreatedAt
	if createdAt.IsZero() {
		createdAt = time.Now()
	}
	return s.q.CreateNotification(ctx, sqlc.CreateNotificationParams{
		ID:         n.ID,
		OrgID:      n.OrgID,
		UserID:     n.UserID,
		Type:       string(n.Type),
		Title:      n.Title,
		Body:       n.Body,
		Link:       n.Link,
		EntityType: n.EntityType,
		EntityID:   n.EntityID,
		ActorID:    nilIfEmpty(n.ActorID),
		CreatedAt:  formatTime(createdAt),
	})
}

func (s *NotificationStore) List(ctx context.Context, orgID, userID string, filter domain.NotificationFilter) (*domain.NotificationListResult, error) {
	limit := filter.Limit
	if limit < 1 || limit > 100 {
		limit = 20
	}

	var cursorCreatedAt, cursorID string
	if filter.Cursor != "" {
		var err error
		cursorCreatedAt, cursorID, err = decodeNotifCursor(filter.Cursor)
		if err != nil {
			return nil, err
		}
	}

	unreadOnly := int64(0)
	if filter.UnreadOnly {
		unreadOnly = 1
	}

	rows, err := s.q.ListNotifications(ctx, sqlc.ListNotificationsParams{
		UserID:          userID,
		OrgID:           orgID,
		UnreadOnly:      unreadOnly,
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

	items := make([]*domain.Notification, len(rows))
	for i, row := range rows {
		items[i] = s.toNotification(row)
	}

	result := &domain.NotificationListResult{
		Items:   items,
		HasMore: hasMore,
	}
	if hasMore && len(rows) > 0 {
		last := rows[len(rows)-1]
		result.NextCursor = encodeNotifCursor(last.CreatedAt, last.ID)
	}

	return result, nil
}

func (s *NotificationStore) GetByID(ctx context.Context, id, userID string) (*domain.Notification, error) {
	row, err := s.q.GetNotificationByID(ctx, sqlc.GetNotificationByIDParams{
		ID:     id,
		UserID: userID,
	})
	if err != nil {
		return nil, mapScanErr(err)
	}
	return s.toNotificationFromGet(row), nil
}

func (s *NotificationStore) MarkRead(ctx context.Context, id, userID string) error {
	rows, err := s.q.MarkNotificationRead(ctx, sqlc.MarkNotificationReadParams{
		ID:     id,
		UserID: userID,
	})
	if err != nil {
		return err
	}
	if rows == 0 {
		// No row matched id+user_id; the notification is missing or belongs
		// to another user. Return ErrNotFound so the handler can 404.
		return apperr.ErrNotFound
	}
	return nil
}

func (s *NotificationStore) MarkAllRead(ctx context.Context, userID string) error {
	return s.q.MarkAllNotificationsRead(ctx, userID)
}

func (s *NotificationStore) CountUnread(ctx context.Context, userID string) (int64, error) {
	return s.q.CountUnreadNotifications(ctx, userID)
}
