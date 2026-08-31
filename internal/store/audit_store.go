package store

import (
	"context"
	"time"

	"ipmanlk/plume/internal/domain"
	"ipmanlk/plume/internal/port"
	"ipmanlk/plume/internal/store/sqlc"
)

type AuditStore struct {
	q *sqlc.Queries
}

func NewAuditStore(q *sqlc.Queries) *AuditStore {
	return &AuditStore{q: q}
}

var _ port.AuditRepository = (*AuditStore)(nil)

func (s *AuditStore) Create(ctx context.Context, entry *domain.AuditEntry) error {
	return s.q.CreateAuditEntry(ctx, sqlc.CreateAuditEntryParams{
		ID:         entry.ID,
		OrgID:      entry.OrgID,
		ActorID:    entry.ActorID,
		Action:     string(entry.Action),
		EntityType: entry.EntityType,
		EntityID:   entry.EntityID,
		Metadata:   entry.Metadata,
	})
}

func (s *AuditStore) List(ctx context.Context, orgID string, limit, offset int, action, actorID *string) ([]*domain.AuditEntry, error) {
	if limit <= 0 || limit > 100 {
		limit = 50
	}
	rows, err := s.q.ListAuditEntries(ctx, sqlc.ListAuditEntriesParams{
		OrgID:         orgID,
		ActionFilter:  action,
		ActorIDFilter: actorID,
		LimitVal:      int64(limit),
		OffsetVal:     int64(offset),
	})
	if err != nil {
		return nil, err
	}
	out := make([]*domain.AuditEntry, len(rows))
	for i, r := range rows {
		out[i] = &domain.AuditEntry{
			ID:         r.ID,
			OrgID:      r.OrgID,
			ActorID:    r.ActorID,
			Action:     domain.AuditAction(r.Action),
			EntityType: r.EntityType,
			EntityID:   r.EntityID,
			Metadata:   r.Metadata,
			CreatedAt:  parseTime(r.CreatedAt),
			ActorName:  r.ActorName,
			ActorEmail: r.ActorEmail,
		}
	}
	return out, nil
}

func (s *AuditStore) Count(ctx context.Context, orgID string, action, actorID *string) (int, error) {
	n, err := s.q.CountAuditEntries(ctx, sqlc.CountAuditEntriesParams{
		OrgID:         orgID,
		ActionFilter:  action,
		ActorIDFilter: actorID,
	})
	if err != nil {
		return 0, err
	}
	return int(n), nil
}

// DeleteOlderThan removes audit entries older than the cutoff (across all
// orgs). Used by the periodic retention cleanup. Returns the row count.
func (s *AuditStore) DeleteOlderThan(ctx context.Context, cutoff time.Time) (int64, error) {
	return s.q.DeleteAuditEntriesOlderThan(ctx, cutoff.UTC().Format("2006-01-02 15:04:05"))
}
