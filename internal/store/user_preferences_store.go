package store

import (
	"context"
	"database/sql"

	"ipmanlk/plume/internal/domain"
	"ipmanlk/plume/internal/port"
	"ipmanlk/plume/internal/store/sqlc"
)

type UserPreferencesStore struct {
	q *sqlc.Queries
}

func NewUserPreferencesStore(q *sqlc.Queries) *UserPreferencesStore {
	return &UserPreferencesStore{q: q}
}

var _ port.UserPreferencesRepository = (*UserPreferencesStore)(nil)

func (s *UserPreferencesStore) Get(ctx context.Context, userID string) (*domain.UserPreferences, error) {
	row, err := s.q.GetUserPreferences(ctx, userID)
	if err != nil {
		if err == sql.ErrNoRows {
			// Return defaults; caller decides whether to persist.
			return domain.DefaultUserPreferences(userID), nil
		}
		return nil, err
	}
	return &domain.UserPreferences{
		UserID:               row.UserID,
		Theme:                row.Theme,
		Language:             row.Language,
		Timezone:             row.Timezone,
		EmailNotifications:   row.EmailNotifications != 0,
		DesktopNotifications: row.DesktopNotifications != 0,
		NotificationSounds:   row.NotificationSounds != 0,
		SidebarCollapsed:     row.SidebarCollapsed != 0,
		MotionSettings:       row.MotionSettings,
	}, nil
}

func (s *UserPreferencesStore) Upsert(ctx context.Context, prefs *domain.UserPreferences) error {
	return s.q.UpsertUserPreferences(ctx, sqlc.UpsertUserPreferencesParams{
		UserID:               prefs.UserID,
		Theme:                prefs.Theme,
		Language:             prefs.Language,
		Timezone:             prefs.Timezone,
		EmailNotifications:   boolToInt64(prefs.EmailNotifications),
		DesktopNotifications: boolToInt64(prefs.DesktopNotifications),
		NotificationSounds:   boolToInt64(prefs.NotificationSounds),
		SidebarCollapsed:     boolToInt64(prefs.SidebarCollapsed),
		MotionSettings:       prefs.MotionSettings,
	})
}
