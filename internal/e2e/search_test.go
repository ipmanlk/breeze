package e2e

import (
	"net/http"
	"testing"
)

type searchResultResponse struct {
	ID       string `json:"id"`
	Type     string `json:"type"`
	Name     string `json:"name"`
	Subtitle string `json:"subtitle"`
	URL      string `json:"url"`
	Color    string `json:"color"`
}

type searchResponse struct {
	Results []searchResultResponse `json:"results"`
}

func TestSearchProjectsAndTasks(t *testing.T) {
	app := newE2EApp(t)
	setupOrg(t, app)
	cookie := loginCookie(t, app)

	var projectID string

	resp := doJSON(t, http.MethodPost, app.URL("/api/projects"), map[string]string{
		"name": "Design System",
	}, cookie)
	var proj projectResponse
	readBodyJSON(t, resp, &proj)
	projectID = proj.ID

	resp = doJSON(t, http.MethodGet, app.URL("/api/projects/"+projectID+"/statuses"), nil, cookie)
	var statuses []taskStatusResponse
	readBodyJSON(t, resp, &statuses)
	if len(statuses) == 0 {
		t.Fatal("no statuses found for project")
	}
	firstStatusID := statuses[0].ID

	resp = doJSON(t, http.MethodPost, app.URL("/api/projects/"+projectID+"/tasks"), map[string]string{
		"title":     "Build button component",
		"status_id": firstStatusID,
	}, cookie)
	var task taskResponse
	readBodyJSON(t, resp, &task)

	t.Run("search_projects_authed", func(t *testing.T) {
		resp := doJSON(t, http.MethodGet, app.URL("/api/search?q=Design")+"&types=project,task", nil, cookie)
		var sr searchResponse
		readBodyJSON(t, resp, &sr)

		found := false
		for _, r := range sr.Results {
			if r.Type == "project" && r.Name == "Design System" {
				found = true
				if r.URL != "/projects/"+proj.Slug {
					t.Fatalf("project URL = %q, want /projects/%s", r.URL, proj.Slug)
				}
			}
		}
		if !found {
			t.Fatalf("did not find Design System project in search results: %+v", sr.Results)
		}
	})

	t.Run("search_tasks_authed", func(t *testing.T) {
		resp := doJSON(t, http.MethodGet, app.URL("/api/search?q=button")+"&types=project,task", nil, cookie)
		var sr searchResponse
		readBodyJSON(t, resp, &sr)

		found := false
		for _, r := range sr.Results {
			if r.Type == "task" && r.Name == "Build button component" {
				found = true
				if r.Subtitle != "Design System" {
					t.Fatalf("task subtitle = %q, want Design System", r.Subtitle)
				}
				expectedURL := "/projects/" + proj.Slug + "?task=" + task.ID
				if r.URL != expectedURL {
					t.Fatalf("task URL = %q, want %s", r.URL, expectedURL)
				}
			}
		}
		if !found {
			t.Fatalf("did not find button task in search results: %+v", sr.Results)
		}
	})

	t.Run("search_no_query", func(t *testing.T) {
		resp := doJSON(t, http.MethodGet, app.URL("/api/search?q=")+"&types=project,task", nil, cookie)
		var sr searchResponse
		readBodyJSON(t, resp, &sr)
		if sr.Results == nil {
			t.Fatalf("expected results (can be empty), got null")
		}
	})

	t.Run("search_unauthed", func(t *testing.T) {
		resp := doJSON(t, http.MethodGet, app.URL("/api/search?q=test"), nil, "")
		resp.Body.Close()
		requireStatus(t, resp, http.StatusUnauthorized, "search without auth")
	})

	t.Run("search_no_results", func(t *testing.T) {
		resp := doJSON(t, http.MethodGet, app.URL("/api/search?q=zzzznonexistent&types=project,task"), nil, cookie)
		var sr searchResponse
		readBodyJSON(t, resp, &sr)
		if len(sr.Results) != 0 {
			t.Fatalf("expected 0 results for nonexistent, got %d", len(sr.Results))
		}
	})
}
