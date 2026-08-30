package domain

import "time"

type Project struct {
	ID                     string
	OrgID                  string
	Name                   string
	Description            string
	Slug                   string
	Color                  string
	Icon                   string
	CreatedBy              string
	CycleDuration          *int
	AutoGenerateCycles     bool
	IncompleteTaskHandling CycleCompletionHandling
	StartsAt               *time.Time
	EndsAt                 *time.Time
	IsArchived             bool
	CreatedAt              time.Time
	UpdatedAt              time.Time
}

type Cycle struct {
	ID                 string
	OrgID              string
	ProjectID          string
	Name               string
	Goal               string
	StartsAt           time.Time
	EndsAt             time.Time
	CreatedBy          string
	IsCompleted        bool
	IsActive           bool
	TaskCount          int
	CompletedTaskCount int
	CreatedAt          time.Time
	UpdatedAt          time.Time
}

// CycleTaskCount holds the total/completed task counts for a single cycle.
// Used by the batched CountTasksByCycles store query.
type CycleTaskCount struct {
	Total     int64
	Completed int64
}

// CycleCompletionPlan carries the resolved plan for atomically completing a cycle.
// The service computes all decisions (what to do with incomplete tasks, whether
// to create a new cycle, which cycle to activate) and passes them to the store
// layer which executes them in a single transaction.
type CycleCompletionPlan struct {
	OrgID             string
	ProjectID         string
	CompletedCycleID  string
	CompletedCycle    Cycle  // with updated IsCompleted/IsActive/UpdatedAt
	NewCycle          *Cycle // non-nil when auto-generating
	MoveTargetCycleID string // empty means unassign incomplete tasks
	SetActiveCycleID  string // optional, cycle to reactivate after deactivation
}

type CycleCompletionHandling string

const (
	CycleHandlingNextCycle CycleCompletionHandling = "next_cycle"
	CycleHandlingBacklog   CycleCompletionHandling = "backlog"
)

// CreateCycleParams contains all parameters for creating a cycle.
// Use this struct to avoid long parameter lists when calling CycleService.Create.
type CreateCycleParams struct {
	OrgID     string
	ProjectID string
	CreatedBy string
	Name      string
	Goal      string
	StartsAt  time.Time
	EndsAt    time.Time
}

type ProjectMember struct {
	ProjectID string
	UserID    string
	Role      Role
	User      *User
}

type UserProjectMembership struct {
	ProjectID string
	Name      string
	Color     string
	Icon      string
	Role      Role
}

type ProjectAssignment struct {
	ProjectID string `json:"project_id"`
	Role      Role   `json:"role"`
}

type TaskStatus struct {
	ID        string
	ProjectID string
	Name      string
	Color     string
	Position  int
	Category  string
	Default   bool
}

// CreateTaskStatusParams contains all parameters for creating a task status.
// Use this struct to avoid long parameter lists when calling TaskStatusService.Create.
type CreateTaskStatusParams struct {
	ProjectID string
	Name      string
	Color     string
	Position  int
	Category  string
}
