package service

import (
	"context"
	"database/sql"
	"path/filepath"
	"testing"

	"ipmanlk/breeze/internal/domain"
	"ipmanlk/breeze/internal/store"
	"ipmanlk/breeze/internal/store/migration"
	"ipmanlk/breeze/internal/store/sqlc"

	_ "modernc.org/sqlite"
)

// ---------------------------------------------------------------------------
// hydrateMentions: unit tests with mocked repos
// ---------------------------------------------------------------------------

func TestHydrateMentions_PerMessageIsolation(t *testing.T) {
	ctx := context.Background()
	orgID := "org-1"

	userRepo := newMockUserRepo()
	projectRepo := newMockProjectRepo()
	taskRepo := newMockTaskRepo()
	convRepo := newMockConversationRepo()

	// Seed users
	alice := &domain.User{ID: "user-alice", Name: "Alice", OrgID: orgID}
	bob := &domain.User{ID: "user-bob", Name: "Bob", OrgID: orgID}
	_ = userRepo.Create(ctx, alice)
	_ = userRepo.Create(ctx, bob)

	// Seed projects
	projA := &domain.Project{ID: "proj-a", Name: "Project A", OrgID: orgID}
	projB := &domain.Project{ID: "proj-b", Name: "Project B", OrgID: orgID}
	_ = projectRepo.Create(ctx, projA)
	_ = projectRepo.Create(ctx, projB)

	// Seed tasks
	task1 := &domain.Task{ID: "task-1", Title: "Task one", ProjectID: "proj-a", OrgID: orgID}
	task2 := &domain.Task{ID: "task-2", Title: "Task two", ProjectID: "proj-b", OrgID: orgID}
	_ = taskRepo.Create(ctx, task1)
	_ = taskRepo.Create(ctx, task2)

	// Seed channels
	_ = convRepo.Create(ctx, &domain.Conversation{ID: "chan-1", OrgID: orgID, Name: "general", Type: domain.ConvChannel})
	_ = convRepo.Create(ctx, &domain.Conversation{ID: "chan-2", OrgID: orgID, Name: "random", Type: domain.ConvChannel})

	svc := NewMessageService(MessageServiceDeps{
		MsgRepo:        newMockMessageRepo(),
		ConvRepo:       convRepo,
		OrgRepo:        newMockOrgRepo(true),
		ProjectRepo:    projectRepo,
		TaskRepo:       taskRepo,
		UserRepo:       userRepo,
		AttRepo:        newMockMessageAttachmentRepo(),
		PendingAttRepo: newMockPendingAttachmentRepo(),
		ReactionRepo:   newMockReactionRepo(),
		PrefRepo:       newMockUserPrefRepo(),
		NotifSvc:       newMockNotificationService(),
		Broadcaster:    newMockBroadcaster(),
		UserPrefRepo:   nil,
		I18n:           nil,
	})

	messages := []*domain.Message{
		{
			ID:      "msg-1",
			OrgID:   orgID,
			Content: "Hey <@user:user-alice>, check <@task:task-1> in <@project:proj-a>",
		},
		{
			ID:      "msg-2",
			OrgID:   orgID,
			Content: "<@user:user-bob> updated <@channel:chan-2>",
		},
		{
			ID:      "msg-3",
			OrgID:   orgID,
			Content: "<@everyone> this has no resolved entities",
		},
	}

	svc.hydrateMentionsList(ctx, messages)

	// msg-1: has Alice, task-1, proj-a
	m1 := messages[0].Mentions
	if m1 == nil {
		t.Fatal("msg-1: Mentions is nil")
	}
	if name, ok := m1.Users["user-alice"]; !ok || name != "Alice" {
		t.Errorf("msg-1: expected user-alice=Alice, got ok=%v name=%q", ok, name)
	}
	if m1.Users["user-bob"] != "" {
		t.Errorf("msg-1: should NOT contain user-bob, got %q", m1.Users["user-bob"])
	}
	if tm, ok := m1.Tasks["task-1"]; !ok || tm.Title != "Task one" {
		t.Errorf("msg-1: expected task-1=Task one, got ok=%v title=%q", ok, tm.Title)
	}
	if _, ok := m1.Tasks["task-2"]; ok {
		t.Errorf("msg-1: should NOT contain task-2")
	}
	if name, ok := m1.Projects["proj-a"]; !ok || name != "Project A" {
		t.Errorf("msg-1: expected proj-a=Project A, got ok=%v name=%q", ok, name)
	}
	if m1.Channels["chan-2"] != "" {
		t.Errorf("msg-1: should NOT contain chan-2, got %q", m1.Channels["chan-2"])
	}

	// msg-2: has Bob, chan-2
	m2 := messages[1].Mentions
	if m2 == nil {
		t.Fatal("msg-2: Mentions is nil")
	}
	if name, ok := m2.Users["user-bob"]; !ok || name != "Bob" {
		t.Errorf("msg-2: expected user-bob=Bob, got ok=%v name=%q", ok, name)
	}
	if m2.Users["user-alice"] != "" {
		t.Errorf("msg-2: should NOT contain user-alice, got %q", m2.Users["user-alice"])
	}
	if name, ok := m2.Channels["chan-2"]; !ok || name != "random" {
		t.Errorf("msg-2: expected chan-2=random, got ok=%v name=%q", ok, name)
	}
	if len(m2.Projects) != 0 {
		t.Errorf("msg-2: expected 0 projects, got %d", len(m2.Projects))
	}
	if len(m2.Tasks) != 0 {
		t.Errorf("msg-2: expected 0 tasks, got %d", len(m2.Tasks))
	}

	// msg-3: @everyone only: no resolved maps
	m3 := messages[2].Mentions
	if m3 == nil {
		t.Fatal("msg-3: Mentions is nil")
	}
	if len(m3.Users)+len(m3.Projects)+len(m3.Tasks)+len(m3.Channels) != 0 {
		t.Errorf("msg-3: expected all maps empty, got users=%d proj=%d tasks=%d chans=%d",
			len(m3.Users), len(m3.Projects), len(m3.Tasks), len(m3.Channels))
	}
}

