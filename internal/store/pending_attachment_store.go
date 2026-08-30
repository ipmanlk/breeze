package store

import (
	"context"
	"time"

	"ipmanlk/breeze/internal/domain"
	"ipmanlk/breeze/internal/port"
	"ipmanlk/breeze/internal/store/sqlc"
)

var _ port.PendingAttachmentRepository = (*PendingAttachmentStore)(nil)

type PendingAttachmentStore struct {
	q *sqlc.Queries
}

func NewPendingAttachmentStore(q *sqlc.Queries) *PendingAttachmentStore {
	return &PendingAttachmentStore{q: q}
}

func (s *PendingAttachmentStore) Create(ctx context.Context, att *domain.PendingAttachment) error {
	return s.q.CreatePendingAttachment(ctx, sqlc.CreatePendingAttachmentParams{
		ID:             att.ID,
		ConversationID: att.ConversationID,
		FileName:       att.FileName,
		FileSize:       att.FileSize,
		ContentType:    att.ContentType,
		StoragePath:    att.StoragePath,
		UploadedBy:     att.UploadedBy,
	})
}

func (s *PendingAttachmentStore) GetByID(ctx context.Context, id, uploadedBy string) (*domain.PendingAttachment, error) {
	row, err := s.q.GetPendingAttachmentByID(ctx, sqlc.GetPendingAttachmentByIDParams{
		ID:         id,
		UploadedBy: uploadedBy,
	})
	if err != nil {
		return nil, mapScanErr(err)
	}
	return pendingToDomain(row), nil
}

func (s *PendingAttachmentStore) Delete(ctx context.Context, id string) error {
	return s.q.DeletePendingAttachment(ctx, id)
}

func (s *PendingAttachmentStore) DeleteOlderThan(ctx context.Context, before time.Time) ([]*domain.PendingAttachment, error) {
	rows, err := s.q.DeletePendingAttachmentsOlderThan(ctx, formatTime(before))
	if err != nil {
		return nil, err
	}
	items := make([]*domain.PendingAttachment, 0, len(rows))
	for _, r := range rows {
		items = append(items, pendingToDomain(r))
	}
	return items, nil
}

func pendingToDomain(r sqlc.PendingAttachment) *domain.PendingAttachment {
	return &domain.PendingAttachment{
		ID:             r.ID,
		ConversationID: r.ConversationID,
		FileName:       r.FileName,
		FileSize:       r.FileSize,
		ContentType:    r.ContentType,
		StoragePath:    r.StoragePath,
		UploadedBy:     r.UploadedBy,
		CreatedAt:      parseTime(r.CreatedAt),
	}
}
