package service

import (
	"context"
	"database/sql"
	"fmt"
	"io"
	"log/slog"
	"os"
	"path/filepath"
	"time"

	"github.com/google/uuid"
	"ipmanlk/plume/internal/apperr"
	"ipmanlk/plume/internal/port"
	"ipmanlk/plume/internal/store"
)

// File suffixes used by the staged-restore flow.
const (
	restorePendingSuffix = ".restore-pending"
	restoreBackupSuffix  = ".bak"
	minBackupSize        = 4096 // SQLite header is 100 bytes; 4KB is a sane floor
)

// backupService implements port.BackupService. Backup uses VACUUM INTO for an
// atomic snapshot; restore validates an uploaded file, stages it to
// <dbPath>.restore-pending, and the swap is performed at server startup
// (before any DB connection is opened) by CompletePendingRestore.
type backupService struct {
	db     *sql.DB
	dbPath string
	log    *slog.Logger
}

func NewBackupService(db *sql.DB, dbPath string, log *slog.Logger) port.BackupService {
	return &backupService{db: db, dbPath: dbPath, log: log}
}

var _ port.BackupService = (*backupService)(nil)

// DownloadBackup creates a VACUUM INTO snapshot of the live database into a
// temp file and returns a reader over it. The reader removes the temp file
// when closed.
func (s *backupService) DownloadBackup(ctx context.Context) (io.ReadCloser, string, error) {
	filename := "plume-backup-" + time.Now().UTC().Format("20060102T150405Z") + ".db"
	tmpPath := filepath.Join(os.TempDir(), "plume-backup-"+uuid.New().String()+".db")

	// VACUUM INTO creates a consistent snapshot with the WAL fully
	// checkpointed, without closing the live connection.
	if _, err := s.db.ExecContext(ctx, "VACUUM INTO ?", tmpPath); err != nil {
		os.Remove(tmpPath)
		return nil, "", fmt.Errorf("vacuum into: %w", err)
	}

	f, err := os.Open(tmpPath)
	if err != nil {
		os.Remove(tmpPath)
		return nil, "", fmt.Errorf("open snapshot: %w", err)
	}
	return &removeOnCloseReader{f: f, path: tmpPath}, filename, nil
}

// StageRestore reads an uploaded backup, validates it is a SQLite DB with the
// expected schema, and writes it to <dbPath>.restore-pending. The server must
// be restarted to apply the restore (the swap happens at startup).
func (s *backupService) StageRestore(ctx context.Context, reader io.Reader) (string, error) {
	dir := filepath.Dir(s.dbPath)
	stagingPath := filepath.Join(dir, ".restore-staging-"+uuid.New().String())
	// Always clean up the staging temp; the final pending file is a copy.
	defer os.Remove(stagingPath)

	// Stream the upload to a temp file (avoid loading the whole DB into memory).
	f, err := os.Create(stagingPath)
	if err != nil {
		return "", fmt.Errorf("create staging file: %w", err)
	}
	if _, err := io.Copy(f, reader); err != nil {
		f.Close()
		return "", fmt.Errorf("write staging file: %w", err)
	}
	if err := f.Close(); err != nil {
		return "", fmt.Errorf("close staging file: %w", err)
	}

	fi, err := os.Stat(stagingPath)
	if err != nil {
		return "", fmt.Errorf("stat staging file: %w", err)
	}
	if fi.Size() < minBackupSize {
		return "", apperr.InvalidInput("backup file is too small to be a valid database")
	}

	// Validate the uploaded file is a real SQLite DB with the expected schema.
	if err := validateBackup(ctx, stagingPath); err != nil {
		return "", err
	}

	pendingPath := s.dbPath + restorePendingSuffix
	// Copy (not rename) so a failure between here and the swap leaves the
	// live DB untouched. os.Rename would also work but copy is explicit.
	if err := copyFile(stagingPath, pendingPath); err != nil {
		return "", fmt.Errorf("stage restore file: %w", err)
	}
	s.log.Info("backup staged for restore", "path", pendingPath, "size", fi.Size())
	return pendingPath, nil
}

// validateBackup opens the file as SQLite, runs a quick integrity check, and
// confirms the expected schema (organizations table exists).
func validateBackup(ctx context.Context, path string) error {
	tmpDB, err := store.NewDB(path)
	if err != nil {
		return apperr.InvalidInput("not a valid SQLite database file")
	}
	defer tmpDB.Close()

	var check string
	if err := tmpDB.QueryRowContext(ctx, "PRAGMA quick_check").Scan(&check); err != nil {
		return apperr.InvalidInput("database integrity check failed")
	}
	if check != "ok" {
		return apperr.InvalidInput("database integrity check failed: " + check)
	}

	// Confirm the schema has the organizations table (rejects random files
	// that happen to be valid SQLite but aren't Plume backups).
	var name string
	err = tmpDB.QueryRowContext(ctx,
		"SELECT name FROM sqlite_master WHERE type='table' AND name='organizations' LIMIT 1",
	).Scan(&name)
	if err != nil {
		return apperr.InvalidInput("unexpected schema; not a Plume database (missing organizations table)")
	}
	return nil
}

func (s *backupService) HasPendingRestore() bool {
	_, _, ok := s.PendingRestoreInfo()
	return ok
}

func (s *backupService) PendingRestoreInfo() (string, int64, bool) {
	p := s.dbPath + restorePendingSuffix
	fi, err := os.Stat(p)
	if err != nil {
		return "", 0, false
	}
	return p, fi.Size(), true
}

func (s *backupService) ClearPendingRestore() error {
	p := s.dbPath + restorePendingSuffix
	if err := os.Remove(p); err != nil && !os.IsNotExist(err) {
		return fmt.Errorf("remove pending restore: %w", err)
	}
	return nil
}

// CompletePendingRestore is NOT called by the service at runtime; the startup
// flow in app.go performs the swap directly via os.Rename before any DB
// connection is opened. This method exists for completeness/tests; it assumes
// the live DB connection is already closed.
func (s *backupService) CompletePendingRestore() (string, error) {
	pending := s.dbPath + restorePendingSuffix
	if _, err := os.Stat(pending); err != nil {
		return "", apperr.NotFound("pending restore", err)
	}
	bak := s.dbPath + restoreBackupSuffix
	// Back up the current DB (if it exists) before swapping.
	if _, err := os.Stat(s.dbPath); err == nil {
		os.Remove(bak)
		if err := os.Rename(s.dbPath, bak); err != nil {
			return "", fmt.Errorf("back up current db: %w", err)
		}
	}
	if err := os.Rename(pending, s.dbPath); err != nil {
		return "", fmt.Errorf("apply staged restore: %w", err)
	}
	return bak, nil
}

// removeOnCloseReader wraps an *os.File, deleting the underlying file when
// Close is called. Used for the VACUUM INTO snapshot temp file.
type removeOnCloseReader struct {
	f    *os.File
	path string
}

func (r *removeOnCloseReader) Read(p []byte) (int, error) { return r.f.Read(p) }
func (r *removeOnCloseReader) Close() error {
	err := r.f.Close()
	os.Remove(r.path)
	return err
}

// copyFile copies src to dst, truncating dst if it exists.
func copyFile(src, dst string) error {
	in, err := os.Open(src)
	if err != nil {
		return err
	}
	defer in.Close()
	out, err := os.Create(dst)
	if err != nil {
		return err
	}
	defer out.Close()
	if _, err := io.Copy(out, in); err != nil {
		return err
	}
	return out.Sync()
}
