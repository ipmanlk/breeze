package store

import (
	"context"
	"database/sql"

	"ipmanlk/plume/internal/domain"
	"ipmanlk/plume/internal/port"
	"ipmanlk/plume/internal/store/sqlc"
)

type CycleStore struct {
	q  *sqlc.Queries
	db *sql.DB
}

func NewCycleStore(q *sqlc.Queries, db *sql.DB) *CycleStore {
	return &CycleStore{q: q, db: db}
}

var _ port.CycleRepository = (*CycleStore)(nil)

func (s *CycleStore) toDomain(c sqlc.Cycle) domain.Cycle {
	return domain.Cycle{
		ID:          c.ID,
		OrgID:       c.OrgID,
		ProjectID:   c.ProjectID,
		Name:        c.Name,
		Goal:        c.Goal,
		StartsAt:    parseTime(c.StartsAt),
		EndsAt:      parseTime(c.EndsAt),
		CreatedBy:   c.CreatedBy,
		IsCompleted: c.IsCompleted,
		IsActive:    c.IsActive,
		CreatedAt:   parseTime(c.CreatedAt),
		UpdatedAt:   parseTime(c.UpdatedAt),
	}
}

func (s *CycleStore) ListByProject(ctx context.Context, projectID string) ([]*domain.Cycle, error) {
	rows, err := s.q.ListCyclesByProject(ctx, projectID)
	if err != nil {
		return nil, err
	}
	cycles := make([]*domain.Cycle, len(rows))
	for i, row := range rows {
		d := s.toDomain(row)
		cycles[i] = &d
	}
	return cycles, nil
}

func (s *CycleStore) GetByID(ctx context.Context, id, projectID string) (*domain.Cycle, error) {
	c, err := s.q.GetCycleByID(ctx, sqlc.GetCycleByIDParams{ID: id, ProjectID: projectID})
	if err != nil {
		return nil, mapScanErr(err)
	}
	d := s.toDomain(c)
	return &d, nil
}

func (s *CycleStore) GetActiveByProject(ctx context.Context, projectID string) (*domain.Cycle, error) {
	c, err := s.q.GetActiveCycleByProject(ctx, projectID)
	if err != nil {
		return nil, mapScanErr(err)
	}
	d := s.toDomain(c)
	return &d, nil
}

func (s *CycleStore) Create(ctx context.Context, c *domain.Cycle) error {
	return s.q.CreateCycle(ctx, sqlc.CreateCycleParams{
		ID:        c.ID,
		OrgID:     c.OrgID,
		ProjectID: c.ProjectID,
		Name:      c.Name,
		Goal:      c.Goal,
		StartsAt:  formatTime(c.StartsAt),
		EndsAt:    formatTime(c.EndsAt),
		CreatedBy: c.CreatedBy,
	})
}

func (s *CycleStore) Update(ctx context.Context, c *domain.Cycle) error {
	return s.q.UpdateCycle(ctx, sqlc.UpdateCycleParams{
		Name:        c.Name,
		Goal:        c.Goal,
		StartsAt:    formatTime(c.StartsAt),
		EndsAt:      formatTime(c.EndsAt),
		IsCompleted: c.IsCompleted,
		IsActive:    c.IsActive,
		ID:          c.ID,
		ProjectID:   c.ProjectID,
	})
}

func (s *CycleStore) Delete(ctx context.Context, id, projectID string) error {
	return s.q.DeleteCycle(ctx, sqlc.DeleteCycleParams{ID: id, ProjectID: projectID})
}

func (s *CycleStore) DeactivateAll(ctx context.Context, projectID string) error {
	return s.q.DeactivateAllCycles(ctx, projectID)
}

func (s *CycleStore) SetActive(ctx context.Context, id, projectID string) error {
	return s.q.SetCycleActive(ctx, sqlc.SetCycleActiveParams{ID: id, ProjectID: projectID})
}

// ActivateCycle atomically deactivates every cycle of the project and marks
// the given one active. Doing this in one transaction preserves the
// "at most one active cycle per project" invariant even on failure.
func (s *CycleStore) ActivateCycle(ctx context.Context, id, projectID string) error {
	tx, err := s.db.BeginTx(ctx, nil)
	if err != nil {
		return err
	}
	defer tx.Rollback()

	q := s.q.WithTx(tx)
	if err := q.DeactivateAllCycles(ctx, projectID); err != nil {
		return err
	}
	if err := q.SetCycleActive(ctx, sqlc.SetCycleActiveParams{ID: id, ProjectID: projectID}); err != nil {
		return err
	}
	return tx.Commit()
}

