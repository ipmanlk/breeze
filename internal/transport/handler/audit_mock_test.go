package handler

import (
	"context"

	"ipmanlk/breeze/internal/domain"
)

// noopAuditService is a port.AuditService that discards everything. Used in
// handler tests where audit logging is not the subject under test.
type noopAuditService struct{}

func (noopAuditService) Record(_ context.Context, _, _ string, _ domain.AuditAction, _, _ string, _ any) {
}

func (noopAuditService) List(_ context.Context, _ string, _, _ int, _, _ *string) ([]*domain.AuditEntry, int, error) {
	return nil, 0, nil
}
