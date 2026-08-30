// Package seed populates the database with sample data for development.
// It uses the store layer for most operations and the service layer where
// business logic is needed (templates, custom fields, views).
// Only clearData uses raw SQL; no service/store equivalent for 40-table DELETE.
package seed

import "time"

// Status represents a task status definition
type Status struct {
	ID       string
	Name     string
	Color    string
	Category string
	Position int
}

// TaskDef represents a task definition from the seed data
type TaskDef struct {
	Title       string
	Description string
	Priority    string
	StatusIdx   int
	Estimate    *int
	StartedAt   *time.Time
	DueAt       *time.Time
	CompletedAt *time.Time
	Subtasks    []SubtaskDef
	TimeEntries []TimeEntryDef
}

// SubtaskDef represents a subtask definition
type SubtaskDef struct {
	Title       string
	IsCompleted bool
}

// TimeEntryDef represents a time entry definition
type TimeEntryDef struct {
	Description string
	Minutes     int
	Date        time.Time
}

// ProjectDef represents a project definition
type ProjectDef struct {
	Name          string
	Color         string
	Icon          string
	WithCycles    bool
	CycleDuration *int
	Tasks         []TaskDef
}

// CycleInfo stores a created cycle's ID and date range for task assignment
type CycleInfo struct {
	ID       string
	StartsAt time.Time
	EndsAt   time.Time
}

// createStatusDefs returns the standard 6-status list for a project.
func createStatusDefs() []Status {
	return []Status{
		{ID: newUUID(), Name: "Backlog", Color: "#94a3b8", Category: "todo", Position: 0},
		{ID: newUUID(), Name: "Todo", Color: "#3b82f6", Category: "todo", Position: 1},
		{ID: newUUID(), Name: "In Progress", Color: "#f59e0b", Category: "in_progress", Position: 2},
		{ID: newUUID(), Name: "In Review", Color: "#8b5cf6", Category: "in_progress", Position: 3},
		{ID: newUUID(), Name: "Done", Color: "#22c55e", Category: "done", Position: 4},
		{ID: newUUID(), Name: "Canceled", Color: "#ef4444", Category: "canceled", Position: 5},
	}
}

// createProjectDefs creates all the project definitions from the original seed data.
func createProjectDefs(projectStart, now time.Time) []ProjectDef {
	return []ProjectDef{
		createWebsiteRedesignProject(projectStart, now),
		createMobileAppProject(projectStart, now),
		createBugBashProject(projectStart, now),
		createInternalToolsProject(projectStart, now),
	}
}

// findCycleForTask matches a task to the appropriate cycle based on dates.
func findCycleForTask(tDef TaskDef, cycles []CycleInfo) *string {
	// Try to match by started_at
	if tDef.StartedAt != nil {
		for _, c := range cycles {
			if tDef.StartedAt.Compare(c.StartsAt) >= 0 && tDef.StartedAt.Compare(c.EndsAt) <= 0 {
				return &c.ID
			}
		}
	}

	// Try to match by due_at
	if tDef.DueAt != nil {
		for _, c := range cycles {
			if tDef.DueAt.Compare(c.StartsAt) >= 0 && tDef.DueAt.Compare(c.EndsAt) <= 0 {
				return &c.ID
			}
		}
	}

	// Try to match by completed_at
	if tDef.CompletedAt != nil {
		for _, c := range cycles {
			if tDef.CompletedAt.Compare(c.StartsAt) >= 0 && tDef.CompletedAt.Compare(c.EndsAt) <= 0 {
				return &c.ID
			}
		}
	}

	// If started and not matched, assign to most recent cycle
	if tDef.StartedAt != nil {
		var lastCycle *CycleInfo
		for i := range cycles {
			if tDef.StartedAt.Compare(cycles[i].StartsAt) >= 0 {
				lastCycle = &cycles[i]
			}
		}
		if lastCycle != nil {
			return &lastCycle.ID
		}
	}

	return nil
}

