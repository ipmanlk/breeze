package domain

import "time"

// TaskTemplate is a reusable task definition that can be manually instantiated
// or auto-generated on a recurrence schedule.
type TaskTemplate struct {
	ID                string
	OrgID             string
	ProjectID         string
	Name              string
	Description       string
	Priority          string
	StatusID          string
	AssigneeIDs       []string
	Estimate          *int
	RecurrencePattern string // "none", "daily", "weekly", "monthly"
	RecurrenceDays    string // weekly: "0,1,2" (0=Sun); monthly: "15"
	NextRunAt         *time.Time
	LastError         string // last instantiation error (empty/"" when ok)
	LastErrorAt       *time.Time
	CreatedBy         string
	CreatedAt         time.Time
	UpdatedAt         time.Time
}

type CreateTaskTemplateParams struct {
	OrgID             string
	ProjectID         string
	Name              string
	Description       string
	Priority          string
	StatusID          string
	AssigneeIDs       []string
	Estimate          *int
	RecurrencePattern string
	RecurrenceDays    string
	CreatedBy         string
}

type UpdateTaskTemplateParams struct {
	ID                string
	OrgID             string
	ProjectID         string
	Name              string
	Description       string
	Priority          string
	StatusID          string
	AssigneeIDs       []string
	Estimate          *int
	RecurrencePattern string
	RecurrenceDays    string
}

// RecurrencePattern constants
const (
	RecurrenceNone    = "none"
	RecurrenceDaily   = "daily"
	RecurrenceWeekly  = "weekly"
	RecurrenceMonthly = "monthly"
)
