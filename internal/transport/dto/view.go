package dto

import (
	"time"

	"ipmanlk/breeze/internal/domain"
)

type ViewFilters struct {
	Search     string   `json:"search,omitempty"`
	Priority   string   `json:"priority,omitempty"`
	StatusID   string   `json:"status_id,omitempty"`
	AssigneeID string   `json:"assignee_id,omitempty"`
	CycleID    string   `json:"cycle_id,omitempty"`
	LabelIDs   []string `json:"label_ids,omitempty"`
}

type CreateViewRequest struct {
	ProjectID *string     `json:"project_id"`
	Name      string      `json:"name" validate:"required"`
	Layout    string      `json:"layout" validate:"required,oneof=board list"`
	Filters   ViewFilters `json:"filters"`
}

type UpdateViewRequest struct {
	Name    *string      `json:"name"`
	Layout  *string      `json:"layout" validate:"omitempty,oneof=board list"`
	Filters *ViewFilters `json:"filters"`
}

type ViewResponse struct {
	ID          string      `json:"id"`
	ProjectID   *string     `json:"project_id,omitempty"`
	ProjectSlug *string     `json:"project_slug,omitempty"`
	ProjectName *string     `json:"project_name,omitempty"`
	CreatedBy   string      `json:"created_by"`
	Name        string      `json:"name"`
	Layout      string      `json:"layout"`
	Filters     ViewFilters `json:"filters"`
	CreatedAt   string      `json:"created_at"`
	UpdatedAt   string      `json:"updated_at"`
}

func NewViewResponse(v *domain.View) *ViewResponse {
	r := &ViewResponse{
		ID:        v.ID,
		CreatedBy: v.CreatedBy,
		Name:      v.Name,
		Layout:    string(v.Layout),
		Filters:   ViewFilters(v.Filters),
		CreatedAt: v.CreatedAt.Format(time.RFC3339),
		UpdatedAt: v.UpdatedAt.Format(time.RFC3339),
	}
	if v.ProjectID != nil {
		r.ProjectID = v.ProjectID
	}
	return r
}
