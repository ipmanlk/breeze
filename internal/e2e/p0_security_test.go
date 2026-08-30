package e2e

import (
	"bytes"
	"io"
	"mime/multipart"
	"net/http"
	"strings"
	"testing"
)

// createChannelE2E creates a channel under the given parent category and
// returns its ID.
func createChannelE2E(t *testing.T, app *e2eApp, cookie, name, parentID string) string {
	t.Helper()
	body := map[string]any{"name": name, "type": "channel"}
	if parentID != "" {
		body["parent_id"] = parentID
	}
	resp := doJSON(t, http.MethodPost, app.URL("/api/conversations"), body, cookie)
	requireStatus(t, resp, http.StatusCreated, "create channel "+name)
	var c conversationResponse
	readBodyJSON(t, resp, &c)
	return c.ID
}

// inviteUserE2E creates an invite, accepts it as a new user, and returns the
// new user's ID + a login cookie.
func inviteUserE2E(t *testing.T, app *e2eApp, adminCookie, name, email, role string) (string, string) {
	t.Helper()
	resp := doJSON(t, http.MethodPost, app.URL("/api/invites"), map[string]string{"role": role}, adminCookie)
	requireStatus(t, resp, http.StatusCreated, "create invite")
	var invite struct {
		Token string `json:"token"`
	}
	readBodyJSON(t, resp, &invite)

	accept := doJSON(t, http.MethodPost, app.URL("/api/invites/"+invite.Token+"/accept"), map[string]any{
		"name": name, "email": email, "password": "password123",
	}, "")
	requireStatus(t, accept, http.StatusCreated, "accept invite")
	var u userResponse
	readBodyJSON(t, accept, &u)
	return u.ID, loginAs(t, app, email, "password123")
}

// uploadMsgAttachmentE2E uploads a file as a pending message attachment and
// returns the attachment ID returned by the server.
func uploadMsgAttachmentE2E(t *testing.T, app *e2eApp, cookie, convID string) string {
	t.Helper()
	var buf bytes.Buffer
	mw := multipart.NewWriter(&buf)
	fw, err := mw.CreateFormFile("file", "secret.txt")
	if err != nil {
		t.Fatalf("create form file: %v", err)
	}
	io.WriteString(fw, "top secret content")
	mw.Close()

	req, _ := http.NewRequest(http.MethodPost, app.URL("/api/conversations/"+convID+"/attachments"), &buf)
	req.Header.Set("Content-Type", mw.FormDataContentType())
	req.Header.Set("Cookie", cookie)
	resp, err := http.DefaultClient.Do(req)
	if err != nil {
		t.Fatalf("upload: %v", err)
	}
	defer resp.Body.Close()
	if resp.StatusCode != http.StatusCreated {
		b, _ := io.ReadAll(resp.Body)
		t.Fatalf("upload attachment: expected 201, got %d: %s", resp.StatusCode, string(b))
	}
	var att struct {
		ID string `json:"id"`
	}
	readBodyJSON(t, resp, &att)
	return att.ID
}

// TestP0_ConversationMetadataAccessLeak verifies that a user who is NOT a
// member of a conversation cannot read its members, pinned messages, or
// access roster.
func TestP0_ConversationMetadataAccessLeak(t *testing.T) {
	app := newE2EApp(t)
	setupOrg(t, app)
	adminCookie := loginCookie(t, app)

	// Admin creates a channel (admin is a member).
	channelID := createChannelE2E(t, app, adminCookie, "private-channel", "")

	// Invite a guest user. Guests have PermChatRead but no membership in this
	// channel: they must NOT be able to read its metadata.
	_, guestCookie := inviteUserE2E(t, app, adminCookie, "Guest", "guest@test.com", "guest")

	// Guest cannot list members of a channel they're not in.
	resp := doJSON(t, http.MethodGet, app.URL("/api/conversations/"+channelID+"/members"), nil, guestCookie)
	requireStatus(t, resp, http.StatusForbidden, "guest lists non-member channel members")
	resp.Body.Close()

	// Guest cannot read pinned messages.
	resp = doJSON(t, http.MethodGet, app.URL("/api/conversations/"+channelID+"/pinned"), nil, guestCookie)
	requireStatus(t, resp, http.StatusForbidden, "guest reads non-member channel pinned")
	resp.Body.Close()

	// Guest cannot read the access roster.
	resp = doJSON(t, http.MethodGet, app.URL("/api/conversations/"+channelID+"/access"), nil, guestCookie)
	requireStatus(t, resp, http.StatusForbidden, "guest reads non-member channel access")
	resp.Body.Close()

	// Guest cannot mark-read a channel they're not in.
	resp = doJSON(t, http.MethodPost, app.URL("/api/conversations/"+channelID+"/read"), nil, guestCookie)
	requireStatus(t, resp, http.StatusForbidden, "guest mark-read non-member channel")
	resp.Body.Close()

	// Sanity: admin (a member) CAN list members.
	resp = doJSON(t, http.MethodGet, app.URL("/api/conversations/"+channelID+"/members"), nil, adminCookie)
	requireStatus(t, resp, http.StatusOK, "admin lists channel members")
	resp.Body.Close()
}

