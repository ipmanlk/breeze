package dto

type PaginatedUsersResponse struct {
	Items      []*UserResponse `json:"items"`
	NextCursor string          `json:"next_cursor,omitempty"`
	HasMore    bool            `json:"has_more"`
}

type TaskAssigneeResponse struct {
	ID        string  `json:"id"`
	Name      string  `json:"name"`
	Email     string  `json:"email"`
	AvatarURL *string `json:"avatar_url,omitempty"`
}

type ProjectMemberResponse struct {
	ID          string  `json:"id"`
	Name        string  `json:"name"`
	Email       string  `json:"email"`
	AvatarURL   *string `json:"avatar_url,omitempty"`
	OrgRole     string  `json:"org_role"`
	ProjectRole string  `json:"project_role"`
	// Role is the effective role for display: the org role for owner/admin/
	// member (whose access is org-wide) and the project role for viewer/guest
	// (whose access is per-project and overridable).
	Role string `json:"role"`
	// RoleOverridable reports whether the project role can be changed for this
	// member: true only for project-scoped org roles (viewer/guest). For
	// owner/admin/member the project role is meaningless, so the UI hides the
	// role editor to avoid confusion.
	RoleOverridable bool `json:"role_overridable"`
}

type AddProjectMemberRequest struct {
	UserID string `json:"user_id" validate:"required"`
	Role   string `json:"role" validate:"required,oneof=admin member viewer guest"`
}

type UpdateProjectMemberRoleRequest struct {
	Role string `json:"role" validate:"required,oneof=admin member viewer guest"`
}

type PaginatedProjectMembersResponse struct {
	Items      []*ProjectMemberResponse `json:"items"`
	NextCursor string                   `json:"next_cursor,omitempty"`
	HasMore    bool                     `json:"has_more"`
}

type UserProjectMembershipResponse struct {
	ProjectID string `json:"project_id"`
	Name      string `json:"name"`
	Color     string `json:"color"`
	Icon      string `json:"icon"`
	Role      string `json:"role"`
}

type ProjectAssignment struct {
	ProjectID string `json:"project_id" validate:"required"`
	Role      string `json:"role" validate:"required,oneof=admin member viewer guest"`
}

type UpdateUserProjectMembershipsRequest struct {
	Assignments []ProjectAssignment `json:"assignments" validate:"required,dive"`
}
