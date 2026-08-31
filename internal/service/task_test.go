package service

import (
	"context"
	"strings"
	"testing"
	"time"

	"ipmanlk/plume/internal/domain"
)

func TestTaskService_Create_Success(t *testing.T) {
	taskRepo := newMockTaskRepo()
	projRepo := newMockProjectRepo()
	statusRepo := newMockTaskStatusRepo()

	projRepo.projectsByID["proj-1"] = &domain.Project{ID: "proj-1", OrgID: "org-1", Slug: "test-project"}

	statusID := "status-1"
	statusRepo.statusesByID[statusID] = &domain.TaskStatus{
		ID: statusID, ProjectID: "proj-1", Name: "Todo", Color: "#3b82f6",
	}

	cycleRepo := newMockCycleRepo()
	svc := NewTaskService(taskRepo, projRepo, statusRepo, cycleRepo, newMockNotificationService(), newMockUserRepo(), newMockConversationRepo())
	task, err := svc.Create(context.Background(), domain.CreateTaskParams{
		OrgID:       "org-1",
		ProjectID:   "proj-1",
		CreatedBy:   "user-1",
		Title:       "Fix login bug",
		Description: "The login form crashes",
		StatusID:    statusID,
		Priority:    "high",
	})
	if err != nil {
		t.Fatalf("Create() error = %v", err)
	}
	if task == nil {
		t.Fatal("Create() task is nil")
	}
	if task.Title != "Fix login bug" {
		t.Errorf("Title = %q, want %q", task.Title, "Fix login bug")
	}
	if task.Priority != "high" {
		t.Errorf("Priority = %q, want %q", task.Priority, "high")
	}
	if task.OrgID != "org-1" {
		t.Errorf("OrgID = %q, want %q", task.OrgID, "org-1")
	}
}

func TestTaskService_Create_EmptyTitle(t *testing.T) {
	svc := NewTaskService(newMockTaskRepo(), newMockProjectRepo(), newMockTaskStatusRepo(), newMockCycleRepo(), newMockNotificationService(), newMockUserRepo(), newMockConversationRepo())
	_, err := svc.Create(context.Background(), domain.CreateTaskParams{
		OrgID:     "org-1",
		ProjectID: "proj-1",
		CreatedBy: "user-1",
		StatusID:  "status-1",
		Priority:  "none",
	})
	if err == nil {
		t.Fatal("expected error for empty title")
	}
}

func TestTaskService_Create_InvalidStatus(t *testing.T) {
	statusRepo := newMockTaskStatusRepo()
	svc := NewTaskService(newMockTaskRepo(), newMockProjectRepo(), statusRepo, newMockCycleRepo(), newMockNotificationService(), newMockUserRepo(), newMockConversationRepo())
	_, err := svc.Create(context.Background(), domain.CreateTaskParams{
		OrgID:     "org-1",
		ProjectID: "proj-1",
		CreatedBy: "user-1",
		Title:     "Task",
		StatusID:  "nonexistent-status",
		Priority:  "none",
	})
	if err == nil {
		t.Fatal("expected error for invalid status")
	}
}

func TestTaskService_Create_InvalidPriorityDefaultsToNone(t *testing.T) {
	projRepo := newMockProjectRepo()
	projRepo.projectsByID["proj-1"] = &domain.Project{ID: "proj-1", OrgID: "org-1", Slug: "test-project"}
	statusRepo := newMockTaskStatusRepo()
	statusID := "status-1"
	statusRepo.statusesByID[statusID] = &domain.TaskStatus{
		ID: statusID, ProjectID: "proj-1", Name: "Todo", Color: "#3b82f6",
	}

	svc := NewTaskService(newMockTaskRepo(), projRepo, statusRepo, newMockCycleRepo(), newMockNotificationService(), newMockUserRepo(), newMockConversationRepo())
	task, err := svc.Create(context.Background(), domain.CreateTaskParams{
		OrgID:     "org-1",
		ProjectID: "proj-1",
		CreatedBy: "user-1",
		Title:     "Task",
		StatusID:  statusID,
		Priority:  "invalid-priority",
	})
	if err != nil {
		t.Fatalf("Create() error = %v", err)
	}
	if task.Priority != "none" {
		t.Errorf("Priority = %q, want %q", task.Priority, "none")
	}
}

func TestTaskService_ListAndDelete(t *testing.T) {
	taskRepo := newMockTaskRepo()
	projRepo := newMockProjectRepo()
	projRepo.projectsByID["proj-1"] = &domain.Project{ID: "proj-1", OrgID: "org-1", Slug: "test-project"}
	statusRepo := newMockTaskStatusRepo()
	statusID := "status-1"
	statusRepo.statusesByID[statusID] = &domain.TaskStatus{
		ID: statusID, ProjectID: "proj-1", Name: "Todo", Color: "#3b82f6",
	}

	cycleRepo := newMockCycleRepo()
	svc := NewTaskService(taskRepo, projRepo, statusRepo, cycleRepo, newMockNotificationService(), newMockUserRepo(), newMockConversationRepo())

	t1, _ := svc.Create(context.Background(), domain.CreateTaskParams{
		OrgID: "org-1", ProjectID: "proj-1", CreatedBy: "user-1",
		Title: "Task 1", StatusID: statusID, Priority: "low",
	})
	t2, _ := svc.Create(context.Background(), domain.CreateTaskParams{
		OrgID: "org-1", ProjectID: "proj-1", CreatedBy: "user-1",
		Title: "Task 2", StatusID: statusID, Priority: "high",
	})

	tasks, err := svc.List(context.Background(), "org-1", "proj-1", domain.TaskFilter{})
	if err != nil {
		t.Fatalf("List() error = %v", err)
	}
	if len(tasks) != 2 {
		t.Errorf("got %d tasks, want 2", len(tasks))
	}

	if err := svc.Delete(context.Background(), "org-1", t1.ID, "proj-1", domain.DeleteSubtaskModeBlock, "user-1"); err != nil {
		t.Fatalf("Delete() error = %v", err)
	}
	tasks, _ = svc.List(context.Background(), "org-1", "proj-1", domain.TaskFilter{})
	if len(tasks) != 1 {
		t.Errorf("after delete got %d tasks, want 1", len(tasks))
	}

	statusRepo.statusesByID["new-status"] = &domain.TaskStatus{
		ID: "new-status", ProjectID: "proj-1", Name: "Done", Color: "#22c55e",
	}
	if err := svc.Move(context.Background(), "user-1", "org-1", t2.ID, "proj-1", "new-status", "6"); err != nil {
		t.Fatalf("Move() error = %v", err)
	}
	updated, _ := svc.GetByID(context.Background(), "org-1", t2.ID, "proj-1")
	if updated.StatusID != "new-status" {
		t.Errorf("after move StatusID = %q, want %q", updated.StatusID, "new-status")
	}
}

