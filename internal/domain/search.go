package domain

// SearchResult represents a single result from a global search query.
// It can match projects, tasks, conversations, or members.
type SearchResult struct {
	ID        string
	Type      SearchType
	Name      string
	Subtitle  string
	URL       string
	Color     string
	Icon      string
	ProjectID string
}

type SearchType string

const (
	SearchTypeProject       SearchType = "project"
	SearchTypeTask          SearchType = "task"
	SearchTypeChannel       SearchType = "channel"
	SearchTypeDirectMessage SearchType = "direct_message"
	SearchTypeMember        SearchType = "member"
)

// SearchParams contains all parameters for a search query.
// Use this struct to avoid long parameter lists when calling SearchService.Search.
type SearchParams struct {
	OrgID  string
	UserID string
	Role   Role
	Query  string
	Limit  int
	Types  []SearchType
	// ProjectID narrows task/project search to a single project (the
	// command palette's "search in this project" mode). Empty means org-wide.
	ProjectID string
}
