package e2e

import (
	"net/http"
	"testing"
)

func TestNotifications(t *testing.T) {
	app := newE2EApp(t)
	setupOrg(t, app)
	cookie := loginCookie(t, app)

	t.Run("unread_count_zero", func(t *testing.T) {
		resp := doJSON(t, http.MethodGet, app.URL("/api/notifications/unread-count"), nil, cookie)
		var c unreadCountResponse
		readBodyJSON(t, resp, &c)
		if c.Count != 0 {
			t.Fatalf("expected 0 unread, got %d", c.Count)
		}
	})

	t.Run("preferences_all_present", func(t *testing.T) {
		resp := doJSON(t, http.MethodGet, app.URL("/api/settings/notifications"), nil, cookie)
		var prefs []notifPrefResponse
		readBodyJSON(t, resp, &prefs)
		if len(prefs) < 7 {
			t.Fatalf("expected at least 7 preferences, got %d", len(prefs))
		}
	})

	t.Run("toggle_preference", func(t *testing.T) {
		resp := doJSON(t, http.MethodPatch, app.URL("/api/settings/notifications/task_assigned"), map[string]bool{"enabled": false}, cookie)
		resp.Body.Close()
		requireStatus(t, resp, http.StatusOK, "set preference")

		resp = doJSON(t, http.MethodGet, app.URL("/api/settings/notifications"), nil, cookie)
		var prefs []notifPrefResponse
		readBodyJSON(t, resp, &prefs)
		for _, p := range prefs {
			if p.Type == "task_assigned" && p.Enabled {
				t.Fatal("task_assigned should be disabled after toggle")
			}
		}
	})

	t.Run("mark_all_read", func(t *testing.T) {
		resp := doJSON(t, http.MethodPatch, app.URL("/api/notifications/read-all"), nil, cookie)
		resp.Body.Close()
		requireStatus(t, resp, http.StatusOK, "mark all read")
	})
}