func createWebsiteRedesignProject(projectStart, now time.Time) ProjectDef {
	cycleDuration := 14
	return ProjectDef{
		Name:          "Website Redesign",
		Color:         "oklch(0.6 0.2 250)",
		Icon:          "GlobeIcon",
		WithCycles:    true,
		CycleDuration: &cycleDuration,
		Tasks: []TaskDef{
			{
				Title:       "Design home page mockup",
				Description: "Create Figma mockups for the new home page layout with hero section, features grid, and footer.",
				Priority:    "high",
				StatusIdx:   4,
				Estimate:    intPtr(3),
				StartedAt:   timePtr(projectStart.Add(2 * 24 * time.Hour)),
				DueAt:       timePtr(projectStart.Add(8 * 24 * time.Hour)),
				CompletedAt: timePtr(projectStart.Add(7 * 24 * time.Hour)),
			},
			{
				Title:       "Implement responsive navigation",
				Description: "Build mobile-first navigation with hamburger menu on small screens and full menu on desktop.",
				Priority:    "high",
				StatusIdx:   4,
				Estimate:    intPtr(2),
				StartedAt:   timePtr(projectStart.Add(5 * 24 * time.Hour)),
				DueAt:       timePtr(projectStart.Add(12 * 24 * time.Hour)),
				CompletedAt: timePtr(projectStart.Add(11 * 24 * time.Hour)),
			},
			{
				Title:       "Set up CI/CD pipeline",
				Description: "Configure the CI pipeline for automated testing, linting, and deployment to staging.",
				Priority:    "medium",
				StatusIdx:   4,
				Estimate:    intPtr(1),
				StartedAt:   timePtr(projectStart.Add(3 * 24 * time.Hour)),
				DueAt:       timePtr(projectStart.Add(6 * 24 * time.Hour)),
				CompletedAt: timePtr(projectStart.Add(5 * 24 * time.Hour)),
			},
			{
				Title:       "Create reusable component library",
				Description: "Extract common UI patterns into a shared component library with Storybook documentation.",
				Priority:    "medium",
				StatusIdx:   3,
				Estimate:    intPtr(5),
				StartedAt:   timePtr(projectStart.Add(8 * 24 * time.Hour)),
				DueAt:       timePtr(projectStart.Add(20 * 24 * time.Hour)),
				CompletedAt: timePtr(projectStart.Add(19 * 24 * time.Hour)),
			},
			{
				Title:       "Implement dark mode support",
				Description: "Add theme switching with CSS custom properties and persist preference in localStorage.",
				Priority:    "low",
				StatusIdx:   2,
				Estimate:    intPtr(2),
				StartedAt:   timePtr(projectStart.Add(6 * 24 * time.Hour)),
				DueAt:       timePtr(projectStart.Add(18 * 24 * time.Hour)),
				CompletedAt: nil,
			},
			{
				Title:       "Build contact form with validation",
				Description: "Create a contact form with client-side and server-side validation, reCAPTCHA integration, and email notifications.",
				Priority:    "high",
				StatusIdx:   1,
				Estimate:    intPtr(3),
				StartedAt:   nil,
				DueAt:       timePtr(projectStart.Add(25 * 24 * time.Hour)),
				CompletedAt: nil,
			},
			{
				Title:       "Add analytics tracking",
				Description: "Integrate Plausible analytics for page views, custom events, and goal tracking.",
				Priority:    "low",
				StatusIdx:   0,
				Estimate:    intPtr(1),
				StartedAt:   nil,
				DueAt:       nil,
				CompletedAt: nil,
			},
			{
				Title:       "SEO optimization pass",
				Description: "Add meta tags, Open Graph images, structured data, and sitemap generation.",
				Priority:    "medium",
				StatusIdx:   0,
				Estimate:    intPtr(2),
				StartedAt:   nil,
				DueAt:       nil,
				CompletedAt: nil,
			},
			{
				Title:       "Performance audit and optimization",
				Description: "Run Lighthouse audit and optimize images, fonts, JS bundles, and caching strategy.",
				Priority:    "medium",
				StatusIdx:   1,
				Estimate:    intPtr(3),
				StartedAt:   nil,
				DueAt:       timePtr(now.Add(7 * 24 * time.Hour)),
				CompletedAt: nil,
			},
			{
				Title:       "Write end-to-end tests",
				Description: "Write Playwright tests for critical user flows: signup, login, search, and checkout.",
				Priority:    "high",
				StatusIdx:   2,
				Estimate:    intPtr(4),
				StartedAt:   timePtr(now.Add(-3 * 24 * time.Hour)),
				DueAt:       timePtr(now.Add(14 * 24 * time.Hour)),
				CompletedAt: nil,
			},
			{
				Title:       "Set up error monitoring",
				Description: "Configure Sentry for frontend error tracking with source maps and release tracking.",
				Priority:    "medium",
				StatusIdx:   1,
				Estimate:    intPtr(1),
				StartedAt:   nil,
				DueAt:       timePtr(now.Add(5 * 24 * time.Hour)),
				CompletedAt: nil,
			},
			{
				Title:       "Add loading states and skeletons",
				Description: "Implement skeleton loading states for all data-fetching views to improve perceived performance.",
				Priority:    "low",
				StatusIdx:   0,
				Estimate:    intPtr(2),
				StartedAt:   nil,
				DueAt:       nil,
				CompletedAt: nil,
			},
			{
				Title:       "Create 404 and error pages",
				Description: "Design custom 404, 500, and maintenance mode pages with branded illustrations.",
				Priority:    "low",
				StatusIdx:   4,
				Estimate:    intPtr(1),
				StartedAt:   timePtr(projectStart.Add(10 * 24 * time.Hour)),
				DueAt:       timePtr(projectStart.Add(14 * 24 * time.Hour)),
				CompletedAt: timePtr(projectStart.Add(13 * 24 * time.Hour)),
			},
			{
				Title:       "Implement search with fuzzy matching",
				Description: "Add client-side search with Fuse.js for instant filtering of documentation and blog posts.",
				Priority:    "medium",
				StatusIdx:   2,
				Estimate:    intPtr(3),
				StartedAt:   timePtr(now.Add(-5 * 24 * time.Hour)),
				DueAt:       timePtr(now.Add(10 * 24 * time.Hour)),
				CompletedAt: nil,
			},
			{
				Title:       "Build admin dashboard",
				Description: "Create an admin dashboard with charts for user growth, revenue, and system health metrics.",
				Priority:    "high",
				StatusIdx:   1,
				Estimate:    intPtr(5),
				StartedAt:   nil,
				DueAt:       timePtr(now.Add(21 * 24 * time.Hour)),
				CompletedAt: nil,
				TimeEntries: []TimeEntryDef{
					{Description: "Research chart library options", Minutes: 60, Date: now.Add(-4 * 24 * time.Hour)},
					{Description: "Set up dashboard layout", Minutes: 120, Date: now.Add(-3 * 24 * time.Hour)},
				},
			},
			{
				Title:       "Fix cross-browser layout bugs",
				Description: "Test and fix layout inconsistencies in Safari, Firefox, and Edge.",
				Priority:    "high",
				StatusIdx:   3,
				Estimate:    intPtr(2),
				StartedAt:   timePtr(now.Add(-2 * 24 * time.Hour)),
				DueAt:       timePtr(now.Add(3 * 24 * time.Hour)),
				CompletedAt: nil,
				Subtasks: []SubtaskDef{
					{Title: "Fix Safari flexbox gap bug", IsCompleted: true},
					{Title: "Fix Firefox grid overflow", IsCompleted: false},
					{Title: "Fix Edge scrollbar styling", IsCompleted: false},
				},
			},
			{
				Title:       "Add PWA support",
				Description: "Add service worker, manifest.json, and offline support for progressive web app capabilities.",
				Priority:    "low",
				StatusIdx:   0,
				Estimate:    intPtr(3),
				StartedAt:   nil,
				DueAt:       nil,
				CompletedAt: nil,
			},
			{
				Title:       "Implement keyboard shortcuts",
				Description: "Add keyboard navigation shortcuts for power users: g+d for dashboard, g+p for projects, ? for help overlay.",
				Priority:    "low",
				StatusIdx:   0,
				Estimate:    intPtr(1),
				StartedAt:   nil,
				DueAt:       nil,
				CompletedAt: nil,
			},
			{
				Title:       "Migrate to TypeScript",
				Description: "Convert all JavaScript files to TypeScript with strict mode and proper type definitions.",
				Priority:    "medium",
				StatusIdx:   1,
				Estimate:    intPtr(4),
				StartedAt:   nil,
				DueAt:       timePtr(now.Add(30 * 24 * time.Hour)),
				CompletedAt: nil,
			},
			{
				Title:       "Write API documentation",
				Description: "Document all REST API endpoints with request/response examples using OpenAPI/Swagger.",
				Priority:    "low",
				StatusIdx:   0,
				Estimate:    intPtr(3),
				StartedAt:   nil,
				DueAt:       nil,
				CompletedAt: nil,
			},
			{
				Title:       "Set up automated backups",
				Description: "Configure daily database backups to S3 with 30-day retention policy.",
				Priority:    "high",
				StatusIdx:   3,
				Estimate:    intPtr(1),
				StartedAt:   timePtr(now.Add(-1 * 24 * time.Hour)),
				DueAt:       timePtr(now.Add(2 * 24 * time.Hour)),
				CompletedAt: nil,
			},
			{
				Title:       "Implement rate limiting",
				Description: "Add rate limiting middleware for API endpoints to prevent abuse.",
				Priority:    "high",
				StatusIdx:   2,
				Estimate:    intPtr(1),
				StartedAt:   timePtr(now.Add(-6 * 24 * time.Hour)),
				DueAt:       timePtr(now.Add(1 * 24 * time.Hour)),
				CompletedAt: nil,
			},
		},
	}
}

