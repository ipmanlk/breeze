package store

import (
	"context"
	"time"

	"ipmanlk/breeze/internal/domain"
	"ipmanlk/breeze/internal/port"
	"ipmanlk/breeze/internal/store/sqlc"
)

type PasswordResetStore struct {
	q *sqlc.Queries
}

func NewPasswordResetStore(q *sqlc.Queries) *PasswordResetStore {
	return &PasswordResetStore{q: q}
}

var _ port.PasswordResetRepository = (*PasswordResetStore)(nil)

func (s *PasswordResetStore) Create(ctx context.Context, reset *domain.PasswordReset) error {
	return s.q.CreatePasswordReset(ctx, sqlc.CreatePasswordResetParams{
		ID:        reset.ID,
		AccountID: reset.AccountID,
		TokenHash: reset.TokenHash,
		ExpiresAt: reset.ExpiresAt.Format(time.RFC3339),
	})
}

func (s *PasswordResetStore) GetByTokenHash(ctx context.Context, hash string) (*domain.PasswordReset, error) {
	r, err := s.q.GetPasswordResetByTokenHash(ctx, hash)
	if err != nil {
		return nil, mapScanErr(err)
	}
	reset := &domain.PasswordReset{
		ID:        r.ID,
		AccountID: r.AccountID,
		TokenHash: r.TokenHash,
		ExpiresAt: parseTime(r.ExpiresAt),
	}
	if r.UsedAt != nil && *r.UsedAt != "" {
		t := parseTime(*r.UsedAt)
		reset.UsedAt = &t
	}
	return reset, nil
}

func (s *PasswordResetStore) MarkUsed(ctx context.Context, id string) (bool, error) {
	rows, err := s.q.MarkPasswordResetUsed(ctx, id)
	if err != nil {
		return false, err
	}
	return rows > 0, nil
}

func (s *PasswordResetStore) DeleteExpired(ctx context.Context) error {
	_, err := s.q.DeleteUsedAndExpiredPasswordResets(ctx)
	return err
}
