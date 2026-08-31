package dto

import (
	"time"

	"ipmanlk/plume/internal/domain"
)

type CustomFieldResponse struct {
	ID        string   `json:"id"`
	ProjectID string   `json:"project_id"`
	Name      string   `json:"name"`
	FieldType string   `json:"field_type"`
	Options   []string `json:"options"`
	Position  int      `json:"position"`
	CreatedAt string   `json:"created_at"`
	UpdatedAt string   `json:"updated_at"`
}

type CreateCustomFieldRequest struct {
	Name      string   `json:"name" validate:"required,min=1,max=64"`
	FieldType string   `json:"field_type" validate:"required"`
	Options   []string `json:"options"`
	Position  int      `json:"position"`
}

type UpdateCustomFieldRequest struct {
	Name     string   `json:"name" validate:"required,min=1,max=64"`
	Options  []string `json:"options"`
	Position int      `json:"position"`
}

type SetCustomFieldValueRequest struct {
	Value string `json:"value"`
}

func NewCustomFieldResponse(f *domain.CustomField) *CustomFieldResponse {
	options := f.Options
	if options == nil {
		options = []string{}
	}
	return &CustomFieldResponse{
		ID:        f.ID,
		ProjectID: f.ProjectID,
		Name:      f.Name,
		FieldType: f.FieldType,
		Options:   options,
		Position:  f.Position,
		CreatedAt: f.CreatedAt.Format(time.RFC3339),
		UpdatedAt: f.UpdatedAt.Format(time.RFC3339),
	}
}
