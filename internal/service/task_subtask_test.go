package service

import (
	"context"
	"errors"
	"testing"

	"ipmanlk/breeze/internal/apperr"
	"ipmanlk/breeze/internal/domain"
)

// newSubtaskTestSvc builds a TaskService wired with a project, status, and a
// parent task + one subtask, ready for subtask-behavior tests.
func newSubtaskTestSvc(t *testing.T) (*TaskService, *mockTaskRepo) {
	t.Helper()
	taskRepo := newMockTaskRepo()
	projRepo := newMockProjectRepo()
	statusRepo := newMockTaskStatusRepo()
	cycleRepo := newMockCycleRepo()
	userRepo := newMockUserRepo()

	projRepo.projectsByID["proj-1"] = &domain.Project{ID: "proj-1", OrgID: "org-1", Slug: "p"}
	statusRepo.statusesByID["status-1"] = &domain.TaskStatus{ID: "status-1", ProjectID: "proj-1", Category: domain.StatusCategoryTodo}
	userRepo.usersByID["user-1"] = &domain.User{ID: "user-1", OrgID: "org-1", IsActive: true}

	parentID := "parent-1"
	taskRepo.tasksByID["parent-1"] = &domain.Task{
		ID: "parent-1", OrgID: "org-1", ProjectID: "proj-1",
		Title: "Parent", StatusID: "status-1", Priority: "none", CreatedBy: "user-1",
	}
	taskRepo.tasksByID["child-1"] = &domain.Task{
		ID: "child-1", OrgID: "org-1", ProjectID: "proj-1",
		Title: "Child", StatusID: "status-1", Priority: "none", CreatedBy: "user-1",
		ParentID: &parentID,
	}

	svc := NewTaskService(taskRepo, projRepo, statusRepo, cycleRepo, newMockNotificationService(), userRepo, newMockConversationRepo())
	return svc, taskRepo
}

func TestTaskService_Create_RejectsGrandchild(t *testing.T) {
	svc, _ := newSubtaskTestSvc(t)
	// child-1 already has a parent (parent-1). Creating a task parented to
	// child-1 would create a grandchild (2-level nesting): must be rejected.
	childID := "child-1"
	_, err := svc.Create(context.Background(), domain.CreateTaskParams{
		OrgID: "org-1", ProjectID: "proj-1", CreatedBy: "user-1",
		Title: "Grandchild", StatusID: "status-1", Priority: "none",
		ParentID: &childID,
	})
	if err == nil {
		t.Fatal("expected error creating a grandchild (depth > 1)")
	}
	if !errors.Is(err, apperr.ErrInvalidInput) {
		t.Errorf("expected ErrInvalidInput, got %v", err)
	}
}

func TestTaskService_Create_AcceptsDirectChild(t *testing.T) {
	svc, _ := newSubtaskTestSvc(t)
	parentID := "parent-1"
	// parent-1 has no parent, so a direct child is allowed.
	task, err := svc.Create(context.Background(), domain.CreateTaskParams{
		OrgID: "org-1", ProjectID: "proj-1", CreatedBy: "user-1",
		Title: "New child", StatusID: "status-1", Priority: "none",
		ParentID: &parentID,
	})
	if err != nil {
		t.Fatalf("Create direct child: %v", err)
	}
	if task.ParentID == nil || *task.ParentID != parentID {
		t.Errorf("ParentID = %v, want %q", task.ParentID, parentID)
	}
}

func TestTaskService_Update_RejectsSelfReference(t *testing.T) {
	svc, taskRepo := newSubtaskTestSvc(t)
	parent := taskRepo.tasksByID["parent-1"]
	parent.ID = "parent-1"
	selfID := "parent-1"
	// Setting a task's parent to itself must be rejected.
	err := svc.Update(context.Background(), "user-1", &domain.Task{
		ID: "parent-1", OrgID: "org-1", ProjectID: "proj-1",
		Title: "Parent", StatusID: "status-1", Priority: "none",
		ParentID: &selfID,
	})
	if err == nil {
		t.Fatal("expected error for self-reference parent")
	}
	if !errors.Is(err, apperr.ErrInvalidInput) {
		t.Errorf("expected ErrInvalidInput, got %v", err)
	}
}

