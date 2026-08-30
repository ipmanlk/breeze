package e2e

import (
	"net/http"
	"strings"
	"testing"
)

func TestMembersInviteAndAccept(t *testing.T) {
	app := newE2EApp(t)
	setupOrg(t, app)

	adminCookie := loginCookie(t, app)

	// 1. Admin creates an invite for a member
	resp := doJSON(t, http.MethodPost, app.URL("/api/invites"), map[string]string{"role": "member"}, adminCookie)
	defer resp.Body.Close()
	requireStatus(t, resp, http.StatusCreated, "create invite")

	var inviteResp struct {
		ID    string `json:"id"`
		Token string `json:"token"`
		URL   string `json:"url"`
		Role  string `json:"role"`
	}
	readBodyJSON(t, resp, &inviteResp)
	if inviteResp.Token == "" {
		t.Fatal("invite token is empty")
	}
	if inviteResp.Role != "member" {
		t.Errorf("role = %q, want member", inviteResp.Role)
	}

	// 2. New user accepts the invite (public endpoint, no auth)
	acceptResp := doJSON(t, http.MethodPost, app.URL("/api/invites/"+inviteResp.Token+"/accept"), map[string]any{
		"name":     "Jane Doe",
		"email":    "jane@test.com",
		"password": "password123",
	}, "")
	defer acceptResp.Body.Close()
	requireStatus(t, acceptResp, http.StatusCreated, "accept invite")

	var userResp struct {
		ID       string `json:"id"`
		Name     string `json:"name"`
		Email    string `json:"email"`
		Role     string `json:"role"`
		IsActive bool   `json:"is_active"`
	}
	readBodyJSON(t, acceptResp, &userResp)
	if userResp.Name != "Jane Doe" {
		t.Errorf("name = %q, want Jane Doe", userResp.Name)
	}
	if userResp.Role != "member" {
		t.Errorf("role = %q, want member", userResp.Role)
	}
	if !userResp.IsActive {
		t.Error("is_active = false, want true")
	}

	// 3. Admin lists users and sees the new member
	resp = doJSON(t, http.MethodGet, app.URL("/api/users"), nil, adminCookie)
	defer resp.Body.Close()
	requireStatus(t, resp, http.StatusOK, "list users")

	var listResp struct {
		Items []struct {
			ID       string `json:"id"`
			Name     string `json:"name"`
			Email    string `json:"email"`
			Role     string `json:"role"`
			IsActive bool   `json:"is_active"`
		} `json:"items"`
		HasMore bool `json:"has_more"`
	}
	readBodyJSON(t, resp, &listResp)
	found := false
	for _, u := range listResp.Items {
		if u.Email == "jane@test.com" {
			found = true
			if u.Role != "member" {
				t.Errorf("listed role = %q, want member", u.Role)
			}
		}
	}
	if !found {
		t.Error("new user not found in member list")
	}

	// 4. Admin updates the new user's role to admin
	resp = doJSON(t, http.MethodPut, app.URL("/api/users/"+userResp.ID+"/role"), map[string]string{"role": "admin"}, adminCookie)
	defer resp.Body.Close()
	requireStatus(t, resp, http.StatusOK, "update role")

	var updatedUser struct {
		Role string `json:"role"`
	}
	readBodyJSON(t, resp, &updatedUser)
	if updatedUser.Role != "admin" {
		t.Errorf("updated role = %q, want admin", updatedUser.Role)
	}

	// 5. Admin deactivates the user
	resp = doJSON(t, http.MethodPut, app.URL("/api/users/"+userResp.ID+"/active"), map[string]bool{"is_active": false}, adminCookie)
	defer resp.Body.Close()
	requireStatus(t, resp, http.StatusOK, "deactivate user")

	var deactivatedUser struct {
		IsActive bool `json:"is_active"`
	}
	readBodyJSON(t, resp, &deactivatedUser)
	if deactivatedUser.IsActive {
		t.Error("is_active = true after deactivation")
	}

	// 6. Deactivated user cannot login
	loginResp := doJSON(t, http.MethodPost, app.URL("/api/auth/login"), map[string]string{
		"email": "jane@test.com", "password": "password123",
	}, "")
	defer loginResp.Body.Close()
	if loginResp.StatusCode != http.StatusUnauthorized {
		t.Errorf("deactivated login status = %d, want 401", loginResp.StatusCode)
	}

	// 7. Admin reactivates the user
	resp = doJSON(t, http.MethodPut, app.URL("/api/users/"+userResp.ID+"/active"), map[string]bool{"is_active": true}, adminCookie)
	defer resp.Body.Close()
	requireStatus(t, resp, http.StatusOK, "reactivate user")

	// 8. Reactivated user can login again
	janeCookie := loginAs(t, app, "jane@test.com", "password123")
	if !strings.HasPrefix(janeCookie, "__Host-token=") {
		t.Error("reactivated user could not login")
	}

	// 9. Admin revokes the invite
	resp = doJSON(t, http.MethodDelete, app.URL("/api/invites/"+inviteResp.ID), nil, adminCookie)
	defer resp.Body.Close()
	requireStatus(t, resp, http.StatusNoContent, "revoke invite")

	// 10. Validate public invite endpoint returns 404 for consumed invite
	validateResp := doJSON(t, http.MethodGet, app.URL("/api/invites/"+inviteResp.Token+"/validate"), nil, "")
	defer validateResp.Body.Close()
	if validateResp.StatusCode != http.StatusNotFound {
		t.Errorf("validate consumed invite status = %d, want 404", validateResp.StatusCode)
	}
}

