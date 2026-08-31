package dto

import (
	"time"

	"ipmanlk/plume/internal/domain"
)

type CreateProjectRequest struct {
	Name                   string  `json:"name" validate:"required"`
	Description            string  `json:"description"`
	Color                  string  `json:"color"`
	Icon                   string  `json:"icon"`
	CycleDuration          *int    `json:"cycle_duration"`
	AutoGenerateCycles     *bool   `json:"auto_generate_cycles"`
	IncompleteTaskHandling *string `json:"incomplete_task_handling"`
	StartsAt               *string `json:"starts_at"`
	EndsAt                 *string `json:"ends_at"`
}

type UpdateProjectRequest struct {
	Name                   string  `json:"name"`
	Description            string  `json:"description"`
	Color                  string  `json:"color"`
	Icon                   string  `json:"icon"`
	CycleDuration          *int    `json:"cycle_duration"`
	AutoGenerateCycles     *bool   `json:"auto_generate_cycles"`
	IncompleteTaskHandling *string `json:"incomplete_task_handling"`
	StartsAt               *string `json:"starts_at"`
	EndsAt                 *string `json:"ends_at"`
}

type ProjectResponse struct {
	ID                     string  `json:"id"`
	Name                   string  `json:"name"`
	Description            string  `json:"description"`
	Slug                   string  `json:"slug"`
	Color                  string  `json:"color"`
	Icon                   string  `json:"icon"`
	CreatedBy              string  `json:"created_by"`
	CycleDuration          *int    `json:"cycle_duration,omitempty"`
	AutoGenerateCycles     bool    `json:"auto_generate_cycles"`
	IncompleteTaskHandling string  `json:"incomplete_task_handling"`
	StartsAt               *string `json:"starts_at,omitempty"`
	EndsAt                 *string `json:"ends_at,omitempty"`
	IsArchived             bool    `json:"is_archived"`
	CreatedAt              string  `json:"created_at"`
	UpdatedAt              string  `json:"updated_at"`
}

// ProjectAccessResponse describes the authenticated user's effective role
// and permission set for a single project. The frontend uses this to
// show/hide/disable UI (tabs, action buttons) without duplicating the
// role→permission map.
type ProjectAccessResponse struct {
	Role        string   `json:"role"`
	Permissions []string `json:"permissions"`
}

func NewProjectResponse(p *domain.Project) *ProjectResponse {
	r := &ProjectResponse{
		ID:                     p.ID,
		Name:                   p.Name,
		Description:            p.Description,
		Slug:                   p.Slug,
		Color:                  p.Color,
		Icon:                   p.Icon,
		CreatedBy:              p.CreatedBy,
		CycleDuration:          p.CycleDuration,
		AutoGenerateCycles:     p.AutoGenerateCycles,
		IncompleteTaskHandling: string(p.IncompleteTaskHandling),
		IsArchived:             p.IsArchived,
		CreatedAt:              p.CreatedAt.Format(time.RFC3339),
		UpdatedAt:              p.UpdatedAt.Format(time.RFC3339),
	}
	if p.StartsAt != nil {
		s := p.StartsAt.Format(time.RFC3339)
		r.StartsAt = &s
	}
	if p.EndsAt != nil {
		s := p.EndsAt.Format(time.RFC3339)
		r.EndsAt = &s
	}
	return r
}
