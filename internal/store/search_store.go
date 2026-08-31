package store

import (
	"context"
	"fmt"

	"ipmanlk/plume/internal/domain"
	"ipmanlk/plume/internal/port"
	"ipmanlk/plume/internal/store/sqlc"
)

type SearchStore struct {
	q *sqlc.Queries
}

func NewSearchStore(q *sqlc.Queries) *SearchStore {
	return &SearchStore{q: q}
}

var _ port.SearchRepository = (*SearchStore)(nil)

func (s *SearchStore) SearchProjects(ctx context.Context, orgID, query string, projectID string, limit int) ([]*domain.SearchResult, error) {
	rows, err := s.q.SearchProjects(ctx, sqlc.SearchProjectsParams{
		OrgID:     orgID,
		Query:     query,
		ProjectID: nilIfEmpty(projectID),
		Limit:     int64(limit),
	})
	if err != nil {
		return nil, err
	}
	results := make([]*domain.SearchResult, len(rows))
	for i, r := range rows {
		results[i] = &domain.SearchResult{
			ID:    r.ID,
			Type:  domain.SearchTypeProject,
			Name:  r.Name,
			URL:   fmt.Sprintf("/projects/%s", r.Slug),
			Color: r.Color,
		}
	}
	return results, nil
}

// SearchProjectsForUser scopes project search to projects the caller is an
// explicit member of. Used for viewer/guest users so they only discover
// projects they can access.
func (s *SearchStore) SearchProjectsForUser(ctx context.Context, orgID, userID, query string, limit int) ([]*domain.SearchResult, error) {
	rows, err := s.q.SearchProjectsForUser(ctx, sqlc.SearchProjectsForUserParams{
		OrgID:  orgID,
		UserID: userID,
		Query:  query,
		Limit:  int64(limit),
	})
	if err != nil {
		return nil, err
	}
	results := make([]*domain.SearchResult, len(rows))
	for i, r := range rows {
		results[i] = &domain.SearchResult{
			ID:    r.ID,
			Type:  domain.SearchTypeProject,
			Name:  r.Name,
			URL:   fmt.Sprintf("/projects/%s", r.Slug),
			Color: r.Color,
		}
	}
	return results, nil
}

func (s *SearchStore) SearchTasks(ctx context.Context, orgID, query string, projectID string, limit int) ([]*domain.SearchResult, error) {
	rows, err := s.q.SearchTasks(ctx, sqlc.SearchTasksParams{
		OrgID:     orgID,
		Query:     query,
		ProjectID: nilIfEmpty(projectID),
		Limit:     int64(limit),
	})
	if err != nil {
		return nil, err
	}
	results := make([]*domain.SearchResult, len(rows))
	for i, r := range rows {
		results[i] = &domain.SearchResult{
			ID:        r.ID,
			Type:      domain.SearchTypeTask,
			Name:      r.Title,
			Subtitle:  r.ProjectName,
			URL:       fmt.Sprintf("/projects/%s?task=%s", r.ProjectSlug, r.ID),
			Color:     r.ProjectColor,
			ProjectID: r.ProjectID,
		}
	}
	return results, nil
}

// SearchTasksForUser scopes task search to projects the caller is an explicit
// member of. Used for viewer/guest users so they only discover tasks in
// projects they can access.
func (s *SearchStore) SearchTasksForUser(ctx context.Context, orgID, userID, query string, projectID string, limit int) ([]*domain.SearchResult, error) {
	rows, err := s.q.SearchTasksForUser(ctx, sqlc.SearchTasksForUserParams{
		OrgID:     orgID,
		UserID:    userID,
		Query:     query,
		ProjectID: nilIfEmpty(projectID),
		Limit:     int64(limit),
	})
	if err != nil {
		return nil, err
	}
	results := make([]*domain.SearchResult, len(rows))
	for i, r := range rows {
		results[i] = &domain.SearchResult{
			ID:        r.ID,
			Type:      domain.SearchTypeTask,
			Name:      r.Title,
			Subtitle:  r.ProjectName,
			URL:       fmt.Sprintf("/projects/%s?task=%s", r.ProjectSlug, r.ID),
			Color:     r.ProjectColor,
			ProjectID: r.ProjectID,
		}
	}
	return results, nil
}

// SearchChannels scopes results to channels the caller can actually see:
// explicit membership, or when includeProjectLinked is set (elevated org
// roles), project-linked channels and their descendants. This mirrors the
// visibility rule of the conversation list (see queries/search.sql).
func (s *SearchStore) SearchChannels(ctx context.Context, orgID, userID string, includeProjectLinked bool, query string, limit int) ([]*domain.SearchResult, error) {
	inclProjectLinked := int64(0)
	if includeProjectLinked {
		inclProjectLinked = 1
	}
	rows, err := s.q.SearchChannels(ctx, sqlc.SearchChannelsParams{
		OrgID:                orgID,
		UserID:               userID,
		IncludeProjectLinked: inclProjectLinked,
		Query:                query,
		Limit:                int64(limit),
	})
	if err != nil {
		return nil, err
	}
	results := make([]*domain.SearchResult, len(rows))
	for i, r := range rows {
		results[i] = &domain.SearchResult{
			ID:   r.ID,
			Type: domain.SearchTypeChannel,
			Name: fmt.Sprintf("#%s", r.Name),
			URL:  fmt.Sprintf("/chat/%s", r.ID),
		}
	}
	return results, nil
}

