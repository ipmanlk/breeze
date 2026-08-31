package store

import (
	"context"
	"database/sql"
	"encoding/base64"
	"encoding/json"
	"fmt"
	"time"

	"ipmanlk/plume/internal/domain"
	"ipmanlk/plume/internal/lexorank"
	"ipmanlk/plume/internal/port"
	"ipmanlk/plume/internal/store/sqlc"
)

type TaskStore struct {
	q  *sqlc.Queries
	db *sql.DB
}

func NewTaskStore(q *sqlc.Queries, db *sql.DB) *TaskStore {
	return &TaskStore{q: q, db: db}
}

var _ port.TaskRepository = (*TaskStore)(nil)

type taskRow struct {
	ID              string
	OrgID           string
	ProjectID       string
	CycleID         *string
	ParentTaskID    *string
	CreatedBy       string
	Title           string
	Description     string
	StatusID        string
	Priority        string
	PositionKey     string
	SubtaskPosition string
	Estimate        *int64
	StartedAt       *string
	DueAt           *string
	CompletedAt     *string
	TemplateID      *string
	CreatedAt       string
	UpdatedAt       string

	// SubtaskCount / CompletedSubtaskCount are populated by list/get queries
	// via correlated subqueries. Zero when not populated (e.g. ListSubtasks).
	SubtaskCount          int64
	CompletedSubtaskCount int64
	// ParentTitle is the parent task's title, populated by GetTaskByID via a
	// LEFT JOIN. nil when the task is top-level.
	ParentTitle *string
}

func taskRowFromList(r sqlc.ListTasksByProjectRow) taskRow {
	return taskRow{
		ID: r.ID, OrgID: r.OrgID, ProjectID: r.ProjectID,
		CycleID: r.CycleID, ParentTaskID: r.ParentTaskID, CreatedBy: r.CreatedBy,
		Title: r.Title, Description: r.Description,
		StatusID: r.StatusID, Priority: r.Priority, PositionKey: r.PositionKey,
		Estimate: r.Estimate, StartedAt: r.StartedAt, DueAt: r.DueAt,
		CompletedAt: r.CompletedAt, TemplateID: r.TemplateID,
		CreatedAt: r.CreatedAt, UpdatedAt: r.UpdatedAt,
		SubtaskCount:          r.SubtaskCount,
		CompletedSubtaskCount: r.CompletedSubtaskCount,
	}
}

func taskRowFromListSubtasks(r sqlc.ListSubtasksRow) taskRow {
	return taskRow{
		ID: r.ID, OrgID: r.OrgID, ProjectID: r.ProjectID,
		CycleID: r.CycleID, ParentTaskID: r.ParentTaskID, CreatedBy: r.CreatedBy,
		Title: r.Title, Description: r.Description,
		StatusID: r.StatusID, Priority: r.Priority, PositionKey: r.PositionKey,
		SubtaskPosition: derefStr(r.SubtaskPosition),
		Estimate:        r.Estimate, StartedAt: r.StartedAt, DueAt: r.DueAt,
		CompletedAt: r.CompletedAt, TemplateID: r.TemplateID,
		CreatedAt: r.CreatedAt, UpdatedAt: r.UpdatedAt,
	}
}

func taskRowFromGet(r sqlc.GetTaskByIDRow) taskRow {
	return taskRow{
		ID: r.ID, OrgID: r.OrgID, ProjectID: r.ProjectID,
		CycleID: r.CycleID, ParentTaskID: r.ParentTaskID, CreatedBy: r.CreatedBy,
		Title: r.Title, Description: r.Description,
		StatusID: r.StatusID, Priority: r.Priority, PositionKey: r.PositionKey,
		SubtaskPosition: derefStr(r.SubtaskPosition),
		Estimate:        r.Estimate, StartedAt: r.StartedAt, DueAt: r.DueAt,
		CompletedAt: r.CompletedAt, TemplateID: r.TemplateID,
		CreatedAt: r.CreatedAt, UpdatedAt: r.UpdatedAt,
		SubtaskCount:          r.SubtaskCount,
		CompletedSubtaskCount: r.CompletedSubtaskCount,
		ParentTitle:           r.ParentTitle,
	}
}

