package dto

import (
	"time"

	"ipmanlk/plume/internal/domain"
)

type LoginRequest struct {
	Email    string `json:"email" validate:"required,email"`
	Password string `json:"password" validate:"required"`
}

type RequestPasswordResetRequest struct {
	Email string `json:"email" validate:"required,email"`
}

type ConfirmPasswordResetRequest struct {
	Token       string `json:"token" validate:"required"`
	NewPassword string `json:"new_password" validate:"required,min=8"`
}

type UserResponse struct {
	ID          string               `json:"id"`
	Email       string               `json:"email"`
	Name        string               `json:"name"`
	Role        string               `json:"role"`
	AvatarURL   *string              `json:"avatar_url,omitempty"`
	IsActive    bool                 `json:"is_active"`
	CreatedAt   string               `json:"created_at"`
	Org         *OrgSummaryResponse  `json:"org,omitempty"`
	Workspaces  []*WorkspaceResponse `json:"workspaces,omitempty"`
	ActiveOrgID string               `json:"active_org_id,omitempty"`
}

type OrgSummaryResponse struct {
	ID   string `json:"id"`
	Name string `json:"name"`
	Slug string `json:"slug"`
}

func NewUserResponse(u *domain.User) *UserResponse {
	avatarURL := u.AvatarURL
	if avatarURL != nil && *avatarURL != "" && u.ID != "" {
		endpoint := "/api/avatars/" + u.ID
		avatarURL = &endpoint
	}
	return &UserResponse{
		ID:        u.ID,
		Email:     u.Email,
		Name:      u.Name,
		Role:      string(u.Role),
		AvatarURL: avatarURL,
		IsActive:  u.IsActive,
		CreatedAt: u.CreatedAt.Format(time.RFC3339),
	}
}
