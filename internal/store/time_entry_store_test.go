package store

import (
	"context"
	"database/sql"
	"path/filepath"
	"strings"
	"testing"

	"ipmanlk/breeze/internal/store/migration"
	"ipmanlk/breeze/internal/store/sqlc"

	_ "modernc.org/sqlite"
)

// setupTimeEntryStoreTest boots a fresh SQLite DB with migrations and seed
// data for time-entry store tests.
func setupTimeEntryStoreTest(t *testing.T) (ctx context.Context, store *TimeEntryStore, conn *sql.DB, ids struct{ org, user, taskA, taskB string }) {
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
	store = NewTimeEntryStore(q, conn)

	org := "org-1"
	mustExec(t, conn, `INSERT INTO organizations (id, name, slug) VALUES (?, 'Org', 'org')`, org)
	mustExec(t, conn, `INSERT INTO accounts (id, email, password_hash) VALUES ('acct-1', 'admin@test.com', 'hash')`)
	mustExec(t, conn, `INSERT INTO users (id, account_id, org_id, name, email) VALUES ('user-1', 'acct-1', ?, 'Admin', 'admin@test.com')`, org)
	mustExec(t, conn, `INSERT INTO projects (id, org_id, name, slug, created_by) VALUES ('proj-1', ?, 'Project', 'proj', 'user-1')`, org)
	mustExec(t, conn, `INSERT INTO task_statuses (id, project_id, name, color, position) VALUES ('status-1', 'proj-1', 'Todo', '#888', 0)`)
	mustExec(t, conn, `INSERT INTO tasks (id, org_id, project_id, created_by, title, status_id, priority) VALUES ('task-a', ?, 'proj-1', 'user-1', 'Task A', 'status-1', 'none')`, org)
	mustExec(t, conn, `INSERT INTO tasks (id, org_id, project_id, created_by, title, status_id, priority) VALUES ('task-b', ?, 'proj-1', 'user-1', 'Task B', 'status-1', 'none')`, org)

	ids = struct{ org, user, taskA, taskB string }{"org-1", "user-1", "task-a", "task-b"}
	return ctx, store, conn, ids
}

// TestTimeEntryStore_UniqueIndex_RejectsSecondActiveTimer verifies that the
// partial unique index (idx_time_entries_active_user) prevents two active
// timers for the same user at the SQL level. This is the safety net that
// catches concurrent Start requests that might race past the stop step.
func TestTimeEntryStore_UniqueIndex_RejectsSecondActiveTimer(t *testing.T) {
	ctx, store, conn, ids := setupTimeEntryStoreTest(t)

	// Create the first active timer via the store method.
	if err := store.StartTimerAtomic(ctx, "timer-1", ids.taskA, ids.user, "first timer"); err != nil {
		t.Fatalf("first StartTimerAtomic: %v", err)
	}

	// Verify exactly one active timer exists.
	var count int
	if err := conn.QueryRowContext(ctx, `SELECT COUNT(*) FROM time_entries WHERE user_id = ? AND ended_at IS NULL`, ids.user).Scan(&count); err != nil {
		t.Fatalf("count: %v", err)
	}
	if count != 1 {
		t.Fatalf("expected 1 active timer, got %d", count)
	}

	// Try to INSERT a second active timer via raw SQL (bypassing the
	// stop-first logic of StartTimerAtomic). The partial unique index
	// should reject this.
	_, err := conn.ExecContext(ctx,
		`INSERT INTO time_entries (id, task_id, user_id, description, started_at)
		 VALUES ('timer-2', ?, ?, 'bad timer', datetime('now'))`,
		ids.taskB, ids.user)
	if err == nil {
		t.Fatal("expected unique constraint error for duplicate active timer, got nil")
	}
	if !strings.Contains(strings.ToLower(err.Error()), "unique constraint") {
		t.Fatalf("expected 'unique constraint' in error, got: %v", err)
	}

	// Verify still exactly one active timer.
	count = 0
	if err := conn.QueryRowContext(ctx, `SELECT COUNT(*) FROM time_entries WHERE user_id = ? AND ended_at IS NULL`, ids.user).Scan(&count); err != nil {
		t.Fatalf("count: %v", err)
	}
	if count != 1 {
		t.Fatalf("expected 1 active timer after rejected insert, got %d", count)
	}
}

// TestTimeEntryStore_StartTimerAtomic_ReplacesExistingTimer verifies that
// StartTimerAtomic atomically stops any existing active timer and starts a
// new one, resulting in exactly one active timer.
func TestTimeEntryStore_StartTimerAtomic_ReplacesExistingTimer(t *testing.T) {
	ctx, store, conn, ids := setupTimeEntryStoreTest(t)

	// Start a timer for task-a.
	if err := store.StartTimerAtomic(ctx, "timer-1", ids.taskA, ids.user, "first"); err != nil {
		t.Fatalf("first StartTimerAtomic: %v", err)
	}

	// Start a second timer for task-b (same user). This should succeed:
	// StartTimerAtomic stops any existing timer before starting the new one.
	if err := store.StartTimerAtomic(ctx, "timer-2", ids.taskB, ids.user, "second"); err != nil {
		t.Fatalf("second StartTimerAtomic should succeed (replaces): %v", err)
	}

	// Verify exactly one active timer.
	var count int
	if err := conn.QueryRowContext(ctx, `SELECT COUNT(*) FROM time_entries WHERE user_id = ? AND ended_at IS NULL`, ids.user).Scan(&count); err != nil {
		t.Fatalf("count: %v", err)
	}
	if count != 1 {
		t.Fatalf("expected 1 active timer after replace, got %d", count)
	}

	// Verify the old timer was stopped.
	var endedAt any
	if err := conn.QueryRowContext(ctx, `SELECT ended_at FROM time_entries WHERE id = 'timer-1'`).Scan(&endedAt); err != nil {
		t.Fatalf("read timer-1: %v", err)
	}
	if endedAt == nil {
		t.Fatal("expected timer-1 to have ended_at set after replacement")
	}

	// Verify the new timer is active.
	var desc string
	var endedAt2 any
	if err := conn.QueryRowContext(ctx, `SELECT description, ended_at FROM time_entries WHERE id = 'timer-2'`).Scan(&desc, &endedAt2); err != nil {
		t.Fatalf("read timer-2: %v", err)
	}
	if desc != "second" {
		t.Errorf("timer-2 description = %q, want %q", desc, "second")
	}
	if endedAt2 != nil {
		t.Fatal("expected timer-2 to be active (ended_at IS NULL)")
	}
}
