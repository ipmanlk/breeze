package domain

import "time"

// Label is an org-scoped tag that can be attached to tasks.
type Label struct {
	ID        string    `json:"id"`
	OrgID     string    `json:"org_id"`
	Name      string    `json:"name"`
	Color     string    `json:"color"`
	CreatedAt time.Time `json:"created_at"`
	UpdatedAt time.Time `json:"updated_at"`
}

// CreateLabelParams carries the fields needed to create a label.
type CreateLabelParams struct {
	OrgID string
	Name  string
	Color string
}

// UpdateLabelParams carries the fields that can be changed on a label.
type UpdateLabelParams struct {
	ID    string
	OrgID string
	Name  string
	Color string
}