func TestTaskService_GetByID_HydratesMentions(t *testing.T) {
	taskRepo := newMockTaskRepo()
	projRepo := newMockProjectRepo()
	statusRepo := newMockTaskStatusRepo()
	userRepo := newMockUserRepo()
	convRepo := newMockConversationRepo()

	projRepo.projectsByID["proj-1"] = &domain.Project{ID: "proj-1", OrgID: "org-1", Slug: "test-project"}
	statusRepo.statusesByID["status-1"] = &domain.TaskStatus{ID: "status-1", ProjectID: "proj-1", Name: "Todo", Color: "#3b82f6"}
	userRepo.usersByID["user-1"] = &domain.User{ID: "user-1", Name: "Alice"}
	userRepo.usersByID["user-2"] = &domain.User{ID: "user-2", Name: "Bob"}

	// Seed a task whose description mentions two users.
	task := &domain.Task{
		ID:          "task-1",
		OrgID:       "org-1",
		ProjectID:   "proj-1",
		Title:       "Task with mentions",
		Description: "Hey <@user:user-1> and <@user:user-2>, check this out",
		StatusID:    "status-1",
		Priority:    "none",
		PositionKey: "a0",
	}
	taskRepo.tasksByID["task-1"] = task

	svc := NewTaskService(taskRepo, projRepo, statusRepo, newMockCycleRepo(), newMockNotificationService(), userRepo, convRepo)
	got, err := svc.GetByID(context.Background(), "org-1", "task-1", "proj-1")
	if err != nil {
		t.Fatalf("GetByID() error = %v", err)
	}
	if got.Mentions == nil {
		t.Fatal("Mentions is nil; expected hydrated payload")
	}
	if got.Mentions.Users["user-1"] != "Alice" {
		t.Errorf("Mentions.Users[user-1] = %q, want %q", got.Mentions.Users["user-1"], "Alice")
	}
	if got.Mentions.Users["user-2"] != "Bob" {
		t.Errorf("Mentions.Users[user-2] = %q, want %q", got.Mentions.Users["user-2"], "Bob")
	}
}

func TestTaskService_GetByID_NoMentionsReturnsEmpty(t *testing.T) {
	taskRepo := newMockTaskRepo()
	projRepo := newMockProjectRepo()
	statusRepo := newMockTaskStatusRepo()

	projRepo.projectsByID["proj-1"] = &domain.Project{ID: "proj-1", OrgID: "org-1", Slug: "test-project"}
	statusRepo.statusesByID["status-1"] = &domain.TaskStatus{ID: "status-1", ProjectID: "proj-1", Name: "Todo", Color: "#3b82f6"}

	taskRepo.tasksByID["task-1"] = &domain.Task{
		ID:          "task-1",
		OrgID:       "org-1",
		ProjectID:   "proj-1",
		Title:       "Plain task",
		Description: "No mentions here",
		StatusID:    "status-1",
		Priority:    "none",
		PositionKey: "a0",
	}

	svc := NewTaskService(taskRepo, projRepo, statusRepo, newMockCycleRepo(), newMockNotificationService(), newMockUserRepo(), newMockConversationRepo())
	got, err := svc.GetByID(context.Background(), "org-1", "task-1", "proj-1")
	if err != nil {
		t.Fatalf("GetByID() error = %v", err)
	}
	if got.Mentions == nil {
		t.Fatal("Mentions is nil; expected non-nil (possibly empty) payload")
	}
	if len(got.Mentions.Users) != 0 {
		t.Errorf("Mentions.Users has %d entries, want 0", len(got.Mentions.Users))
	}
}

func TestTaskService_BatchUpdate_AppliesPriorityAndStatus(t *testing.T) {
	taskRepo := newMockTaskRepo()
	projRepo := newMockProjectRepo()
	statusRepo := newMockTaskStatusRepo()
	cycleRepo := newMockCycleRepo()

	projRepo.projectsByID["proj-1"] = &domain.Project{ID: "proj-1", OrgID: "org-1", Slug: "p"}
	statusRepo.statusesByID["status-todo"] = &domain.TaskStatus{ID: "status-todo", ProjectID: "proj-1"}
	statusRepo.statusesByID["status-done"] = &domain.TaskStatus{ID: "status-done", ProjectID: "proj-1"}

	for _, id := range []string{"t1", "t2"} {
		taskRepo.tasksByID[id] = &domain.Task{
			ID: id, OrgID: "org-1", ProjectID: "proj-1",
			Title: "T", StatusID: "status-todo", Priority: "none",
		}
	}

	svc := NewTaskService(taskRepo, projRepo, statusRepo, cycleRepo, newMockNotificationService(), newMockUserRepo(), newMockConversationRepo())

	done := "status-done"
	high := "high"
	updated, err := svc.BatchUpdate(context.Background(), "org-1", domain.BatchUpdateParams{
		TaskIDs:   []string{"t1", "t2"},
		ProjectID: "proj-1",
		StatusID:  &done,
		Priority:  &high,
	}, "user-1")
	if err != nil {
		t.Fatalf("BatchUpdate() error = %v", err)
	}
	if len(updated) != 2 {
		t.Fatalf("BatchUpdate returned %d tasks, want 2", len(updated))
	}
	for _, tk := range taskRepo.tasksByID {
		if tk.StatusID != "status-done" {
			t.Errorf("task %s status = %q, want status-done", tk.ID, tk.StatusID)
		}
		if tk.Priority != "high" {
			t.Errorf("task %s priority = %q, want high", tk.ID, tk.Priority)
		}
	}
}

func TestTaskService_BatchUpdate_RejectsForeignProjectTask(t *testing.T) {
	taskRepo := newMockTaskRepo()
	projRepo := newMockProjectRepo()
	statusRepo := newMockTaskStatusRepo()
	cycleRepo := newMockCycleRepo()
	projRepo.projectsByID["proj-1"] = &domain.Project{ID: "proj-1", OrgID: "org-1", Slug: "p"}

	// t-foreign belongs to a different project.
	taskRepo.tasksByID["t-foreign"] = &domain.Task{
		ID: "t-foreign", OrgID: "org-1", ProjectID: "proj-2",
		Title: "X", StatusID: "s", Priority: "none",
	}

	svc := NewTaskService(taskRepo, projRepo, statusRepo, cycleRepo, newMockNotificationService(), newMockUserRepo(), newMockConversationRepo())

	_, err := svc.BatchUpdate(context.Background(), "org-1", domain.BatchUpdateParams{
		TaskIDs:   []string{"t-foreign"},
		ProjectID: "proj-1",
		Priority:  strPtr("high"),
	}, "user-1")
	if err == nil {
		t.Fatal("BatchUpdate foreign task = nil error, want validation error")
	}
}

func strPtr(s string) *string { return &s }

