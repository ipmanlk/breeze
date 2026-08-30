package dto

import (
	"time"

	"ipmanlk/breeze/internal/domain"
)

// OrganizationResponse is the full org view returned by GET /api/organization.
type OrganizationResponse struct {
	ID                      string `json:"id"`
	Name                    string `json:"name"`
	Slug                    string `json:"slug"`
	MessageEditWindowMinute int    `json:"message_edit_window_minutes"`
	CreatedAt               string `json:"created_at"`
}

// UpdateOrganizationRequest is the body of PATCH /api/organization.
type UpdateOrganizationRequest struct {
	Name                    string `json:"name" validate:"required,min=2,max=64"`
	MessageEditWindowMinute int    `json:"message_edit_window_minutes" validate:"min=0,max=10080"`
}

// DeleteOrganizationRequest is the body of DELETE /api/organization. Confirm
// must match the org's current name (type-to-confirm); verified by the
// handler before the service deletes the org.
type DeleteOrganizationRequest struct {
	Confirm string `json:"confirm" validate:"required"`
}

func NewOrganizationResponse(org *domain.Organization) *OrganizationResponse {
	return &OrganizationResponse{
		ID:                      org.ID,
		Name:                    org.Name,
		Slug:                    org.Slug,
		MessageEditWindowMinute: org.MessageEditWindowMinute,
		CreatedAt:               org.CreatedAt.Format(time.RFC3339),
	}
}
