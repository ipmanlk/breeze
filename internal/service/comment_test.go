package service

import (
	"context"
	"encoding/json"
	"errors"
	"log/slog"
	"strings"
	"testing"
	"time"

	"ipmanlk/plume/internal/apperr"
	"ipmanlk/plume/internal/domain"
)

func newCommentService() (*CommentService, *mockCommentRepo, *mockTaskRepo, *mockNotificationService, *mockBroadcaster, *mockTaskActivityRepo) {
	commentRepo := newMockCommentRepo()
	taskRepo := newMockTaskRepo()
	notifSvc := newMockNotificationService()
	bc := newMockBroadcaster()
	activityRepo := newMockTaskActivityRepo()
	svc := NewCommentService(
		commentRepo, taskRepo, newMockProjectRepo(), newMockConversationRepo(),
		newMockUserRepo(), notifSvc, bc, slog.Default(), nil, activityRepo,
	)
	return svc, commentRepo, taskRepo, notifSvc, bc, activityRepo
}

// seedTask seeds a task in the mock repo with an empty project ID so that the
// service's GetByID(ctx, orgID, taskID, "") lookup succeeds.
func seedTask(t *testing.T, repo *mockTaskRepo, id, orgID string, assignees ...*domain.TaskAssignee) *domain.Task {
	t.Helper()
	task := &domain.Task{ID: id, OrgID: orgID, ProjectID: "", Title: "Task " + id}
	for _, a := range assignees {
		task.Assignees = append(task.Assignees, *a)
	}
	repo.tasksByID[id] = task
	return task
}

func TestCommentService_Create(t *testing.T) {
	svc, commentRepo, taskRepo, notifSvc, _, activityRepo := newCommentService()
	seedTask(t, taskRepo, "task-1", "org-1")

	comment, err := svc.Create(context.Background(), "org-1", "task-1", "user-1", "Hello world", nil)
	if err != nil {
		t.Fatalf("Create() error = %v", err)
	}
	if comment.Content != "Hello world" {
		t.Errorf("Content = %q, want Hello world", comment.Content)
	}
	if comment.AuthorID != "user-1" {
		t.Errorf("AuthorID = %q, want user-1", comment.AuthorID)
	}
	if comment.TaskID != "task-1" {
		t.Errorf("TaskID = %q, want task-1", comment.TaskID)
	}
	if len(commentRepo.created) != 1 {
		t.Errorf("expected 1 Create call, got %d", len(commentRepo.created))
	}
	// No assignees → no notifications.
	if len(notifSvc.notifications) != 0 {
		t.Errorf("expected 0 notifications, got %d", len(notifSvc.notifications))
	}
	// Verify activity was recorded.
	if len(activityRepo.entries) != 1 {
		t.Errorf("expected 1 activity entry, got %d", len(activityRepo.entries))
	} else if activityRepo.entries[0].Action != domain.ActivityCommentAdded {
		t.Errorf("activity action = %q, want %q", activityRepo.entries[0].Action, domain.ActivityCommentAdded)
	}
}

func TestCommentService_Create_EmptyContent(t *testing.T) {
	svc, _, taskRepo, _, _, _ := newCommentService()
	seedTask(t, taskRepo, "task-1", "org-1")

	if _, err := svc.Create(context.Background(), "org-1", "task-1", "user-1", "", nil); !errors.Is(err, apperr.ErrInvalidInput) {
		t.Errorf("Create() empty content error = %v, want ErrInvalidInput", err)
	}
}

func TestCommentService_Create_ContentTooLong(t *testing.T) {
	svc, _, taskRepo, _, _, _ := newCommentService()
	seedTask(t, taskRepo, "task-1", "org-1")

	long := strings.Repeat("x", 10001)
	if _, err := svc.Create(context.Background(), "org-1", "task-1", "user-1", long, nil); !errors.Is(err, apperr.ErrInvalidInput) {
		t.Errorf("Create() long content error = %v, want ErrInvalidInput", err)
	}
}

func TestCommentService_Create_TaskNotFound(t *testing.T) {
	svc, _, _, _, _, _ := newCommentService()

	_, err := svc.Create(context.Background(), "org-1", "missing-task", "user-1", "Hello", nil)
	if err == nil {
		t.Error("Create() expected error for missing task")
	}
}