func createMobileAppProject(projectStart, now time.Time) ProjectDef {
	cycleDuration := 14
	return ProjectDef{
		Name:          "Mobile App",
		Color:         "oklch(0.65 0.25 160)",
		Icon:          "SmartphoneIcon",
		WithCycles:    true,
		CycleDuration: &cycleDuration,
		Tasks: []TaskDef{
			{
				Title:       "Set up React Native project",
				Description: "Initialize React Native project with TypeScript, ESLint, and Prettier configuration.",
				Priority:    "high",
				StatusIdx:   4,
				Estimate:    intPtr(1),
				StartedAt:   timePtr(projectStart.Add(1 * 24 * time.Hour)),
				DueAt:       timePtr(projectStart.Add(3 * 24 * time.Hour)),
				CompletedAt: timePtr(projectStart.Add(2 * 24 * time.Hour)),
			},
			{
				Title:       "Design app icon and splash screen",
				Description: "Create app icon variants for iOS and Android plus animated splash screen.",
				Priority:    "medium",
				StatusIdx:   4,
				Estimate:    intPtr(2),
				StartedAt:   timePtr(projectStart.Add(3 * 24 * time.Hour)),
				DueAt:       timePtr(projectStart.Add(8 * 24 * time.Hour)),
				CompletedAt: timePtr(projectStart.Add(7 * 24 * time.Hour)),
			},
			{
				Title:       "Build authentication screens",
				Description: "Implement login, signup, and password reset screens with form validation.",
				Priority:    "high",
				StatusIdx:   4,
				Estimate:    intPtr(3),
				StartedAt:   timePtr(projectStart.Add(5 * 24 * time.Hour)),
				DueAt:       timePtr(projectStart.Add(14 * 24 * time.Hour)),
				CompletedAt: timePtr(projectStart.Add(12 * 24 * time.Hour)),
			},
			{
				Title:       "Implement bottom tab navigation",
				Description: "Set up React Navigation with bottom tabs for main sections: Home, Search, Profile, Settings.",
				Priority:    "high",
				StatusIdx:   4,
				Estimate:    intPtr(2),
				StartedAt:   timePtr(projectStart.Add(7 * 24 * time.Hour)),
				DueAt:       timePtr(projectStart.Add(15 * 24 * time.Hour)),
				CompletedAt: timePtr(projectStart.Add(14 * 24 * time.Hour)),
			},
			{
				Title:       "Build home feed with pull-to-refresh",
				Description: "Create scrollable home feed with pull-to-refresh and infinite scroll pagination.",
				Priority:    "high",
				StatusIdx:   3,
				Estimate:    intPtr(3),
				StartedAt:   timePtr(projectStart.Add(12 * 24 * time.Hour)),
				DueAt:       timePtr(projectStart.Add(22 * 24 * time.Hour)),
				CompletedAt: nil,
			},
			{
				Title:       "Add push notifications",
				Description: "Integrate Firebase Cloud Messaging for push notifications with user preferences.",
				Priority:    "medium",
				StatusIdx:   2,
				Estimate:    intPtr(4),
				StartedAt:   timePtr(projectStart.Add(15 * 24 * time.Hour)),
				DueAt:       timePtr(projectStart.Add(28 * 24 * time.Hour)),
				CompletedAt: nil,
			},
			{
				Title:       "Implement offline-first with WatermelonDB",
				Description: "Set up WatermelonDB for local data persistence with sync engine.",
				Priority:    "medium",
				StatusIdx:   1,
				Estimate:    intPtr(5),
				StartedAt:   nil,
				DueAt:       timePtr(projectStart.Add(35 * 24 * time.Hour)),
				CompletedAt: nil,
			},
			{
				Title:       "Build camera and image picker",
				Description: "Integrate camera module for photo capture and device gallery image picker.",
				Priority:    "low",
				StatusIdx:   1,
				Estimate:    intPtr(2),
				StartedAt:   nil,
				DueAt:       timePtr(projectStart.Add(40 * 24 * time.Hour)),
				CompletedAt: nil,
			},
			{
				Title:       "Add biometric authentication",
				Description: "Implement fingerprint and Face ID login for enhanced security.",
				Priority:    "high",
				StatusIdx:   2,
				Estimate:    intPtr(2),
				StartedAt:   timePtr(now.Add(-10 * 24 * time.Hour)),
				DueAt:       timePtr(now.Add(-2 * 24 * time.Hour)),
				CompletedAt: nil,
			},
			{
				Title:       "Build settings screen",
				Description: "Create settings screen with theme toggle, notification preferences, and account management.",
				Priority:    "low",
				StatusIdx:   0,
				Estimate:    intPtr(2),
				StartedAt:   nil,
				DueAt:       nil,
				CompletedAt: nil,
			},
			{
				Title:       "Implement deep linking",
				Description: "Configure deep linking for sharing content and password reset flows.",
				Priority:    "medium",
				StatusIdx:   0,
				Estimate:    intPtr(2),
				StartedAt:   nil,
				DueAt:       nil,
				CompletedAt: nil,
			},
			{
				Title:       "Add crash reporting with Sentry",
				Description: "Integrate Sentry SDK for crash reporting with user context and breadcrumbs.",
				Priority:    "high",
				StatusIdx:   1,
				Estimate:    intPtr(1),
				StartedAt:   nil,
				DueAt:       timePtr(now.Add(7 * 24 * time.Hour)),
				CompletedAt: nil,
			},
			{
				Title:       "Build search with recent history",
				Description: "Implement search screen with recent searches, trending tags, and autocomplete.",
				Priority:    "medium",
				StatusIdx:   2,
				Estimate:    intPtr(3),
				StartedAt:   timePtr(now.Add(-7 * 24 * time.Hour)),
				DueAt:       timePtr(now.Add(5 * 24 * time.Hour)),
				CompletedAt: nil,
				TimeEntries: []TimeEntryDef{
					{Description: "Implement search API integration", Minutes: 90, Date: now.Add(-5 * 24 * time.Hour)},
					{Description: "Build recent searches persistence", Minutes: 45, Date: now.Add(-4 * 24 * time.Hour)},
					{Description: "Add autocomplete debounce", Minutes: 30, Date: now.Add(-3 * 24 * time.Hour)},
				},
			},
			{
				Title:       "Add dark mode support",
				Description: "Implement dark mode with system preference detection and manual override.",
				Priority:    "low",
				StatusIdx:   0,
				Estimate:    intPtr(2),
				StartedAt:   nil,
				DueAt:       nil,
				CompletedAt: nil,
			},
			{
				Title:       "Write UI component tests",
				Description: "Write Jest + React Native Testing Library tests for all shared components.",
				Priority:    "medium",
				StatusIdx:   1,
				Estimate:    intPtr(4),
				StartedAt:   nil,
				DueAt:       timePtr(now.Add(14 * 24 * time.Hour)),
				CompletedAt: nil,
			},
			{
				Title:       "Implement gesture-based navigation",
				Description: "Add swipe gestures for back navigation, dismiss modals, and delete items.",
				Priority:    "low",
				StatusIdx:   0,
				Estimate:    intPtr(3),
				StartedAt:   nil,
				DueAt:       nil,
				CompletedAt: nil,
			},
			{
				Title:       "Build onboarding flow",
				Description: "Create multi-step onboarding with illustrations, permissions request, and personalization.",
				Priority:    "medium",
				StatusIdx:   2,
				Estimate:    intPtr(3),
				StartedAt:   timePtr(now.Add(-4 * 24 * time.Hour)),
				DueAt:       timePtr(now.Add(12 * 24 * time.Hour)),
				CompletedAt: nil,
			},
			{
				Title:       "Add widget support (iOS)",
				Description: "Create iOS widgets for home screen showing key metrics and quick actions.",
				Priority:    "low",
				StatusIdx:   0,
				Estimate:    intPtr(4),
				StartedAt:   nil,
				DueAt:       nil,
				CompletedAt: nil,
			},
			{
				Title:       "Implement share extension",
				Description: "Build iOS share extension and Android intent filter for sharing content from other apps.",
				Priority:    "low",
				StatusIdx:   0,
				Estimate:    intPtr(3),
				StartedAt:   nil,
				DueAt:       nil,
				CompletedAt: nil,
			},
			{
				Title:       "Performance optimization pass",
				Description: "Profile and optimize render performance, memory usage, and cold start time.",
				Priority:    "high",
				StatusIdx:   1,
				Estimate:    intPtr(3),
				StartedAt:   nil,
				DueAt:       timePtr(now.Add(20 * 24 * time.Hour)),
				CompletedAt: nil,
			},
		},
	}
}

