package seed

import (
	"fmt"
	"log"
	"math/rand"
	"time"

	"ipmanlk/breeze/internal/auth"
	"ipmanlk/breeze/internal/domain"
	"ipmanlk/breeze/internal/lexorank"
)

// ---- tables-to-clear list ----

var tablesToClear = []string{
	"user_invite_acceptances",
	"user_invites",
	"invite_projects",
	"messages_fts",
	"attachments",
	"audit_log",
	"comments",
	"conversation_members",
	"conversations",
	"cycles",
	"labels",
	"message_attachments",
	"message_reactions",
	"messages",
	"notifications",
	"notification_preferences",
	"password_resets",
	"pending_attachments",
	"project_members",
	"projects",
	"projects_fts",
	"push_subscriptions",
	"task_activity",
	"task_assignees",
	"task_dependencies",
	"task_labels",
	"task_statuses",
	"tasks",
	"tasks_fts",
	"task_templates",
	"task_custom_field_values",
	"custom_fields",
	"time_entries",
	"voice_participants",
	"channel_project_links",
	"channel_permissions",
	"channel_user_overrides",
	"user_channel_preferences",
	"user_preferences",
	"user_presence",
	"dashboard_preferences",
	"view_pins",
	"views",
	"sessions",
	"users",
	"accounts",
	"organizations",
}

// clearData wipes all tables. Raw SQL: no store equivalent for 40-table bulk DELETE.
func (s *Seeder) clearData() {
	log.Println("Wiping database...")
	for _, table := range tablesToClear {
		_, err := s.db.Exec(fmt.Sprintf("DELETE FROM %s", table))
		if err != nil {
			continue
		}
		log.Printf("  Cleared %s", table)
	}
	// Note: sqlite_sequence is not used; all Breeze PKs are UUIDs, not AUTOINCREMENT.
	log.Println("Database wiped.")
}

// ---- primary user + org ----

func (s *Seeder) createUserAndOrg() (userID, orgID string) {
	orgID = newUUID()
	userID = newUUID()

	now := time.Now()
	passwordHash, err := auth.HashPassword("test@test.com")
	if err != nil {
		log.Fatalf("Failed to hash password: %v", err)
	}

	// Use store Create methods
	if err := s.stores.Org.Create(s.ctx, &domain.Organization{
		ID:        orgID,
		Name:      "Acme Corp",
		Slug:      "acme-corp",
		CreatedAt: now,
		UpdatedAt: now,
	}); err != nil {
		log.Fatalf("Failed to create organization via store: %v", err)
	}

	accountID := newUUID()
	if err := s.stores.Account.Create(s.ctx, &domain.Account{
		ID:           accountID,
		Email:        "test@test.com",
		PasswordHash: passwordHash,
		CreatedAt:    now,
		UpdatedAt:    now,
	}); err != nil {
		log.Fatalf("Failed to create account via store: %v", err)
	}

	if err := s.stores.User.Create(s.ctx, &domain.User{
		ID:        userID,
		AccountID: accountID,
		OrgID:     orgID,
		Email:     "test@test.com",
		Name:      "Test User",
		Role:      "owner",
		IsActive:  true,
		CreatedAt: now,
		UpdatedAt: now,
	}); err != nil {
		log.Fatalf("Failed to create user via store: %v", err)
	}

	return userID, orgID
}

// ---- secondary users ----

func (s *Seeder) createSecondUser(orgID string) string {
	userID := newUUID()
	now := time.Now()
	passwordHash, err := auth.HashPassword("test1@test.com")
	if err != nil {
		log.Fatalf("Failed to hash password: %v", err)
	}

	accountID := newUUID()
	if err := s.stores.Account.Create(s.ctx, &domain.Account{
		ID:           accountID,
		Email:        "test1@test.com",
		PasswordHash: passwordHash,
		CreatedAt:    now,
		UpdatedAt:    now,
	}); err != nil {
		log.Fatalf("Failed to create second account via store: %v", err)
	}

	if err := s.stores.User.Create(s.ctx, &domain.User{
		ID:        userID,
		AccountID: accountID,
		OrgID:     orgID,
		Email:     "test1@test.com",
		Name:      "Test User 2",
		Role:      "member",
		IsActive:  true,
		CreatedAt: now,
		UpdatedAt: now,
	}); err != nil {
		log.Fatalf("Failed to create second user via store: %v", err)
	}

	return userID
}

func (s *Seeder) createGuestUser(orgID string) string {
	userID := newUUID()
	now := time.Now()
	passwordHash, err := auth.HashPassword("guest@test.com")
	if err != nil {
		log.Fatalf("Failed to hash password: %v", err)
	}

	accountID := newUUID()
	if err := s.stores.Account.Create(s.ctx, &domain.Account{
		ID:           accountID,
		Email:        "guest@test.com",
		PasswordHash: passwordHash,
		CreatedAt:    now,
		UpdatedAt:    now,
	}); err != nil {
		log.Fatalf("Failed to create guest account via store: %v", err)
	}

	if err := s.stores.User.Create(s.ctx, &domain.User{
		ID:        userID,
		AccountID: accountID,
		OrgID:     orgID,
		Email:     "guest@test.com",
		Name:      "Guest User",
		Role:      "guest",
		IsActive:  true,
		CreatedAt: now,
		UpdatedAt: now,
	}); err != nil {
		log.Fatalf("Failed to create guest user via store: %v", err)
	}

	return userID
}

