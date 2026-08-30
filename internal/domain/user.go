package domain

import (
	"strings"
	"time"
)

// NormalizeEmail trims and lowercases an email so account-identity lookups
// are case-insensitive. Every service entry point that uses an email as a
// login/invite key must pass it through this before storage or lookup.
func NormalizeEmail(email string) string {
	return strings.ToLower(strings.TrimSpace(email))
}

type Role string

const (
	RoleOwner  Role = "owner"
	RoleAdmin  Role = "admin"
	RoleMember Role = "member"
	RoleViewer Role = "viewer"
	RoleGuest  Role = "guest"
)

// RoleEveryone is the pseudo-role used in channel permission rules to mean
// "any authenticated user, regardless of org role." It is never stored as a
// user's Role; it only appears in category_permissions and
// channel_permission_overrides as a fallback rule that applies to everyone.
const RoleEveryone Role = "everyone"

// IsOrgElevatedRole reports whether an org role carries implicit access to
// every project within its org (owner/admin/member). Viewers and guests are
// project-scoped: they only access projects they have an explicit
// project_members row in. This is the canonical definition used both by the
// project access resolver and by search scoping.
func IsOrgElevatedRole(role Role) bool {
	return role == RoleOwner || role == RoleAdmin || role == RoleMember
}

// Account is the global credential + login-key record. One account per person
// (unique email); it owns the password hash. A user (membership) links an
// account to an organization with a role. See docs/api/workspaces.md.
type Account struct {
	ID           string
	Email        string
	PasswordHash string
	CreatedAt    time.Time
	UpdatedAt    time.Time
}

// User is a membership: an account's role within one organization. Display
// columns (Email, Name, AvatarURL) are denormalized copies of the account's
// identity so existing display/join queries are untouched. The credential
// (PasswordHash) lives on Account, not here.
type User struct {
	ID        string
	AccountID string
	OrgID     string
	Email     string
	Name      string
	Role      Role
	AvatarURL *string
	IsActive  bool
	CreatedAt time.Time
	UpdatedAt time.Time
}

type UserFilter struct {
	Cursor          string
	Search          string
	Limit           int
	Role            string
	IncludeInactive bool
}

type UserListResult struct {
	Users      []*User
	NextCursor string
	HasMore    bool
}

// ProjectMemberListResult is the paginated result of listing a project's
// members. Each item pairs a user (carrying the org role) with the user's
// project membership role, so callers can derive the effective role.
type ProjectMemberListResult struct {
	Members    []*ProjectMember
	NextCursor string
	HasMore    bool
}
