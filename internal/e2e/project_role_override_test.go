package e2e

import (
	"net/http"
	"testing"
)

// TestProjectRoleOverride exercises the per-project role override end-to-end:
//   - a viewer invited to the org has no project access until added as a member
//   - once added with project role "viewer", they can view the project but not
//     create tasks
//   - promoting them to project "admin" grants task:create on that project only
//   - the projects list is membership-filtered for project-scoped users
func TestProjectRoleOverride(t *testing.T) {
	app := newE2EApp(t)
	setupOrg(t, app)
	adminCookie := loginCookie(t, app)

	// Admin creates a project and grabs a status id (needed to create tasks).
	projectID := createProject(t, app, adminCookie)
	resp := doJSON(t, http.MethodGet, app.URL("/api/projects/"+projectID+"/statuses"), nil, adminCookie)
	var statuses []taskStatusResponse
	readBodyJSON(t, resp, &statuses)
	if len(statuses) == 0 {
		t.Fatal("expected default statuses")
	}
	statusID := statuses[0].ID

	// Invite a viewer and have them accept.
	inviteResp := doJSON(t, http.MethodPost, app.URL("/api/invites"), map[string]string{"role": "viewer"}, adminCookie)
	var invite struct {
		Token string `json:"token"`
	}
	readBodyJSON(t, inviteResp, &invite)
	if invite.Token == "" {
		t.Fatal("empty invite token")
	}

	acceptResp := doJSON(t, http.MethodPost, app.URL("/api/invites/"+invite.Token+"/accept"), map[string]any{
		"name": "Vera Viewer", "email": "vera@test.com", "password": "password123",
	}, "")
	requireStatus(t, acceptResp, http.StatusCreated, "accept viewer invite")
	acceptResp.Body.Close()

	viewerCookie := loginAs(t, app, "vera@test.com", "password123")

	// 1. Before being added: viewer cannot open the project (403) and cannot
	//    create tasks (403).
	resp = doJSON(t, http.MethodGet, app.URL("/api/projects/"+projectID), nil, viewerCookie)
	requireStatus(t, resp, http.StatusForbidden, "viewer get project before membership")
	resp = doJSON(t, http.MethodPost, app.URL("/api/projects/"+projectID+"/tasks"), map[string]string{
		"title": "viewer task", "status_id": statusID,
	}, viewerCookie)
	requireStatus(t, resp, http.StatusForbidden, "viewer create task before membership")

	// Resolve the viewer's user id (membership takes a user id, not an email).
	resp = doJSON(t, http.MethodGet, app.URL("/api/users"), nil, adminCookie)
	var usersPage struct {
		Items []userResponse `json:"items"`
	}
	readBodyJSON(t, resp, &usersPage)
	var viewerID string
	for _, u := range usersPage.Items {
		if u.Email == "vera@test.com" {
			viewerID = u.ID
		}
	}
	if viewerID == "" {
		t.Fatal("could not resolve viewer user id")
	}

	// 2. Admin adds the viewer as a project member with role "viewer".
	addResp := doJSON(t, http.MethodPost, app.URL("/api/projects/"+projectID+"/members"), map[string]any{
		"user_id": viewerID, "role": "viewer",
	}, adminCookie)
	requireStatus(t, addResp, http.StatusCreated, "add viewer as member")
	addResp.Body.Close()

	// 3. Now the viewer can view the project; my-access reports the viewer role
	//    and viewer permissions, but still NOT task:create.
	resp = doJSON(t, http.MethodGet, app.URL("/api/projects/"+projectID), nil, viewerCookie)
	requireStatus(t, resp, http.StatusOK, "viewer get project after membership")
	resp = doJSON(t, http.MethodGet, app.URL("/api/projects/"+projectID+"/my-access"), nil, viewerCookie)
	requireStatus(t, resp, http.StatusOK, "viewer my-access")
	var access struct {
		Role        string   `json:"role"`
		Permissions []string `json:"permissions"`
	}
	readBodyJSON(t, resp, &access)
	if access.Role != "viewer" {
		t.Errorf("viewer project role = %q, want viewer", access.Role)
	}
	if contains(access.Permissions, "task:create") {
		t.Error("viewer should NOT have task:create before override")
	}
	resp = doJSON(t, http.MethodPost, app.URL("/api/projects/"+projectID+"/tasks"), map[string]string{
		"title": "viewer task", "status_id": statusID,
	}, viewerCookie)
	requireStatus(t, resp, http.StatusForbidden, "viewer create task as viewer")

	// 4. Admin promotes the viewer to project "admin" (the override).
	updResp := doJSON(t, http.MethodPut, app.URL("/api/projects/"+projectID+"/members/"+viewerID), map[string]string{"role": "admin"}, adminCookie)
	requireStatus(t, updResp, http.StatusOK, "promote viewer to project admin")
	updResp.Body.Close()

	// 5. The viewer now has admin permissions on THIS project and can create a
	//    task. my-access reflects the override.
	resp = doJSON(t, http.MethodGet, app.URL("/api/projects/"+projectID+"/my-access"), nil, viewerCookie)
	requireStatus(t, resp, http.StatusOK, "viewer my-access after override")
	readBodyJSON(t, resp, &access)
	if access.Role != "admin" {
		t.Errorf("overridden role = %q, want admin", access.Role)
	}
	if !contains(access.Permissions, "task:create") {
		t.Error("project admin should have task:create")
	}
	resp = doJSON(t, http.MethodPost, app.URL("/api/projects/"+projectID+"/tasks"), map[string]string{
		"title": "viewer admin task", "status_id": statusID,
	}, viewerCookie)
	requireStatus(t, resp, http.StatusCreated, "project admin create task")

	// 6. The override is project-scoped: the viewer still cannot create a task
	//    in a different project they're not a member of.
	otherResp := doJSON(t, http.MethodPost, app.URL("/api/projects"), map[string]string{
		"name": "Other Project", "slug": "other-project",
	}, adminCookie)
	requireStatus(t, otherResp, http.StatusCreated, "create other project")
	var otherProj projectResponse
	readBodyJSON(t, otherResp, &otherProj)
	otherResp.Body.Close()
	resp = doJSON(t, http.MethodPost, app.URL("/api/projects/"+otherProj.ID+"/tasks"), map[string]string{
		"title": "other task", "status_id": statusID,
	}, viewerCookie)
	requireStatus(t, resp, http.StatusForbidden, "override must not leak to other projects")

	// 7. Projects list is membership-filtered: viewer only sees projectID.
	resp = doJSON(t, http.MethodGet, app.URL("/api/projects"), nil, viewerCookie)
	var projects []projectResponse
	readBodyJSON(t, resp, &projects)
	if len(projects) != 1 || projects[0].ID != projectID {
		ids := make([]string, len(projects))
		for i, p := range projects {
			ids[i] = p.ID
		}
		t.Errorf("viewer project list = %v, want only [%s]", ids, projectID)
	}
}

func contains(haystack []string, needle string) bool {
	for _, s := range haystack {
		if s == needle {
			return true
		}
	}
	return false
}
