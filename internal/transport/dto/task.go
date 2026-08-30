package dto

import (
	"time"

	"ipmanlk/breeze/internal/domain"
)

type CreateTaskRequest struct {
	Title       string   `json:"title" validate:"required"`
	Description string   `json:"description"`
	StatusID    string   `json:"status_id" validate:"required"`
	Priority    string   `json:"priority"`
	AssigneeIDs []string `json:"assignee_ids"`
	CycleID     *string  `json:"cycle_id"`
	ParentID    *string  `json:"parent_task_id"`
	Estimate    *int     `json:"estimate"`
	StartedAt   *string  `json:"started_at"`
	DueAt       *string  `json:"due_at"`
}

type UpdateTaskRequest struct {
	Title       *string  `json:"title,omitempty"`
	Description *string  `json:"description,omitempty"`
	StatusID    *string  `json:"status_id,omitempty"`
	Priority    *string  `json:"priority,omitempty"`
	AssigneeIDs []string `json:"assignee_ids"`
	CycleID     *string  `json:"cycle_id"`
	ParentID    *string  `json:"parent_task_id"`
	Estimate    *int     `json:"estimate"`
	StartedAt   *string  `json:"started_at"`
	DueAt       *string  `json:"due_at"`
}

type MoveTaskRequest struct {
	StatusID    string `json:"status_id" validate:"required"`
	PositionKey string `json:"position_key" validate:"required"`
}

type ReorderOp struct {
	TaskID      string `json:"task_id" validate:"required"`
	PositionKey string `json:"position_key" validate:"required"`
}

type ReorderRequest struct {
	Operations []ReorderOp `json:"operations" validate:"required,min=1,max=500"`
}

// BatchUpdateRequest applies a partial update to many tasks. Pointer fields
// are applied only when present in the JSON; absent fields are left untouched.
type BatchUpdateRequest struct {
	TaskIDs      []string `json:"task_ids" validate:"required,min=1,max=500"`
	StatusID     *string  `json:"status_id,omitempty"`
	Priority     *string  `json:"priority,omitempty"`
	AssigneeIDs  []string `json:"assignee_ids,omitempty"`
	AssigneeMode string   `json:"assignee_mode,omitempty"` // replace (default) | add | remove
	CycleID      *string  `json:"cycle_id,omitempty"`
}

// MoveToProjectRequest moves a task to a different project. to_status_id is
// optional; when omitted the target project's default status is used so the
// caller doesn't need to pre-load the target's statuses.
type MoveToProjectRequest struct {
	ToProjectID string `json:"to_project_id" validate:"required"`
	ToStatusID  string `json:"to_status_id,omitempty"`
}

type TaskResponse struct {
	ID              string                 `json:"id"`
	ProjectID       string                 `json:"project_id"`
	CycleID         *string                `json:"cycle_id,omitempty"`
	ParentID        *string                `json:"parent_task_id,omitempty"`
	ParentTitle     string                 `json:"parent_title,omitempty"`
	CreatedBy       string                 `json:"created_by"`
	Assignees       []TaskAssigneeResponse `json:"assignees"`
	Labels          []LabelResponse        `json:"labels,omitempty"`
	Title           string                 `json:"title"`
	Description     string                 `json:"description"`
	StatusID        string                 `json:"status_id"`
	Priority        string                 `json:"priority"`
	PositionKey     string                 `json:"position_key"`
	SubtaskPosition string                 `json:"subtask_position,omitempty"`
	Estimate        *int                   `json:"estimate,omitempty"`
	StartedAt       *string                `json:"started_at,omitempty"`
	DueAt           *string                `json:"due_at,omitempty"`
	CompletedAt     *string                `json:"completed_at,omitempty"`
	TemplateID      *string                `json:"template_id,omitempty"`
	CreatedAt       string                 `json:"created_at"`
	UpdatedAt       string                 `json:"updated_at"`

	// SubtaskCount / CompletedSubtaskCount let the frontend render progress
	// (done/total) without client-side counting from a filtered task list.
	SubtaskCount          int `json:"subtask_count"`
	CompletedSubtaskCount int `json:"completed_subtask_count"`

	// Mentions holds resolved labels for <@type:id> tokens in Description,
	// mirroring the chat/comment shape so the frontend reuses the same
	// markdown renderer + mention chip rendering.
	Mentions *MentionsResponse `json:"mentions,omitempty"`
}

func NewTaskResponse(t *domain.Task) *TaskResponse {
	assignees := make([]TaskAssigneeResponse, len(t.Assignees))
	for i, a := range t.Assignees {
		assignees[i] = TaskAssigneeResponse{
			ID:        a.ID,
			Name:      a.Name,
			Email:     a.Email,
			AvatarURL: publicAvatarURL(a.ID, a.AvatarURL),
		}
	}
	labels := make([]LabelResponse, len(t.Labels))
	for i, l := range t.Labels {
		labels[i] = LabelResponse{
			ID:    l.ID,
			Name:  l.Name,
			Color: l.Color,
		}
	}
	r := &TaskResponse{
		ID:                    t.ID,
		ProjectID:             t.ProjectID,
		CycleID:               t.CycleID,
		ParentID:              t.ParentID,
		ParentTitle:           t.ParentTitle,
		CreatedBy:             t.CreatedBy,
		Assignees:             assignees,
		Labels:                labels,
		Title:                 t.Title,
		Description:           t.Description,
		StatusID:              t.StatusID,
		Priority:              t.Priority,
		PositionKey:           t.PositionKey,
		SubtaskPosition:       t.SubtaskPosition,
		Estimate:              t.Estimate,
		SubtaskCount:          t.SubtaskCount,
		CompletedSubtaskCount: t.CompletedSubtaskCount,
		Mentions:              ToMentionsResponse(t.Mentions),
		CreatedAt:             t.CreatedAt.Format(time.RFC3339),
		UpdatedAt:             t.UpdatedAt.Format(time.RFC3339),
	}
	if t.StartedAt != nil {
		s := t.StartedAt.Format(time.RFC3339)
		r.StartedAt = &s
	}
	if t.DueAt != nil {
		s := t.DueAt.Format(time.RFC3339)
		r.DueAt = &s
	}
	if t.CompletedAt != nil {
		s := t.CompletedAt.Format(time.RFC3339)
		r.CompletedAt = &s
	}
	r.TemplateID = t.TemplateID
	return r
}
