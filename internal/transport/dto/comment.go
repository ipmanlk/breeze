package dto

import (
	"time"

	"ipmanlk/breeze/internal/domain"
)

type CommentResponse struct {
	ID        string  `json:"id"`
	TaskID    string  `json:"task_id"`
	ProjectID string  `json:"project_id"`
	AuthorID  string  `json:"author_id"`
	Content   string  `json:"content"`
	ParentID  *string `json:"parent_id,omitempty"`
	CreatedAt string  `json:"created_at"`
	UpdatedAt string  `json:"updated_at"`
	EditedAt  *string `json:"edited_at,omitempty"`

	AuthorName      string  `json:"author_name"`
	AuthorEmail     string  `json:"author_email"`
	AuthorAvatarURL *string `json:"author_avatar_url,omitempty"`

	// Mentions holds resolved labels for <@type:id> tokens in content,
	// mirroring the chat message shape so the frontend reuses the same
	// markdown renderer + mention chip rendering.
	Mentions *MentionsResponse `json:"mentions,omitempty"`
}

type CommentListResponse struct {
	Items      []*CommentResponse `json:"items"`
	NextCursor string             `json:"next_cursor"`
	HasMore    bool               `json:"has_more"`
}

type CreateCommentRequest struct {
	Content  string  `json:"content" validate:"required,min=1,max=10000"`
	ParentID *string `json:"parent_id,omitempty"`
}

type UpdateCommentRequest struct {
	Content string `json:"content" validate:"required,min=1,max=10000"`
}

func NewCommentResponse(c *domain.Comment) *CommentResponse {
	r := &CommentResponse{
		ID:              c.ID,
		TaskID:          c.TaskID,
		ProjectID:       c.ProjectID,
		AuthorID:        c.AuthorID,
		Content:         c.Content,
		ParentID:        c.ParentID,
		CreatedAt:       c.CreatedAt.Format(time.RFC3339),
		UpdatedAt:       c.UpdatedAt.Format(time.RFC3339),
		EditedAt:        nil,
		AuthorName:      c.AuthorName,
		AuthorEmail:     c.AuthorEmail,
		AuthorAvatarURL: publicAvatarURL(c.AuthorID, c.AuthorAvatarURL),
		Mentions:        ToMentionsResponse(c.Mentions),
	}
	if c.EditedAt != nil {
		ts := c.EditedAt.Format(time.RFC3339)
		r.EditedAt = &ts
	}
	return r
}
