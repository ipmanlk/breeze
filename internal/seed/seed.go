package seed

import (
	"context"
	"database/sql"
	"log"
	"time"

	"ipmanlk/breeze/internal/store"
	"ipmanlk/breeze/internal/store/sqlc"
)

// Seeder orchestrates the database seeding process, using the store layer
// for data creation and the service layer where business logic is valuable.
// Only clearData uses raw SQL; no store/service equivalent for 40-table DELETE.
type Seeder struct {
	db     *sql.DB
	q      *sqlc.Queries
	stores *Stores
	ctx    context.Context
}

// Stores holds all store instances needed by the seeder.
type Stores struct {
	Org                *store.OrgStore
	Account            *store.AccountStore
	User               *store.UserStore
	Project            *store.ProjectStore
	TaskStatus         *store.TaskStatusStore
	Task               *store.TaskStore
	Cycle              *store.CycleStore
	TimeEntry          *store.TimeEntryStore
	Label              *store.LabelStore
	Comment            *store.CommentStore
	TaskActivity       *store.TaskActivityStore
	Audit              *store.AuditStore
	Notification       *store.NotificationStore
	CustomField        *store.CustomFieldStore
	View               *store.ViewStore
	Conversation       *store.ConversationStore
	Message            *store.MessageStore
	TaskTemplate       *store.TaskTemplateStore
	ChannelPerm        *store.ChannelPermissionStore
	ChannelProjectLink *store.ChannelProjectLinkStore
}

// NewSeeder creates a new Seeder with all stores initialized.
func NewSeeder(db *sql.DB) *Seeder {
	q := sqlc.New(db)
	return &Seeder{
		db:  db,
		q:   q,
		ctx: context.Background(),
		stores: &Stores{
			Org:                store.NewOrgStore(q, db),
			Account:            store.NewAccountStore(q),
			User:               store.NewUserStore(q, db),
			Project:            store.NewProjectStore(q, db),
			TaskStatus:         store.NewTaskStatusStore(q),
			Task:               store.NewTaskStore(q, db),
			Cycle:              store.NewCycleStore(q, db),
			TimeEntry:          store.NewTimeEntryStore(q, db),
			Label:              store.NewLabelStore(q, db),
			Comment:            store.NewCommentStore(q),
			TaskActivity:       store.NewTaskActivityStore(q),
			Audit:              store.NewAuditStore(q),
			Notification:       store.NewNotificationStore(q),
			CustomField:        store.NewCustomFieldStore(q),
			View:               store.NewViewStore(q),
			Conversation:       store.NewConversationStore(q, db),
			Message:            store.NewMessageStore(q),
			TaskTemplate:       store.NewTaskTemplateStore(q, db),
			ChannelPerm:        store.NewChannelPermissionStore(q),
			ChannelProjectLink: store.NewChannelProjectLinkStore(q, db),
		},
	}
}

// Seed performs the full database population.
func (s *Seeder) Seed() {
	log.Println("🌱 Breeze database seeder")

	// Wipe existing data
	s.clearData()

	// Create primary user and organization
	userID, orgID := s.createUserAndOrg()
	log.Printf("Created user %s and org %s\n", userID[:8], orgID[:8])

	// Create secondary users
	userID2 := s.createSecondUser(orgID)
	log.Printf("Created second user %s\n", userID2[:8])

	userID3 := s.createGuestUser(orgID)
	log.Printf("Created guest user %s\n", userID3[:8])

	// Create seed data
	now := time.Now().Round(0)
	projectStart := now.AddDate(0, 0, -28)

	projects := createProjectDefs(projectStart, now)

	for _, def := range projects {
		s.createProject(def, orgID, userID, projectStart, now)
	}

	// Add second user to all projects
	s.addSecondUserToProjects(orgID, userID2)

	// Create cross-cutting data (labels, comments, dependencies, activity, etc.)
	s.seedCrossCuttingData(orgID, userID, userID2, now)

	// Create sample views
	s.createSampleViews(orgID, userID)

	// Create sample chat data
	s.seedChat(orgID, userID, userID2, userID3)

	log.Println("✅ Done! Seeded", len(projects), "projects with tasks, cycles, subtasks, and time entries.")
}
