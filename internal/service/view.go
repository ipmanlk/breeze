package service

import (
	"context"
	"fmt"

	"github.com/google/uuid"
	"ipmanlk/plume/internal/apperr"
	"ipmanlk/plume/internal/domain"
	"ipmanlk/plume/internal/port"
)

type ViewService struct {
	viewRepo port.ViewRepository
	access   port.AccessChecker
}

var _ port.ViewService = (*ViewService)(nil)

func NewViewService(viewRepo port.ViewRepository, access port.AccessChecker) *ViewService {
	return &ViewService{viewRepo: viewRepo, access: access}
}

func (s *ViewService) Create(ctx context.Context, params domain.CreateViewParams) (*domain.View, error) {
	id := uuid.New().String()
	v := &domain.View{
		ID:        id,
		OrgID:     params.OrgID,
		ProjectID: params.ProjectID,
		CreatedBy: params.CreatedBy,
		Name:      params.Name,
		Layout:    params.Layout,
		Filters:   params.Filters,
	}
	// If this is a project-scoped view, verify the user can access the project.
	if s.access != nil && params.ProjectID != nil && *params.ProjectID != "" {
		if err := s.access.RequireProjectAccess(ctx, params.CreatedBy, params.OrgID, *params.ProjectID, domain.PermProjectView); err != nil {
			return nil, err
		}
	}
	if err := s.viewRepo.Create(ctx, v); err != nil {
		return nil, fmt.Errorf("create view: %w", err)
	}
	return s.viewRepo.GetByID(ctx, params.OrgID, id)
}

func (s *ViewService) Update(ctx context.Context, userID string, params domain.UpdateViewParams) (*domain.View, error) {
	v, err := s.viewRepo.GetByID(ctx, params.OrgID, params.ID)
	if err != nil {
		return nil, err
	}
	// Only the creator can modify a view.
	if v.CreatedBy != userID {
		return nil, apperr.ErrForbidden
	}
	if params.Name != nil {
		v.Name = *params.Name
	}
	if params.Layout != nil {
		v.Layout = *params.Layout
	}
	if params.Filters != nil {
		v.Filters = *params.Filters
	}
	if err := s.viewRepo.Update(ctx, v); err != nil {
		return nil, fmt.Errorf("update view: %w", err)
	}
	return s.viewRepo.GetByID(ctx, params.OrgID, params.ID)
}

func (s *ViewService) Delete(ctx context.Context, userID, orgID, id string) error {
	v, err := s.viewRepo.GetByID(ctx, orgID, id)
	if err != nil {
		return err
	}
	// Only the creator can delete a view.
	if v.CreatedBy != userID {
		return apperr.ErrForbidden
	}
	return s.viewRepo.Delete(ctx, orgID, id)
}

func (s *ViewService) GetByID(ctx context.Context, orgID, id string) (*domain.View, error) {
	return s.viewRepo.GetByID(ctx, orgID, id)
}

func (s *ViewService) ListByProject(ctx context.Context, orgID, projectID string) ([]*domain.View, error) {
	return s.viewRepo.ListByProject(ctx, orgID, projectID)
}

func (s *ViewService) ListGlobal(ctx context.Context, orgID string) ([]*domain.View, error) {
	return s.viewRepo.ListGlobal(ctx, orgID)
}

func (s *ViewService) ListPinned(ctx context.Context, userID string) ([]*domain.View, error) {
	return s.viewRepo.ListPinned(ctx, userID)
}

func (s *ViewService) Pin(ctx context.Context, orgID, viewID, userID string) error {
	// Verify the view exists in the caller's org before pinning; the pin row
	// itself keys on (view_id, user_id) with no org column.
	if _, err := s.viewRepo.GetByID(ctx, orgID, viewID); err != nil {
		return err
	}
	return s.viewRepo.Pin(ctx, viewID, userID)
}

func (s *ViewService) Unpin(ctx context.Context, orgID, viewID, userID string) error {
	if _, err := s.viewRepo.GetByID(ctx, orgID, viewID); err != nil {
		return err
	}
	return s.viewRepo.Unpin(ctx, viewID, userID)
}