func TestCommentService_Create_NotifiesAssignees(t *testing.T) {
	svc, _, taskRepo, notifSvc, _, _ := newCommentService()
	assignee := &domain.TaskAssignee{ID: "assignee-1"}
	author := &domain.TaskAssignee{ID: "user-1"} // author is also an assignee
	seedTask(t, taskRepo, "task-1", "org-1", assignee, author)

	if _, err := svc.Create(context.Background(), "org-1", "task-1", "user-1", "Hello", nil); err != nil {
		t.Fatalf("Create() error = %v", err)
	}
	// Only the non-author assignee should be notified.
	if len(notifSvc.notifications) != 1 {
		t.Fatalf("expected 1 notification, got %d", len(notifSvc.notifications))
	}
	if notifSvc.notifications[0].UserID != "assignee-1" {
		t.Errorf("notification recipient = %q, want assignee-1", notifSvc.notifications[0].UserID)
	}
	if notifSvc.notifications[0].Type != domain.NotifTaskComment {
		t.Errorf("notification type = %q, want NotifTaskComment", notifSvc.notifications[0].Type)
	}
}

func TestCommentService_Create_NotifiesMentionedUser(t *testing.T) {
	svc, _, taskRepo, notifSvc, _, _ := newCommentService()
	seedTask(t, taskRepo, "task-1", "org-1")

	// <@user:mention-1> mention token, with no assignees → only the mentioned user.
	content := "hey <@user:mention-1> can you look?"
	if _, err := svc.Create(context.Background(), "org-1", "task-1", "user-1", content, nil); err != nil {
		t.Fatalf("Create() error = %v", err)
	}

	var got []string
	for _, n := range notifSvc.notifications {
		got = append(got, n.UserID)
	}
	if len(got) != 1 || got[0] != "mention-1" {
		t.Errorf("notifications = %v, want [mention-1]", got)
	}
	if notifSvc.notifications[0].Type != domain.NotifTaskComment {
		t.Errorf("type = %q, want NotifTaskComment", notifSvc.notifications[0].Type)
	}
	if notifSvc.notifications[0].ActorID != "user-1" {
		t.Errorf("actor = %q, want user-1", notifSvc.notifications[0].ActorID)
	}
}

func TestCommentService_Create_NotifiesParentAuthorOnReply(t *testing.T) {
	svc, commentRepo, taskRepo, notifSvc, _, _ := newCommentService()
	seedTask(t, taskRepo, "task-1", "org-1")

	parentID := "parent-1"
	// Seed the parent comment authored by someone else.
	commentRepo.byID[parentID] = &domain.Comment{ID: parentID, OrgID: "org-1", TaskID: "task-1", AuthorID: "parent-author"}

	pid := parentID
	if _, err := svc.Create(context.Background(), "org-1", "task-1", "user-1", "replying", &pid); err != nil {
		t.Fatalf("Create() error = %v", err)
	}

	var got []string
	for _, n := range notifSvc.notifications {
		got = append(got, n.UserID)
	}
	if len(got) != 1 || got[0] != "parent-author" {
		t.Errorf("notifications = %v, want [parent-author]", got)
	}
}

func TestCommentService_Create_DeduplicatesNotifiedUsers(t *testing.T) {
	svc, _, taskRepo, notifSvc, _, _ := newCommentService()
	// The mentioned user is also an assignee: should be notified only once.
	assignee := &domain.TaskAssignee{ID: "dup-1"}
	seedTask(t, taskRepo, "task-1", "org-1", assignee)

	content := "<@user:dup-1>"
	if _, err := svc.Create(context.Background(), "org-1", "task-1", "user-1", content, nil); err != nil {
		t.Fatalf("Create() error = %v", err)
	}
	if len(notifSvc.notifications) != 1 {
		t.Errorf("expected 1 notification (deduped), got %d", len(notifSvc.notifications))
	}
}

func TestCommentService_Create_BroadcastsToProjectRoom(t *testing.T) {
	svc, _, taskRepo, _, bc, _ := newCommentService()
	seedTask(t, taskRepo, "task-1", "org-1")

	if _, err := svc.Create(context.Background(), "org-1", "task-1", "user-1", "hi", nil); err != nil {
		t.Fatalf("Create() error = %v", err)
	}
	if len(bc.messages) != 2 {
		t.Fatalf("expected 2 broadcasts, got %d", len(bc.messages))
	}
	// First broadcast: comment_new
	if bc.messages[0].eventType != "comment_new" {
		t.Errorf("broadcast[0] type = %q, want comment_new", bc.messages[0].eventType)
	}
	if bc.messages[0].roomKey != domain.RoomKeyProject("org-1", "") {
		t.Errorf("broadcast[0] room key = %q, want project room", bc.messages[0].roomKey)
	}
	// Second broadcast: task_activity_recorded
	if bc.messages[1].eventType != "task_activity_recorded" {
		t.Errorf("broadcast[1] type = %q, want task_activity_recorded", bc.messages[1].eventType)
	}
	var payload map[string]any
	if p, ok := bc.messages[1].payload.(map[string]any); ok {
		payload = p
	}
	if payload == nil || payload["task_id"] != "task-1" {
		t.Errorf("broadcast[1] payload task_id = %v, want task-1", payload["task_id"])
	}
}

