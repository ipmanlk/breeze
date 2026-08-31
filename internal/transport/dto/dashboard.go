package dto

import (
	"encoding/json"
	"time"

	"ipmanlk/plume/internal/domain"
)

// DashboardSectionResponse is the API representation of a single dashboard section.
type DashboardSectionResponse struct {
	Type  string `json:"type"`
	Title string `json:"title"`
	Data  any    `json:"data"`
}

// DashboardResponse is the API response for GET /api/dashboard.
type DashboardResponse struct {
	Sections []DashboardSectionResponse `json:"sections"`
}

// DashboardTaskResponse is the API representation of a task in dashboard lists.
type DashboardTaskResponse struct {
	ID          string  `json:"id"`
	Title       string  `json:"title"`
	Priority    string  `json:"priority"`
	StatusID    string  `json:"status_id"`
	StatusName  string  `json:"status_name"`
	StatusColor string  `json:"status_color"`
	ProjectID   string  `json:"project_id"`
	ProjectName string  `json:"project_name"`
	ProjectSlug string  `json:"project_slug"`
	DueAt       *string `json:"due_at,omitempty"`
}

// StatsResponse is the API representation of dashboard stats.
type StatsResponse struct {
	AssignedCount    int `json:"assigned_count"`
	OverdueCount     int `json:"overdue_count"`
	DueThisWeekCount int `json:"due_this_week_count"`
	CompletedCount   int `json:"completed_count"`
	TotalProjects    int `json:"total_projects"`
}

// ActivityResponse is the API representation of a dashboard activity.
type ActivityResponse struct {
	ID          string `json:"id"`
	Type        string `json:"type"`
	Title       string `json:"title"`
	Body        string `json:"body"`
	Link        string `json:"link"`
	EntityType  string `json:"entity_type"`
	EntityID    string `json:"entity_id"`
	ActorName   string `json:"actor_name"`
	ProjectSlug string `json:"project_slug"`
	IsUnread    bool   `json:"is_unread"`
	CreatedAt   string `json:"created_at"`
}

// ProjectSummaryResponse is the API representation of a project on the dashboard.
type ProjectSummaryResponse struct {
	ID          string `json:"id"`
	Name        string `json:"name"`
	Slug        string `json:"slug"`
	Color       string `json:"color"`
	Icon        string `json:"icon"`
	TaskCount   int    `json:"task_count"`
	MemberCount int    `json:"member_count"`
}

// VisibilityRequest is the request body for PATCH /api/dashboard/visibility.
type VisibilityRequest struct {
	Sections []string `json:"sections"`
}

// VisibilityResponse is the response for PATCH /api/dashboard/visibility.
type VisibilityResponse struct {
	Sections []string `json:"sections"`
}

// --- Constructors ---

func NewDashboardResponse(data *domain.DashboardData) (*DashboardResponse, error) {
	sections := make([]DashboardSectionResponse, 0, len(data.Sections))
	for _, sd := range data.Sections {
		var raw json.RawMessage
		var err error
		switch sd.Type {
		case domain.SectionMyTasks, domain.SectionDueSoon:
			tasks := convertTasks(sd.Data)
			raw, err = json.Marshal(tasks)
		case domain.SectionStats:
			stats := convertStats(sd.Data)
			raw, err = json.Marshal(stats)
		case domain.SectionActivity:
			activities := convertActivities(sd.Data)
			raw, err = json.Marshal(activities)
		case domain.SectionProjects:
			projects := convertProjects(sd.Data)
			raw, err = json.Marshal(projects)
		}
		if err != nil {
			return nil, err
		}
		sections = append(sections, DashboardSectionResponse{
			Type:  string(sd.Type),
			Title: sd.Title,
			Data:  raw,
		})
	}
	return &DashboardResponse{Sections: sections}, nil
}

func convertTasks(data any) []DashboardTaskResponse {
	tasks, ok := data.([]*domain.DashboardTask)
	if !ok {
		return []DashboardTaskResponse{}
	}
	result := make([]DashboardTaskResponse, len(tasks))
	for i, t := range tasks {
		result[i] = DashboardTaskResponse{
			ID:          t.ID,
			Title:       t.Title,
			Priority:    t.Priority,
			StatusID:    t.StatusID,
			StatusName:  t.StatusName,
			StatusColor: t.StatusColor,
			ProjectID:   t.ProjectID,
			ProjectName: t.ProjectName,
			ProjectSlug: t.ProjectSlug,
			DueAt:       timePtrToStr(t.DueAt),
		}
	}
	return result
}

func convertStats(data any) *StatsResponse {
	stats, ok := data.(*domain.DashboardStats)
	if !ok {
		return &StatsResponse{}
	}
	return &StatsResponse{
		AssignedCount:    stats.AssignedCount,
		OverdueCount:     stats.OverdueCount,
		DueThisWeekCount: stats.DueThisWeekCount,
		CompletedCount:   stats.CompletedCount,
		TotalProjects:    stats.TotalProjects,
	}
}

func convertActivities(data any) []ActivityResponse {
	activities, ok := data.([]*domain.DashboardActivity)
	if !ok {
		return []ActivityResponse{}
	}
	result := make([]ActivityResponse, len(activities))
	for i, a := range activities {
		result[i] = ActivityResponse{
			ID:          a.ID,
			Type:        a.Type,
			Title:       a.Title,
			Body:        a.Body,
			Link:        a.Link,
			EntityType:  a.EntityType,
			EntityID:    a.EntityID,
			ActorName:   a.ActorName,
			ProjectSlug: a.ProjectSlug,
			IsUnread:    a.IsUnread,
			CreatedAt:   a.CreatedAt.Format("2006-01-02T15:04:05Z07:00"),
		}
	}
	return result
}

func convertProjects(data any) []ProjectSummaryResponse {
	projects, ok := data.([]*domain.DashboardProject)
	if !ok {
		return []ProjectSummaryResponse{}
	}
	result := make([]ProjectSummaryResponse, len(projects))
	for i, p := range projects {
		result[i] = ProjectSummaryResponse{
			ID:          p.ID,
			Name:        p.Name,
			Slug:        p.Slug,
			Color:       p.Color,
			Icon:        p.Icon,
			TaskCount:   p.TaskCount,
			MemberCount: p.MemberCount,
		}
	}
	return result
}

func NewVisibilityResponse(v *domain.DashboardVisibility) *VisibilityResponse {
	return &VisibilityResponse{
		Sections: domain.SectionTypesToStrings(v.Sections),
	}
}

func timePtrToStr(t *time.Time) *string {
	if t == nil {
		return nil
	}
	s := t.Format("2006-01-02T15:04:05Z07:00")
	return &s
}
