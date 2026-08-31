package store

import (
	"context"

	"ipmanlk/plume/internal/domain"
	"ipmanlk/plume/internal/port"
	"ipmanlk/plume/internal/store/sqlc"
)

type DashboardStore struct {
	q *sqlc.Queries
}

func NewDashboardStore(q *sqlc.Queries) *DashboardStore {
	return &DashboardStore{q: q}
}

var _ port.DashboardRepository = (*DashboardStore)(nil)

func (s *DashboardStore) MyTasks(ctx context.Context, orgID, userID string, limit int) ([]*domain.DashboardTask, error) {
	rows, err := s.q.MyTasks(ctx, sqlc.MyTasksParams{
		UserID: userID,
		OrgID:  orgID,
		Limit:  int64(limit),
	})
	if err != nil {
		return nil, err
	}
	results := make([]*domain.DashboardTask, len(rows))
	for i, r := range rows {
		results[i] = &domain.DashboardTask{
			ID:          r.ID,
			Title:       r.Title,
			Priority:    r.Priority,
			StatusID:    r.StatusID,
			StatusName:  r.StatusName,
			StatusColor: r.StatusColor,
			ProjectID:   r.ProjectID,
			ProjectName: r.ProjectName,
			ProjectSlug: r.ProjectSlug,
			DueAt:       parseTimePtr(r.DueAt),
		}
	}
	return results, nil
}

func (s *DashboardStore) DueSoonTasks(ctx context.Context, orgID, userID string, limit int) ([]*domain.DashboardTask, error) {
	rows, err := s.q.DueSoonTasks(ctx, sqlc.DueSoonTasksParams{
		UserID: userID,
		OrgID:  orgID,
		Limit:  int64(limit),
	})
	if err != nil {
		return nil, err
	}
	results := make([]*domain.DashboardTask, len(rows))
	for i, r := range rows {
		results[i] = &domain.DashboardTask{
			ID:          r.ID,
			Title:       r.Title,
			Priority:    r.Priority,
			StatusID:    r.StatusID,
			StatusName:  r.StatusName,
			StatusColor: r.StatusColor,
			ProjectID:   r.ProjectID,
			ProjectName: r.ProjectName,
			ProjectSlug: r.ProjectSlug,
			DueAt:       parseTimePtr(r.DueAt),
		}
	}
	return results, nil
}

func (s *DashboardStore) MyTaskStats(ctx context.Context, orgID, userID string) (*domain.DashboardStats, error) {
	row, err := s.q.MyTaskStats(ctx, sqlc.MyTaskStatsParams{
		UserID: userID,
		OrgID:  orgID,
	})
	if err != nil {
		return nil, err
	}
	return &domain.DashboardStats{
		AssignedCount:    int(row.AssignedCount),
		OverdueCount:     int(row.OverdueCount),
		DueThisWeekCount: int(row.DueThisWeekCount),
		CompletedCount:   int(row.CompletedCount),
		TotalProjects:    int(row.TotalProjects),
	}, nil
}

func (s *DashboardStore) RecentActivity(ctx context.Context, orgID, userID string, limit int) ([]*domain.DashboardActivity, error) {
	rows, err := s.q.RecentActivity(ctx, sqlc.RecentActivityParams{
		UserID: userID,
		OrgID:  orgID,
		Limit:  int64(limit),
	})
	if err != nil {
		return nil, err
	}
	results := make([]*domain.DashboardActivity, len(rows))
	for i, r := range rows {
		results[i] = &domain.DashboardActivity{
			ID:          r.ID,
			Type:        r.Type,
			Title:       r.Title,
			Body:        r.Body,
			Link:        r.Link,
			EntityType:  r.EntityType,
			EntityID:    r.EntityID,
			ActorName:   r.ActorName,
			ProjectSlug: r.ProjectSlug,
			IsUnread:    r.IsRead == 1,
			CreatedAt:   parseTime(r.CreatedAt),
		}
	}
	return results, nil
}

func (s *DashboardStore) OrgProjects(ctx context.Context, orgID string) ([]*domain.DashboardProject, error) {
	rows, err := s.q.OrgProjects(ctx, orgID)
	if err != nil {
		return nil, err
	}
	results := make([]*domain.DashboardProject, len(rows))
	for i, r := range rows {
		results[i] = &domain.DashboardProject{
			ID:          r.ID,
			Name:        r.Name,
			Slug:        r.Slug,
			Color:       r.Color,
			Icon:        r.Icon,
			TaskCount:   int(r.TaskCount),
			MemberCount: int(r.MemberCount),
		}
	}
	return results, nil
}

func (s *DashboardStore) OrgProjectsForUser(ctx context.Context, orgID, userID string) ([]*domain.DashboardProject, error) {
	rows, err := s.q.OrgProjectsForUser(ctx, sqlc.OrgProjectsForUserParams{
		OrgID:  orgID,
		UserID: userID,
	})
	if err != nil {
		return nil, err
	}
	results := make([]*domain.DashboardProject, len(rows))
	for i, r := range rows {
		results[i] = &domain.DashboardProject{
			ID:          r.ID,
			Name:        r.Name,
			Slug:        r.Slug,
			Color:       r.Color,
			Icon:        r.Icon,
			TaskCount:   int(r.TaskCount),
			MemberCount: int(r.MemberCount),
		}
	}
	return results, nil
}

func (s *DashboardStore) GetPreferences(ctx context.Context, orgID, userID string) ([]domain.SectionType, error) {
	sections, err := s.q.GetDashboardPreferences(ctx, sqlc.GetDashboardPreferencesParams{
		UserID: userID,
		OrgID:  orgID,
	})
	if err != nil {
		return domain.DefaultDashboardSections, nil
	}
	result, err := domain.SectionsFromJSON(sections)
	if err != nil {
		return domain.DefaultDashboardSections, nil
	}
	return result, nil
}

func (s *DashboardStore) SetPreferences(ctx context.Context, orgID, userID string, sections []domain.SectionType) error {
	raw, err := domain.SectionsToJSON(sections)
	if err != nil {
		return err
	}
	return s.q.SetDashboardPreferences(ctx, sqlc.SetDashboardPreferencesParams{
		UserID:   userID,
		OrgID:    orgID,
		Sections: raw,
	})
}
