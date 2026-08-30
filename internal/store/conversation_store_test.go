package store

import (
	"context"
	"database/sql"
	"testing"

	"ipmanlk/breeze/internal/domain"
	"ipmanlk/breeze/internal/store/migration"
	"ipmanlk/breeze/internal/store/sqlc"

	_ "modernc.org/sqlite"
)

// setupConversationStoreTest creates a fresh in-memory SQLite DB with migrations,
// seeds minimal org + account + two users (owner/guest role) + a project, and returns
// the dependencies needed for conversation listing/search tests.
func setupConversationStoreTest(t *testing.T) (context.Context, *sql.DB, *ConversationStore, string, string, string, string) {
	t.Helper()
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

	orgID := "org-1"
	ownerUserID := "user-owner"
	guestUserID := "user-guest"
	acctID := "acct-1"
	projID := "proj-1"

	// Seed org
	if _, err := conn.Exec(`INSERT INTO organizations (id, name, slug) VALUES (?, 'Test Org', 'test-org')`, orgID); err != nil {
		t.Fatalf("insert org: %v", err)
	}

	// Seed account
	if _, err := conn.Exec(`INSERT INTO accounts (id, email, password_hash) VALUES (?, 'admin@test.com', 'hash')`, acctID); err != nil {
		t.Fatalf("insert account: %v", err)
	}

	// Seed owner user (org owner)
	if _, err := conn.Exec(`INSERT INTO users (id, account_id, org_id, email, name, role, is_active) VALUES (?, ?, ?, 'owner@test.com', 'Owner', 'owner', 1)`, ownerUserID, acctID, orgID); err != nil {
		t.Fatalf("insert owner user: %v", err)
	}

	// Seed guest user (org guest)
	if _, err := conn.Exec(`INSERT INTO users (id, account_id, org_id, email, name, role, is_active) VALUES (?, ?, ?, 'guest@test.com', 'Guest', 'guest', 1)`, guestUserID, acctID, orgID); err != nil {
		t.Fatalf("insert guest user: %v", err)
	}

	// Seed project
	if _, err := conn.Exec(`INSERT INTO projects (id, org_id, name, slug, description, color, icon, created_by) VALUES (?, ?, 'Test Project', 'test-project', '', '#000', '', ?)`, projID, orgID, ownerUserID); err != nil {
		t.Fatalf("insert project: %v", err)
	}

	convStore := NewConversationStore(q, conn)
	return ctx, conn, convStore, orgID, ownerUserID, guestUserID, projID
}

// createProjectLinkedConv seeds a category + child channel and links the channel to a project.
func createProjectLinkedConv(t *testing.T, conn *sql.DB, orgID, projID, ownerUserID string) (catID, chID string) {
	t.Helper()
	catID = "cat-1"
	chID = "ch-pl-1"

	// Category (parent)
	if _, err := conn.Exec(`INSERT INTO conversations (id, org_id, name, type, created_by, position_key) VALUES (?, ?, 'Category', 'category', ?, 'a')`, catID, orgID, ownerUserID); err != nil {
		t.Fatalf("insert category conversation: %v", err)
	}
	// Channel (child of category)
	if _, err := conn.Exec(`INSERT INTO conversations (id, org_id, parent_id, name, type, created_by, position_key) VALUES (?, ?, ?, 'Project-Linked Channel', 'channel', ?, 'b')`, chID, orgID, catID, ownerUserID); err != nil {
		t.Fatalf("insert channel conversation: %v", err)
	}
	// Project link: channel → project
	if _, err := conn.Exec(`INSERT INTO channel_project_links (channel_id, project_id) VALUES (?, ?)`, chID, projID); err != nil {
		t.Fatalf("insert channel_project_link: %v", err)
	}
	return catID, chID
}

// createOrphanConv seeds a channel with NO project link and NO explicit member (should NEVER appear).
func createOrphanConv(t *testing.T, conn *sql.DB, orgID, ownerUserID string) string {
	t.Helper()
	id := "ch-orphan"
	if _, err := conn.Exec(`INSERT INTO conversations (id, org_id, name, type, created_by, position_key) VALUES (?, ?, 'Orphan Channel', 'channel', ?, 'z')`, id, orgID, ownerUserID); err != nil {
		t.Fatalf("insert orphan conversation: %v", err)
	}
	return id
}

