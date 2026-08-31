package e2e

import (
	"net/http"
	"testing"

	"ipmanlk/plume/internal/transport/dto"
)

func createProject(t *testing.T, app *e2eApp, cookie string) string {
	t.Helper()
	resp := doJSON(t, http.MethodPost, app.URL("/api/projects"), map[string]string{"name": "Test Project", "slug": "test-project"}, cookie)
	if resp.StatusCode != http.StatusOK && resp.StatusCode != http.StatusCreated {
		t.Fatalf("createProject: expected 200 or 201, got %d", resp.StatusCode)
	}
	var p projectResponse
	readBodyJSON(t, resp, &p)
	return p.ID
}

func TestChatE2E_CreateChannelAndPostMessage(t *testing.T) {
	app := newE2EApp(t)
	setupOrg(t, app)
	cookie := loginCookie(t, app)

	projID := createProject(t, app, cookie)

	// Create a category (type=category, parent_id=null)
	resp := doJSON(t, http.MethodPost, app.URL("/api/conversations"), map[string]any{
		"name": "General",
		"type": "category",
	}, cookie)
	if resp.StatusCode != http.StatusCreated {
		t.Fatalf("createCategory: expected 201, got %d: %s", resp.StatusCode, readBodyStr(t, resp))
	}
	var cat conversationResponse
	readBodyJSON(t, resp, &cat)
	if cat.ID == "" {
		t.Fatal("expected non-empty category ID")
	}
	if cat.Name != "General" {
		t.Errorf("category name = %s, want General", cat.Name)
	}
	if cat.Type != "category" {
		t.Errorf("category type = %s, want category", cat.Type)
	}

	// List conversations (workspace scope includes categories)
	resp = doJSON(t, http.MethodGet, app.URL("/api/conversations?scope=workspace"), nil, cookie)
	requireStatus(t, resp, http.StatusOK, "listConversations")
	var convList conversationListResponse
	readBodyJSON(t, resp, &convList)
	if len(convList.Items) < 1 {
		t.Fatal("expected at least 1 conversation")
	}
	found := false
	for _, c := range convList.Items {
		if c.Name == "General" && c.Type == "category" {
			found = true
			break
		}
	}
	if !found {
		t.Error("category 'General' not found in workspace list")
	}

	// Create a channel under the category
	resp = doJSON(t, http.MethodPost, app.URL("/api/conversations"), map[string]any{
		"name":        "test-channel",
		"type":        "channel",
		"parent_id":   cat.ID,
		"project_ids": []string{projID},
	}, cookie)
	if resp.StatusCode != http.StatusCreated {
		t.Fatalf("createChannel: expected 201, got %d: %s", resp.StatusCode, readBodyStr(t, resp))
	}
	var conv conversationResponse
	readBodyJSON(t, resp, &conv)
	if conv.ID == "" {
		t.Fatal("expected non-empty conversation ID")
	}

	// List conversations
	resp = doJSON(t, http.MethodGet, app.URL("/api/conversations"), nil, cookie)
	requireStatus(t, resp, http.StatusOK, "listConversations")
	readBodyJSON(t, resp, &convList)
	if len(convList.Items) < 1 {
		t.Fatal("expected at least 1 conversation")
	}

	// Send a message
	resp = doJSON(t, http.MethodPost, app.URL("/api/conversations/"+conv.ID+"/messages"), map[string]string{
		"content": "Hello from e2e!",
	}, cookie)
	if resp.StatusCode != http.StatusCreated {
		t.Fatalf("sendMessage: expected 201, got %d: %s", resp.StatusCode, readBodyStr(t, resp))
	}
	var msg messageResponse
	readBodyJSON(t, resp, &msg)
	if msg.ID == "" {
		t.Fatal("expected non-empty message ID")
	}
	if msg.Content != "Hello from e2e!" {
		t.Errorf("message content = %s, want 'Hello from e2e!'", msg.Content)
	}

	// List messages
	resp = doJSON(t, http.MethodGet, app.URL("/api/conversations/"+conv.ID+"/messages"), nil, cookie)
	requireStatus(t, resp, http.StatusOK, "listMessages")
	var msgList messageListResponse
	readBodyJSON(t, resp, &msgList)
	if len(msgList.Items) != 1 {
		t.Fatalf("expected 1 message, got %d", len(msgList.Items))
	}
	if msgList.Items[0].Content != "Hello from e2e!" {
		t.Errorf("listed message content = %s, want 'Hello from e2e!'", msgList.Items[0].Content)
	}

	// Get channel by id
	resp = doJSON(t, http.MethodGet, app.URL("/api/conversations/"+conv.ID), nil, cookie)
	requireStatus(t, resp, http.StatusOK, "getConversation")
	var gotConv conversationResponse
	readBodyJSON(t, resp, &gotConv)
	if gotConv.Name != "test-channel" {
		t.Errorf("conversation name = %s, want test-channel", gotConv.Name)
	}

	// Check my-permissions
	resp = doJSON(t, http.MethodGet, app.URL("/api/conversations/"+conv.ID+"/my-permissions"), nil, cookie)
	requireStatus(t, resp, http.StatusOK, "myPermissions")
	var perms struct {
		CanView bool `json:"can_view"`
		CanSend bool `json:"can_send"`
	}
	readBodyJSON(t, resp, &perms)
	if !perms.CanView {
		t.Error("expected can_view=true")
	}
	if !perms.CanSend {
		t.Error("expected can_send=true")
	}

	// Update conversation (rename)
	resp = doJSON(t, http.MethodPatch, app.URL("/api/conversations/"+conv.ID), map[string]string{
		"name": "renamed-channel",
	}, cookie)
	requireStatus(t, resp, http.StatusOK, "updateConversation")

	// Verify rename
	resp = doJSON(t, http.MethodGet, app.URL("/api/conversations/"+conv.ID), nil, cookie)
	requireStatus(t, resp, http.StatusOK, "getRenamedConversation")
	readBodyJSON(t, resp, &gotConv)
	if gotConv.Name != "renamed-channel" {
		t.Errorf("renamed conversation name = %s, want renamed-channel", gotConv.Name)
	}

	// Delete conversation
	resp = doJSON(t, http.MethodDelete, app.URL("/api/conversations/"+conv.ID), nil, cookie)
	requireStatus(t, resp, http.StatusNoContent, "deleteConversation")

	// Verify deletion: conversation should not appear in listing
	resp = doJSON(t, http.MethodGet, app.URL("/api/conversations?scope=workspace"), nil, cookie)
	requireStatus(t, resp, http.StatusOK, "listAfterDelete")
	var listResp dto.ConversationListResponse
	readBodyJSON(t, resp, &listResp)
	for _, item := range listResp.Items {
		if item.ID == conv.ID {
			t.Errorf("deleted conversation still appears in list")
			break
		}
	}

	// Verify get by ID still returns the conversation with data (soft delete)
	resp = doJSON(t, http.MethodGet, app.URL("/api/conversations/"+conv.ID), nil, cookie)
	requireStatus(t, resp, http.StatusOK, "getDeletedConversation")
	var deletedConv dto.ConversationResponse
	readBodyJSON(t, resp, &deletedConv)
	if deletedConv.ID == "" {
		t.Errorf("soft-deleted conversation should still be retrievable by ID")
	}
}

