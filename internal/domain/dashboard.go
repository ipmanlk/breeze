package domain

import (
	"encoding/json"
	"strings"
	"time"
)

// DashboardData contains all data needed to render the home page dashboard.
type DashboardData struct {
	Sections []DashboardSectionData
}

// DashboardSectionData holds the data for a single dashboard section.
type DashboardSectionData struct {
	Type  SectionType
	Title string
	Data  any
}

// DashboardTask is a lightweight task representation for the dashboard.
type DashboardTask struct {
	ID          string
	Title       string
	Priority    string
	StatusID    string
	StatusName  string
	StatusColor string
	ProjectID   string
	ProjectName string
	ProjectSlug string
	DueAt       *time.Time
}

// DashboardStats holds aggregate counts for the current user.
type DashboardStats struct {
	AssignedCount    int
	OverdueCount     int
	DueThisWeekCount int
	CompletedCount   int
	TotalProjects    int
}

// DashboardActivity represents a recent notification in the activity feed.
type DashboardActivity struct {
	ID          string
	Type        string
	Title       string
	Body        string
	Link        string
	EntityType  string
	EntityID    string
	ActorName   string
	ProjectSlug string
	IsUnread    bool
	CreatedAt   time.Time
}

// DashboardProject is a project summary for the dashboard.
type DashboardProject struct {
	ID          string
	Name        string
	Slug        string
	Color       string
	Icon        string
	TaskCount   int
	MemberCount int
}

// SectionType represents a dashboard section that can be toggled and reordered.
type SectionType string

const (
	SectionMyTasks  SectionType = "my_tasks"
	SectionDueSoon  SectionType = "due_soon"
	SectionActivity SectionType = "activity"
	SectionStats    SectionType = "stats"
	SectionProjects SectionType = "projects"
)

var DefaultDashboardSections = []SectionType{
	SectionMyTasks,
	SectionDueSoon,
	SectionActivity,
	SectionStats,
	SectionProjects,
}

var SectionTitles = map[SectionType]string{
	SectionMyTasks:  "My Tasks",
	SectionDueSoon:  "Due Soon",
	SectionActivity: "Recent Activity",
	SectionStats:    "Your Stats",
	SectionProjects: "Projects",
}

// GetDashboardParams contains all parameters for GetDashboard.
type GetDashboardParams struct {
	OrgID    string
	UserID   string
	Role     Role // caller's org role; viewer/guest get membership-scoped sections
	Sections []SectionType
}

// SetVisibilityParams contains all parameters for SetVisibility.
type SetVisibilityParams struct {
	UserID   string
	OrgID    string
	Sections []SectionType
}

// DashboardVisibility is returned after updating dashboard preferences.
type DashboardVisibility struct {
	UserID   string
	Sections []SectionType
}

// --- Helpers for serializing section configs ---

func SectionTypesToStrings(sections []SectionType) []string {
	result := make([]string, len(sections))
	for i, s := range sections {
		result[i] = string(s)
	}
	return result
}

func SectionsToJSON(sections []SectionType) (string, error) {
	b, err := json.Marshal(SectionTypesToStrings(sections))
	if err != nil {
		return "", err
	}
	return string(b), nil
}

func SectionsFromJSON(raw string) ([]SectionType, error) {
	var names []string
	if raw == "" {
		return DefaultDashboardSections, nil
	}
	if err := json.Unmarshal([]byte(raw), &names); err != nil {
		return DefaultDashboardSections, nil
	}
	result := make([]SectionType, 0, len(names))
	for _, n := range names {
		n = strings.TrimSpace(n)
		if n != "" {
			result = append(result, SectionType(n))
		}
	}
	if len(result) == 0 {
		return DefaultDashboardSections, nil
	}
	return result, nil
}
