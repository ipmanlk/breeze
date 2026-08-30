package domain

import "time"

// CustomField is a project-scoped field definition (text/number/select/date)
// that can be attached to tasks in that project.
type CustomField struct {
	ID        string
	OrgID     string
	ProjectID string
	Name      string
	FieldType string // "text", "number", "select", "date"
	Options   []string
	Position  int
	CreatedAt time.Time
	UpdatedAt time.Time
}

type CreateCustomFieldParams struct {
	OrgID     string
	ProjectID string
	Name      string
	FieldType string
	Options   []string
	Position  int
}

type UpdateCustomFieldParams struct {
	ID        string
	OrgID     string
	ProjectID string
	Name      string
	Options   []string
	Position  int
}

// TaskCustomFieldValue holds the value of a custom field for a specific task.
type TaskCustomFieldValue struct {
	TaskID        string
	CustomFieldID string
	Value         string
	UpdatedAt     time.Time
}

// CustomFieldType constants
const (
	CustomFieldText   = "text"
	CustomFieldNumber = "number"
	CustomFieldSelect = "select"
	CustomFieldDate   = "date"
)
