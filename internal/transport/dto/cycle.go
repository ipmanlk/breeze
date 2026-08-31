package dto

import (
	"time"

	"ipmanlk/plume/internal/domain"
)

type CreateCycleRequest struct {
	Name     string `json:"name"`
	Goal     string `json:"goal"`
	StartsAt string `json:"starts_at" validate:"required"`
	EndsAt   string `json:"ends_at" validate:"required"`
}

type UpdateCycleRequest struct {
	Name        string `json:"name"`
	Goal        string `json:"goal"`
	StartsAt    string `json:"starts_at"`
	EndsAt      string `json:"ends_at"`
	IsCompleted *bool  `json:"is_completed"`
}

type CompleteCycleRequest struct {
	MoveToCycleID string `json:"move_to_cycle_id"`
}

type CycleResponse struct {
	ID                 string `json:"id"`
	ProjectID          string `json:"project_id"`
	Name               string `json:"name"`
	Goal               string `json:"goal"`
	StartsAt           string `json:"starts_at"`
	EndsAt             string `json:"ends_at"`
	CreatedBy          string `json:"created_by"`
	IsCompleted        bool   `json:"is_completed"`
	IsActive           bool   `json:"is_active"`
	TaskCount          int    `json:"task_count"`
	CompletedTaskCount int    `json:"completed_task_count"`
	CreatedAt          string `json:"created_at"`
	UpdatedAt          string `json:"updated_at"`
}

func NewCycleResponse(c *domain.Cycle) *CycleResponse {
	return &CycleResponse{
		ID:                 c.ID,
		ProjectID:          c.ProjectID,
		Name:               c.Name,
		Goal:               c.Goal,
		StartsAt:           c.StartsAt.Format(time.RFC3339),
		EndsAt:             c.EndsAt.Format(time.RFC3339),
		CreatedBy:          c.CreatedBy,
		IsCompleted:        c.IsCompleted,
		IsActive:           c.IsActive,
		TaskCount:          c.TaskCount,
		CompletedTaskCount: c.CompletedTaskCount,
		CreatedAt:          c.CreatedAt.Format(time.RFC3339),
		UpdatedAt:          c.UpdatedAt.Format(time.RFC3339),
	}
}