func TestTaskService_Update_RejectsReparentToSubtask(t *testing.T) {
	svc, taskRepo := newSubtaskTestSvc(t)
	parent := taskRepo.tasksByID["parent-1"]
	childID := "child-1"
	// Reparenting parent-1 to child-1 (which is itself a subtask) would create
	// a 2-level chain: must be rejected.
	err := svc.Update(context.Background(), "user-1", &domain.Task{
		ID: parent.ID, OrgID: "org-1", ProjectID: "proj-1",
		Title: "Parent", StatusID: "status-1", Priority: "none",
		ParentID: &childID,
	})
	if err == nil {
		t.Fatal("expected error reparenting to a subtask (depth > 1)")
	}
}

func TestTaskService_Update_ReparentsTask(t *testing.T) {
	svc, taskRepo := newSubtaskTestSvc(t)
	// Create a second top-level parent, then reparent child-1 to it.
	taskRepo.tasksByID["parent-2"] = &domain.Task{
		ID: "parent-2", OrgID: "org-1", ProjectID: "proj-1",
		Title: "Parent 2", StatusID: "status-1", Priority: "none", CreatedBy: "user-1",
	}
	newParent := "parent-2"
	err := svc.Update(context.Background(), "user-1", &domain.Task{
		ID: "child-1", OrgID: "org-1", ProjectID: "proj-1",
		Title: "Child", StatusID: "status-1", Priority: "none",
		ParentID: &newParent,
	})
	if err != nil {
		t.Fatalf("Update reparent: %v", err)
	}
	// mockTaskRepo.Update stores the task; verify the parent changed.
	updated := taskRepo.tasksByID["child-1"]
	if updated.ParentID == nil || *updated.ParentID != newParent {
		t.Errorf("ParentID = %v, want %q", updated.ParentID, newParent)
	}
}

func TestTaskService_Update_ClearsParent(t *testing.T) {
	svc, taskRepo := newSubtaskTestSvc(t)
	// Promote child-1 to top-level by clearing its parent.
	err := svc.Update(context.Background(), "user-1", &domain.Task{
		ID: "child-1", OrgID: "org-1", ProjectID: "proj-1",
		Title: "Child", StatusID: "status-1", Priority: "none",
		ParentID: nil,
	})
	if err != nil {
		t.Fatalf("Update clear parent: %v", err)
	}
	updated := taskRepo.tasksByID["child-1"]
	if updated.ParentID != nil {
		t.Errorf("ParentID = %v, want nil", updated.ParentID)
	}
}

func TestTaskService_Delete_BlockMode_ConflictWhenSubtasksExist(t *testing.T) {
	svc, taskRepo := newSubtaskTestSvc(t)
	// parent-1 has child-1; block mode must return conflict.
	err := svc.Delete(context.Background(), "org-1", "parent-1", "proj-1", domain.DeleteSubtaskModeBlock, "user-1")
	if err == nil {
		t.Fatal("expected conflict error for block mode with subtasks")
	}
	if !errors.Is(err, apperr.ErrConflict) {
		t.Errorf("expected ErrConflict, got %v", err)
	}
	// Parent + child should both still exist.
	if _, ok := taskRepo.tasksByID["parent-1"]; !ok {
		t.Error("parent was deleted in block mode")
	}
	if _, ok := taskRepo.tasksByID["child-1"]; !ok {
		t.Error("child was deleted in block mode")
	}
}

func TestTaskService_Delete_CascadeMode_DeletesChildren(t *testing.T) {
	svc, taskRepo := newSubtaskTestSvc(t)
	err := svc.Delete(context.Background(), "org-1", "parent-1", "proj-1", domain.DeleteSubtaskModeCascade, "user-1")
	if err != nil {
		t.Fatalf("Delete cascade: %v", err)
	}
	if _, ok := taskRepo.tasksByID["parent-1"]; ok {
		t.Error("parent should be deleted")
	}
	if _, ok := taskRepo.tasksByID["child-1"]; ok {
		t.Error("child should be deleted in cascade mode")
	}
}

