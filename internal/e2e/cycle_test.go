package e2e

import (
	"net/http"
	"testing"
)

func TestCycleWorkflow(t *testing.T) {
	app := newE2EApp(t)
	setupOrg(t, app)
	cookie := loginCookie(t, app)

	resp := doJSON(t, http.MethodPost, app.URL("/api/projects"), map[string]string{"name": "Cycle Project"}, cookie)
	var project projectResponse
	readBodyJSON(t, resp, &project)

	var cycleID string

	t.Run("create", func(t *testing.T) {
		resp := doJSON(t, http.MethodPost, app.URL("/api/projects/"+project.ID+"/cycles"), map[string]any{
			"name":      "Sprint 1",
			"starts_at": "2024-01-01T00:00:00Z",
			"ends_at":   "2024-01-14T00:00:00Z",
		}, cookie)
		var c cycleResponse
		readBodyJSON(t, resp, &c)
		if c.Name != "Sprint 1" {
			t.Fatalf("name = %q, want Sprint 1", c.Name)
		}
		cycleID = c.ID
	})

	t.Run("list", func(t *testing.T) {
		resp := doJSON(t, http.MethodGet, app.URL("/api/projects/"+project.ID+"/cycles"), nil, cookie)
		var cycles []cycleResponse
		readBodyJSON(t, resp, &cycles)
		if len(cycles) != 1 {
			t.Fatalf("expected 1 cycle, got %d", len(cycles))
		}
	})

	t.Run("update", func(t *testing.T) {
		resp := doJSON(t, http.MethodPut, app.URL("/api/projects/"+project.ID+"/cycles/"+cycleID), map[string]string{
			"name": "Sprint 1 (updated)",
		}, cookie)
		var c cycleResponse
		readBodyJSON(t, resp, &c)
		if c.Name != "Sprint 1 (updated)" {
			t.Fatalf("name = %q, want Sprint 1 (updated)", c.Name)
		}
	})
}