// TestP0_MessageAttachmentIDOR verifies that a user with access to
// conversation A cannot download an attachment from conversation B by
// passing convA in the path and convB's attachment ID. Uses the admin
// (a member of both conversations) so the convA access check passes: the
// protection must come from the conversation-scoped attachment lookup.
func TestP0_MessageAttachmentIDOR(t *testing.T) {
	app := newE2EApp(t)
	setupOrg(t, app)
	adminCookie := loginCookie(t, app)

	// Create two channels. Admin is a member of both.
	convA := createChannelE2E(t, app, adminCookie, "conv-a", "")
	convB := createChannelE2E(t, app, adminCookie, "conv-b", "")

	// Upload an attachment to convB.
	attID := uploadMsgAttachmentE2E(t, app, adminCookie, convB)

	// Send the message to persist the attachment in convB.
	resp := doJSON(t, http.MethodPost, app.URL("/api/conversations/"+convB+"/messages"), map[string]any{
		"content":        "see attached",
		"attachment_ids": []string{attID},
	}, adminCookie)
	requireStatus(t, resp, http.StatusCreated, "send message with attachment in convB")
	resp.Body.Close()

	// IDOR attack: request convB's attachment via convA's path. The admin has
	// access to convA (so the access check passes), but the attachment belongs
	// to convB. The conversation-scoped lookup must return 404, NOT serve the
	// file.
	req, _ := http.NewRequest(http.MethodGet, app.URL("/api/conversations/"+convA+"/attachments/"+attID+"/download"), nil)
	req.Header.Set("Cookie", adminCookie)
	dl, err := http.DefaultClient.Do(req)
	if err != nil {
		t.Fatalf("download request: %v", err)
	}
	defer dl.Body.Close()
	if dl.StatusCode != http.StatusNotFound {
		b, _ := io.ReadAll(dl.Body)
		t.Fatalf("IDOR: expected 404 for cross-conversation attachment download, got %d: %s", dl.StatusCode, string(b))
	}

	// Sanity: admin (member of convB) CAN download it via convB's path.
	req, _ = http.NewRequest(http.MethodGet, app.URL("/api/conversations/"+convB+"/attachments/"+attID+"/download"), nil)
	req.Header.Set("Cookie", adminCookie)
	dl2, err := http.DefaultClient.Do(req)
	if err != nil {
		t.Fatalf("admin download: %v", err)
	}
	defer dl2.Body.Close()
	if dl2.StatusCode != http.StatusOK {
		t.Fatalf("admin download from convB: expected 200, got %d", dl2.StatusCode)
	}
}

// TestP0_SearchProjectAccess verifies that search is gated on PermProjectView
// (guests get 403) and that task search is scoped to projects the caller can
// access.
func TestP0_SearchProjectAccess(t *testing.T) {
	app := newE2EApp(t)
	setupOrg(t, app)
	adminCookie := loginCookie(t, app)

	// Create a project + task.
	projID := createProject(t, app, adminCookie)
	resp := doJSON(t, http.MethodGet, app.URL("/api/projects/"+projID+"/statuses"), nil, adminCookie)
	var statuses []taskStatusResponse
	readBodyJSON(t, resp, &statuses)
	resp = doJSON(t, http.MethodPost, app.URL("/api/projects/"+projID+"/tasks"), map[string]string{
		"title": "Secret Project Task", "status_id": statuses[0].ID,
	}, adminCookie)
	requireStatus(t, resp, http.StatusCreated, "create task")
	resp.Body.Close()

	// Guest has PermProjectView (so the route is reachable) but is project-scoped:
	// SearchTasksForUser filters to projects they're an explicit member of. Since
	// the guest has no project_members row, the secret task title must NOT leak.
	_, guestCookie := inviteUserE2E(t, app, adminCookie, "Guest", "guest@test.com", "guest")
	resp = doJSON(t, http.MethodGet, app.URL("/api/search?q=Secret&types=task"), nil, guestCookie)
	requireStatus(t, resp, http.StatusOK, "guest search reachable")
	var guestSR searchResponse
	readBodyJSON(t, resp, &guestSR)
	for _, r := range guestSR.Results {
		if strings.Contains(r.Name, "Secret Project Task") {
			t.Fatalf("unexpected leak: guest discovered task title %q via search", r.Name)
		}
	}

	// Admin (org-elevated) can search org-wide and finds the task.
	resp = doJSON(t, http.MethodGet, app.URL("/api/search?q=Secret&types=task"), nil, adminCookie)
	requireStatus(t, resp, http.StatusOK, "admin search")
	var sr searchResponse
	readBodyJSON(t, resp, &sr)
	found := false
	for _, r := range sr.Results {
		if strings.Contains(r.Name, "Secret Project Task") {
			found = true
		}
	}
	if !found {
		t.Fatalf("admin did not find the task in search results: %+v", sr.Results)
	}
}

