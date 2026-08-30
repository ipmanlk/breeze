package store

import (
	"context"
	"database/sql"
	"path/filepath"
	"strings"
	"testing"

	"ipmanlk/breeze/internal/domain"
	"ipmanlk/breeze/internal/store/migration"
	"ipmanlk/breeze/internal/store/sqlc"

	_ "modernc.org/sqlite"
)

func setupMessageStoreTest(t *testing.T) (context.Context, *sql.DB, *MessageStore, *ReactionStore, string, string) {
	t.Helper()
	ctx := context.Background()
	tmpDir := t.TempDir()
	dbPath := filepath.Join(tmpDir, "test.db")

	conn, err := NewDB(dbPath)
	if err != nil {
		t.Fatalf("open db: %v", err)
	}
	t.Cleanup(func() { conn.Close() })

	if err := migration.RunMigrations(conn); err != nil {
		t.Fatalf("run migrations: %v", err)
	}

	q := sqlc.New(conn)

	orgID := "org-1"
	userID := "user-1"

	_, err = conn.Exec(`INSERT INTO organizations (id, name, slug) VALUES (?, 'Test Org', 'test-org')`, orgID)
	if err != nil {
		t.Fatalf("insert org: %v", err)
	}

	_, err = conn.Exec(`INSERT INTO accounts (id, email, password_hash) VALUES ('acct-1', 'alice@test.com', 'hash')`)
	if err != nil {
		t.Fatalf("insert account: %v", err)
	}

	_, err = conn.Exec(`INSERT INTO users (id, account_id, org_id, name, email) VALUES (?, 'acct-1', ?, 'Alice', 'alice@test.com')`, userID, orgID)
	if err != nil {
		t.Fatalf("insert user: %v", err)
	}

	_, err = conn.Exec(`INSERT INTO conversations (id, org_id, name, type, created_by) VALUES (?, ?, 'general', 'channel', ?)`, "conv-1", orgID, userID)
	if err != nil {
		t.Fatalf("insert conversation: %v", err)
	}

	_, err = conn.Exec(`INSERT INTO conversation_members (conversation_id, user_id, org_id, joined_at) VALUES (?, ?, ?, datetime('now'))`, "conv-1", userID, orgID)
	if err != nil {
		t.Fatalf("insert member: %v", err)
	}

	msgStore := NewMessageStore(q)
	reactionStore := NewReactionStore(q)
	return ctx, conn, msgStore, reactionStore, orgID, userID
}

func insertMessage(t *testing.T, conn *sql.DB, id, convID, orgID, senderID, content string) {
	t.Helper()
	_, err := conn.Exec(
		`INSERT INTO messages (id, conversation_id, org_id, sender_id, content, search_content, created_at) VALUES (?, ?, ?, ?, ?, ?, datetime('now'))`,
		id, convID, orgID, senderID, content, domain.StripMentionTokens(content),
	)
	if err != nil {
		t.Fatalf("insert message %s: %v", id, err)
	}
}

func TestMessageStore_CreateAndGet(t *testing.T) {
	ctx, _, msgStore, _, orgID, userID := setupMessageStoreTest(t)

	msg := &domain.Message{
		ID:             "msg-1",
		ConversationID: "conv-1",
		OrgID:          orgID,
		SenderID:       userID,
		Content:        "Hello, world!",
	}
	if err := msgStore.Create(ctx, msg); err != nil {
		t.Fatalf("create message: %v", err)
	}
	if msg.CreatedAt.IsZero() {
		t.Fatal("expected created_at to be set after create")
	}

	got, err := msgStore.GetByID(ctx, "msg-1", "conv-1")
	if err != nil {
		t.Fatalf("get message: %v", err)
	}
	if got.Content != "Hello, world!" {
		t.Errorf("expected 'Hello, world!', got '%s'", got.Content)
	}
	if got.SenderID != userID {
		t.Errorf("expected sender %s, got %s", userID, got.SenderID)
	}
	if got.Sender == nil || got.Sender.Name != "Alice" {
		t.Errorf("expected sender with name Alice, got %+v", got.Sender)
	}
}

