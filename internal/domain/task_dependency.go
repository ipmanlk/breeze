package domain

import "time"

// TaskDependency represents a blocking relationship: TaskID is blocked by
// BlocksTaskID (BlocksTaskID must complete first). Both ends reference tasks
// in the same org; cross-project dependencies are allowed within an org.
type TaskDependency struct {
	TaskID       string
	BlocksTaskID string
	CreatedAt    time.Time
}
