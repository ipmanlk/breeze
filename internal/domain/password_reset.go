package domain

import "time"

type PasswordReset struct {
	ID        string
	AccountID string
	TokenHash string
	ExpiresAt time.Time
	UsedAt    *time.Time
	CreatedAt time.Time
}