func TestMessageStore_CreateAndGetAnyConv(t *testing.T) {
	ctx, _, msgStore, _, orgID, userID := setupMessageStoreTest(t)

	msg := &domain.Message{
		ID:             "msg-1",
		ConversationID: "conv-1",
		OrgID:          orgID,
		SenderID:       userID,
		Content:        "find me by any conv",
	}
	if err := msgStore.Create(ctx, msg); err != nil {
		t.Fatalf("create message: %v", err)
	}

	got, err := msgStore.GetByIDAnyConv(ctx, "msg-1")
	if err != nil {
		t.Fatalf("get message any conv: %v", err)
	}
	if got.Content != "find me by any conv" {
		t.Errorf("expected 'find me by any conv', got '%s'", got.Content)
	}
}

func TestMessageStore_UpdateMessage(t *testing.T) {
	ctx, _, msgStore, _, orgID, userID := setupMessageStoreTest(t)

	msg := &domain.Message{
		ID:             "msg-1",
		ConversationID: "conv-1",
		OrgID:          orgID,
		SenderID:       userID,
		Content:        "original content",
	}
	if err := msgStore.Create(ctx, msg); err != nil {
		t.Fatalf("create message: %v", err)
	}

	msg.Content = "updated content"
	if err := msgStore.Update(ctx, msg); err != nil {
		t.Fatalf("update message: %v", err)
	}

	got, err := msgStore.GetByID(ctx, "msg-1", "conv-1")
	if err != nil {
		t.Fatalf("get after update: %v", err)
	}
	if got.Content != "updated content" {
		t.Errorf("expected 'updated content', got '%s'", got.Content)
	}
	if got.EditedAt == nil {
		t.Fatal("expected edited_at to be set after update")
	}
}

func TestMessageStore_SoftDelete(t *testing.T) {
	ctx, _, msgStore, _, orgID, userID := setupMessageStoreTest(t)

	msg := &domain.Message{
		ID:             "msg-1",
		ConversationID: "conv-1",
		OrgID:          orgID,
		SenderID:       userID,
		Content:        "delete me",
	}
	if err := msgStore.Create(ctx, msg); err != nil {
		t.Fatalf("create message: %v", err)
	}

	if err := msgStore.SoftDelete(ctx, "msg-1", "conv-1"); err != nil {
		t.Fatalf("soft delete: %v", err)
	}

	_, err := msgStore.GetByID(ctx, "msg-1", "conv-1")
	if err == nil {
		t.Fatal("expected error getting deleted message")
	}

	result, err := msgStore.ListByConversation(ctx, orgID, "conv-1", domain.MessageFilter{Limit: 50})
	if err != nil {
		t.Fatalf("list messages: %v", err)
	}
	if len(result.Items) != 0 {
		t.Errorf("expected 0 messages after delete, got %d", len(result.Items))
	}
}

func TestMessageStore_PinAndUnpin(t *testing.T) {
	ctx, _, msgStore, _, orgID, userID := setupMessageStoreTest(t)

	msg := &domain.Message{
		ID:             "msg-1",
		ConversationID: "conv-1",
		OrgID:          orgID,
		SenderID:       userID,
		Content:        "pin me",
	}
	if err := msgStore.Create(ctx, msg); err != nil {
		t.Fatalf("create message: %v", err)
	}

	if err := msgStore.Pin(ctx, "msg-1", "conv-1", userID); err != nil {
		t.Fatalf("pin message: %v", err)
	}

	got, err := msgStore.GetByID(ctx, "msg-1", "conv-1")
	if err != nil {
		t.Fatalf("get after pin: %v", err)
	}
	if !got.Pinned {
		t.Fatal("expected message to be pinned")
	}
	if got.PinnedBy == nil || *got.PinnedBy != userID {
		t.Errorf("expected pinned_by %s, got %v", userID, got.PinnedBy)
	}
	if got.PinnedAt == nil {
		t.Fatal("expected pinned_at to be set")
	}

	if err := msgStore.Unpin(ctx, "msg-1", "conv-1"); err != nil {
		t.Fatalf("unpin message: %v", err)
	}

	got, err = msgStore.GetByID(ctx, "msg-1", "conv-1")
	if err != nil {
		t.Fatalf("get after unpin: %v", err)
	}
	if got.Pinned {
		t.Fatal("expected message to be unpinned")
	}
}

