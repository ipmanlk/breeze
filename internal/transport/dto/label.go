package dto

import "ipmanlk/plume/internal/domain"

type LabelResponse struct {
	ID    string `json:"id"`
	Name  string `json:"name"`
	Color string `json:"color"`
}

type CreateLabelRequest struct {
	Name  string `json:"name" validate:"required,min=1,max=32"`
	Color string `json:"color"`
}

type UpdateLabelRequest struct {
	Name  string `json:"name" validate:"required,min=1,max=32"`
	Color string `json:"color"`
}

type SetTaskLabelsRequest struct {
	LabelIDs []string `json:"label_ids"`
}

func NewLabelResponse(l *domain.Label) *LabelResponse {
	return &LabelResponse{
		ID:    l.ID,
		Name:  l.Name,
		Color: l.Color,
	}
}