func TestTaskService_Duplicate(t *testing.T) {
	taskRepo := newMockTaskRepo()
	projRepo := newMockProjectRepo()
	statusRepo := newMockTaskStatusRepo()
	cycleRepo := newMockCycleRepo()
	userRepo := newMockUserRepo()

	projRepo.projectsByID["proj-1"] = &domain.Project{ID: "proj-1", OrgID: "org-1", Slug: "p"}
	statusRepo.statusesByID["status-1"] = &domain.TaskStatus{ID: "status-1", ProjectID: "proj-1"}
	userRepo.usersByID["user-1"] = &domain.User{ID: "user-1", OrgID: "org-1", IsActive: true}

	parentID := "t1"
	taskRepo.tasksByID["t1"] = &domain.Task{
		ID: "t1", OrgID: "org-1", ProjectID: "proj-1",
		Title: "Fix bug", Description: "details", StatusID: "status-1", Priority: "high",
		Assignees: []domain.TaskAssignee{{ID: "user-1"}},
	}
	taskRepo.tasksByID["t2"] = &domain.Task{
		ID: "t2", OrgID: "org-1", ProjectID: "proj-1",
		Title: "Subtask", Description: "sub", StatusID: "status-1", Priority: "low",
		ParentID:  &parentID,
		Assignees: []domain.TaskAssignee{{ID: "user-1"}},
	}

	svc := NewTaskService(taskRepo, projRepo, statusRepo, cycleRepo, newMockNotificationService(), userRepo, newMockConversationRepo())

	dup, err := svc.Duplicate(context.Background(), "org-1", "t1", "proj-1", false, "user-1")
	if err != nil {
		t.Fatalf("Duplicate() error = %v", err)
	}
	if dup == nil || dup.ID == "t1" {
		t.Fatal("Duplicate returned the same task or nil")
	}
	if dup.Title != "Fix bug (copy)" {
		t.Errorf("Title = %q, want 'Fix bug (copy)'", dup.Title)
	}
	if dup.Description != "details" {
		t.Errorf("Description = %q, want 'details'", dup.Description)
	}
	if dup.Priority != "high" {
		t.Errorf("Priority = %q, want 'high'", dup.Priority)
	}
	// Both the original and the copy should exist in the repo.
	if _, ok := taskRepo.tasksByID[dup.ID]; !ok {
		t.Error("duplicate not persisted")
	}
	if _, ok := taskRepo.tasksByID["t1"]; !ok {
		t.Error("original was removed by duplication")
	}

	// Duplicate a subtask: the copy should be a top-level task (no ParentID).
	dupSub, err := svc.Duplicate(context.Background(), "org-1", "t2", "proj-1", false, "user-1")
	if err != nil {
		t.Fatalf("Duplicate(subtask) error = %v", err)
	}
	if dupSub.Title != "Subtask (copy)" {
		t.Errorf("Title = %q, want 'Subtask (copy)'", dupSub.Title)
	}
	if dupSub.ParentID != nil {
		t.Errorf("ParentID = %v, want nil (duplicate is top-level, not a child of the source's parent)", dupSub.ParentID)
	}
	// The original subtask still has its parent.
	origSub, ok := taskRepo.tasksByID["t2"]
	if !ok {
		t.Fatal("original subtask was removed by duplication")
	}
	if origSub.ParentID == nil || *origSub.ParentID != "t1" {
		t.Error("original subtask lost its ParentID")
	}
}

func TestTaskService_MoveToProject(t *testing.T) {
	taskRepo := newMockTaskRepo()
	projRepo := newMockProjectRepo()
	statusRepo := newMockTaskStatusRepo()
	cycleRepo := newMockCycleRepo()

	projRepo.projectsByID["proj-1"] = &domain.Project{ID: "proj-1", OrgID: "org-1", Slug: "p1"}
	projRepo.projectsByID["proj-2"] = &domain.Project{ID: "proj-2", OrgID: "org-1", Slug: "p2"}
	statusRepo.statusesByID["status-src"] = &domain.TaskStatus{ID: "status-src", ProjectID: "proj-1"}
	statusRepo.statusesByID["status-dst"] = &domain.TaskStatus{ID: "status-dst", ProjectID: "proj-2"}

	cyc := "cyc-1"
	taskRepo.tasksByID["t1"] = &domain.Task{
		ID: "t1", OrgID: "org-1", ProjectID: "proj-1",
		Title: "Move me", StatusID: "status-src", Priority: "none", CycleID: &cyc,
	}

	svc := NewTaskService(taskRepo, projRepo, statusRepo, cycleRepo, newMockNotificationService(), newMockUserRepo(), newMockConversationRepo())

	moved, err := svc.MoveToProject(context.Background(), "org-1", "t1", "proj-1", "proj-2", "status-dst", "user-1")
	if err != nil {
		t.Fatalf("MoveToProject() error = %v", err)
	}
	if moved.ProjectID != "proj-2" {
		t.Errorf("ProjectID = %q, want proj-2", moved.ProjectID)
	}
	if moved.StatusID != "status-dst" {
		t.Errorf("StatusID = %q, want status-dst", moved.StatusID)
	}
	if moved.CycleID != nil {
		t.Errorf("CycleID = %v, want nil (cleared on move)", moved.CycleID)
	}
	if moved.PositionKey == "" {
		t.Error("PositionKey empty, want a fresh key")
	}
}

func TestTaskService_MoveToProject_RejectsSameProject(t *testing.T) {
	taskRepo := newMockTaskRepo()
	projRepo := newMockProjectRepo()
	statusRepo := newMockTaskStatusRepo()
	cycleRepo := newMockCycleRepo()
	projRepo.projectsByID["proj-1"] = &domain.Project{ID: "proj-1", OrgID: "org-1", Slug: "p1"}
	statusRepo.statusesByID["status-1"] = &domain.TaskStatus{ID: "status-1", ProjectID: "proj-1"}
	taskRepo.tasksByID["t1"] = &domain.Task{ID: "t1", OrgID: "org-1", ProjectID: "proj-1", Title: "T", StatusID: "status-1"}

	svc := NewTaskService(taskRepo, projRepo, statusRepo, cycleRepo, newMockNotificationService(), newMockUserRepo(), newMockConversationRepo())

	_, err := svc.MoveToProject(context.Background(), "org-1", "t1", "proj-1", "proj-1", "status-1", "user-1")
	if err == nil {
		t.Fatal("MoveToProject same project = nil error, want validation error")
	}
}

func TestTaskService_Create_SetsCompletedAtForDoneStatus(t *testing.T) {
	taskRepo := newMockTaskRepo()
	projRepo := newMockProjectRepo()
	statusRepo := newMockTaskStatusRepo()

	projRepo.projectsByID["proj-1"] = &domain.Project{ID: "proj-1", OrgID: "org-1", Slug: "p"}
	statusRepo.statusesByID["status-done"] = &domain.TaskStatus{
		ID: "status-done", ProjectID: "proj-1", Name: "Done", Color: "#22c55e", Category: "done",
	}

	svc := NewTaskService(taskRepo, projRepo, statusRepo, newMockCycleRepo(), newMockNotificationService(), newMockUserRepo(), newMockConversationRepo())
	task, err := svc.Create(context.Background(), domain.CreateTaskParams{
		OrgID:     "org-1",
		ProjectID: "proj-1",
		CreatedBy: "user-1",
		Title:     "Already done",
		StatusID:  "status-done",
		Priority:  "none",
	})
	if err != nil {
		t.Fatalf("Create() error = %v", err)
	}
	if task.CompletedAt == nil {
		t.Fatal("expected CompletedAt to be set for done status")
	}
}

func TestTaskService_Update_SyncsCompletedAtOnStatusTransition(t *testing.T) {
	taskRepo := newMockTaskRepo()
	projRepo := newMockProjectRepo()
	statusRepo := newMockTaskStatusRepo()

	projRepo.projectsByID["proj-1"] = &domain.Project{ID: "proj-1", OrgID: "org-1", Slug: "p"}
	statusRepo.statusesByID["status-todo"] = &domain.TaskStatus{
		ID: "status-todo", ProjectID: "proj-1", Name: "Todo", Color: "#3b82f6", Category: "todo",
	}
	statusRepo.statusesByID["status-done"] = &domain.TaskStatus{
		ID: "status-done", ProjectID: "proj-1", Name: "Done", Color: "#22c55e", Category: "done",
	}

	taskRepo.tasksByID["task-1"] = &domain.Task{
		ID: "task-1", OrgID: "org-1", ProjectID: "proj-1",
		Title: "Task", StatusID: "status-todo", Priority: "none", PositionKey: "a0",
	}

	svc := NewTaskService(taskRepo, projRepo, statusRepo, newMockCycleRepo(), newMockNotificationService(), newMockUserRepo(), newMockConversationRepo())

	err := svc.Update(context.Background(), "user-2", &domain.Task{
		ID: "task-1", OrgID: "org-1", ProjectID: "proj-1",
		Title: "Task", StatusID: "status-done", Priority: "none", PositionKey: "a0",
	})
	if err != nil {
		t.Fatalf("Update() error = %v", err)
	}

	updated := taskRepo.tasksByID["task-1"]
	if updated.CompletedAt == nil {
		t.Fatal("expected CompletedAt to be set after moving to done")
	}

	// Move back to todo: CompletedAt should clear.
	err = svc.Update(context.Background(), "user-2", &domain.Task{
		ID: "task-1", OrgID: "org-1", ProjectID: "proj-1",
		Title: "Task", StatusID: "status-todo", Priority: "none", PositionKey: "a0",
	})
	if err != nil {
		t.Fatalf("Update() error = %v", err)
	}
	updated = taskRepo.tasksByID["task-1"]
	if updated.CompletedAt != nil {
		t.Fatalf("expected CompletedAt to be cleared after moving back to todo, got %v", updated.CompletedAt)
	}
}

