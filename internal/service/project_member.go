package service

import (
	"context"
	"fmt"

	"ipmanlk/breeze/internal/apperr"
	"ipmanlk/breeze/internal/domain"
	"ipmanlk/breeze/internal/port"
)

type ProjectMemberService struct {
	pmRepo   port.ProjectMemberRepository
	userRepo port.UserRepository
}

var _ port.ProjectMemberService = (*ProjectMemberService)(nil)

func NewProjectMemberService(pmRepo port.ProjectMemberRepository, userRepo port.UserRepository) *ProjectMemberService {
	return &ProjectMemberService{pmRepo: pmRepo, userRepo: userRepo}
}

func (s *ProjectMemberService) List(ctx context.Context, orgID, projectID string, filter domain.UserFilter) (*domain.ProjectMemberListResult, error) {
	return s.pmRepo.List(ctx, orgID, projectID, filter)
}

// validateTargetUser confirms the user being added/modified is a member of the
// caller's org. Without this, a project admin could add an arbitrary user UUID
// (possibly from another org) to their project_members row.
func (s *ProjectMemberService) validateTargetUser(ctx context.Context, orgID, userID string) error {
	if _, err := s.userRepo.GetByID(ctx, orgID, userID); err != nil {
		return err
	}
	return nil
}

func validProjectRole(role domain.Role) bool {
	return role == domain.RoleAdmin || role == domain.RoleMember ||
		role == domain.RoleViewer || role == domain.RoleGuest
}

func (s *ProjectMemberService) Add(ctx context.Context, orgID, projectID, userID string, role domain.Role) error {
	if !validProjectRole(role) {
		return apperr.InvalidInput(fmt.Sprintf("invalid project role: %s", role))
	}
	if err := s.validateTargetUser(ctx, orgID, userID); err != nil {
		return err
	}
	return s.pmRepo.Add(ctx, projectID, userID, role)
}

func (s *ProjectMemberService) Remove(ctx context.Context, orgID, projectID, userID string) error {
	if err := s.validateTargetUser(ctx, orgID, userID); err != nil {
		return err
	}
	return s.pmRepo.Remove(ctx, projectID, userID)
}

func (s *ProjectMemberService) UpdateRole(ctx context.Context, orgID, projectID, userID string, role domain.Role) error {
	if !validProjectRole(role) {
		return apperr.InvalidInput(fmt.Sprintf("invalid project role: %s", role))
	}
	if err := s.validateTargetUser(ctx, orgID, userID); err != nil {
		return err
	}
	return s.pmRepo.UpdateRole(ctx, projectID, userID, role)
}

func (s *ProjectMemberService) ListByUser(ctx context.Context, orgID, userID string) ([]*domain.UserProjectMembership, error) {
	return s.pmRepo.ListByUser(ctx, orgID, userID)
}

func (s *ProjectMemberService) SetMemberships(ctx context.Context, orgID, userID string, assignments []domain.ProjectAssignment) error {
	if err := s.validateTargetUser(ctx, orgID, userID); err != nil {
		return fmt.Errorf("validate target user: %w", err)
	}
	for _, a := range assignments {
		if !validProjectRole(a.Role) {
			return apperr.InvalidInput(fmt.Sprintf("invalid project role: %s", a.Role))
		}
	}
	return s.pmRepo.SetMemberships(ctx, orgID, userID, assignments)
}
