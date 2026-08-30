package domain

import "time"

// AuditAction enumerates the admin actions recorded in the audit log. Keeping
// these as constants (rather than free strings) makes the call sites
// grep-able and prevents typos from creating silent gaps in the log.
type AuditAction string

const (
	AuditActionRoleChanged       AuditAction = "role_changed"
	AuditActionUserActivated     AuditAction = "user_activated"
	AuditActionUserDeactivated   AuditAction = "user_deactivated"
	AuditActionMemberRemoved     AuditAction = "member_removed"
	AuditActionMemberRoleChanged AuditAction = "member_role_changed"
	AuditActionOrgDeleted        AuditAction = "org_deleted"
	AuditActionProjectDeleted    AuditAction = "project_deleted"
	AuditActionInviteRevoked     AuditAction = "invite_revoked"
	// Task-level events, extending the audit log with a unified trail.
	AuditActionTaskCreated AuditAction = "task_created"
	AuditActionTaskDeleted AuditAction = "task_deleted"
)

// AuditEntry is a single auditable admin action. Metadata is a JSON string
// carrying action-specific detail (e.g. {"old_role":"member","new_role":"admin"}).
type AuditEntry struct {
	ID         string
	OrgID      string
	ActorID    string
	Action     AuditAction
	EntityType string
	EntityID   string
	Metadata   string
	CreatedAt  time.Time

	// Joined actor display fields (populated by List, not by Record).
	ActorName  string
	ActorEmail string
}
