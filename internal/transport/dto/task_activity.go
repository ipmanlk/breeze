package dto

import "ipmanlk/plume/internal/domain"

// TaskActivityResponse is a single activity entry for a task's activity feed.
type TaskActivityResponse struct {
	ID         string `json:"id"`
	TaskID     string `json:"task_id"`
	ActorID    string `json:"actor_id"`
	ActorName  string `json:"actor_name"`
	ActorEmail string `json:"actor_email"`
	Action     string `json:"action"`
	Field      string `json:"field"`
	OldValue   string `json:"old_value"`
	NewValue   string `json:"new_value"`
	CreatedAt  string `json:"created_at"`
}

func NewTaskActivityResponse(a *domain.TaskActivity) *TaskActivityResponse {
	return &TaskActivityResponse{
		ID:         a.ID,
		TaskID:     a.TaskID,
		ActorID:    a.ActorID,
		ActorName:  a.ActorName,
		ActorEmail: a.ActorEmail,
		Action:     string(a.Action),
		Field:      a.Field,
		OldValue:   a.OldValue,
		NewValue:   a.NewValue,
		CreatedAt:  a.CreatedAt.Format("2006-01-02T15:04:05Z07:00"),
	}
}

// PaginatedTaskActivityResponse is the paginated envelope for task activity.
type PaginatedTaskActivityResponse struct {
	Items      []*TaskActivityResponse `json:"items"`
	NextCursor string                  `json:"next_cursor,omitempty"`
	HasMore    bool                    `json:"has_more"`
}

func NewPaginatedTaskActivityResponse(r *domain.TaskActivityResult) *PaginatedTaskActivityResponse {
	items := make([]*TaskActivityResponse, len(r.Items))
	for i, a := range r.Items {
		items[i] = NewTaskActivityResponse(a)
	}
	return &PaginatedTaskActivityResponse{
		Items:      items,
		NextCursor: r.NextCursor,
		HasMore:    r.HasMore,
	}
}