func TestMessageStore_ListMessages_Pagination(t *testing.T) {
	ctx, conn, msgStore, _, orgID, userID := setupMessageStoreTest(t)

	for i := 0; i < 5; i++ {
		insertMessage(t, conn, "msg-"+runeStr(i), "conv-1", orgID, userID, "msg "+strings.Repeat("x", i+1))
	}

	result, err := msgStore.ListByConversation(ctx, orgID, "conv-1", domain.MessageFilter{Limit: 3})
	if err != nil {
		t.Fatalf("list messages: %v", err)
	}
	if len(result.Items) > 3 {
		t.Errorf("expected at most 3 messages, got %d", len(result.Items))
	}
	if !result.HasMore {
		t.Error("expected hasMore=true with 5 messages and limit 3")
	}
	if result.NextCursor == "" {
		t.Error("expected non-empty next cursor")
	}

	// Paginate with cursor
	result2, err := msgStore.ListByConversation(ctx, orgID, "conv-1", domain.MessageFilter{Before: result.NextCursor, Limit: 3})
	if err != nil {
		t.Fatalf("list messages page 2: %v", err)
	}
	if len(result2.Items) == 0 {
		t.Fatal("expected messages in page 2")
	}
}

func TestMessageStore_ListReplies(t *testing.T) {
	ctx, conn, msgStore, _, orgID, userID := setupMessageStoreTest(t)

	insertMessage(t, conn, "parent-1", "conv-1", orgID, userID, "parent message")
	insertMessage(t, conn, "reply-1", "conv-1", orgID, userID, "first reply")
	insertMessage(t, conn, "reply-2", "conv-1", orgID, userID, "second reply")

	// Set parent_id on replies
	_, err := conn.Exec(`UPDATE messages SET parent_id = 'parent-1' WHERE id = 'reply-1'`)
	if err != nil {
		t.Fatalf("set parent on reply-1: %v", err)
	}
	_, err = conn.Exec(`UPDATE messages SET parent_id = 'parent-1' WHERE id = 'reply-2'`)
	if err != nil {
		t.Fatalf("set parent on reply-2: %v", err)
	}

	result, err := msgStore.ListReplies(ctx, orgID, "conv-1", "parent-1", domain.MessageFilter{Limit: 50})
	if err != nil {
		t.Fatalf("list replies: %v", err)
	}
	if len(result.Items) != 2 {
		t.Fatalf("expected 2 replies, got %d", len(result.Items))
	}
	// ListReplies returns replies in chronological order (oldest first),
	// consistent with ListMessages.
	if result.Items[0].ID != "reply-1" {
		t.Errorf("expected first reply to be reply-1, got %s", result.Items[0].ID)
	}
	if result.Items[1].ID != "reply-2" {
		t.Errorf("expected second reply to be reply-2, got %s", result.Items[1].ID)
	}
}

func TestMessageStore_ListReplies_Pagination(t *testing.T) {
	ctx, conn, msgStore, _, orgID, userID := setupMessageStoreTest(t)

	insertMessage(t, conn, "parent-1", "conv-1", orgID, userID, "parent message")

	// Insert 4 replies with explicit, increasing created_at so the cursor
	// ordering is deterministic regardless of insertion speed.
	replies := []struct {
		id      string
		content string
		ts      string
	}{
		{"reply-1", "first", "2026-01-01 10:00:00"},
		{"reply-2", "second", "2026-01-01 10:00:01"},
		{"reply-3", "third", "2026-01-01 10:00:02"},
		{"reply-4", "fourth", "2026-01-01 10:00:03"},
	}
	for _, r := range replies {
		_, err := conn.Exec(
			`INSERT INTO messages (id, conversation_id, org_id, sender_id, content, search_content, parent_id, created_at) VALUES (?, 'conv-1', ?, ?, ?, ?, 'parent-1', ?)`,
			r.id, orgID, userID, r.content, domain.StripMentionTokens(r.content), r.ts,
		)
		if err != nil {
			t.Fatalf("insert %s: %v", r.id, err)
		}
	}

	// Page 1 (limit 2): newest two replies, in chronological order within the page.
	p1, err := msgStore.ListReplies(ctx, orgID, "conv-1", "parent-1", domain.MessageFilter{Limit: 2})
	if err != nil {
		t.Fatalf("list page 1: %v", err)
	}
	if len(p1.Items) != 2 || p1.Items[0].ID != "reply-3" || p1.Items[1].ID != "reply-4" {
		t.Fatalf("page 1 = %v, want [reply-3, reply-4]", ids(p1.Items))
	}
	if !p1.HasMore {
		t.Fatal("expected HasMore=true after page 1")
	}
	if p1.NextCursor == "" {
		t.Fatal("expected non-empty NextCursor after page 1")
	}

	// Page 2 (before=p1.NextCursor): the next older two replies.
	// This is where the reversed ASC cursor used to return the wrong page.
	p2, err := msgStore.ListReplies(ctx, orgID, "conv-1", "parent-1", domain.MessageFilter{Limit: 2, Before: p1.NextCursor})
	if err != nil {
		t.Fatalf("list page 2: %v", err)
	}
	if len(p2.Items) != 2 || p2.Items[0].ID != "reply-1" || p2.Items[1].ID != "reply-2" {
		t.Fatalf("page 2 = %v, want [reply-1, reply-2]", ids(p2.Items))
	}
	if p2.HasMore {
		t.Fatal("expected HasMore=false after page 2")
	}
}