func TestChatE2E_DeleteCategoryCascade(t *testing.T) {
	app := newE2EApp(t)
	setupOrg(t, app)
	cookie := loginCookie(t, app)

	// Create a category
	resp := doJSON(t, http.MethodPost, app.URL("/api/conversations"), map[string]any{
		"name": "Engineering",
		"type": "category",
	}, cookie)
	requireStatus(t, resp, http.StatusCreated, "createCategory")
	var cat conversationResponse
	readBodyJSON(t, resp, &cat)

	// Create channels under the category
	channelIDs := make([]string, 2)
	for i, name := range []string{"frontend", "backend"} {
		resp = doJSON(t, http.MethodPost, app.URL("/api/conversations"), map[string]any{
			"name":      name,
			"type":      "channel",
			"parent_id": cat.ID,
		}, cookie)
		requireStatus(t, resp, http.StatusCreated, "createChannel_"+name)
		var ch conversationResponse
		readBodyJSON(t, resp, &ch)
		channelIDs[i] = ch.ID
	}

	// Verify channels appear in listing before delete
	resp = doJSON(t, http.MethodGet, app.URL("/api/conversations?scope=workspace"), nil, cookie)
	requireStatus(t, resp, http.StatusOK, "listBeforeDelete")
	var list conversationListResponse
	readBodyJSON(t, resp, &list)
	for _, id := range channelIDs {
		found := false
		for _, item := range list.Items {
			if item.ID == id {
				found = true
				break
			}
		}
		if !found {
			t.Errorf("channel %s should appear before delete", id)
		}
	}

	// Find the category and verify it appears
	catFound := false
	for _, item := range list.Items {
		if item.ID == cat.ID {
			catFound = true
			break
		}
	}
	if !catFound {
		t.Error("category should appear before delete")
	}

	// Delete the category (cascade to channels)
	resp = doJSON(t, http.MethodDelete, app.URL("/api/conversations/"+cat.ID), nil, cookie)
	requireStatus(t, resp, http.StatusNoContent, "deleteCategory")

	// Verify category is no longer in listing
	resp = doJSON(t, http.MethodGet, app.URL("/api/conversations?scope=workspace"), nil, cookie)
	requireStatus(t, resp, http.StatusOK, "listAfterCategoryDelete")
	readBodyJSON(t, resp, &list)
	for _, item := range list.Items {
		if item.ID == cat.ID {
			t.Errorf("deleted category still appears in list")
		}
	}

	// Verify child channels are no longer in listing (cascade delete)
	for _, id := range channelIDs {
		for _, item := range list.Items {
			if item.ID == id {
				t.Errorf("deleted channel %s still appears in list after category delete", id)
			}
		}
	}

	// Verify soft delete: category still retrievable by ID
	resp = doJSON(t, http.MethodGet, app.URL("/api/conversations/"+cat.ID), nil, cookie)
	requireStatus(t, resp, http.StatusOK, "getDeletedCategory")

	// Verify channels still retrievable by ID (soft delete)
	for _, id := range channelIDs {
		resp = doJSON(t, http.MethodGet, app.URL("/api/conversations/"+id), nil, cookie)
		requireStatus(t, resp, http.StatusOK, "getDeletedChannel_"+id)
	}
}