func TestTaskService_BatchUpdate_SyncsCompletedAt(t *testing.T) {
	taskRepo := newMockTaskRepo()
	projRepo := newMockProjectRepo()
	statusRepo := newMockTaskStatusRepo()

	projRepo.projectsByID["proj-1"] = &domain.Project{ID: "proj-1", OrgID: "org-1", Slug: "p"}
	statusRepo.statusesByID["status-todo"] = &domain.TaskStatus{
		ID: "status-todo", ProjectID: "proj-1", Name: "Todo", Color: "#3b82f6", Category: "todo",
	}
	statusRepo.statusesByID["status-done"] = &domain.TaskStatus{
		ID: "status-done", ProjectID: "proj-1", Name: "Done", Color: "#22c55e", Category: "done",
	}

	taskRepo.tasksByID["task-1"] = &domain.Task{
		ID: "task-1", OrgID: "org-1", ProjectID: "proj-1",
		Title: "Task", StatusID: "status-todo", Priority: "none", PositionKey: "a0",
	}

	svc := NewTaskService(taskRepo, projRepo, statusRepo, newMockCycleRepo(), newMockNotificationService(), newMockUserRepo(), newMockConversationRepo())
	done := "status-done"
	if _, err := svc.BatchUpdate(context.Background(), "org-1", domain.BatchUpdateParams{
		TaskIDs:   []string{"task-1"},
		ProjectID: "proj-1",
		StatusID:  &done,
	}, "user-1"); err != nil {
		t.Fatalf("BatchUpdate() error = %v", err)
	}

	if taskRepo.tasksByID["task-1"].CompletedAt == nil {
		t.Fatal("expected CompletedAt to be set after batch update to done")
	}
}

func TestTaskService_Move_SyncsCompletedAt(t *testing.T) {
	taskRepo := newMockTaskRepo()
	projRepo := newMockProjectRepo()
	statusRepo := newMockTaskStatusRepo()

	projRepo.projectsByID["proj-1"] = &domain.Project{ID: "proj-1", OrgID: "org-1", Slug: "p"}
	statusRepo.statusesByID["status-todo"] = &domain.TaskStatus{
		ID: "status-todo", ProjectID: "proj-1", Name: "Todo", Color: "#3b82f6", Category: "todo",
	}
	statusRepo.statusesByID["status-done"] = &domain.TaskStatus{
		ID: "status-done", ProjectID: "proj-1", Name: "Done", Color: "#22c55e", Category: "done",
	}

	taskRepo.tasksByID["task-1"] = &domain.Task{
		ID: "task-1", OrgID: "org-1", ProjectID: "proj-1",
		Title: "Task", StatusID: "status-todo", Priority: "none", PositionKey: "a0",
	}

	svc := NewTaskService(taskRepo, projRepo, statusRepo, newMockCycleRepo(), newMockNotificationService(), newMockUserRepo(), newMockConversationRepo())
	if err := svc.Move(context.Background(), "user-1", "org-1", "task-1", "proj-1", "status-done", "b0"); err != nil {
		t.Fatalf("Move() error = %v", err)
	}
	if taskRepo.tasksByID["task-1"].CompletedAt == nil {
		t.Fatal("expected CompletedAt to be set after moving to done")
	}
}

func TestTaskService_Update_NotificationActorIsCurrentUser(t *testing.T) {
	taskRepo := newMockTaskRepo()
	projRepo := newMockProjectRepo()
	statusRepo := newMockTaskStatusRepo()
	notifSvc := newMockNotificationService()
	userRepo := newMockUserRepo()

	projRepo.projectsByID["proj-1"] = &domain.Project{ID: "proj-1", OrgID: "org-1", Slug: "p"}
	statusRepo.statusesByID["status-todo"] = &domain.TaskStatus{
		ID: "status-todo", ProjectID: "proj-1", Name: "Todo", Color: "#3b82f6", Category: "todo",
	}
	statusRepo.statusesByID["status-done"] = &domain.TaskStatus{
		ID: "status-done", ProjectID: "proj-1", Name: "Done", Color: "#22c55e", Category: "done",
	}
	userRepo.usersByID["assignee-1"] = &domain.User{ID: "assignee-1", OrgID: "org-1", IsActive: true}

	taskRepo.tasksByID["task-1"] = &domain.Task{
		ID: "task-1", OrgID: "org-1", ProjectID: "proj-1",
		Title: "Task", StatusID: "status-todo", Priority: "none", PositionKey: "a0",
		CreatedBy: "creator",
		Assignees: []domain.TaskAssignee{{ID: "assignee-1"}},
	}

	svc := NewTaskService(taskRepo, projRepo, statusRepo, newMockCycleRepo(), notifSvc, userRepo, newMockConversationRepo())

	if err := svc.Update(context.Background(), "editor-123", &domain.Task{
		ID: "task-1", OrgID: "org-1", ProjectID: "proj-1",
		Title: "Task", StatusID: "status-done", Priority: "none", PositionKey: "a0",
		Assignees: []domain.TaskAssignee{{ID: "assignee-1"}},
	}); err != nil {
		t.Fatalf("Update() error = %v", err)
	}

	if len(notifSvc.notifications) == 0 {
		t.Fatalf("expected a status-change notification, got %d", len(notifSvc.notifications))
	}
	if notifSvc.notifications[0].ActorID != "editor-123" {
		t.Errorf("notification actor = %q, want %q", notifSvc.notifications[0].ActorID, "editor-123")
	}
}