func (s *CycleStore) CountTasksByCycle(ctx context.Context, id string) (total, completed int64, err error) {
	row, err := s.q.CountTasksByCycle(ctx, nilIfEmpty(id))
	if err != nil {
		return 0, 0, err
	}
	total = row.Total
	if row.Completed != nil {
		completed = int64(*row.Completed)
	}
	return total, completed, nil
}

func (s *CycleStore) CountTasksByCycles(ctx context.Context, projectID string) (map[string]domain.CycleTaskCount, error) {
	rows, err := s.q.CountTasksByCycles(ctx, projectID)
	if err != nil {
		return nil, err
	}
	out := make(map[string]domain.CycleTaskCount, len(rows))
	for _, r := range rows {
		completed := int64(0)
		if r.Completed != nil {
			completed = int64(*r.Completed)
		}
		out[r.CycleID] = domain.CycleTaskCount{Total: r.Total, Completed: completed}
	}
	return out, nil
}

// CompleteCycle executes all cycle-completion DB operations atomically.
// The service computes the plan including which cycle to set active, whether
// to auto-generate a next cycle, and where to move incomplete tasks.
func (s *CycleStore) CompleteCycle(ctx context.Context, plan domain.CycleCompletionPlan) error {
	tx, err := s.db.BeginTx(ctx, nil)
	if err != nil {
		return err
	}
	defer tx.Rollback()

	q := s.q.WithTx(tx)

	// 1. Create the new auto-generated cycle if the plan includes one.
	if plan.NewCycle != nil {
		if err := q.CreateCycle(ctx, sqlc.CreateCycleParams{
			ID:        plan.NewCycle.ID,
			OrgID:     plan.NewCycle.OrgID,
			ProjectID: plan.NewCycle.ProjectID,
			Name:      plan.NewCycle.Name,
			Goal:      plan.NewCycle.Goal,
			StartsAt:  formatTime(plan.NewCycle.StartsAt),
			EndsAt:    formatTime(plan.NewCycle.EndsAt),
			CreatedBy: plan.NewCycle.CreatedBy,
		}); err != nil {
			return err
		}
	}

	// 2. Deactivate all cycles for this project.
	if err := q.DeactivateAllCycles(ctx, plan.ProjectID); err != nil {
		return err
	}

	// 3. Move incomplete tasks to the target cycle, or unassign them.
	if plan.MoveTargetCycleID != "" {
		if err := q.MoveIncompleteTasksToCycle(ctx, sqlc.MoveIncompleteTasksToCycleParams{
			ToCycleID:   &plan.MoveTargetCycleID,
			FromCycleID: &plan.CompletedCycleID,
			OrgID:       plan.OrgID,
		}); err != nil {
			return err
		}
	} else {
		if err := q.UnassignCycleFromIncompleteTasks(ctx, sqlc.UnassignCycleFromIncompleteTasksParams{
			CycleID: &plan.CompletedCycleID,
			OrgID:   plan.OrgID,
		}); err != nil {
			return err
		}
	}

	// 4. Set the target cycle as active (if any).
	if plan.SetActiveCycleID != "" {
		if err := q.SetCycleActive(ctx, sqlc.SetCycleActiveParams{
			ID:        plan.SetActiveCycleID,
			ProjectID: plan.ProjectID,
		}); err != nil {
			return err
		}
	}

	// 5. Update the completed cycle.
	if err := q.UpdateCycle(ctx, sqlc.UpdateCycleParams{
		Name:        plan.CompletedCycle.Name,
		Goal:        plan.CompletedCycle.Goal,
		StartsAt:    formatTime(plan.CompletedCycle.StartsAt),
		EndsAt:      formatTime(plan.CompletedCycle.EndsAt),
		IsCompleted: plan.CompletedCycle.IsCompleted,
		IsActive:    plan.CompletedCycle.IsActive,
		ID:          plan.CompletedCycle.ID,
		ProjectID:   plan.CompletedCycle.ProjectID,
	}); err != nil {
		return err
	}

	return tx.Commit()
}