// ---- project creation (project + statuses + cycles + tasks + subtasks + time entries) ----

func (s *Seeder) createProject(def ProjectDef, orgID, userID string, projectStart, now time.Time) {
	projectID := newUUID()
	projectSlug := slug(def.Name)

	log.Printf("\nCreating project: %s (%s)", def.Name, projectID[:8])

	var startsAt, endsAt *time.Time
	if def.WithCycles {
		ends := projectStart.AddDate(0, 6, 0)
		startsAt = &projectStart
		endsAt = &ends
	}

	// Use store Create method for project
	if err := s.stores.Project.Create(s.ctx, &domain.Project{
		ID:                     projectID,
		OrgID:                  orgID,
		Name:                   def.Name,
		Description:            "",
		Slug:                   projectSlug,
		Color:                  def.Color,
		Icon:                   def.Icon,
		CreatedBy:              userID,
		CycleDuration:          def.CycleDuration,
		AutoGenerateCycles:     def.WithCycles,
		IncompleteTaskHandling: "next_cycle",
		StartsAt:               startsAt,
		EndsAt:                 endsAt,
		CreatedAt:              projectStart.Add(time.Duration(randomInt(5)) * 24 * time.Hour),
		UpdatedAt:              now,
	}); err != nil {
		log.Fatalf("Failed to insert project: %v", err)
	}

	// Add project member (raw SQL; project_member store has no direct Add method)
	// Note: ProjectMemberStore uses sqlc.AddProjectMember for Add but wraps it as Add method.
	// We'll use raw SQL for simplicity since there's no public store method.
	if _, err := s.db.Exec("INSERT INTO project_members (project_id, user_id, role) VALUES (?, ?, ?)",
		projectID, userID, "admin"); err != nil {
		log.Fatalf("Failed to insert project member: %v", err)
	}

	// Create statuses using store Create
	statuses := createStatusDefs()
	for i, st := range statuses {
		if err := s.stores.TaskStatus.Create(s.ctx, &domain.TaskStatus{
			ID:        st.ID,
			ProjectID: projectID,
			Name:      st.Name,
			Color:     st.Color,
			Position:  st.Position,
			Category:  st.Category,
			Default:   i == 1,
		}); err != nil {
			log.Fatalf("Failed to insert status: %v", err)
		}
	}
	log.Printf("  Created %d statuses", len(statuses))

	// Create cycles if applicable
	var cycles []CycleInfo
	if def.WithCycles {
		cycleLen := 14
		if def.CycleDuration != nil {
			cycleLen = *def.CycleDuration
		}

		cycleNames := []string{"Sprint 1", "Sprint 2", "Sprint 3", "Sprint 4"}
		cycleGoals := []string{
			"Foundation: set up project structure and core components",
			"Core features: implement main user flows and functionality",
			"Polish: refine UX, fix bugs, and optimize performance",
			"Ship: final testing, documentation, and deployment preparation",
		}

		for i := 0; i < 4; i++ {
			cycleStart := projectStart.Add(time.Duration(i*cycleLen) * 24 * time.Hour)
			cycleEnd := cycleStart.Add(time.Duration(cycleLen-1) * 24 * time.Hour)
			cycleID := newUUID()
			isActive := i == 2
			isCompleted := i < 2

			if err := s.stores.Cycle.Create(s.ctx, &domain.Cycle{
				ID:          cycleID,
				OrgID:       orgID,
				ProjectID:   projectID,
				Name:        cycleNames[i],
				Goal:        cycleGoals[i],
				StartsAt:    cycleStart,
				EndsAt:      cycleEnd,
				CreatedBy:   userID,
				IsCompleted: isCompleted,
				IsActive:    isActive,
				CreatedAt:   cycleStart,
				UpdatedAt:   cycleStart.Add(24 * time.Hour),
			}); err != nil {
				log.Fatalf("Failed to insert cycle: %v", err)
			}

			cycles = append(cycles, CycleInfo{
				ID:       cycleID,
				StartsAt: cycleStart,
				EndsAt:   cycleEnd,
			})
			log.Printf("  Created cycle: %s", cycleNames[i])
		}
	}

	// Insert tasks
	var lastPositionKey string
	taskCount := 0

	for _, tDef := range def.Tasks {
		status := statuses[tDef.StatusIdx]
		positionKey := nextPositionKey(&lastPositionKey)

		var cycleID *string
		if len(cycles) > 0 {
			cycleID = findCycleForTask(tDef, cycles)
		}

		createdAt := tDef.StartedAt
		if createdAt == nil {
			createdAt = tDef.DueAt
		}
		if createdAt == nil {
			createdAt = &now
		}

		updatedAt := tDef.CompletedAt
		if updatedAt == nil {
			updatedAt = &now
		}

		taskID := newUUID()

		// Use store Create method for task; it handles assignees and transaction
		task := &domain.Task{
			ID:          taskID,
			OrgID:       orgID,
			ProjectID:   projectID,
			CycleID:     cycleID,
			ParentID:    nil,
			CreatedBy:   userID,
			Title:       tDef.Title,
			Description: tDef.Description,
			StatusID:    status.ID,
			Priority:    tDef.Priority,
			PositionKey: positionKey,
			Estimate:    tDef.Estimate,
			StartedAt:   tDef.StartedAt,
			DueAt:       tDef.DueAt,
			CompletedAt: tDef.CompletedAt,
			CreatedAt:   *createdAt,
			UpdatedAt:   *updatedAt,
			Assignees:   []domain.TaskAssignee{{ID: userID}},
		}

		if err := s.stores.Task.Create(s.ctx, task); err != nil {
			log.Fatalf("Failed to insert task: %v", err)
		}

		// Insert time entries using store Create method
		for _, te := range tDef.TimeEntries {
			teEnd := te.Date.Add(time.Duration(te.Minutes) * time.Minute)
			if err := s.stores.TimeEntry.Create(s.ctx, &domain.TimeEntry{
				ID:              newUUID(),
				TaskID:          taskID,
				UserID:          userID,
				Description:     te.Description,
				StartedAt:       te.Date,
				EndedAt:         &teEnd,
				DurationMinutes: intPtr(te.Minutes),
				CreatedAt:       te.Date,
				UpdatedAt:       te.Date,
			}); err != nil {
				log.Fatalf("Failed to insert time entry: %v", err)
			}
		}

		// Insert subtasks
		for _, st := range tDef.Subtasks {
			stStatusIdx := 2 // In Progress
			if st.IsCompleted {
				stStatusIdx = 4 // Done
			}
			stStatus := statuses[stStatusIdx]
			subtaskID := newUUID()
			var stCompletedAt *time.Time
			if st.IsCompleted {
				t := now.Add(-24 * time.Hour)
				stCompletedAt = &t
			}

			// Generate subtask position key
			subtaskPosKey, err := lexorank.GenerateKeyBetween("", "")
			if err != nil {
				subtaskPosKey = lexorank.FirstKey()
			}

			subtask := &domain.Task{
				ID:              subtaskID,
				OrgID:           orgID,
				ProjectID:       projectID,
				CycleID:         cycleID,
				ParentID:        &taskID,
				SubtaskPosition: subtaskPosKey,
				CreatedBy:       userID,
				Title:           st.Title,
				Description:     "",
				StatusID:        stStatus.ID,
				Priority:        tDef.Priority,
				PositionKey:     nextPositionKey(&lastPositionKey),
				Estimate:        nil,
				StartedAt:       nil,
				DueAt:           nil,
				CompletedAt:     stCompletedAt,
				CreatedAt:       now.Add(-3 * 24 * time.Hour),
				UpdatedAt:       now,
				Assignees:       []domain.TaskAssignee{{ID: userID}},
			}

			if err := s.stores.Task.Create(s.ctx, subtask); err != nil {
				log.Fatalf("Failed to insert subtask: %v", err)
			}
		}
		taskCount++
	}

	// Count subtasks and time entries
	subtaskCount := 0
	timeEntryCount := 0
	for _, t := range def.Tasks {
		subtaskCount += len(t.Subtasks)
		timeEntryCount += len(t.TimeEntries)
	}

	log.Printf("  Created %d tasks (%d subtasks)", taskCount, subtaskCount)
	log.Printf("  Created %d time entries", timeEntryCount)

	// Seed templates + custom fields for the first project only
	if def.Name == "Website Redesign" {
		s.seedTemplatesAndCustomFields(projectID, orgID, userID, statuses, now)
	}
}