func taskRowFromGetByOrg(r sqlc.GetTaskByIDAndOrgRow) taskRow {
	return taskRow{
		ID: r.ID, OrgID: r.OrgID, ProjectID: r.ProjectID,
		CycleID: r.CycleID, ParentTaskID: r.ParentTaskID, CreatedBy: r.CreatedBy,
		Title: r.Title, Description: r.Description,
		StatusID: r.StatusID, Priority: r.Priority, PositionKey: r.PositionKey,
		Estimate: r.Estimate, StartedAt: r.StartedAt, DueAt: r.DueAt,
		CompletedAt: r.CompletedAt, TemplateID: r.TemplateID,
		CreatedAt: r.CreatedAt, UpdatedAt: r.UpdatedAt,
	}
}

func taskRowFromGetByIDsFull(r sqlc.Task) taskRow {
	return taskRow{
		ID: r.ID, OrgID: r.OrgID, ProjectID: r.ProjectID,
		CycleID: r.CycleID, ParentTaskID: r.ParentTaskID, CreatedBy: r.CreatedBy,
		Title: r.Title, Description: r.Description,
		StatusID: r.StatusID, Priority: r.Priority, PositionKey: r.PositionKey,
		SubtaskPosition: derefStr(r.SubtaskPosition),
		Estimate:        r.Estimate, StartedAt: r.StartedAt, DueAt: r.DueAt,
		CompletedAt: r.CompletedAt, TemplateID: r.TemplateID,
		CreatedAt: r.CreatedAt, UpdatedAt: r.UpdatedAt,
	}
}

func taskRowFromUserList(r sqlc.ListTasksByUserRow) taskRow {
	return taskRow{
		ID: r.ID, OrgID: r.OrgID, ProjectID: r.ProjectID,
		CycleID: r.CycleID, ParentTaskID: r.ParentTaskID, CreatedBy: r.CreatedBy,
		Title: r.Title, Description: r.Description,
		StatusID: r.StatusID, Priority: r.Priority, PositionKey: r.PositionKey,
		Estimate: r.Estimate, StartedAt: r.StartedAt, DueAt: r.DueAt,
		CompletedAt: r.CompletedAt, TemplateID: r.TemplateID,
		CreatedAt: r.CreatedAt, UpdatedAt: r.UpdatedAt,
	}
}

func (s *TaskStore) toDomain(r taskRow) domain.Task {
	return domain.Task{
		ID:                    r.ID,
		OrgID:                 r.OrgID,
		ProjectID:             r.ProjectID,
		CycleID:               r.CycleID,
		ParentID:              r.ParentTaskID,
		CreatedBy:             r.CreatedBy,
		Title:                 r.Title,
		Description:           r.Description,
		StatusID:              r.StatusID,
		Priority:              r.Priority,
		PositionKey:           r.PositionKey,
		SubtaskPosition:       r.SubtaskPosition,
		Estimate:              intPtr(r.Estimate),
		StartedAt:             parseTimePtr(r.StartedAt),
		DueAt:                 parseTimePtr(r.DueAt),
		CompletedAt:           parseTimePtr(r.CompletedAt),
		TemplateID:            r.TemplateID,
		CreatedAt:             parseTime(r.CreatedAt),
		UpdatedAt:             parseTime(r.UpdatedAt),
		SubtaskCount:          int(r.SubtaskCount),
		CompletedSubtaskCount: int(r.CompletedSubtaskCount),
		ParentTitle:           derefStr(r.ParentTitle),
	}
}

func (s *TaskStore) loadAssigneesBatch(ctx context.Context, taskIDs []string) (map[string][]domain.TaskAssignee, error) {
	if len(taskIDs) == 0 {
		return nil, nil
	}
	rows, err := s.q.ListAssigneesByTaskIDs(ctx, taskIDs)
	if err != nil {
		return nil, err
	}
	result := make(map[string][]domain.TaskAssignee)
	for _, row := range rows {
		result[row.TaskID] = append(result[row.TaskID], domain.TaskAssignee{
			ID:        row.ID,
			Name:      row.Name,
			Email:     row.Email,
			AvatarURL: row.AvatarUrl,
		})
	}
	return result, nil
}

// loadLabelsBatch fetches labels for many tasks in one query and groups them
// by task ID. Mirrors loadAssigneesBatch so list endpoints avoid N+1 queries.
func (s *TaskStore) loadLabelsBatch(ctx context.Context, taskIDs []string) (map[string][]domain.Label, error) {
	if len(taskIDs) == 0 {
		return nil, nil
	}
	rows, err := s.q.ListLabelsByTaskIDs(ctx, taskIDs)
	if err != nil {
		return nil, err
	}
	result := make(map[string][]domain.Label)
	for _, row := range rows {
		result[row.TaskID] = append(result[row.TaskID], domain.Label{
			ID:    row.ID,
			OrgID: row.OrgID,
			Name:  row.Name,
			Color: row.Color,
		})
	}
	return result, nil
}

