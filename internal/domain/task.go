package domain

import (
	"io"
	"time"
)

// Task priority values. These are the string literals stored in the priority
// column and validated by the service layer.
const (
	PriorityNone   = "none"
	PriorityLow    = "low"
	PriorityMedium = "medium"
	PriorityHigh   = "high"
	PriorityUrgent = "urgent"
)

// Task status category values. These are the string literals stored in the
// category column of task_statuses and used to determine completed_at.
const (
	StatusCategoryTodo       = "todo"
	StatusCategoryInProgress = "in_progress"
	StatusCategoryDone       = "done"
	StatusCategoryCanceled   = "canceled"
)

// DeleteSubtaskMode controls what happens to a task's subtasks when the task
// is deleted.
//   - DeleteSubtaskModeBlock:  return 409 Conflict if the task has subtasks
//     (the caller must choose a mode). This is the default when no mode is
//     specified, so a parent is never silently orphaned.
//   - DeleteSubtaskModeCascade: delete all subtasks along with the parent.
//   - DeleteSubtaskModePromote: clear parent_task_id on all subtasks so they
//     become top-level tasks.
type DeleteSubtaskMode string

const (
	DeleteSubtaskModeBlock   DeleteSubtaskMode = "block"
	DeleteSubtaskModeCascade DeleteSubtaskMode = "cascade"
	DeleteSubtaskModePromote DeleteSubtaskMode = "promote"
)

type Task struct {
	ID        string  `json:"id"`
	OrgID     string  `json:"org_id"`
	ProjectID string  `json:"project_id"`
	CycleID   *string `json:"cycle_id,omitempty"`
	ParentID  *string `json:"parent_id,omitempty"`
	// SubtaskPosition orders children within their parent (lexorank). Empty
	// for top-level tasks. Set by the service when creating a subtask.
	SubtaskPosition string         `json:"subtask_position,omitempty"`
	CreatedBy       string         `json:"created_by"`
	Assignees       []TaskAssignee `json:"assignees,omitempty"`
	Title           string         `json:"title"`
	Description     string         `json:"description,omitempty"`
	StatusID        string         `json:"status_id"`
	Priority        string         `json:"priority"`
	PositionKey     string         `json:"position_key"`
	Estimate        *int           `json:"estimate,omitempty"`
	StartedAt       *time.Time     `json:"started_at,omitempty"`
	DueAt           *time.Time     `json:"due_at,omitempty"`
	CompletedAt     *time.Time     `json:"completed_at,omitempty"`
	TemplateID      *string        `json:"template_id,omitempty"`
	CreatedAt       time.Time      `json:"created_at"`
	UpdatedAt       time.Time      `json:"updated_at"`
	LabelIDs        []string       `json:"label_ids,omitempty"`
	Labels          []Label        `json:"labels,omitempty"`

	// SubtaskCount / CompletedSubtaskCount are populated by list/get queries
	// (correlated subqueries) so the frontend can render progress without
	// client-side counting from a filtered task list. Zero for subtasks
	// themselves (1-level nesting; subtasks cannot have children).
	SubtaskCount          int `json:"subtask_count"`
	CompletedSubtaskCount int `json:"completed_subtask_count"`

	// ParentTitle is the title of the parent task (when ParentID is set),
	// populated via a LEFT JOIN in GetTaskByID so the frontend can render a
	// breadcrumb without an extra round-trip. Empty for top-level tasks.
	ParentTitle string `json:"parent_title,omitempty"`

	// Mentions holds resolved labels for <@type:id> tokens in Description,
	// populated by the service layer so the frontend can render mention chips
	// without an extra round-trip. Mirrors Comment.Mentions / Message.Mentions.
	Mentions *Mentions `json:"mentions,omitempty"`
}

type CreateTaskParams struct {
	OrgID       string
	ProjectID   string
	CreatedBy   string
	Title       string
	Description string
	StatusID    string
	Priority    string
	AssigneeIDs []string
	CycleID     *string
	ParentID    *string
	Estimate    *int
	StartedAt   *time.Time
	DueAt       *time.Time
}

