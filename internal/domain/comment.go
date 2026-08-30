package domain

import "time"

type Comment struct {
	ID        string
	OrgID     string
	TaskID    string
	ProjectID string
	AuthorID  string
	Content   string
	ParentID  *string
	CreatedAt time.Time
	UpdatedAt time.Time
	EditedAt  *time.Time
	DeletedAt *time.Time

	// Author is denormalized in the comment query via JOIN with users.
	AuthorName      string
	AuthorEmail     string
	AuthorAvatarURL *string

	// Mentions holds resolved mention labels for all types found in content,
	// hydrated by the service (mirrors the chat message pattern).
	Mentions *Mentions
}

type CreateCommentParams struct {
	OrgID    string
	TaskID   string
	AuthorID string
	Content  string
	ParentID *string
}

type UpdateCommentParams struct {
	ID      string
	OrgID   string
	Content string
}

type CommentFilter struct {
	TaskID string
	// OrgID scopes the query so a foreign-org task_id cannot read comments.
	OrgID string
	// BeforeCursor is the created_at of the oldest loaded comment.
	// Empty = load the newest page.
	BeforeCursor string
	Limit        int
}

type CommentListResult struct {
	Items      []*Comment `json:"items"`
	NextCursor string     `json:"next_cursor"`
	HasMore    bool       `json:"has_more"`
}