func ids(msgs []*domain.Message) []string {
	out := make([]string, len(msgs))
	for i, m := range msgs {
		out[i] = m.ID
	}
	return out
}

func TestMessageStore_Reactions(t *testing.T) {
	ctx, conn, _, reactionStore, orgID, userID := setupMessageStoreTest(t)

	// Create user-2 for reaction FK
	_, err := conn.Exec(`INSERT INTO accounts (id, email, password_hash) VALUES ('acct-2', 'bob@test.com', 'hash')`)
	if err != nil {
		t.Fatalf("insert account: %v", err)
	}
	_, err = conn.Exec(`INSERT INTO users (id, account_id, org_id, name, email) VALUES ('user-2', 'acct-2', ?, 'Bob', 'bob@test.com')`, orgID)
	if err != nil {
		t.Fatalf("insert user-2: %v", err)
	}

	// Create a message to react to
	insertMessage(t, conn, "msg-1", "conv-1", orgID, userID, "react to me")

	// Add reaction
	if err := reactionStore.Add(ctx, orgID, "msg-1", userID, "🎉"); err != nil {
		t.Fatalf("add reaction: %v", err)
	}
	if err := reactionStore.Add(ctx, orgID, "msg-1", "user-2", "🎉"); err != nil {
		t.Fatalf("add user-2 reaction: %v", err)
	}
	if err := reactionStore.Add(ctx, orgID, "msg-1", userID, "👍"); err != nil {
		t.Fatalf("add thumbs up: %v", err)
	}

	reactions, err := reactionStore.ListForMessages(ctx, []string{"msg-1"})
	if err != nil {
		t.Fatalf("list reactions: %v", err)
	}
	if len(reactions) != 3 {
		t.Fatalf("expected 3 reactions, got %d", len(reactions))
	}

	removed, err := reactionStore.Remove(ctx, "msg-1", userID, "🎉")
	if err != nil {
		t.Fatalf("remove reaction: %v", err)
	}
	if !removed {
		t.Fatal("expected Remove to report the reaction was removed")
	}

	if again, err := reactionStore.Remove(ctx, "msg-1", userID, "🎉"); err != nil || again {
		t.Fatalf("second remove: removed=%v err=%v, want false/nil (idempotent)", again, err)
	}

	reactions, err = reactionStore.ListForMessages(ctx, []string{"msg-1"})
	if err != nil {
		t.Fatalf("list reactions after remove: %v", err)
	}
	if len(reactions) != 2 {
		t.Fatalf("expected 2 reactions after remove, got %d", len(reactions))
	}
}

func TestMessageStore_Reactions_DuplicateIdempotent(t *testing.T) {
	ctx, conn, _, reactionStore, orgID, userID := setupMessageStoreTest(t)

	insertMessage(t, conn, "msg-1", "conv-1", orgID, userID, "react to me")

	if err := reactionStore.Add(ctx, orgID, "msg-1", userID, "🎉"); err != nil {
		t.Fatalf("first add: %v", err)
	}
	if err := reactionStore.Add(ctx, orgID, "msg-1", userID, "🎉"); err != nil {
		t.Fatalf("duplicate add should be idempotent: %v", err)
	}

	reactions, err := reactionStore.ListForMessages(ctx, []string{"msg-1"})
	if err != nil {
		t.Fatalf("list reactions: %v", err)
	}
	if len(reactions) != 1 {
		t.Fatalf("expected 1 reaction (duplicate was idempotent), got %d", len(reactions))
	}
}

