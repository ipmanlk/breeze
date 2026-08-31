package store

import (
	"context"
	"path/filepath"
	"testing"

	"ipmanlk/plume/internal/store/migration"
	"ipmanlk/plume/internal/store/sqlc"
)

// TestMigration_VoiceParticipants_ConnectionID verifies that the squashed
// baseline (00001_initial.sql) defines the voice_participants schema including
// connection_id, and that the generated sqlc code reads/writes it round-trip.
func TestMigration_VoiceParticipants_ConnectionID(t *testing.T) {
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

	// Seed minimal FK rows.
	_, err = conn.Exec(`INSERT INTO organizations (id, name, slug) VALUES ('org-1', 'Org', 'org')`)
	if err != nil {
		t.Fatalf("insert org: %v", err)
	}
	_, err = conn.Exec(`INSERT INTO accounts (id, email, password_hash) VALUES ('acct-1', 'a@x.com', 'h')`)
	if err != nil {
		t.Fatalf("insert account: %v", err)
	}
	_, err = conn.Exec(`INSERT INTO users (id, account_id, org_id, name, email) VALUES ('u1', 'acct-1', 'org-1', 'A', 'a@x.com')`)
	if err != nil {
		t.Fatalf("insert user: %v", err)
	}
	_, err = conn.Exec(`INSERT INTO conversations (id, org_id, name, type, created_by) VALUES ('c1', 'org-1', 'Voice', 'voice', 'u1')`)
	if err != nil {
		t.Fatalf("insert conversation: %v", err)
	}

	q := sqlc.New(conn)

	// Join with a connection_id.
	err = q.JoinVoiceChannel(ctx, sqlc.JoinVoiceChannelParams{
		ID:             "vp1",
		ConversationID: "c1",
		OrgID:          "org-1",
		UserID:         "u1",
		ConnectionID:   "conn-abc",
	})
	if err != nil {
		t.Fatalf("join: %v", err)
	}

	// Round-trip: the connection_id is read back.
	got, err := q.GetVoiceParticipant(ctx, sqlc.GetVoiceParticipantParams{
		ConversationID: "c1", UserID: "u1", OrgID: "org-1",
	})
	if err != nil {
		t.Fatalf("get: %v", err)
	}
	if got.ConnectionID != "conn-abc" {
		t.Errorf("connection_id round-trip: got %q want conn-abc", got.ConnectionID)
	}

	// Reassign the connection (takeover).
	if err := q.UpdateVoiceConnection(ctx, sqlc.UpdateVoiceConnectionParams{
		ConnectionID: "conn-xyz", ConversationID: "c1", UserID: "u1", OrgID: "org-1",
	}); err != nil {
		t.Fatalf("update connection: %v", err)
	}
	got, err = q.GetVoiceParticipant(ctx, sqlc.GetVoiceParticipantParams{
		ConversationID: "c1", UserID: "u1", OrgID: "org-1",
	})
	if err != nil {
		t.Fatalf("get after update: %v", err)
	}
	if got.ConnectionID != "conn-xyz" {
		t.Errorf("connection_id after update: got %q want conn-xyz", got.ConnectionID)
	}

	// Count works.
	count, err := q.CountVoiceParticipants(ctx, sqlc.CountVoiceParticipantsParams{
		ConversationID: "c1", OrgID: "org-1",
	})
	if err != nil {
		t.Fatalf("count: %v", err)
	}
	if count != 1 {
		t.Errorf("count: got %d want 1", count)
	}
}

// TestStore_VoiceParticipant_ListByConversationWithUser verifies the JOIN
// query returns participant rows with user name and avatar_url.
func TestStore_VoiceParticipant_ListByConversationWithUser(t *testing.T) {
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

	// Seed FK rows.
	_, err = conn.Exec(`INSERT INTO organizations (id, name, slug) VALUES ('org-1', 'Org', 'org')`)
	if err != nil {
		t.Fatalf("insert org: %v", err)
	}
	_, err = conn.Exec(`INSERT INTO accounts (id, email, password_hash) VALUES ('acct-1', 'a@x.com', 'h')`)
	if err != nil {
		t.Fatalf("insert account: %v", err)
	}
	_, err = conn.Exec(`INSERT INTO accounts (id, email, password_hash) VALUES ('acct-2', 'b@x.com', 'h')`)
	if err != nil {
		t.Fatalf("insert account: %v", err)
	}
	_, err = conn.Exec(`INSERT INTO users (id, account_id, org_id, name, email) VALUES ('u1', 'acct-1', 'org-1', 'Alice', 'a@x.com')`)
	if err != nil {
		t.Fatalf("insert user u1: %v", err)
	}
	_, err = conn.Exec(`INSERT INTO users (id, account_id, org_id, name, email, avatar_url) VALUES ('u2', 'acct-2', 'org-1', 'Bob', 'b@x.com', 'https://example.com/avatar.png')`)
	if err != nil {
		t.Fatalf("insert user u2: %v", err)
	}
	_, err = conn.Exec(`INSERT INTO conversations (id, org_id, name, type, created_by) VALUES ('c1', 'org-1', 'Voice', 'voice', 'u1')`)
	if err != nil {
		t.Fatalf("insert conversation: %v", err)
	}

	q := sqlc.New(conn)

	// Join two participants.
	for _, p := range []struct {
		id, userID, connID string
	}{
		{"vp1", "u1", "conn-a"},
		{"vp2", "u2", "conn-b"},
	} {
		if err := q.JoinVoiceChannel(ctx, sqlc.JoinVoiceChannelParams{
			ID:             p.id,
			ConversationID: "c1",
			OrgID:          "org-1",
			UserID:         p.userID,
			ConnectionID:   p.connID,
		}); err != nil {
			t.Fatalf("join %s: %v", p.userID, err)
		}
	}

	store := NewVoiceParticipantStore(q)
	infos, err := store.ListByConversationWithUser(ctx, "org-1", "c1")
	if err != nil {
		t.Fatalf("ListByConversationWithUser: %v", err)
	}

	if len(infos) != 2 {
		t.Fatalf("expected 2 participant infos, got %d", len(infos))
	}

	// Check u1 has name but no avatar.
	if infos[0].UserID == "u1" {
		if infos[0].Name != "Alice" {
			t.Errorf("expected Name='Alice' for u1, got %q", infos[0].Name)
		}
		if infos[0].AvatarURL != "" {
			t.Errorf("expected empty AvatarURL for u1, got %q", infos[0].AvatarURL)
		}
	}

	// Check u2 has name and avatar.
	for _, info := range infos {
		if info.UserID == "u2" {
			if info.Name != "Bob" {
				t.Errorf("expected Name='Bob' for u2, got %q", info.Name)
			}
			if info.AvatarURL != "https://example.com/avatar.png" {
				t.Errorf("expected AvatarURL for u2, got %q", info.AvatarURL)
			}
		}
	}
}
