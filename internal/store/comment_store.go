package store

import (
	"context"
	"time"

	"ipmanlk/breeze/internal/domain"
	"ipmanlk/breeze/internal/port"
	"ipmanlk/breeze/internal/store/sqlc"
)

type CommentStore struct {
	q *sqlc.Queries
}

func NewCommentStore(q *sqlc.Queries) *CommentStore {
	return &CommentStore{q: q}
}

var _ port.CommentRepository = (*CommentStore)(nil)

func (s *CommentStore) GetByID(ctx context.Context, orgID, id string) (*domain.Comment, error) {
	c, err := s.q.GetCommentByID(ctx, sqlc.GetCommentByIDParams{ID: id, OrgID: orgID})
	if err != nil {
		return nil, mapScanErr(err)
	}
	return commentByIDRowToDomain(c), nil
}

func (s *CommentStore) ListByTask(ctx context.Context, filter domain.CommentFilter) (*domain.CommentListResult, error) {
	limit := filter.Limit
	if limit <= 0 || limit > 100 {
		limit = 50
	}
	rows, err := s.q.ListCommentsByTask(ctx, sqlc.ListCommentsByTaskParams{
		TaskID:       filter.TaskID,
		OrgID:        filter.OrgID,
		BeforeCursor: filter.BeforeCursor,
		Limit:        int64(limit),
	})
	if err != nil {
		return nil, err
	}
	items := make([]*domain.Comment, len(rows))
	for i, r := range rows {
		items[i] = commentListRowToDomain(r)
	}
	// Rows are DESC (newest first). Reverse to ASC for display.
	for i, j := 0, len(items)-1; i < j; i, j = i+1, j-1 {
		items[i], items[j] = items[j], items[i]
	}
	result := &domain.CommentListResult{Items: items}
	if len(items) == limit {
		result.HasMore = true
		result.NextCursor = items[0].CreatedAt.Format(time.RFC3339Nano)
	}
	return result, nil
}

func (s *CommentStore) Create(ctx context.Context, comment *domain.Comment) error {
	return s.q.CreateComment(ctx, sqlc.CreateCommentParams{
		ID:        comment.ID,
		OrgID:     comment.OrgID,
		TaskID:    comment.TaskID,
		ProjectID: comment.ProjectID,
		AuthorID:  comment.AuthorID,
		Content:   comment.Content,
		ParentID:  comment.ParentID,
	})
}

func (s *CommentStore) Update(ctx context.Context, comment *domain.Comment) error {
	return s.q.UpdateComment(ctx, sqlc.UpdateCommentParams{
		Content: comment.Content,
		ID:      comment.ID,
		OrgID:   comment.OrgID,
	})
}

func (s *CommentStore) SoftDelete(ctx context.Context, orgID, id string) error {
	return s.q.SoftDeleteComment(ctx, sqlc.SoftDeleteCommentParams{ID: id, OrgID: orgID})
}

func commentByIDRowToDomain(c sqlc.GetCommentByIDRow) *domain.Comment {
	return &domain.Comment{
		ID:              c.ID,
		OrgID:           c.OrgID,
		TaskID:          c.TaskID,
		ProjectID:       c.ProjectID,
		AuthorID:        c.AuthorID,
		Content:         c.Content,
		ParentID:        c.ParentID,
		CreatedAt:       parseTime(c.CreatedAt),
		UpdatedAt:       parseTime(c.UpdatedAt),
		EditedAt:        parseTimePtr(c.EditedAt),
		DeletedAt:       parseTimePtr(c.DeletedAt),
		AuthorName:      c.AuthorName,
		AuthorEmail:     c.AuthorEmail,
		AuthorAvatarURL: c.AuthorAvatarUrl,
	}
}

func commentListRowToDomain(r sqlc.ListCommentsByTaskRow) *domain.Comment {
	return &domain.Comment{
		ID:              r.ID,
		OrgID:           r.OrgID,
		TaskID:          r.TaskID,
		ProjectID:       r.ProjectID,
		AuthorID:        r.AuthorID,
		Content:         r.Content,
		ParentID:        r.ParentID,
		CreatedAt:       parseTime(r.CreatedAt),
		UpdatedAt:       parseTime(r.UpdatedAt),
		EditedAt:        parseTimePtr(r.EditedAt),
		DeletedAt:       parseTimePtr(r.DeletedAt),
		AuthorName:      r.AuthorName,
		AuthorEmail:     r.AuthorEmail,
		AuthorAvatarURL: r.AuthorAvatarUrl,
	}
}
