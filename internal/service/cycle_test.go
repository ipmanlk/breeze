package service

import (
	"context"
	"testing"
	"time"

	"ipmanlk/breeze/internal/domain"
)

func ptr(s string) *string {
	return &s
}

func TestCycleService_Create_Success(t *testing.T) {
	cycleRepo := newMockCycleRepo()
	projRepo := newMockProjectRepo()
	taskRepo := newMockTaskRepo()
	svc := NewCycleService(cycleRepo, projRepo, taskRepo, nil)

	start := time.Now()
	end := start.Add(14 * 24 * time.Hour)

	cycle, err := svc.Create(context.Background(), domain.CreateCycleParams{
		OrgID:     "org-1",
		ProjectID: "proj-1",
		CreatedBy: "user-1",
		Name:      "Cycle 1",
		Goal:      "Build the MVP",
		StartsAt:  start,
		EndsAt:    end,
	})
	if err != nil {
		t.Fatalf("Create() error = %v", err)
	}
	if cycle == nil {
		t.Fatal("Create() cycle is nil")
	}
	if cycle.Name != "Cycle 1" {
		t.Errorf("Name = %q, want %q", cycle.Name, "Cycle 1")
	}
	if cycle.Goal != "Build the MVP" {
		t.Errorf("Goal = %q, want %q", cycle.Goal, "Build the MVP")
	}
}

func TestCycleService_Create_AutoName(t *testing.T) {
	cycleRepo := newMockCycleRepo()
	taskRepo := newMockTaskRepo()
	svc := NewCycleService(cycleRepo, newMockProjectRepo(), taskRepo, nil)

	start := time.Now()
	end := start.Add(14 * 24 * time.Hour)

	c1, err := svc.Create(context.Background(), domain.CreateCycleParams{
		OrgID:     "org-1",
		ProjectID: "proj-1",
		CreatedBy: "user-1",
		Name:      "",
		Goal:      "First",
		StartsAt:  start,
		EndsAt:    end,
	})
	if err != nil {
		t.Fatalf("Create() error = %v", err)
	}
	if c1.Name != "Cycle 1" {
		t.Errorf("Name = %q, want %q", c1.Name, "Cycle 1")
	}

	c2, err := svc.Create(context.Background(), domain.CreateCycleParams{
		OrgID:     "org-1",
		ProjectID: "proj-1",
		CreatedBy: "user-1",
		Name:      "",
		Goal:      "Second",
		StartsAt:  start,
		EndsAt:    end,
	})
	if err != nil {
		t.Fatalf("Create() error = %v", err)
	}
	if c2.Name != "Cycle 2" {
		t.Errorf("Name = %q, want %q", c2.Name, "Cycle 2")
	}
}

func TestCycleService_Create_EndBeforeStart(t *testing.T) {
	svc := NewCycleService(newMockCycleRepo(), newMockProjectRepo(), newMockTaskRepo(), nil)

	start := time.Now()
	end := start.Add(-1 * 24 * time.Hour)

	_, err := svc.Create(context.Background(), domain.CreateCycleParams{
		OrgID:     "org-1",
		ProjectID: "proj-1",
		CreatedBy: "user-1",
		Name:      "Bad Cycle",
		Goal:      "",
		StartsAt:  start,
		EndsAt:    end,
	})
	if err == nil {
		t.Fatal("expected error for end before start")
	}
}

func TestCycleService_List(t *testing.T) {
	cycleRepo := newMockCycleRepo()
	taskRepo := newMockTaskRepo()
	svc := NewCycleService(cycleRepo, newMockProjectRepo(), taskRepo, nil)

	cycleRepo.cyclesByID["c1"] = &domain.Cycle{ID: "c1", ProjectID: "proj-1", Name: "Cycle 1"}
	cycleRepo.cyclesByID["c2"] = &domain.Cycle{ID: "c2", ProjectID: "proj-1", Name: "Cycle 2"}
	cycleRepo.cyclesByID["c3"] = &domain.Cycle{ID: "c3", ProjectID: "proj-2", Name: "Other"}

	cycles, err := svc.List(context.Background(), "proj-1")
	if err != nil {
		t.Fatalf("List() error = %v", err)
	}
	if len(cycles) != 2 {
		t.Errorf("got %d cycles, want 2", len(cycles))
	}
}