func (s *SearchStore) SearchDirectMessages(ctx context.Context, orgID, userID, query string, limit int) ([]*domain.SearchResult, error) {
	rows, err := s.q.SearchDirectMessages(ctx, sqlc.SearchDirectMessagesParams{
		OrgID:  orgID,
		UserID: userID,
		Query:  query,
		Limit:  int64(limit),
	})
	if err != nil {
		return nil, err
	}
	results := make([]*domain.SearchResult, len(rows))
	for i, r := range rows {
		results[i] = &domain.SearchResult{
			ID:   r.ID,
			Type: domain.SearchTypeDirectMessage,
			Name: r.PartnerName,
			URL:  fmt.Sprintf("/messages/%s", r.ID),
		}
	}
	return results, nil
}

func (s *SearchStore) SearchMembers(ctx context.Context, orgID, query string, limit int) ([]*domain.SearchResult, error) {
	rows, err := s.q.SearchMembers(ctx, sqlc.SearchMembersParams{
		OrgID: orgID,
		Query: query,
		Limit: int64(limit),
	})
	if err != nil {
		return nil, err
	}
	results := make([]*domain.SearchResult, len(rows))
	for i, r := range rows {
		avatar := ""
		if r.AvatarUrl != nil {
			avatar = *r.AvatarUrl
		}
		results[i] = &domain.SearchResult{
			ID:   r.ID,
			Type: domain.SearchTypeMember,
			Name: r.Name,
			Icon: avatar,
		}
	}
	return results, nil
}

func (s *SearchStore) RecentProjects(ctx context.Context, orgID string, limit int) ([]*domain.SearchResult, error) {
	rows, err := s.q.RecentProjects(ctx, sqlc.RecentProjectsParams{
		OrgID: orgID,
		Limit: int64(limit),
	})
	if err != nil {
		return nil, err
	}
	results := make([]*domain.SearchResult, len(rows))
	for i, r := range rows {
		results[i] = &domain.SearchResult{
			ID:    r.ID,
			Type:  domain.SearchTypeProject,
			Name:  r.Name,
			URL:   fmt.Sprintf("/projects/%s", r.Slug),
			Color: r.Color,
		}
	}
	return results, nil
}

// RecentProjectsForUser scopes the recent-projects list to projects the
// caller is an explicit member of. Used for viewer/guest users.
func (s *SearchStore) RecentProjectsForUser(ctx context.Context, orgID, userID string, limit int) ([]*domain.SearchResult, error) {
	rows, err := s.q.RecentProjectsForUser(ctx, sqlc.RecentProjectsForUserParams{
		OrgID:  orgID,
		UserID: userID,
		Limit:  int64(limit),
	})
	if err != nil {
		return nil, err
	}
	results := make([]*domain.SearchResult, len(rows))
	for i, r := range rows {
		results[i] = &domain.SearchResult{
			ID:    r.ID,
			Type:  domain.SearchTypeProject,
			Name:  r.Name,
			URL:   fmt.Sprintf("/projects/%s", r.Slug),
			Color: r.Color,
		}
	}
	return results, nil
}

func (s *SearchStore) RecentTasks(ctx context.Context, orgID string, limit int) ([]*domain.SearchResult, error) {
	rows, err := s.q.RecentTasks(ctx, sqlc.RecentTasksParams{
		OrgID: orgID,
		Limit: int64(limit),
	})
	if err != nil {
		return nil, err
	}
	results := make([]*domain.SearchResult, len(rows))
	for i, r := range rows {
		results[i] = &domain.SearchResult{
			ID:        r.ID,
			Type:      domain.SearchTypeTask,
			Name:      r.Title,
			Subtitle:  r.ProjectName,
			URL:       fmt.Sprintf("/projects/%s?task=%s", r.ProjectSlug, r.ID),
			Color:     r.ProjectColor,
			ProjectID: r.ProjectID,
		}
	}
	return results, nil
}

// RecentTasksForUser scopes recent tasks to projects the caller is an explicit
// member of. Used for viewer/guest users.
func (s *SearchStore) RecentTasksForUser(ctx context.Context, orgID, userID string, limit int) ([]*domain.SearchResult, error) {
	rows, err := s.q.RecentTasksForUser(ctx, sqlc.RecentTasksForUserParams{
		OrgID:  orgID,
		UserID: userID,
		Limit:  int64(limit),
	})
	if err != nil {
		return nil, err
	}
	results := make([]*domain.SearchResult, len(rows))
	for i, r := range rows {
		results[i] = &domain.SearchResult{
			ID:        r.ID,
			Type:      domain.SearchTypeTask,
			Name:      r.Title,
			Subtitle:  r.ProjectName,
			URL:       fmt.Sprintf("/projects/%s?task=%s", r.ProjectSlug, r.ID),
			Color:     r.ProjectColor,
			ProjectID: r.ProjectID,
		}
	}
	return results, nil
}
