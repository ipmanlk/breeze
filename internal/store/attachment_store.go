package store

import (
	"context"

	"ipmanlk/plume/internal/domain"
	"ipmanlk/plume/internal/port"
	"ipmanlk/plume/internal/store/sqlc"
)

type AttachmentStore struct {
	q *sqlc.Queries
}

func NewAttachmentStore(q *sqlc.Queries) *AttachmentStore {
	return &AttachmentStore{q: q}
}

var _ port.AttachmentRepository = (*AttachmentStore)(nil)

func (s *AttachmentStore) toDomain(a sqlc.Attachment) domain.Attachment {
	return domain.Attachment{
		ID:          a.ID,
		TaskID:      a.TaskID,
		Filename:    a.Filename,
		ContentType: a.ContentType,
		Size:        a.Size,
		StoragePath: a.StoragePath,
		CreatedBy:   a.CreatedBy,
		CreatedAt:   parseTime(a.CreatedAt),
	}
}

func (s *AttachmentStore) ListByTask(ctx context.Context, taskID string) ([]*domain.Attachment, error) {
	rows, err := s.q.ListAttachments(ctx, taskID)
	if err != nil {
		return nil, err
	}
	items := make([]*domain.Attachment, len(rows))
	for i, row := range rows {
		d := s.toDomain(row)
		items[i] = &d
	}
	return items, nil
}

func (s *AttachmentStore) GetByID(ctx context.Context, id string) (*domain.Attachment, error) {
	a, err := s.q.GetAttachment(ctx, id)
	if err != nil {
		return nil, mapScanErr(err)
	}
	d := s.toDomain(a)
	return &d, nil
}

func (s *AttachmentStore) Create(ctx context.Context, a *domain.Attachment) error {
	return s.q.CreateAttachment(ctx, sqlc.CreateAttachmentParams{
		ID:          a.ID,
		TaskID:      a.TaskID,
		Filename:    a.Filename,
		ContentType: a.ContentType,
		Size:        a.Size,
		StoragePath: a.StoragePath,
		CreatedBy:   a.CreatedBy,
	})
}

func (s *AttachmentStore) Delete(ctx context.Context, id, taskID string) error {
	return s.q.DeleteAttachment(ctx, sqlc.DeleteAttachmentParams{ID: id, TaskID: taskID})
}
