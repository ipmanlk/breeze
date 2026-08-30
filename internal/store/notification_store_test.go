package store

import (
	"context"
	"path/filepath"
	"testing"
	"time"

	"ipmanlk/breeze/internal/domain"
	"ipmanlk/breeze/internal/store/migration"
	"ipmanlk/breeze/internal/store/sqlc"

	_ "modernc.org/sqlite"
)

func TestNotificationStore_Create_PersistsCreatedAt(t *testing.T) {
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
	store := NewNotificationStore(q)

	org := "org-notif-1"
	mustExec(t, conn, `INSERT INTO organizations (id, name, slug) VALUES (?, 'Notif Org', 'notif-org')`, org)
	mustExec(t, conn, `INSERT INTO accounts (id, email, password_hash) VALUES ('acct-notif', 'notif@test.com', 'hash')`)
	mustExec(t, conn, `INSERT INTO users (id, account_id, org_id, name, email) VALUES ('user-notif', 'acct-notif', ?, 'Notif Tester', 'notif@test.com')`, org)

	// Set a fixed time with no sub-second (matches formatTime precision).
	fixedTime := time.Date(2025, 6, 15, 14, 30, 0, 0, time.UTC)

	n := &domain.Notification{
		ID:         "notif-1",
		OrgID:      org,
		UserID:     "user-notif",
		Type:       domain.NotifTaskAssigned,
		Title:      "Test notification",
		Body:       "This is a test body",
		Link:       "/projects/p1?task=t1",
		EntityType: "task",
		EntityID:   "t1",
		ActorID:    "",
		CreatedAt:  fixedTime,
	}

	if err := store.Create(ctx, n); err != nil {
		t.Fatalf("Create: %v", err)
	}

	// Read back raw created_at column (avoids parseTime round-trip masking errors).
	var rawCreatedAt string
	row := conn.QueryRowContext(ctx, `SELECT created_at FROM notifications WHERE id = 'notif-1'`)
	if err := row.Scan(&rawCreatedAt); err != nil {
		t.Fatalf("scan raw created_at: %v", err)
	}

	expected := "2025-06-15 14:30:00"
	if rawCreatedAt != expected {
		t.Errorf("created_at = %q, want %q", rawCreatedAt, expected)
	}

	// Also verify round-trip through GetByID returns the correct time.
	refetched, err := store.GetByID(ctx, "notif-1", "user-notif")
	if err != nil {
		t.Fatalf("GetByID: %v", err)
	}
	if !refetched.CreatedAt.Equal(fixedTime) {
		t.Errorf("refetched CreatedAt = %v, want %v", refetched.CreatedAt, fixedTime)
	}
}

func TestNotificationStore_Create_ZeroCreatedAtDefaultsToNow(t *testing.T) {
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
	store := NewNotificationStore(q)

	org := "org-notif-2"
	mustExec(t, conn, `INSERT INTO organizations (id, name, slug) VALUES (?, 'Notif Org 2', 'notif-org-2')`, org)
	mustExec(t, conn, `INSERT INTO accounts (id, email, password_hash) VALUES ('acct-notif-2', 'notif2@test.com', 'hash')`)
	mustExec(t, conn, `INSERT INTO users (id, account_id, org_id, name, email) VALUES ('user-notif-2', 'acct-notif-2', ?, 'Notif Tester 2', 'notif2@test.com')`, org)

	// Leave CreatedAt as zero value: wrapper should default to ~time.Now().
	n := &domain.Notification{
		ID:         "notif-2",
		OrgID:      org,
		UserID:     "user-notif-2",
		Type:       domain.NotifTaskAssigned,
		Title:      "Default time test",
		Body:       "Body",
		Link:       "",
		EntityType: "task",
		EntityID:   "t2",
		ActorID:    "",
	}

	if err := store.Create(ctx, n); err != nil {
		t.Fatalf("Create: %v", err)
	}

	// Read back raw created_at.
	var rawCreatedAt string
	row := conn.QueryRowContext(ctx, `SELECT created_at FROM notifications WHERE id = 'notif-2'`)
	if err := row.Scan(&rawCreatedAt); err != nil {
		t.Fatalf("scan raw created_at: %v", err)
	}

	// The format is "2006-01-02 15:04:05": we can't assert exact time,
	// but we can check it's reasonably close to now.
	parsed, err := time.Parse("2006-01-02 15:04:05", rawCreatedAt)
	if err != nil {
		t.Fatalf("parse raw created_at %q: %v", rawCreatedAt, err)
	}
	if time.Since(parsed) > 30*time.Second {
		t.Errorf("created_at = %v, which is more than 30s from now", parsed)
	}

	// Also verify it round-trips through GetByID correctly.
	refetched, err := store.GetByID(ctx, "notif-2", "user-notif-2")
	if err != nil {
		t.Fatalf("GetByID: %v", err)
	}
	if refetched.CreatedAt.IsZero() {
		t.Error("refetched CreatedAt is zero: expected a non-zero time")
	}
}