// seedTemplatesAndCustomFields creates a recurring task template and custom fields.
// Uses store Create methods where available; raw SQL for custom field values.
func (s *Seeder) seedTemplatesAndCustomFields(projectID, orgID, userID string, statuses []Status, now time.Time) {
	todoStatusID := ""
	for _, st := range statuses {
		if st.Category == "todo" {
			todoStatusID = st.ID
			break
		}
	}
	if todoStatusID == "" {
		todoStatusID = statuses[0].ID
	}
	nextRun := now.AddDate(0, 0, 7)

	if err := s.stores.TaskTemplate.Create(s.ctx, &domain.TaskTemplate{
		ID:                newUUID(),
		OrgID:             orgID,
		ProjectID:         projectID,
		Name:              "Weekly Sprint Review",
		Description:       "Review completed work and plan the next sprint.",
		Priority:          "medium",
		StatusID:          todoStatusID,
		AssigneeIDs:       []string{},
		Estimate:          nil,
		RecurrencePattern: "weekly",
		RecurrenceDays:    "1",
		NextRunAt:         &nextRun,
		CreatedBy:         userID,
	}); err != nil {
		log.Printf("  Warning: failed to seed task template: %v", err)
	} else {
		log.Printf("  Created 1 task template (weekly recurring)")
	}

	// Custom fields via store Create
	cf1 := newUUID()
	if err := s.stores.CustomField.Create(s.ctx, &domain.CustomField{
		ID:        cf1,
		OrgID:     orgID,
		ProjectID: projectID,
		Name:      "Story Points",
		FieldType: "number",
		Options:   []string{},
		Position:  0,
	}); err != nil {
		log.Printf("Warning: failed to seed custom field Story Points: %v", err)
	}

	cf2 := newUUID()
	if err := s.stores.CustomField.Create(s.ctx, &domain.CustomField{
		ID:        cf2,
		OrgID:     orgID,
		ProjectID: projectID,
		Name:      "Department",
		FieldType: "select",
		Options:   []string{"Design", "Engineering", "Marketing", "QA"},
		Position:  1,
	}); err != nil {
		log.Printf("Warning: failed to seed custom field Department: %v", err)
	}

	log.Printf("  Created 2 custom fields (Story Points, Department)")
}

