package service

import (
	"context"
	"fmt"
	"time"

	"ipmanlk/breeze/internal/apperr"
	"ipmanlk/breeze/internal/domain"
	"ipmanlk/breeze/internal/port"

	"github.com/google/uuid"
)

type CycleService struct {
	cycleRepo port.CycleRepository
	projRepo  port.ProjectRepository
	taskRepo  port.TaskRepository
	access    port.AccessChecker
}

var _ port.CycleService = (*CycleService)(nil)

func NewCycleService(cycleRepo port.CycleRepository, projRepo port.ProjectRepository, taskRepo port.TaskRepository, access port.AccessChecker) *CycleService {
	return &CycleService{cycleRepo: cycleRepo, projRepo: projRepo, taskRepo: taskRepo, access: access}
}

func (s *CycleService) Create(ctx context.Context, p domain.CreateCycleParams) (*domain.Cycle, error) {
	if p.Name == "" {
		var err error
		p.Name, err = s.nextCycleName(ctx, p.ProjectID)
		if err != nil {
			return nil, err
		}
	}

	if p.EndsAt.Before(p.StartsAt) {
		return nil, apperr.InvalidInput("cycle end date must be after start date")
	}

	if s.access != nil {
		if err := s.access.RequireProjectAccess(ctx, p.CreatedBy, p.OrgID, p.ProjectID, domain.PermProjectCycleManage); err != nil {
			return nil, err
		}
	}

	now := time.Now()
	cycle := &domain.Cycle{
		ID:        uuid.New().String(),
		OrgID:     p.OrgID,
		ProjectID: p.ProjectID,
		Name:      p.Name,
		Goal:      p.Goal,
		StartsAt:  p.StartsAt,
		EndsAt:    p.EndsAt,
		CreatedBy: p.CreatedBy,
		CreatedAt: now,
		UpdatedAt: now,
	}

	if err := s.cycleRepo.Create(ctx, cycle); err != nil {
		return nil, err
	}

	return s.cycleRepo.GetByID(ctx, cycle.ID, p.ProjectID)
}

func (s *CycleService) List(ctx context.Context, projectID string) ([]*domain.Cycle, error) {
	cycles, err := s.cycleRepo.ListByProject(ctx, projectID)
	if err != nil {
		return nil, err
	}
	counts, err := s.cycleRepo.CountTasksByCycles(ctx, projectID)
	if err != nil {
		return nil, err
	}
	for _, c := range cycles {
		if ct, ok := counts[c.ID]; ok {
			c.TaskCount = int(ct.Total)
			c.CompletedTaskCount = int(ct.Completed)
		}
	}
	return cycles, nil
}

func (s *CycleService) GetByID(ctx context.Context, id, projectID string) (*domain.Cycle, error) {
	c, err := s.cycleRepo.GetByID(ctx, id, projectID)
	if err != nil {
		return nil, err
	}
	total, completed, err := s.cycleRepo.CountTasksByCycle(ctx, c.ID)
	if err != nil {
		return nil, err
	}
	c.TaskCount = int(total)
	c.CompletedTaskCount = int(completed)
	return c, nil
}

func (s *CycleService) GetActive(ctx context.Context, projectID string) (*domain.Cycle, error) {
	c, err := s.cycleRepo.GetActiveByProject(ctx, projectID)
	if err != nil {
		return nil, err
	}
	total, completed, err := s.cycleRepo.CountTasksByCycle(ctx, c.ID)
	if err != nil {
		return nil, err
	}
	c.TaskCount = int(total)
	c.CompletedTaskCount = int(completed)
	return c, nil
}

func (s *CycleService) Update(ctx context.Context, userID, orgID string, c *domain.Cycle) error {
	if c.EndsAt.Before(c.StartsAt) {
		return apperr.InvalidInput("cycle end date must be after start date")
	}
	if s.access != nil {
		if err := s.access.RequireProjectAccess(ctx, userID, orgID, c.ProjectID, domain.PermProjectCycleManage); err != nil {
			return err
		}
	}
	c.UpdatedAt = time.Now()
	return s.cycleRepo.Update(ctx, c)
}

func (s *CycleService) Delete(ctx context.Context, userID, orgID, id, projectID string) error {
	if s.access != nil {
		if err := s.access.RequireProjectAccess(ctx, userID, orgID, projectID, domain.PermProjectCycleManage); err != nil {
			return err
		}
	}
	return s.cycleRepo.Delete(ctx, id, projectID)
}

