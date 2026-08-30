package store

import (
	"context"

	"ipmanlk/breeze/internal/domain"
	"ipmanlk/breeze/internal/port"
	"ipmanlk/breeze/internal/store/sqlc"
)

var _ port.PresenceRepository = (*PresenceStore)(nil)

type PresenceStore struct {
	q *sqlc.Queries
}

func NewPresenceStore(q *sqlc.Queries) *PresenceStore {
	return &PresenceStore{q: q}
}

func (s *PresenceStore) Upsert(ctx context.Context, orgID, userID string, status domain.PresenceStatus) error {
	return s.q.UpsertPresence(ctx, sqlc.UpsertPresenceParams{
		UserID: userID,
		OrgID:  orgID,
		Status: string(status),
	})
}

func (s *PresenceStore) Get(ctx context.Context, orgID, userID string) (*domain.UserPresence, error) {
	row, err := s.q.GetPresence(ctx, sqlc.GetPresenceParams{UserID: userID, OrgID: orgID})
	if err != nil {
		return nil, err
	}
	return presenceToDomain(row)
}

func (s *PresenceStore) ListForOrg(ctx context.Context, orgID string) ([]*domain.UserPresence, error) {
	rows, err := s.q.ListOrgPresence(ctx, orgID)
	if err != nil {
		return nil, err
	}
	items := make([]*domain.UserPresence, 0, len(rows))
	for _, r := range rows {
		items = append(items, &domain.UserPresence{
			UserID:   r.UserID,
			OrgID:    r.OrgID,
			Status:   domain.PresenceStatus(r.Status),
			LastSeen: parseTime(r.LastSeen),
			User: &domain.User{
				ID:        r.UserID,
				Name:      r.UserName,
				Email:     r.UserEmail,
				AvatarURL: r.UserAvatarUrl,
			},
		})
	}
	return items, nil
}

func presenceToDomain(r sqlc.UserPresence) (*domain.UserPresence, error) {
	return &domain.UserPresence{
		UserID:   r.UserID,
		OrgID:    r.OrgID,
		Status:   domain.PresenceStatus(r.Status),
		LastSeen: parseTime(r.LastSeen),
	}, nil
}
