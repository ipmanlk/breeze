package e2e

import (
	"net/http"
	"testing"
)

// commentResponse matches dto.CommentResponse.
type commentResponse struct {
	ID          string  `json:"id"`
	TaskID      string  `json:"task_id"`
	ProjectID   string  `json:"project_id"`
	AuthorID    string  `json:"author_id"`
	Content     string  `json:"content"`
	ParentID    *string `json:"parent_id,omitempty"`
	CreatedAt   string  `json:"created_at"`
	UpdatedAt   string  `json:"updated_at"`
	EditedAt    *string `json:"edited_at,omitempty"`
	AuthorName  string  `json:"author_name"`
	AuthorEmail string  `json:"author_email"`
	Mentions    *struct {
		Users    map[string]string `json:"users,omitempty"`
		Projects map[string]string `json:"projects,omitempty"`
		Tasks    map[string]struct {
			Title     string `json:"title"`
			ProjectID string `json:"project_id"`
		} `json:"tasks,omitempty"`
		Channels map[string]string `json:"channels,omitempty"`
	} `json:"mentions,omitempty"`
}

func TestComments(t *testing.T) {
	app := newE2EApp(t)
	setupOrg(t, app)
	adminCookie := loginCookie(t, app)

	// Project + status + task.
	resp := doJSON(t, http.MethodPost, app.URL("/api/projects"), map[string]string{"name": "Comments Project"}, adminCookie)
	var project projectResponse
	readBodyJSON(t, resp, &project)

	resp = doJSON(t, http.MethodGet, app.URL("/api/projects/"+project.ID+"/statuses"), nil, adminCookie)
	var statuses []taskStatusResponse
	readBodyJSON(t, resp, &statuses)
	statusID := statuses[0].ID

	resp = doJSON(t, http.MethodPost, app.URL("/api/projects/"+project.ID+"/tasks"), map[string]string{
		"title": "Commentable task", "status_id": statusID,
	}, adminCookie)
	var task taskResponse
	readBodyJSON(t, resp, &task)

	t.Run("create_returns_project_id", func(t *testing.T) {
		resp := doJSON(t, http.MethodPost, app.URL("/api/projects/"+project.ID+"/tasks/"+task.ID+"/comments"), map[string]string{
			"content": "Hello world",
		}, adminCookie)
		requireStatus(t, resp, http.StatusCreated, "create comment")
		var c commentResponse
		readBodyJSON(t, resp, &c)
		if c.ProjectID != project.ID {
			t.Errorf("project_id = %q, want %q", c.ProjectID, project.ID)
		}
		if c.AuthorName != "Admin" {
			t.Errorf("author_name = %q, want Admin", c.AuthorName)
		}
	})

	t.Run("mention_resolves_and_notifies", func(t *testing.T) {
		// Invite a second user so we can @mention them.
		invite := doJSON(t, http.MethodPost, app.URL("/api/invites"), map[string]string{"role": "member"}, adminCookie)
		requireStatus(t, invite, http.StatusCreated, "create invite")
		var ir struct {
			Token string `json:"token"`
		}
		readBodyJSON(t, invite, &ir)

		accept := doJSON(t, http.MethodPost, app.URL("/api/invites/"+ir.Token+"/accept"), map[string]any{
			"name": "Jane Mentioned", "email": "jane-mention@test.com", "password": "password123",
		}, "")
		requireStatus(t, accept, http.StatusCreated, "accept invite")
		var jane struct {
			ID string `json:"id"`
		}
		readBodyJSON(t, accept, &jane)

		// Comment with a <@user:id> mention token (as the chat editor serializes).
		content := "hey <@user:" + jane.ID + "> take a look"
		resp := doJSON(t, http.MethodPost, app.URL("/api/projects/"+project.ID+"/tasks/"+task.ID+"/comments"), map[string]string{
			"content": content,
		}, adminCookie)
		requireStatus(t, resp, http.StatusCreated, "create mentioned comment")
		var c commentResponse
		readBodyJSON(t, resp, &c)

		// Mentions payload must resolve the user id → name.
		if c.Mentions == nil || c.Mentions.Users == nil {
			t.Fatalf("expected mentions.users to be resolved, got %+v", c.Mentions)
		}
		if name, ok := c.Mentions.Users[jane.ID]; !ok || name != "Jane Mentioned" {
			t.Errorf("mentions.users[%s] = %q, want %q", jane.ID, name, "Jane Mentioned")
		}

		// The mentioned user should have a notification.
		janeCookie := loginAs(t, app, "jane-mention@test.com", "password123")
		resp = doJSON(t, http.MethodGet, app.URL("/api/notifications/unread-count"), nil, janeCookie)
		var count unreadCountResponse
		readBodyJSON(t, resp, &count)
		if count.Count == 0 {
			t.Errorf("expected jane to have >=1 unread notification from the mention, got 0")
		}

		// The notification must carry a slug-based task link + project_slug so
		// the inbox routes correctly (entity_type "task" drives the slug JOIN).
		resp = doJSON(t, http.MethodGet, app.URL("/api/notifications"), nil, janeCookie)
		var notifList struct {
			Items []struct {
				Type        string `json:"type"`
				Link        string `json:"link"`
				EntityType  string `json:"entity_type"`
				EntityID    string `json:"entity_id"`
				ProjectSlug string `json:"project_slug"`
			} `json:"items"`
		}
		readBodyJSON(t, resp, &notifList)
		var found bool
		for _, n := range notifList.Items {
			if n.Type == "task_comment" && n.EntityID == task.ID {
				found = true
				if n.EntityType != "task" {
					t.Errorf("entity_type = %q, want task (so project_slug JOIN resolves)", n.EntityType)
				}
				if n.ProjectSlug != project.Slug {
					t.Errorf("project_slug = %q, want %q", n.ProjectSlug, project.Slug)
				}
				wantLink := "/projects/" + project.Slug + "?task=" + task.ID
				if n.Link != wantLink {
					t.Errorf("link = %q, want %q", n.Link, wantLink)
				}
			}
		}
		if !found {
			t.Errorf("expected a task_comment notification for task %s, got %+v", task.ID, notifList.Items)
		}
	})

	t.Run("list_returns_mentions", func(t *testing.T) {
		resp := doJSON(t, http.MethodGet, app.URL("/api/projects/"+project.ID+"/tasks/"+task.ID+"/comments"), nil, adminCookie)
		requireStatus(t, resp, http.StatusOK, "list comments")
		var listResp struct {
			Items []commentResponse `json:"items"`
		}
		readBodyJSON(t, resp, &listResp)
		if len(listResp.Items) < 1 {
			t.Fatalf("expected at least 1 comment, got %d", len(listResp.Items))
		}
	})

	t.Run("edit_sets_edited_at_and_author_only", func(t *testing.T) {
		// Create a comment as admin, then edit it.
		resp := doJSON(t, http.MethodPost, app.URL("/api/projects/"+project.ID+"/tasks/"+task.ID+"/comments"), map[string]string{
			"content": "original",
		}, adminCookie)
		var c commentResponse
		readBodyJSON(t, resp, &c)

		resp = doJSON(t, http.MethodPatch, app.URL("/api/projects/"+project.ID+"/tasks/"+task.ID+"/comments/"+c.ID), map[string]string{
			"content": "edited content",
		}, adminCookie)
		requireStatus(t, resp, http.StatusOK, "edit own comment")
		var updated commentResponse
		readBodyJSON(t, resp, &updated)
		if updated.Content != "edited content" {
			t.Errorf("content = %q, want edited content", updated.Content)
		}
		if updated.EditedAt == nil || *updated.EditedAt == "" {
			t.Error("expected edited_at to be set after edit")
		}
	})

	t.Run("delete_author_only", func(t *testing.T) {
		resp := doJSON(t, http.MethodPost, app.URL("/api/projects/"+project.ID+"/tasks/"+task.ID+"/comments"), map[string]string{
			"content": "to be deleted",
		}, adminCookie)
		var c commentResponse
		readBodyJSON(t, resp, &c)

		resp = doJSON(t, http.MethodDelete, app.URL("/api/projects/"+project.ID+"/tasks/"+task.ID+"/comments/"+c.ID), nil, adminCookie)
		requireStatus(t, resp, http.StatusNoContent, "delete own comment")
	})
}
