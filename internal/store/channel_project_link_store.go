package store

import (
	"context"
	"database/sql"

	"ipmanlk/breeze/internal/domain"
	"ipmanlk/breeze/internal/port"
	"ipmanlk/breeze/internal/store/sqlc"
)

var _ port.ChannelProjectLinkRepository = (*ChannelProjectLinkStore)(nil)

type ChannelProjectLinkStore struct {
	q  *sqlc.Queries
	db *sql.DB
}

func NewChannelProjectLinkStore(q *sqlc.Queries, db *sql.DB) *ChannelProjectLinkStore {
	return &ChannelProjectLinkStore{q: q, db: db}
}

func (s *ChannelProjectLinkStore) Create(ctx context.Context, channelID, projectID string) error {
	return s.q.CreateChannelProjectLink(ctx, sqlc.CreateChannelProjectLinkParams{
		ChannelID: channelID,
		ProjectID: projectID,
	})
}

func (s *ChannelProjectLinkStore) Delete(ctx context.Context, channelID, projectID string) error {
	return s.q.DeleteChannelProjectLink(ctx, sqlc.DeleteChannelProjectLinkParams{
		ChannelID: channelID,
		ProjectID: projectID,
	})
}

func (s *ChannelProjectLinkStore) DeleteByChannel(ctx context.Context, channelID string) error {
	return s.q.DeleteChannelProjectLinks(ctx, channelID)
}

func (s *ChannelProjectLinkStore) GetByChannel(ctx context.Context, channelID string) ([]string, error) {
	rows, err := s.q.GetChannelProjectLinks(ctx, channelID)
	if err != nil {
		return nil, err
	}
	result := make([]string, len(rows))
	copy(result, rows)
	return result, nil
}

func (s *ChannelProjectLinkStore) GetByProject(ctx context.Context, projectID string) ([]string, error) {
	rows, err := s.q.GetProjectChannelLinks(ctx, projectID)
	if err != nil {
		return nil, err
	}
	result := make([]string, len(rows))
	copy(result, rows)
	return result, nil
}

func (s *ChannelProjectLinkStore) GetUsersWithProjectAccess(ctx context.Context, orgID, projectID string) ([]*domain.User, error) {
	rows, err := s.q.GetUsersWithProjectAccess(ctx, sqlc.GetUsersWithProjectAccessParams{
		OrgID:     orgID,
		ProjectID: projectID,
	})
	if err != nil {
		return nil, err
	}
	users := make([]*domain.User, len(rows))
	for i, row := range rows {
		users[i] = &domain.User{
			ID:        row.ID,
			Name:      row.Name,
			Email:     row.Email,
			AvatarURL: row.AvatarUrl,
			Role:      domain.Role(row.Role),
		}
	}
	return users, nil
}

func (s *ChannelProjectLinkStore) SetProjectLinks(ctx context.Context, channelID string, projectIDs []string) error {
	tx, err := s.db.BeginTx(ctx, nil)
	if err != nil {
		return err
	}
	defer tx.Rollback()

	q := s.q.WithTx(tx)

	if err := q.DeleteChannelProjectLinks(ctx, channelID); err != nil {
		return err
	}

	for _, pid := range projectIDs {
		if err := q.CreateChannelProjectLink(ctx, sqlc.CreateChannelProjectLinkParams{
			ChannelID: channelID,
			ProjectID: pid,
		}); err != nil {
			return err
		}
	}

	return tx.Commit()
}
