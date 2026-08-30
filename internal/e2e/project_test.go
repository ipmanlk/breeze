package e2e

import (
	"net/http"
	"testing"
)

func TestProjectCRUD(t *testing.T) {
	app := newE2EApp(t)
	setupOrg(t, app)
	cookie := loginCookie(t, app)

	var projectID string

	t.Run("create", func(t *testing.T) {
		resp := doJSON(t, http.MethodPost, app.URL("/api/projects"), map[string]string{"name": "My Project"}, cookie)
		var p projectResponse
		readBodyJSON(t, resp, &p)
		if p.Name != "My Project" {
			t.Fatalf("name = %q, want My Project", p.Name)
		}
		projectID = p.ID
	})

	t.Run("list", func(t *testing.T) {
		resp := doJSON(t, http.MethodGet, app.URL("/api/projects"), nil, cookie)
		var list []projectResponse
		readBodyJSON(t, resp, &list)
		if len(list) != 1 {
			t.Fatalf("expected 1 project, got %d", len(list))
		}
	})

	t.Run("get_by_id", func(t *testing.T) {
		resp := doJSON(t, http.MethodGet, app.URL("/api/projects/"+projectID), nil, cookie)
		var p projectResponse
		readBodyJSON(t, resp, &p)
		if p.ID != projectID {
			t.Fatalf("id mismatch: %q vs %q", p.ID, projectID)
		}
	})

	t.Run("update", func(t *testing.T) {
		resp := doJSON(t, http.MethodPut, app.URL("/api/projects/"+projectID), map[string]string{"name": "Renamed"}, cookie)
		var p projectResponse
		readBodyJSON(t, resp, &p)
		if p.Name != "Renamed" {
			t.Fatalf("name = %q, want Renamed", p.Name)
		}
	})

	t.Run("delete", func(t *testing.T) {
		resp := doJSON(t, http.MethodDelete, app.URL("/api/projects/"+projectID), nil, cookie)
		resp.Body.Close()
		requireStatus(t, resp, http.StatusNoContent, "delete project")
	})
}
