package service

import (
	"context"
	"encoding/json"
	"log/slog"

	"ipmanlk/plume/internal/domain"
	"ipmanlk/plume/internal/port"

	"github.com/google/uuid"
)

// AuditService records admin actions and lists them for an organization.
//
// Record is best-effort: it logs but never returns an error to the caller,
// because an audit-log failure must not block the admin action it accompanies.
// List returns paginated entries (newest first) with the actor's display
// name/email joined in.
type AuditService struct {
	repo port.AuditRepository
	log  *slog.Logger
}

func NewAuditService(repo port.AuditRepository, log *slog.Logger) *AuditService {
	return &AuditService{repo: repo, log: log}
}

var _ port.AuditService = (*AuditService)(nil)

// Record persists an audit entry. It is best-effort: failures are logged and
// swallowed so the calling handler's response is unaffected.
func (s *AuditService) Record(ctx context.Context, orgID, actorID string, action domain.AuditAction, entityType, entityID string, metadata any) {
	var metaJSON string
	if metadata != nil {
		b, err := json.Marshal(metadata)
		if err != nil {
			s.log.Warn("audit metadata marshal", "action", action, "error", err)
		} else {
			metaJSON = string(b)
		}
	}
	entry := &domain.AuditEntry{
		ID:         uuid.New().String(),
		OrgID:      orgID,
		ActorID:    actorID,
		Action:     action,
		EntityType: entityType,
		EntityID:   entityID,
		Metadata:   metaJSON,
	}
	if err := s.repo.Create(ctx, entry); err != nil {
		// Never surface audit errors to the admin action that triggered this.
		s.log.Warn("audit record", "action", action, "error", err)
	}
}

// List returns a page of audit entries (newest first) plus the total count.
// action and actorID are optional filters (nil = no filter).
func (s *AuditService) List(ctx context.Context, orgID string, limit, offset int, action, actorID *string) ([]*domain.AuditEntry, int, error) {
	total, err := s.repo.Count(ctx, orgID, action, actorID)
	if err != nil {
		return nil, 0, err
	}
	entries, err := s.repo.List(ctx, orgID, limit, offset, action, actorID)
	if err != nil {
		return nil, 0, err
	}
	return entries, total, nil
}
