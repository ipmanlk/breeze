package port

import (
	"context"

	"ipmanlk/plume/internal/domain"
)

type CommentRepository interface {
	GetByID(ctx context.Context, orgID, id string) (*domain.Comment, error)
	ListByTask(ctx context.Context, filter domain.CommentFilter) (*domain.CommentListResult, error)
	Create(ctx context.Context, comment *domain.Comment) error
	Update(ctx context.Context, comment *domain.Comment) error
	SoftDelete(ctx context.Context, orgID, id string) error
}