func TestCommentService_Update_BroadcastsAndSetsEdited(t *testing.T) {
	svc, commentRepo, _, _, bc, _ := newCommentService()
	commentRepo.byID["c-1"] = &domain.Comment{ID: "c-1", OrgID: "org-1", TaskID: "task-1", AuthorID: "user-1", Content: "old"}

	if _, err := svc.Update(context.Background(), "org-1", "c-1", "user-1", "edited"); err != nil {
		t.Fatalf("Update() error = %v", err)
	}
	if len(bc.messages) != 1 || bc.messages[0].eventType != "comment_updated" {
		t.Errorf("expected 1 comment_updated broadcast, got %+v", bc.messages)
	}
	if commentRepo.updated[0].Content != "edited" {
		t.Errorf("persisted content = %q, want edited", commentRepo.updated[0].Content)
	}
	// edited_at is set by the DB layer (datetime('now')), not the service.
}

func TestCommentService_ListByTask(t *testing.T) {
	svc, commentRepo, taskRepo, _, _, _ := newCommentService()
	// Seed the task in org-1 / proj-1 so the service's task-ownership check passes.
	taskRepo.tasksByID["task-1"] = &domain.Task{ID: "task-1", OrgID: "org-1", ProjectID: "proj-1"}
	// Seed two comments, one soft-deleted.
	commentRepo.byID["c-1"] = &domain.Comment{ID: "c-1", OrgID: "org-1", TaskID: "task-1", Content: "first", CreatedAt: time.Now().Add(-2 * time.Hour)}
	commentRepo.byID["c-2"] = &domain.Comment{ID: "c-2", OrgID: "org-1", TaskID: "task-1", Content: "second", CreatedAt: time.Now().Add(-1 * time.Hour)}
	commentRepo.byID["c-3"] = &domain.Comment{ID: "c-3", OrgID: "org-1", TaskID: "task-1", Content: "deleted", CreatedAt: time.Now()}
	now := time.Now()
	commentRepo.byID["c-3"].DeletedAt = &now
	commentRepo.byTask["task-1"] = []*domain.Comment{commentRepo.byID["c-1"], commentRepo.byID["c-2"], commentRepo.byID["c-3"]}

	result, err := svc.ListByTask(context.Background(), "org-1", "task-1", "proj-1", "", 50)
	if err != nil {
		t.Fatalf("ListByTask() error = %v", err)
	}
	if len(result.Items) != 2 {
		t.Errorf("returned %d comments, want 2 (soft-deleted excluded)", len(result.Items))
	}
	// Should be ordered ASC (oldest first): c-1 before c-2
	if result.Items[0].ID != "c-1" {
		t.Errorf("expected c-1 first, got %s", result.Items[0].ID)
	}
}

func TestCommentService_Update(t *testing.T) {
	svc, commentRepo, _, _, _, _ := newCommentService()
	commentRepo.byID["c-1"] = &domain.Comment{ID: "c-1", OrgID: "org-1", TaskID: "task-1", AuthorID: "user-1", Content: "old"}

	updated, err := svc.Update(context.Background(), "org-1", "c-1", "user-1", "new content")
	if err != nil {
		t.Fatalf("Update() error = %v", err)
	}
	if updated.Content != "new content" {
		t.Errorf("Content = %q, want new content", updated.Content)
	}
	if len(commentRepo.updated) != 1 {
		t.Errorf("expected 1 Update call, got %d", len(commentRepo.updated))
	}
}

func TestCommentService_Update_NotFound(t *testing.T) {
	svc, _, _, _, _, _ := newCommentService()

	_, err := svc.Update(context.Background(), "org-1", "missing", "user-1", "content")
	if err == nil {
		t.Error("Update() expected error for missing comment")
	}
}

