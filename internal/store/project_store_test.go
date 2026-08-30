package store

import (
	"context"
	"database/sql"
	"path/filepath"
	"testing"

	"ipmanlk/breeze/internal/domain"
	"ipmanlk/breeze/internal/store/migration"
	"ipmanlk/breeze/internal/store/sqlc"

	_ "modernc.org/sqlite"
)

// setupProjectStoreTest boots a fresh SQLite DB with migrations and minimal
// seed data for project-store tests.
func setupProjectStoreTest(t *testing.T) (ctx context.Context, store *ProjectStore, conn *sql.DB, q *sqlc.Queries, orgID string) {
	t.Helper()
	ctx = context.Background()
	tmpDir := t.TempDir()
	dbPath := filepath.Join(tmpDir, "test.db")

	conn, err := NewDB(dbPath)
	if err != nil {
		t.Fatalf("NewDB: %v", err)
	}
	t.Cleanup(func() { conn.Close() })

	if err := migration.RunMigrations(conn); err != nil {
		t.Fatalf("RunMigrations: %v", err)
	}

	q = sqlc.New(conn)
	store = NewProjectStore(q, conn)

	orgID = "org-1"
	mustExec(t, conn, `INSERT INTO organizations (id, name, slug) VALUES (?, 'Test Org', 'test-org')`, orgID)
	mustExec(t, conn, `INSERT INTO accounts (id, email, password_hash) VALUES ('acct-1', 'admin@test.com', 'hash')`)
	mustExec(t, conn, `INSERT INTO users (id, account_id, org_id, name, email) VALUES ('user-1', 'acct-1', ?, 'Admin', 'admin@test.com')`, orgID)

	return ctx, store, conn, q, orgID
}

// TestProjectStore_CreateWithStatuses_AllSixCreated verifies that
// CreateWithStatuses creates the project and all 6 statuses atomically.
func TestProjectStore_CreateWithStatuses_AllSixCreated(t *testing.T) {
	ctx, store, _, q, orgID := setupProjectStoreTest(t)

	project := &domain.Project{
		ID:                     "proj-1",
		OrgID:                  orgID,
		Name:                   "My Project",
		Slug:                   "my-project",
		Color:                  "oklch(0.6 0.15 250)",
		Icon:                   "FolderIcon",
		CreatedBy:              "user-1",
		IncompleteTaskHandling: domain.CycleHandlingBacklog,
	}

	statuses := []*domain.TaskStatus{
		{ID: "status-1", ProjectID: "proj-1", Name: "Backlog", Color: "#94a3b8", Position: 0, Category: "todo"},
		{ID: "status-2", ProjectID: "proj-1", Name: "Todo", Color: "#3b82f6", Position: 1, Category: "todo"},
		{ID: "status-3", ProjectID: "proj-1", Name: "In Progress", Color: "#f59e0b", Position: 2, Category: "in_progress"},
		{ID: "status-4", ProjectID: "proj-1", Name: "In Review", Color: "#8b5cf6", Position: 3, Category: "in_progress"},
		{ID: "status-5", ProjectID: "proj-1", Name: "Done", Color: "#22c55e", Position: 4, Category: "done"},
		{ID: "status-6", ProjectID: "proj-1", Name: "Canceled", Color: "#ef4444", Position: 5, Category: "canceled"},
	}

	if err := store.CreateWithStatuses(ctx, project, statuses); err != nil {
		t.Fatalf("CreateWithStatuses: %v", err)
	}

	// Verify the project exists.
	p, err := q.GetProjectByID(ctx, sqlc.GetProjectByIDParams{ID: "proj-1", OrgID: orgID})
	if err != nil {
		t.Fatalf("project not found after create: %v", err)
	}
	if p.Name != "My Project" {
		t.Errorf("project name = %q, want %q", p.Name, "My Project")
	}

	// Verify all 6 statuses exist for the project.
	rows, err := q.ListStatusesByProject(ctx, "proj-1")
	if err != nil {
		t.Fatalf("list statuses: %v", err)
	}
	if len(rows) != 6 {
		t.Fatalf("expected 6 statuses, got %d", len(rows))
	}
}

// TestProjectStore_CreateWithStatuses_RollbackOnDuplicateStatus proves that
// when a status insert fails (duplicate PK), the previously-inserted project
// is rolled back and NOT persisted.
func TestProjectStore_CreateWithStatuses_RollbackOnDuplicateStatus(t *testing.T) {
	ctx, store, conn, q, orgID := setupProjectStoreTest(t)

	// Pre-insert a status with ID "status-dup" for a different project to
	// reserve the primary key value. We'll reuse this ID inside the tx to
	// trigger a PRIMARY KEY violation.
	mustExec(t, conn, `INSERT INTO projects (id, org_id, name, slug, created_by) VALUES ('other-proj', ?, 'Other', 'other', 'user-1')`, orgID)
	mustExec(t, conn, `INSERT INTO task_statuses (id, project_id, name, color, position) VALUES ('status-dup', 'other-proj', 'Dup', '#000', 99)`)

	project := &domain.Project{
		ID:                     "proj-1",
		OrgID:                  orgID,
		Name:                   "New Project",
		Slug:                   "new-project",
		Color:                  "oklch(0.6 0.15 250)",
		Icon:                   "FolderIcon",
		CreatedBy:              "user-1",
		IncompleteTaskHandling: domain.CycleHandlingBacklog,
	}

	statuses := []*domain.TaskStatus{
		{ID: "status-dup", ProjectID: "proj-1", Name: "Backlog", Color: "#94a3b8", Position: 0, Category: "todo"},
		{ID: "status-2", ProjectID: "proj-1", Name: "Todo", Color: "#3b82f6", Position: 1, Category: "todo"},
	}

	err := store.CreateWithStatuses(ctx, project, statuses)
	if err == nil {
		t.Fatal("expected error (duplicate status PK), got nil")
	}

	// Assert the new project was NOT persisted (transaction rolled back).
	_, err = q.GetProjectByID(ctx, sqlc.GetProjectByIDParams{ID: "proj-1", OrgID: orgID})
	if err == nil {
		t.Fatal("expected project to not exist after rollback")
	}

	// Assert no statuses for the new project were created.
	rows, err := q.ListStatusesByProject(ctx, "proj-1")
	if err != nil {
		t.Fatalf("list statuses for rolled-back project: %v", err)
	}
	if len(rows) != 0 {
		t.Fatalf("expected 0 statuses for rolled-back project, got %d", len(rows))
	}

	// Assert the pre-existing status still exists.
	row, err := q.GetStatusByID(ctx, "status-dup")
	if err != nil {
		t.Fatalf("pre-existing status should remain: %v", err)
	}
	if row.ProjectID != "other-proj" {
		t.Errorf("pre-existing status project_id = %q, want %q", row.ProjectID, "other-proj")
	}
}
