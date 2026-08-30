package e2e

import (
	"net/http"
	"testing"
)

// TestFullJourney exercises the complete system path: setup → login → CRUD → logout.
// It uses a different admin user than the default setupOrg helper to prove
// the system supports arbitrary first-user creation.
func TestFullJourney(t *testing.T) {
	app := newE2EApp(t)

	// Setup: fresh app should need setup.
	resp := doJSON(t, http.MethodGet, app.URL("/api/setup"), nil, "")
	var check setupCheckResponse
	readBodyJSON(t, resp, &check)
	if !check.NeedsSetup {
		t.Fatal("initial: needs_setup should be true")
	}

	// Complete setup with a different user than the default helper.
	resp = doJSON(t, http.MethodPost, app.URL("/api/setup"), map[string]string{
		"org_name": "Startup Inc",
		"name":     "CEO",
		"email":    "ceo@startup.com",
		"password": "secure123",
	}, "")
	resp.Body.Close()
	requireStatus(t, resp, http.StatusOK, "setup")

	cookie := loginAs(t, app, "ceo@startup.com", "secure123")

	// Verify identity.
	resp = doJSON(t, http.MethodGet, app.URL("/api/auth/me"), nil, cookie)
	var me userResponse
	readBodyJSON(t, resp, &me)
	if me.Email != "ceo@startup.com" {
		t.Fatalf("me: email = %q, want ceo@startup.com", me.Email)
	}

	// Create a project.
	resp = doJSON(t, http.MethodPost, app.URL("/api/projects"), map[string]string{"name": "Launch MVP"}, cookie)
	var project projectResponse
	readBodyJSON(t, resp, &project)

	// Find the "Todo" status for task creation.
	resp = doJSON(t, http.MethodGet, app.URL("/api/projects/"+project.ID+"/statuses"), nil, cookie)
	var statuses []taskStatusResponse
	readBodyJSON(t, resp, &statuses)
	todoID := ""
	for _, s := range statuses {
		if s.Name == "Todo" {
			todoID = s.ID
			break
		}
	}
	if todoID == "" {
		t.Fatal("expected Todo status to exist")
	}

	// Create two tasks.
	resp = doJSON(t, http.MethodPost, app.URL("/api/projects/"+project.ID+"/tasks"), map[string]string{
		"title": "Set up CI/CD", "status_id": todoID,
	}, cookie)
	var task1 taskResponse
	readBodyJSON(t, resp, &task1)

	resp = doJSON(t, http.MethodPost, app.URL("/api/projects/"+project.ID+"/tasks"), map[string]string{
		"title": "Write tests", "status_id": todoID,
	}, cookie)
	resp.Body.Close()

	// List tasks: should have 2.
	resp = doJSON(t, http.MethodGet, app.URL("/api/projects/"+project.ID+"/tasks"), nil, cookie)
	var tasks []taskResponse
	readBodyJSON(t, resp, &tasks)
	if len(tasks) != 2 {
		t.Fatalf("expected 2 tasks, got %d", len(tasks))
	}

	// Update the first task.
	resp = doJSON(t, http.MethodPut, app.URL("/api/projects/"+project.ID+"/tasks/"+task1.ID), map[string]string{
		"title": "Set up CI/CD pipeline",
	}, cookie)
	resp.Body.Close()

	// List projects: should have 1.
	resp = doJSON(t, http.MethodGet, app.URL("/api/projects"), nil, cookie)
	var projectList []projectResponse
	readBodyJSON(t, resp, &projectList)
	if len(projectList) != 1 {
		t.Fatalf("expected 1 project, got %d", len(projectList))
	}

	// Logout.
	resp = doJSON(t, http.MethodPost, app.URL("/api/auth/logout"), nil, cookie)
	resp.Body.Close()
	requireStatus(t, resp, http.StatusOK, "logout")

	// After logout: must be 401.
	resp = doJSON(t, http.MethodGet, app.URL("/api/auth/me"), nil, cookie)
	resp.Body.Close()
	requireStatus(t, resp, http.StatusUnauthorized, "after logout")
}
