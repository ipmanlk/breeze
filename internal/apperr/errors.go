package apperr

import (
	"errors"
	"fmt"
)

// Sentinel errors. Services and stores return these (optionally wrapping a
// cause via fmt.Errorf("...: %w", err)). The transport layer maps them to
// HTTP status codes via transport.RespondWithError.
var (
	ErrNotFound        = errors.New("not found")
	ErrAlreadyExists   = errors.New("already exists")
	ErrSetupComplete   = errors.New("setup already completed")
	ErrInvalidCreds    = errors.New("invalid email or password")
	ErrUserDeactivated = errors.New("user account is deactivated")
	ErrForbidden       = errors.New("forbidden")
	ErrForbiddenViewer = errors.New("viewers cannot perform this action")
	ErrSessionExpired  = errors.New("session expired")
	ErrSessionNotFound = errors.New("session not found")
	ErrConflict        = errors.New("conflict")
	ErrInvalidInput    = errors.New("invalid input")
	ErrInternal        = errors.New("internal error")
)

type ErrorResponse struct {
	Code    string `json:"code"`
	Message string `json:"message"`
}

// NotFound wraps err (when non-nil) with the resource name so logs show which
// lookup failed. When err is nil, returns ErrNotFound directly. The returned
// error always satisfies errors.Is(err, ErrNotFound) when err is nil or itself
// wraps ErrNotFound (the store layer maps sql.ErrNoRows → ErrNotFound).
func NotFound(resource string, err error) error {
	if err != nil {
		return fmt.Errorf("%s: %w", resource, err)
	}
	return ErrNotFound
}

// Forbidden wraps ErrForbidden with a contextual message. The message is
// surfaced in logs but not in the HTTP response (RespondWithError returns a
// generic "insufficient permissions" to avoid leaking authorization logic).
func Forbidden(msg string) error {
	return fmt.Errorf("%s: %w", msg, ErrForbidden)
}

// Internal wraps err (when non-nil) or ErrInternal with a contextual message.
// Used for unexpected failures where the caller can provide a human-readable
// description of what operation failed.
func Internal(msg string, err error) error {
	if err != nil {
		return fmt.Errorf("%s: %w", msg, err)
	}
	return fmt.Errorf("%s: %w", msg, ErrInternal)
}

// InvalidInput returns ErrInvalidInput wrapped with a contextual message.
// The message is safe to surface to API consumers (e.g. field names).
func InvalidInput(msg string) error {
	return fmt.Errorf("%s: %w", msg, ErrInvalidInput)
}

// Conflict returns ErrConflict wrapped with an optional contextual message.
func Conflict(msg string) error {
	if msg == "" {
		return ErrConflict
	}
	return fmt.Errorf("%s: %w", msg, ErrConflict)
}