func TestTaskService_Delete_PromoteMode_ChildrenBecomeTopLevel(t *testing.T) {
	svc, taskRepo := newSubtaskTestSvc(t)
	err := svc.Delete(context.Background(), "org-1", "parent-1", "proj-1", domain.DeleteSubtaskModePromote, "user-1")
	if err != nil {
		t.Fatalf("Delete promote: %v", err)
	}
	if _, ok := taskRepo.tasksByID["parent-1"]; ok {
		t.Error("parent should be deleted")
	}
	child, ok := taskRepo.tasksByID["child-1"]
	if !ok {
		t.Fatal("child should still exist (promoted)")
	}
	if child.ParentID != nil {
		t.Errorf("child ParentID = %v, want nil (promoted to top-level)", child.ParentID)
	}
}

func TestTaskService_Delete_NoSubtasks_SucceedsWithBlockMode(t *testing.T) {
	svc, taskRepo := newSubtaskTestSvc(t)
	// child-1 has no subtasks; block mode should succeed (nothing to block).
	err := svc.Delete(context.Background(), "org-1", "child-1", "proj-1", domain.DeleteSubtaskModeBlock, "user-1")
	if err != nil {
		t.Fatalf("Delete block (no subtasks): %v", err)
	}
	if _, ok := taskRepo.tasksByID["child-1"]; ok {
		t.Error("child should be deleted")
	}
}

func TestTaskService_Duplicate_WithSubtasks(t *testing.T) {
	svc, taskRepo := newSubtaskTestSvc(t)
	// Duplicate parent-1 with subtasks. parent-1 has one child (child-1).
	dup, err := svc.Duplicate(context.Background(), "org-1", "parent-1", "proj-1", true, "user-1")
	if err != nil {
		t.Fatalf("Duplicate with subtasks: %v", err)
	}
	// The duplicate should be top-level and have a subtask parented to it.
	if dup.ParentID != nil {
		t.Errorf("duplicate ParentID = %v, want nil", dup.ParentID)
	}
	// Count tasks parented to the duplicate.
	var subtaskCount int
	for _, task := range taskRepo.tasksByID {
		if task.ParentID != nil && *task.ParentID == dup.ID {
			subtaskCount++
		}
	}
	if subtaskCount != 1 {
		t.Errorf("expected 1 subtask on the duplicate, got %d", subtaskCount)
	}
}

func TestTaskService_Duplicate_WithoutSubtasks(t *testing.T) {
	svc, taskRepo := newSubtaskTestSvc(t)
	dup, err := svc.Duplicate(context.Background(), "org-1", "parent-1", "proj-1", false, "user-1")
	if err != nil {
		t.Fatalf("Duplicate without subtasks: %v", err)
	}
	// No subtasks should be parented to the duplicate.
	for _, task := range taskRepo.tasksByID {
		if task.ParentID != nil && *task.ParentID == dup.ID {
			t.Errorf("unexpected subtask %s parented to duplicate", task.ID)
		}
	}
}

func TestTaskService_ListSubtasks(t *testing.T) {
	svc, _ := newSubtaskTestSvc(t)
	subtasks, err := svc.ListSubtasks(context.Background(), "org-1", "proj-1", "parent-1")
	if err != nil {
		t.Fatalf("ListSubtasks: %v", err)
	}
	if len(subtasks) != 1 {
		t.Fatalf("expected 1 subtask, got %d", len(subtasks))
	}
	if subtasks[0].ID != "child-1" {
		t.Errorf("subtask ID = %q, want child-1", subtasks[0].ID)
	}
}

func TestTaskService_ListSubtasks_NoChildren(t *testing.T) {
	svc, _ := newSubtaskTestSvc(t)
	// child-1 has no subtasks.
	subtasks, err := svc.ListSubtasks(context.Background(), "org-1", "proj-1", "child-1")
	if err != nil {
		t.Fatalf("ListSubtasks: %v", err)
	}
	if len(subtasks) != 0 {
		t.Errorf("expected 0 subtasks, got %d", len(subtasks))
	}
}
