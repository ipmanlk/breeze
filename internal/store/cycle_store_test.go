package store

import (
	"context"
	"database/sql"
	"testing"
	"time"

	"ipmanlk/plume/internal/domain"
	"ipmanlk/plume/internal/store/migration"
	"ipmanlk/plume/internal/store/sqlc"

	_ "modernc.org/sqlite"
)

func TestCycleStore_CompleteCycle_Atomicity(t *testing.T) {
	ctx := context.Background()
	conn := setupTestCycleStore(t)
	q := sqlc.New(conn)
	store := NewCycleStore(q, conn)

	// Seed: org + project + two cycles + two tasks (one incomplete, one complete).
	createTestCycleData(t, q, "org-1", "proj-1")

	c1, _ := q.GetCycleByID(ctx, sqlc.GetCycleByIDParams{ID: "c1", ProjectID: "proj-1"})
	if c1.IsActive != true {
		t.Fatal("expected c1 to be active before CompleteCycle")
	}
	if c1.IsCompleted != false {
		t.Fatal("expected c1 to not be completed before CompleteCycle")
	}

	// Execute the complete plan: move incomplete tasks to c2, activate c2, complete c1.
	now := time.Now()
	completedCycle := &domain.Cycle{
		ID:          "c1",
		OrgID:       "org-1",
		ProjectID:   "proj-1",
		Name:        "Cycle 1",
		Goal:        "",
		StartsAt:    now.Add(-14 * 24 * time.Hour),
		EndsAt:      now,
		CreatedBy:   "user-1",
		IsCompleted: true,
		IsActive:    false,
		UpdatedAt:   now,
	}

	plan := domain.CycleCompletionPlan{
		OrgID:             "org-1",
		ProjectID:         "proj-1",
		CompletedCycleID:  "c1",
		CompletedCycle:    *completedCycle,
		MoveTargetCycleID: "c2",
		SetActiveCycleID:  "c2",
	}

	if err := store.CompleteCycle(ctx, plan); err != nil {
		t.Fatalf("CompleteCycle() error = %v", err)
	}

	// Verify: c1 completed, inactive
	updatedC1, _ := q.GetCycleByID(ctx, sqlc.GetCycleByIDParams{ID: "c1", ProjectID: "proj-1"})
	if !updatedC1.IsCompleted {
		t.Error("c1 should be completed")
	}
	if updatedC1.IsActive {
		t.Error("c1 should be inactive")
	}

	// Verify: c2 active
	updatedC2, _ := q.GetCycleByID(ctx, sqlc.GetCycleByIDParams{ID: "c2", ProjectID: "proj-1"})
	if !updatedC2.IsActive {
		t.Error("c2 should be active")
	}
	if updatedC2.IsCompleted {
		t.Error("c2 should not be completed")
	}

	// Verify: incomplete task t1 moved to c2
	task1, _ := q.GetTaskByID(ctx, sqlc.GetTaskByIDParams{ID: "t1", ProjectID: "proj-1", OrgID: "org-1"})
	if task1.CycleID == nil || *task1.CycleID != "c2" {
		t.Errorf("incomplete task should be in c2, got cycle_id = %v", task1.CycleID)
	}

	// Verify: completed task t2 stayed in c1
	task2, _ := q.GetTaskByID(ctx, sqlc.GetTaskByIDParams{ID: "t2", ProjectID: "proj-1", OrgID: "org-1"})
	if task2.CycleID == nil || *task2.CycleID != "c1" {
		t.Errorf("completed task should stay in c1, got cycle_id = %v", task2.CycleID)
	}
}

func TestCycleStore_CompleteCycle_AutoGenerate(t *testing.T) {
	ctx := context.Background()
	conn := setupTestCycleStore(t)
	q := sqlc.New(conn)
	store := NewCycleStore(q, conn)

	createTestCycleData(t, q, "org-1", "proj-1")

	now := time.Now()
	completedCycle := &domain.Cycle{
		ID:          "c1",
		OrgID:       "org-1",
		ProjectID:   "proj-1",
		CreatedBy:   "user-1",
		IsCompleted: true,
		IsActive:    false,
		UpdatedAt:   now,
	}

	newCycle := &domain.Cycle{
		ID:        "c3",
		OrgID:     "org-1",
		ProjectID: "proj-1",
		Name:      "Cycle 3",
		StartsAt:  now,
		EndsAt:    now.Add(14 * 24 * time.Hour),
		CreatedBy: "user-1",
		CreatedAt: now,
		UpdatedAt: now,
	}

	plan := domain.CycleCompletionPlan{
		OrgID:             "org-1",
		ProjectID:         "proj-1",
		CompletedCycleID:  "c1",
		CompletedCycle:    *completedCycle,
		NewCycle:          newCycle,
		MoveTargetCycleID: "c3",
		SetActiveCycleID:  "c3",
	}

	if err := store.CompleteCycle(ctx, plan); err != nil {
		t.Fatalf("CompleteCycle(auto-gen) error = %v", err)
	}

	// Verify new cycle was created and is active
	c3, err := q.GetCycleByID(ctx, sqlc.GetCycleByIDParams{ID: "c3", ProjectID: "proj-1"})
	if err != nil {
		t.Fatal("expected c3 to exist")
	}
	if !c3.IsActive {
		t.Error("c3 should be active")
	}

	// c1 should be completed
	c1, _ := q.GetCycleByID(ctx, sqlc.GetCycleByIDParams{ID: "c1", ProjectID: "proj-1"})
	if !c1.IsCompleted {
		t.Error("c1 should be completed")
	}

	// Incomplete task should have moved to c3
	task1, _ := q.GetTaskByID(ctx, sqlc.GetTaskByIDParams{ID: "t1", ProjectID: "proj-1", OrgID: "org-1"})
	if task1.CycleID == nil || *task1.CycleID != "c3" {
		t.Errorf("incomplete task should be in c3, got cycle_id = %v", task1.CycleID)
	}
}

