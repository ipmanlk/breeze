package dto

import (
	"time"

	"ipmanlk/plume/internal/domain"
)

type CreateInviteRequest struct {
	Role               string              `json:"role" validate:"required,oneof=admin member viewer guest"`
	Email              *string             `json:"email,omitempty" validate:"omitempty,email"`
	ProjectAssignments []ProjectAssignment `json:"project_assignments,omitempty" validate:"omitempty,dive"`
}

type CreateInviteResponse struct {
	ID        string    `json:"id"`
	Token     string    `json:"token"`
	URL       string    `json:"url"`
	Role      string    `json:"role"`
	ExpiresAt time.Time `json:"expires_at"`
}

type InviteResponse struct {
	ID            string  `json:"id"`
	Role          string  `json:"role"`
	Email         *string `json:"email,omitempty"`
	InvitedBy     string  `json:"invited_by"`
	InvitedByName string  `json:"invited_by_name"`
	UseCount      int     `json:"use_count"`
	ExpiresAt     string  `json:"expires_at"`
	CreatedAt     string  `json:"created_at"`
}

type AcceptInviteRequest struct {
	Name     string `json:"name" validate:"required"`
	Email    string `json:"email" validate:"required,email"`
	Password string `json:"password" validate:"required,min=8"`
}

type UpdateUserRoleRequest struct {
	Role string `json:"role" validate:"required,oneof=owner admin member viewer guest"`
}

type UpdateUserActiveRequest struct {
	IsActive bool `json:"is_active"`
}

func NewInviteResponse(i *domain.UserInvite, invitedByName string) *InviteResponse {
	return &InviteResponse{
		ID:            i.ID,
		Role:          string(i.Role),
		Email:         i.Email,
		InvitedBy:     i.InvitedBy,
		InvitedByName: invitedByName,
		UseCount:      i.UseCount,
		ExpiresAt:     i.ExpiresAt.Format(time.RFC3339),
		CreatedAt:     i.CreatedAt.Format(time.RFC3339),
	}
}