func TestMessageStore_Reactions_RemoveNonExistent(t *testing.T) {
	ctx, conn, _, reactionStore, orgID, userID := setupMessageStoreTest(t)

	insertMessage(t, conn, "msg-1", "conv-1", orgID, userID, "react to me")

	removed, err := reactionStore.Remove(ctx, "msg-1", userID, "🎉")
	if err != nil {
		t.Fatalf("remove non-existent reaction should not error: %v", err)
	}
	if removed {
		t.Fatal("removing a non-existent reaction should report removed=false")
	}
}

func TestMessageStore_ListReactions_Empty(t *testing.T) {
	ctx, _, _, reactionStore, _, _ := setupMessageStoreTest(t)

	reactions, err := reactionStore.ListForMessages(ctx, []string{})
	if err != nil {
		t.Fatalf("list reactions empty: %v", err)
	}
	if len(reactions) != 0 {
		t.Errorf("expected 0 reactions for empty input, got %d", len(reactions))
	}
}

func TestMessageStore_CountMessages(t *testing.T) {
	ctx, conn, msgStore, _, orgID, userID := setupMessageStoreTest(t)

	insertMessage(t, conn, "msg-1", "conv-1", orgID, userID, "msg 1")
	insertMessage(t, conn, "msg-2", "conv-1", orgID, userID, "msg 2")

	count, err := msgStore.Count(ctx, "conv-1")
	if err != nil {
		t.Fatalf("count: %v", err)
	}
	if count != 2 {
		t.Errorf("expected 2, got %d", count)
	}

	// Delete one and count again
	if err := msgStore.SoftDelete(ctx, "msg-1", "conv-1"); err != nil {
		t.Fatalf("soft delete: %v", err)
	}
	count, err = msgStore.Count(ctx, "conv-1")
	if err != nil {
		t.Fatalf("count after delete: %v", err)
	}
	if count != 1 {
		t.Errorf("expected 1 after delete, got %d", count)
	}
}

func TestMessageStore_GetConversationLastMessage(t *testing.T) {
	ctx, conn, msgStore, _, orgID, userID := setupMessageStoreTest(t)

	insertMessage(t, conn, "msg-1", "conv-1", orgID, userID, "older message")
	insertMessage(t, conn, "msg-2", "conv-1", orgID, userID, "newer message")

	last, err := msgStore.GetConversationLastMessage(ctx, "conv-1")
	if err != nil {
		t.Fatalf("get last message: %v", err)
	}
	if last.Content != "newer message" {
		t.Errorf("expected 'newer message', got '%s'", last.Content)
	}
}

func TestMessageStore_EditTriggersFTSUpdate(t *testing.T) {
	ctx, conn, msgStore, _, orgID, userID := setupMessageStoreTest(t)

	insertMessage(t, conn, "msg-1", "conv-1", orgID, userID, "original content")

	msg, err := msgStore.GetByID(ctx, "msg-1", "conv-1")
	if err != nil {
		t.Fatalf("get message: %v", err)
	}

	msg.Content = "updated content for search"
	if err := msgStore.Update(ctx, msg); err != nil {
		t.Fatalf("update message: %v", err)
	}

	// Search for the new content
	searchMsg, err := msgStore.SearchMessages(ctx, orgID, userID, domain.MessageSearchFilter{
		Query: "updated",
		Scope: domain.MessageSearchScopeAll,
		Limit: 20,
	})
	if err != nil {
		t.Fatalf("search after edit: %v", err)
	}
	found := false
	for _, item := range searchMsg.Items {
		if item.Message.Content == "updated content for search" {
			found = true
			break
		}
	}
	if !found {
		t.Fatal("expected FTS to find message after content update")
	}

	// Search for old content should NOT find it
	searchOld, err := msgStore.SearchMessages(ctx, orgID, userID, domain.MessageSearchFilter{
		Query: "original",
		Scope: domain.MessageSearchScopeAll,
		Limit: 20,
	})
	if err != nil {
		t.Fatalf("search for old content: %v", err)
	}
	for _, item := range searchOld.Items {
		if item.Message.Content == "original content" {
			t.Fatal("FTS should not find old content after update")
		}
	}
}

