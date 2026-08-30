package store

import (
	"context"
	"fmt"

	"ipmanlk/breeze/internal/domain"
	"ipmanlk/breeze/internal/port"
	"ipmanlk/breeze/internal/store/sqlc"
)

var _ port.MessageAttachmentRepository = (*MessageAttachmentStore)(nil)

type MessageAttachmentStore struct {
	q *sqlc.Queries
}

func NewMessageAttachmentStore(q *sqlc.Queries) *MessageAttachmentStore {
	return &MessageAttachmentStore{q: q}
}

func (s *MessageAttachmentStore) Create(ctx context.Context, att *domain.MessageAttachment) error {
	return s.q.CreateMessageAttachment(ctx, sqlc.CreateMessageAttachmentParams{
		ID:          att.ID,
		MessageID:   att.MessageID,
		FileName:    att.FileName,
		FileSize:    att.FileSize,
		ContentType: att.ContentType,
		StoragePath: att.StoragePath,
	})
}

func (s *MessageAttachmentStore) ListByMessage(ctx context.Context, messageID string) ([]*domain.MessageAttachment, error) {
	rows, err := s.q.ListMessageAttachments(ctx, messageID)
	if err != nil {
		return nil, err
	}
	items := make([]*domain.MessageAttachment, 0, len(rows))
	for _, r := range rows {
		items = append(items, attToDomain(r))
	}
	return items, nil
}

func (s *MessageAttachmentStore) ListByMessages(ctx context.Context, messageIDs []string) (map[string][]*domain.MessageAttachment, error) {
	if len(messageIDs) == 0 {
		return map[string][]*domain.MessageAttachment{}, nil
	}
	out := make(map[string][]*domain.MessageAttachment, len(messageIDs))
	for _, id := range messageIDs {
		atts, err := s.ListByMessage(ctx, id)
		if err != nil {
			return nil, fmt.Errorf("attachments for %s: %w", id, err)
		}
		out[id] = atts
	}
	return out, nil
}

func (s *MessageAttachmentStore) GetByID(ctx context.Context, id string) (*domain.MessageAttachment, error) {
	row, err := s.q.GetMessageAttachmentByID(ctx, id)
	if err != nil {
		return nil, mapScanErr(err)
	}
	return attToDomain(row), nil
}

// GetByIDAndConversation fetches an attachment by ID, verifying it belongs to
// the given conversation (via the messages join). Returns ErrNotFound when the
// attachment doesn't exist OR doesn't belong to the conversation. Used by the
// download endpoint to prevent cross-conversation IDOR.
func (s *MessageAttachmentStore) GetByIDAndConversation(ctx context.Context, id, conversationID string) (*domain.MessageAttachment, error) {
	row, err := s.q.GetMessageAttachmentByIDAndConversation(ctx, sqlc.GetMessageAttachmentByIDAndConversationParams{
		ID:             id,
		ConversationID: conversationID,
	})
	if err != nil {
		return nil, mapScanErr(err)
	}
	return attToDomain(row), nil
}

func (s *MessageAttachmentStore) Delete(ctx context.Context, id string) error {
	return s.q.DeleteMessageAttachment(ctx, id)
}

func (s *MessageAttachmentStore) UpdateMessageID(ctx context.Context, id, messageID string) error {
	return s.q.UpdateAttachmentMessageID(ctx, sqlc.UpdateAttachmentMessageIDParams{
		ID:        id,
		MessageID: messageID,
	})
}

func attToDomain(r sqlc.MessageAttachment) *domain.MessageAttachment {
	createdAt := parseTime(r.CreatedAt)
	return &domain.MessageAttachment{
		ID:          r.ID,
		MessageID:   r.MessageID,
		FileName:    r.FileName,
		FileSize:    r.FileSize,
		ContentType: r.ContentType,
		StoragePath: r.StoragePath,
		CreatedAt:   createdAt,
	}
}