func (s *TaskStore) ListByProject(ctx context.Context, orgID, projectID string, filter domain.TaskFilter) ([]*domain.Task, error) {
	// instr() does a literal substring match (no LIKE wildcards), so pass the
	// raw search term. Empty search is handled in SQL via @search = '%%'.
	search := filter.Search
	if search == "" {
		search = "%%"
	}
	// sqlc.narg renders as a nullable param; pass 1 to enable the label
	// EXISTS clause, 0 to skip it entirely (avoids `IN (NULL)` always-false
	// when no label filter is requested).
	hasLabelFilter := 0
	var labelIDs []string
	if len(filter.LabelIDs) > 0 {
		hasLabelFilter = 1
		labelIDs = filter.LabelIDs
	}
	rows, err := s.q.ListTasksByProject(ctx, sqlc.ListTasksByProjectParams{
		ProjectID:       projectID,
		OrgID:           orgID,
		IncludeSubtasks: boolToInt64(filter.IncludeSubtasks),
		StatusID:        filter.StatusID,
		CycleID:         filter.CycleID,
		AssigneeID:      filter.AssigneeID,
		Priority:        filter.Priority,
		Search:          search,
		HasLabelFilter:  hasLabelFilter,
		LabelIds:        labelIDs,
	})
	if err != nil {
		return nil, err
	}

	taskIDs := make([]string, len(rows))
	for i, row := range rows {
		taskIDs[i] = row.ID
	}

	assigneeMap, err := s.loadAssigneesBatch(ctx, taskIDs)
	if err != nil {
		return nil, err
	}
	labelMap, err := s.loadLabelsBatch(ctx, taskIDs)
	if err != nil {
		return nil, err
	}

	tasks := make([]*domain.Task, len(rows))
	for i, row := range rows {
		d := s.toDomain(taskRowFromList(row))
		d.Assignees = assigneeMap[row.ID]
		d.Labels = labelMap[row.ID]
		tasks[i] = &d
	}
	return tasks, nil
}

func (s *TaskStore) GetByID(ctx context.Context, orgID, id, projectID string) (*domain.Task, error) {
	t, err := s.q.GetTaskByID(ctx, sqlc.GetTaskByIDParams{ID: id, ProjectID: projectID, OrgID: orgID})
	if err != nil {
		return nil, mapScanErr(err)
	}
	d := s.toDomain(taskRowFromGet(t))

	assigneeMap, err := s.loadAssigneesBatch(ctx, []string{t.ID})
	if err != nil {
		return nil, err
	}
	d.Assignees = assigneeMap[t.ID]
	labelMap, err := s.loadLabelsBatch(ctx, []string{t.ID})
	if err != nil {
		return nil, err
	}
	d.Labels = labelMap[t.ID]
	return &d, nil
}

func (s *TaskStore) ListSubtasks(ctx context.Context, orgID, parentID string) ([]*domain.Task, error) {
	rows, err := s.q.ListSubtasks(ctx, sqlc.ListSubtasksParams{ParentTaskID: &parentID, OrgID: orgID})
	if err != nil {
		return nil, mapScanErr(err)
	}
	taskIDs := make([]string, len(rows))
	for i, r := range rows {
		taskIDs[i] = r.ID
	}
	assigneeMap, err := s.loadAssigneesBatch(ctx, taskIDs)
	if err != nil {
		return nil, err
	}
	labelMap, err := s.loadLabelsBatch(ctx, taskIDs)
	if err != nil {
		return nil, err
	}
	tasks := make([]*domain.Task, 0, len(rows))
	for _, r := range rows {
		d := s.toDomain(taskRowFromListSubtasks(r))
		d.Assignees = assigneeMap[r.ID]
		d.Labels = labelMap[r.ID]
		tasks = append(tasks, &d)
	}
	return tasks, nil
}

func (s *TaskStore) GetByIDAndOrg(ctx context.Context, orgID, id string) (*domain.Task, error) {
	t, err := s.q.GetTaskByIDAndOrg(ctx, sqlc.GetTaskByIDAndOrgParams{ID: id, OrgID: orgID})
	if err != nil {
		return nil, mapScanErr(err)
	}
	d := s.toDomain(taskRowFromGetByOrg(t))

	assigneeMap, err := s.loadAssigneesBatch(ctx, []string{t.ID})
	if err != nil {
		return nil, err
	}
	d.Assignees = assigneeMap[t.ID]
	labelMap, err := s.loadLabelsBatch(ctx, []string{t.ID})
	if err != nil {
		return nil, err
	}
	d.Labels = labelMap[t.ID]
	return &d, nil
}