// addSecondUserToProjects adds userID2 as admin member to every project.
// Raw SQL; we need to bulk-add across all projects in one pass.
func (s *Seeder) addSecondUserToProjects(orgID, userID2 string) {
	rows, err := s.db.Query("SELECT id FROM projects WHERE org_id = ?", orgID)
	if err != nil {
		log.Printf("Warning: failed to query projects for second user: %v", err)
		return
	}

	projectIDs := []string{}
	for rows.Next() {
		var projectID string
		if err := rows.Scan(&projectID); err != nil {
			continue
		}
		projectIDs = append(projectIDs, projectID)
	}
	rows.Close()

	count := 0
	for _, projectID := range projectIDs {
		_, err := s.db.Exec(
			"INSERT OR IGNORE INTO project_members (project_id, user_id, role) VALUES (?, ?, ?)",
			projectID, userID2, "admin",
		)
		if err != nil {
			log.Printf("Warning: failed to add second user to project %s: %v", projectID[:8], err)
			continue
		}
		count++
	}
	if count > 0 {
		log.Printf("Added second user to %d projects", count)
	}
}

// ---- cross-cutting data ----

func (s *Seeder) seedCrossCuttingData(orgID, userID, userID2 string, now time.Time) {
	s.seedLabels(orgID, userID2, now)
	s.seedComments(orgID, userID, userID2, now)
	s.seedTaskDependencies(orgID)
	s.seedTaskActivity(orgID, userID, userID2, now)
	s.seedAuditLog(orgID, userID, now)
	s.seedNotifications(orgID, userID, now)
	s.seedCustomFieldValues(orgID, now)
}

func (s *Seeder) seedLabels(orgID, userID2 string, now time.Time) {
	labelDefs := []struct{ Name, Color string }{
		{"Frontend", "#3b82f6"},
		{"Backend", "#22c55e"},
		{"Design", "#a855f7"},
		{"Bug", "#ef4444"},
		{"Feature", "#06b6d4"},
		{"Tech Debt", "#f59e0b"},
		{"Documentation", "#64748b"},
		{"Urgent", "#dc2626"},
		{"Research", "#8b5cf6"},
		{"Refactor", "#0ea5e9"},
	}
	labelIDs := make([]string, len(labelDefs))
	for i, l := range labelDefs {
		id := newUUID()
		labelIDs[i] = id
		if err := s.stores.Label.Create(s.ctx, &domain.Label{
			ID:        id,
			OrgID:     orgID,
			Name:      l.Name,
			Color:     l.Color,
			CreatedAt: now.Add(-20 * 24 * time.Hour),
			UpdatedAt: now,
		}); err != nil {
			log.Printf("Warning: failed to seed label %s: %v", l.Name, err)
		}
	}
	log.Printf("Created %d labels", len(labelDefs))

	// Attach labels to tasks + assign userID2 to some tasks (raw SQL; no store batch method)
	rows, err := s.db.Query("SELECT id FROM tasks WHERE org_id = ? AND parent_task_id IS NULL", orgID)
	if err != nil {
		return
	}
	defer rows.Close()

	taskIDs := []string{}
	for rows.Next() {
		var id string
		if rows.Scan(&id) == nil {
			taskIDs = append(taskIDs, id)
		}
	}

	count := 0
	for i, tid := range taskIDs {
		numLabels := 1 + (i % 3)
		for j := 0; j < numLabels; j++ {
			labelIdx := (i*3 + j) % len(labelIDs)
			_, err := s.db.Exec(
				"INSERT OR IGNORE INTO task_labels (task_id, label_id) VALUES (?, ?)",
				tid, labelIDs[labelIdx],
			)
			if err == nil {
				count++
			}
		}
		if i%3 == 0 {
			s.db.Exec("INSERT OR IGNORE INTO task_assignees (task_id, user_id) VALUES (?, ?)", tid, userID2)
		}
	}
	log.Printf("Created %d task-label associations", count)
}

