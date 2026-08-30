package store

import (
	"path/filepath"
	"testing"

	"ipmanlk/breeze/internal/store/migration"

	_ "modernc.org/sqlite"
)

// TestMigrations_CreateSubtasksSchema verifies the baseline (00001_initial.sql)
// produces the correct schema for subtasks on a fresh database: the
// subtask_position column exists, the idx_tasks_parent index exists, and the
// old checklist_items table is absent (subtasks replaced checklists).
func TestMigrations_CreateSubtasksSchema(t *testing.T) {
	tmpDir := t.TempDir()
	dbPath := filepath.Join(tmpDir, "test.db")

	conn, err := NewDB(dbPath)
	if err != nil {
		t.Fatalf("open db: %v", err)
	}
	t.Cleanup(func() { conn.Close() })

	if err := migration.RunMigrations(conn); err != nil {
		t.Fatalf("run migrations: %v", err)
	}

	// subtask_position column must exist on tasks.
	var col string
	err = conn.QueryRow(
		`SELECT name FROM pragma_table_info('tasks') WHERE name = 'subtask_position'`,
	).Scan(&col)
	if err != nil {
		t.Errorf("subtask_position column missing after migrations: %v", err)
	}

	// idx_tasks_parent partial index must exist.
	var idx string
	err = conn.QueryRow(
		`SELECT name FROM sqlite_master WHERE type = 'index' AND name = 'idx_tasks_parent'`,
	).Scan(&idx)
	if err != nil {
		t.Errorf("idx_tasks_parent index missing after migrations: %v", err)
	}

	// checklist_items must not exist: subtasks replaced checklists, so the
	// baseline never defines it.
	var tbl string
	err = conn.QueryRow(
		`SELECT name FROM sqlite_master WHERE type = 'table' AND name = 'checklist_items'`,
	).Scan(&tbl)
	if err == nil {
		t.Errorf("checklist_items table still exists after migrations; want it dropped")
	}

	// A subtask can be created + ordered by subtask_position (exercises the
	// ListSubtasks ORDER BY path that crashed on legacy DBs without the column).
	parent := "parent-1"
	mustExec(t, conn, `INSERT INTO organizations (id, name, slug) VALUES ('org-1', 'O', 'o')`)
	mustExec(t, conn, `INSERT INTO users (id, org_id, email, name, role) VALUES ('u-1', 'org-1', 'a@b.c', 'A', 'admin')`)
	mustExec(t, conn, `INSERT INTO projects (id, org_id, slug, name, created_by) VALUES ('p-1', 'org-1', 'p', 'P', 'u-1')`)
	mustExec(t, conn, `INSERT INTO task_statuses (id, project_id, name, color, category) VALUES ('s-1', 'p-1', 'Todo', '#fff', 'todo')`)
	mustExec(t, conn, `INSERT INTO tasks (id, org_id, project_id, created_by, title, description, status_id, priority, position_key, created_at, updated_at) VALUES (?, 'org-1', 'p-1', 'u-1', 'Parent', '', 's-1', 'none', 'a', datetime('now'), datetime('now'))`, parent)
	mustExec(t, conn, `INSERT INTO tasks (id, org_id, project_id, parent_task_id, created_by, title, description, status_id, priority, position_key, subtask_position, created_at, updated_at) VALUES ('c-1', 'org-1', 'p-1', ?, 'u-1', 'Child', '', 's-1', 'none', 'a', 'a0', datetime('now'), datetime('now'))`, parent)

	var got string
	err = conn.QueryRow(
		`SELECT id FROM tasks WHERE parent_task_id = ? ORDER BY subtask_position ASC LIMIT 1`,
		parent,
	).Scan(&got)
	if err != nil {
		t.Errorf("query subtask by subtask_position: %v", err)
	}
	if got != "c-1" {
		t.Errorf("subtask query returned %q, want c-1", got)
	}
}

// TestMigrations_IdempotentReapply guards against double-apply: running the
// squashed baseline (00001_initial.sql) twice on the same DB must be a no-op,
// not an error. Every statement in the baseline uses IF NOT EXISTS / IF EXISTS
// so goose detecting all versions applied means the second run changes nothing.
func TestMigrations_IdempotentReapply(t *testing.T) {
	tmpDir := t.TempDir()
	dbPath := filepath.Join(tmpDir, "test.db")

	conn, err := NewDB(dbPath)
	if err != nil {
		t.Fatalf("open db: %v", err)
	}
	t.Cleanup(func() { conn.Close() })

	if err := migration.RunMigrations(conn); err != nil {
		t.Fatalf("first run: %v", err)
	}
	// Second run must succeed (goose detects all applied) and not error on
	// e.g. a duplicate CREATE INDEX without IF NOT EXISTS.
	if err := migration.RunMigrations(conn); err != nil {
		t.Fatalf("second run (should be no-op): %v", err)
	}

	// Confirm the schema is still correct.
	var n int
	_ = conn.QueryRow(`SELECT COUNT(*) FROM pragma_table_info('tasks') WHERE name = 'subtask_position'`).Scan(&n)
	if n != 1 {
		t.Errorf("subtask_position column count = %d after reapply, want 1", n)
	}
}