func TestConversationStore_ListByUser_ProjectLinkedChannel(t *testing.T) {
	ctx, conn, convStore, orgID, ownerUserID, guestUserID, projID := setupConversationStoreTest(t)

	// Seed: one project-linked channel (via category parent → channel → project_link),
	// one orphan channel (no link, no member).
	_, chID := createProjectLinkedConv(t, conn, orgID, projID, ownerUserID)
	orphanID := createOrphanConv(t, conn, orgID, ownerUserID)

	t.Run("owner_sees_project_linked_channel", func(t *testing.T) {
		// Owner has no explicit conversation_members row, but the channel has a project link.
		// includeProjectLinked=true should surface it.
		filter := domain.ConversationFilter{
			Scope:                ptr("workspace"),
			Limit:                50,
			IncludeProjectLinked: true,
		}
		result, err := convStore.ListByUser(ctx, orgID, ownerUserID, filter)
		if err != nil {
			t.Fatalf("ListByUser: %v", err)
		}
		if result == nil {
			t.Fatal("expected non-nil result")
		}

		found := false
		for _, c := range result.Items {
			if c.ID == chID {
				found = true
				break
			}
		}
		if !found {
			t.Errorf("owner should see project-linked channel %q (includeProjectLinked=true), but it was not returned", chID)
		}

		// Orphan channel should NOT be visible (no membership, no link).
		for _, c := range result.Items {
			if c.ID == orphanID {
				t.Errorf("orphan channel %q should NOT be visible to owner (no membership, no link)", orphanID)
			}
		}
	})

	t.Run("owner_does_not_see_channel_without_includeflag", func(t *testing.T) {
		filter := domain.ConversationFilter{
			Scope:                ptr("workspace"),
			Limit:                50,
			IncludeProjectLinked: false,
		}
		result, err := convStore.ListByUser(ctx, orgID, ownerUserID, filter)
		if err != nil {
			t.Fatalf("ListByUser: %v", err)
		}
		for _, c := range result.Items {
			if c.ID == chID {
				t.Errorf("owner should NOT see project-linked channel when IncludeProjectLinked=false")
			}
		}
	})

	t.Run("guest_does_not_see_project_linked_channel", func(t *testing.T) {
		// Guest role: handler passes includeProjectLinked=false.
		// Without explicit membership and without the flag, the channel must not appear.
		filter := domain.ConversationFilter{
			Scope:                ptr("workspace"),
			Limit:                50,
			IncludeProjectLinked: false,
		}
		result, err := convStore.ListByUser(ctx, orgID, guestUserID, filter)
		if err != nil {
			t.Fatalf("ListByUser: %v", err)
		}
		for _, c := range result.Items {
			if c.ID == chID {
				t.Errorf("guest should NOT see project-linked channel (includeProjectLinked=false, no membership)")
			}
		}
	})
}

func TestConversationStore_ListByUser_AncestorProjectLink(t *testing.T) {
	// Test that a channel inherits access when its CATEGORY (parent) has the project link,
	// not the channel itself. This exercises the recursive CTE step.
	ctx, conn, convStore, orgID, ownerUserID, _, projID := setupConversationStoreTest(t)

	catID := "cat-link"
	chID := "ch-inherit"

	// Category has direct link to project
	if _, err := conn.Exec(`INSERT INTO conversations (id, org_id, name, type, created_by, position_key) VALUES (?, ?, 'Linked Category', 'category', ?, 'a')`, catID, orgID, ownerUserID); err != nil {
		t.Fatalf("insert category: %v", err)
	}
	// Channel child of the linked category: NO direct link
	if _, err := conn.Exec(`INSERT INTO conversations (id, org_id, parent_id, name, type, created_by, position_key) VALUES (?, ?, ?, 'Child Channel', 'channel', ?, 'b')`, chID, orgID, catID, ownerUserID); err != nil {
		t.Fatalf("insert child channel: %v", err)
	}
	// Project link only on CATEGORY, not on channel
	if _, err := conn.Exec(`INSERT INTO channel_project_links (channel_id, project_id) VALUES (?, ?)`, catID, projID); err != nil {
		t.Fatalf("insert channel_project_link on category: %v", err)
	}

	filter := domain.ConversationFilter{
		Scope:                ptr("workspace"),
		Limit:                50,
		IncludeProjectLinked: true,
	}
	result, err := convStore.ListByUser(ctx, orgID, ownerUserID, filter)
	if err != nil {
		t.Fatalf("ListByUser: %v", err)
	}

	foundCat := false
	foundCh := false
	for _, c := range result.Items {
		if c.ID == catID {
			foundCat = true
		}
		if c.ID == chID {
			foundCh = true
		}
	}
	if !foundCat {
		t.Errorf("linked category %q should be visible (direct link)", catID)
	}
	if !foundCh {
		t.Errorf("child channel %q should be visible (inherited via recursive CTE from category)", chID)
	}
}

func TestConversationStore_ListByParent_ProjectLinkedChannel(t *testing.T) {
	ctx, conn, convStore, orgID, ownerUserID, _, projID := setupConversationStoreTest(t)

	catID, chID := createProjectLinkedConv(t, conn, orgID, projID, ownerUserID)

	t.Run("owner_sees_project_linked_channel_in_parent_listing", func(t *testing.T) {
		// ListByParent with includeProjectLinked=true
		convs, err := convStore.ListByParent(ctx, orgID, catID, ownerUserID, true)
		if err != nil {
			t.Fatalf("ListByParent: %v", err)
		}

		found := false
		for _, c := range convs {
			if c.ID == chID {
				found = true
				break
			}
		}
		if !found {
			t.Errorf("owner should see project-linked channel %q in ListByParent with includeProjectLinked=true", chID)
		}
	})

	t.Run("owner_does_not_see_without_flag", func(t *testing.T) {
		convs, err := convStore.ListByParent(ctx, orgID, catID, ownerUserID, false)
		if err != nil {
			t.Fatalf("ListByParent: %v", err)
		}
		for _, c := range convs {
			if c.ID == chID {
				t.Errorf("owner should NOT see project-linked channel when includeProjectLinked=false")
			}
		}
	})

	t.Run("guest_does_not_see_even_with_flag", func(t *testing.T) {
		// Guest role: handler passes includeProjectLinked=false.
		// Without explicit membership and without the flag, the channel must not appear.
		convs, err := convStore.ListByParent(ctx, orgID, catID, "user-guest", false)
		if err != nil {
			t.Fatalf("ListByParent: %v", err)
		}
		for _, c := range convs {
			if c.ID == chID {
				t.Errorf("guest should NOT see project-linked channel (includeProjectLinked=false, no membership)")
			}
		}
	})
}
