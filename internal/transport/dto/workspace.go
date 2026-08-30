package dto

import "time"

// WorkspaceResponse is one item in the workspace switcher list: the org plus
// the caller's role in it and whether the caller is its owner.
type WorkspaceResponse struct {
	ID        string    `json:"id"`
	Name      string    `json:"name"`
	Slug      string    `json:"slug"`
	Role      string    `json:"role"`
	IsOwner   bool      `json:"is_owner"`
	CreatedAt time.Time `json:"created_at"`
}

type CreateWorkspaceRequest struct {
	Name string `json:"name" validate:"required,min=2,max=64"`
}

// WorkspacesResponse wraps the list + the active workspace id for /auth/me and
// /auth/login responses. Embedded into UserResponse via the Workspaces field.
type WorkspacesResponse struct {
	Workspaces  []*WorkspaceResponse `json:"workspaces"`
	ActiveOrgID string               `json:"active_org_id,omitempty"`
}

func NewWorkspaceResponse(orgID, name, slug, role string, isOwner bool, createdAt time.Time) *WorkspaceResponse {
	return &WorkspaceResponse{
		ID:        orgID,
		Name:      name,
		Slug:      slug,
		Role:      role,
		IsOwner:   isOwner,
		CreatedAt: createdAt,
	}
}
