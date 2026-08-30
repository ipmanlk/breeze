package domain

import "time"

type Session struct {
	ID        string
	UserID    string
	OrgID     string
	Role      Role
	ExpiresAt time.Time
	RevokedAt *time.Time
	CreatedAt time.Time

	// Client context captured at login so the Sessions settings page can
	// render a recognizable device/browser label.
	UserAgent string
	IPAddress string
}

func (s *Session) IsRevoked() bool {
	return s.RevokedAt != nil
}

// LoginParams carries credentials plus the client context captured at login
// so a session row can record the browser/agent that created it.
type LoginParams struct {
	Email     string
	Password  string
	UserAgent string
	IPAddress string
}