func TestMessageStore_SoftDeleteClearsFTS(t *testing.T) {
	ctx, conn, msgStore, _, orgID, userID := setupMessageStoreTest(t)

	insertMessage(t, conn, "msg-1", "conv-1", orgID, userID, "this will be deleted")

	// Soft delete
	if err := msgStore.SoftDelete(ctx, "msg-1", "conv-1"); err != nil {
		t.Fatalf("soft delete: %v", err)
	}

	// Search should not find it (deleted_at IS NULL filter)
	searchMsg, err := msgStore.SearchMessages(ctx, orgID, userID, domain.MessageSearchFilter{
		Query: "deleted",
		Scope: domain.MessageSearchScopeAll,
		Limit: 20,
	})
	if err != nil {
		t.Fatalf("search after delete: %v", err)
	}
	if len(searchMsg.Items) != 0 {
		t.Errorf("expected 0 search results after delete, got %d", len(searchMsg.Items))
	}
}

// TestMessageStore_SearchMessages_FTS5EndToEnd exercises FTS5 search with
// various query types against a real SQLite database.
func TestMessageStore_SearchMessages_FTS5EndToEnd(t *testing.T) {
	ctx, conn, msgStore, _, orgID, userID := setupMessageStoreTest(t)

	msgs := []string{
		"Hello everyone, welcome to the workspace!",
		"Check the API docs for the latest changes.",
		"We need more tests for the search feature.",
	}
	for i, content := range msgs {
		msgID := "msg-" + runeStr(i)
		insertMessage(t, conn, msgID, "conv-1", orgID, userID, content)
	}

	t.Run("basic_search", func(t *testing.T) {
		result, err := msgStore.SearchMessages(ctx, orgID, userID, domain.MessageSearchFilter{
			Query: "hello",
			Scope: domain.MessageSearchScopeAll,
			Limit: 20,
		})
		if err != nil {
			t.Fatalf("search messages: %v", err)
		}
		if len(result.Items) == 0 {
			t.Fatal("expected at least one result for 'hello'")
		}
		found := false
		for _, item := range result.Items {
			if item.Message.Content == "Hello everyone, welcome to the workspace!" {
				found = true
				if item.Snippet == "" {
					t.Fatal("expected non-empty snippet for matching message")
				}
				break
			}
		}
		if !found {
			t.Fatalf("expected to find hello message, got %+v", result.Items)
		}
	})

	t.Run("substring_match", func(t *testing.T) {
		result, err := msgStore.SearchMessages(ctx, orgID, userID, domain.MessageSearchFilter{
			Query: "sear",
			Scope: domain.MessageSearchScopeAll,
			Limit: 20,
		})
		if err != nil {
			t.Fatalf("search messages: %v", err)
		}
		found := false
		for _, item := range result.Items {
			if item.Message.Content == "We need more tests for the search feature." {
				found = true
				break
			}
		}
		if !found {
			t.Fatalf("expected substring 'sear' to match 'search', got %+v", result.Items)
		}
	})

	t.Run("no_results", func(t *testing.T) {
		result, err := msgStore.SearchMessages(ctx, orgID, userID, domain.MessageSearchFilter{
			Query: "zzzznonexistent",
			Scope: domain.MessageSearchScopeAll,
			Limit: 20,
		})
		if err != nil {
			t.Fatalf("search messages: %v", err)
		}
		if len(result.Items) != 0 {
			t.Fatalf("expected 0 results, got %d", len(result.Items))
		}
	})

	t.Run("scope_workspace", func(t *testing.T) {
		result, err := msgStore.SearchMessages(ctx, orgID, userID, domain.MessageSearchFilter{
			Query: "hello",
			Scope: domain.MessageSearchScopeWorkspace,
			Limit: 20,
		})
		if err != nil {
			t.Fatalf("search messages: %v", err)
		}
		if len(result.Items) == 0 {
			t.Fatal("expected workspace scope to find hello message")
		}
	})

	t.Run("attachment_name_search", func(t *testing.T) {
		msgID := "msg-att"
		insertMessage(t, conn, msgID, "conv-1", orgID, userID, "check this out")
		_, err := conn.Exec(`UPDATE messages_fts SET attachment_names = 'design_mockup_v2.pdf' WHERE message_id = ?`, msgID)
		if err != nil {
			t.Fatalf("update fts attachment_names: %v", err)
		}

		result, err := msgStore.SearchMessages(ctx, orgID, userID, domain.MessageSearchFilter{
			Query: "mockup",
			Scope: domain.MessageSearchScopeAll,
			Limit: 20,
		})
		if err != nil {
			t.Fatalf("search messages: %v", err)
		}
		found := false
		for _, item := range result.Items {
			if item.Message.ID == msgID {
				found = true
				break
			}
		}
		if !found {
			t.Fatalf("expected to find message by attachment name 'mockup', got %+v", result.Items)
		}
	})
}