func (s *Seeder) seedComments(orgID, userID, userID2 string, now time.Time) {
	rows, err := s.db.Query("SELECT id, project_id FROM tasks WHERE org_id = ? AND parent_task_id IS NULL", orgID)
	if err != nil {
		return
	}
	defer rows.Close()

	type taskInfo struct{ ID, ProjectID string }
	tasks := []taskInfo{}
	for rows.Next() {
		var t taskInfo
		if rows.Scan(&t.ID, &t.ProjectID) == nil {
			tasks = append(tasks, t)
		}
	}

	commentTemplates := [][]string{
		{"I'll take a look at this today.", "Sounds good, let me know if you need help."},
		{"This is blocked by the API changes.", "I'll prioritize the API work then."},
		{"Can we discuss the approach in standup?", "Sure, I'll add it to the agenda."},
		{"I've started working on this.", "Great! How's it going?", "Making good progress, should have a PR up soon."},
		{"This needs design review before we proceed.", "I'll ping the design team."},
		{"Updated the estimate based on new requirements.", "Thanks for the heads up."},
		{"Found an edge case we need to handle.", "Good catch. Can you add a test for it?", "Already on it."},
		{"This is ready for review.", "I'll review it this afternoon."},
	}

	count := 0
	for i, t := range tasks {
		if i%3 != 0 {
			continue
		}
		thread := commentTemplates[i%len(commentTemplates)]
		var parentID *string
		for j, content := range thread {
			commentID := newUUID()
			author := userID
			if j%2 == 1 {
				author = userID2
			}
			createdAt := now.Add(-time.Duration((len(thread)-j)*24+rand.Intn(12)) * time.Hour)
			if err := s.stores.Comment.Create(s.ctx, &domain.Comment{
				ID:        commentID,
				OrgID:     orgID,
				TaskID:    t.ID,
				ProjectID: t.ProjectID,
				AuthorID:  author,
				Content:   content,
				ParentID:  parentID,
				CreatedAt: createdAt,
				UpdatedAt: createdAt,
			}); err == nil {
				count++
				parentID = &commentID
			} else {
				log.Printf("Warning: failed to create comment: %v", err)
			}
		}
	}
	log.Printf("Created %d comments across tasks", count)
}

func (s *Seeder) seedTaskDependencies(orgID string) {
	rows, err := s.db.Query("SELECT id FROM tasks WHERE org_id = ? AND parent_task_id IS NULL ORDER BY created_at", orgID)
	if err != nil {
		return
	}
	defer rows.Close()

	type taskInfo struct{ ID string }
	var tasks []taskInfo
	for rows.Next() {
		var t taskInfo
		if rows.Scan(&t.ID) == nil {
			tasks = append(tasks, t)
		}
	}

	// Raw SQL; domain.TaskDependency type doesn't exist; sqlc has AddTaskDependency
	// but no public store method exposes it.
	count := 0
	for i := 0; i < len(tasks)-2; i++ {
		if i%2 != 0 {
			continue
		}
		_, err := s.db.Exec(
			"INSERT OR IGNORE INTO task_dependencies (task_id, blocks_task_id) VALUES (?, ?)",
			tasks[i+1].ID, tasks[i].ID,
		)
		if err == nil {
			count++
		}
		if i+2 < len(tasks) {
			_, err := s.db.Exec(
				"INSERT OR IGNORE INTO task_dependencies (task_id, blocks_task_id) VALUES (?, ?)",
				tasks[i+2].ID, tasks[i].ID,
			)
			if err == nil {
				count++
			}
		}
	}
	log.Printf("Created %d task dependencies", count)
}

func (s *Seeder) seedTaskActivity(orgID, userID, userID2 string, now time.Time) {
	rows, err := s.db.Query(`
		SELECT t.id, t.project_id FROM tasks t
		WHERE t.org_id = ? AND t.parent_task_id IS NULL
		ORDER BY t.created_at LIMIT 30`, orgID)
	if err != nil {
		return
	}
	defer rows.Close()

	type taskInfo struct{ ID, ProjectID string }
	var tasks []taskInfo
	for rows.Next() {
		var t taskInfo
		if rows.Scan(&t.ID, &t.ProjectID) == nil {
			tasks = append(tasks, t)
		}
	}

	activities := []struct {
		Action, Field, OldVal, NewVal string
	}{
		{"status_changed", "status_id", "Backlog", "Todo"},
		{"status_changed", "status_id", "Todo", "In Progress"},
		{"status_changed", "status_id", "In Progress", "In Review"},
		{"status_changed", "status_id", "In Review", "Done"},
		{"priority_changed", "priority", "none", "medium"},
		{"priority_changed", "priority", "medium", "high"},
		{"assignee_added", "assignee", "", "user-1"},
		{"assignee_removed", "assignee", "user-1", ""},
		{"due_date_set", "due_at", "", "2026-01-15"},
		{"due_date_changed", "due_at", "2026-01-15", "2026-01-20"},
		{"estimate_set", "estimate", "", "4"},
		{"title_changed", "title", "Old title", "New title"},
	}

	count := 0
	for i, t := range tasks {
		numActivities := 2 + (i % 3)
		for j := 0; j < numActivities; j++ {
			actorID := userID
			if (i+j)%3 == 1 {
				actorID = userID2
			}
			act := activities[(i+j)%len(activities)]
			createdAt := now.Add(-time.Duration(numActivities-j) * 24 * time.Hour)
			if err := s.stores.TaskActivity.Create(s.ctx, &domain.TaskActivity{
				ID:        newUUID(),
				TaskID:    t.ID,
				OrgID:     orgID,
				ProjectID: t.ProjectID,
				ActorID:   actorID,
				Action:    domain.ActivityAction(act.Action),
				Field:     act.Field,
				OldValue:  act.OldVal,
				NewValue:  act.NewVal,
				CreatedAt: createdAt,
			}); err == nil {
				count++
			}
		}
	}
	log.Printf("Created %d task activity entries", count)
}

