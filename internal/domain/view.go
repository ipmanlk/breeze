package domain

import "time"

type ViewLayout string

const (
	ViewLayoutBoard ViewLayout = "board"
	ViewLayoutList  ViewLayout = "list"
)

// ViewFilters holds the structured filter state for a saved view.
// All fields are optional; zero values mean "not filtered".
type ViewFilters struct {
	Search     string
	Priority   string
	StatusID   string
	AssigneeID string
	CycleID    string
	LabelIDs   []string
}

type View struct {
	ID        string
	OrgID     string
	ProjectID *string // nil = global
	CreatedBy string
	Name      string
	Layout    ViewLayout
	Filters   ViewFilters
	CreatedAt time.Time
	UpdatedAt time.Time
}

type CreateViewParams struct {
	OrgID     string
	ProjectID *string
	CreatedBy string
	Name      string
	Layout    ViewLayout
	Filters   ViewFilters
}

type UpdateViewParams struct {
	ID      string
	OrgID   string
	Name    *string
	Layout  *ViewLayout
	Filters *ViewFilters
}
