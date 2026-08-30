package e2e

import (
	"net/http"
	"testing"
)

func TestTaskCRUD(t *testing.T) {
	app := newE2EApp(t)
	setupOrg(t, app)
	cookie := loginCookie(t, app)

	// Create project first.
	resp := doJSON(t, http.MethodPost, app.URL("/api/projects"), map[string]string{"name": "Tasks Project"}, cookie)
	var project projectResponse
	readBodyJSON(t, resp, &project)

	// Get a status ID for task creation.
	resp = doJSON(t, http.MethodGet, app.URL("/api/projects/"+project.ID+"/statuses"), nil, cookie)
	var statuses []taskStatusResponse
	readBodyJSON(t, resp, &statuses)
	if len(statuses) == 0 {
		t.Fatal("expected default statuses")
	}
	statusID := statuses[0].ID

	var taskID string

	t.Run("create", func(t *testing.T) {
		resp := doJSON(t, http.MethodPost, app.URL("/api/projects/"+project.ID+"/tasks"), map[string]string{
			"title": "First task", "status_id": statusID,
		}, cookie)
		var task taskResponse
		readBodyJSON(t, resp, &task)
		if task.Title != "First task" {
			t.Fatalf("title = %q, want First task", task.Title)
		}
		taskID = task.ID
	})

	t.Run("list", func(t *testing.T) {
		resp := doJSON(t, http.MethodGet, app.URL("/api/projects/"+project.ID+"/tasks"), nil, cookie)
		var tasks []taskResponse
		readBodyJSON(t, resp, &tasks)
		if len(tasks) == 0 {
			t.Fatal("expected at least 1 task")
		}
	})

	t.Run("update", func(t *testing.T) {
		resp := doJSON(t, http.MethodPut, app.URL("/api/projects/"+project.ID+"/tasks/"+taskID), map[string]string{
			"title": "Updated task",
		}, cookie)
		var task taskResponse
		readBodyJSON(t, resp, &task)
		if task.Title != "Updated task" {
			t.Fatalf("title = %q, want Updated task", task.Title)
		}
	})

	t.Run("get_by_id", func(t *testing.T) {
		resp := doJSON(t, http.MethodGet, app.URL("/api/projects/"+project.ID+"/tasks/"+taskID), nil, cookie)
		var task taskResponse
		readBodyJSON(t, resp, &task)
		if task.Title != "Updated task" {
			t.Fatalf("title = %q, want Updated task", task.Title)
		}
	})

	t.Run("delete", func(t *testing.T) {
		resp := doJSON(t, http.MethodDelete, app.URL("/api/projects/"+project.ID+"/tasks/"+taskID), nil, cookie)
		resp.Body.Close()
		requireStatus(t, resp, http.StatusNoContent, "delete task")
	})

	t.Run("list_after_delete", func(t *testing.T) {
		resp := doJSON(t, http.MethodGet, app.URL("/api/projects/"+project.ID+"/tasks"), nil, cookie)
		var tasks []taskResponse
		readBodyJSON(t, resp, &tasks)
		if len(tasks) != 0 {
			t.Fatalf("expected 0 tasks after delete, got %d", len(tasks))
		}
	})
}

func TestTaskMove(t *testing.T) {
	app := newE2EApp(t)
	setupOrg(t, app)
	cookie := loginCookie(t, app)

	// Create project and get two statuses.
	resp := doJSON(t, http.MethodPost, app.URL("/api/projects"), map[string]string{"name": "Move Project"}, cookie)
	var project projectResponse
	readBodyJSON(t, resp, &project)

	resp = doJSON(t, http.MethodGet, app.URL("/api/projects/"+project.ID+"/statuses"), nil, cookie)
	var statuses []taskStatusResponse
	readBodyJSON(t, resp, &statuses)
	if len(statuses) < 2 {
		t.Fatal("expected at least 2 statuses")
	}
	statusID1, statusID2 := statuses[0].ID, statuses[1].ID

	// Create a task in status 1.
	resp = doJSON(t, http.MethodPost, app.URL("/api/projects/"+project.ID+"/tasks"), map[string]string{
		"title": "Movable", "status_id": statusID1,
	}, cookie)
	var task taskResponse
	readBodyJSON(t, resp, &task)

	// Move it to status 2.
	resp = doJSON(t, http.MethodPatch, app.URL("/api/projects/"+project.ID+"/tasks/"+task.ID+"/position"), map[string]string{
		"status_id": statusID2, "position_key": "b0",
	}, cookie)
	var moved taskResponse
	readBodyJSON(t, resp, &moved)
	if moved.StatusID != statusID2 {
		t.Fatalf("status_id after move = %q, want %q", moved.StatusID, statusID2)
	}

	// Verify via GET.
	resp = doJSON(t, http.MethodGet, app.URL("/api/projects/"+project.ID+"/tasks/"+task.ID), nil, cookie)
	var single taskResponse
	readBodyJSON(t, resp, &single)
	if single.StatusID != statusID2 {
		t.Fatalf("fetched status_id = %q, want %q", single.StatusID, statusID2)
	}
}

// TestTaskBatchUpdate_StatusChange exercises the BatchUpdate endpoint with a
// status change across multiple tasks. This path runs the mutation loop
// inside RunInTransaction, which calls repo.Update / GeneratePositionKey /
// SetAssignees: each of which must reuse the outer transaction rather than
// starting a nested one (would deadlock under SetMaxOpenConns(1)).
//
// Regression guard for BatchUpdate atomicity and the nested-tx fix.
func TestTaskBatchUpdate_StatusChange(t *testing.T) {
	app := newE2EApp(t)
	setupOrg(t, app)
	cookie := loginCookie(t, app)

	// Create project + fetch statuses.
	resp := doJSON(t, http.MethodPost, app.URL("/api/projects"), map[string]string{"name": "Batch Project"}, cookie)
	var project projectResponse
	readBodyJSON(t, resp, &project)

	resp = doJSON(t, http.MethodGet, app.URL("/api/projects/"+project.ID+"/statuses"), nil, cookie)
	var statuses []taskStatusResponse
	readBodyJSON(t, resp, &statuses)
	if len(statuses) < 2 {
		t.Fatalf("expected >=2 statuses, got %d", len(statuses))
	}
	srcStatus := statuses[0].ID
	dstStatus := statuses[1].ID

	// Create 3 tasks in the source status.
	var taskIDs []string
	for i := 0; i < 3; i++ {
		resp := doJSON(t, http.MethodPost, app.URL("/api/projects/"+project.ID+"/tasks"), map[string]string{
			"title": "Batch task", "status_id": srcStatus,
		}, cookie)
		var task taskResponse
		readBodyJSON(t, resp, &task)
		taskIDs = append(taskIDs, task.ID)
	}

	// Batch-update all 3 to the destination status. This must not deadlock.
	resp = doJSON(t, http.MethodPost, app.URL("/api/projects/"+project.ID+"/tasks/batch"), map[string]any{
		"task_ids":  taskIDs,
		"status_id": dstStatus,
	}, cookie)
	if resp.StatusCode != http.StatusOK {
		t.Fatalf("batch update status = %d, want 200 (deadlock or error)", resp.StatusCode)
	}
	var updated []taskResponse
	readBodyJSON(t, resp, &updated)
	if len(updated) != 3 {
		t.Fatalf("expected 3 updated tasks, got %d", len(updated))
	}
	for _, u := range updated {
		if u.StatusID != dstStatus {
			t.Fatalf("task %s status = %q, want %q", u.ID, u.StatusID, dstStatus)
		}
	}
}
