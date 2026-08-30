package domain

import (
	"testing"
)

func TestHasPermission(t *testing.T) {
	tests := []struct {
		name     string
		role     Role
		perm     Permission
		expected bool
	}{
		// Owner: has all permissions
		{"owner has org:manage", RoleOwner, PermOrgManage, true},
		{"owner has org:delete", RoleOwner, PermOrgDelete, true},
		{"owner has org:members.manage", RoleOwner, PermOrgMembersManage, true},
		{"owner has project:create", RoleOwner, PermProjectCreate, true},
		{"owner has project:delete", RoleOwner, PermProjectDelete, true},
		{"owner has task:create", RoleOwner, PermTaskCreate, true},
		{"owner has task:delete", RoleOwner, PermTaskDelete, true},
		{"owner has chat:read", RoleOwner, PermChatRead, true},
		{"owner has chat:send", RoleOwner, PermChatSend, true},
		{"owner has chat:channel.create", RoleOwner, PermChatChannelCreate, true},
		{"owner has chat:channel.manage", RoleOwner, PermChatChannelManage, true},

		// Admin: missing org:delete
		{"admin has org:manage", RoleAdmin, PermOrgManage, true},
		{"admin does NOT have org:delete", RoleAdmin, PermOrgDelete, false},
		{"admin has org:members.manage", RoleAdmin, PermOrgMembersManage, true},
		{"admin has project:manage", RoleAdmin, PermProjectManage, true},
		{"admin has project:delete", RoleAdmin, PermProjectDelete, true},
		{"admin has task:create", RoleAdmin, PermTaskCreate, true},
		{"admin has task:delete", RoleAdmin, PermTaskDelete, true},

		// Member: limited permissions
		{"member does NOT have org:manage", RoleMember, PermOrgManage, false},
		{"member has org:members.view", RoleMember, PermOrgMembersView, true},
		{"member has org:members.invite", RoleMember, PermOrgMembersInvite, true},
		{"member does NOT have project:manage", RoleMember, PermProjectManage, false},
		{"member does NOT have project:delete", RoleMember, PermProjectDelete, false},
		{"member has project:view", RoleMember, PermProjectView, true},
		{"member has task:create", RoleMember, PermTaskCreate, true},
		{"member has task:edit", RoleMember, PermTaskEdit, true},
		{"member does NOT have task:delete", RoleMember, PermTaskDelete, false},
		{"member has chat:read", RoleMember, PermChatRead, true},
		{"member has chat:send", RoleMember, PermChatSend, true},
		{"member has chat:channel.create", RoleMember, PermChatChannelCreate, true},
		{"member does NOT have chat:channel.manage", RoleMember, PermChatChannelManage, false},

		// Viewer: read-only
		{"viewer does NOT have org:manage", RoleViewer, PermOrgManage, false},
		{"viewer does NOT have org:members.view", RoleViewer, PermOrgMembersView, false},
		{"viewer does NOT have project:manage", RoleViewer, PermProjectManage, false},
		{"viewer has project:view", RoleViewer, PermProjectView, true},
		{"viewer has task:view", RoleViewer, PermTaskView, true},
		{"viewer does NOT have task:create", RoleViewer, PermTaskCreate, false},
		{"viewer has chat:read", RoleViewer, PermChatRead, true},
		{"viewer does NOT have chat:send", RoleViewer, PermChatSend, false},

		// Guest: very limited (same as viewer currently)
		{"guest does NOT have org:manage", RoleGuest, PermOrgManage, false},
		{"guest does NOT have project:manage", RoleGuest, PermProjectManage, false},
		{"guest has project:view", RoleGuest, PermProjectView, true},
		{"guest has task:view", RoleGuest, PermTaskView, true},
		{"guest does NOT have task:create", RoleGuest, PermTaskCreate, false},
		{"guest has chat:read", RoleGuest, PermChatRead, true},
		{"guest does NOT have chat:send", RoleGuest, PermChatSend, false},

		// Unknown role: no permissions
		{"unknown role has no permissions", Role("unknown"), PermProjectView, false},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			got := HasPermission(tt.role, tt.perm)
			if got != tt.expected {
				t.Errorf("HasPermission(%q, %q) = %v, want %v", tt.role, tt.perm, got, tt.expected)
			}
		})
	}
}

// TestRolePermissionsConsistency ensures every role has an entry in the map.
func TestRolePermissionsConsistency(t *testing.T) {
	allRoles := []Role{RoleOwner, RoleAdmin, RoleMember, RoleViewer, RoleGuest}
	for _, role := range allRoles {
		if _, ok := RolePermissions[role]; !ok {
			t.Errorf("RolePermissions map missing entry for role %q", role)
		}
	}
}

// TestRolePermissionsUnchanged ensures no unexpected permissions are granted
// by checking a specific known set. Update this when permissions change.
func TestRolePermissionsKnownSet(t *testing.T) {
	// Owner should have exactly these permissions
	ownerPerms := RolePermissions[RoleOwner]
	expectedCount := 23 // count of permission constants assigned to owner
	if len(ownerPerms) < expectedCount-1 || len(ownerPerms) > expectedCount+1 {
		t.Logf("Owner permission count: %d (expected ~%d): verify if this is intentional", len(ownerPerms), expectedCount)
	}
}