func TestMembersInvite_EmailRestriction(t *testing.T) {
	app := newE2EApp(t)
	setupOrg(t, app)
	adminCookie := loginCookie(t, app)

	restrictedEmail := "specific@test.com"
	resp := doJSON(t, http.MethodPost, app.URL("/api/invites"), map[string]any{
		"role":  "member",
		"email": restrictedEmail,
	}, adminCookie)
	defer resp.Body.Close()
	requireStatus(t, resp, http.StatusCreated, "create restricted invite")

	var inviteResp struct {
		Token string `json:"token"`
	}
	readBodyJSON(t, resp, &inviteResp)

	// Accept with wrong email should fail
	acceptResp := doJSON(t, http.MethodPost, app.URL("/api/invites/"+inviteResp.Token+"/accept"), map[string]any{
		"name":     "Jane",
		"email":    "wrong@test.com",
		"password": "password123",
	}, "")
	defer acceptResp.Body.Close()
	if acceptResp.StatusCode != http.StatusForbidden {
		t.Errorf("wrong email accept status = %d, want 403", acceptResp.StatusCode)
	}

	// Accept with correct email should succeed
	acceptResp = doJSON(t, http.MethodPost, app.URL("/api/invites/"+inviteResp.Token+"/accept"), map[string]any{
		"name":     "Jane",
		"email":    restrictedEmail,
		"password": "password123",
	}, "")
	defer acceptResp.Body.Close()
	requireStatus(t, acceptResp, http.StatusCreated, "accept with correct email")
}