func TestHydrateMentions_AllTypesResolved(t *testing.T) {
	ctx := context.Background()
	orgID := "org-1"

	userRepo := newMockUserRepo()
	projectRepo := newMockProjectRepo()
	taskRepo := newMockTaskRepo()
	convRepo := newMockConversationRepo()

	_ = userRepo.Create(ctx, &domain.User{ID: "user-1", Name: "Alice", OrgID: orgID})
	_ = projectRepo.Create(ctx, &domain.Project{ID: "proj-1", Name: "Platform", OrgID: orgID})
	_ = taskRepo.Create(ctx, &domain.Task{ID: "task-1", Title: "Fix bug", ProjectID: "proj-1", OrgID: orgID})
	_ = convRepo.Create(ctx, &domain.Conversation{ID: "chan-1", OrgID: orgID, Name: "general", Type: domain.ConvChannel})

	svc := NewMessageService(MessageServiceDeps{
		MsgRepo:        newMockMessageRepo(),
		ConvRepo:       convRepo,
		OrgRepo:        newMockOrgRepo(true),
		ProjectRepo:    projectRepo,
		TaskRepo:       taskRepo,
		UserRepo:       userRepo,
		AttRepo:        newMockMessageAttachmentRepo(),
		PendingAttRepo: newMockPendingAttachmentRepo(),
		ReactionRepo:   newMockReactionRepo(),
		PrefRepo:       newMockUserPrefRepo(),
		NotifSvc:       newMockNotificationService(),
		Broadcaster:    newMockBroadcaster(),
		UserPrefRepo:   nil,
		I18n:           nil,
	})

	msg := &domain.Message{
		ID:      "msg-1",
		OrgID:   orgID,
		Content: "<@user:user-1> <@project:proj-1> <@task:task-1> <@channel:chan-1>",
	}
	svc.hydrateMentionsList(ctx, []*domain.Message{msg})

	if msg.Mentions == nil {
		t.Fatal("Mentions is nil")
	}
	if got := msg.Mentions.Users["user-1"]; got != "Alice" {
		t.Errorf("user: expected Alice, got %q", got)
	}
	if got := msg.Mentions.Projects["proj-1"]; got != "Platform" {
		t.Errorf("project: expected Platform, got %q", got)
	}
	if got := msg.Mentions.Tasks["task-1"]; got.Title != "Fix bug" {
		t.Errorf("task: expected Fix bug, got %q", got.Title)
	}
	if got := msg.Mentions.Tasks["task-1"]; got.ProjectID != "proj-1" {
		t.Errorf("task project_id: expected proj-1, got %q", got.ProjectID)
	}
	if got := msg.Mentions.Channels["chan-1"]; got != "general" {
		t.Errorf("channel: expected general, got %q", got)
	}
}

