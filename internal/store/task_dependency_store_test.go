package store

import (
	"context"
	"testing"

	"ipmanlk/breeze/internal/domain"
	"ipmanlk/breeze/internal/store/migration"
	"ipmanlk/breeze/internal/store/sqlc"

	_ "modernc.org/sqlite"
)

func setupDependencyTest(t *testing.T) (context.Context, *TaskDependencyStore, *TaskStore, string) {
	t.Helper()
	ctx := context.Background()
	tmpDir := t.TempDir()
	conn, err := NewDB(tmpDir + "/test.db")
	if err != nil {
		t.Fatalf("open db: %v", err)
	}
	t.Cleanup(func() { conn.Close() })
	if err := migration.RunMigrations(conn); err != nil {
		t.Fatalf("migrations: %v", err)
	}
	q := sqlc.New(conn)
	mustExec(t, conn, `INSERT INTO organizations (id, name, slug) VALUES ('org-1', 'O', 'o')`)
	mustExec(t, conn, `INSERT INTO accounts (id, email, password_hash) VALUES ('acct-1', 'a@t.com', 'h')`)
	mustExec(t, conn, `INSERT INTO users (id, account_id, org_id, name, email) VALUES ('user-1', 'acct-1', 'org-1', 'A', 'a@t.com')`)
	mustExec(t, conn, `INSERT INTO projects (id, org_id, name, slug, created_by) VALUES ('proj-1', 'org-1', 'P', 'p', 'user-1')`)
	mustExec(t, conn, `INSERT INTO task_statuses (id, project_id, name, color, position) VALUES ('status-1', 'proj-1', 'Todo', '#888', 0)`)
	for _, tid := range []string{"t-a", "t-b", "t-c"} {
		mustExec(t, conn,
			`INSERT INTO tasks (id, org_id, project_id, created_by, title, status_id, priority) VALUES (?, 'org-1', 'proj-1', 'user-1', ?, 'status-1', 'none')`,
			tid, tid,
		)
	}
	return ctx, NewTaskDependencyStore(q), NewTaskStore(q, conn), "org-1"
}

func TestTaskDependencyStore_AddAndList(t *testing.T) {
	ctx, depStore, _, _ := setupDependencyTest(t)

	// t-a is blocked by t-b and t-c.
	if err := depStore.Add(ctx, "t-a", "t-b"); err != nil {
		t.Fatalf("Add t-a<-t-b: %v", err)
	}
	if err := depStore.Add(ctx, "t-a", "t-c"); err != nil {
		t.Fatalf("Add t-a<-t-c: %v", err)
	}

	blocking, err := depStore.ListBlocking(ctx, "t-a")
	if err != nil {
		t.Fatalf("ListBlocking: %v", err)
	}
	if len(blocking) != 2 {
		t.Fatalf("blocking = %d, want 2", len(blocking))
	}

	blocked, err := depStore.ListBlocked(ctx, "t-b")
	if err != nil {
		t.Fatalf("ListBlocked: %v", err)
	}
	if len(blocked) != 1 || blocked[0].ID != "t-a" {
		t.Errorf("blocked by t-b = %v, want [t-a]", taskIDs(blocked))
	}
}

func TestTaskDependencyStore_SelfReferenceRejectedByDB(t *testing.T) {
	ctx, depStore, _, _ := setupDependencyTest(t)
	// The CHECK constraint should reject a self-edge.
	if err := depStore.Add(ctx, "t-a", "t-a"); err == nil {
		t.Fatal("self-dependency was allowed by the DB")
	}
}

func TestTaskDependencyStore_Remove(t *testing.T) {
	ctx, depStore, _, _ := setupDependencyTest(t)
	_ = depStore.Add(ctx, "t-a", "t-b")
	if err := depStore.Remove(ctx, "t-a", "t-b"); err != nil {
		t.Fatalf("Remove: %v", err)
	}
	blocking, _ := depStore.ListBlocking(ctx, "t-a")
	if len(blocking) != 0 {
		t.Errorf("after remove, blocking = %d, want 0", len(blocking))
	}
}

var _ = domain.Task{}
