package store

import (
	"context"
	"database/sql"

	"ipmanlk/breeze/internal/domain"
	"ipmanlk/breeze/internal/port"
	"ipmanlk/breeze/internal/store/sqlc"

	"github.com/google/uuid"
)

type InviteStore struct {
	q  *sqlc.Queries
	db *sql.DB
}

func NewInviteStore(q *sqlc.Queries, db *sql.DB) *InviteStore {
	return &InviteStore{q: q, db: db}
}

var _ port.UserInviteRepository = (*InviteStore)(nil)

func (s *InviteStore) toDomain(i sqlc.UserInvite) *domain.UserInvite {
	maxUses := intPtr(i.MaxUses)
	return &domain.UserInvite{
		ID:        i.ID,
		OrgID:     i.OrgID,
		Email:     i.Email,
		Role:      domain.Role(i.Role),
		TokenHash: i.TokenHash,
		InvitedBy: i.InvitedBy,
		MaxUses:   maxUses,
		UseCount:  int(i.UseCount),
		ExpiresAt: parseTime(i.ExpiresAt),
		CreatedAt: parseTime(i.CreatedAt),
	}
}

func (s *InviteStore) Create(ctx context.Context, invite *domain.UserInvite) error {
	var maxUses *int64
	if invite.MaxUses != nil {
		v := int64(*invite.MaxUses)
		maxUses = &v
	}
	return s.q.CreateUserInvite(ctx, sqlc.CreateUserInviteParams{
		ID:        invite.ID,
		OrgID:     invite.OrgID,
		Email:     invite.Email,
		Role:      string(invite.Role),
		TokenHash: invite.TokenHash,
		InvitedBy: invite.InvitedBy,
		MaxUses:   maxUses,
		ExpiresAt: formatTime(invite.ExpiresAt),
	})
}

func (s *InviteStore) GetByTokenHash(ctx context.Context, hash string) (*domain.UserInvite, error) {
	i, err := s.q.GetUserInviteByTokenHash(ctx, hash)
	if err != nil {
		return nil, mapScanErr(err)
	}
	return s.toDomain(i), nil
}

func (s *InviteStore) ListByOrg(ctx context.Context, orgID string, limit int) ([]*domain.UserInvite, error) {
	if limit <= 0 {
		limit = 50
	}
	rows, err := s.q.ListUserInvites(ctx, sqlc.ListUserInvitesParams{
		OrgID:    orgID,
		LimitVal: int64(limit),
	})
	if err != nil {
		return nil, err
	}
	items := make([]*domain.UserInvite, len(rows))
	for i, r := range rows {
		items[i] = &domain.UserInvite{
			ID:        r.ID,
			OrgID:     r.OrgID,
			Email:     r.Email,
			Role:      domain.Role(r.Role),
			InvitedBy: r.InvitedBy,
			UseCount:  int(r.UseCount),
			ExpiresAt: parseTime(r.ExpiresAt),
			CreatedAt: parseTime(r.CreatedAt),
		}
	}
	return items, nil
}

func (s *InviteStore) Delete(ctx context.Context, orgID, id string) error {
	return s.q.DeleteUserInvite(ctx, sqlc.DeleteUserInviteParams{
		ID:    id,
		OrgID: orgID,
	})
}

func (s *InviteStore) IncrementUseCount(ctx context.Context, id string) error {
	return s.q.IncrementInviteUseCount(ctx, id)
}

func (s *InviteStore) RecordAcceptance(ctx context.Context, inviteID, userID string) error {
	return s.q.RecordInviteAcceptance(ctx, sqlc.RecordInviteAcceptanceParams{
		ID:       uuid.New().String(),
		InviteID: inviteID,
		UserID:   userID,
	})
}

func (s *InviteStore) AddInviteProject(ctx context.Context, inviteID, projectID string, role domain.Role) error {
	return s.q.AddInviteProject(ctx, sqlc.AddInviteProjectParams{
		InviteID:  inviteID,
		ProjectID: projectID,
		Role:      string(role),
	})
}

func (s *InviteStore) ListInviteProjects(ctx context.Context, inviteID string) ([]*domain.InviteProjectAssignment, error) {
	rows, err := s.q.ListInviteProjects(ctx, inviteID)
	if err != nil {
		return nil, err
	}
	items := make([]*domain.InviteProjectAssignment, len(rows))
	for i, r := range rows {
		items[i] = &domain.InviteProjectAssignment{
			ProjectID: r.ProjectID,
			Role:      domain.Role(r.Role),
		}
	}
	return items, nil
}

func (s *InviteStore) DeleteInviteProjects(ctx context.Context, inviteID string) error {
	return s.q.DeleteInviteProjects(ctx, inviteID)
}

// AcceptInvite atomically increments the invite use count and records the
// acceptance. This prevents a partial state where the invite is consumed
// but acceptance is not recorded, or vice versa.
func (s *InviteStore) AcceptInvite(ctx context.Context, inviteID, userID string) error {
	tx, err := s.db.BeginTx(ctx, nil)
	if err != nil {
		return err
	}
	defer tx.Rollback()

	q := s.q.WithTx(tx)

	if err := q.IncrementInviteUseCount(ctx, inviteID); err != nil {
		return err
	}

	if err := q.RecordInviteAcceptance(ctx, sqlc.RecordInviteAcceptanceParams{
		ID:       uuid.New().String(),
		InviteID: inviteID,
		UserID:   userID,
	}); err != nil {
		return err
	}

	return tx.Commit()
}
