package store

import (
	"context"
	"database/sql"
	"path/filepath"
	"testing"
	"time"

	"ipmanlk/breeze/internal/domain"
	"ipmanlk/breeze/internal/store/migration"
	"ipmanlk/breeze/internal/store/sqlc"

	_ "modernc.org/sqlite"
)

// setupTaskLabelTest boots an isolated DB with an org, user, project, status,
// and three tasks. Returns the store plus the IDs the tests need.
func setupTaskLabelTest(t *testing.T) (ctx context.Context, store *TaskStore, labelStore *LabelStore, conn *sql.DB, ids struct{ org, proj, status, taskA, taskB, taskC, labelBug, labelFE string }) {
	t.Helper()
	ctx = context.Background()
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

	q := sqlc.New(conn)
	store = NewTaskStore(q, conn)
	labelStore = NewLabelStore(q, conn)

	org := "org-1"
	mustExec(t, conn, `INSERT INTO organizations (id, name, slug) VALUES (?, 'Test Org', 'test-org')`, org)
	mustExec(t, conn, `INSERT INTO accounts (id, email, password_hash) VALUES ('acct-1', 'alice@test.com', 'hash')`)
	mustExec(t, conn, `INSERT INTO users (id, account_id, org_id, name, email) VALUES ('user-1', 'acct-1', ?, 'Alice', 'alice@test.com')`, org)
	mustExec(t, conn, `INSERT INTO projects (id, org_id, name, slug, created_by) VALUES ('proj-1', ?, 'Project 1', 'proj-1', 'user-1')`, org)
	mustExec(t, conn, `INSERT INTO task_statuses (id, project_id, name, color, position) VALUES ('status-1', 'proj-1', 'Todo', '#888', 0)`)

	for _, tid := range []string{"task-a", "task-b", "task-c"} {
		mustExec(t, conn,
			`INSERT INTO tasks (id, org_id, project_id, created_by, title, status_id, priority) VALUES (?, ?, 'proj-1', 'user-1', ?, 'status-1', 'none')`,
			tid, org, tid,
		)
	}

	// Two org-scoped labels.
	mustExec(t, conn, `INSERT INTO labels (id, org_id, name, color) VALUES ('label-bug', ?, 'Bug', '#ef4444')`, org)
	mustExec(t, conn, `INSERT INTO labels (id, org_id, name, color) VALUES ('label-fe', ?, 'Frontend', '#22c55e')`, org)

	// task-a: Bug + Frontend. task-b: Bug only. task-c: no labels.
	mustExec(t, conn, `INSERT INTO task_labels (task_id, label_id) VALUES ('task-a', 'label-bug')`)
	mustExec(t, conn, `INSERT INTO task_labels (task_id, label_id) VALUES ('task-a', 'label-fe')`)
	mustExec(t, conn, `INSERT INTO task_labels (task_id, label_id) VALUES ('task-b', 'label-bug')`)

	ids = struct{ org, proj, status, taskA, taskB, taskC, labelBug, labelFE string }{
		org, "proj-1", "status-1", "task-a", "task-b", "task-c", "label-bug", "label-fe",
	}
	return ctx, store, labelStore, conn, ids
}

func mustExec(t *testing.T, conn *sql.DB, query string, args ...any) {
	t.Helper()
	if _, err := conn.Exec(query, args...); err != nil {
		t.Fatalf("exec %q: %v", query, err)
	}
}

// Tasks returned by ListByProject must carry their resolved labels (no N+1).
func TestTaskStore_ListByProject_LoadsLabels(t *testing.T) {
	ctx, store, _, _, ids := setupTaskLabelTest(t)

	tasks, err := store.ListByProject(ctx, ids.org, ids.proj, domain.TaskFilter{})
	if err != nil {
		t.Fatalf("ListByProject: %v", err)
	}

	byID := map[string]*domain.Task{}
	for _, tk := range tasks {
		byID[tk.ID] = tk
	}

	if got := len(byID[ids.taskA].Labels); got != 2 {
		t.Errorf("task-a labels = %d, want 2", got)
	}
	if got := len(byID[ids.taskB].Labels); got != 1 {
		t.Errorf("task-b labels = %d, want 1", got)
	}
	if got := len(byID[ids.taskC].Labels); got != 0 {
		t.Errorf("task-c labels = %d, want 0", got)
	}

	// Names should be populated, not just IDs.
	names := map[string]bool{}
	for _, l := range byID[ids.taskA].Labels {
		names[l.Name] = true
	}
	if !names["Bug"] || !names["Frontend"] {
		t.Errorf("task-a label names = %v, want Bug+Frontend", names)
	}
}