func TestHydrateMentions_UnknownIDsOmitted(t *testing.T) {
	ctx := context.Background()
	orgID := "org-1"

	svc := NewMessageService(MessageServiceDeps{
		MsgRepo:        newMockMessageRepo(),
		ConvRepo:       newMockConversationRepo(),
		OrgRepo:        newMockOrgRepo(true),
		ProjectRepo:    newMockProjectRepo(),
		TaskRepo:       newMockTaskRepo(),
		UserRepo:       newMockUserRepo(),
		AttRepo:        newMockMessageAttachmentRepo(),
		PendingAttRepo: newMockPendingAttachmentRepo(),
		ReactionRepo:   newMockReactionRepo(),
		PrefRepo:       newMockUserPrefRepo(),
		NotifSvc:       newMockNotificationService(),
		Broadcaster:    newMockBroadcaster(),
		UserPrefRepo:   nil,
		I18n:           nil,
	})

	msg := &domain.Message{
		ID:      "msg-1",
		OrgID:   orgID,
		Content: "<@user:nonexistent> <@project:notfound> <@task:missing> <@channel:gone>",
	}
	svc.hydrateMentionsList(ctx, []*domain.Message{msg})

	if msg.Mentions == nil {
		t.Fatal("Mentions is nil")
	}
	// All maps should exist but be empty since none of the IDs resolved
	if len(msg.Mentions.Users) != 0 {
		t.Errorf("expected 0 users for non-existent IDs, got %d", len(msg.Mentions.Users))
	}
	if len(msg.Mentions.Projects) != 0 {
		t.Errorf("expected 0 projects for non-existent IDs, got %d", len(msg.Mentions.Projects))
	}
	if len(msg.Mentions.Tasks) != 0 {
		t.Errorf("expected 0 tasks for non-existent IDs, got %d", len(msg.Mentions.Tasks))
	}
	if len(msg.Mentions.Channels) != 0 {
		t.Errorf("expected 0 channels for non-existent IDs, got %d", len(msg.Mentions.Channels))
	}
}

func TestHydrateMentions_EmptyMessages(t *testing.T) {
	ctx := context.Background()

	svc := NewMessageService(MessageServiceDeps{
		MsgRepo:        newMockMessageRepo(),
		ConvRepo:       newMockConversationRepo(),
		OrgRepo:        newMockOrgRepo(true),
		ProjectRepo:    newMockProjectRepo(),
		TaskRepo:       newMockTaskRepo(),
		UserRepo:       newMockUserRepo(),
		AttRepo:        newMockMessageAttachmentRepo(),
		PendingAttRepo: newMockPendingAttachmentRepo(),
		ReactionRepo:   newMockReactionRepo(),
		PrefRepo:       newMockUserPrefRepo(),
		NotifSvc:       newMockNotificationService(),
		Broadcaster:    newMockBroadcaster(),
		UserPrefRepo:   nil,
		I18n:           nil,
	})

	// Should not panic with nil/empty slice
	svc.hydrateMentionsList(ctx, nil)
	svc.hydrateMentionsList(ctx, []*domain.Message{})

	// Message with no mention tokens
	msg := &domain.Message{ID: "msg-1", OrgID: "org-1", Content: "just plain text"}
	svc.hydrateMentionsList(ctx, []*domain.Message{msg})
	if msg.Mentions == nil {
		t.Fatal("Mentions is nil for no-mention message")
	}
	if len(msg.Mentions.Users)+len(msg.Mentions.Projects)+len(msg.Mentions.Tasks)+len(msg.Mentions.Channels) != 0 {
		t.Error("expected all maps empty for plain text message")
	}
}