func TestCycleService_Delete(t *testing.T) {
	cycleRepo := newMockCycleRepo()
	taskRepo := newMockTaskRepo()
	svc := NewCycleService(cycleRepo, newMockProjectRepo(), taskRepo, nil)

	cycleRepo.cyclesByID["c1"] = &domain.Cycle{ID: "c1", ProjectID: "proj-1", Name: "Cycle 1"}
	if err := svc.Delete(context.Background(), "", "org-1", "c1", "proj-1"); err != nil {
		t.Fatalf("Delete() error = %v", err)
	}
	if _, err := cycleRepo.GetByID(context.Background(), "c1", "proj-1"); err == nil {
		t.Error("expected cycle to be deleted")
	}
}

func TestCycleService_Update_Success(t *testing.T) {
	cycleRepo := newMockCycleRepo()
	taskRepo := newMockTaskRepo()
	svc := NewCycleService(cycleRepo, newMockProjectRepo(), taskRepo, nil)

	start := time.Now()
	end := start.Add(14 * 24 * time.Hour)

	cycle, err := svc.Create(context.Background(), domain.CreateCycleParams{
		OrgID:     "org-1",
		ProjectID: "proj-1",
		CreatedBy: "user-1",
		Name:      "Original",
		StartsAt:  start,
		EndsAt:    end,
	})
	if err != nil {
		t.Fatalf("Create() error = %v", err)
	}

	cycle.Name = "Updated"
	cycle.Goal = "New goal"

	if err := svc.Update(context.Background(), "", "org-1", cycle); err != nil {
		t.Fatalf("Update() error = %v", err)
	}

	updated, err := svc.GetByID(context.Background(), cycle.ID, "proj-1")
	if err != nil {
		t.Fatalf("GetByID() error = %v", err)
	}
	if updated.Name != "Updated" {
		t.Errorf("Name = %q, want %q", updated.Name, "Updated")
	}
	if updated.Goal != "New goal" {
		t.Errorf("Goal = %q, want %q", updated.Goal, "New goal")
	}
}

func TestCycleService_Update_EndBeforeStart(t *testing.T) {
	cycleRepo := newMockCycleRepo()
	taskRepo := newMockTaskRepo()
	svc := NewCycleService(cycleRepo, newMockProjectRepo(), taskRepo, nil)

	cycleRepo.cyclesByID["c1"] = &domain.Cycle{
		ID:        "c1",
		ProjectID: "proj-1",
		Name:      "Bad Cycle",
		StartsAt:  time.Now().Add(7 * 24 * time.Hour),
		EndsAt:    time.Now(),
	}

	err := svc.Update(context.Background(), "", "org-1", cycleRepo.cyclesByID["c1"])
	if err == nil {
		t.Fatal("expected error for end before start")
	}
}

func TestCycleService_GetByID(t *testing.T) {
	cycleRepo := newMockCycleRepo()
	taskRepo := newMockTaskRepo()
	svc := NewCycleService(cycleRepo, newMockProjectRepo(), taskRepo, nil)

	cycleRepo.cyclesByID["c1"] = &domain.Cycle{ID: "c1", ProjectID: "proj-1", Name: "Cycle 1"}
	cycleRepo.cyclesByID["c2"] = &domain.Cycle{ID: "c2", ProjectID: "proj-2", Name: "Other"}

	c, err := svc.GetByID(context.Background(), "c1", "proj-1")
	if err != nil {
		t.Fatalf("GetByID() error = %v", err)
	}
	if c.Name != "Cycle 1" {
		t.Errorf("Name = %q, want %q", c.Name, "Cycle 1")
	}

	_, err = svc.GetByID(context.Background(), "c2", "proj-1")
	if err == nil {
		t.Error("expected error for cycle from different project")
	}
}

func TestCycleService_GetActive(t *testing.T) {
	cycleRepo := newMockCycleRepo()
	taskRepo := newMockTaskRepo()
	svc := NewCycleService(cycleRepo, newMockProjectRepo(), taskRepo, nil)

	cycleRepo.cyclesByID["c1"] = &domain.Cycle{ID: "c1", ProjectID: "proj-1", Name: "Active C", IsActive: true}
	cycleRepo.cyclesByID["c2"] = &domain.Cycle{ID: "c2", ProjectID: "proj-1", Name: "Inactive", IsActive: false}

	c, err := svc.GetActive(context.Background(), "proj-1")
	if err != nil {
		t.Fatalf("GetActive() error = %v", err)
	}
	if c.Name != "Active C" {
		t.Errorf("Name = %q, want %q", c.Name, "Active C")
	}
}