func TestMembersInvite_RoleCap(t *testing.T) {
	app := newE2EApp(t)
	setupOrg(t, app)
	adminCookie := loginCookie(t, app)

	// Create a member invite, accept it
	resp := doJSON(t, http.MethodPost, app.URL("/api/invites"), map[string]string{"role": "member"}, adminCookie)
	defer resp.Body.Close()
	requireStatus(t, resp, http.StatusCreated, "create member invite")

	var inv struct {
		Token string `json:"token"`
	}
	readBodyJSON(t, resp, &inv)

	accept := doJSON(t, http.MethodPost, app.URL("/api/invites/"+inv.Token+"/accept"), map[string]any{
		"name": "Member", "email": "member@test.com", "password": "password123",
	}, "")
	defer accept.Body.Close()
	requireStatus(t, accept, http.StatusCreated, "accept member invite")

	var mUser struct {
		ID string `json:"id"`
	}
	readBodyJSON(t, accept, &mUser)

	memberCookie := loginAs(t, app, "member@test.com", "password123")

	// Member can invite another member
	resp = doJSON(t, http.MethodPost, app.URL("/api/invites"), map[string]string{"role": "member"}, memberCookie)
	defer resp.Body.Close()
	requireStatus(t, resp, http.StatusCreated, "member invites member")

	// Member cannot invite admin
	resp = doJSON(t, http.MethodPost, app.URL("/api/invites"), map[string]string{"role": "admin"}, memberCookie)
	defer resp.Body.Close()
	if resp.StatusCode != http.StatusForbidden {
		t.Errorf("member invites admin status = %d, want 403", resp.StatusCode)
	}

	// Admin cannot promote member to owner (but owner can: setup user is owner)
	// Let's test admin directly: create an admin user first
	resp = doJSON(t, http.MethodPost, app.URL("/api/invites"), map[string]string{"role": "admin"}, adminCookie)
	defer resp.Body.Close()
	requireStatus(t, resp, http.StatusCreated, "create admin invite")

	var adminInv struct {
		Token string `json:"token"`
	}
	readBodyJSON(t, resp, &adminInv)

	accept = doJSON(t, http.MethodPost, app.URL("/api/invites/"+adminInv.Token+"/accept"), map[string]any{
		"name": "Admin2", "email": "admin2@test.com", "password": "password123",
	}, "")
	defer accept.Body.Close()
	requireStatus(t, accept, http.StatusCreated, "accept admin invite")

	admin2Cookie := loginAs(t, app, "admin2@test.com", "password123")

	// Admin2 cannot promote member to owner
	resp = doJSON(t, http.MethodPut, app.URL("/api/users/"+mUser.ID+"/role"), map[string]string{"role": "owner"}, admin2Cookie)
	defer resp.Body.Close()
	if resp.StatusCode != http.StatusForbidden {
		t.Errorf("admin promotes to owner status = %d, want 403", resp.StatusCode)
	}
}

func TestMembersInvite_DuplicateEmail(t *testing.T) {
	app := newE2EApp(t)
	setupOrg(t, app)
	adminCookie := loginCookie(t, app)

	resp := doJSON(t, http.MethodPost, app.URL("/api/invites"), map[string]string{"role": "member"}, adminCookie)
	defer resp.Body.Close()
	requireStatus(t, resp, http.StatusCreated, "create first invite")

	var inv struct {
		Token string `json:"token"`
	}
	readBodyJSON(t, resp, &inv)

	// First acceptance
	accept := doJSON(t, http.MethodPost, app.URL("/api/invites/"+inv.Token+"/accept"), map[string]any{
		"name": "Alice", "email": "alice@test.com", "password": "password123",
	}, "")
	defer accept.Body.Close()
	requireStatus(t, accept, http.StatusCreated, "first accept")

	// Second invite with same email should fail on accept
	resp = doJSON(t, http.MethodPost, app.URL("/api/invites"), map[string]string{"role": "member"}, adminCookie)
	defer resp.Body.Close()
	requireStatus(t, resp, http.StatusCreated, "create second invite")

	var inv2 struct {
		Token string `json:"token"`
	}
	readBodyJSON(t, resp, &inv2)

	accept2 := doJSON(t, http.MethodPost, app.URL("/api/invites/"+inv2.Token+"/accept"), map[string]any{
		"name": "Alice2", "email": "alice@test.com", "password": "password123",
	}, "")
	defer accept2.Body.Close()
	if accept2.StatusCode != http.StatusConflict {
		t.Errorf("duplicate email accept status = %d, want 409", accept2.StatusCode)
	}
}