// GetByID must also load labels for a single task.
func TestTaskStore_GetByID_LoadsLabels(t *testing.T) {
	ctx, store, _, _, ids := setupTaskLabelTest(t)

	task, err := store.GetByID(ctx, ids.org, ids.taskA, ids.proj)
	if err != nil {
		t.Fatalf("GetByID: %v", err)
	}
	if len(task.Labels) != 2 {
		t.Errorf("task-a labels = %d, want 2", len(task.Labels))
	}
}

// The label filter must narrow the result set to tasks carrying the label.
func TestTaskStore_ListByProject_LabelFilter(t *testing.T) {
	ctx, store, _, _, ids := setupTaskLabelTest(t)

	// Filter by "Frontend": only task-a matches.
	tasks, err := store.ListByProject(ctx, ids.org, ids.proj, domain.TaskFilter{
		LabelIDs: []string{ids.labelFE},
	})
	if err != nil {
		t.Fatalf("ListByProject with label filter: %v", err)
	}
	if len(tasks) != 1 || tasks[0].ID != ids.taskA {
		t.Errorf("label filter Frontend = %v, want [task-a]", taskIDs(tasks))
	}

	// Filter by "Bug": task-a and task-b match.
	tasks, err = store.ListByProject(ctx, ids.org, ids.proj, domain.TaskFilter{
		LabelIDs: []string{ids.labelBug},
	})
	if err != nil {
		t.Fatalf("ListByProject with label filter: %v", err)
	}
	if len(tasks) != 2 {
		t.Errorf("label filter Bug = %d tasks, want 2", len(tasks))
	}

	// Empty label filter returns all tasks (no accidental IN (NULL) false).
	tasks, err = store.ListByProject(ctx, ids.org, ids.proj, domain.TaskFilter{})
	if err != nil {
		t.Fatalf("ListByProject no filter: %v", err)
	}
	if len(tasks) != 3 {
		t.Errorf("no filter = %d tasks, want 3", len(tasks))
	}
}

func taskIDs(tasks []*domain.Task) []string {
	out := make([]string, len(tasks))
	for i, t := range tasks {
		out[i] = t.ID
	}
	return out
}

// TestTaskStore_ListByUser_FiltersByAssigneeAndLabel verifies that the
// cross-project "My Tasks" query respects both assignee_id and label_ids
// filters.
func TestTaskStore_ListByUser_FiltersByAssigneeAndLabel(t *testing.T) {
	ctx, store, _, conn, ids := setupTaskLabelTest(t)

	// Seed two assignees.
	mustExec(t, conn, `INSERT INTO accounts (id, email, password_hash) VALUES ('acct-2', 'bob@test.com', 'hash')`)
	mustExec(t, conn, `INSERT INTO users (id, account_id, org_id, name, email) VALUES ('user-2', 'acct-2', ?, 'Bob', 'bob@test.com')`, ids.org)
	mustExec(t, conn, `INSERT INTO task_assignees (task_id, user_id) VALUES ('task-a', 'user-1')`)
	mustExec(t, conn, `INSERT INTO task_assignees (task_id, user_id) VALUES ('task-b', 'user-2')`)

	// Default query (no assignee filter) returns tasks assigned to current user (user-1).
	res, err := store.ListByUser(ctx, ids.org, "user-1", domain.TaskListFilter{Limit: 20})
	if err != nil {
		t.Fatalf("ListByUser: %v", err)
	}
	if len(res.Items) != 1 || res.Items[0].ID != "task-a" {
		t.Fatalf("expected task-a for user-1, got %v", enrichedTaskIDs(res.Items))
	}

	// Explicit assignee filter for user-2.
	assigneeID := "user-2"
	res, err = store.ListByUser(ctx, ids.org, "user-1", domain.TaskListFilter{AssigneeID: &assigneeID, Limit: 20})
	if err != nil {
		t.Fatalf("ListByUser assignee filter: %v", err)
	}
	if len(res.Items) != 1 || res.Items[0].ID != "task-b" {
		t.Fatalf("expected task-b for user-2 filter, got %v", enrichedTaskIDs(res.Items))
	}

	// Label filter for 'label-bug' on current user's tasks.
	res, err = store.ListByUser(ctx, ids.org, "user-1", domain.TaskListFilter{LabelIDs: []string{"label-bug"}, Limit: 20})
	if err != nil {
		t.Fatalf("ListByUser label filter: %v", err)
	}
	if len(res.Items) != 1 || res.Items[0].ID != "task-a" {
		t.Fatalf("expected task-a with bug label, got %v", enrichedTaskIDs(res.Items))
	}

	// Combined filter: user-2 + label-bug => task-b only.
	res, err = store.ListByUser(ctx, ids.org, "user-1", domain.TaskListFilter{AssigneeID: &assigneeID, LabelIDs: []string{"label-bug"}, Limit: 20})
	if err != nil {
		t.Fatalf("ListByUser combined filter: %v", err)
	}
	if len(res.Items) != 1 || res.Items[0].ID != "task-b" {
		t.Fatalf("expected task-b with user-2+bug, got %v", enrichedTaskIDs(res.Items))
	}
}

