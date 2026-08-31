package service

import (
	"context"
	"strings"

	"ipmanlk/plume/internal/apperr"
	"ipmanlk/plume/internal/domain"
	"ipmanlk/plume/internal/port"
)

type SearchService struct {
	repo port.SearchRepository
}

var _ port.SearchService = (*SearchService)(nil)

func NewSearchService(repo port.SearchRepository) *SearchService {
	return &SearchService{repo: repo}
}

func (s *SearchService) Search(ctx context.Context, params domain.SearchParams) ([]*domain.SearchResult, error) {
	limit := params.Limit
	if limit <= 0 || limit > 50 {
		limit = 20
	}

	typeSet := make(map[domain.SearchType]bool, len(params.Types))
	for _, t := range params.Types {
		typeSet[t] = true
	}
	if len(typeSet) == 0 {
		typeSet[domain.SearchTypeProject] = true
		typeSet[domain.SearchTypeTask] = true
	}

	// Project-scoped roles (viewer/guest) only discover tasks in projects they
	// can access. Elevated org roles (owner/admin/member) search org-wide.
	projectScoped := !domain.IsOrgElevatedRole(params.Role)

	if params.Query == "" {
		return s.recentResults(ctx, params, typeSet, limit, projectScoped)
	}

	var all []*domain.SearchResult
	mapFTSErr := func(err error) error {
		// FTS5 syntax errors are common with user input (quotes, operators).
		// Surface them as validation errors instead of 500s.
		if strings.Contains(err.Error(), "fts5") && strings.Contains(err.Error(), "syntax error") {
			return apperr.InvalidInput("invalid search query syntax")
		}
		return err
	}

	if typeSet[domain.SearchTypeProject] {
		var results []*domain.SearchResult
		var err error
		if projectScoped {
			results, err = s.repo.SearchProjectsForUser(ctx, params.OrgID, params.UserID, params.Query, limit)
		} else {
			results, err = s.repo.SearchProjects(ctx, params.OrgID, params.Query, params.ProjectID, limit)
		}
		if err != nil {
			return nil, mapFTSErr(err)
		}
		all = append(all, results...)
	}

	if typeSet[domain.SearchTypeTask] {
		var results []*domain.SearchResult
		var err error
		if projectScoped {
			results, err = s.repo.SearchTasksForUser(ctx, params.OrgID, params.UserID, params.Query, params.ProjectID, limit)
		} else {
			results, err = s.repo.SearchTasks(ctx, params.OrgID, params.Query, params.ProjectID, limit)
		}
		if err != nil {
			return nil, mapFTSErr(err)
		}
		all = append(all, results...)
	}

	if typeSet[domain.SearchTypeChannel] {
		// Channel search follows conversation-list visibility: elevated org
		// roles also see project-linked channels (and their descendants);
		// viewer/guest roles only see channels they are explicit members of.
		results, err := s.repo.SearchChannels(ctx, params.OrgID, params.UserID, !projectScoped, params.Query, limit)
		if err != nil {
			return nil, mapFTSErr(err)
		}
		all = append(all, results...)
	}

	if typeSet[domain.SearchTypeDirectMessage] {
		results, err := s.repo.SearchDirectMessages(ctx, params.OrgID, params.UserID, params.Query, limit)
		if err != nil {
			return nil, err
		}
		all = append(all, results...)
	}

	if typeSet[domain.SearchTypeMember] {
		results, err := s.repo.SearchMembers(ctx, params.OrgID, params.Query, limit)
		if err != nil {
			return nil, err
		}
		all = append(all, results...)
	}

	return all, nil
}

func (s *SearchService) recentResults(ctx context.Context, params domain.SearchParams, typeSet map[domain.SearchType]bool, limit int, projectScoped bool) ([]*domain.SearchResult, error) {
	var all []*domain.SearchResult

	if typeSet[domain.SearchTypeProject] {
		var results []*domain.SearchResult
		var err error
		if projectScoped {
			results, err = s.repo.RecentProjectsForUser(ctx, params.OrgID, params.UserID, limit)
		} else {
			results, err = s.repo.RecentProjects(ctx, params.OrgID, limit)
		}
		if err != nil {
			return nil, err
		}
		all = append(all, results...)
	}

	if typeSet[domain.SearchTypeTask] {
		var results []*domain.SearchResult
		var err error
		if projectScoped {
			results, err = s.repo.RecentTasksForUser(ctx, params.OrgID, params.UserID, limit)
		} else {
			results, err = s.repo.RecentTasks(ctx, params.OrgID, limit)
		}
		if err != nil {
			return nil, err
		}
		all = append(all, results...)
	}

	return all, nil
}