func (s *Seeder) seedAuditLog(orgID, userID string, now time.Time) {
	rows, err := s.db.Query("SELECT id, name FROM projects WHERE org_id = ?", orgID)
	if err != nil {
		return
	}
	defer rows.Close()

	type projInfo struct{ ID, Name string }
	var projects []projInfo
	for rows.Next() {
		var p projInfo
		if rows.Scan(&p.ID, &p.Name) == nil {
			projects = append(projects, p)
		}
	}

	actions := []string{
		"project.created", "project.updated", "task.created", "task.updated",
		"task.completed", "member.added", "member.removed", "cycle.completed",
		"status.created", "label.created",
	}
	entityTypes := []string{"project", "task", "member", "cycle", "status", "label"}

	count := 0
	for _, p := range projects {
		numEntries := 5 + rand.Intn(4)
		for j := 0; j < numEntries; j++ {
			action := actions[rand.Intn(len(actions))]
			entityType := entityTypes[rand.Intn(len(entityTypes))]
			metadata := `{"project":"` + p.Name + `"}`
			createdAt := now.Add(-time.Duration(rand.Intn(28)+1) * 24 * time.Hour)
			if err := s.stores.Audit.Create(s.ctx, &domain.AuditEntry{
				ID:         newUUID(),
				OrgID:      orgID,
				ActorID:    userID,
				Action:     domain.AuditAction(action),
				EntityType: entityType,
				EntityID:   p.ID,
				Metadata:   metadata,
				CreatedAt:  createdAt,
			}); err == nil {
				count++
			}
		}
	}
	log.Printf("Created %d audit log entries", count)
}

func (s *Seeder) seedNotifications(orgID, userID string, now time.Time) {
	notiTypes := []string{
		"task_assigned", "task_mentioned", "due_reminder",
		"cycle_started", "cycle_ending", "comment_reply", "task_completed",
	}
	count := 0
	for i := 0; i < 15; i++ {
		notiType := notiTypes[i%len(notiTypes)]
		isRead := i%3 == 0
		var readAt *time.Time
		if isRead {
			t := now.Add(-time.Duration(i) * time.Hour)
			readAt = &t
		}
		createdAt := now.Add(-time.Duration(rand.Intn(72)+1) * time.Hour)
		if err := s.stores.Notification.Create(s.ctx, &domain.Notification{
			ID:        newUUID(),
			OrgID:     orgID,
			UserID:    userID,
			Type:      domain.NotificationType(notiType),
			Title:     "Notification: " + notiType,
			Body:      "This is a sample notification body for testing.",
			IsRead:    isRead,
			ReadAt:    readAt,
			CreatedAt: createdAt,
		}); err == nil {
			count++
		}
	}
	log.Printf("Created %d notifications", count)
}

// seedCustomFieldValues populates custom field values for tasks in the first
// project (Website Redesign). Raw SQL; no store method for task_custom_field_values.
func (s *Seeder) seedCustomFieldValues(orgID string, now time.Time) {
	var storyPointsFieldID, departmentFieldID string
	rows, err := s.db.Query("SELECT id, name FROM custom_fields WHERE org_id = ?", orgID)
	if err != nil {
		return
	}
	for rows.Next() {
		var id, name string
		if rows.Scan(&id, &name) != nil {
			continue
		}
		switch name {
		case "Story Points":
			storyPointsFieldID = id
		case "Department":
			departmentFieldID = id
		}
	}
	rows.Close()

	taskRows, err := s.db.Query(`
		SELECT t.id FROM tasks t
		JOIN projects p ON t.project_id = p.id
		WHERE t.org_id = ? AND t.parent_task_id IS NULL AND p.name = 'Website Redesign'
	`, orgID)
	if err != nil {
		return
	}

	taskIDs := []string{}
	for taskRows.Next() {
		var taskID string
		if taskRows.Scan(&taskID) != nil {
			continue
		}
		taskIDs = append(taskIDs, taskID)
	}
	taskRows.Close()

	departments := []string{"Design", "Engineering", "Marketing", "QA"}
	storyPoints := []string{"1", "2", "3", "5", "8", "13"}
	count := 0
	for taskIdx, taskID := range taskIDs {
		if storyPointsFieldID != "" {
			spValue := storyPoints[taskIdx%len(storyPoints)]
			_, err := s.db.Exec(
				`INSERT OR IGNORE INTO task_custom_field_values (task_id, custom_field_id, value, updated_at) VALUES (?, ?, ?, ?)`,
				taskID, storyPointsFieldID, spValue, formatTime(now),
			)
			if err == nil {
				count++
			}
		}
		if departmentFieldID != "" {
			dept := departments[taskIdx%len(departments)]
			_, err := s.db.Exec(
				`INSERT OR IGNORE INTO task_custom_field_values (task_id, custom_field_id, value, updated_at) VALUES (?, ?, ?, ?)`,
				taskID, departmentFieldID, dept, formatTime(now),
			)
			if err == nil {
				count++
			}
		}
	}
	if count > 0 {
		log.Printf("Created %d custom field values", count)
	}
}

