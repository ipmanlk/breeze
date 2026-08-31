package store

import (
	"context"
	"database/sql"

	"ipmanlk/plume/internal/domain"
	"ipmanlk/plume/internal/port"
	"ipmanlk/plume/internal/store/sqlc"
)

type OrgStore struct {
	q  *sqlc.Queries
	db *sql.DB
}

func NewOrgStore(q *sqlc.Queries, db *sql.DB) *OrgStore {
	return &OrgStore{q: q, db: db}
}

var _ port.OrganizationRepository = (*OrgStore)(nil)

func (s *OrgStore) toDomain(o sqlc.Organization) domain.Organization {
	return domain.Organization{
		ID:                      o.ID,
		Name:                    o.Name,
		Slug:                    o.Slug,
		MessageEditWindowMinute: int(o.MessageEditWindowMinutes),
		CreatedAt:               parseTime(o.CreatedAt),
		UpdatedAt:               parseTime(o.UpdatedAt),
	}
}

func (s *OrgStore) GetByID(ctx context.Context, id string) (*domain.Organization, error) {
	o, err := s.q.GetOrganizationByID(ctx, id)
	if err != nil {
		return nil, mapScanErr(err)
	}
	d := s.toDomain(o)
	return &d, nil
}

func (s *OrgStore) GetBySlug(ctx context.Context, slug string) (*domain.Organization, error) {
	o, err := s.q.GetOrganizationBySlug(ctx, slug)
	if err != nil {
		return nil, mapScanErr(err)
	}
	d := domain.Organization{
		ID:                      o.ID,
		Name:                    o.Name,
		Slug:                    o.Slug,
		MessageEditWindowMinute: int(o.MessageEditWindowMinutes),
		CreatedAt:               parseTime(o.CreatedAt),
		UpdatedAt:               parseTime(o.UpdatedAt),
	}
	return &d, nil
}

func (s *OrgStore) Create(ctx context.Context, org *domain.Organization) error {
	return s.q.CreateOrganization(ctx, sqlc.CreateOrganizationParams{
		ID:   org.ID,
		Name: org.Name,
		Slug: org.Slug,
	})
}

func (s *OrgStore) Exists(ctx context.Context) (bool, error) {
	return s.q.OrganizationExists(ctx)
}

func (s *OrgStore) Count(ctx context.Context) (int64, error) {
	return s.q.OrganizationCount(ctx)
}

func (s *OrgStore) Update(ctx context.Context, org *domain.Organization) error {
	return s.q.UpdateOrganization(ctx, sqlc.UpdateOrganizationParams{
		ID:                       org.ID,
		Name:                     org.Name,
		Slug:                     org.Slug,
		MessageEditWindowMinutes: int64(org.MessageEditWindowMinute),
	})
}

func (s *OrgStore) Delete(ctx context.Context, id string) error {
	return s.q.DeleteOrganization(ctx, id)
}

// CreateOrgWithAccountAndUser creates an organization, account, and user
// atomically in a single transaction.
func (s *OrgStore) CreateOrgWithAccountAndUser(ctx context.Context, org *domain.Organization, accountID, userID, passwordHash, adminEmail, adminName string) error {
	tx, err := s.db.BeginTx(ctx, nil)
	if err != nil {
		return err
	}
	defer tx.Rollback()

	q := s.q.WithTx(tx)

	if err := q.CreateOrganization(ctx, sqlc.CreateOrganizationParams{
		ID:   org.ID,
		Name: org.Name,
		Slug: org.Slug,
	}); err != nil {
		return err
	}

	if err := q.CreateAccount(ctx, sqlc.CreateAccountParams{
		ID:           accountID,
		Email:        adminEmail,
		PasswordHash: passwordHash,
	}); err != nil {
		return err
	}

	if err := q.CreateUser(ctx, sqlc.CreateUserParams{
		ID:        userID,
		AccountID: &accountID,
		OrgID:     org.ID,
		Email:     adminEmail,
		Name:      adminName,
		Role:      string(domain.RoleOwner),
		IsActive:  boolToInt64(true),
	}); err != nil {
		return err
	}

	return tx.Commit()
}

// CreateOrgWithUser creates an organization and user atomically in a single
// transaction. Used when the account already exists.
func (s *OrgStore) CreateOrgWithUser(ctx context.Context, org *domain.Organization, userID, accountID, displayName, email string, avatarURL *string) error {
	tx, err := s.db.BeginTx(ctx, nil)
	if err != nil {
		return err
	}
	defer tx.Rollback()

	q := s.q.WithTx(tx)

	if err := q.CreateOrganization(ctx, sqlc.CreateOrganizationParams{
		ID:   org.ID,
		Name: org.Name,
		Slug: org.Slug,
	}); err != nil {
		return err
	}

	if err := q.CreateUser(ctx, sqlc.CreateUserParams{
		ID:        userID,
		AccountID: &accountID,
		OrgID:     org.ID,
		Email:     email,
		Name:      displayName,
		Role:      string(domain.RoleOwner),
		AvatarUrl: avatarURL,
		IsActive:  boolToInt64(true),
	}); err != nil {
		return err
	}

	return tx.Commit()
}

// ListForAccount returns every org the given account is a member of, paired
// with the account's role and active flag in that org. Used by the workspace
// switcher list. Joins users(membership) on organizations.
func (s *OrgStore) ListForAccount(ctx context.Context, accountID string) ([]*domain.Workspace, error) {
	rows, err := s.q.ListOrganizationsByAccount(ctx, &accountID)
	if err != nil {
		return nil, err
	}
	out := make([]*domain.Workspace, len(rows))
	for i, r := range rows {
		org := domain.Organization{
			ID:                      r.ID,
			Name:                    r.Name,
			Slug:                    r.Slug,
			MessageEditWindowMinute: int(r.MessageEditWindowMinutes),
			CreatedAt:               parseTime(r.CreatedAt),
			UpdatedAt:               parseTime(r.UpdatedAt),
		}
		role := domain.Role(r.UserRole)
		out[i] = &domain.Workspace{
			Organization: org,
			Role:         role,
			IsOwner:      role == domain.RoleOwner,
		}
	}
	return out, nil
}
