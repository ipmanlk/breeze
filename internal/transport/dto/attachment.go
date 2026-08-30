package dto

import (
	"time"

	"ipmanlk/breeze/internal/domain"
)

type AttachmentResponse struct {
	ID          string `json:"id"`
	TaskID      string `json:"task_id"`
	Filename    string `json:"filename"`
	ContentType string `json:"content_type"`
	Size        int64  `json:"size"`
	CreatedBy   string `json:"created_by"`
	CreatedAt   string `json:"created_at"`
}

func NewAttachmentResponse(a *domain.Attachment) *AttachmentResponse {
	return &AttachmentResponse{
		ID:          a.ID,
		TaskID:      a.TaskID,
		Filename:    a.Filename,
		ContentType: a.ContentType,
		Size:        a.Size,
		CreatedBy:   a.CreatedBy,
		CreatedAt:   a.CreatedAt.Format(time.RFC3339),
	}
}
