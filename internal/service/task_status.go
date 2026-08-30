package service

import (
	"context"
	"fmt"

	"ipmanlk/breeze/internal/apperr"
	"ipmanlk/breeze/internal/domain"
	"ipmanlk/breeze/internal/port"

	"github.com/google/uuid"
)

type TaskStatusService struct {
	repo   port.TaskStatusRepository
	access port.AccessChecker
}

var _ port.TaskStatusService = (*TaskStatusService)(nil)

func NewTaskStatusService(repo port.TaskStatusRepository, access port.AccessChecker) *TaskStatusService {
	return &TaskStatusService{repo: repo, access: access}
}

func (s *TaskStatusService) List(ctx context.Context, projectID string) ([]*domain.TaskStatus, error) {
	return s.repo.ListByProject(ctx, projectID)
}

func (s *TaskStatusService) GetByID(ctx context.Context, id string) (*domain.TaskStatus, error) {
	return s.repo.GetByID(ctx, id)
}

func (s *TaskStatusService) Create(ctx context.Context, userID, orgID string, p domain.CreateTaskStatusParams) (*domain.TaskStatus, error) {
	if s.access != nil {
		if err := s.access.RequireProjectAccess(ctx, userID, orgID, p.ProjectID, domain.PermProjectStatusManage); err != nil {
			return nil, err
		}
	}
	st := &domain.TaskStatus{
		ID:        uuid.New().String(),
		ProjectID: p.ProjectID,
		Name:      p.Name,
		Color:     p.Color,
		Position:  p.Position,
		Category:  p.Category,
	}
	return st, s.repo.Create(ctx, st)
}

func (s *TaskStatusService) Update(ctx context.Context, userID, orgID string, st *domain.TaskStatus) error {
	if s.access != nil {
		if err := s.access.RequireProjectAccess(ctx, userID, orgID, st.ProjectID, domain.PermProjectStatusManage); err != nil {
			return err
		}
	}
	return s.repo.Update(ctx, st)
}

func (s *TaskStatusService) Delete(ctx context.Context, userID, orgID, id, projectID string) error {
	if s.access != nil {
		if err := s.access.RequireProjectAccess(ctx, userID, orgID, projectID, domain.PermProjectStatusManage); err != nil {
			return err
		}
	}
	// Pre-check: reject deletion if tasks still reference this status.
	// The foreign-key constraint (ON DELETE RESTRICT) is the ultimate guard,
	// but this check provides a friendly 400 with the task count instead of
	// a raw SQL foreign-key error.
	count, err := s.repo.CountTasksByStatus(ctx, id, projectID)
	if err != nil {
		return err
	}
	if count > 0 {
		return apperr.InvalidInput(fmt.Sprintf("cannot delete status: %d task(s) still use it; reassign them first", count))
	}
	return s.repo.Delete(ctx, id, projectID)
}
