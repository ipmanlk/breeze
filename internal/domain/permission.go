package domain

type Permission string

const (
	PermOrgManage        Permission = "org:manage"
	PermOrgDelete        Permission = "org:delete"
	PermOrgMembersManage Permission = "org:members.manage"
	PermOrgMembersView   Permission = "org:members.view"
	PermOrgMembersInvite Permission = "org:members.invite"

	PermProjectCreate        Permission = "project:create"
	PermProjectManage        Permission = "project:manage"
	PermProjectDelete        Permission = "project:delete"
	PermProjectView          Permission = "project:view"
	PermProjectStatusManage  Permission = "project:status.manage"
	PermProjectCycleManage   Permission = "project:cycle.manage"
	PermProjectMembersManage Permission = "project:members.manage"

	PermTaskCreate Permission = "task:create"
	PermTaskEdit   Permission = "task:edit"
	PermTaskDelete Permission = "task:delete"
	PermTaskView   Permission = "task:view"

	PermAttachmentCreate Permission = "attachment:create"
	PermAttachmentDelete Permission = "attachment:delete"
	PermTimeCreate       Permission = "time:create"
	PermTimeDelete       Permission = "time:delete"

	PermNotificationView Permission = "notification:view"

	PermChatRead          Permission = "chat:read"
	PermChatSend          Permission = "chat:send"
	PermChatChannelCreate Permission = "chat:channel.create"
	PermChatChannelManage Permission = "chat:channel.manage"

	// Channel-level permissions (stored in category_permissions and channel_permission_overrides tables)
	PermChannelView        Permission = "channel:view"
	PermChannelSend        Permission = "channel:send"
	PermChannelManage      Permission = "channel:manage"
	PermChannelPermissions Permission = "channel:permissions"
)

var RolePermissions = map[Role][]Permission{
	RoleOwner: {
		PermOrgManage, PermOrgDelete,
		PermOrgMembersManage, PermOrgMembersView, PermOrgMembersInvite,
		PermProjectCreate, PermProjectManage, PermProjectDelete, PermProjectView,
		PermProjectStatusManage, PermProjectCycleManage, PermProjectMembersManage,
		PermTaskCreate, PermTaskEdit, PermTaskDelete, PermTaskView,
		PermAttachmentCreate, PermAttachmentDelete,
		PermTimeCreate, PermTimeDelete,
		PermNotificationView,
		PermChatRead, PermChatSend, PermChatChannelCreate, PermChatChannelManage,
	},
	RoleAdmin: {
		PermOrgManage,
		PermOrgMembersManage, PermOrgMembersView, PermOrgMembersInvite,
		PermProjectCreate, PermProjectManage, PermProjectDelete, PermProjectView,
		PermProjectStatusManage, PermProjectCycleManage, PermProjectMembersManage,
		PermTaskCreate, PermTaskEdit, PermTaskDelete, PermTaskView,
		PermAttachmentCreate, PermAttachmentDelete,
		PermTimeCreate, PermTimeDelete,
		PermNotificationView,
		PermChatRead, PermChatSend, PermChatChannelCreate, PermChatChannelManage,
	},
	RoleMember: {
		PermOrgMembersView, PermOrgMembersInvite,
		PermProjectView,
		PermTaskCreate, PermTaskEdit, PermTaskView,
		PermAttachmentCreate,
		PermTimeCreate,
		PermNotificationView,
		PermChatRead, PermChatSend, PermChatChannelCreate,
	},
	RoleViewer: {
		PermProjectView,
		PermTaskView,
		PermNotificationView,
		PermChatRead,
	},
	RoleGuest: {
		PermProjectView,
		PermTaskView,
		PermNotificationView,
		PermChatRead,
	},
}

func HasPermission(role Role, perm Permission) bool {
	for _, p := range RolePermissions[role] {
		if p == perm {
			return true
		}
	}
	return false
}

// PermissionsForRole returns the full list of permissions granted to a role.
// Used to expose the effective permission set to clients (e.g. the project
// "my-access" endpoint) so the frontend can show/hide/disable UI without
// duplicating the role→permission map.
func PermissionsForRole(role Role) []Permission {
	perms := RolePermissions[role]
	out := make([]Permission, len(perms))
	copy(out, perms)
	return out
}
