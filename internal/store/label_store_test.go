package store

import (
	"context"
	"database/sql"
	"path/filepath"
	"testing"

	"ipmanlk/breeze/internal/store/migration"
	"ipmanlk/breeze/internal/store/sqlc"

	_ "modernc.org/sqlite"
)

// setupLabelStoreTest boots a fresh SQLite DB with an org, project, task, and
// two labels for label-store tests.
func setupLabelStoreTest(t *testing.T) (ctx context.Context, store *LabelStore, conn *sql.DB, ids struct{ org, proj, task, labelA, labelB string }) {
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

	q := sqlc.New(conn)
	store = NewLabelStore(q, conn)

	org := "org-1"
	mustExec(t, conn, `INSERT INTO organizations (id, name, slug) VALUES (?, 'Test Org', 'test-org')`, org)
	mustExec(t, conn, `INSERT INTO accounts (id, email, password_hash) VALUES ('acct-1', 'admin@test.com', 'hash')`)
	mustExec(t, conn, `INSERT INTO users (id, account_id, org_id, name, email) VALUES ('user-1', 'acct-1', ?, 'Admin', 'admin@test.com')`, org)
	mustExec(t, conn, `INSERT INTO projects (id, org_id, name, slug, created_by) VALUES ('proj-1', ?, 'Project 1', 'proj-1', 'user-1')`, org)
	mustExec(t, conn, `INSERT INTO task_statuses (id, project_id, name, color, position) VALUES ('status-1', 'proj-1', 'Todo', '#888', 0)`)
	mustExec(t, conn, `INSERT INTO tasks (id, org_id, project_id, created_by, title, status_id, priority) VALUES ('task-1', ?, 'proj-1', 'user-1', 'Test Task', 'status-1', 'none')`, org)

	// Two labels
	mustExec(t, conn, `INSERT INTO labels (id, org_id, name, color) VALUES ('label-a', ?, 'Label A', '#f00')`, org)
	mustExec(t, conn, `INSERT INTO labels (id, org_id, name, color) VALUES ('label-b', ?, 'Label B', '#0f0')`, org)

	ids = struct{ org, proj, task, labelA, labelB string }{"org-1", "proj-1", "task-1", "label-a", "label-b"}
	return ctx, store, conn, ids
}

// TestLabelStore_SetTaskLabels_Atomic verifies that SetTaskLabels correctly
// replaces all labels for a task.
func TestLabelStore_SetTaskLabels_Atomic(t *testing.T) {
	ctx, store, _, ids := setupLabelStoreTest(t)

	// 1. Set labels to [label-a, label-b].
	if err := store.SetTaskLabels(ctx, ids.task, []string{ids.labelA, ids.labelB}); err != nil {
		t.Fatalf("SetTaskLabels [A,B]: %v", err)
	}

	labels, err := store.GetTaskLabels(ctx, ids.task)
	if err != nil {
		t.Fatalf("GetTaskLabels after first set: %v", err)
	}
	if len(labels) != 2 {
		t.Fatalf("expected 2 labels after first set, got %d", len(labels))
	}

	// 2. Replace with just [label-a].
	if err := store.SetTaskLabels(ctx, ids.task, []string{ids.labelA}); err != nil {
		t.Fatalf("SetTaskLabels [A]: %v", err)
	}

	labels, err = store.GetTaskLabels(ctx, ids.task)
	if err != nil {
		t.Fatalf("GetTaskLabels after replace: %v", err)
	}
	if len(labels) != 1 {
		t.Fatalf("expected 1 label after replace, got %d", len(labels))
	}
	if labels[0].ID != ids.labelA {
		t.Errorf("remaining label = %q, want %q", labels[0].ID, ids.labelA)
	}
}

// TestLabelStore_SetTaskLabels_RollbackOnInvalidLabel proves that when an
// AddTaskLabel insert fails (FK constraint on non-existent label_id), the
// preceding ClearTaskLabels is rolled back and the original labels remain.
func TestLabelStore_SetTaskLabels_RollbackOnInvalidLabel(t *testing.T) {
	ctx, store, _, ids := setupLabelStoreTest(t)

	// Pre-set labels [label-a, label-b].
	if err := store.SetTaskLabels(ctx, ids.task, []string{ids.labelA, ids.labelB}); err != nil {
		t.Fatalf("initial SetTaskLabels: %v", err)
	}

	// Attempt to replace with a valid label + a non-existent label.
	// task_labels.label_id has FK REFERENCES labels(id) ON DELETE CASCADE,
	// so "nonexistent" triggers a FK violation inside the tx.
	err := store.SetTaskLabels(ctx, ids.task, []string{ids.labelA, "nonexistent"})
	if err == nil {
		t.Fatal("expected error for non-existent label ID, got nil")
	}

	// Assert the ORIGINAL labels are still present (tx rolled back).
	labels, err := store.GetTaskLabels(ctx, ids.task)
	if err != nil {
		t.Fatalf("GetTaskLabels after failed replace: %v", err)
	}
	if len(labels) != 2 {
		t.Fatalf("expected 2 labels after rollback, got %d", len(labels))
	}

	seen := make(map[string]bool)
	for _, l := range labels {
		seen[l.ID] = true
	}
	if !seen[ids.labelA] {
		t.Errorf("label-a missing after rollback")
	}
	if !seen[ids.labelB] {
		t.Errorf("label-b missing after rollback")
	}
}
