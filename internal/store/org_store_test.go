package store

import (
	"context"
	"database/sql"
	"path/filepath"
	"testing"

	"ipmanlk/plume/internal/domain"
	"ipmanlk/plume/internal/store/migration"
	"ipmanlk/plume/internal/store/sqlc"

	_ "modernc.org/sqlite"
)

// setupOrgStoreTest boots a fresh SQLite DB with migrations for org store tests.
func setupOrgStoreTest(t *testing.T) (context.Context, *OrgStore, *sql.DB, *sqlc.Queries) {
	t.Helper()
	ctx := context.Background()
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
	store := NewOrgStore(q, conn)
	return ctx, store, conn, q
}

// TestOrgStore_CreateOrgWithAccountAndUser_RollbackOnFailure proves that when
// the account-insert step fails (duplicate email), the previously-inserted
// org is rolled back and NOT persisted to the database.
func TestOrgStore_CreateOrgWithAccountAndUser_RollbackOnFailure(t *testing.T) {
	ctx, store, conn, q := setupOrgStoreTest(t)
	dupEmail := "dup@test.com"

	// Pre-insert an account with the email we'll re-use to force a UNIQUE
	// constraint violation on accounts.email.
	mustExec(t, conn, `INSERT INTO accounts (id, email, password_hash) VALUES ('existing-acct', ?, 'hash')`, dupEmail)

	// Attempt to create a new org+account+user with the SAME email.
	org := &domain.Organization{
		ID:   "new-org",
		Name: "New Org",
		Slug: "new-org",
	}
	err := store.CreateOrgWithAccountAndUser(ctx, org, "new-acct", "new-user", "hash", dupEmail, "Admin")
	if err == nil {
		t.Fatal("expected error (duplicate email), got nil")
	}

	// Assert the NEW org was NOT persisted (transaction rolled back).
	_, err = q.GetOrganizationByID(ctx, "new-org")
	if err != sql.ErrNoRows {
		t.Fatalf("expected sql.ErrNoRows for rolled-back org, got err=%v", err)
	}

	// Assert the pre-existing account is still present.
	acct, err := q.GetAccountByEmail(ctx, dupEmail)
	if err != nil {
		t.Fatalf("pre-existing account should remain: %v", err)
	}
	if acct.ID != "existing-acct" {
		t.Errorf("account ID = %q, want %q", acct.ID, "existing-acct")
	}
}

// TestOrgStore_CreateOrgWithUser_RollbackOnFailure proves that when the
// user-insert step fails (duplicate primary key), the previously-inserted
// org is rolled back and NOT persisted.
func TestOrgStore_CreateOrgWithUser_RollbackOnFailure(t *testing.T) {
	ctx, store, conn, q := setupOrgStoreTest(t)

	// Pre-insert an organization and a user. We'll use a duplicate user ID
	// to force a PRIMARY KEY violation on users.id inside the transaction.
	mustExec(t, conn, `INSERT INTO organizations (id, name, slug) VALUES ('existing-org', 'Existing', 'existing-org')`)
	mustExec(t, conn, `INSERT INTO accounts (id, email, password_hash) VALUES ('acct-1', 'a@test.com', 'hash')`)
	mustExec(t, conn, `INSERT INTO users (id, account_id, org_id, name, email) VALUES ('dup-user', 'acct-1', 'existing-org', 'Dup', 'a@test.com')`)

	// Attempt to create a new org+user with the pre-existing user ID.
	org := &domain.Organization{
		ID:   "new-org",
		Name: "New Org",
		Slug: "new-org",
	}
	avatarURL := ""
	err := store.CreateOrgWithUser(ctx, org, "dup-user", "acct-1", "Admin", "admin@test.com", &avatarURL)
	if err == nil {
		t.Fatal("expected error (duplicate user ID), got nil")
	}

	// Assert the NEW org was NOT persisted (transaction rolled back).
	_, err = q.GetOrganizationByID(ctx, "new-org")
	if err != sql.ErrNoRows {
		t.Fatalf("expected sql.ErrNoRows for rolled-back org, got err=%v", err)
	}

	// Assert the existing org is still present.
	existing, err := q.GetOrganizationByID(ctx, "existing-org")
	if err != nil {
		t.Fatalf("existing org should remain: %v", err)
	}
	if existing.Name != "Existing" {
		t.Errorf("existing org name = %q, want %q", existing.Name, "Existing")
	}
}
