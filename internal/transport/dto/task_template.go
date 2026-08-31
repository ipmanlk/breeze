package dto

import (
	"time"

	"ipmanlk/plume/internal/domain"
)

type TaskTemplateResponse struct {
	ID                string   `json:"id"`
	ProjectID         string   `json:"project_id"`
	Name              string   `json:"name"`
	Description       string   `json:"description"`
	Priority          string   `json:"priority"`
	StatusID          string   `json:"status_id"`
	AssigneeIDs       []string `json:"assignee_ids"`
	Estimate          *int     `json:"estimate,omitempty"`
	RecurrencePattern string   `json:"recurrence_pattern"`
	RecurrenceDays    string   `json:"recurrence_days,omitempty"`
	NextRunAt         *string  `json:"next_run_at,omitempty"`
	CreatedBy         string   `json:"created_by"`
	CreatedAt         string   `json:"created_at"`
	UpdatedAt         string   `json:"updated_at"`
}

type CreateTaskTemplateRequest struct {
	Name              string   `json:"name" validate:"required,min=1,max=200"`
	Description       string   `json:"description"`
	Priority          string   `json:"priority"`
	StatusID          string   `json:"status_id" validate:"required"`
	AssigneeIDs       []string `json:"assignee_ids"`
	Estimate          *int     `json:"estimate"`
	RecurrencePattern string   `json:"recurrence_pattern"`
	RecurrenceDays    string   `json:"recurrence_days"`
}

type UpdateTaskTemplateRequest struct {
	Name              string   `json:"name" validate:"required,min=1,max=200"`
	Description       string   `json:"description"`
	Priority          string   `json:"priority"`
	StatusID          string   `json:"status_id" validate:"required"`
	AssigneeIDs       []string `json:"assignee_ids"`
	Estimate          *int     `json:"estimate"`
	RecurrencePattern string   `json:"recurrence_pattern"`
	RecurrenceDays    string   `json:"recurrence_days"`
}

func NewTaskTemplateResponse(t *domain.TaskTemplate) *TaskTemplateResponse {
	var assigneeIDs []string
	if t.AssigneeIDs != nil {
		assigneeIDs = t.AssigneeIDs
	}
	var nextRunAt *string
	if t.NextRunAt != nil {
		s := t.NextRunAt.Format(time.RFC3339)
		nextRunAt = &s
	}
	return &TaskTemplateResponse{
		ID:                t.ID,
		ProjectID:         t.ProjectID,
		Name:              t.Name,
		Description:       t.Description,
		Priority:          t.Priority,
		StatusID:          t.StatusID,
		AssigneeIDs:       assigneeIDs,
		Estimate:          t.Estimate,
		RecurrencePattern: t.RecurrencePattern,
		RecurrenceDays:    t.RecurrenceDays,
		NextRunAt:         nextRunAt,
		CreatedBy:         t.CreatedBy,
		CreatedAt:         t.CreatedAt.Format(time.RFC3339),
		UpdatedAt:         t.UpdatedAt.Format(time.RFC3339),
	}
}
