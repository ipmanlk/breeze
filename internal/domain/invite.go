package domain

import "time"

type UserInvite struct {
	ID        string
	OrgID     string
	Email     *string
	Role      Role
	TokenHash string
	InvitedBy string
	MaxUses   *int
	UseCount  int
	ExpiresAt time.Time
	CreatedAt time.Time
}

type InviteProjectAssignment struct {
	ProjectID string
	Role      Role
}

type CreateInviteParams struct {
	OrgID              string
	InvitedBy          string
	Role               Role
	Email              *string
	ExpiresAt          time.Time
	ProjectAssignments []InviteProjectAssignment
}

type AcceptInviteParams struct {
	Token    string
	Name     string
	Email    string
	Password string
}

func (u *UserInvite) IsExpired() bool {
	return time.Now().UTC().After(u.ExpiresAt)
}

func (u *UserInvite) HasUsesRemaining() bool {
	if u.MaxUses == nil {
		return true
	}
	return u.UseCount < *u.MaxUses
}
