package e2e

import (
	"encoding/json"
	"net/http"
	"testing"
)

type dashboardSectionResponse struct {
	Type  string          `json:"type"`
	Title string          `json:"title"`
	Data  json.RawMessage `json:"data"`
}

type dashboardResponse struct {
	Sections []dashboardSectionResponse `json:"sections"`
}

type dashboardTaskResponse struct {
	ID          string  `json:"id"`
	Title       string  `json:"title"`
	Priority    string  `json:"priority"`
	ProjectName string  `json:"project_name"`
	DueAt       *string `json:"due_at,omitempty"`
}

type dashboardStatsResponse struct {
	AssignedCount    int `json:"assigned_count"`
	OverdueCount     int `json:"overdue_count"`
	DueThisWeekCount int `json:"due_this_week_count"`
	CompletedCount   int `json:"completed_count"`
	TotalProjects    int `json:"total_projects"`
}

type dashboardActivityResponse struct {
	ID    string `json:"id"`
	Type  string `json:"type"`
	Title string `json:"title"`
}

type projectSummaryResponse struct {
	ID   string `json:"id"`
	Name string `json:"name"`
	Slug string `json:"slug"`
}

type visibilityResponse struct {
	Sections []string `json:"sections"`
}

func TestDashboard(t *testing.T) {
	app := newE2EApp(t)
	setupOrg(t, app)
	cookie := loginCookie(t, app)

	var projectID string
	var adminID string

	resp := doJSON(t, http.MethodGet, app.URL("/api/auth/me"), nil, cookie)
	var me userResponse
	readBodyJSON(t, resp, &me)
	adminID = me.ID

	resp = doJSON(t, http.MethodPost, app.URL("/api/projects"), map[string]string{
		"name": "Dashboard Project",
	}, cookie)
	var proj projectResponse
	readBodyJSON(t, resp, &proj)
	projectID = proj.ID

	resp = doJSON(t, http.MethodGet, app.URL("/api/projects/"+projectID+"/statuses"), nil, cookie)
	var statuses []taskStatusResponse
	readBodyJSON(t, resp, &statuses)
	if len(statuses) == 0 {
		t.Fatal("no statuses found")
	}
	statusID := statuses[0].ID

	resp = doJSON(t, http.MethodPost, app.URL("/api/projects/"+projectID+"/tasks"), map[string]any{
		"title":        "Fix dashboard bug",
		"status_id":    statusID,
		"assignee_ids": []string{adminID},
	}, cookie)
	var task taskResponse
	readBodyJSON(t, resp, &task)

	t.Run("get_dashboard_default_sections", func(t *testing.T) {
		resp := doJSON(t, http.MethodGet, app.URL("/api/dashboard"), nil, cookie)
		var dr dashboardResponse
		readBodyJSON(t, resp, &dr)

		if len(dr.Sections) == 0 {
			t.Fatal("expected at least one dashboard section")
		}

		foundSections := map[string]bool{}
		for _, s := range dr.Sections {
			foundSections[s.Type] = true
		}

		expectedTypes := []string{"my_tasks", "due_soon", "activity", "stats", "projects"}
		for _, et := range expectedTypes {
			if !foundSections[et] {
				t.Errorf("expected section %q not found in dashboard", et)
			}
		}
	})

	t.Run("get_dashboard_stats_has_correct_counts", func(t *testing.T) {
		resp := doJSON(t, http.MethodGet, app.URL("/api/dashboard"), nil, cookie)
		var dr dashboardResponse
		readBodyJSON(t, resp, &dr)

		for _, s := range dr.Sections {
			if s.Type == "stats" {
				var stats dashboardStatsResponse
				if err := json.Unmarshal(s.Data, &stats); err != nil {
					t.Fatalf("unmarshal stats: %v", err)
				}
				if stats.TotalProjects < 1 {
					t.Errorf("expected at least 1 project, got %d", stats.TotalProjects)
				}
				if stats.AssignedCount < 1 {
					t.Errorf("expected at least 1 assigned task, got %d", stats.AssignedCount)
				}
				return
			}
		}
		t.Fatal("stats section not found")
	})

	t.Run("get_dashboard_my_tasks_has_task", func(t *testing.T) {
		resp := doJSON(t, http.MethodGet, app.URL("/api/dashboard"), nil, cookie)
		var dr dashboardResponse
		readBodyJSON(t, resp, &dr)

		for _, s := range dr.Sections {
			if s.Type == "my_tasks" {
				var tasks []dashboardTaskResponse
				if err := json.Unmarshal(s.Data, &tasks); err != nil {
					t.Fatalf("unmarshal my_tasks: %v", err)
				}
				found := false
				for _, dt := range tasks {
					if dt.Title == "Fix dashboard bug" {
						found = true
						if dt.ProjectName != "Dashboard Project" {
							t.Errorf("expected project name 'Dashboard Project', got %q", dt.ProjectName)
						}
					}
				}
				if !found {
					t.Errorf("task 'Fix dashboard bug' not found in my_tasks: %+v", tasks)
				}
				return
			}
		}
		t.Fatal("my_tasks section not found")
	})

	t.Run("get_dashboard_projects_section", func(t *testing.T) {
		resp := doJSON(t, http.MethodGet, app.URL("/api/dashboard"), nil, cookie)
		var dr dashboardResponse
		readBodyJSON(t, resp, &dr)

		for _, s := range dr.Sections {
			if s.Type == "projects" {
				var projects []projectSummaryResponse
				if err := json.Unmarshal(s.Data, &projects); err != nil {
					t.Fatalf("unmarshal projects: %v", err)
				}
				found := false
				for _, p := range projects {
					if p.Name == "Dashboard Project" {
						found = true
						if p.Slug != proj.Slug {
							t.Errorf("expected slug %q, got %q", proj.Slug, p.Slug)
						}
					}
				}
				if !found {
					t.Errorf("project 'Dashboard Project' not found in projects section: %+v", projects)
				}
				return
			}
		}
		t.Fatal("projects section not found")
	})

	t.Run("update_visibility_reorder", func(t *testing.T) {
		reordered := []string{"stats", "projects", "my_tasks", "due_soon", "activity"}
		resp := doJSON(t, http.MethodPatch, app.URL("/api/dashboard/visibility"), map[string]any{
			"sections": reordered,
		}, cookie)
		var vr visibilityResponse
		readBodyJSON(t, resp, &vr)

		if len(vr.Sections) != len(reordered) {
			t.Fatalf("expected %d sections, got %d", len(reordered), len(vr.Sections))
		}
		for i, s := range vr.Sections {
			if s != reordered[i] {
				t.Errorf("section at position %d: expected %q, got %q", i, reordered[i], s)
			}
		}

		// verify GET respects the new order
		resp = doJSON(t, http.MethodGet, app.URL("/api/dashboard"), nil, cookie)
		var dr dashboardResponse
		readBodyJSON(t, resp, &dr)

		for i, s := range dr.Sections {
			if s.Type != reordered[i] {
				t.Errorf("GET dashboard section %d: expected %q, got %q", i, reordered[i], s.Type)
			}
		}
	})

	t.Run("get_dashboard_unauthed", func(t *testing.T) {
		resp := doJSON(t, http.MethodGet, app.URL("/api/dashboard"), nil, "")
		resp.Body.Close()
		requireStatus(t, resp, http.StatusUnauthorized, "dashboard without auth")
	})

	t.Run("update_visibility_unauthed", func(t *testing.T) {
		resp := doJSON(t, http.MethodPatch, app.URL("/api/dashboard/visibility"), map[string]any{
			"sections": []string{"my_tasks"},
		}, "")
		resp.Body.Close()
		requireStatus(t, resp, http.StatusUnauthorized, "visibility update without auth")
	})
}