func runeStr(i int) string {
	return string(rune('a' + i))
}

func TestMessageStore_Search_ProjectLinkedChannel(t *testing.T) {
	ctx := context.Background()
	tmpDir := t.TempDir()
	conn, err := NewDB(tmpDir + "/test.db")
	if err != nil {
		t.Fatalf("NewDB: %v", err)
	}
	t.Cleanup(func() { conn.Close() })

	if err := migration.RunMigrations(conn); err != nil {
		t.Fatalf("RunMigrations: %v", err)
	}

	q := sqlc.New(conn)
	msgStore := NewMessageStore(q)

	orgID := "org-1"
	userID := "user-1"
	acctID := "acct-1"
	projID := "proj-1"
	chID := "ch-search-pl"

	// Seed org
	if _, err := conn.Exec(`INSERT INTO organizations (id, name, slug) VALUES (?, 'Test Org', 'test-org')`, orgID); err != nil {
		t.Fatalf("insert org: %v", err)
	}
	// Seed account
	if _, err := conn.Exec(`INSERT INTO accounts (id, email, password_hash) VALUES (?, 'alice@test.com', 'hash')`, acctID); err != nil {
		t.Fatalf("insert account: %v", err)
	}
	// Seed user
	if _, err := conn.Exec(`INSERT INTO users (id, account_id, org_id, email, name, role, is_active) VALUES (?, ?, ?, 'alice@test.com', 'Alice', 'member', 1)`, userID, acctID, orgID); err != nil {
		t.Fatalf("insert user: %v", err)
	}
	// Seed project
	if _, err := conn.Exec(`INSERT INTO projects (id, org_id, name, slug, description, color, icon, created_by) VALUES (?, ?, 'Test Project', 'test-project', '', '#000', '', ?)`, projID, orgID, userID); err != nil {
		t.Fatalf("insert project: %v", err)
	}
	// Seed project-linked channel (NO conversation_members row for user-1)
	if _, err := conn.Exec(`INSERT INTO conversations (id, org_id, name, type, created_by, position_key) VALUES (?, ?, 'Project-Linked Chat', 'channel', ?, 'a')`, chID, orgID, userID); err != nil {
		t.Fatalf("insert project-linked channel: %v", err)
	}
	if _, err := conn.Exec(`INSERT INTO channel_project_links (channel_id, project_id) VALUES (?, ?)`, chID, projID); err != nil {
		t.Fatalf("insert channel_project_link: %v", err)
	}

	// Seed a message in the project-linked channel
	if _, err := conn.Exec(
		`INSERT INTO messages (id, conversation_id, org_id, sender_id, content, search_content, created_at) VALUES (?, ?, ?, ?, ?, ?, datetime('now'))`,
		"msg-pl", chID, orgID, userID, "This message is in a project-linked channel",
		"This message is in a project-linked channel",
	); err != nil {
		t.Fatalf("insert message in project-linked channel: %v", err)
	}

	t.Run("find_message_with_project_link", func(t *testing.T) {
		result, err := msgStore.SearchMessages(ctx, orgID, userID, domain.MessageSearchFilter{
			Query:                "channel",
			Scope:                domain.MessageSearchScopeAll,
			Limit:                20,
			IncludeProjectLinked: true,
		})
		if err != nil {
			t.Fatalf("SearchMessages: %v", err)
		}
		found := false
		for _, item := range result.Items {
			if item.Message.ID == "msg-pl" {
				found = true
				break
			}
		}
		if !found {
			t.Errorf("expected to find message in project-linked channel with IncludeProjectLinked=true, got %d results", len(result.Items))
		}
	})

	t.Run("hide_message_without_project_link", func(t *testing.T) {
		result, err := msgStore.SearchMessages(ctx, orgID, userID, domain.MessageSearchFilter{
			Query:                "channel",
			Scope:                domain.MessageSearchScopeAll,
			Limit:                20,
			IncludeProjectLinked: false,
		})
		if err != nil {
			t.Fatalf("SearchMessages: %v", err)
		}
		for _, item := range result.Items {
			if item.Message.ID == "msg-pl" {
				t.Errorf("should NOT find message in project-linked channel with IncludeProjectLinked=false")
			}
		}
	})
}