func (s *TaskStore) Create(ctx context.Context, t *domain.Task) error {
	ids := make([]string, len(t.Assignees))
	for i, a := range t.Assignees {
		ids[i] = a.ID
	}

	q, commit, rollback, err := s.txScope(ctx)
	if err != nil {
		return err
	}
	defer rollback()

	if err := q.CreateTask(ctx, sqlc.CreateTaskParams{
		ID:              t.ID,
		OrgID:           t.OrgID,
		ProjectID:       t.ProjectID,
		CycleID:         t.CycleID,
		ParentTaskID:    t.ParentID,
		SubtaskPosition: nilIfEmpty(t.SubtaskPosition),
		CreatedBy:       t.CreatedBy,
		Title:           t.Title,
		Description:     t.Description,
		StatusID:        t.StatusID,
		Priority:        t.Priority,
		PositionKey:     t.PositionKey,
		Estimate:        int64Ptr(t.Estimate),
		StartedAt:       formatTimePtr(t.StartedAt),
		DueAt:           formatTimePtr(t.DueAt),
		CompletedAt:     formatTimePtr(t.CompletedAt),
		TemplateID:      t.TemplateID,
	}); err != nil {
		return err
	}
	if err := s.setAssigneesWithTx(ctx, q, t.ID, ids); err != nil {
		return err
	}
	return commit()
}

func (s *TaskStore) Update(ctx context.Context, t *domain.Task) error {
	updatedAt := t.UpdatedAt
	if updatedAt.IsZero() {
		updatedAt = time.Now()
	}
	ids := make([]string, len(t.Assignees))
	for i, a := range t.Assignees {
		ids[i] = a.ID
	}

	q, commit, rollback, err := s.txScope(ctx)
	if err != nil {
		return err
	}
	defer rollback()

	if err := q.UpdateTask(ctx, sqlc.UpdateTaskParams{
		Title:           t.Title,
		Description:     t.Description,
		StatusID:        t.StatusID,
		Priority:        t.Priority,
		CycleID:         t.CycleID,
		ParentTaskID:    t.ParentID,
		Estimate:        int64Ptr(t.Estimate),
		StartedAt:       formatTimePtr(t.StartedAt),
		DueAt:           formatTimePtr(t.DueAt),
		CompletedAt:     formatTimePtr(t.CompletedAt),
		PositionKey:     t.PositionKey,
		SubtaskPosition: nilIfEmpty(t.SubtaskPosition),
		UpdatedAt:       formatTime(updatedAt),
		ID:              t.ID,
		ProjectID:       t.ProjectID,
		OrgID:           t.OrgID,
	}); err != nil {
		return err
	}
	if err := s.setAssigneesWithTx(ctx, q, t.ID, ids); err != nil {
		return err
	}
	return commit()
}

func (s *TaskStore) ListAssigneesByTaskIDs(ctx context.Context, taskIDs []string) (map[string][]domain.TaskAssignee, error) {
	return s.loadAssigneesBatch(ctx, taskIDs)
}

// setAssigneesWithTx is the transactional core of SetAssignees. It takes a
// tx-scoped *sqlc.Queries and performs the delete-then-insert dance without
// creating a new transaction. Callers that already hold a transaction (e.g.
// Update/Create) pass their tx-scoped q here to keep all mutations atomic.
func (s *TaskStore) setAssigneesWithTx(ctx context.Context, q *sqlc.Queries, taskID string, userIDs []string) error {
	if err := q.SetTaskAssignees(ctx, taskID); err != nil {
		return err
	}
	for _, uid := range userIDs {
		if err := q.AddTaskAssignee(ctx, sqlc.AddTaskAssigneeParams{TaskID: taskID, UserID: uid}); err != nil {
			return err
		}
	}
	return nil
}

func (s *TaskStore) SetAssignees(ctx context.Context, taskID string, userIDs []string) error {
	q, commit, rollback, err := s.txScope(ctx)
	if err != nil {
		return err
	}
	defer rollback()

	if err := s.setAssigneesWithTx(ctx, q, taskID, userIDs); err != nil {
		return err
	}
	return commit()
}

func (s *TaskStore) Move(ctx context.Context, orgID, id, projectID, statusID, positionKey string) error {
	return s.q.UpdateTaskPosition(ctx, sqlc.UpdateTaskPositionParams{
		StatusID:    statusID,
		PositionKey: positionKey,
		ID:          id,
		ProjectID:   projectID,
		OrgID:       orgID,
	})
}