func createBugBashProject(projectStart, now time.Time) ProjectDef {
	return ProjectDef{
		Name:       "Bug Bash",
		Color:      "oklch(0.6 0.2 20)",
		Icon:       "BugIcon",
		WithCycles: false,
		Tasks: []TaskDef{
			{
				Title:       "Fix login session timeout not redirecting",
				Description: "When session expires, user should be redirected to login page instead of getting a blank screen.",
				Priority:    "urgent",
				StatusIdx:   4,
				Estimate:    intPtr(1),
				StartedAt:   timePtr(projectStart.Add(2 * 24 * time.Hour)),
				DueAt:       timePtr(projectStart.Add(4 * 24 * time.Hour)),
				CompletedAt: timePtr(projectStart.Add(3 * 24 * time.Hour)),
			},
			{
				Title:       "Correct date formatting on task cards",
				Description: "Dates show as ISO strings instead of localized format in Firefox.",
				Priority:    "high",
				StatusIdx:   4,
				Estimate:    intPtr(1),
				StartedAt:   timePtr(projectStart.Add(3 * 24 * time.Hour)),
				DueAt:       timePtr(projectStart.Add(5 * 24 * time.Hour)),
				CompletedAt: timePtr(projectStart.Add(4 * 24 * time.Hour)),
			},
			{
				Title:       "Drag and drop broken on mobile viewports",
				Description: "Kanban drag and drop triggers page scroll instead of moving cards on touch devices.",
				Priority:    "high",
				StatusIdx:   3,
				Estimate:    intPtr(2),
				StartedAt:   timePtr(projectStart.Add(5 * 24 * time.Hour)),
				DueAt:       timePtr(projectStart.Add(10 * 24 * time.Hour)),
				CompletedAt: nil,
			},
			{
				Title:       "Notification count badge not updating",
				Description: "After reading notifications, the badge count stays the same until page refresh.",
				Priority:    "medium",
				StatusIdx:   4,
				Estimate:    intPtr(1),
				StartedAt:   timePtr(projectStart.Add(4 * 24 * time.Hour)),
				DueAt:       timePtr(projectStart.Add(7 * 24 * time.Hour)),
				CompletedAt: timePtr(projectStart.Add(6 * 24 * time.Hour)),
			},
			{
				Title:       "File upload fails for large images",
				Description: "Images over 5MB fail silently with no error message to the user.",
				Priority:    "high",
				StatusIdx:   4,
				Estimate:    intPtr(2),
				StartedAt:   timePtr(projectStart.Add(7 * 24 * time.Hour)),
				DueAt:       timePtr(projectStart.Add(11 * 24 * time.Hour)),
				CompletedAt: timePtr(projectStart.Add(10 * 24 * time.Hour)),
			},
			{
				Title:       "Search returns duplicate results",
				Description: "Searching for keywords returns duplicate entries when items are in multiple categories.",
				Priority:    "medium",
				StatusIdx:   2,
				Estimate:    intPtr(2),
				StartedAt:   timePtr(now.Add(-12 * 24 * time.Hour)),
				DueAt:       timePtr(now.Add(-3 * 24 * time.Hour)),
				CompletedAt: nil,
			},
			{
				Title:       "Email notification contains broken links",
				Description: "Links in notification emails point to staging URLs instead of production.",
				Priority:    "urgent",
				StatusIdx:   4,
				Estimate:    intPtr(1),
				StartedAt:   timePtr(now.Add(-8 * 24 * time.Hour)),
				DueAt:       timePtr(now.Add(-6 * 24 * time.Hour)),
				CompletedAt: timePtr(now.Add(-6 * 24 * time.Hour)),
			},
			{
				Title:       "Pagination resets on data refresh",
				Description: "When data refreshes automatically, pagination jumps back to page 1.",
				Priority:    "medium",
				StatusIdx:   1,
				Estimate:    intPtr(2),
				StartedAt:   nil,
				DueAt:       timePtr(now.Add(5 * 24 * time.Hour)),
				CompletedAt: nil,
			},
			{
				Title:       "GraphQL query returns 500 for empty result sets",
				Description: "Some GraphQL queries crash the server when the result set is empty.",
				Priority:    "urgent",
				StatusIdx:   3,
				Estimate:    intPtr(1),
				StartedAt:   timePtr(now.Add(-3 * 24 * time.Hour)),
				DueAt:       timePtr(now.Add(1 * 24 * time.Hour)),
				CompletedAt: nil,
				Subtasks: []SubtaskDef{
					{Title: "Identify failing queries", IsCompleted: true},
					{Title: "Add null checks to resolvers", IsCompleted: true},
					{Title: "Add integration tests", IsCompleted: false},
				},
			},
			{
				Title:       "CSS bleeding between components",
				Description: "Component styles leak into other parts of the application due to missing CSS modules.",
				Priority:    "high",
				StatusIdx:   2,
				Estimate:    intPtr(3),
				StartedAt:   timePtr(now.Add(-5 * 24 * time.Hour)),
				DueAt:       timePtr(now.Add(4 * 24 * time.Hour)),
				CompletedAt: nil,
				TimeEntries: []TimeEntryDef{
					{Description: "Audit all component styles", Minutes: 120, Date: now.Add(-4 * 24 * time.Hour)},
					{Description: "Fix Button component CSS", Minutes: 60, Date: now.Add(-3 * 24 * time.Hour)},
				},
			},
			{
				Title:       "Keyboard shortcuts conflict with form inputs",
				Description: "When typing in input fields, keyboard shortcuts trigger navigation actions.",
				Priority:    "medium",
				StatusIdx:   1,
				Estimate:    intPtr(1),
				StartedAt:   nil,
				DueAt:       timePtr(now.Add(6 * 24 * time.Hour)),
				CompletedAt: nil,
			},
			{
				Title:       "Mobile menu doesn't close on route change",
				Description: "The mobile navigation menu stays open after clicking a link.",
				Priority:    "low",
				StatusIdx:   4,
				Estimate:    intPtr(1),
				StartedAt:   timePtr(now.Add(-10 * 24 * time.Hour)),
				DueAt:       timePtr(now.Add(-8 * 24 * time.Hour)),
				CompletedAt: timePtr(now.Add(-8 * 24 * time.Hour)),
			},
			{
				Title:       "Image lazy loading causes layout shift",
				Description: "Images without dimensions cause content to jump when they load.",
				Priority:    "medium",
				StatusIdx:   1,
				Estimate:    intPtr(1),
				StartedAt:   nil,
				DueAt:       timePtr(now.Add(8 * 24 * time.Hour)),
				CompletedAt: nil,
			},
			{
				Title:       "Form validation errors not accessible",
				Description: "Validation error messages are not associated with form inputs via aria attributes.",
				Priority:    "high",
				StatusIdx:   2,
				Estimate:    intPtr(2),
				StartedAt:   timePtr(now.Add(-2 * 24 * time.Hour)),
				DueAt:       timePtr(now.Add(7 * 24 * time.Hour)),
				CompletedAt: nil,
			},
			{
				Title:       "Infinite loop in useEffect dependencies",
				Description: "React component enters infinite render loop due to missing dependency array.",
				Priority:    "urgent",
				StatusIdx:   3,
				Estimate:    intPtr(1),
				StartedAt:   timePtr(now.Add(-1 * 24 * time.Hour)),
				DueAt:       timePtr(now.Add(2 * 24 * time.Hour)),
				CompletedAt: nil,
			},
			{
				Title:       "Timezone handling incorrect for UTC offsets",
				Description: "Date displays are off by one hour for timezones with half-hour offsets like UTC+5:30.",
				Priority:    "medium",
				StatusIdx:   0,
				Estimate:    intPtr(2),
				StartedAt:   nil,
				DueAt:       nil,
				CompletedAt: nil,
			},
			{
				Title:       "Memory leak in WebSocket connection",
				Description: "WebSocket connections are not properly cleaned up on component unmount.",
				Priority:    "high",
				StatusIdx:   2,
				Estimate:    intPtr(1),
				StartedAt:   timePtr(now.Add(-4 * 24 * time.Hour)),
				DueAt:       timePtr(now.Add(3 * 24 * time.Hour)),
				CompletedAt: nil,
			},
			{
				Title:       "API rate limiter too aggressive",
				Description: "Legitimate requests get rate-limited during normal usage patterns.",
				Priority:    "medium",
				StatusIdx:   0,
				Estimate:    intPtr(1),
				StartedAt:   nil,
				DueAt:       nil,
				CompletedAt: nil,
			},
		},
	}
}

