package dto

import (
	"time"

	"ipmanlk/plume/internal/domain"
)

// SessionResponse describes a session row for the Sessions settings page.
type SessionResponse struct {
	ID        string `json:"id"`
	UserAgent string `json:"user_agent"`
	IPAddress string `json:"ip_address"`
	Role      string `json:"role"`
	ExpiresAt string `json:"expires_at"`
	RevokedAt string `json:"revoked_at,omitempty"`
	CreatedAt string `json:"created_at"`
	// IsCurrent is true for the session this request was authenticated with.
	// The frontend can't read the session ID from the HttpOnly JWT, so the
	// server flags it here.
	IsCurrent bool `json:"is_current"`
}

func NewSessionResponse(s *domain.Session) *SessionResponse {
	r := &SessionResponse{
		ID:        s.ID,
		UserAgent: s.UserAgent,
		IPAddress: s.IPAddress,
		Role:      string(s.Role),
		ExpiresAt: s.ExpiresAt.Format(time.RFC3339),
		CreatedAt: s.CreatedAt.Format(time.RFC3339),
	}
	if s.RevokedAt != nil {
		r.RevokedAt = s.RevokedAt.Format(time.RFC3339)
	}
	return r
}
