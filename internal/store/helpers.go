package store

import (
	"database/sql"
	"errors"
	"log/slog"
	"time"

	"ipmanlk/breeze/internal/apperr"
)

// mapScanErr converts a sqlc scan/query error into a typed apperr value.
// sql.ErrNoRows is mapped to apperr.ErrNotFound so callers (services,
// handlers) never need to import database/sql to distinguish "missing row"
// from a real DB failure. All other errors pass through unchanged.
func mapScanErr(err error) error {
	if err == nil {
		return nil
	}
	if errors.Is(err, sql.ErrNoRows) {
		return apperr.ErrNotFound
	}
	return err
}

// timeLayouts lists the datetime formats the app may persist. Rows are
// written with formatTime / SQLite's datetime('now'), but hand-edited or
// imported databases could carry other shapes.
var timeLayouts = []string{
	"2006-01-02 15:04:05",
	time.RFC3339,
}

// parseTime parses a SQLite datetime string. On failure it logs and returns
// the zero time; callers treat that as "unset". The nullable variant
// parseTimePtr maps failures to nil instead.
func parseTime(s string) time.Time {
	for _, layout := range timeLayouts {
		if t, err := time.Parse(layout, s); err == nil {
			return t
		}
	}
	slog.Warn("store: failed to parse time", "value", s)
	return time.Time{}
}

// parseTimePtr parses a nullable SQLite datetime string. On parse failure it
// returns nil rather than a pointer to a zero time, which is safer for
// callers that distinguish nil from a real value.
func parseTimePtr(s *string) *time.Time {
	if s == nil || *s == "" {
		return nil
	}
	for _, layout := range timeLayouts {
		if t, err := time.Parse(layout, *s); err == nil {
			return &t
		}
	}
	slog.Warn("store: failed to parse nullable time", "value", *s)
	return nil
}

func formatTime(t time.Time) string {
	return t.UTC().Format("2006-01-02 15:04:05")
}

func formatTimePtr(t *time.Time) *string {
	if t == nil {
		return nil
	}
	s := t.UTC().Format("2006-01-02 15:04:05")
	return &s
}

func intPtr(v *int64) *int {
	if v == nil {
		return nil
	}
	x := int(*v)
	return &x
}

func int64Ptr(v *int) *int64 {
	if v == nil {
		return nil
	}
	x := int64(*v)
	return &x
}

// nilIfEmpty returns a pointer to s for non-empty strings, nil otherwise.
// Used to bind optional sqlc string params to nullable columns.
func nilIfEmpty(s string) *string {
	if s == "" {
		return nil
	}
	return &s
}

func boolToInt64(v bool) int64 {
	if v {
		return 1
	}
	return 0
}

// derefStr returns *s's value, or "" if nil. sqlc emits *string for nullable
// columns (e.g. users.account_id, added via ALTER TABLE). Domain types use
// plain string, so deref on read.
func derefStr(s *string) string {
	if s == nil {
		return ""
	}
	return *s
}