func enrichedTaskIDs(tasks []*domain.EnrichedTask) []string {
	ids := make([]string, len(tasks))
	for i, t := range tasks {
		ids[i] = t.ID
	}
	return ids
}

// TestTaskStore_ListByUser_RequireProjectMembership verifies that
// non-elevated roles (viewer/guest) must not see tasks in projects they lack
// explicit membership in, while elevated roles (owner/admin/member) see all
// their assigned tasks regardless of project_members rows.
func TestTaskStore_ListByUser_RequireProjectMembership(t *testing.T) {
	ctx := context.Background()
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

	q := sqlc.New(conn)
	store := NewTaskStore(q, conn)

	org := "org-1"
	proj := "proj-1"
	statusID := "status-1"

	// Seed org, accounts, users, project, status.
	mustExec(t, conn, `INSERT INTO organizations (id, name, slug) VALUES (?, 'Test Org', 'test-org')`, org)
	mustExec(t, conn, `INSERT INTO accounts (id, email, password_hash) VALUES ('acct-1', 'admin@test.com', 'hash')`)
	mustExec(t, conn, `INSERT INTO accounts (id, email, password_hash) VALUES ('acct-2', 'viewer@test.com', 'hash')`)
	mustExec(t, conn, `INSERT INTO users (id, account_id, org_id, name, email, role) VALUES ('user-admin', 'acct-1', ?, 'Admin', 'admin@test.com', 'admin')`, org)
	mustExec(t, conn, `INSERT INTO users (id, account_id, org_id, name, email, role) VALUES ('user-viewer', 'acct-2', ?, 'Viewer', 'viewer@test.com', 'viewer')`, org)
	mustExec(t, conn, `INSERT INTO projects (id, org_id, name, slug, created_by) VALUES (?, ?, 'Project 1', 'proj-1', 'user-admin')`, proj, org)
	mustExec(t, conn, `INSERT INTO task_statuses (id, project_id, name, color, position) VALUES (?, ?, 'Todo', '#888', 0)`, statusID, proj)

	// Create a task assigned to the viewer.
	mustExec(t, conn,
		`INSERT INTO tasks (id, org_id, project_id, created_by, title, status_id, priority) VALUES ('task-1', ?, ?, 'user-admin', 'Viewer task', ?, 'none')`,
		org, proj, statusID,
	)
	mustExec(t, conn, `INSERT INTO task_assignees (task_id, user_id) VALUES ('task-1', 'user-viewer')`)

	// Create a task assigned to the admin.
	mustExec(t, conn,
		`INSERT INTO tasks (id, org_id, project_id, created_by, title, status_id, priority) VALUES ('task-2', ?, ?, 'user-admin', 'Admin task', ?, 'none')`,
		org, proj, statusID,
	)
	mustExec(t, conn, `INSERT INTO task_assignees (task_id, user_id) VALUES ('task-2', 'user-admin')`)

	// CASE 1: Viewer WITHOUT project_members row + RequireProjectMembership=true => 0 tasks.
	res, err := store.ListByUser(ctx, org, "user-viewer", domain.TaskListFilter{
		RequireProjectMembership: true,
		Limit:                    20,
		ShowCompleted:            true,
	})
	if err != nil {
		t.Fatalf("viewer no membership: %v", err)
	}
	if len(res.Items) != 0 {
		t.Fatalf("viewer without project_members saw %d tasks, expected 0", len(res.Items))
	}

	// CASE 2: Viewer WITH project_members row + RequireProjectMembership=true => 1 task.
	mustExec(t, conn, `INSERT INTO project_members (project_id, user_id, role) VALUES (?, 'user-viewer', 'viewer')`, proj)
	res, err = store.ListByUser(ctx, org, "user-viewer", domain.TaskListFilter{
		RequireProjectMembership: true,
		Limit:                    20,
		ShowCompleted:            true,
	})
	if err != nil {
		t.Fatalf("viewer with membership: %v", err)
	}
	if len(res.Items) != 1 || res.Items[0].ID != "task-1" {
		t.Fatalf("viewer with project_members saw %d tasks, expected 1 (task-1)", len(res.Items))
	}

	// CASE 3: Admin WITHOUT project_members row + RequireProjectMembership=false => sees their task.
	res, err = store.ListByUser(ctx, org, "user-admin", domain.TaskListFilter{
		RequireProjectMembership: false,
		Limit:                    20,
		ShowCompleted:            true,
	})
	if err != nil {
		t.Fatalf("admin no membership: %v", err)
	}
	if len(res.Items) != 1 || res.Items[0].ID != "task-2" {
		t.Fatalf("admin without project_members saw %d tasks, expected 1 (task-2)", len(res.Items))
	}
}