func TestCycleService_Activate(t *testing.T) {
	cycleRepo := newMockCycleRepo()
	taskRepo := newMockTaskRepo()
	svc := NewCycleService(cycleRepo, newMockProjectRepo(), taskRepo, nil)

	cycleRepo.cyclesByID["c1"] = &domain.Cycle{ID: "c1", ProjectID: "proj-1", Name: "Cycle 1", IsActive: true}
	cycleRepo.cyclesByID["c2"] = &domain.Cycle{ID: "c2", ProjectID: "proj-1", Name: "Cycle 2", IsActive: false}

	activated, err := svc.Activate(context.Background(), "", "org-1", "c2", "proj-1")
	if err != nil {
		t.Fatalf("Activate() error = %v", err)
	}
	if !activated.IsActive {
		t.Error("expected cycle to be active")
	}

	c1, _ := cycleRepo.GetByID(context.Background(), "c1", "proj-1")
	if c1.IsActive {
		t.Error("expected old cycle to be inactive after activate")
	}
}

func TestCycleService_Complete_MoveToTarget(t *testing.T) {
	cycleRepo := newMockCycleRepo()
	taskRepo := newMockTaskRepo()
	cycleRepo.taskRepo = taskRepo
	svc := NewCycleService(cycleRepo, newMockProjectRepo(), taskRepo, nil)

	cycleRepo.cyclesByID["c1"] = &domain.Cycle{
		ID: "c1", OrgID: "org-1", ProjectID: "proj-1", Name: "Cycle 1", IsActive: true,
		StartsAt: time.Now(), EndsAt: time.Now().Add(14 * 24 * time.Hour),
	}

	taskRepo.tasksByID["t1"] = &domain.Task{
		ID: "t1", OrgID: "org-1", ProjectID: "proj-1", Title: "Task 1", CycleID: ptr("c1"),
	}
	// Completed task should stay in the old cycle
	completedAt := time.Now()
	taskRepo.tasksByID["t2"] = &domain.Task{
		ID: "t2", OrgID: "org-1", ProjectID: "proj-1", Title: "Completed Task", CycleID: ptr("c1"),
		CompletedAt: &completedAt,
	}

	cycleRepo.cyclesByID["c2"] = &domain.Cycle{
		ID: "c2", OrgID: "org-1", ProjectID: "proj-1", Name: "Target", IsActive: false,
		StartsAt: time.Now().Add(14 * 24 * time.Hour), EndsAt: time.Now().Add(28 * 24 * time.Hour),
	}

	completed, err := svc.Complete(context.Background(), "", "org-1", "c1", "proj-1", "c2")
	if err != nil {
		t.Fatalf("Complete() error = %v", err)
	}
	if !completed.IsCompleted {
		t.Error("expected cycle to be completed")
	}

	t1, _ := taskRepo.GetByID(context.Background(), "org-1", "t1", "proj-1")
	if t1.CycleID == nil || *t1.CycleID != "c2" {
		t.Errorf("expected incomplete task to move to c2, got cycle_id = %v", t1.CycleID)
	}

	t2, _ := taskRepo.GetByID(context.Background(), "org-1", "t2", "proj-1")
	if t2.CycleID == nil || *t2.CycleID != "c1" {
		t.Errorf("expected completed task to stay in c1, got cycle_id = %v", t2.CycleID)
	}
}

