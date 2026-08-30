package store

import (
	"context"
	"database/sql"

	"ipmanlk/breeze/internal/domain"
	"ipmanlk/breeze/internal/port"
	"ipmanlk/breeze/internal/store/sqlc"
)

type TimeEntryStore struct {
	q  *sqlc.Queries
	db *sql.DB
}

func NewTimeEntryStore(q *sqlc.Queries, db *sql.DB) *TimeEntryStore {
	return &TimeEntryStore{q: q, db: db}
}

var _ port.TimeEntryRepository = (*TimeEntryStore)(nil)

func (s *TimeEntryStore) toDomain(te sqlc.TimeEntry) domain.TimeEntry {
	var dur *int
	if te.DurationMinutes != nil {
		v := int(*te.DurationMinutes)
		dur = &v
	}
	return domain.TimeEntry{
		ID:              te.ID,
		TaskID:          te.TaskID,
		UserID:          te.UserID,
		Description:     te.Description,
		StartedAt:       parseTime(te.StartedAt),
		EndedAt:         parseTimePtr(te.EndedAt),
		DurationMinutes: dur,
		CreatedAt:       parseTime(te.CreatedAt),
		UpdatedAt:       parseTime(te.UpdatedAt),
	}
}

func (s *TimeEntryStore) ListByTask(ctx context.Context, taskID string) ([]*domain.TimeEntry, error) {
	rows, err := s.q.ListTimeEntries(ctx, taskID)
	if err != nil {
		return nil, err
	}
	entries := make([]*domain.TimeEntry, len(rows))
	for i, row := range rows {
		d := s.toDomain(row)
		entries[i] = &d
	}
	return entries, nil
}

func (s *TimeEntryStore) GetActiveTimer(ctx context.Context, taskID, userID string) (*domain.TimeEntry, error) {
	te, err := s.q.GetActiveTimer(ctx, sqlc.GetActiveTimerParams{TaskID: taskID, UserID: userID})
	if err != nil {
		return nil, err
	}
	d := s.toDomain(te)
	return &d, nil
}

func (s *TimeEntryStore) GetActiveTimerByUser(ctx context.Context, userID string) ([]*domain.TimeEntry, error) {
	rows, err := s.q.GetActiveTimerByUser(ctx, userID)
	if err != nil {
		return nil, err
	}
	entries := make([]*domain.TimeEntry, len(rows))
	for i, row := range rows {
		d := s.toDomain(row)
		entries[i] = &d
	}
	return entries, nil
}

func (s *TimeEntryStore) StartTimer(ctx context.Context, id, taskID, userID, description string) error {
	return s.q.StartTimer(ctx, sqlc.StartTimerParams{
		ID:          id,
		TaskID:      taskID,
		UserID:      userID,
		Description: description,
	})
}

// StartTimerAtomic stops any active timer for the user and starts a new one
// in a single transaction. Combined with the partial unique index
// idx_time_entries_active_user (see migration 00040), this ensures a user
// can never have more than one active timer even under concurrent requests.
func (s *TimeEntryStore) StartTimerAtomic(ctx context.Context, id, taskID, userID, description string) error {
	tx, err := s.db.BeginTx(ctx, nil)
	if err != nil {
		return err
	}
	defer tx.Rollback()

	q := s.q.WithTx(tx)

	// Stop any active timer(s) for this user.
	if err := q.StopActiveTimersForUser(ctx, userID); err != nil {
		return err
	}

	// Start the new timer.
	if err := q.StartTimer(ctx, sqlc.StartTimerParams{
		ID:          id,
		TaskID:      taskID,
		UserID:      userID,
		Description: description,
	}); err != nil {
		return err
	}

	return tx.Commit()
}

func (s *TimeEntryStore) StopTimer(ctx context.Context, id, userID string) error {
	return s.q.StopTimer(ctx, sqlc.StopTimerParams{ID: id, UserID: userID})
}

func (s *TimeEntryStore) Create(ctx context.Context, entry *domain.TimeEntry) error {
	var dm *int64
	if entry.DurationMinutes != nil {
		v := int64(*entry.DurationMinutes)
		dm = &v
	}
	return s.q.CreateTimeEntry(ctx, sqlc.CreateTimeEntryParams{
		ID:              entry.ID,
		TaskID:          entry.TaskID,
		UserID:          entry.UserID,
		Description:     entry.Description,
		StartedAt:       formatTime(entry.StartedAt),
		EndedAt:         formatTimePtr(entry.EndedAt),
		DurationMinutes: dm,
	})
}

func (s *TimeEntryStore) Update(ctx context.Context, entry *domain.TimeEntry) error {
	var dm *int64
	if entry.DurationMinutes != nil {
		v := int64(*entry.DurationMinutes)
		dm = &v
	}
	return s.q.UpdateTimeEntry(ctx, sqlc.UpdateTimeEntryParams{
		Description:     entry.Description,
		StartedAt:       formatTime(entry.StartedAt),
		EndedAt:         formatTimePtr(entry.EndedAt),
		DurationMinutes: dm,
		ID:              entry.ID,
		TaskID:          entry.TaskID,
	})
}

func (s *TimeEntryStore) Delete(ctx context.Context, id, taskID string) error {
	return s.q.DeleteTimeEntry(ctx, sqlc.DeleteTimeEntryParams{ID: id, TaskID: taskID})
}

func (s *TimeEntryStore) TotalTimeByTask(ctx context.Context, taskID string) (int64, error) {
	v, err := s.q.TotalTimeByTask(ctx, taskID)
	if err != nil {
		return 0, err
	}
	if t, ok := v.(int64); ok {
		return t, nil
	}
	return 0, nil
}
