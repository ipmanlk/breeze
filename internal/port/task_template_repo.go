package port

import (
	"context"
	"time"

	"ipmanlk/plume/internal/domain"
)

type TaskTemplateRepository interface {
	Create(ctx context.Context, t *domain.TaskTemplate) error
	GetByID(ctx context.Context, orgID, id string) (*domain.TaskTemplate, error)
	ListByProject(ctx context.Context, orgID, projectID string) ([]*domain.TaskTemplate, error)
	ListDueRecurring(ctx context.Context, before time.Time) ([]*domain.TaskTemplate, error)
	Update(ctx context.Context, t *domain.TaskTemplate) error
	UpdateNextRun(ctx context.Context, orgID, id string, nextRun *time.Time) error
	ClaimDueRecurring(ctx context.Context, orgID, id string, currentNextRun, newNextRun *time.Time) (bool, error)
	// SetLastError records the last instantiation error for a recurring
	// template (visibility for silent failures). Empty msg clears it.
	SetLastError(ctx context.Context, orgID, id, msg string) error
	Delete(ctx context.Context, orgID, id string) error
}
