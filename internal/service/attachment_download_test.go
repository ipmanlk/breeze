package service

import (
	"bytes"
	"context"
	"errors"
	"io"
	"testing"
	"time"

	"ipmanlk/breeze/internal/apperr"
	"ipmanlk/breeze/internal/domain"
	"ipmanlk/breeze/internal/port"
)

// stubTaskRepoForDownload is a minimal port.TaskRepository that only
// implements GetByIDAndOrg (the method AttachmentService.Download now uses).
// Other methods panic if called: they are irrelevant to the Download path.
type stubTaskRepoForDownload struct {
	task *domain.Task
}

func (s *stubTaskRepoForDownload) GetByIDAndOrg(ctx context.Context, orgID, id string) (*domain.Task, error) {
	if s.task != nil && s.task.ID == id && s.task.OrgID == orgID {
		return s.task, nil
	}
	return nil, apperr.ErrNotFound
}

// The following methods satisfy port.TaskRepository but are unused by Download.
func (s *stubTaskRepoForDownload) ListByProject(context.Context, string, string, domain.TaskFilter) ([]*domain.Task, error) {
	panic("unused")
}
func (s *stubTaskRepoForDownload) ListByUser(context.Context, string, string, domain.TaskListFilter) (*domain.TaskListResult, error) {
	panic("unused")
}
func (s *stubTaskRepoForDownload) ListByIDs(context.Context, string, []string) ([]*domain.Task, error) {
	panic("unused")
}
func (s *stubTaskRepoForDownload) ListByIDsFull(context.Context, string, []string) ([]*domain.Task, error) {
	panic("unused")
}
func (s *stubTaskRepoForDownload) GetByID(context.Context, string, string, string) (*domain.Task, error) {
	panic("unused")
}
func (s *stubTaskRepoForDownload) ListSubtasks(context.Context, string, string) ([]*domain.Task, error) {
	panic("unused")
}
func (s *stubTaskRepoForDownload) Create(context.Context, *domain.Task) error { panic("unused") }
func (s *stubTaskRepoForDownload) Update(context.Context, *domain.Task) error { panic("unused") }
func (s *stubTaskRepoForDownload) Move(context.Context, string, string, string, string, string) error {
	panic("unused")
}
func (s *stubTaskRepoForDownload) MoveToProject(context.Context, string, string, string, string, string, string, *time.Time) error {
	panic("unused")
}
func (s *stubTaskRepoForDownload) GetLastPositionKey(context.Context, string, string, string) (string, error) {
	panic("unused")
}
func (s *stubTaskRepoForDownload) GeneratePositionKey(context.Context, string, string, string) (string, error) {
	panic("unused")
}
func (s *stubTaskRepoForDownload) GetPositionKeyNeighbors(context.Context, string, string) (string, string, error) {
	panic("unused")
}
func (s *stubTaskRepoForDownload) Delete(context.Context, string, string, string) error {
	panic("unused")
}
func (s *stubTaskRepoForDownload) DeleteSubtasks(context.Context, string, string) error {
	panic("unused")
}
func (s *stubTaskRepoForDownload) PromoteSubtasks(context.Context, string, string) error {
	panic("unused")
}
func (s *stubTaskRepoForDownload) CountSubtasks(context.Context, string, string) (int64, error) {
	panic("unused")
}
func (s *stubTaskRepoForDownload) GetLastSubtaskPosition(context.Context, string, string) (string, error) {
	panic("unused")
}
func (s *stubTaskRepoForDownload) GenerateSubtaskPositionKey(context.Context, string, string) (string, error) {
	panic("unused")
}
func (s *stubTaskRepoForDownload) RunInTransaction(_ context.Context, fn func(port.TaskRepository) error) error {
	return fn(s)
}
func (s *stubTaskRepoForDownload) ReorderSubtasks(context.Context, string, string, []domain.ReorderOp) error {
	panic("unused")
}
func (s *stubTaskRepoForDownload) SetAssignees(context.Context, string, []string) error {
	panic("unused")
}
func (s *stubTaskRepoForDownload) ListAssigneesByTaskIDs(context.Context, []string) (map[string][]domain.TaskAssignee, error) {
	panic("unused")
}
func (s *stubTaskRepoForDownload) UnassignCycleFromTasks(context.Context, string, string) error {
	panic("unused")
}
func (s *stubTaskRepoForDownload) MoveTasksToCycle(context.Context, string, string, string) error {
	panic("unused")
}
func (s *stubTaskRepoForDownload) MoveIncompleteTasksToCycle(context.Context, string, string, string) error {
	panic("unused")
}
func (s *stubTaskRepoForDownload) UnassignCycleFromIncompleteTasks(context.Context, string, string) error {
	panic("unused")
}
func (s *stubTaskRepoForDownload) ListByCycle(context.Context, string, string) ([]*domain.Task, error) {
	panic("unused")
}

type stubAttRepo struct {
	att *domain.Attachment
}

func (s *stubAttRepo) Create(context.Context, *domain.Attachment) error { return nil }
func (s *stubAttRepo) GetByID(ctx context.Context, id string) (*domain.Attachment, error) {
	if s.att != nil && s.att.ID == id {
		return s.att, nil
	}
	return nil, apperr.ErrNotFound
}
func (s *stubAttRepo) ListByTask(context.Context, string) ([]*domain.Attachment, error) {
	return nil, nil
}
func (s *stubAttRepo) Delete(context.Context, string, string) error { return nil }

type stubStorage struct{ content []byte }

func (s *stubStorage) Save(context.Context, string, io.Reader) error { return nil }
func (s *stubStorage) Get(ctx context.Context, path string) (io.ReadCloser, error) {
	if path == "" {
		return nil, errors.New("no path")
	}
	return io.NopCloser(bytes.NewReader(s.content)), nil
}
func (s *stubStorage) Delete(context.Context, string) error        { return nil }
func (s *stubStorage) URL(context.Context, string) (string, error) { return "", nil }

// TestAttachmentService_Download_OrgScoped verifies that Download
// resolves the owning task with an org-scoped lookup (GetByIDAndOrg), so an
// attachment whose task belongs to a different org is rejected (returns an
// error → handler 404) instead of leaking the file. Previously Download
// called GetByID(ctx, "", taskID, "") which never matched and always 404'd.
func TestAttachmentService_Download_OrgScoped(t *testing.T) {
	task := &domain.Task{ID: "task-1", OrgID: "org-A", ProjectID: "proj-A"}
	att := &domain.Attachment{ID: "att-1", TaskID: "task-1", StoragePath: "p/f.bin", ContentType: "image/png"}

	svc := NewAttachmentService(&stubAttRepo{att: att}, &stubTaskRepoForDownload{task: task}, &stubStorage{content: []byte("x")}, nil, nil, nil)

	// Same org: download succeeds and returns the correct project ID.
	_, ct, projID, _, err := svc.Download(context.Background(), "org-A", "att-1")
	if err != nil {
		t.Fatalf("same-org download: %v", err)
	}
	if projID != "proj-A" {
		t.Errorf("projectID = %q, want proj-A", projID)
	}
	if ct != "image/png" {
		t.Errorf("contentType = %q, want image/png", ct)
	}

	// Different org: download must fail (task lookup is org-scoped).
	_, _, _, _, err = svc.Download(context.Background(), "org-B", "att-1")
	if err == nil {
		t.Fatal("cross-org download: expected error (org-scoped task not found), got nil")
	}
}