func TestTaskService_BroadcastsTaskEvents(t *testing.T) {
	taskRepo := newMockTaskRepo()
	projRepo := newMockProjectRepo()
	statusRepo := newMockTaskStatusRepo()

	projRepo.projectsByID["proj-1"] = &domain.Project{ID: "proj-1", OrgID: "org-1", Slug: "proj-1"}
	statusRepo.statusesByID["status-todo"] = &domain.TaskStatus{ID: "status-todo", ProjectID: "proj-1", Name: "Todo", Category: "todo", Position: 0}
	statusRepo.statusesByID["status-done"] = &domain.TaskStatus{ID: "status-done", ProjectID: "proj-1", Name: "Done", Category: "done", Position: 1}

	bc := newMockBroadcaster()
	svc := NewTaskServiceWithDeps(TaskServiceDeps{
		TaskRepo:    taskRepo,
		ProjRepo:    projRepo,
		StatusRepo:  statusRepo,
		CycleRepo:   newMockCycleRepo(),
		NotifSvc:    newMockNotificationService(),
		UserRepo:    newMockUserRepo(),
		ConvRepo:    newMockConversationRepo(),
		Broadcaster: bc,
	})

	// Create → task_created broadcast
	task, err := svc.Create(context.Background(), domain.CreateTaskParams{
		OrgID: "org-1", ProjectID: "proj-1", CreatedBy: "user-1",
		Title: "New Task", StatusID: "status-todo", Priority: "none",
	})
	if err != nil {
		t.Fatalf("Create() error = %v", err)
	}
	if got := findBroadcast(bc, "task_created"); got == nil {
		t.Errorf("expected task_created broadcast, got %d messages", len(bc.messages))
	}

	// Update → task_updated broadcast
	if err := svc.Update(context.Background(), "user-1", &domain.Task{
		ID: task.ID, OrgID: "org-1", ProjectID: "proj-1",
		Title: "Updated", StatusID: "status-done", Priority: "none", PositionKey: "a0",
	}); err != nil {
		t.Fatalf("Update() error = %v", err)
	}
	if got := findBroadcast(bc, "task_updated"); got == nil {
		t.Errorf("expected task_updated broadcast, got %d messages", len(bc.messages))
	}

	// Move → task_moved broadcast
	if err := svc.Move(context.Background(), "user-1", "org-1", task.ID, "proj-1", "status-todo", "b0"); err != nil {
		t.Fatalf("Move() error = %v", err)
	}
	if got := findBroadcast(bc, "task_moved"); got == nil {
		t.Errorf("expected task_moved broadcast, got %d messages", len(bc.messages))
	}

	// Delete → task_deleted broadcast
	if err := svc.Delete(context.Background(), "org-1", task.ID, "proj-1", domain.DeleteSubtaskModeBlock, "user-1"); err != nil {
		t.Fatalf("Delete() error = %v", err)
	}
	if got := findBroadcast(bc, "task_deleted"); got == nil {
		t.Errorf("expected task_deleted broadcast, got %d messages", len(bc.messages))
	}
}

func findBroadcast(bc *mockBroadcaster, eventType string) any {
	for _, m := range bc.messages {
		if m.eventType == eventType {
			return m
		}
	}
	return nil
}

func TestTaskService_RecordsActivityOnMutations(t *testing.T) {
	taskRepo := newMockTaskRepo()
	projRepo := newMockProjectRepo()
	statusRepo := newMockTaskStatusRepo()

	projRepo.projectsByID["proj-1"] = &domain.Project{ID: "proj-1", OrgID: "org-1", Slug: "proj-1"}
	statusRepo.statusesByID["status-todo"] = &domain.TaskStatus{ID: "status-todo", ProjectID: "proj-1", Name: "Todo", Category: "todo", Position: 0}
	statusRepo.statusesByID["status-done"] = &domain.TaskStatus{ID: "status-done", ProjectID: "proj-1", Name: "Done", Category: "done", Position: 1}

	cycleRepo := newMockCycleRepo()
	cycleRepo.cyclesByID["cyc-sprint"] = &domain.Cycle{ID: "cyc-sprint", ProjectID: "proj-1", Name: "Sprint 1"}

	activityRepo := newMockTaskActivityRepo()
	svc := NewTaskServiceWithDeps(TaskServiceDeps{
		TaskRepo:     taskRepo,
		ProjRepo:     projRepo,
		StatusRepo:   statusRepo,
		CycleRepo:    cycleRepo,
		NotifSvc:     newMockNotificationService(),
		UserRepo:     newMockUserRepo(),
		ConvRepo:     newMockConversationRepo(),
		ActivityRepo: activityRepo,
	})

	// Create → records an "created" activity
	task, err := svc.Create(context.Background(), domain.CreateTaskParams{
		OrgID: "org-1", ProjectID: "proj-1", CreatedBy: "user-1",
		Title: "My Task", Description: "Initial short description", StatusID: "status-todo", Priority: "none",
	})
	if err != nil {
		t.Fatalf("Create() error = %v", err)
	}
	if len(activityRepo.entries) != 1 || activityRepo.entries[0].Action != domain.ActivityCreated {
		t.Fatalf("expected 1 created activity, got %d entries", len(activityRepo.entries))
	}

	startTime := time.Date(2025, 6, 15, 10, 0, 0, 0, time.UTC)

	// Build a long description (>200 chars) to test truncation.
	longDesc := "Lorem ipsum dolor sit amet, consectetur adipiscing elit. " +
		"Sed do eiusmod tempor incididunt ut labore et dolore magna aliqua. " +
		"Ut enim ad minim veniam, quis nostrud exercitation ullamco laboris nisi ut aliquip."

	// Update status, priority, estimate, cycle, description, and started_at → records the corresponding activity entries.
	if err := svc.Update(context.Background(), "user-2", &domain.Task{
		ID: task.ID, OrgID: "org-1", ProjectID: "proj-1",
		Title: "My Task", Description: longDesc, StatusID: "status-done", Priority: "high", PositionKey: "a0",
		Estimate: intPtr(5), CycleID: ptrString("cyc-sprint"), StartedAt: &startTime,
	}); err != nil {
		t.Fatalf("Update() error = %v", err)
	}

	// Assert status_changed entry has human-readable names (not raw IDs)
	var statusEntry *domain.TaskActivity
	for _, e := range activityRepo.entries {
		if e.Action == domain.ActivityStatusChanged && e.ActorID == "user-2" {
			statusEntry = e
			break
		}
	}
	if statusEntry == nil {
		t.Fatalf("expected a status_changed activity by user-2, entries: %+v", activityRepo.entries)
	}
	if statusEntry.OldValue != "Todo" {
		t.Errorf("status_changed OldValue = %q, want %q", statusEntry.OldValue, "Todo")
	}
	if statusEntry.NewValue != "Done" {
		t.Errorf("status_changed NewValue = %q, want %q", statusEntry.NewValue, "Done")
	}

	// Assert priority_changed entry has capitalized display name
	var prioEntry *domain.TaskActivity
	for _, e := range activityRepo.entries {
		if e.Action == domain.ActivityPriorityChanged {
			prioEntry = e
			break
		}
	}
	if prioEntry == nil {
		t.Fatalf("expected a priority_changed activity, entries: %+v", activityRepo.entries)
	}
	if prioEntry.OldValue != "None" {
		t.Errorf("priority_changed OldValue = %q, want %q", prioEntry.OldValue, "None")
	}
	if prioEntry.NewValue != "High" {
		t.Errorf("priority_changed NewValue = %q, want %q", prioEntry.NewValue, "High")
	}

	// Assert estimate_changed entry has human-readable int value.
	var estEntry *domain.TaskActivity
	for _, e := range activityRepo.entries {
		if e.Action == domain.ActivityEstimateChanged {
			estEntry = e
			break
		}
	}
	if estEntry == nil {
		t.Fatalf("expected an estimate_changed activity, entries: %+v", activityRepo.entries)
	}
	if estEntry.OldValue != "" {
		t.Errorf("estimate_changed OldValue = %q, want %q", estEntry.OldValue, "")
	}
	if estEntry.NewValue != "5" {
		t.Errorf("estimate_changed NewValue = %q, want %q", estEntry.NewValue, "5")
	}

	// Assert cycle_changed entry has cycle NAME (not raw ID).
	var cycleEntry *domain.TaskActivity
	for _, e := range activityRepo.entries {
		if e.Action == domain.ActivityCycleChanged {
			cycleEntry = e
			break
		}
	}
	if cycleEntry == nil {
		t.Fatalf("expected a cycle_changed activity, entries: %+v", activityRepo.entries)
	}
	if cycleEntry.OldValue != "" {
		t.Errorf("cycle_changed OldValue = %q, want %q", cycleEntry.OldValue, "")
	}
	if cycleEntry.NewValue != "Sprint 1" {
		t.Errorf("cycle_changed NewValue = %q, want %q", cycleEntry.NewValue, "Sprint 1")
	}

	// Assert started_at_changed entry has a friendly date.
	var startedEntry *domain.TaskActivity
	for _, e := range activityRepo.entries {
		if e.Action == domain.ActivityStartedAtChanged {
			startedEntry = e
			break
		}
	}
	if startedEntry == nil {
		t.Fatalf("expected a started_at_changed activity, entries: %+v", activityRepo.entries)
	}
	if startedEntry.OldValue != "" {
		t.Errorf("started_at_changed OldValue = %q, want empty", startedEntry.OldValue)
	}
	if startedEntry.NewValue != "Jun 15, 2025" {
		t.Errorf("started_at_changed NewValue = %q, want %q", startedEntry.NewValue, "Jun 15, 2025")
	}

	// Assert description_changed entry is truncated (not full text).
	var descEntry *domain.TaskActivity
	for _, e := range activityRepo.entries {
		if e.Action == domain.ActivityDescriptionChanged {
			descEntry = e
			break
		}
	}
	if descEntry == nil {
		t.Fatalf("expected a description_changed activity, entries: %+v", activityRepo.entries)
	}
	if descEntry.OldValue != "Initial short description" {
		t.Errorf("description_changed OldValue = %q, want %q", descEntry.OldValue, "Initial short description")
	}
	if len(descEntry.NewValue) > 125 {
		t.Errorf("description_changed NewValue length = %d, want <= 125 (truncated)", len(descEntry.NewValue))
	}
	if !strings.HasSuffix(descEntry.NewValue, "…") {
		t.Errorf("description_changed NewValue = %q, want suffix '…'", descEntry.NewValue)
	}
}

