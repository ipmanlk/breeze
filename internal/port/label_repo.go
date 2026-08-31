package port

import (
	"context"

	"ipmanlk/plume/internal/domain"
)

// Add these to repo.go alongside the existing repository interfaces.

type LabelRepository interface {
	GetByID(ctx context.Context, orgID, id string) (*domain.Label, error)
	ListByOrg(ctx context.Context, orgID string) ([]*domain.Label, error)
	Create(ctx context.Context, label *domain.Label) error
	Update(ctx context.Context, label *domain.Label) error
	Delete(ctx context.Context, orgID, id string) error

	// Task-label assignment
	ClearTaskLabels(ctx context.Context, taskID string) error
	AddTaskLabel(ctx context.Context, taskID, labelID string) error
	GetTaskLabels(ctx context.Context, taskID string) ([]*domain.Label, error)
	ListLabelsByTaskIDs(ctx context.Context, taskIDs []string) (map[string][]*domain.Label, error)
	// SetTaskLabels atomically replaces all labels for a task within a single transaction.
	SetTaskLabels(ctx context.Context, taskID string, labelIDs []string) error
}