// ---- views ----

// createSampleViews creates global and project-specific saved views.
// Uses store View.Create method.
func (s *Seeder) createSampleViews(orgID, userID string) {
	if err := s.stores.View.Create(s.ctx, &domain.View{
		ID:        newUUID(),
		OrgID:     orgID,
		ProjectID: nil,
		CreatedBy: userID,
		Name:      "All Tasks",
		Layout:    domain.ViewLayout("board"),
		Filters:   domain.ViewFilters{},
	}); err != nil {
		log.Printf("Warning: failed to create global view: %v", err)
	} else {
		log.Println("  Created global view: All Tasks")
	}

	// Get a demo project ID
	var demoProjectID string
	err := s.db.QueryRow("SELECT id FROM projects WHERE name = 'Website Redesign' LIMIT 1").Scan(&demoProjectID)
	if err == nil && demoProjectID != "" {
		if err := s.stores.View.Create(s.ctx, &domain.View{
			ID:        newUUID(),
			OrgID:     orgID,
			ProjectID: &demoProjectID,
			CreatedBy: userID,
			Name:      "High Priority",
			Layout:    domain.ViewLayout("list"),
			Filters:   domain.ViewFilters{Priority: "high"},
		}); err != nil {
			log.Printf("Warning: failed to create project view: %v", err)
		} else {
			log.Println("  Created project view: High Priority")
		}
	}
}

// ---- chat seed ----

