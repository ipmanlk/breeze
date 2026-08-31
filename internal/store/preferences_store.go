package store

import (
	"context"

	"ipmanlk/plume/internal/domain"
	"ipmanlk/plume/internal/port"
	"ipmanlk/plume/internal/store/sqlc"
)

var _ port.UserChannelPreferenceRepository = (*UserChannelPreferenceStore)(nil)

type UserChannelPreferenceStore struct {
	q *sqlc.Queries
}

func NewUserChannelPreferenceStore(q *sqlc.Queries) *UserChannelPreferenceStore {
	return &UserChannelPreferenceStore{q: q}
}

func (s *UserChannelPreferenceStore) Upsert(ctx context.Context, pref *domain.UserChannelPreference) error {
	return s.q.UpsertChannelPreference(ctx, sqlc.UpsertChannelPreferenceParams{
		UserID:            pref.UserID,
		ConversationID:    pref.ConversationID,
		OrgID:             pref.OrgID,
		NotificationLevel: string(pref.NotificationLevel),
		Muted:             boolToInt(pref.Muted),
	})
}

func (s *UserChannelPreferenceStore) SetMuted(ctx context.Context, orgID, userID, convID string, muted bool) error {
	return s.q.UpsertChannelMute(ctx, sqlc.UpsertChannelMuteParams{
		UserID:         userID,
		ConversationID: convID,
		OrgID:          orgID,
		Muted:          boolToInt(muted),
	})
}

func (s *UserChannelPreferenceStore) SetNotificationLevel(ctx context.Context, orgID, userID, convID string, level domain.NotificationLevel) error {
	return s.q.UpsertChannelNotificationLevel(ctx, sqlc.UpsertChannelNotificationLevelParams{
		UserID:            userID,
		ConversationID:    convID,
		OrgID:             orgID,
		NotificationLevel: string(level),
	})
}

func (s *UserChannelPreferenceStore) Get(ctx context.Context, userID, convID string) (*domain.UserChannelPreference, error) {
	row, err := s.q.GetChannelPreference(ctx, sqlc.GetChannelPreferenceParams{
		UserID:         userID,
		ConversationID: convID,
	})
	if err != nil {
		return nil, err
	}
	return prefToDomain(row)
}

func (s *UserChannelPreferenceStore) UpdateLastRead(ctx context.Context, userID, convID string) error {
	return s.q.UpdateChannelLastRead(ctx, sqlc.UpdateChannelLastReadParams{
		UserID:         userID,
		ConversationID: convID,
	})
}

func (s *UserChannelPreferenceStore) GetNotificationLevel(ctx context.Context, userID, convID string) (domain.NotificationLevel, error) {
	row, err := s.q.GetNotificationLevel(ctx, sqlc.GetNotificationLevelParams{
		UserID:         userID,
		ConversationID: convID,
	})
	if err != nil {
		return "", err
	}
	return domain.NotificationLevel(row), nil
}

func prefToDomain(r sqlc.UserChannelPreference) (*domain.UserChannelPreference, error) {
	lastRead := parseTime(r.LastReadAt)
	return &domain.UserChannelPreference{
		UserID:            r.UserID,
		ConversationID:    r.ConversationID,
		OrgID:             r.OrgID,
		NotificationLevel: domain.NotificationLevel(r.NotificationLevel),
		Muted:             r.Muted == 1,
		LastReadAt:        lastRead,
	}, nil
}

func boolToInt(b bool) int64 {
	if b {
		return 1
	}
	return 0
}
