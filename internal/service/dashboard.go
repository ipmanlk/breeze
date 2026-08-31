package service

import (
	"context"

	"ipmanlk/plume/internal/domain"
	"ipmanlk/plume/internal/port"
)

type DashboardService struct {
	repo port.DashboardRepository
}

var _ port.DashboardService = (*DashboardService)(nil)

func NewDashboardService(repo port.DashboardRepository) *DashboardService {
	return &DashboardService{repo: repo}
}

const defaultDashboardLimit = 15

func (s *DashboardService) GetDashboard(ctx context.Context, params domain.GetDashboardParams) (*domain.DashboardData, error) {
	sections, err := s.repo.GetPreferences(ctx, params.OrgID, params.UserID)
	if err != nil {
		return nil, err
	}
	if len(params.Sections) > 0 {
		sections = params.Sections
	}

	var allSections []domain.DashboardSectionData

	for _, st := range sections {
		title, ok := domain.SectionTitles[st]
		if !ok {
			title = string(st)
		}
		sd := domain.DashboardSectionData{Type: st, Title: title}
		switch st {
		case domain.SectionMyTasks:
			tasks, err := s.repo.MyTasks(ctx, params.OrgID, params.UserID, defaultDashboardLimit)
			if err != nil {
				return nil, err
			}
			sd.Data = tasks
		case domain.SectionDueSoon:
			tasks, err := s.repo.DueSoonTasks(ctx, params.OrgID, params.UserID, defaultDashboardLimit)
			if err != nil {
				return nil, err
			}
			sd.Data = tasks
		case domain.SectionActivity:
			activities, err := s.repo.RecentActivity(ctx, params.OrgID, params.UserID, defaultDashboardLimit)
			if err != nil {
				return nil, err
			}
			sd.Data = activities
		case domain.SectionStats:
			stats, err := s.repo.MyTaskStats(ctx, params.OrgID, params.UserID)
			if err != nil {
				return nil, err
			}
			sd.Data = stats
		case domain.SectionProjects:
			// Viewer/guest roles only see projects they are explicit members
			// of; elevated org roles see every project in the org.
			var projects []*domain.DashboardProject
			var err error
			if domain.IsOrgElevatedRole(params.Role) {
				projects, err = s.repo.OrgProjects(ctx, params.OrgID)
			} else {
				projects, err = s.repo.OrgProjectsForUser(ctx, params.OrgID, params.UserID)
			}
			if err != nil {
				return nil, err
			}
			sd.Data = projects
		}
		allSections = append(allSections, sd)
	}

	return &domain.DashboardData{Sections: allSections}, nil
}

func (s *DashboardService) SetVisibility(ctx context.Context, params domain.SetVisibilityParams) (*domain.DashboardVisibility, error) {
	if err := s.repo.SetPreferences(ctx, params.OrgID, params.UserID, params.Sections); err != nil {
		return nil, err
	}
	return &domain.DashboardVisibility{
		UserID:   params.UserID,
		Sections: params.Sections,
	}, nil
}
