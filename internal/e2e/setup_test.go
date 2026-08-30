package e2e

import (
	"net/http"
	"testing"
)

func TestSetupFlow(t *testing.T) {
	app := newE2EApp(t)

	// Pre-setup: needs_setup must be true.
	resp := doJSON(t, http.MethodGet, app.URL("/api/setup"), nil, "")
	var check setupCheckResponse
	readBodyJSON(t, resp, &check)
	if !check.NeedsSetup {
		t.Fatal("expected needs_setup=true")
	}

	// Perform setup.
	resp = doJSON(t, http.MethodPost, app.URL("/api/setup"), map[string]string{
		"org_name": "Acme Corp",
		"name":     "Admin",
		"email":    "admin@test.com",
		"password": "admin123",
	}, "")
	resp.Body.Close()
	requireStatus(t, resp, http.StatusOK, "setup")

	// Post-setup: needs_setup must be false.
	resp = doJSON(t, http.MethodGet, app.URL("/api/setup"), nil, "")
	readBodyJSON(t, resp, &check)
	if check.NeedsSetup {
		t.Fatal("expected needs_setup=false after setup")
	}

	// Second setup must be rejected.
	resp = doJSON(t, http.MethodPost, app.URL("/api/setup"), map[string]string{
		"org_name": "Second", "name": "Admin",
		"email": "admin@test.com", "password": "admin123",
	}, "")
	resp.Body.Close()
	requireStatus(t, resp, http.StatusConflict, "second setup")
}
