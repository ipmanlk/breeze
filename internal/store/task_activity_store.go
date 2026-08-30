package store

import (
	"context"
	"encoding/base64"
	"encoding/json"
	"fmt"

	"ipmanlk/breeze/internal/domain"
	"ipmanlk/breeze/internal/port"
	"ipmanlk/breeze/internal/store/sqlc"
)

type TaskActivityStore struct {
	q *sqlc.Queries
}

func NewTaskActivityStore(q *sqlc.Queries) *TaskActivityStore {
	return &TaskActivityStore{q: q}
}

var _ port.TaskActivityRepository = (*TaskActivityStore)(nil)

// cursorDataActivity is the on-wire cursor payload for pagination.
type cursorDataActivity struct {
	C string `json:"c"` // created_at (SQLite datetime string)
	I string `json:"i"` // id (task_activity row id)
}

func encodeActivityCursor(createdAt, id string) string {
	data, _ := json.Marshal(cursorDataActivity{C: createdAt, I: id})
	return base64.StdEncoding.EncodeToString(data)
}

func decodeActivityCursor(cursor string) (createdAt, id string, err error) {
	data, err := base64.StdEncoding.DecodeString(cursor)
	if err != nil {
		return "", "", fmt.Errorf("invalid cursor: %w", err)
	}
	var c cursorDataActivity
	if err := json.Unmarshal(data, &c); err != nil {
		return "", "", fmt.Errorf("invalid cursor: %w", err)
	}
	return c.C, c.I, nil
}

func (s *TaskActivityStore) Create(ctx context.Context, entry *domain.TaskActivity) error {
	return s.q.CreateTaskActivity(ctx, sqlc.CreateTaskActivityParams{
		ID:        entry.ID,
		TaskID:    entry.TaskID,
		OrgID:     entry.OrgID,
		ProjectID: entry.ProjectID,
		ActorID:   entry.ActorID,
		Action:    string(entry.Action),
		Field:     entry.Field,
		OldValue:  entry.OldValue,
		NewValue:  entry.NewValue,
	})
}

func (s *TaskActivityStore) List(ctx context.Context, taskID string, filter domain.TaskActivityFilter) (*domain.TaskActivityResult, error) {
	limit := filter.Limit
	if limit <= 0 || limit > 100 {
		limit = 20
	}

	var cursorCreatedAt, cursorID string
	if filter.Cursor != "" {
		var err error
		cursorCreatedAt, cursorID, err = decodeActivityCursor(filter.Cursor)
		if err != nil {
			return nil, err
		}
	}

	rows, err := s.q.ListTaskActivity(ctx, sqlc.ListTaskActivityParams{
		TaskID:          taskID,
		CursorCreatedAt: cursorCreatedAt,
		CursorID:        cursorID,
		LimitVal:        int64(limit + 1), // +1 overflow for has_more detection
	})
	if err != nil {
		return nil, err
	}

	hasMore := len(rows) > limit
	if hasMore {
		rows = rows[:limit]
	}

	items := make([]*domain.TaskActivity, len(rows))
	for i, r := range rows {
		items[i] = &domain.TaskActivity{
			ID:         r.ID,
			TaskID:     r.TaskID,
			OrgID:      r.OrgID,
			ProjectID:  r.ProjectID,
			ActorID:    r.ActorID,
			Action:     domain.ActivityAction(r.Action),
			Field:      r.Field,
			OldValue:   r.OldValue,
			NewValue:   r.NewValue,
			CreatedAt:  parseTime(r.CreatedAt),
			ActorName:  r.ActorName,
			ActorEmail: r.ActorEmail,
		}
	}

	result := &domain.TaskActivityResult{
		Items:   items,
		HasMore: hasMore,
	}
	if hasMore && len(rows) > 0 {
		last := rows[len(rows)-1]
		result.NextCursor = encodeActivityCursor(last.CreatedAt, last.ID)
	}

	return result, nil
}
