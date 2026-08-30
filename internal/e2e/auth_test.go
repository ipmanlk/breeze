package e2e

import (
	"net/http"
	"testing"
)

func TestAuthFlow(t *testing.T) {
	app := newE2EApp(t)
	setupOrg(t, app)

	cookie := loginCookie(t, app)

	// Me with valid session.
	resp := doJSON(t, http.MethodGet, app.URL("/api/auth/me"), nil, cookie)
	var me userResponse
	readBodyJSON(t, resp, &me)
	if me.Email != "admin@test.com" {
		t.Errorf("email = %q, want admin@test.com", me.Email)
	}
	if me.Role != "owner" {
		t.Errorf("role = %q, want owner", me.Role)
	}

	// Logout.
	resp = doJSON(t, http.MethodPost, app.URL("/api/auth/logout"), nil, cookie)
	resp.Body.Close()
	requireStatus(t, resp, http.StatusOK, "logout")

	// Me after logout: must be 401.
	resp = doJSON(t, http.MethodGet, app.URL("/api/auth/me"), nil, cookie)
	resp.Body.Close()
	requireStatus(t, resp, http.StatusUnauthorized, "me after logout")

	// Me without any auth: must be 401.
	resp = doJSON(t, http.MethodGet, app.URL("/api/auth/me"), nil, "")
	resp.Body.Close()
	requireStatus(t, resp, http.StatusUnauthorized, "me without auth")

	// Wrong password.
	resp = doJSON(t, http.MethodPost, app.URL("/api/auth/login"), map[string]string{
		"email": "admin@test.com", "password": "wrong",
	}, "")
	resp.Body.Close()
	requireStatus(t, resp, http.StatusUnauthorized, "wrong password login")
}
