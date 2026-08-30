package service

import (
	"context"
	"strings"

	"ipmanlk/breeze/internal/domain"
	"ipmanlk/breeze/internal/port"
)

// MentionService handles @mention autocomplete search across all mentionable entity types.
//
// Access control:
//   - Users: all active org users are visible.
//   - Channels: only channels the requesting user is a member of.
//   - Projects/Tasks: org owners/admins/members see all; viewers only see
//     projects/tasks they are explicitly a project_member of.
type MentionService struct {
	mentionRepo port.MentionSearchRepository
	convRepo    port.ConversationRepository
}

var _ port.MentionService = (*MentionService)(nil)

func NewMentionService(mentionRepo port.MentionSearchRepository, convRepo port.ConversationRepository) *MentionService {
	return &MentionService{mentionRepo: mentionRepo, convRepo: convRepo}
}

// Search returns mention suggestions across all requested types, respecting access control.
//
// Parameters:
//   - orgID: the organization the request belongs to
//   - userID: the requesting user's ID (for access-scoped queries)
//   - userRole: the requesting user's org role ("owner","admin","member","viewer")
//   - query: the search text typed after @
//   - types: which entity types to include (empty = all)
//   - limit: max total results (capped at 20)
func (s *MentionService) Search(
	ctx context.Context,
	orgID, userID, userRole, query string,
	types []domain.MentionType,
	limit int,
) ([]*domain.MentionResult, error) {
	if limit <= 0 || limit > 20 {
		limit = 10
	}
	q := strings.TrimSpace(query)

	// Default to all types if none specified
	if len(types) == 0 {
		types = []domain.MentionType{
			domain.MentionEveryone,
			domain.MentionUser,
			domain.MentionChannel,
			domain.MentionProject,
			domain.MentionTask,
		}
	}

	// Distribute limit roughly across types so no single type hogs results.
	// @everyone is never more than 1 result and only shown when query matches.
	perType := limit / len(types)
	if perType < 2 {
		perType = 2
	}

	var results []*domain.MentionResult

	isPrivileged := userRole == string(domain.RoleOwner) ||
		userRole == string(domain.RoleAdmin) ||
		userRole == string(domain.RoleMember)

	for _, t := range types {
		if len(results) >= limit {
			break
		}
		remaining := limit - len(results)
		if remaining <= 0 {
			break
		}
		typeLimit := perType
		if typeLimit > remaining {
			typeLimit = remaining
		}

		switch t {
		case domain.MentionEveryone:
			// @everyone: only show when query explicitly matches "everyone"
			if q != "" && strings.Contains("everyone", strings.ToLower(q)) {
				results = append(results, &domain.MentionResult{
					ID:    "@everyone",
					Type:  domain.MentionEveryone,
					Label: "everyone",
				})
			}

		case domain.MentionUser:
			users, err := s.mentionRepo.SearchUsers(ctx, orgID, q, int64(typeLimit))
			if err == nil {
				for _, u := range users {
					results = append(results, &domain.MentionResult{
						ID:        u.ID,
						Type:      domain.MentionUser,
						Label:     u.Name,
						AvatarURL: u.AvatarURL,
					})
				}
			}

		case domain.MentionChannel:
			channels, err := s.mentionRepo.SearchChannels(ctx, userID, orgID, q, int64(typeLimit))
			if err == nil {
				for _, c := range channels {
					results = append(results, &domain.MentionResult{
						ID:    c.ID,
						Type:  domain.MentionChannel,
						Label: c.Name,
					})
				}
			}

		case domain.MentionProject:
			if isPrivileged {
				projects, err := s.mentionRepo.SearchProjects(ctx, orgID, q, int64(typeLimit))
				if err == nil {
					for _, p := range projects {
						results = append(results, &domain.MentionResult{
							ID:    p.ID,
							Type:  domain.MentionProject,
							Label: p.Name,
						})
					}
				}
			} else {
				projects, err := s.mentionRepo.SearchProjectsForUser(ctx, userID, orgID, q, int64(typeLimit))
				if err == nil {
					for _, p := range projects {
						results = append(results, &domain.MentionResult{
							ID:    p.ID,
							Type:  domain.MentionProject,
							Label: p.Name,
						})
					}
				}
			}

		case domain.MentionTask:
			if isPrivileged {
				tasks, err := s.mentionRepo.SearchTasks(ctx, orgID, q, int64(typeLimit))
				if err == nil {
					for _, t := range tasks {
						pName := t.ProjectName
						pID := t.ProjectID
						results = append(results, &domain.MentionResult{
							ID:          t.ID,
							Type:        domain.MentionTask,
							Label:       t.Title,
							ProjectID:   &pID,
							ProjectName: &pName,
						})
					}
				}
			} else {
				tasks, err := s.mentionRepo.SearchTasksForUser(ctx, orgID, userID, q, int64(typeLimit))
				if err == nil {
					for _, t := range tasks {
						pName := t.ProjectName
						pID := t.ProjectID
						results = append(results, &domain.MentionResult{
							ID:          t.ID,
							Type:        domain.MentionTask,
							Label:       t.Title,
							ProjectID:   &pID,
							ProjectName: &pName,
						})
					}
				}
			}
		}
	}

	return results, nil
}