type TaskFilter struct {
	StatusID   *string
	CycleID    *string
	AssigneeID *string
	Priority   string
	Search     string
	LabelIDs   []string
	// IncludeSubtasks controls whether subtasks (tasks with a non-null
	// parent_task_id) appear in the project task list. When false (the
	// default), only top-level tasks are returned so the board stays clean.
	IncludeSubtasks bool
}

// TaskListFilter is a cross-project filter for listing a user's tasks.
type TaskListFilter struct {
	StatusID                 *string
	AssigneeID               *string
	CycleID                  *string
	Priority                 string
	Search                   string
	LabelIDs                 []string
	GroupBy                  string // status or priority or project or "" (none)
	ShowCompleted            bool
	RequireProjectMembership bool   // when true, only return tasks in projects the user is a member of
	Cursor                   string // base64-encoded cursor from previous page
	Limit                    int
}

// TaskListResult is the paginated envelope for the cross-project /api/tasks endpoint.
type TaskListResult struct {
	Items      []*EnrichedTask `json:"items"`
	NextCursor string          `json:"next_cursor"`
	HasMore    bool            `json:"has_more"`
}

type ReorderOp struct {
	TaskID      string
	PositionKey string
}

// AssigneeMode values for BatchUpdateParams.AssigneeMode.
const (
	AssigneeModeReplace = "replace"
	AssigneeModeAdd     = "add"
	AssigneeModeRemove  = "remove"
)

// BatchUpdateParams applies a partial update to many tasks at once. Each
// pointer field is applied only when non-nil, so callers can change just
// the fields they intend to. Title/Description are intentionally omitted;
// bulk-edit is for categorization fields (status/priority/assignees/cycle),
// not free-text.
type BatchUpdateParams struct {
	TaskIDs      []string
	ProjectID    string
	StatusID     *string
	Priority     *string
	AssigneeIDs  []string
	AssigneeMode string // one of AssigneeMode* constants; empty defaults to "replace"
	CycleID      *string
}

// EnrichedTask is a Task with project and status context for cross-project views.
type EnrichedTask struct {
	Task
	ProjectName  string
	ProjectSlug  string
	ProjectColor string
	StatusName   string
	StatusColor  string
}

type TaskAssignee struct {
	ID        string  `json:"id"`
	Name      string  `json:"name"`
	Email     string  `json:"email"`
	AvatarURL *string `json:"avatar_url,omitempty"`
}

type Attachment struct {
	ID          string
	TaskID      string
	Filename    string
	ContentType string
	Size        int64
	StoragePath string
	CreatedBy   string
	CreatedAt   time.Time
}

// CreateAttachmentParams contains all parameters for creating an attachment.
// Use this struct to avoid long parameter lists when calling AttachmentService.Create.
type CreateAttachmentParams struct {
	OrgID       string
	TaskID      string
	ProjectID   string
	CreatedBy   string
	File        io.Reader
	Filename    string
	ContentType string
	Size        int64
}

type TimeEntry struct {
	ID              string
	TaskID          string
	UserID          string
	Description     string
	StartedAt       time.Time
	EndedAt         *time.Time
	DurationMinutes *int
	CreatedAt       time.Time
	UpdatedAt       time.Time
}

// CreateTimeEntryParams contains all parameters for creating a time entry.
// Use this struct to avoid long parameter lists when calling TimeEntryService.Create.
type CreateTimeEntryParams struct {
	OrgID           string
	TaskID          string
	ProjectID       string
	UserID          string
	Description     string
	DurationMinutes int
}

// UpdateTimeEntryParams contains all parameters for updating a time entry.
// Use this struct to avoid long parameter lists when calling TimeEntryService.Update.
type UpdateTimeEntryParams struct {
	ID              string
	OrgID           string
	TaskID          string
	ProjectID       string
	Description     *string
	DurationMinutes *int
}