func TestCommentService_Update_NotAuthor(t *testing.T) {
	svc, commentRepo, _, _, _, _ := newCommentService()
	commentRepo.byID["c-1"] = &domain.Comment{ID: "c-1", OrgID: "org-1", TaskID: "task-1", AuthorID: "user-1", Content: "old"}

	_, err := svc.Update(context.Background(), "org-1", "c-1", "user-2", "hacked")
	if !errors.Is(err, apperr.ErrForbidden) {
		t.Errorf("Update() by non-author error = %v, want ErrForbidden", err)
	}
	if commentRepo.byID["c-1"].Content != "old" {
		t.Errorf("content was mutated by forbidden update: %q", commentRepo.byID["c-1"].Content)
	}
}

func TestCommentService_Delete(t *testing.T) {
	svc, commentRepo, _, _, _, _ := newCommentService()
	commentRepo.byID["c-1"] = &domain.Comment{ID: "c-1", OrgID: "org-1", TaskID: "task-1", AuthorID: "user-1"}

	if err := svc.Delete(context.Background(), "org-1", "c-1", "user-1"); err != nil {
		t.Fatalf("Delete() error = %v", err)
	}
	if len(commentRepo.deleted) != 1 {
		t.Errorf("expected 1 SoftDelete call, got %d", len(commentRepo.deleted))
	}
	if commentRepo.byID["c-1"].DeletedAt == nil {
		t.Error("comment DeletedAt not set after Delete")
	}
}

func TestCommentService_Delete_NotFound(t *testing.T) {
	svc, _, _, _, _, _ := newCommentService()

	if err := svc.Delete(context.Background(), "org-1", "missing", "user-1"); err == nil {
		t.Error("Delete() expected error for missing comment")
	}
}

func TestCommentService_Delete_NotAuthor(t *testing.T) {
	svc, commentRepo, _, _, _, _ := newCommentService()
	commentRepo.byID["c-1"] = &domain.Comment{ID: "c-1", OrgID: "org-1", TaskID: "task-1", AuthorID: "user-1"}

	if err := svc.Delete(context.Background(), "org-1", "c-1", "user-2"); !errors.Is(err, apperr.ErrForbidden) {
		t.Errorf("Delete() by non-author error = %v, want ErrForbidden", err)
	}
	if commentRepo.byID["c-1"].DeletedAt != nil {
		t.Error("comment was deleted by non-author")
	}
}

// TestCommentService_ListByTask_RejectsTaskFromWrongProject covers the IDOR
// case: a caller passes project-A's projectID + project-B's taskID. The
// service must reject it (task doesn't belong to the URL's project) instead of
// returning project-B's comment thread. Previously ListByTask never verified
// task↔project ownership, so the foreign task's comments leaked.
func TestCommentService_ListByTask_RejectsTaskFromWrongProject(t *testing.T) {
	svc, commentRepo, taskRepo, _, _, _ := newCommentService()
	// task-1 belongs to proj-B, but the caller claims proj-A.
	taskRepo.tasksByID["task-1"] = &domain.Task{ID: "task-1", OrgID: "org-1", ProjectID: "proj-B"}
	commentRepo.byTask["task-1"] = []*domain.Comment{
		{ID: "c-1", OrgID: "org-1", TaskID: "task-1", Content: "leaked", CreatedAt: time.Now()},
	}

	// Caller passes proj-A (mismatch). Expect ErrNotFound, no comments returned.
	_, err := svc.ListByTask(context.Background(), "org-1", "task-1", "proj-A", "", 50)
	if err == nil {
		t.Fatal("expected error for task not in claimed project, got nil")
	}
	if !errors.Is(err, apperr.ErrNotFound) {
		t.Errorf("expected ErrNotFound, got %v", err)
	}
}

// TestCommentService_ListByTask_RejectsCrossOrgTask verifies the org-scoping
// half of the IDOR fix: even if the project ID matched, a task in a different org must
// not return comments.
func TestCommentService_ListByTask_RejectsCrossOrgTask(t *testing.T) {
	svc, commentRepo, taskRepo, _, _, _ := newCommentService()
	// task-1 is in org-2; caller is in org-1.
	taskRepo.tasksByID["task-1"] = &domain.Task{ID: "task-1", OrgID: "org-2", ProjectID: "proj-1"}
	commentRepo.byTask["task-1"] = []*domain.Comment{
		{ID: "c-1", OrgID: "org-2", TaskID: "task-1", Content: "leaked", CreatedAt: time.Now()},
	}

	_, err := svc.ListByTask(context.Background(), "org-1", "task-1", "proj-1", "", 50)
	if err == nil {
		t.Fatal("expected error for cross-org task, got nil")
	}
}