// MoveToProject re-homes a task into a different project + status, clearing
// its cycle + parent (both are project-scoped and may not exist in the
// target). Caller must generate a fresh position key for the target status.
func (s *TaskStore) MoveToProject(ctx context.Context, orgID, id, fromProjectID, toProjectID, toStatusID, positionKey string, completedAt *time.Time) error {
	return s.q.MoveTaskToProject(ctx, sqlc.MoveTaskToProjectParams{
		ProjectID:   toProjectID,
		StatusID:    toStatusID,
		PositionKey: positionKey,
		CompletedAt: formatTimePtr(completedAt),
		ID:          id,
		ProjectID_2: fromProjectID,
		OrgID:       orgID,
	})
}

func (s *TaskStore) Delete(ctx context.Context, orgID, id, projectID string) error {
	return s.q.DeleteTask(ctx, sqlc.DeleteTaskParams{ID: id, ProjectID: projectID, OrgID: orgID})
}

// DeleteSubtasks removes all direct children of a task. The service enforces
// 1-level nesting, so no recursion is needed.
func (s *TaskStore) DeleteSubtasks(ctx context.Context, orgID, parentID string) error {
	return s.q.DeleteSubtasksByParent(ctx, sqlc.DeleteSubtasksByParentParams{ParentTaskID: &parentID, OrgID: orgID})
}

// PromoteSubtasks clears parent_task_id on all direct children so they become
// top-level tasks. Used by the promote delete mode.
func (s *TaskStore) PromoteSubtasks(ctx context.Context, orgID, parentID string) error {
	return s.q.PromoteSubtasksByParent(ctx, sqlc.PromoteSubtasksByParentParams{ParentTaskID: &parentID, OrgID: orgID})
}

// CountSubtasks returns the number of direct children of a task.
func (s *TaskStore) CountSubtasks(ctx context.Context, orgID, parentID string) (int64, error) {
	return s.q.CountSubtasksByParent(ctx, sqlc.CountSubtasksByParentParams{ParentTaskID: &parentID, OrgID: orgID})
}

// GetLastSubtaskPosition returns the highest subtask_position among a task's
// children (lexorank), or "" if the parent has no children. Used to generate
// a new key at the end when creating a subtask.
func (s *TaskStore) GetLastSubtaskPosition(ctx context.Context, orgID, parentID string) (string, error) {
	v, err := s.q.GetLastSubtaskPosition(ctx, sqlc.GetLastSubtaskPositionParams{ParentTaskID: &parentID, OrgID: orgID})
	if err != nil {
		return "", err
	}
	if v == nil {
		return "", nil
	}
	sval, ok := v.(string)
	if !ok {
		return "", nil
	}
	return sval, nil
}

// GenerateSubtaskPositionKey atomically reads the last subtask_position for
// the given parent and generates the next lexorank key (appends at end).
// Transaction-wrapped to prevent the lexorank race on concurrent subtask
// creation; the same race as the project-wide position key.
func (s *TaskStore) GenerateSubtaskPositionKey(ctx context.Context, orgID, parentID string) (string, error) {
	q, commit, rollback, err := s.txScope(ctx)
	if err != nil {
		return "", err
	}
	defer rollback()

	v, err := q.GetLastSubtaskPosition(ctx, sqlc.GetLastSubtaskPositionParams{ParentTaskID: &parentID, OrgID: orgID})
	if err != nil {
		return "", err
	}
	lastKey := ""
	if v != nil {
		if sval, ok := v.(string); ok {
			lastKey = sval
		}
	}
	pk, err := lexorank.GenerateKeyBetween(lastKey, "")
	if err != nil {
		return "", err
	}
	return pk, commit()
}

// ReorderSubtasks re-keys the subtask_position of each child of parentID to
// the given position keys. Each op is {taskID, positionKey}. The caller must
// generate valid lexorank keys.
func (s *TaskStore) ReorderSubtasks(ctx context.Context, orgID, parentID string, ops []domain.ReorderOp) error {
	q, commit, rollback, err := s.txScope(ctx)
	if err != nil {
		return err
	}
	defer rollback()

	for _, op := range ops {
		if err := q.UpdateSubtaskPosition(ctx, sqlc.UpdateSubtaskPositionParams{
			SubtaskPosition: &op.PositionKey,
			ID:              op.TaskID,
			ParentTaskID:    &parentID,
			OrgID:           orgID,
		}); err != nil {
			return err
		}
	}
	return commit()
}