// TestTaskStore_ListByProject_SearchWildcardsNotInjected verifies that
// a search term containing LIKE wildcards ("%", "_") must NOT act as a
// wildcard. Previously the store built "%%<term>%%" and used LIKE, so
// searching for "%" matched every task. The queries now use instr() (literal
// substring), so "%" matches only tasks whose title literally contains "%".
func TestTaskStore_ListByProject_SearchWildcardsNotInjected(t *testing.T) {
	ctx, store, _, _, ids := setupTaskLabelTest(t)

	// Add a task whose title literally contains a percent sign.
	mustExec(t, store.db, `INSERT INTO tasks (id, org_id, project_id, created_by, title, status_id, priority) VALUES ('task-pct', ?, 'proj-1', 'user-1', 'Fix 100% bug', 'status-1', 'none')`, ids.org)

	// Searching for "%" should return ONLY the task whose title contains "%",
	// not all three tasks.
	tasks, err := store.ListByProject(ctx, ids.org, ids.proj, domain.TaskFilter{Search: "%"})
	if err != nil {
		t.Fatalf("ListByProject: %v", err)
	}
	if len(tasks) != 1 || tasks[0].ID != "task-pct" {
		var got []string
		for _, tk := range tasks {
			got = append(got, tk.ID)
		}
		t.Errorf("search '%%' returned %v, want only [task-pct]", got)
	}

	// Normal case-insensitive substring search still works.
	tasks, err = store.ListByProject(ctx, ids.org, ids.proj, domain.TaskFilter{Search: "TASK-A"})
	if err != nil {
		t.Fatalf("ListByProject case-insensitive: %v", err)
	}
	if len(tasks) != 1 || tasks[0].ID != ids.taskA {
		t.Errorf("case-insensitive search for 'TASK-A' returned %d tasks, want 1 (task-a)", len(tasks))
	}
}

func TestTaskStore_Update_PersistsUpdatedAt(t *testing.T) {
	ctx := context.Background()
	tmpDir := t.TempDir()
	conn, err := NewDB(filepath.Join(tmpDir, "test.db"))
	if err != nil {
		t.Fatalf("NewDB: %v", err)
	}
	t.Cleanup(func() { conn.Close() })
	if err := migration.RunMigrations(conn); err != nil {
		t.Fatalf("RunMigrations: %v", err)
	}
	q := sqlc.New(conn)
	store := NewTaskStore(q, conn)

	org := "org-upd-1"
	mustExec(t, conn, `INSERT INTO organizations (id, name, slug) VALUES (?, 'Upd Org', 'upd-org')`, org)
	mustExec(t, conn, `INSERT INTO accounts (id, email, password_hash) VALUES ('acct-upd', 'upd@test.com', 'hash')`)
	mustExec(t, conn, `INSERT INTO users (id, account_id, org_id, name, email) VALUES ('user-upd', 'acct-upd', ?, 'Updater', 'upd@test.com')`, org)
	mustExec(t, conn, `INSERT INTO projects (id, org_id, name, slug, created_by) VALUES ('proj-upd', ?, 'Upd Proj', 'upd-proj', 'user-upd')`, org)
	mustExec(t, conn, `INSERT INTO task_statuses (id, project_id, name, color, position) VALUES ('stat-upd', 'proj-upd', 'Todo', '#888', 0)`)
	mustExec(t, conn, `INSERT INTO tasks (id, org_id, project_id, created_by, title, status_id, priority) VALUES ('task-upd', ?, 'proj-upd', 'user-upd', 'Update test', 'stat-upd', 'none')`, org)

	// Set a fixed known time (no sub-second precision: matches formatTime).
	fixedTime := time.Date(2025, 01, 15, 10, 30, 0, 0, time.UTC)
	task, err := store.GetByID(ctx, org, "task-upd", "proj-upd")
	if err != nil {
		t.Fatalf("GetByID before update: %v", err)
	}
	task.UpdatedAt = fixedTime
	task.Description = "updated-desc"

	if err := store.Update(ctx, task); err != nil {
		t.Fatalf("Update: %v", err)
	}

	// Read back using raw query (avoids parseTime round-trip masking errors).
	var rawUpdatedAt string
	row := conn.QueryRowContext(ctx, `SELECT updated_at FROM tasks WHERE id = 'task-upd'`)
	if err := row.Scan(&rawUpdatedAt); err != nil {
		t.Fatalf("scan raw updated_at: %v", err)
	}

	expected := "2025-01-15 10:30:00"
	if rawUpdatedAt != expected {
		t.Errorf("updated_at = %q, want %q", rawUpdatedAt, expected)
	}

	// Also verify the updated task round-trips through GetByID correctly.
	refetched, err := store.GetByID(ctx, org, "task-upd", "proj-upd")
	if err != nil {
		t.Fatalf("GetByID after update: %v", err)
	}
	if !refetched.UpdatedAt.Equal(fixedTime) {
		t.Errorf("refetched UpdatedAt = %v, want %v", refetched.UpdatedAt, fixedTime)
	}
}