// ---------------------------------------------------------------------------
// MentionService.Search: integration tests with real SQLite
// ---------------------------------------------------------------------------

func setupMentionSearchDB(t *testing.T) (context.Context, *sqlc.Queries, *sql.DB, string, string) {
	t.Helper()
	ctx := context.Background()
	tmpDir := t.TempDir()
	dbPath := filepath.Join(tmpDir, "mention_test.db")

	db, err := store.NewDB(dbPath)
	if err != nil {
		t.Fatalf("open db: %v", err)
	}
	t.Cleanup(func() { db.Close() })

	if err := migration.RunMigrations(db); err != nil {
		t.Fatalf("migrations: %v", err)
	}

	q := sqlc.New(db)
	orgID := "org-mention-test"

	// Org
	_, err = db.Exec(`INSERT INTO organizations (id, name, slug) VALUES (?, 'Mention Test', 'mention-test')`, orgID)
	if err != nil {
		t.Fatalf("insert org: %v", err)
	}

	// Two users: Alice (member role) and Bob (viewer role). Each needs an
	// account row (credential) + a users (membership) row.
	_, err = db.Exec(`INSERT INTO accounts (id, email, password_hash) VALUES ('acct-alice', 'alice@test.com', 'hash')`)
	if err != nil {
		t.Fatalf("insert alice account: %v", err)
	}
	_, err = db.Exec(`INSERT INTO users (id, account_id, org_id, name, email, role, is_active) VALUES (?, 'acct-alice', ?, 'Alice', 'alice@test.com', 'member', 1)`, "user-alice", orgID)
	if err != nil {
		t.Fatalf("insert alice: %v", err)
	}
	_, err = db.Exec(`INSERT INTO accounts (id, email, password_hash) VALUES ('acct-bob', 'bob@test.com', 'hash')`)
	if err != nil {
		t.Fatalf("insert bob account: %v", err)
	}
	_, err = db.Exec(`INSERT INTO users (id, account_id, org_id, name, email, role, is_active) VALUES (?, 'acct-bob', ?, 'Bob', 'bob@test.com', 'viewer', 1)`, "user-bob", orgID)
	if err != nil {
		t.Fatalf("insert bob: %v", err)
	}
	// Inactive user should not appear in user mentions
	_, err = db.Exec(`INSERT INTO accounts (id, email, password_hash) VALUES ('acct-inactive', 'inactive@test.com', 'hash')`)
	if err != nil {
		t.Fatalf("insert inactive account: %v", err)
	}
	_, err = db.Exec(`INSERT INTO users (id, account_id, org_id, name, email, role, is_active) VALUES (?, 'acct-inactive', ?, 'Inactive', 'inactive@test.com', 'member', 0)`, "user-inactive", orgID)
	if err != nil {
		t.Fatalf("insert inactive: %v", err)
	}

	// Two channels: alice is in general, not in private
	_, err = db.Exec(`INSERT INTO conversations (id, org_id, name, type, created_by) VALUES (?, ?, 'general', 'channel', ?)`, "chan-general", orgID, "user-alice")
	if err != nil {
		t.Fatalf("insert channel general: %v", err)
	}
	_, err = db.Exec(`INSERT INTO conversations (id, org_id, name, type, created_by) VALUES (?, ?, 'private', 'channel', ?)`, "chan-private", orgID, "user-alice")
	if err != nil {
		t.Fatalf("insert channel private: %v", err)
	}
	// Alice is member of general
	_, err = db.Exec(`INSERT INTO conversation_members (conversation_id, user_id, org_id, joined_at) VALUES (?, ?, ?, datetime('now'))`, "chan-general", "user-alice", orgID)
	if err != nil {
		t.Fatalf("add alice to general: %v", err)
	}

	// Projects
	_, err = db.Exec(`INSERT INTO projects (id, org_id, name, slug, color, created_by) VALUES (?, ?, 'Frontend', 'frontend', '#ff0000', ?)`, "proj-frontend", orgID, "user-alice")
	if err != nil {
		t.Fatalf("insert project frontend: %v", err)
	}
	_, err = db.Exec(`INSERT INTO projects (id, org_id, name, slug, color, created_by) VALUES (?, ?, 'Backend', 'backend', '#00ff00', ?)`, "proj-backend", orgID, "user-alice")
	if err != nil {
		t.Fatalf("insert project backend: %v", err)
	}

	// Task statuses
	_, err = db.Exec(`INSERT INTO task_statuses (id, project_id, name, color, category, is_default, position) VALUES (?, ?, 'Todo', '#ccc', 'todo', 1, 0)`, "status-todo-f", "proj-frontend")
	if err != nil {
		t.Fatalf("insert status: %v", err)
	}
	_, err = db.Exec(`INSERT INTO task_statuses (id, project_id, name, color, category, is_default, position) VALUES (?, ?, 'Todo', '#ccc', 'todo', 1, 0)`, "status-todo-b", "proj-backend")
	if err != nil {
		t.Fatalf("insert status: %v", err)
	}

	// Tasks
	_, err = db.Exec(`INSERT INTO tasks (id, org_id, project_id, title, status_id, created_by) VALUES (?, ?, ?, 'Setup Vite', ?, ?)`, "task-vite", orgID, "proj-frontend", "status-todo-f", "user-alice")
	if err != nil {
		t.Fatalf("insert task vite: %v", err)
	}
	_, err = db.Exec(`INSERT INTO tasks (id, org_id, project_id, title, status_id, created_by) VALUES (?, ?, ?, 'API Gateway', ?, ?)`, "task-api", orgID, "proj-backend", "status-todo-b", "user-alice")
	if err != nil {
		t.Fatalf("insert task api: %v", err)
	}
	// Task with "everyone" in title to ensure it doesn't interfere with @everyone
	_, err = db.Exec(`INSERT INTO tasks (id, org_id, project_id, title, status_id, created_by) VALUES (?, ?, ?, 'everyone task', ?, ?)`, "task-everyone", orgID, "proj-frontend", "status-todo-f", "user-alice")
	if err != nil {
		t.Fatalf("insert task everyone: %v", err)
	}

	// Project member: Bob (viewer) is a member of Backend only
	_, err = db.Exec(`INSERT INTO project_members (project_id, user_id, role) VALUES (?, ?, 'viewer')`, "proj-backend", "user-bob")
	if err != nil {
		t.Fatalf("add bob to backend: %v", err)
	}

	return ctx, q, db, orgID, "user-alice"
}