func TestTaskService_MoveToProject_MovesSubtasks(t *testing.T) {
	taskRepo := newMockTaskRepo()
	projRepo := newMockProjectRepo()
	statusRepo := newMockTaskStatusRepo()
	cycleRepo := newMockCycleRepo()

	projRepo.projectsByID["proj-1"] = &domain.Project{ID: "proj-1", OrgID: "org-1", Slug: "p1"}
	projRepo.projectsByID["proj-2"] = &domain.Project{ID: "proj-2", OrgID: "org-1", Slug: "p2"}
	statusRepo.statusesByID["status-src"] = &domain.TaskStatus{ID: "status-src", ProjectID: "proj-1", Category: "todo"}
	statusRepo.statusesByID["status-dst"] = &domain.TaskStatus{ID: "status-dst", ProjectID: "proj-2", Category: "todo"}

	parentID := "parent-1"
	taskRepo.tasksByID["parent-1"] = &domain.Task{
		ID: "parent-1", OrgID: "org-1", ProjectID: "proj-1",
		Title: "Parent", StatusID: "status-src", Priority: "none",
	}
	taskRepo.tasksByID["child-1"] = &domain.Task{
		ID: "child-1", OrgID: "org-1", ProjectID: "proj-1",
		Title: "Child 1", StatusID: "status-src", Priority: "none",
		ParentID: &parentID,
	}
	// Nested subtask (grandchild)
	taskRepo.tasksByID["grandchild-1"] = &domain.Task{
		ID: "grandchild-1", OrgID: "org-1", ProjectID: "proj-1",
		Title: "Grandchild", StatusID: "status-src", Priority: "none",
		ParentID: ptrString("child-1"),
	}

	svc := NewTaskService(taskRepo, projRepo, statusRepo, cycleRepo, newMockNotificationService(), newMockUserRepo(), newMockConversationRepo())

	_, err := svc.MoveToProject(context.Background(), "org-1", "parent-1", "proj-1", "proj-2", "status-dst", "user-1")
	if err != nil {
		t.Fatalf("MoveToProject() error = %v", err)
	}

	// Parent moved
	if taskRepo.tasksByID["parent-1"].ProjectID != "proj-2" {
		t.Errorf("parent ProjectID = %q, want proj-2", taskRepo.tasksByID["parent-1"].ProjectID)
	}
	// Child moved + still linked to parent
	child := taskRepo.tasksByID["child-1"]
	if child.ProjectID != "proj-2" {
		t.Errorf("child ProjectID = %q, want proj-2", child.ProjectID)
	}
	if child.ParentID == nil || *child.ParentID != "parent-1" {
		t.Errorf("child ParentID = %v, want parent-1", child.ParentID)
	}
	// Grandchild moved + still linked to child
	gc := taskRepo.tasksByID["grandchild-1"]
	if gc.ProjectID != "proj-2" {
		t.Errorf("grandchild ProjectID = %q, want proj-2", gc.ProjectID)
	}
	if gc.ParentID == nil || *gc.ParentID != "child-1" {
		t.Errorf("grandchild ParentID = %v, want child-1", gc.ParentID)
	}
}

func TestTaskService_MoveToProject_SyncsSubtaskCompletedAt(t *testing.T) {
	now := time.Now()

	t.Run("done_subtask_moves_to_todo_clears_completed_at", func(t *testing.T) {
		taskRepo := newMockTaskRepo()
		projRepo := newMockProjectRepo()
		statusRepo := newMockTaskStatusRepo()
		cycleRepo := newMockCycleRepo()

		projRepo.projectsByID["proj-1"] = &domain.Project{ID: "proj-1", OrgID: "org-1", Slug: "p1"}
		projRepo.projectsByID["proj-2"] = &domain.Project{ID: "proj-2", OrgID: "org-1", Slug: "p2"}
		statusRepo.statusesByID["status-todo"] = &domain.TaskStatus{ID: "status-todo", ProjectID: "proj-1", Category: "todo"}
		statusRepo.statusesByID["status-done"] = &domain.TaskStatus{ID: "status-done", ProjectID: "proj-1", Category: "done"}
		statusRepo.statusesByID["status-dst"] = &domain.TaskStatus{ID: "status-dst", ProjectID: "proj-2", Category: "todo"}

		parentID := "parent-1"
		taskRepo.tasksByID["parent-1"] = &domain.Task{
			ID: "parent-1", OrgID: "org-1", ProjectID: "proj-1",
			Title: "Parent", StatusID: "status-todo", Priority: "none",
		}
		taskRepo.tasksByID["child-done"] = &domain.Task{
			ID: "child-done", OrgID: "org-1", ProjectID: "proj-1",
			Title: "Child Done", StatusID: "status-done", Priority: "none",
			ParentID: &parentID, CompletedAt: &now,
		}

		svc := NewTaskService(taskRepo, projRepo, statusRepo, cycleRepo, newMockNotificationService(), newMockUserRepo(), newMockConversationRepo())

		_, err := svc.MoveToProject(context.Background(), "org-1", "parent-1", "proj-1", "proj-2", "status-dst", "user-1")
		if err != nil {
			t.Fatalf("MoveToProject() error = %v", err)
		}

		child := taskRepo.tasksByID["child-done"]
		if child.CompletedAt != nil {
			t.Errorf("child.CompletedAt = %v, want nil (done subtask moved to todo should clear completed_at)", child.CompletedAt)
		}
	})

	t.Run("todo_subtask_moves_to_done_sets_completed_at", func(t *testing.T) {
		taskRepo := newMockTaskRepo()
		projRepo := newMockProjectRepo()
		statusRepo := newMockTaskStatusRepo()
		cycleRepo := newMockCycleRepo()

		projRepo.projectsByID["proj-1"] = &domain.Project{ID: "proj-1", OrgID: "org-1", Slug: "p1"}
		projRepo.projectsByID["proj-2"] = &domain.Project{ID: "proj-2", OrgID: "org-1", Slug: "p2"}
		statusRepo.statusesByID["status-todo"] = &domain.TaskStatus{ID: "status-todo", ProjectID: "proj-1", Category: "todo"}
		statusRepo.statusesByID["status-dst-done"] = &domain.TaskStatus{ID: "status-dst-done", ProjectID: "proj-2", Category: "done"}

		parentID := "parent-2"
		taskRepo.tasksByID["parent-2"] = &domain.Task{
			ID: "parent-2", OrgID: "org-1", ProjectID: "proj-1",
			Title: "Parent 2", StatusID: "status-todo", Priority: "none",
		}
		taskRepo.tasksByID["child-todo"] = &domain.Task{
			ID: "child-todo", OrgID: "org-1", ProjectID: "proj-1",
			Title: "Child Todo", StatusID: "status-todo", Priority: "none",
			ParentID: &parentID,
		}

		svc := NewTaskService(taskRepo, projRepo, statusRepo, cycleRepo, newMockNotificationService(), newMockUserRepo(), newMockConversationRepo())

		_, err := svc.MoveToProject(context.Background(), "org-1", "parent-2", "proj-1", "proj-2", "status-dst-done", "user-1")
		if err != nil {
			t.Fatalf("MoveToProject() error = %v", err)
		}

		child := taskRepo.tasksByID["child-todo"]
		if child.CompletedAt == nil {
			t.Errorf("child.CompletedAt = nil, want non-nil (todo subtask moved to done should set completed_at)")
		}
	})
}

