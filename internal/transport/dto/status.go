package dto

import "ipmanlk/plume/internal/domain"

type CreateStatusRequest struct {
	Name     string `json:"name" validate:"required"`
	Color    string `json:"color" validate:"required"`
	Position int    `json:"position"`
	Category string `json:"category"`
}

type UpdateStatusRequest struct {
	Name     string `json:"name"`
	Color    string `json:"color"`
	Position *int   `json:"position,omitempty"`
	Category string `json:"category"`
}

type TaskStatusResponse struct {
	ID        string `json:"id"`
	ProjectID string `json:"project_id"`
	Name      string `json:"name"`
	Color     string `json:"color"`
	Position  int    `json:"position"`
	Category  string `json:"category"`
	Default   bool   `json:"default"`
}

func NewTaskStatusResponse(s *domain.TaskStatus) *TaskStatusResponse {
	return &TaskStatusResponse{
		ID:        s.ID,
		ProjectID: s.ProjectID,
		Name:      s.Name,
		Color:     s.Color,
		Position:  s.Position,
		Category:  s.Category,
		Default:   s.Default,
	}
}
