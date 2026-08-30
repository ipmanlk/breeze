package port

import (
	"context"

	"ipmanlk/breeze/internal/domain"
)

// MentionSearchRepository provides mention autocomplete search across users,
// channels, projects, and tasks. Implemented by the store layer; consumed by
// MentionService.
type MentionSearchRepository interface {
	SearchUsers(ctx context.Context, orgID, query string, limit int64) ([]domain.MentionUserResult, error)
	SearchChannels(ctx context.Context, userID, orgID, query string, limit int64) ([]domain.MentionChannelResult, error)
	SearchProjects(ctx context.Context, orgID, query string, limit int64) ([]domain.MentionProjectResult, error)
	SearchProjectsForUser(ctx context.Context, userID, orgID, query string, limit int64) ([]domain.MentionProjectResult, error)
	SearchTasks(ctx context.Context, orgID, query string, limit int64) ([]domain.MentionTaskResult, error)
	SearchTasksForUser(ctx context.Context, orgID, userID, query string, limit int64) ([]domain.MentionTaskResult, error)
}
