package store

import (
	"context"

	"ipmanlk/breeze/internal/domain"
	"ipmanlk/breeze/internal/store/sqlc"
)

// MentionSearchStore wraps sqlc mention-search queries, converting sqlc row
// types to domain types so the service layer never imports sqlc.
type MentionSearchStore struct {
	q *sqlc.Queries
}

func NewMentionSearchStore(q *sqlc.Queries) *MentionSearchStore {
	return &MentionSearchStore{q: q}
}

func (s *MentionSearchStore) SearchUsers(ctx context.Context, orgID, query string, limit int64) ([]domain.MentionUserResult, error) {
	rows, err := s.q.SearchMentionUsers(ctx, sqlc.SearchMentionUsersParams{
		OrgID:    orgID,
		Query:    query,
		LimitVal: limit,
	})
	if err != nil {
		return nil, err
	}
	out := make([]domain.MentionUserResult, len(rows))
	for i, u := range rows {
		out[i] = domain.MentionUserResult{
			ID:        u.ID,
			Name:      u.Name,
			AvatarURL: u.AvatarUrl,
		}
	}
	return out, nil
}

func (s *MentionSearchStore) SearchChannels(ctx context.Context, userID, orgID, query string, limit int64) ([]domain.MentionChannelResult, error) {
	rows, err := s.q.SearchMentionChannels(ctx, sqlc.SearchMentionChannelsParams{
		UserID:   userID,
		OrgID:    orgID,
		Query:    query,
		LimitVal: limit,
	})
	if err != nil {
		return nil, err
	}
	out := make([]domain.MentionChannelResult, len(rows))
	for i, c := range rows {
		out[i] = domain.MentionChannelResult{ID: c.ID, Name: c.Name}
	}
	return out, nil
}

func (s *MentionSearchStore) SearchProjects(ctx context.Context, orgID, query string, limit int64) ([]domain.MentionProjectResult, error) {
	rows, err := s.q.SearchMentionProjects(ctx, sqlc.SearchMentionProjectsParams{
		OrgID:    orgID,
		Query:    query,
		LimitVal: limit,
	})
	if err != nil {
		return nil, err
	}
	out := make([]domain.MentionProjectResult, len(rows))
	for i, p := range rows {
		out[i] = domain.MentionProjectResult{
			ID:    p.ID,
			Name:  p.Name,
			Icon:  p.Icon,
			Color: p.Color,
		}
	}
	return out, nil
}

func (s *MentionSearchStore) SearchProjectsForUser(ctx context.Context, userID, orgID, query string, limit int64) ([]domain.MentionProjectResult, error) {
	rows, err := s.q.SearchMentionProjectsForUser(ctx, sqlc.SearchMentionProjectsForUserParams{
		UserID:   userID,
		OrgID:    orgID,
		Query:    query,
		LimitVal: limit,
	})
	if err != nil {
		return nil, err
	}
	out := make([]domain.MentionProjectResult, len(rows))
	for i, p := range rows {
		out[i] = domain.MentionProjectResult{
			ID:    p.ID,
			Name:  p.Name,
			Icon:  p.Icon,
			Color: p.Color,
		}
	}
	return out, nil
}

func (s *MentionSearchStore) SearchTasks(ctx context.Context, orgID, query string, limit int64) ([]domain.MentionTaskResult, error) {
	rows, err := s.q.SearchMentionTasks(ctx, sqlc.SearchMentionTasksParams{
		OrgID:    orgID,
		Query:    query,
		LimitVal: limit,
	})
	if err != nil {
		return nil, err
	}
	out := make([]domain.MentionTaskResult, len(rows))
	for i, t := range rows {
		out[i] = domain.MentionTaskResult{
			ID:          t.ID,
			Title:       t.Title,
			ProjectID:   t.ProjectID,
			ProjectName: t.ProjectName,
		}
	}
	return out, nil
}

func (s *MentionSearchStore) SearchTasksForUser(ctx context.Context, orgID, userID, query string, limit int64) ([]domain.MentionTaskResult, error) {
	rows, err := s.q.SearchMentionTasksForUser(ctx, sqlc.SearchMentionTasksForUserParams{
		OrgID:    orgID,
		UserID:   userID,
		Query:    query,
		LimitVal: limit,
	})
	if err != nil {
		return nil, err
	}
	out := make([]domain.MentionTaskResult, len(rows))
	for i, t := range rows {
		out[i] = domain.MentionTaskResult{
			ID:          t.ID,
			Title:       t.Title,
			ProjectID:   t.ProjectID,
			ProjectName: t.ProjectName,
		}
	}
	return out, nil
}
