package domain

import "time"

// TaskActivityFilter holds pagination parameters for listing task activity.
type TaskActivityFilter struct {
	Cursor string
	Limit  int
}

// TaskActivityResult is a paginated list of task activity entries.
type TaskActivityResult struct {
	Items      []*TaskActivity
	NextCursor string
	HasMore    bool
}

// ActivityAction enumerates the recordable task activity actions. Keeping
// these as constants (rather than free strings) makes call sites grep-able
// and prevents typos from creating silent gaps in the feed.
type ActivityAction string

const (
	ActivityCreated            ActivityAction = "created"
	ActivityUpdated            ActivityAction = "updated"
	ActivityStatusChanged      ActivityAction = "status_changed"
	ActivityAssigned           ActivityAction = "assigned"
	ActivityUnassigned         ActivityAction = "unassigned"
	ActivityPriorityChanged    ActivityAction = "priority_changed"
	ActivityDueDateChanged     ActivityAction = "due_date_changed"
	ActivityMoved              ActivityAction = "moved"
	ActivityDeleted            ActivityAction = "deleted"
	ActivityTitleChanged       ActivityAction = "title_changed"
	ActivityDescriptionChanged ActivityAction = "description_changed"
	ActivityLabelsChanged      ActivityAction = "labels_changed"
	ActivityEstimateChanged    ActivityAction = "estimate_changed"
	ActivityCycleChanged       ActivityAction = "cycle_changed"
	ActivityReparented         ActivityAction = "reparented"
	ActivityStartedAtChanged   ActivityAction = "started_at_changed"
	ActivityMovedToProject     ActivityAction = "moved_to_project"
	ActivityDuplicated         ActivityAction = "duplicated"
	ActivityCommentAdded       ActivityAction = "comment_added"
	ActivityFileAttached       ActivityAction = "file_attached"
	ActivityFileRemoved        ActivityAction = "file_removed"
	ActivityTimeLogged         ActivityAction = "time_logged"
	ActivityDependencyAdded    ActivityAction = "dependency_added"
	ActivityDependencyRemoved  ActivityAction = "dependency_removed"
)

// TaskActivity is a single activity entry for a task. OldValue/NewValue are
// human-readable strings (e.g. "Todo" → "Done"); Field names the changed
// field (e.g. "status", "assignee", "priority", "due_at").
// ActorName/ActorEmail are hydrated by the store via a JOIN to users.
type TaskActivity struct {
	ID        string
	TaskID    string
	OrgID     string
	ProjectID string
	ActorID   string
	Action    ActivityAction
	Field     string
	OldValue  string
	NewValue  string
	CreatedAt time.Time

	// Hydrated by the store (not stored in the table).
	ActorName  string
	ActorEmail string
}