func TestMentionSearch_EmptyQueryReturnsEveryone(t *testing.T) {
	ctx, q, db, orgID, userID := setupMentionSearchDB(t)
	convRepo := store.NewConversationStore(q, db)
	svc := NewMentionService(store.NewMentionSearchStore(q), convRepo)

	results, err := svc.Search(ctx, orgID, userID, "member", "", nil, 10)
	if err != nil {
		t.Fatalf("Search: %v", err)
	}

	// Empty query should NOT include @everyone (prevent accidental mass pings)
	foundEveryone := false
	foundAlice := false
	for _, r := range results {
		if r.Type == domain.MentionEveryone && r.ID == "@everyone" {
			foundEveryone = true
		}
		if r.Type == domain.MentionUser && r.Label == "Alice" {
			foundAlice = true
		}
	}
	if foundEveryone {
		t.Error("@everyone should not appear for empty query")
	}
	if !foundAlice {
		t.Error("expected Alice in results for empty query")
	}
	// Inactive users should not appear
	for _, r := range results {
		if r.Label == "Inactive" {
			t.Error("inactive user should not appear in mention results")
		}
	}
}

func TestMentionSearch_QueryFiltersResults(t *testing.T) {
	ctx, q, db, orgID, userID := setupMentionSearchDB(t)
	convRepo := store.NewConversationStore(q, db)
	svc := NewMentionService(store.NewMentionSearchStore(q), convRepo)

	results, err := svc.Search(ctx, orgID, userID, "member", "ali", nil, 10)
	if err != nil {
		t.Fatalf("Search: %v", err)
	}

	// Should find Alice but not Bob
	for _, r := range results {
		if r.Type == domain.MentionUser {
			if r.Label == "Bob" {
				t.Errorf("Bob should not match query 'ali', got %+v", r)
			}
		}
	}
	// Should find at least Alice
	foundAlice := false
	for _, r := range results {
		if r.Type == domain.MentionUser && r.Label == "Alice" {
			foundAlice = true
		}
	}
	if !foundAlice {
		t.Error("expected Alice to match query 'ali'")
	}
	// @everyone should NOT appear when query doesn't match "everyone"
	for _, r := range results {
		if r.Type == domain.MentionEveryone {
			t.Error("@everyone should not appear when query is 'ali'")
		}
	}
}