func (s *TaskStore) GetLastPositionKey(ctx context.Context, orgID, projectID, statusID string) (string, error) {
	v, err := s.q.GetLastPositionKey(ctx, sqlc.GetLastPositionKeyParams{
		ProjectID: projectID,
		StatusID:  statusID,
		OrgID:     orgID,
	})
	if err != nil {
		return "", err
	}
	if v == nil {
		return "", nil
	}
	sval, ok := v.(string)
	if !ok {
		return "", nil
	}
	return sval, nil
}

// GeneratePositionKey atomically reads the last position key for the given
// (org, project, status) scope and generates the next lexorank key (appends
// at the end). The read+generate is wrapped in a transaction so that
// concurrent callers cannot both read the same lastKey and produce a
// duplicate position_key. With SQLite's single-writer connection
// (SetMaxOpenConns(1)), BeginTx serializes the read against other writers,
// closing the lexorank race.
func (s *TaskStore) GeneratePositionKey(ctx context.Context, orgID, projectID, statusID string) (string, error) {
	q, commit, rollback, err := s.txScope(ctx)
	if err != nil {
		return "", err
	}
	defer rollback()

	v, err := q.GetLastPositionKey(ctx, sqlc.GetLastPositionKeyParams{
		ProjectID: projectID,
		StatusID:  statusID,
		OrgID:     orgID,
	})
	if err != nil {
		return "", err
	}
	lastKey := ""
	if v != nil {
		if sval, ok := v.(string); ok {
			lastKey = sval
		}
	}
	pk, err := lexorank.GenerateKeyBetween(lastKey, "")
	if err != nil {
		return "", err
	}
	return pk, commit()
}

// RunInTransaction runs fn against a transaction-scoped TaskRepository.
// It creates a tx-backed TaskStore whose db is nil (so its mutation methods
// reuse the open transaction instead of starting a nested one; avoids
// deadlock with SetMaxOpenConns(1)) and whose q is tx-scoped. If fn returns
// nil, the tx commits; otherwise it rolls back. This lets BatchUpdate make
// multi-task mutations atomic while respecting the layer boundary: the
// service drives the logic but the store owns the transaction.
func (s *TaskStore) RunInTransaction(ctx context.Context, fn func(port.TaskRepository) error) error {
	if s.db == nil {
		// Already inside a transaction: delegate to the caller's tx by
		// running fn against this store's tx-scoped q. No commit here.
		return fn(s)
	}
	tx, err := s.db.BeginTx(ctx, nil)
	if err != nil {
		return err
	}
	defer tx.Rollback()

	txStore := &TaskStore{q: s.q.WithTx(tx), db: nil}
	if err := fn(txStore); err != nil {
		return err
	}
	return tx.Commit()
}

// txScope returns a tx-scoped *sqlc.Queries plus commit/rollback functions.
// When the store already holds a transaction (db == nil, set by
// RunInTransaction), it reuses the existing tx-scoped q and returns no-op
// commit/rollback (the outer RunInTransaction owns the lifecycle).
// Otherwise it begins a new transaction. This avoids nested BeginTx calls
// that would deadlock under SetMaxOpenConns(1).
func (s *TaskStore) txScope(ctx context.Context) (q *sqlc.Queries, commit, rollback func() error, err error) {
	if s.db == nil {
		// Already in a transaction: reuse the tx-scoped queries.
		return s.q, func() error { return nil }, func() error { return nil }, nil
	}
	tx, err := s.db.BeginTx(ctx, nil)
	if err != nil {
		return nil, nil, nil, err
	}
	return s.q.WithTx(tx), tx.Commit, tx.Rollback, nil
}

func (s *TaskStore) GetPositionKeyNeighbors(ctx context.Context, orgID, taskID string) (prev, next string, err error) {
	result, err := s.q.GetPositionKeyNeighbors(ctx, sqlc.GetPositionKeyNeighborsParams{ID: taskID, OrgID: orgID})
	if err != nil {
		return "", "", err
	}
	prev = ""
	if result.PrevKey != nil {
		prev, _ = result.PrevKey.(string)
	}
	next = ""
	if result.NextKey != nil {
		next, _ = result.NextKey.(string)
	}
	return prev, next, nil
}

func (s *TaskStore) InsertAtEnd(ctx context.Context, orgID, projectID, statusID string) (string, error) {
	lastKey, err := s.GetLastPositionKey(ctx, orgID, projectID, statusID)
	if err != nil {
		return "", err
	}
	newKey, err := lexorank.GenerateKeyBetween(lastKey, "")
	if err != nil {
		return "", err
	}
	return newKey, nil
}

