package store

import (
	"context"

	"ipmanlk/breeze/internal/domain"
	"ipmanlk/breeze/internal/port"
	"ipmanlk/breeze/internal/store/sqlc"
)

// AccountStore implements port.AccountRepository over sqlc. Accounts own the
// credential (password_hash) + globally-unique login key (email). See
// docs/api/workspaces.md.
type AccountStore struct {
	q *sqlc.Queries
}

func NewAccountStore(q *sqlc.Queries) *AccountStore {
	return &AccountStore{q: q}
}

var _ port.AccountRepository = (*AccountStore)(nil)

func accountToDomain(a sqlc.Account) domain.Account {
	return domain.Account{
		ID:           a.ID,
		Email:        a.Email,
		PasswordHash: a.PasswordHash,
		CreatedAt:    parseTime(a.CreatedAt),
		UpdatedAt:    parseTime(a.UpdatedAt),
	}
}

func (s *AccountStore) GetByEmail(ctx context.Context, email string) (*domain.Account, error) {
	a, err := s.q.GetAccountByEmail(ctx, email)
	if err != nil {
		return nil, mapScanErr(err)
	}
	d := accountToDomain(a)
	return &d, nil
}

func (s *AccountStore) GetByID(ctx context.Context, id string) (*domain.Account, error) {
	a, err := s.q.GetAccountByID(ctx, id)
	if err != nil {
		return nil, mapScanErr(err)
	}
	d := accountToDomain(a)
	return &d, nil
}

func (s *AccountStore) Create(ctx context.Context, account *domain.Account) error {
	return s.q.CreateAccount(ctx, sqlc.CreateAccountParams{
		ID:           account.ID,
		Email:        account.Email,
		PasswordHash: account.PasswordHash,
	})
}

func (s *AccountStore) UpdatePassword(ctx context.Context, id, passwordHash string) error {
	return s.q.UpdateAccountPassword(ctx, sqlc.UpdateAccountPasswordParams{
		ID:           id,
		PasswordHash: passwordHash,
	})
}

func (s *AccountStore) Exists(ctx context.Context) (bool, error) {
	return s.q.AccountExists(ctx)
}
