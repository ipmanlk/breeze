package dto

// AddDependencyRequest records that the path task is blocked by the given
// blocking task.
type AddDependencyRequest struct {
	BlocksTaskID string `json:"blocks_task_id" validate:"required"`
}
