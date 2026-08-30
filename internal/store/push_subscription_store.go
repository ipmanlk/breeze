package store

import (
	"context"

	"ipmanlk/breeze/internal/domain"
	"ipmanlk/breeze/internal/store/sqlc"
)

type PushSubscriptionStore struct {
	q *sqlc.Queries
}

func NewPushSubscriptionStore(q *sqlc.Queries) *PushSubscriptionStore {
	return &PushSubscriptionStore{q: q}
}

func (s *PushSubscriptionStore) Upsert(ctx context.Context, sub *domain.PushSubscription) (*domain.PushSubscription, error) {
	row, err := s.q.CreatePushSubscription(ctx, sqlc.CreatePushSubscriptionParams{
		ID:       sub.ID,
		UserID:   sub.UserID,
		OrgID:    sub.OrgID,
		Endpoint: sub.Endpoint,
		P256dh:   sub.P256dh,
		AuthKey:  sub.Auth,
	})
	if err != nil {
		return nil, err
	}
	return toDomainPushSub(row), nil
}

func (s *PushSubscriptionStore) ListByUser(ctx context.Context, userID string) ([]*domain.PushSubscription, error) {
	rows, err := s.q.ListPushSubscriptionsByUser(ctx, userID)
	if err != nil {
		return nil, err
	}
	out := make([]*domain.PushSubscription, 0, len(rows))
	for _, r := range rows {
		out = append(out, toDomainPushSub(r))
	}
	return out, nil
}

func (s *PushSubscriptionStore) Delete(ctx context.Context, userID, endpoint string) (int64, error) {
	return s.q.DeletePushSubscription(ctx, sqlc.DeletePushSubscriptionParams{
		UserID:   userID,
		Endpoint: endpoint,
	})
}

func (s *PushSubscriptionStore) DeleteByUser(ctx context.Context, userID string) (int64, error) {
	return s.q.DeletePushSubscriptionsByUser(ctx, userID)
}

func toDomainPushSub(r sqlc.PushSubscription) *domain.PushSubscription {
	return &domain.PushSubscription{
		ID:        r.ID,
		UserID:    r.UserID,
		OrgID:     r.OrgID,
		Endpoint:  r.Endpoint,
		P256dh:    r.P256dh,
		Auth:      r.AuthKey,
		CreatedAt: r.CreatedAt,
	}
}