func TestMentionSearch_EveryoneQuery(t *testing.T) {
	ctx, q, db, orgID, userID := setupMentionSearchDB(t)
	convRepo := store.NewConversationStore(q, db)
	svc := NewMentionService(store.NewMentionSearchStore(q), convRepo)

	results, err := svc.Search(ctx, orgID, userID, "member", "everyone", nil, 10)
	if err != nil {
		t.Fatalf("Search: %v", err)
	}

	foundEveryone := false
	for _, r := range results {
		if r.Type == domain.MentionEveryone && r.ID == "@everyone" {
			foundEveryone = true
			break
		}
	}
	if !foundEveryone {
		t.Error("expected @everyone when query matches 'everyone'")
	}
}

func TestMentionSearch_ChannelMembershipFilter(t *testing.T) {
	ctx, q, db, orgID, userID := setupMentionSearchDB(t)
	convRepo := store.NewConversationStore(q, db)
	svc := NewMentionService(store.NewMentionSearchStore(q), convRepo)

	// alice is only in general, not in private
	results, err := svc.Search(ctx, orgID, userID, "member", "", []domain.MentionType{domain.MentionChannel}, 10)
	if err != nil {
		t.Fatalf("Search: %v", err)
	}

	foundGeneral := false
	foundPrivate := false
	for _, r := range results {
		if r.Type == domain.MentionChannel && r.Label == "general" {
			foundGeneral = true
		}
		if r.Type == domain.MentionChannel && r.Label == "private" {
			foundPrivate = true
		}
	}
	if !foundGeneral {
		t.Error("expected general channel (alice is a member)")
	}
	if foundPrivate {
		t.Error("private channel should not appear (alice is not a member)")
	}
}

func TestMentionSearch_OnlyRequestedTypes(t *testing.T) {
	ctx, q, db, orgID, userID := setupMentionSearchDB(t)
	convRepo := store.NewConversationStore(q, db)
	svc := NewMentionService(store.NewMentionSearchStore(q), convRepo)

	results, err := svc.Search(ctx, orgID, userID, "member", "", []domain.MentionType{domain.MentionUser, domain.MentionChannel}, 10)
	if err != nil {
		t.Fatalf("Search: %v", err)
	}

	for _, r := range results {
		if r.Type != domain.MentionUser && r.Type != domain.MentionChannel {
			t.Errorf("unexpected type %q in results (only user+channel requested)", r.Type)
		}
	}
}