func TestTaskStore_Update_ZeroUpdatedAtDefaultsToNow(t *testing.T) {
	ctx := context.Background()
	tmpDir := t.TempDir()
	conn, err := NewDB(filepath.Join(tmpDir, "test.db"))
	if err != nil {
		t.Fatalf("NewDB: %v", err)
	}
	t.Cleanup(func() { conn.Close() })
	if err := migration.RunMigrations(conn); err != nil {
		t.Fatalf("RunMigrations: %v", err)
	}
	q := sqlc.New(conn)
	store := NewTaskStore(q, conn)

	org := "org-zero-1"
	mustExec(t, conn, `INSERT INTO organizations (id, name, slug) VALUES (?, 'Zero Org', 'zero-org')`, org)
	mustExec(t, conn, `INSERT INTO accounts (id, email, password_hash) VALUES ('acct-zero', 'zero@test.com', 'hash')`)
	mustExec(t, conn, `INSERT INTO users (id, account_id, org_id, name, email) VALUES ('user-zero', 'acct-zero', ?, 'Zero', 'zero@test.com')`, org)
	mustExec(t, conn, `INSERT INTO projects (id, org_id, name, slug, created_by) VALUES ('proj-zero', ?, 'Zero Proj', 'zero-proj', 'user-zero')`, org)
	mustExec(t, conn, `INSERT INTO task_statuses (id, project_id, name, color, position) VALUES ('stat-zero', 'proj-zero', 'Todo', '#888', 0)`)
	mustExec(t, conn, `INSERT INTO tasks (id, org_id, project_id, created_by, title, status_id, priority) VALUES ('task-zero', ?, 'proj-zero', 'user-zero', 'Zero upd test', 'stat-zero', 'none')`, org)

	task, err := store.GetByID(ctx, org, "task-zero", "proj-zero")
	if err != nil {
		t.Fatalf("GetByID before update: %v", err)
	}
	// Leave UpdatedAt as zero value: wrapper must default to time.Now().
	before := time.Now()
	task.Description = "zero-updated"
	if err := store.Update(ctx, task); err != nil {
		t.Fatalf("Update: %v", err)
	}
	after := time.Now()

	// Read back raw updated_at and verify it's between before and after.
	var rawUpdatedAt string
	row := conn.QueryRowContext(ctx, `SELECT updated_at FROM tasks WHERE id = 'task-zero'`)
	if err := row.Scan(&rawUpdatedAt); err != nil {
		t.Fatalf("scan raw updated_at: %v", err)
	}

	parsed, err := time.Parse("2006-01-02 15:04:05", rawUpdatedAt)
	if err != nil {
		t.Fatalf("parse raw updated_at %q: %v", rawUpdatedAt, err)
	}
	if parsed.Before(before.Truncate(time.Second)) || parsed.After(after.Add(time.Second)) {
		t.Errorf("updated_at = %v (parsed from %q), expected between %v and %v", parsed, rawUpdatedAt, before, after)
	}
}
