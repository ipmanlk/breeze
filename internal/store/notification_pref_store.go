package store

import (
	"context"
	"time"

	"ipmanlk/plume/internal/domain"
	"ipmanlk/plume/internal/port"
	"ipmanlk/plume/internal/store/sqlc"
)

type NotificationPreferenceStore struct {
	q *sqlc.Queries
}

func NewNotificationPreferenceStore(q *sqlc.Queries) *NotificationPreferenceStore {
	return &NotificationPreferenceStore{q: q}
}

var _ port.NotificationPreferenceRepository = (*NotificationPreferenceStore)(nil)

func (s *NotificationPreferenceStore) List(ctx context.Context, userID string) ([]*domain.NotificationPreference, error) {
	rows, err := s.q.ListNotificationPreferences(ctx, userID)
	if err != nil {
		return nil, err
	}
	prefs := make([]*domain.NotificationPreference, len(rows))
	for i, r := range rows {
		prefs[i] = &domain.NotificationPreference{
			Type:    domain.NotificationType(r.Type),
			Enabled: r.Enabled == 1,
		}
	}
	return prefs, nil
}

func (s *NotificationPreferenceStore) GetByType(ctx context.Context, userID string, notifType domain.NotificationType) (*domain.NotificationPreference, error) {
	row, err := s.q.GetNotificationPreferenceByType(ctx, sqlc.GetNotificationPreferenceByTypeParams{
		UserID: userID,
		Type:   string(notifType),
	})
	if err != nil {
		return nil, mapScanErr(err)
	}
	return &domain.NotificationPreference{
		Type:    domain.NotificationType(row.Type),
		Enabled: row.Enabled == 1,
	}, nil
}

func (s *NotificationPreferenceStore) Set(ctx context.Context, userID, notifType string, enabled bool) error {
	v := int64(0)
	if enabled {
		v = 1
	}
	return s.q.SetNotificationPreference(ctx, sqlc.SetNotificationPreferenceParams{
		UserID:  userID,
		Type:    notifType,
		Enabled: v,
	})
}

func (s *NotificationPreferenceStore) FindDueNotifications(ctx context.Context, nowMinus1h, now, nowPlus24h time.Time, dueSoonType, overdueType string) ([]domain.DueTaskRow, error) {
	rows, err := s.q.FindDueNotificationTasks(ctx, sqlc.FindDueNotificationTasksParams{
		Now:          formatTimePtr(&now),
		OverdueStart: formatTimePtr(&nowMinus1h),
		DueSoonEnd:   formatTimePtr(&nowPlus24h),
		DueSoonType:  dueSoonType,
		OverdueType:  overdueType,
	})
	if err != nil {
		return nil, err
	}
	items := make([]domain.DueTaskRow, len(rows))
	for i, r := range rows {
		items[i] = domain.DueTaskRow{
			TaskID:      r.TaskID,
			Title:       r.TaskTitle,
			DueAt:       ptrStrOrEmpty(r.DueAt),
			StatusID:    r.StatusID,
			AssigneeID:  r.AssigneeID,
			OrgID:       r.OrgID,
			ProjectID:   r.ProjectID,
			ProjectSlug: r.ProjectSlug,
		}
	}
	return items, nil
}

func ptrStrOrEmpty(s *string) string {
	if s == nil {
		return ""
	}
	return *s
}