func (s *TaskStore) UnassignCycleFromTasks(ctx context.Context, orgID, cycleID string) error {
	return s.q.UnassignCycleFromTasks(ctx, sqlc.UnassignCycleFromTasksParams{CycleID: &cycleID, OrgID: orgID})
}

func (s *TaskStore) MoveTasksToCycle(ctx context.Context, orgID, fromCycleID, toCycleID string) error {
	return s.q.MoveTasksToCycle(ctx, sqlc.MoveTasksToCycleParams{
		ToCycleID:   &toCycleID,
		FromCycleID: &fromCycleID,
		OrgID:       orgID,
	})
}

func (s *TaskStore) MoveIncompleteTasksToCycle(ctx context.Context, orgID, fromCycleID, toCycleID string) error {
	return s.q.MoveIncompleteTasksToCycle(ctx, sqlc.MoveIncompleteTasksToCycleParams{
		ToCycleID:   &toCycleID,
		FromCycleID: &fromCycleID,
		OrgID:       orgID,
	})
}

func (s *TaskStore) UnassignCycleFromIncompleteTasks(ctx context.Context, orgID, cycleID string) error {
	return s.q.UnassignCycleFromIncompleteTasks(ctx, sqlc.UnassignCycleFromIncompleteTasksParams{CycleID: &cycleID, OrgID: orgID})
}

func (s *TaskStore) ListByCycle(ctx context.Context, orgID, cycleID string) ([]*domain.Task, error) {
	rows, err := s.q.ListTasksByCycle(ctx, sqlc.ListTasksByCycleParams{CycleID: &cycleID, OrgID: orgID})
	if err != nil {
		return nil, err
	}
	tasks := make([]*domain.Task, len(rows))
	for i, row := range rows {
		d := s.toDomain(taskRow{
			ID: row.ID, OrgID: row.OrgID, ProjectID: row.ProjectID,
			CycleID: row.CycleID, ParentTaskID: row.ParentTaskID, CreatedBy: row.CreatedBy,
			Title: row.Title, Description: row.Description,
			StatusID: row.StatusID, Priority: row.Priority, PositionKey: row.PositionKey,
			Estimate: row.Estimate, StartedAt: row.StartedAt, DueAt: row.DueAt,
			CompletedAt: row.CompletedAt, CreatedAt: row.CreatedAt, UpdatedAt: row.UpdatedAt,
		})
		tasks[i] = &d
	}
	return tasks, nil
}

func (s *TaskStore) ListByIDs(ctx context.Context, orgID string, ids []string) ([]*domain.Task, error) {
	if len(ids) == 0 {
		return nil, nil
	}
	rows, err := s.q.GetTasksByIDs(ctx, sqlc.GetTasksByIDsParams{Ids: ids, OrgID: orgID})
	if err != nil {
		return nil, err
	}
	tasks := make([]*domain.Task, 0, len(rows))
	for _, row := range rows {
		d := &domain.Task{
			ID:        row.ID,
			Title:     row.Title,
			ProjectID: row.ProjectID,
			OrgID:     row.OrgID,
		}
		tasks = append(tasks, d)
	}
	return tasks, nil
}

// ListByIDsFull is the full-projection batch fetch (org-scoped). Used by
// BatchUpdate which needs complete task rows to update safely: ListByIDs
// (above) is intentionally minimal for mention hydration. Loads assignees
// and labels in bulk so the returned tasks are ready to mutate.
func (s *TaskStore) ListByIDsFull(ctx context.Context, orgID string, ids []string) ([]*domain.Task, error) {
	if len(ids) == 0 {
		return nil, nil
	}
	rows, err := s.q.GetTasksByIDsFull(ctx, sqlc.GetTasksByIDsFullParams{Ids: ids, OrgID: orgID})
	if err != nil {
		return nil, err
	}
	tasks := make([]*domain.Task, 0, len(rows))
	for _, row := range rows {
		d := s.toDomain(taskRowFromGetByIDsFull(row))
		tasks = append(tasks, &d)
	}
	if len(tasks) == 0 {
		return tasks, nil
	}
	taskIDs := make([]string, len(tasks))
	for i, t := range tasks {
		taskIDs[i] = t.ID
	}
	assigneeMap, err := s.loadAssigneesBatch(ctx, taskIDs)
	if err != nil {
		return nil, err
	}
	labelMap, err := s.loadLabelsBatch(ctx, taskIDs)
	if err != nil {
		return nil, err
	}
	for _, t := range tasks {
		t.Assignees = assigneeMap[t.ID]
		t.Labels = labelMap[t.ID]
	}
	return tasks, nil
}