func ptrString(s string) *string { return &s }

func TestTaskService_Create_RejectsInvalidAssignee(t *testing.T) {
	taskRepo := newMockTaskRepo()
	projRepo := newMockProjectRepo()
	statusRepo := newMockTaskStatusRepo()
	cycleRepo := newMockCycleRepo()
	userRepo := newMockUserRepo()

	projRepo.projectsByID["proj-1"] = &domain.Project{ID: "proj-1", OrgID: "org-1", Slug: "p1"}
	statusRepo.statusesByID["s1"] = &domain.TaskStatus{ID: "s1", ProjectID: "proj-1", Category: "todo"}
	// Only user-1 exists; user-bogus does not.
	userRepo.usersByID["user-1"] = &domain.User{ID: "user-1", OrgID: "org-1", IsActive: true}

	svc := NewTaskService(taskRepo, projRepo, statusRepo, cycleRepo, newMockNotificationService(), userRepo, newMockConversationRepo())

	_, err := svc.Create(context.Background(), domain.CreateTaskParams{
		OrgID: "org-1", ProjectID: "proj-1", Title: "Test", StatusID: "s1",
		AssigneeIDs: []string{"user-1", "user-bogus"},
	})
	if err == nil {
		t.Fatal("expected error for invalid assignee, got nil")
	}
}

func TestTaskService_Update_RejectsInvalidAssignee(t *testing.T) {
	taskRepo := newMockTaskRepo()
	projRepo := newMockProjectRepo()
	statusRepo := newMockTaskStatusRepo()
	cycleRepo := newMockCycleRepo()
	userRepo := newMockUserRepo()

	projRepo.projectsByID["proj-1"] = &domain.Project{ID: "proj-1", OrgID: "org-1", Slug: "p1"}
	statusRepo.statusesByID["s1"] = &domain.TaskStatus{ID: "s1", ProjectID: "proj-1", Category: "todo"}
	userRepo.usersByID["user-1"] = &domain.User{ID: "user-1", OrgID: "org-1", IsActive: true}

	taskRepo.tasksByID["t1"] = &domain.Task{
		ID: "t1", OrgID: "org-1", ProjectID: "proj-1",
		Title: "Test", StatusID: "s1", Priority: "none",
	}

	svc := NewTaskService(taskRepo, projRepo, statusRepo, cycleRepo, newMockNotificationService(), userRepo, newMockConversationRepo())

	err := svc.Update(context.Background(), "actor-1", &domain.Task{
		ID: "t1", OrgID: "org-1", ProjectID: "proj-1",
		Title: "Test", StatusID: "s1", Priority: "none",
		Assignees: []domain.TaskAssignee{{ID: "user-bogus"}},
	})
	if err == nil {
		t.Fatal("expected error for invalid assignee, got nil")
	}
}

func TestTaskService_Update_RejectsStatusFromDifferentProject(t *testing.T) {
	taskRepo := newMockTaskRepo()
	projRepo := newMockProjectRepo()
	statusRepo := newMockTaskStatusRepo()
	cycleRepo := newMockCycleRepo()
	userRepo := newMockUserRepo()

	projRepo.projectsByID["proj-1"] = &domain.Project{ID: "proj-1", OrgID: "org-1", Slug: "p1"}
	projRepo.projectsByID["proj-2"] = &domain.Project{ID: "proj-2", OrgID: "org-1", Slug: "p2"}
	// s1 belongs to proj-1, s2 belongs to proj-2
	statusRepo.statusesByID["s1"] = &domain.TaskStatus{ID: "s1", ProjectID: "proj-1", Category: "todo"}
	statusRepo.statusesByID["s2"] = &domain.TaskStatus{ID: "s2", ProjectID: "proj-2", Category: "done"}

	taskRepo.tasksByID["t1"] = &domain.Task{
		ID: "t1", OrgID: "org-1", ProjectID: "proj-1",
		Title: "Test", StatusID: "s1", Priority: "none",
	}

	svc := NewTaskService(taskRepo, projRepo, statusRepo, cycleRepo, newMockNotificationService(), userRepo, newMockConversationRepo())

	// Try to update task in proj-1 with a status from proj-2
	err := svc.Update(context.Background(), "actor-1", &domain.Task{
		ID: "t1", OrgID: "org-1", ProjectID: "proj-1",
		Title: "Test", StatusID: "s2", Priority: "none",
	})
	if err == nil {
		t.Fatal("expected error for status from different project, got nil")
	}
}