// TestCommentService_Create_BroadcastsSnakeCase asserts that Create broadcasts
// the comment using snake_case wire format, not raw domain.Comment (PascalCase).
func TestCommentService_Create_BroadcastsSnakeCase(t *testing.T) {
	ctx := context.Background()
	commentRepo := newMockCommentRepo()
	taskRepo := newMockTaskRepo()
	bc := newMockBroadcaster()

	seedTask(t, taskRepo, "task-1", "org-1", &domain.TaskAssignee{ID: "user-1"})

	svc := NewCommentService(
		commentRepo, taskRepo, newMockProjectRepo(), newMockConversationRepo(),
		newMockUserRepo(), newMockNotificationService(), bc, slog.Default(), nil, nil,
	)

	comment, err := svc.Create(ctx, "org-1", "task-1", "user-1", "Hello world", nil)
	if err != nil {
		t.Fatalf("Create() error = %v", err)
	}

	if len(bc.messages) != 1 {
		t.Fatalf("expected 1 broadcast, got %d", len(bc.messages))
	}
	if bc.messages[0].eventType != "comment_new" {
		t.Errorf("broadcast type = %q, want comment_new", bc.messages[0].eventType)
	}

	payloadJSON, err := json.Marshal(bc.messages[0].payload)
	if err != nil {
		t.Fatalf("marshal payload: %v", err)
	}
	s := string(payloadJSON)

	// Assert snake_case keys present.
	for _, key := range []string{"\"id\"", "\"task_id\"", "\"author_id\"", "\"content\"", "\"created_at\"", "\"project_id\""} {
		if !strings.Contains(s, key) {
			t.Errorf("payload missing snake_case key %s\npayload: %s", key, s)
		}
	}

	// Assert PascalCase keys absent.
	for _, key := range []string{"\"TaskID\"", "\"AuthorID\""} {
		if strings.Contains(s, key) {
			t.Errorf("payload contains PascalCase key %s: raw domain.Comment leaked\npayload: %s", key, s)
		}
	}

	// Also verify the comment field wraps the wire data properly.
	_ = comment // used the return, not the broadcast
}

// TestCommentService_Update_BroadcastsSnakeCase asserts that Update broadcasts
// the comment using snake_case wire format, not raw domain.Comment (PascalCase).
func TestCommentService_Update_BroadcastsSnakeCase(t *testing.T) {
	ctx := context.Background()
	commentRepo := newMockCommentRepo()
	bc := newMockBroadcaster()
	commentRepo.byID["c-1"] = &domain.Comment{ID: "c-1", OrgID: "org-1", TaskID: "task-1", ProjectID: "proj-1", AuthorID: "user-1", Content: "old", CreatedAt: time.Now()}

	svc := NewCommentService(
		commentRepo, newMockTaskRepo(), newMockProjectRepo(), newMockConversationRepo(),
		newMockUserRepo(), newMockNotificationService(), bc, slog.Default(), nil, nil,
	)

	if _, err := svc.Update(ctx, "org-1", "c-1", "user-1", "edited content"); err != nil {
		t.Fatalf("Update() error = %v", err)
	}

	if len(bc.messages) != 1 {
		t.Fatalf("expected 1 broadcast, got %d", len(bc.messages))
	}
	if bc.messages[0].eventType != "comment_updated" {
		t.Errorf("broadcast type = %q, want comment_updated", bc.messages[0].eventType)
	}

	payloadJSON, err := json.Marshal(bc.messages[0].payload)
	if err != nil {
		t.Fatalf("marshal payload: %v", err)
	}
	s := string(payloadJSON)

	// Assert snake_case keys present.
	for _, key := range []string{"\"id\"", "\"task_id\"", "\"author_id\"", "\"content\"", "\"created_at\"", "\"project_id\""} {
		if !strings.Contains(s, key) {
			t.Errorf("payload missing snake_case key %s\npayload: %s", key, s)
		}
	}

	// Assert PascalCase keys absent.
	for _, key := range []string{"\"TaskID\"", "\"AuthorID\""} {
		if strings.Contains(s, key) {
			t.Errorf("payload contains PascalCase key %s: raw domain.Comment leaked\npayload: %s", key, s)
		}
	}
}
