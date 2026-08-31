package e2e

import (
	"net/http"
	"testing"

	"github.com/google/uuid"

	"ipmanlk/plume/internal/auth"
	"ipmanlk/plume/internal/domain"
)

// TestCrossOrgIsolation verifies that a user in org A cannot read or mutate
// org B's projects, tasks, statuses, cycles, members, or project-child
// resources: even when the user has an elevated org role (owner/admin) in
// org A.
func TestCrossOrgIsolation(t *testing.T) {
	app := newE2EApp(t)
	setupOrg(t, app)
	cookieA := loginCookie(t, app)

	// Create a project in org A (sanity check).
	projectA := createProject(t, app, cookieA)

	// Seed a second org with its own owner directly via SQL (the setup
	// endpoint only allows one org per app).
	orgBID := uuid.New().String()
	userBID := uuid.New().String()
	orgBSlug := "org-b"
	userBEmail := "beta@b.test"
	userBPass := "password123"

	passHash, err := auth.HashPassword(userBPass)
	if err != nil {
		t.Fatalf("hash password: %v", err)
	}

	_, err = app.db.Exec(
		`INSERT INTO organizations (id, name, slug) VALUES (?, ?, ?)`,
		orgBID, "Org B", orgBSlug,
	)
	if err != nil {
		t.Fatalf("create org B: %v", err)
	}
	_, err = app.db.Exec(
		`INSERT INTO accounts (id, email, password_hash) VALUES (?, ?, ?)`,
		uuid.New().String(), userBEmail, passHash,
	)
	if err != nil {
		t.Fatalf("create account B: %v", err)
	}
	accountBID := app.db.QueryRow(`SELECT id FROM accounts WHERE email = ?`, userBEmail)
	var acctBID string
	if err := accountBID.Scan(&acctBID); err != nil {
		t.Fatalf("lookup account B: %v", err)
	}
	_, err = app.db.Exec(
		`INSERT INTO users (id, account_id, org_id, email, name, role, is_active) VALUES (?, ?, ?, ?, ?, ?, ?)`,
		userBID, acctBID, orgBID, userBEmail, "Beta", string(domain.RoleOwner), true,
	)
	if err != nil {
		t.Fatalf("create user B: %v", err)
	}

	cookieB := loginAs(t, app, userBEmail, userBPass)
	projectB := createProject(t, app, cookieB)

	// Grab status B for task creation.
	resp := doJSON(t, http.MethodGet, app.URL("/api/projects/"+projectB+"/statuses"), nil, cookieB)
	var statuses []taskStatusResponse
	readBodyJSON(t, resp, &statuses)
	if len(statuses) == 0 {
		t.Fatal("expected default statuses in project B")
	}
	statusB := statuses[0].ID

	// Create a task in project B.
	resp = doJSON(t, http.MethodPost, app.URL("/api/projects/"+projectB+"/tasks"), map[string]any{
		"title": "Secret task B", "status_id": statusB,
	}, cookieB)
	requireStatus(t, resp, http.StatusCreated, "create task in B")
	resp.Body.Close()

	// --- Cross-org attacks: org A admin's cookie against org B's resources ---

	// 1. Cannot read project B.
	resp = doJSON(t, http.MethodGet, app.URL("/api/projects/"+projectB), nil, cookieA)
	requireStatus(t, resp, http.StatusForbidden, "A admin reads B project")
	resp.Body.Close()

	// 2. Cannot list tasks in project B.
	resp = doJSON(t, http.MethodGet, app.URL("/api/projects/"+projectB+"/tasks"), nil, cookieA)
	requireStatus(t, resp, http.StatusForbidden, "A admin lists B tasks")
	resp.Body.Close()

	// 3. Cannot create a task in project B.
	resp = doJSON(t, http.MethodPost, app.URL("/api/projects/"+projectB+"/tasks"), map[string]any{
		"title": "A attack task", "status_id": statusB,
	}, cookieA)
	requireStatus(t, resp, http.StatusForbidden, "A admin creates task in B")
	resp.Body.Close()

	// 4. Cannot update project B.
	resp = doJSON(t, http.MethodPut, app.URL("/api/projects/"+projectB), map[string]string{
		"name": "Hijacked",
	}, cookieA)
	requireStatus(t, resp, http.StatusForbidden, "A admin updates B project")
	resp.Body.Close()

	// 5. Cannot delete project B.
	resp = doJSON(t, http.MethodDelete, app.URL("/api/projects/"+projectB), nil, cookieA)
	requireStatus(t, resp, http.StatusForbidden, "A admin deletes B project")
	resp.Body.Close()

	// 6. Cannot list B's statuses.
	resp = doJSON(t, http.MethodGet, app.URL("/api/projects/"+projectB+"/statuses"), nil, cookieA)
	requireStatus(t, resp, http.StatusForbidden, "A admin lists B statuses")
	resp.Body.Close()

	// 7. Cannot create a status in B.
	resp = doJSON(t, http.MethodPost, app.URL("/api/projects/"+projectB+"/statuses"), map[string]any{
		"name": "A status", "color": "#ff0000", "position": 0, "category": "todo",
	}, cookieA)
	requireStatus(t, resp, http.StatusForbidden, "A admin creates status in B")
	resp.Body.Close()

	// 8. Cannot list cycles in B.
	resp = doJSON(t, http.MethodGet, app.URL("/api/projects/"+projectB+"/cycles"), nil, cookieA)
	requireStatus(t, resp, http.StatusForbidden, "A admin lists B cycles")
	resp.Body.Close()

	// 9. Cannot access B's project members.
	resp = doJSON(t, http.MethodGet, app.URL("/api/projects/"+projectB+"/members"), nil, cookieA)
	requireStatus(t, resp, http.StatusForbidden, "A admin lists B members")
	resp.Body.Close()

	resp = doJSON(t, http.MethodPost, app.URL("/api/projects/"+projectB+"/members"), map[string]any{
		"user_id": "nonexistent", "role": "member",
	}, cookieA)
	requireStatus(t, resp, http.StatusForbidden, "A admin adds member to B")
	resp.Body.Close()

	// 10. Org A's project list does not include project B.
	resp = doJSON(t, http.MethodGet, app.URL("/api/projects"), nil, cookieA)
	var listA []projectResponse
	readBodyJSON(t, resp, &listA)
	for _, p := range listA {
		if p.ID == projectB {
			t.Error("project B leaked into org A's project list")
		}
	}

	_ = projectA // used for sanity check
	t.Log("all cross-org isolation checks passed")
}
