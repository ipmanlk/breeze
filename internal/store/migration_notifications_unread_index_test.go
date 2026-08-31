package store

import (
	"path/filepath"
	"strings"
	"testing"

	"ipmanlk/plume/internal/store/migration"

	_ "modernc.org/sqlite"
)

// TestMigrations_NotificationsUnreadIndex verifies the squashed baseline
// (00001_initial.sql) defines idx_notifications_unread with the predicate that
// covers UNREAD notifications (WHERE is_read = 0).
//
// The hot-path queries: CountUnreadNotifications, the unread-only filter in
// ListNotifications, and MarkAllNotificationsRead: all filter on is_read = 0,
// so the partial index must be built on that predicate. A predicate of
// is_read = 1 would leave every unread query scanning the full per-user
// notification set.
func TestMigrations_NotificationsUnreadIndex(t *testing.T) {
	tmpDir := t.TempDir()
	conn, err := NewDB(filepath.Join(tmpDir, "test.db"))
	if err != nil {
		t.Fatalf("open db: %v", err)
	}
	t.Cleanup(func() { conn.Close() })

	if err := migration.RunMigrations(conn); err != nil {
		t.Fatalf("run migrations: %v", err)
	}

	var idxSQL string
	err = conn.QueryRow(
		`SELECT sql FROM sqlite_master WHERE type = 'index' AND name = 'idx_notifications_unread'`,
	).Scan(&idxSQL)
	if err != nil {
		t.Fatalf("idx_notifications_unread index missing: %v", err)
	}
	if !strings.Contains(idxSQL, "is_read = 0") {
		t.Errorf("index predicate should be 'is_read = 0' (unread), got: %s", idxSQL)
	}
	if strings.Contains(idxSQL, "is_read = 1") {
		t.Errorf("index predicate must NOT be 'is_read = 1' (read), got: %s", idxSQL)
	}
}
