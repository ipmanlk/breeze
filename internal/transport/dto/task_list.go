package dto

import (
	"time"

	"ipmanlk/plume/internal/domain"
)

// TaskListPageResponse is the paginated envelope for GET /tasks.
// Matches the cursor-based pagination pattern used across all list endpoints
// (see docs/api/pagination.md).
type TaskListPageResponse struct {
	Items      []*TaskListResponse `json:"items"`
	NextCursor string              `json:"next_cursor"`
	HasMore    bool                `json:"has_more"`
}

// TaskListResponse augments TaskResponse with project/status context needed for cross-project views.
type TaskListResponse struct {
	ID           string                 `json:"id"`
	ProjectID    string                 `json:"project_id"`
	ProjectName  string                 `json:"project_name"`
	ProjectSlug  string                 `json:"project_slug"`
	ProjectColor string                 `json:"project_color"`
	StatusName   string                 `json:"status_name"`
	StatusColor  string                 `json:"status_color"`
	CycleID      *string                `json:"cycle_id,omitempty"`
	ParentID     *string                `json:"parent_task_id,omitempty"`
	CreatedBy    string                 `json:"created_by"`
	Assignees    []TaskAssigneeResponse `json:"assignees"`
	Title        string                 `json:"title"`
	Description  string                 `json:"description"`
	StatusID     string                 `json:"status_id"`
	Priority     string                 `json:"priority"`
	PositionKey  string                 `json:"position_key"`
	Estimate     *int                   `json:"estimate,omitempty"`
	StartedAt    *string                `json:"started_at,omitempty"`
	DueAt        *string                `json:"due_at,omitempty"`
	CompletedAt  *string                `json:"completed_at,omitempty"`
	CreatedAt    string                 `json:"created_at"`
	UpdatedAt    string                 `json:"updated_at"`
}

func NewTaskListResponse(t *domain.EnrichedTask) *TaskListResponse {
	assignees := make([]TaskAssigneeResponse, len(t.Assignees))
	for i, a := range t.Assignees {
		assignees[i] = TaskAssigneeResponse{
			ID:        a.ID,
			Name:      a.Name,
			Email:     a.Email,
			AvatarURL: publicAvatarURL(a.ID, a.AvatarURL),
		}
	}
	r := &TaskListResponse{
		ID:           t.ID,
		ProjectID:    t.ProjectID,
		ProjectName:  t.ProjectName,
		ProjectSlug:  t.ProjectSlug,
		ProjectColor: t.ProjectColor,
		StatusName:   t.StatusName,
		StatusColor:  t.StatusColor,
		CycleID:      t.CycleID,
		ParentID:     t.ParentID,
		CreatedBy:    t.CreatedBy,
		Assignees:    assignees,
		Title:        t.Title,
		Description:  t.Description,
		StatusID:     t.StatusID,
		Priority:     t.Priority,
		PositionKey:  t.PositionKey,
		Estimate:     t.Estimate,
		CreatedAt:    t.CreatedAt.Format(time.RFC3339),
		UpdatedAt:    t.UpdatedAt.Format(time.RFC3339),
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
	return r
}