func TestMessageStore_GetLastMessagesForConversations_TieBreaksByID(t *testing.T) {
	ctx, conn, msgStore, _, orgID, userID := setupMessageStoreTest(t)

	_, err := conn.Exec(
		`INSERT INTO messages (id, conversation_id, org_id, sender_id, content, search_content, created_at) VALUES (?, ?, ?, ?, ?, ?, '2024-01-01 12:00:00')`,
		"msg-a", "conv-1", orgID, userID, "older", "older",
	)
	if err != nil {
		t.Fatalf("insert msg-a: %v", err)
	}
	_, err = conn.Exec(
		`INSERT INTO messages (id, conversation_id, org_id, sender_id, content, search_content, created_at) VALUES (?, ?, ?, ?, ?, ?, '2024-01-01 12:00:00')`,
		"msg-b", "conv-1", orgID, userID, "newer", "newer",
	)
	if err != nil {
		t.Fatalf("insert msg-b: %v", err)
	}

	result, err := msgStore.GetLastMessagesForConversations(ctx, []string{"conv-1"})
	if err != nil {
		t.Fatalf("GetLastMessagesForConversations: %v", err)
	}

	got, ok := result["conv-1"]
	if !ok {
		t.Fatal("expected conv-1 in result")
	}
	if got.ID != "msg-b" {
		t.Errorf("expected msg-b (higher id) as tie-break winner, got %s", got.ID)
	}
	if got.Content != "newer" {
		t.Errorf("expected content 'newer', got %s", got.Content)
	}
}

func TestMessageStore_SearchMessages_HasLinkFilter(t *testing.T) {
	ctx, conn, msgStore, _, orgID, userID := setupMessageStoreTest(t)

	insertMessage(t, conn, "msg-http", "conv-1", orgID, userID, "test http://example.com/page")
	insertMessage(t, conn, "msg-https", "conv-1", orgID, userID, "test https://secure.com")
	insertMessage(t, conn, "msg-httpping", "conv-1", orgID, userID, "test httpping spam")
	insertMessage(t, conn, "msg-async", "conv-1", orgID, userID, "test async http client")

	t.Run("has_link_true_finds_http_and_https", func(t *testing.T) {
		result, err := msgStore.SearchMessages(ctx, orgID, userID, domain.MessageSearchFilter{
			Query:   "test",
			Scope:   domain.MessageSearchScopeAll,
			Limit:   20,
			HasLink: true,
		})
		if err != nil {
			t.Fatalf("SearchMessages: %v", err)
		}

		foundHTTP := false
		foundHTTPS := false
		foundHttpping := false
		foundAsyncHTTP := false
		for _, item := range result.Items {
			mid := item.Message.ID
			if mid == "msg-http" {
				foundHTTP = true
			}
			if mid == "msg-https" {
				foundHTTPS = true
			}
			if mid == "msg-httpping" {
				foundHttpping = true
			}
			if mid == "msg-async" {
				foundAsyncHTTP = true
			}
		}

		if !foundHTTP {
			t.Errorf("has_link=true should match 'http://example.com'")
		}
		if !foundHTTPS {
			t.Errorf("has_link=true should match 'https://secure.com'")
		}
		if foundHttpping {
			t.Errorf("has_link=true should NOT match 'httpping' (no URL scheme)")
		}
		if foundAsyncHTTP {
			t.Errorf("has_link=true should NOT match 'async http' (no URL scheme)")
		}
	})

	t.Run("has_link_false_returns_all", func(t *testing.T) {
		result, err := msgStore.SearchMessages(ctx, orgID, userID, domain.MessageSearchFilter{
			Query:   "test",
			Scope:   domain.MessageSearchScopeAll,
			Limit:   20,
			HasLink: false,
		})
		if err != nil {
			t.Fatalf("SearchMessages: %v", err)
		}
		ids := make(map[string]bool)
		for _, item := range result.Items {
			ids[item.Message.ID] = true
		}
		if !ids["msg-http"] || !ids["msg-https"] || !ids["msg-httpping"] || !ids["msg-async"] {
			t.Errorf("has_link=false should return all messages; got %v", ids)
		}
	})
}
