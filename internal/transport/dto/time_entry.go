package dto

import (
	"time"

	"ipmanlk/breeze/internal/domain"
)

type StartTimerRequest struct {
	Description string `json:"description" validate:"max=5000"`
}

type CreateTimeEntryRequest struct {
	Description     string `json:"description"`
	DurationMinutes int    `json:"duration_minutes" validate:"required,min=1"`
}

type UpdateTimeEntryRequest struct {
	Description     *string `json:"description,omitempty"`
	DurationMinutes *int    `json:"duration_minutes,omitempty"`
}

type TimeEntryResponse struct {
	ID              string  `json:"id"`
	TaskID          string  `json:"task_id"`
	UserID          string  `json:"user_id"`
	Description     string  `json:"description"`
	StartedAt       string  `json:"started_at"`
	EndedAt         *string `json:"ended_at,omitempty"`
	DurationMinutes *int    `json:"duration_minutes,omitempty"`
	CreatedAt       string  `json:"created_at"`
	UpdatedAt       string  `json:"updated_at"`
}

func NewTimeEntryResponse(e *domain.TimeEntry) *TimeEntryResponse {
	r := &TimeEntryResponse{
		ID:              e.ID,
		TaskID:          e.TaskID,
		UserID:          e.UserID,
		Description:     e.Description,
		StartedAt:       e.StartedAt.Format(time.RFC3339),
		DurationMinutes: e.DurationMinutes,
		CreatedAt:       e.CreatedAt.Format(time.RFC3339),
		UpdatedAt:       e.UpdatedAt.Format(time.RFC3339),
	}
	if e.EndedAt != nil {
		s := e.EndedAt.Format(time.RFC3339)
		r.EndedAt = &s
	}
	return r
}