func (s *CycleService) Activate(ctx context.Context, userID, orgID, id, projectID string) (*domain.Cycle, error) {
	if s.access != nil {
		if err := s.access.RequireProjectAccess(ctx, userID, orgID, projectID, domain.PermProjectCycleManage); err != nil {
			return nil, err
		}
	}
	// Single atomic store call: deactivating everything first and activating
	// the target second, as separate autocommit statements, could leave the
	// project with no active cycle if the process died in between.
	if err := s.cycleRepo.ActivateCycle(ctx, id, projectID); err != nil {
		return nil, err
	}
	return s.GetByID(ctx, id, projectID)
}

func (s *CycleService) Complete(ctx context.Context, userID, orgID, id, projectID string, moveToCycleID string) (*domain.Cycle, error) {
	if s.access != nil {
		if err := s.access.RequireProjectAccess(ctx, userID, orgID, projectID, domain.PermProjectCycleManage); err != nil {
			return nil, err
		}
	}
	cycle, err := s.cycleRepo.GetByID(ctx, id, projectID)
	if err != nil {
		return nil, err
	}

	cycle.IsCompleted = true
	cycle.IsActive = false
	cycle.UpdatedAt = time.Now()

	plan := domain.CycleCompletionPlan{
		OrgID:            cycle.OrgID,
		ProjectID:        projectID,
		CompletedCycleID: id,
		CompletedCycle:   *cycle,
	}

	if moveToCycleID != "" {
		// Validate the move target belongs to THIS project. The completion
		// query only filters by the source cycle's org, so an unvalidated ID
		// from another project (or another org) would silently re-home tasks
		// onto a foreign cycle.
		if _, err := s.cycleRepo.GetByID(ctx, moveToCycleID, projectID); err != nil {
			return nil, apperr.InvalidInput("move-to cycle does not belong to this project")
		}
		plan.MoveTargetCycleID = moveToCycleID
	} else {
		proj, err := s.projRepo.GetByID(ctx, cycle.OrgID, projectID)
		if err != nil {
			return nil, err
		}

		if proj.IncompleteTaskHandling == domain.CycleHandlingNextCycle {
			if proj.AutoGenerateCycles {
				newCycle := s.buildNextCycle(ctx, cycle)
				plan.NewCycle = newCycle
				plan.MoveTargetCycleID = newCycle.ID
				plan.SetActiveCycleID = newCycle.ID
			} else {
				nextCycle, err := s.findNextUncompletedCycle(ctx, projectID, cycle.StartsAt)
				if err == nil && nextCycle != nil {
					plan.MoveTargetCycleID = nextCycle.ID
					plan.SetActiveCycleID = nextCycle.ID
				} else {
					// No next cycle found; unassign incomplete tasks.
					plan.MoveTargetCycleID = ""
					plan.SetActiveCycleID = ""
				}
			}
		} else {
			// backlog handling; unassign incomplete tasks.
			plan.MoveTargetCycleID = ""
			plan.SetActiveCycleID = ""
		}
	}

	if err := s.cycleRepo.CompleteCycle(ctx, plan); err != nil {
		return nil, err
	}

	return s.GetByID(ctx, id, projectID)
}

func (s *CycleService) buildNextCycle(ctx context.Context, completed *domain.Cycle) *domain.Cycle {
	name, _ := s.nextCycleName(ctx, completed.ProjectID)

	return &domain.Cycle{
		ID:        uuid.New().String(),
		OrgID:     completed.OrgID,
		ProjectID: completed.ProjectID,
		Name:      name,
		Goal:      "",
		StartsAt:  completed.EndsAt,
		EndsAt:    completed.EndsAt.Add(completed.EndsAt.Sub(completed.StartsAt)),
		CreatedBy: completed.CreatedBy,
		CreatedAt: time.Now(),
		UpdatedAt: time.Now(),
	}
}

func (s *CycleService) findNextUncompletedCycle(ctx context.Context, projectID string, after time.Time) (*domain.Cycle, error) {
	cycles, err := s.cycleRepo.ListByProject(ctx, projectID)
	if err != nil {
		return nil, err
	}
	for _, c := range cycles {
		if !c.IsCompleted && c.StartsAt.After(after) {
			return c, nil
		}
	}
	return nil, nil
}

func (s *CycleService) nextCycleName(ctx context.Context, projectID string) (string, error) {
	cycles, err := s.cycleRepo.ListByProject(ctx, projectID)
	if err != nil {
		return "", err
	}
	return fmt.Sprintf("Cycle %d", len(cycles)+1), nil
}
