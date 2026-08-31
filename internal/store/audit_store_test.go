package store

import (
	"context"
	"database/sql"
	"testing"

	"ipmanlk/plume/internal/domain"
	"ipmanlk/plume/internal/store/migration"
	"ipmanlk/plume/internal/store/sqlc"

	_ "modernc.org/sqlite"
)

func setupAuditTest(t *testing.T) (context.Context, *AuditStore, *sql.DB, string) {
	t.Helper()
	ctx := context.Background()
	conn, err := NewDB(t.TempDir() + "/test.db")
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
	mustExec(t, conn, `INSERT INTO users (id, account_id, org_id, name, email) VALUES ('user-1', 'acct-1', 'org-1', 'Alice', 'a@t.com')`)
	return ctx, NewAuditStore(q), conn, "org-1"
}

func TestAuditStore_CreateAndList(t *testing.T) {
	ctx, store, _, orgID := setupAuditTest(t)

	for i := 0; i < 3; i++ {
		if err := store.Create(ctx, &domain.AuditEntry{
			ID:         "a-" + string(rune('1'+i)),
			OrgID:      orgID,
			ActorID:    "user-1",
			Action:     domain.AuditActionRoleChanged,
			EntityType: "user",
			EntityID:   "target-" + string(rune('1'+i)),
			Metadata:   `{"new_role":"admin"}`,
		}); err != nil {
			t.Fatalf("Create[%d]: %v", i, err)
		}
	}

	entries, err := store.List(ctx, orgID, 50, 0, nil, nil)
	if err != nil {
		t.Fatalf("List: %v", err)
	}
	if len(entries) != 3 {
		t.Fatalf("List returned %d entries, want 3", len(entries))
	}
	// The actor name should be joined in.
	if entries[0].ActorName != "Alice" {
		t.Errorf("actor name = %q, want Alice", entries[0].ActorName)
	}
	// All three target IDs should be present (order is non-deterministic when
	// entries share a created_at second).
	seen := make(map[string]bool)
	for _, e := range entries {
		seen[e.EntityID] = true
	}
	for _, want := range []string{"target-1", "target-2", "target-3"} {
		if !seen[want] {
			t.Errorf("missing entry %q in list", want)
		}
	}

	total, err := store.Count(ctx, orgID, nil, nil)
	if err != nil {
		t.Fatalf("Count: %v", err)
	}
	if total != 3 {
		t.Errorf("Count = %d, want 3", total)
	}
}

func TestAuditStore_ListIsOrgScoped(t *testing.T) {
	ctx, store, conn, orgID := setupAuditTest(t)
	// Seed a second org + user so we can insert a foreign-org entry.
	mustExec(t, conn, `INSERT INTO organizations (id, name, slug) VALUES ('org-2', 'O2', 'o2')`)
	mustExec(t, conn, `INSERT INTO users (id, account_id, org_id, name, email) VALUES ('user-2', 'acct-1', 'org-2', 'Bob', 'b@t.com')`)
	_ = store.Create(ctx, &domain.AuditEntry{ID: "a-1", OrgID: orgID, ActorID: "user-1", Action: domain.AuditActionProjectDeleted, EntityType: "project", EntityID: "p-1"})
	_ = store.Create(ctx, &domain.AuditEntry{ID: "a-2", OrgID: "org-2", ActorID: "user-2", Action: domain.AuditActionProjectDeleted, EntityType: "project", EntityID: "p-2"})

	entries, _ := store.List(ctx, orgID, 50, 0, nil, nil)
	if len(entries) != 1 {
		t.Fatalf("org-1 list = %d, want 1 (org-scoped)", len(entries))
	}
	if entries[0].EntityID != "p-1" {
		t.Errorf("org-1 entry = %q, want p-1", entries[0].EntityID)
	}
}
