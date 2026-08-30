package store

import (
	"database/sql"
	"fmt"
	"strings"

	_ "modernc.org/sqlite"
)

func NewDB(dbPath string) (*sql.DB, error) {
	// PRAGMAs that must hold for every connection are passed as DSN
	// parameters so the driver applies them in its connect hook; including
	// on any replacement connection the pool dials later. Setting them once
	// via db.Exec would silently stop applying if that connection were ever
	// recreated (e.g. after a driver-level error), leaving a fresh
	// connection with foreign_keys OFF.
	var dsn strings.Builder
	dsn.WriteString(dbPath)
	dsn.WriteString("?_pragma=busy_timeout(5000)")
	dsn.WriteString("&_pragma=foreign_keys(1)")
	dsn.WriteString("&_pragma=cache_size(-20000)")
	dsn.WriteString("&_pragma=synchronous(NORMAL)")
	dsn.WriteString("&_pragma=journal_mode(WAL)")

	db, err := sql.Open("sqlite", dsn.String())
	if err != nil {
		return nil, fmt.Errorf("open db: %w", err)
	}

	db.SetMaxOpenConns(1)
	db.SetMaxIdleConns(1)
	db.SetConnMaxLifetime(0)

	return db, nil
}
