package domain

import "time"

type Organization struct {
	ID                      string
	Name                    string
	Slug                    string
	MessageEditWindowMinute int
	CreatedAt               time.Time
	UpdatedAt               time.Time
}

// Workspace is the switcher-list view of an account's membership in an org:
// the organization plus the account's role in it and whether the account is
// the owner (drives UI affordances like leaving/transferring later). See
// docs/api/workspaces.md. Role is the account's org role in THIS workspace,
// derived from the users (membership) row, not the request-context role
// (which carries the active workspace's role only).
type Workspace struct {
	Organization Organization
	Role         Role
	IsOwner      bool
}

// MembershipResult pairs an account with its memberships, returned by the auth
// service on login so the frontend can list/switch workspaces.
type MembershipResult struct {
	Account     *Account
	Memberships []*User
}