// setupTestCycleStore boots a fresh SQLite DB with migrations for cycle tests.
func setupTestCycleStore(t *testing.T) *sql.DB {
	t.Helper()
	db, err := NewDB(t.TempDir() + "/test.db")
	if err != nil {
		t.Fatalf("NewDB: %v", err)
	}
	t.Cleanup(func() { db.Close() })
	if err := migration.RunMigrations(db); err != nil {
		t.Fatalf("RunMigrations: %v", err)
	}
	return db
}

// createTestCycleData seeds org, project, cycles, and tasks for cycle tests.
func createTestCycleData(t *testing.T, q *sqlc.Queries, orgID, projectID string) {
	t.Helper()
	ctx := context.Background()

	// Create org
	if err := q.CreateOrganization(ctx, sqlc.CreateOrganizationParams{
		ID: orgID, Name: "Test Org", Slug: "test-org",
	}); err != nil {
		t.Fatalf("create org: %v", err)
	}

	// Create org account + user (referenced by project.created_by and cycle.created_by)
	acctID := "acct-1"
	if err := q.CreateAccount(ctx, sqlc.CreateAccountParams{
		ID: acctID, Email: "admin@test.com", PasswordHash: "hash",
	}); err != nil {
		t.Fatalf("create account: %v", err)
	}
	if err := q.CreateUser(ctx, sqlc.CreateUserParams{
		ID: "user-1", AccountID: &acctID, OrgID: orgID,
		Email: "admin@test.com", Name: "Admin", Role: string(domain.RoleAdmin),
		IsActive: 1,
	}); err != nil {
		t.Fatalf("create user: %v", err)
	}

	// Create project
	if err := q.CreateProject(ctx, sqlc.CreateProjectParams{
		OrgID: orgID, ID: projectID, Name: "Test Project", Slug: "test-project",
		Description: "", Color: "#000", Icon: "", CreatedBy: "user-1",
		IncompleteTaskHandling: string(domain.CycleHandlingBacklog),
	}); err != nil {
		t.Fatalf("create project: %v", err)
	}

	// Create two cycles
	now := time.Now()
	fmtTime := func(t time.Time) string { return t.UTC().Format("2006-01-02 15:04:05") }
	if err := q.CreateCycle(ctx, sqlc.CreateCycleParams{
		ID: "c1", OrgID: orgID, ProjectID: projectID, Name: "Cycle 1",
		StartsAt:  fmtTime(now.Add(-28 * 24 * time.Hour)),
		EndsAt:    fmtTime(now.Add(-14 * 24 * time.Hour)),
		CreatedBy: "user-1",
	}); err != nil {
		t.Fatalf("create c1: %v", err)
	}
	if err := q.CreateCycle(ctx, sqlc.CreateCycleParams{
		ID: "c2", OrgID: orgID, ProjectID: projectID, Name: "Cycle 2",
		StartsAt:  fmtTime(now.Add(-14 * 24 * time.Hour)),
		EndsAt:    fmtTime(now),
		CreatedBy: "user-1",
	}); err != nil {
		t.Fatalf("create c2: %v", err)
	}

	// Set c1 as active
	if err := q.SetCycleActive(ctx, sqlc.SetCycleActiveParams{ID: "c1", ProjectID: projectID}); err != nil {
		t.Fatalf("activate c1: %v", err)
	}

	// Create a status
	if err := q.CreateStatus(ctx, sqlc.CreateStatusParams{
		ID: "st-1", ProjectID: projectID, Name: "Todo",
		Category: "todo", Position: 0, Color: "#888", IsDefault: true,
	}); err != nil {
		t.Fatalf("create status: %v", err)
	}

	// Create two tasks in c1: t1 (incomplete), t2 (completed)
	if err := q.CreateTask(ctx, sqlc.CreateTaskParams{
		ID: "t1", ProjectID: projectID, OrgID: orgID, Title: "Task 1",
		StatusID: "st-1", PositionKey: "a", CycleID: ptr("c1"),
		CreatedBy: "user-1", Priority: "none",
	}); err != nil {
		t.Fatalf("create t1: %v", err)
	}
	completedAt := fmtTime(now)
	if err := q.CreateTask(ctx, sqlc.CreateTaskParams{
		ID: "t2", ProjectID: projectID, OrgID: orgID, Title: "Task 2",
		StatusID: "st-1", PositionKey: "b", CycleID: ptr("c1"),
		CompletedAt: &completedAt, CreatedBy: "user-1", Priority: "none",
	}); err != nil {
		t.Fatalf("create t2: %v", err)
	}
}

func ptr(s string) *string { return &s }
