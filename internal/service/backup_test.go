package service

import (
	"context"
	"io"
	"os"
	"path/filepath"
	"testing"

	"ipmanlk/breeze/internal/store"
	"log/slog"
)

// newTestBackupDB creates a fresh SQLite DB at a temp path with the
// organizations table so it passes the restore schema validation.
func newTestBackupDB(t *testing.T) string {
	t.Helper()
	path := filepath.Join(t.TempDir(), "test.db")
	db, err := store.NewDB(path)
	if err != nil {
		t.Fatalf("NewDB: %v", err)
	}
	if _, err := db.Exec("CREATE TABLE organizations (id TEXT PRIMARY KEY)"); err != nil {
		t.Fatalf("create table: %v", err)
	}
	if _, err := db.Exec("INSERT INTO organizations (id) VALUES ('org-1')"); err != nil {
		t.Fatalf("insert: %v", err)
	}
	db.Close()
	return path
}

func TestBackupService_DownloadBackup(t *testing.T) {
	srcPath := newTestBackupDB(t)
	db, err := store.NewDB(srcPath)
	if err != nil {
		t.Fatalf("NewDB: %v", err)
	}
	defer db.Close()

	svc := NewBackupService(db, srcPath, slog.Default())
	reader, filename, err := svc.DownloadBackup(context.Background())
	if err != nil {
		t.Fatalf("DownloadBackup: %v", err)
	}
	defer reader.Close()

	if filename == "" || filepath.Ext(filename) != ".db" {
		t.Errorf("filename = %q, want a .db filename", filename)
	}

	// The snapshot should be a valid SQLite DB with the organizations row.
	out, err := io.ReadAll(reader)
	if err != nil {
		t.Fatalf("ReadAll: %v", err)
	}
	if len(out) < minBackupSize {
		t.Errorf("snapshot size = %d, want >= %d", len(out), minBackupSize)
	}

	// Verify the temp file is removed after Close.
	tmpPath := reader.(*removeOnCloseReader).path
	reader.Close()
	if _, err := os.Stat(tmpPath); !os.IsNotExist(err) {
		t.Errorf("temp file not removed after close: %s", tmpPath)
	}
}

func TestBackupService_StageRestore_ValidBackup(t *testing.T) {
	srcPath := newTestBackupDB(t)
	db, err := store.NewDB(srcPath)
	if err != nil {
		t.Fatalf("NewDB: %v", err)
	}
	defer db.Close()

	// Create a valid backup snapshot to upload.
	svc := NewBackupService(db, srcPath, slog.Default())
	reader, _, err := svc.DownloadBackup(context.Background())
	if err != nil {
		t.Fatalf("DownloadBackup: %v", err)
	}
	defer reader.Close()

	pendingPath, err := svc.StageRestore(context.Background(), reader)
	if err != nil {
		t.Fatalf("StageRestore: %v", err)
	}
	if !svc.HasPendingRestore() {
		t.Error("HasPendingRestore = false, want true after staging")
	}
	if _, err := os.Stat(pendingPath); err != nil {
		t.Errorf("pending file missing: %v", err)
	}

	// Clean up.
	if err := svc.ClearPendingRestore(); err != nil {
		t.Fatalf("ClearPendingRestore: %v", err)
	}
	if svc.HasPendingRestore() {
		t.Error("HasPendingRestore = true after clear, want false")
	}
}

func TestBackupService_StageRestore_RejectsInvalidFile(t *testing.T) {
	srcPath := newTestBackupDB(t)
	db, err := store.NewDB(srcPath)
	if err != nil {
		t.Fatalf("NewDB: %v", err)
	}
	defer db.Close()

	svc := NewBackupService(db, srcPath, slog.Default())

	// A random small file that is not a SQLite DB.
	r := &bytesReader{b: []byte("not a database file")}
	if _, err := svc.StageRestore(context.Background(), r); err == nil {
		t.Fatal("StageRestore with invalid file: expected error, got nil")
	}
	if svc.HasPendingRestore() {
		t.Error("HasPendingRestore = true after failed stage, want false")
	}
}

// bytesReader is a minimal io.Reader for test data.
type bytesReader struct {
	b []byte
	i int
}

func (r *bytesReader) Read(p []byte) (int, error) {
	if r.i >= len(r.b) {
		return 0, io.EOF
	}
	n := copy(p, r.b[r.i:])
	r.i += n
	return n, nil
}