func createInternalToolsProject(projectStart, now time.Time) ProjectDef {
	return ProjectDef{
		Name:       "Internal Tools",
		Color:      "oklch(0.6 0.15 300)",
		Icon:       "WrenchIcon",
		WithCycles: false,
		Tasks: []TaskDef{
			{
				Title:       "Build deployment dashboard",
				Description: "Create a dashboard showing deployment status, history, and rollback options for all environments.",
				Priority:    "high",
				StatusIdx:   4,
				Estimate:    intPtr(4),
				StartedAt:   timePtr(projectStart.Add(3 * 24 * time.Hour)),
				DueAt:       timePtr(projectStart.Add(15 * 24 * time.Hour)),
				CompletedAt: timePtr(projectStart.Add(14 * 24 * time.Hour)),
			},
			{
				Title:       "Create log aggregation tool",
				Description: "Build a centralized log viewer with filtering, search, and export capabilities.",
				Priority:    "high",
				StatusIdx:   3,
				Estimate:    intPtr(5),
				StartedAt:   timePtr(projectStart.Add(10 * 24 * time.Hour)),
				DueAt:       timePtr(projectStart.Add(25 * 24 * time.Hour)),
				CompletedAt: nil,
			},
			{
				Title:       "Automate employee onboarding script",
				Description: "Write script to create accounts, assign permissions, and send welcome email for new hires.",
				Priority:    "medium",
				StatusIdx:   4,
				Estimate:    intPtr(2),
				StartedAt:   timePtr(projectStart.Add(5 * 24 * time.Hour)),
				DueAt:       timePtr(projectStart.Add(12 * 24 * time.Hour)),
				CompletedAt: timePtr(projectStart.Add(11 * 24 * time.Hour)),
			},
			{
				Title:       "Build internal CLI tool for DB migrations",
				Description: "Go CLI tool for running, rolling back, and inspecting database migrations across environments.",
				Priority:    "medium",
				StatusIdx:   4,
				Estimate:    intPtr(3),
				StartedAt:   timePtr(projectStart.Add(8 * 24 * time.Hour)),
				DueAt:       timePtr(projectStart.Add(18 * 24 * time.Hour)),
				CompletedAt: timePtr(projectStart.Add(17 * 24 * time.Hour)),
			},
			{
				Title:       "Create S3 bucket management UI",
				Description: "Web interface for managing S3 buckets: create, delete, set policies, and view usage metrics.",
				Priority:    "low",
				StatusIdx:   1,
				Estimate:    intPtr(4),
				StartedAt:   nil,
				DueAt:       timePtr(now.Add(14 * 24 * time.Hour)),
				CompletedAt: nil,
			},
			{
				Title:       "Build health check monitoring system",
				Description: "System that pings all services every minute and alerts on failures via a notification channel.",
				Priority:    "high",
				StatusIdx:   2,
				Estimate:    intPtr(3),
				StartedAt:   timePtr(now.Add(-10 * 24 * time.Hour)),
				DueAt:       timePtr(now.Add(5 * 24 * time.Hour)),
				CompletedAt: nil,
				TimeEntries: []TimeEntryDef{
					{Description: "Set up health check endpoints", Minutes: 90, Date: now.Add(-8 * 24 * time.Hour)},
					{Description: "Build notification integration", Minutes: 60, Date: now.Add(-7 * 24 * time.Hour)},
					{Description: "Add uptime reporting", Minutes: 45, Date: now.Add(-6 * 24 * time.Hour)},
				},
			},
			{
				Title:       "Create automated testing dashboard",
				Description: "Dashboard showing test results, coverage trends, and flaky test detection.",
				Priority:    "medium",
				StatusIdx:   1,
				Estimate:    intPtr(3),
				StartedAt:   nil,
				DueAt:       timePtr(now.Add(12 * 24 * time.Hour)),
				CompletedAt: nil,
			},
			{
				Title:       "Build feature flag management tool",
				Description: "Internal tool for managing feature flags with percentage rollouts and user targeting.",
				Priority:    "high",
				StatusIdx:   2,
				Estimate:    intPtr(4),
				StartedAt:   timePtr(now.Add(-6 * 24 * time.Hour)),
				DueAt:       timePtr(now.Add(10 * 24 * time.Hour)),
				CompletedAt: nil,
			},
			{
				Title:       "Write infrastructure documentation",
				Description: "Document all infrastructure components, networking, and disaster recovery procedures.",
				Priority:    "low",
				StatusIdx:   0,
				Estimate:    intPtr(3),
				StartedAt:   nil,
				DueAt:       nil,
				CompletedAt: nil,
			},
			{
				Title:       "Create runbook templates",
				Description: "Standardized runbook templates for incident response with checklists and escalation paths.",
				Priority:    "low",
				StatusIdx:   0,
				Estimate:    intPtr(2),
				StartedAt:   nil,
				DueAt:       nil,
				CompletedAt: nil,
			},
			{
				Title:       "Build on-call schedule management",
				Description: "Tool for managing on-call rotations with calendar integration and escalation policies.",
				Priority:    "medium",
				StatusIdx:   1,
				Estimate:    intPtr(5),
				StartedAt:   nil,
				DueAt:       timePtr(now.Add(21 * 24 * time.Hour)),
				CompletedAt: nil,
			},
			{
				Title:       "Set up automated dependency updates",
				Description: "Configure Renovate bot for automated dependency PRs with security vulnerability checks.",
				Priority:    "medium",
				StatusIdx:   4,
				Estimate:    intPtr(1),
				StartedAt:   timePtr(now.Add(-14 * 24 * time.Hour)),
				DueAt:       timePtr(now.Add(-10 * 24 * time.Hour)),
				CompletedAt: timePtr(now.Add(-10 * 24 * time.Hour)),
			},
			{
				Title:       "Create backup verification tool",
				Description: "Script that verifies database backups by restoring them to a test environment and running integrity checks.",
				Priority:    "high",
				StatusIdx:   2,
				Estimate:    intPtr(2),
				StartedAt:   timePtr(now.Add(-5 * 24 * time.Hour)),
				DueAt:       timePtr(now.Add(4 * 24 * time.Hour)),
				CompletedAt: nil,
			},
			{
				Title:       "Build API usage analytics",
				Description: "Track and visualize API usage by endpoint, user, and time period for capacity planning.",
				Priority:    "low",
				StatusIdx:   0,
				Estimate:    intPtr(3),
				StartedAt:   nil,
				DueAt:       nil,
				CompletedAt: nil,
			},
			{
				Title:       "Create incident postmortem tool",
				Description: "Template-based postmortem creation tool with notification-channel integration for tracking action items.",
				Priority:    "medium",
				StatusIdx:   1,
				Estimate:    intPtr(3),
				StartedAt:   nil,
				DueAt:       timePtr(now.Add(18 * 24 * time.Hour)),
				CompletedAt: nil,
			},
			{
				Title:       "Build cost optimization dashboard",
				Description: "Dashboard showing cloud spend by service, team, and project with optimization recommendations.",
				Priority:    "low",
				StatusIdx:   0,
				Estimate:    intPtr(4),
				StartedAt:   nil,
				DueAt:       nil,
				CompletedAt: nil,
			},
			{
				Title:       "Set up automated user provisioning",
				Description: "SCIM-based user provisioning for automated account creation and deactivation.",
				Priority:    "medium",
				StatusIdx:   1,
				Estimate:    intPtr(3),
				StartedAt:   nil,
				DueAt:       timePtr(now.Add(25 * 24 * time.Hour)),
				CompletedAt: nil,
			},
		},
	}
}
