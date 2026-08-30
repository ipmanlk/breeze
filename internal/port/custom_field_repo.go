package port

import (
	"context"

	"ipmanlk/breeze/internal/domain"
)

type CustomFieldRepository interface {
	Create(ctx context.Context, f *domain.CustomField) error
	GetByID(ctx context.Context, orgID, id string) (*domain.CustomField, error)
	ListByProject(ctx context.Context, orgID, projectID string) ([]*domain.CustomField, error)
	Update(ctx context.Context, f *domain.CustomField) error
	Delete(ctx context.Context, orgID, id string) error
	SetValue(ctx context.Context, taskID, fieldID, value string) error
	GetValuesByTask(ctx context.Context, taskID string) (map[string]string, error)
	ListValuesByTaskIDs(ctx context.Context, taskIDs []string) (map[string]map[string]string, error)
	DeleteValuesForTask(ctx context.Context, taskID string) error
}