func TestCycleService_Complete_Backlog(t *testing.T) {
	cycleRepo := newMockCycleRepo()
	taskRepo := newMockTaskRepo()
	cycleRepo.taskRepo = taskRepo
	projRepo := newMockProjectRepo()
	svc := NewCycleService(cycleRepo, projRepo, taskRepo, nil)

	projRepo.projectsByID["proj-1"] = &domain.Project{
		ID:                     "proj-1",
		OrgID:                  "org-1",
		IncompleteTaskHandling: domain.CycleHandlingBacklog,
	}

	cycleRepo.cyclesByID["c1"] = &domain.Cycle{
		ID: "c1", OrgID: "org-1", ProjectID: "proj-1", Name: "Cycle 1", IsActive: true,
		StartsAt: time.Now(), EndsAt: time.Now().Add(14 * 24 * time.Hour),
	}

	taskRepo.tasksByID["t1"] = &domain.Task{
		ID: "t1", OrgID: "org-1", ProjectID: "proj-1", CycleID: ptr("c1"),
	}
	// Completed task should keep its cycle_id
	completedAt := time.Now()
	taskRepo.tasksByID["t2"] = &domain.Task{
		ID: "t2", OrgID: "org-1", ProjectID: "proj-1", CycleID: ptr("c1"),
		CompletedAt: &completedAt,
	}

	completed, err := svc.Complete(context.Background(), "", "org-1", "c1", "proj-1", "")
	if err != nil {
		t.Fatalf("Complete() error = %v", err)
	}
	if !completed.IsCompleted {
		t.Error("expected cycle to be completed")
	}

	t1, _ := taskRepo.GetByID(context.Background(), "org-1", "t1", "proj-1")
	if t1.CycleID != nil {
		t.Errorf("expected incomplete task cycle_id to be nil, got %v", *t1.CycleID)
	}

	t2, _ := taskRepo.GetByID(context.Background(), "org-1", "t2", "proj-1")
	if t2.CycleID == nil || *t2.CycleID != "c1" {
		t.Errorf("expected completed task to keep cycle_id c1, got %v", t2.CycleID)
	}
}

func TestCycleService_Complete_AutoGenerateNextCycle(t *testing.T) {
	cycleRepo := newMockCycleRepo()
	taskRepo := newMockTaskRepo()
	cycleRepo.taskRepo = taskRepo
	projRepo := newMockProjectRepo()
	svc := NewCycleService(cycleRepo, projRepo, taskRepo, nil)

	projRepo.projectsByID["proj-1"] = &domain.Project{
		ID:                     "proj-1",
		OrgID:                  "org-1",
		AutoGenerateCycles:     true,
		IncompleteTaskHandling: domain.CycleHandlingNextCycle,
	}

	start := time.Now()
	cycleRepo.cyclesByID["c1"] = &domain.Cycle{
		ID: "c1", OrgID: "org-1", ProjectID: "proj-1", Name: "Cycle 1", IsActive: true,
		StartsAt: start, EndsAt: start.Add(14 * 24 * time.Hour),
	}

	// Incomplete task
	taskRepo.tasksByID["t1"] = &domain.Task{
		ID: "t1", OrgID: "org-1", ProjectID: "proj-1", CycleID: ptr("c1"),
	}
	// Completed task stays
	completedAt := time.Now()
	taskRepo.tasksByID["t2"] = &domain.Task{
		ID: "t2", OrgID: "org-1", ProjectID: "proj-1", CycleID: ptr("c1"),
		CompletedAt: &completedAt,
	}

	completed, err := svc.Complete(context.Background(), "", "org-1", "c1", "proj-1", "")
	if err != nil {
		t.Fatalf("Complete() error = %v", err)
	}
	if !completed.IsCompleted {
		t.Error("expected cycle to be completed")
	}
	if completed.IsActive {
		t.Error("expected completed cycle to be inactive")
	}

	// Find the new auto-generated cycle
	var newCycle *domain.Cycle
	for _, c := range cycleRepo.cyclesByID {
		if c.ID != "c1" && c.ProjectID == "proj-1" && c.IsActive {
			newCycle = c
			break
		}
	}
	if newCycle == nil {
		t.Fatal("expected a new active cycle to be auto-generated")
	}
	if newCycle.IsCompleted {
		t.Error("new cycle should not be completed")
	}
	if !newCycle.StartsAt.Equal(start.Add(14 * 24 * time.Hour)) {
		t.Errorf("new cycle starts_at = %v, want %v", newCycle.StartsAt, start.Add(14*24*time.Hour))
	}

	// Incomplete task should have moved to the new cycle
	t1, _ := taskRepo.GetByID(context.Background(), "org-1", "t1", "proj-1")
	if t1.CycleID == nil || *t1.CycleID != newCycle.ID {
		t.Errorf("expected incomplete task to move to new cycle %s, got %v", newCycle.ID, t1.CycleID)
	}

	// Completed task should have stayed
	t2, _ := taskRepo.GetByID(context.Background(), "org-1", "t2", "proj-1")
	if t2.CycleID == nil || *t2.CycleID != "c1" {
		t.Errorf("expected completed task to stay in c1, got %v", t2.CycleID)
	}
}