// taskCursor is the wire format for the opaque pagination cursor in
// ListTasksByUser-based endpoints. Matches the encodeMsgCursor pattern used
// for messages.
type taskCursor struct {
	C string `json:"c"` // updated_at formatted as "2006-01-02 15:04:05"
	I string `json:"i"` // task id
}

func encodeTaskCursor(updatedAt time.Time, id string) string {
	b, _ := json.Marshal(taskCursor{C: updatedAt.UTC().Format("2006-01-02 15:04:05"), I: id})
	return base64.URLEncoding.EncodeToString(b)
}

func decodeTaskCursor(s string) (updatedAt, id string, err error) {
	if s == "" {
		return "", "", nil
	}
	b, err := base64.URLEncoding.DecodeString(s)
	if err != nil {
		return "", "", err
	}
	var c taskCursor
	if err := json.Unmarshal(b, &c); err != nil {
		return "", "", err
	}
	return c.C, c.I, nil
}

func (s *TaskStore) ListByUser(ctx context.Context, orgID, userID string, filter domain.TaskListFilter) (*domain.TaskListResult, error) {
	search := filter.Search
	if search == "" {
		search = "%%"
	}
	showCompleted := int64(0)
	if filter.ShowCompleted {
		showCompleted = 1
	}

	cursorUpdatedAt, cursorID, err := decodeTaskCursor(filter.Cursor)
	if err != nil {
		return nil, fmt.Errorf("invalid cursor: %w", err)
	}

	// Fetch limit+1 to determine has_more.
	limit := int64(filter.Limit + 1)

	hasLabelFilter := int64(0)
	labelIDs := filter.LabelIDs
	if len(labelIDs) > 0 {
		hasLabelFilter = 1
	} else {
		// The slice placeholder must always have at least one value so sqlc's
		// generated placeholder numbering stays contiguous. When the filter is
		// off has_label_filter=0 short-circuits the EXISTS anyway.
		labelIDs = []string{"__no_label__"}
	}
	effectiveAssigneeID := userID
	if filter.AssigneeID != nil && *filter.AssigneeID != "" {
		effectiveAssigneeID = *filter.AssigneeID
	}

	requireProjectMembership := int64(0)
	if filter.RequireProjectMembership {
		requireProjectMembership = 1
	}

	rows, err := s.q.ListTasksByUser(ctx, sqlc.ListTasksByUserParams{
		EffectiveAssigneeID:      effectiveAssigneeID,
		OrgID:                    orgID,
		StatusID:                 filter.StatusID,
		CycleID:                  filter.CycleID,
		Priority:                 filter.Priority,
		ShowCompleted:            showCompleted,
		LabelIds:                 labelIDs,
		HasLabelFilter:           hasLabelFilter,
		RequireProjectMembership: requireProjectMembership,
		Search:                   search,
		CursorUpdatedAt:          cursorUpdatedAt,
		CursorID:                 cursorID,
		LimitVal:                 limit,
	})
	if err != nil {
		return nil, err
	}

	hasMore := len(rows) > filter.Limit
	if hasMore {
		rows = rows[:filter.Limit]
	}

	taskIDs := make([]string, len(rows))
	for i, row := range rows {
		taskIDs[i] = row.ID
	}

	assigneeMap, err := s.loadAssigneesBatch(ctx, taskIDs)
	if err != nil {
		return nil, err
	}
	labelMap, err := s.loadLabelsBatch(ctx, taskIDs)
	if err != nil {
		return nil, err
	}

	tasks := make([]*domain.EnrichedTask, len(rows))
	for i, row := range rows {
		tasks[i] = &domain.EnrichedTask{
			Task:         s.toDomain(taskRowFromUserList(row)),
			ProjectName:  row.ProjectName,
			ProjectSlug:  row.ProjectSlug,
			ProjectColor: row.ProjectColor,
			StatusName:   row.StatusName,
			StatusColor:  row.StatusColor,
		}
		tasks[i].Assignees = assigneeMap[row.ID]
		tasks[i].Labels = labelMap[row.ID]
	}

	nextCursor := ""
	if hasMore && len(rows) > 0 {
		last := rows[len(rows)-1]
		nextCursor = encodeTaskCursor(parseTime(last.UpdatedAt), last.ID)
	}

	return &domain.TaskListResult{
		Items:      tasks,
		NextCursor: nextCursor,
		HasMore:    hasMore,
	}, nil
}