func TestTaskService_RecordsActivityOnReparented(t *testing.T) {
	taskRepo := newMockTaskRepo()
	projRepo := newMockProjectRepo()
	statusRepo := newMockTaskStatusRepo()
	cycleRepo := newMockCycleRepo()
	activityRepo := newMockTaskActivityRepo()

	projRepo.projectsByID["proj-1"] = &domain.Project{ID: "proj-1", OrgID: "org-1", Slug: "p-1"}
	statusRepo.statusesByID["status-todo"] = &domain.TaskStatus{ID: "status-todo", ProjectID: "proj-1", Name: "Todo", Category: "todo"}

	// Seed a parent task for reparenting.
	taskRepo.tasksByID["parent-1"] = &domain.Task{
		ID: "parent-1", OrgID: "org-1", ProjectID: "proj-1",
		Title: "Parent Task", StatusID: "status-todo", Priority: "none",
	}

	svc := NewTaskServiceWithDeps(TaskServiceDeps{
		TaskRepo:     taskRepo,
		ProjRepo:     projRepo,
		StatusRepo:   statusRepo,
		CycleRepo:    cycleRepo,
		NotifSvc:     newMockNotificationService(),
		UserRepo:     newMockUserRepo(),
		ConvRepo:     newMockConversationRepo(),
		ActivityRepo: activityRepo,
	})

	task, err := svc.Create(context.Background(), domain.CreateTaskParams{
		OrgID: "org-1", ProjectID: "proj-1", CreatedBy: "user-1",
		Title: "Child Task", StatusID: "status-todo", Priority: "none",
	})
	if err != nil {
		t.Fatalf("Create() error = %v", err)
	}

	// Reparent the task to parent-1.
	parentID := "parent-1"
	if err := svc.Update(context.Background(), "user-2", &domain.Task{
		ID: task.ID, OrgID: "org-1", ProjectID: "proj-1",
		Title: "Child Task", StatusID: "status-todo", Priority: "none",
		ParentID: &parentID, PositionKey: "a0",
	}); err != nil {
		t.Fatalf("Update() with parent error = %v", err)
	}

	// Find the reparented activity entry.
	var reparentedEntry *domain.TaskActivity
	for _, e := range activityRepo.entries {
		if e.Action == domain.ActivityReparented {
			reparentedEntry = e
			break
		}
	}
	if reparentedEntry == nil {
		t.Fatalf("expected a reparented activity, entries: %+v", activityRepo.entries)
	}
	if reparentedEntry.OldValue != "" {
		t.Errorf("reparented OldValue = %q, want empty", reparentedEntry.OldValue)
	}
	if reparentedEntry.NewValue != "Parent Task" {
		t.Errorf("reparented NewValue = %q, want %q", reparentedEntry.NewValue, "Parent Task")
	}
}

// TestTaskService_DeleteSkipsCascadeDoomedActivity asserts that Delete does
// NOT write a task_activity entry: task_activity rows cascade-delete with
// their task, so such an entry would be erased the instant the delete
// commits. Deletions are recorded in the audit log by the handler instead.
func TestTaskService_DeleteSkipsCascadeDoomedActivity(t *testing.T) {
	taskRepo := newMockTaskRepo()
	projRepo := newMockProjectRepo()
	statusRepo := newMockTaskStatusRepo()
	cycleRepo := newMockCycleRepo()
	activityRepo := newMockTaskActivityRepo()

	projRepo.projectsByID["proj-1"] = &domain.Project{ID: "proj-1", OrgID: "org-1", Slug: "p-1"}
	statusRepo.statusesByID["status-todo"] = &domain.TaskStatus{ID: "status-todo", ProjectID: "proj-1", Name: "Todo", Category: "todo"}

	svc := NewTaskServiceWithDeps(TaskServiceDeps{
		TaskRepo:     taskRepo,
		ProjRepo:     projRepo,
		StatusRepo:   statusRepo,
		CycleRepo:    cycleRepo,
		NotifSvc:     newMockNotificationService(),
		UserRepo:     newMockUserRepo(),
		ConvRepo:     newMockConversationRepo(),
		Broadcaster:  newMockBroadcaster(),
		ActivityRepo: activityRepo,
		Access:       nil,
	})

	task, err := svc.Create(context.Background(), domain.CreateTaskParams{
		OrgID: "org-1", ProjectID: "proj-1", CreatedBy: "user-1",
		Title: "Delete Me", StatusID: "status-todo", Priority: "none",
	})
	if err != nil {
		t.Fatalf("Create() error = %v", err)
	}
	if len(activityRepo.entries) == 0 {
		t.Fatal("expected create to record activity")
	}

	beforeDelete := len(activityRepo.entries)
	if err := svc.Delete(context.Background(), "org-1", task.ID, "proj-1", domain.DeleteSubtaskModeBlock, "user-2"); err != nil {
		t.Fatalf("Delete() error = %v", err)
	}

	for _, e := range activityRepo.entries[beforeDelete:] {
		if e.Action == domain.ActivityDeleted {
			t.Fatalf("delete must not write a doomed activity entry, got %+v", e)
		}
	}
	if len(activityRepo.entries) != beforeDelete {
		t.Fatalf("expected no new activity entries after delete, got %d", len(activityRepo.entries)-beforeDelete)
	}
}

func TestTaskService_BatchUpdate_RecordsActivityWithStatusNames(t *testing.T) {
	taskRepo := newMockTaskRepo()
	projRepo := newMockProjectRepo()
	statusRepo := newMockTaskStatusRepo()
	cycleRepo := newMockCycleRepo()
	activityRepo := newMockTaskActivityRepo()

	projRepo.projectsByID["proj-1"] = &domain.Project{ID: "proj-1", OrgID: "org-1", Slug: "proj-1"}
	statusRepo.statusesByID["status-todo"] = &domain.TaskStatus{ID: "status-todo", ProjectID: "proj-1", Name: "Todo"}
	statusRepo.statusesByID["status-done"] = &domain.TaskStatus{ID: "status-done", ProjectID: "proj-1", Name: "Done"}

	for _, id := range []string{"t1", "t2"} {
		taskRepo.tasksByID[id] = &domain.Task{
			ID: id, OrgID: "org-1", ProjectID: "proj-1",
			Title: "T", StatusID: "status-todo", Priority: "none",
		}
	}

	svc := NewTaskServiceWithDeps(TaskServiceDeps{
		TaskRepo:     taskRepo,
		ProjRepo:     projRepo,
		StatusRepo:   statusRepo,
		CycleRepo:    cycleRepo,
		NotifSvc:     newMockNotificationService(),
		UserRepo:     newMockUserRepo(),
		ConvRepo:     newMockConversationRepo(),
		ActivityRepo: activityRepo,
	})

	done := "status-done"
	high := "high"
	_, err := svc.BatchUpdate(context.Background(), "org-1", domain.BatchUpdateParams{
		TaskIDs:   []string{"t1", "t2"},
		ProjectID: "proj-1",
		StatusID:  &done,
		Priority:  &high,
	}, "user-batch")
	if err != nil {
		t.Fatalf("BatchUpdate() error = %v", err)
	}

	// Each of 2 tasks gets status_changed + priority_changed = 4 entries.
	if len(activityRepo.entries) != 4 {
		t.Fatalf("expected 4 activity entries (2 tasks × 2 changes), got %d: %+v", len(activityRepo.entries), activityRepo.entries)
	}

	// Assert all status_changed entries use status NAMES (not raw IDs).
	var statusNames []string
	for _, e := range activityRepo.entries {
		if e.Action == domain.ActivityStatusChanged {
			statusNames = append(statusNames, e.OldValue, e.NewValue)
		}
	}
	if len(statusNames) != 4 {
		t.Fatalf("expected 4 status name values (2 tasks × 2), got %d", len(statusNames))
	}
	// Each task moves from "Todo" to "Done"
	for i, name := range statusNames {
		expected := "Todo"
		if i%2 == 1 {
			expected = "Done"
		}
		if name != expected {
			t.Errorf("status_name[%d] = %q, want %q", i, name, expected)
		}
	}

	// Assert all priority_changed entries use capitalized display names.
	for _, e := range activityRepo.entries {
		if e.Action == domain.ActivityPriorityChanged {
			if e.OldValue != "None" {
				t.Errorf("priority_changed OldValue = %q, want %q", e.OldValue, "None")
			}
			if e.NewValue != "High" {
				t.Errorf("priority_changed NewValue = %q, want %q", e.NewValue, "High")
			}
		}
	}
}
