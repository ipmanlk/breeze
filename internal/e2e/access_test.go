package e2e

import (
	"fmt"
	"net/http"
	"testing"
)

func TestUnauthenticatedAccess(t *testing.T) {
	app := newE2EApp(t)
	setupOrg(t, app)

	tests := []struct {
		method string
		path   string
	}{
		{http.MethodGet, "/api/auth/me"},
		{http.MethodGet, "/api/projects"},
		{http.MethodPost, "/api/projects"},
		{http.MethodGet, "/api/notifications"},
		{http.MethodGet, "/api/notifications/unread-count"},
		{http.MethodPatch, "/api/notifications/read-all"},
		{http.MethodPatch, "/api/notifications/n1/read"},
		{http.MethodGet, "/api/users"},
	}

	for _, tc := range tests {
		t.Run(fmt.Sprintf("%s %s", tc.method, tc.path), func(t *testing.T) {
			resp := doJSON(t, tc.method, app.URL(tc.path), nil, "")
			resp.Body.Close()
			if resp.StatusCode != http.StatusUnauthorized {
				t.Errorf("%s %s: expected 401, got %d", tc.method, tc.path, resp.StatusCode)
			}
		})
	}
}