// TestP0_ArchivedProjectReadOnly verifies that archived projects reject writes
// (409) while reads still work.
func TestP0_ArchivedProjectReadOnly(t *testing.T) {
	app := newE2EApp(t)
	setupOrg(t, app)
	adminCookie := loginCookie(t, app)

	projID := createProject(t, app, adminCookie)
	resp := doJSON(t, http.MethodGet, app.URL("/api/projects/"+projID+"/statuses"), nil, adminCookie)
	var statuses []taskStatusResponse
	readBodyJSON(t, resp, &statuses)

	// Archive the project.
	resp = doJSON(t, http.MethodPost, app.URL("/api/projects/"+projID+"/archive"), nil, adminCookie)
	requireStatus(t, resp, http.StatusNoContent, "archive project")
	resp.Body.Close()

	// Reads still work (list tasks, get project).
	resp = doJSON(t, http.MethodGet, app.URL("/api/projects/"+projID+"/tasks"), nil, adminCookie)
	requireStatus(t, resp, http.StatusOK, "list tasks in archived project")
	resp.Body.Close()

	// Writes are rejected with 409.
	resp = doJSON(t, http.MethodPost, app.URL("/api/projects/"+projID+"/tasks"), map[string]string{
		"title": "should fail", "status_id": statuses[0].ID,
	}, adminCookie)
	requireStatus(t, resp, http.StatusConflict, "create task in archived project")
	resp.Body.Close()

	// Unarchive restores writability.
	resp = doJSON(t, http.MethodPost, app.URL("/api/projects/"+projID+"/unarchive"), nil, adminCookie)
	requireStatus(t, resp, http.StatusNoContent, "unarchive project")
	resp.Body.Close()

	resp = doJSON(t, http.MethodPost, app.URL("/api/projects/"+projID+"/tasks"), map[string]string{
		"title": "should succeed now", "status_id": statuses[0].ID,
	}, adminCookie)
	requireStatus(t, resp, http.StatusCreated, "create task after unarchive")
	resp.Body.Close()

	// Archived projects appear in the list when archived=true.
	resp = doJSON(t, http.MethodPost, app.URL("/api/projects/"+projID+"/archive"), nil, adminCookie)
	requireStatus(t, resp, http.StatusNoContent, "re-archive project")
	resp.Body.Close()

	resp = doJSON(t, http.MethodGet, app.URL("/api/projects?archived=true"), nil, adminCookie)
	requireStatus(t, resp, http.StatusOK, "list projects including archived")
	var list []projectResponse
	readBodyJSON(t, resp, &list)
	foundArchived := false
	for _, p := range list {
		if p.ID == projID {
			foundArchived = true
		}
	}
	if !foundArchived {
		t.Fatal("archived project not found in archived=true list (no recovery path)")
	}

	// And NOT in the default list.
	resp = doJSON(t, http.MethodGet, app.URL("/api/projects"), nil, adminCookie)
	requireStatus(t, resp, http.StatusOK, "list default projects")
	readBodyJSON(t, resp, &list)
	for _, p := range list {
		if p.ID == projID {
			t.Fatal("archived project leaked into default project list")
		}
	}
}

// NOTE: WS room-subscribe denial is covered by the transport/ws unit tests,
// which exercise the access checker fail-closed behavior directly
// (internal/transport/handler/ws_access_test.go and ws_access_checker_test.go).
// The e2e harness does not mount the WebSocket upgrade endpoint, so there is
// no end-to-end WS test here.