// seedChat creates sample categories, channels, permissions, and messages.
// Uses store Create for conversations and messages; raw SQL for members,
// permissions, and channel links where no store method exists.
func (s *Seeder) seedChat(orgID, userID, userID2, userID3 string) {
	log.Println("Seeding chat data...")

	// Create a category
	catID := newUUID()
	if err := s.stores.Conversation.Create(s.ctx, &domain.Conversation{
		ID:          catID,
		OrgID:       orgID,
		ParentID:    nil,
		Name:        "General",
		Type:        domain.ConversationType("category"),
		CreatedBy:   userID,
		PositionKey: "h",
		CreatedAt:   time.Now(),
		UpdatedAt:   time.Now(),
	}); err == nil {
		// Add members (raw SQL; ConversationStore.CreateWithMembers could do this,
		// but we need multiple member inserts for different convs)
		s.db.Exec(`INSERT INTO conversation_members (conversation_id, user_id, org_id, joined_at) VALUES (?, ?, ?, datetime('now'))`, catID, userID, orgID)
		s.db.Exec(`INSERT INTO conversation_members (conversation_id, user_id, org_id, joined_at) VALUES (?, ?, ?, datetime('now'))`, catID, userID2, orgID)
	}

	// Create workspace #general channel
	workspaceGenID := newUUID()
	if err := s.stores.Conversation.Create(s.ctx, &domain.Conversation{
		ID:          workspaceGenID,
		OrgID:       orgID,
		ParentID:    &catID,
		Name:        "general",
		Type:        domain.ConversationType("channel"),
		CreatedBy:   userID,
		PositionKey: "h",
		CreatedAt:   time.Now(),
		UpdatedAt:   time.Now(),
	}); err != nil {
		log.Printf("Warning: failed to create workspace channel: %v", err)
		return
	}
	s.db.Exec(`INSERT INTO conversation_members (conversation_id, user_id, org_id, joined_at) VALUES (?, ?, ?, datetime('now'))`, workspaceGenID, userID, orgID)
	s.db.Exec(`INSERT INTO conversation_members (conversation_id, user_id, org_id, joined_at) VALUES (?, ?, ?, datetime('now'))`, workspaceGenID, userID2, orgID)

	// Create project channel and link it
	var projectID string
	var projectName string
	_ = s.db.QueryRow("SELECT id, name FROM projects WHERE org_id = ? ORDER BY created_at LIMIT 1", orgID).Scan(&projectID, &projectName)
	if projectID != "" {
		projectChanName := slug(projectName)
		projectChanID := newUUID()
		if err := s.stores.Conversation.Create(s.ctx, &domain.Conversation{
			ID:          projectChanID,
			OrgID:       orgID,
			ParentID:    &catID,
			Name:        projectChanName,
			Type:        domain.ConversationType("channel"),
			CreatedBy:   userID,
			PositionKey: "n",
			CreatedAt:   time.Now(),
			UpdatedAt:   time.Now(),
		}); err == nil {
			s.db.Exec(`INSERT INTO conversation_members (conversation_id, user_id, org_id, joined_at) VALUES (?, ?, ?, datetime('now'))`, projectChanID, userID, orgID)
			s.db.Exec(`INSERT INTO conversation_members (conversation_id, user_id, org_id, joined_at) VALUES (?, ?, ?, datetime('now'))`, projectChanID, userID2, orgID)

			// Link channel to project
			if err := s.stores.ChannelProjectLink.Create(s.ctx, projectChanID, projectID); err != nil {
				log.Printf("Warning: failed to link channel to project: %v", err)
			}

			// Guest overrides (raw SQL; no store method for channel_user_overrides)
			s.db.Exec(`INSERT INTO channel_user_overrides (channel_id, user_id, permission, allow) VALUES (?, ?, 'channel:view', 1)`, projectChanID, userID3)
			s.db.Exec(`INSERT INTO channel_user_overrides (channel_id, user_id, permission, allow) VALUES (?, ?, 'channel:send', 1)`, projectChanID, userID3)

			// Add guest as viewer to the project
			var firstProjID string
			_ = s.db.QueryRow("SELECT id FROM projects WHERE org_id = ? ORDER BY created_at LIMIT 1", orgID).Scan(&firstProjID)
			if firstProjID != "" {
				s.db.Exec(`INSERT INTO project_members (project_id, user_id, role) VALUES (?, ?, 'viewer')`, firstProjID, userID3)
			}
		}
	}

	// Seed default permissions for the category (raw SQL; ChannelPermissionStore uses SetPermissions which is batch)
	// Using raw SQL for simplicity since we need individual per-role inserts.
	s.db.Exec(`INSERT INTO channel_permissions (channel_id, role, permission, allow) VALUES (?, 'everyone', 'channel:view', 1)`, catID)
	s.db.Exec(`INSERT INTO channel_permissions (channel_id, role, permission, allow) VALUES (?, 'everyone', 'channel:send', 1)`, catID)
	s.db.Exec(`INSERT INTO channel_permissions (channel_id, role, permission, allow) VALUES (?, 'member', 'channel:manage', 1)`, catID)
	s.db.Exec(`INSERT INTO channel_permissions (channel_id, role, permission, allow) VALUES (?, 'guest', 'channel:view', 1)`, catID)

	// Create a voice channel
	voiceChanID := newUUID()
	if err := s.stores.Conversation.Create(s.ctx, &domain.Conversation{
		ID:          voiceChanID,
		OrgID:       orgID,
		ParentID:    &catID,
		Name:        "Voice Chat",
		Type:        domain.ConversationType("voice"),
		CreatedBy:   userID,
		PositionKey: "t",
		CreatedAt:   time.Now(),
		UpdatedAt:   time.Now(),
	}); err != nil {
		log.Printf("Warning: failed to create voice channel: %v", err)
	} else {
		s.db.Exec(`INSERT INTO conversation_members (conversation_id, user_id, org_id, joined_at) VALUES (?, ?, ?, datetime('now'))`, voiceChanID, userID, orgID)
		s.db.Exec(`INSERT INTO conversation_members (conversation_id, user_id, org_id, joined_at) VALUES (?, ?, ?, datetime('now'))`, voiceChanID, userID2, orgID)
		log.Println("  Created voice channel: Voice Chat")
	}

	// Insert sample messages using store Create
	msgs := []struct {
		content string
		pinned  bool
	}{
		{"Welcome everyone to the workspace!", true},
		{"Has anyone seen the latest design mockups?", false},
		{"Check out this link: https://example.com/design", false},
		{"I uploaded the requirements doc.", false},
		{"Meeting at 3pm today.", false},
		{"@everyone please review the sprint board.", false},
		{"Great work on the landing page!", false},
		{"Found a bug in the checkout flow.", false},
		{"The API docs are updated.", false},
		{"Deploy is scheduled for Friday.", false},
		{"Can someone help with the auth refactor?", false},
		{"We need more tests for the search feature.", false},
	}
	for _, m := range msgs {
		msgID := newUUID()
		daysAgo := rand.Intn(30)
		createdAt := time.Now().Round(0).AddDate(0, 0, -daysAgo)
		if err := s.stores.Message.Create(s.ctx, &domain.Message{
			ID:             msgID,
			ConversationID: workspaceGenID,
			OrgID:          orgID,
			SenderID:       userID,
			Content:        m.content,
			CreatedAt:      createdAt,
		}); err != nil {
			log.Printf("Warning: failed to insert message: %v", err)
		}
		// Pin message separately (the store Create doesn't handle pinned flag)
		if m.pinned {
			s.db.Exec(`UPDATE messages SET pinned = 1 WHERE id = ?`, msgID)
		}
	}

	log.Println("  Seeded chat channels, messages, attachments metadata.")
}