func TestMentionSearch_ViewerSeesOnlyMemberProjects(t *testing.T) {
	ctx, q, db, orgID, _ := setupMentionSearchDB(t)
	convRepo := store.NewConversationStore(q, db)
	svc := NewMentionService(store.NewMentionSearchStore(q), convRepo)

	// Bob is a viewer who's only member of Backend project
	results, err := svc.Search(ctx, orgID, "user-bob", "viewer", "", []domain.MentionType{domain.MentionProject}, 10)
	if err != nil {
		t.Fatalf("Search: %v", err)
	}

	foundBackend := false
	foundFrontend := false
	for _, r := range results {
		if r.Type == domain.MentionProject && r.Label == "Backend" {
			foundBackend = true
		}
		if r.Type == domain.MentionProject && r.Label == "Frontend" {
			foundFrontend = true
		}
	}
	if !foundBackend {
		t.Error("viewer should see Backend (they are a member)")
	}
	if foundFrontend {
		t.Error("viewer should NOT see Frontend (they are not a member)")
	}
}

func TestMentionSearch_ViewerSeesOnlyMemberProjectTasks(t *testing.T) {
	ctx, q, db, orgID, _ := setupMentionSearchDB(t)
	convRepo := store.NewConversationStore(q, db)
	svc := NewMentionService(store.NewMentionSearchStore(q), convRepo)

	// Bob is a viewer who's only member of Backend project
	results, err := svc.Search(ctx, orgID, "user-bob", "viewer", "", []domain.MentionType{domain.MentionTask}, 10)
	if err != nil {
		t.Fatalf("Search: %v", err)
	}

	foundAPI := false
	foundVite := false
	for _, r := range results {
		if r.Type == domain.MentionTask && r.Label == "API Gateway" {
			foundAPI = true
		}
		if r.Type == domain.MentionTask && r.Label == "Setup Vite" {
			foundVite = true
		}
	}
	if !foundAPI {
		t.Error("viewer should see API Gateway task (in Backend project they are member of)")
	}
	if foundVite {
		t.Error("viewer should NOT see Setup Vite task (in Frontend project they are not member of)")
	}
}

func TestMentionSearch_ProjectTaskHasProjectInfo(t *testing.T) {
	ctx, q, db, orgID, userID := setupMentionSearchDB(t)
	convRepo := store.NewConversationStore(q, db)
	svc := NewMentionService(store.NewMentionSearchStore(q), convRepo)

	results, err := svc.Search(ctx, orgID, userID, "member", "vite", []domain.MentionType{domain.MentionTask}, 10)
	if err != nil {
		t.Fatalf("Search: %v", err)
	}

	for _, r := range results {
		if r.Type == domain.MentionTask {
			if r.ProjectName == nil || *r.ProjectName == "" {
				t.Errorf("task %q missing project_name", r.Label)
			}
			if r.ProjectID == nil || *r.ProjectID == "" {
				t.Errorf("task %q missing project_id", r.Label)
			}
		}
	}
}

func TestMentionSearch_LimitCapsAt20(t *testing.T) {
	ctx, q, db, orgID, userID := setupMentionSearchDB(t)
	convRepo := store.NewConversationStore(q, db)
	svc := NewMentionService(store.NewMentionSearchStore(q), convRepo)

	// Passing limit=100 should cap at 20
	results, err := svc.Search(ctx, orgID, userID, "member", "", nil, 100)
	if err != nil {
		t.Fatalf("Search: %v", err)
	}
	if len(results) > 20 {
		t.Errorf("limit should cap at 20, got %d", len(results))
	}
}

func TestMentionSearch_EmptyResultsForNoMatch(t *testing.T) {
	ctx, q, db, orgID, userID := setupMentionSearchDB(t)
	convRepo := store.NewConversationStore(q, db)
	svc := NewMentionService(store.NewMentionSearchStore(q), convRepo)

	results, err := svc.Search(ctx, orgID, userID, "member", "zzzznonexistent", nil, 10)
	if err != nil {
		t.Fatalf("Search: %v", err)
	}

	// With a non-matching query, no users/channels/projects/tasks match.
	// @everyone should NOT appear either since query doesn't match "everyone".
	for _, r := range results {
		if r.Type == domain.MentionEveryone {
			t.Error("@everyone should not appear for non-matching query")
		}
	}
}
