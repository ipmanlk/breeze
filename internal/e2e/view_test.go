package e2e

import (
	"net/http"
	"testing"
)

type viewFilters struct {
	Search     string `json:"search,omitempty"`
	Priority   string `json:"priority,omitempty"`
	StatusID   string `json:"status_id,omitempty"`
	AssigneeID string `json:"assignee_id,omitempty"`
	CycleID    string `json:"cycle_id,omitempty"`
}

type viewResponse struct {
	ID        string      `json:"id"`
	ProjectID *string     `json:"project_id,omitempty"`
	Name      string      `json:"name"`
	Layout    string      `json:"layout"`
	Filters   viewFilters `json:"filters"`
}

func TestViewLifecycle(t *testing.T) {
	app := newE2EApp(t)
	setupOrg(t, app)
	cookie := loginCookie(t, app)

	// 1. Create a project
	var projectID string
	t.Run("create_project", func(t *testing.T) {
		resp := doJSON(t, http.MethodPost, app.URL("/api/projects"), map[string]string{"name": "View Test Project"}, cookie)
		var p projectResponse
		readBodyJSON(t, resp, &p)
		projectID = p.ID
	})

	// 2. Verify no views exist yet (no default views)
	t.Run("no_default_views", func(t *testing.T) {
		resp := doJSON(t, http.MethodGet, app.URL("/api/projects/"+projectID+"/views"), nil, cookie)
		var views []viewResponse
		readBodyJSON(t, resp, &views)
		if len(views) != 0 {
			t.Fatalf("expected 0 views, got %d", len(views))
		}
	})

	// 3. Create a custom project view
	var customViewID string
	t.Run("create_custom_view", func(t *testing.T) {
		resp := doJSON(t, http.MethodPost, app.URL("/api/views"), map[string]any{
			"project_id": projectID,
			"name":       "High Priority",
			"layout":     "list",
			"filters":    map[string]string{"priority": "high"},
		}, cookie)
		var v viewResponse
		readBodyJSON(t, resp, &v)
		customViewID = v.ID
		if v.Name != "High Priority" {
			t.Fatalf("name = %q, want High Priority", v.Name)
		}
		if v.Layout != "list" {
			t.Fatalf("layout = %q, want list", v.Layout)
		}
		if v.Filters.Priority != "high" {
			t.Fatalf("filters.priority = %q, want high", v.Filters.Priority)
		}
	})

	// 4. Update the view
	t.Run("update_view", func(t *testing.T) {
		resp := doJSON(t, http.MethodPatch, app.URL("/api/views/"+customViewID), map[string]any{
			"name":    "Urgent + High",
			"filters": map[string]string{"priority": "urgent"},
		}, cookie)
		var v viewResponse
		readBodyJSON(t, resp, &v)
		if v.Name != "Urgent + High" {
			t.Fatalf("name = %q, want Urgent + High", v.Name)
		}
		if v.Filters.Priority != "urgent" {
			t.Fatalf("filters.priority = %q, want urgent", v.Filters.Priority)
		}
	})

	// 5. Pin the view
	t.Run("pin_view", func(t *testing.T) {
		resp := doJSON(t, http.MethodPost, app.URL("/api/views/"+customViewID+"/pin"), nil, cookie)
		requireStatus(t, resp, http.StatusNoContent, "pin view")
	})

	// 6. Verify pinned views include it
	t.Run("list_pinned", func(t *testing.T) {
		resp := doJSON(t, http.MethodGet, app.URL("/api/views/pins"), nil, cookie)
		var pinned []viewResponse
		readBodyJSON(t, resp, &pinned)
		if len(pinned) < 1 {
			t.Fatalf("expected at least 1 pinned view, got %d", len(pinned))
		}
		found := false
		for _, v := range pinned {
			if v.ID == customViewID {
				found = true
				break
			}
		}
		if !found {
			t.Fatalf("custom view not found in pinned list")
		}
	})

	// 7. Unpin the view
	t.Run("unpin_view", func(t *testing.T) {
		resp := doJSON(t, http.MethodDelete, app.URL("/api/views/"+customViewID+"/pin"), nil, cookie)
		requireStatus(t, resp, http.StatusNoContent, "unpin view")
	})

	// 8. Delete the view
	t.Run("delete_custom_view", func(t *testing.T) {
		resp := doJSON(t, http.MethodDelete, app.URL("/api/views/"+customViewID), nil, cookie)
		requireStatus(t, resp, http.StatusNoContent, "delete view")
	})

	// 9. Verify 404 after delete
	t.Run("get_deleted_view_returns_404", func(t *testing.T) {
		resp := doJSON(t, http.MethodGet, app.URL("/api/views/"+customViewID), nil, cookie)
		requireStatus(t, resp, http.StatusNotFound, "get deleted view")
	})
}
