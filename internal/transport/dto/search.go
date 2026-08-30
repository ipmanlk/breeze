package dto

import (
	"ipmanlk/breeze/internal/domain"
)

// SearchResultResponse is the API representation of a single search result.
type SearchResultResponse struct {
	ID        string `json:"id"`
	Type      string `json:"type"`
	Name      string `json:"name"`
	Subtitle  string `json:"subtitle,omitempty"`
	URL       string `json:"url"`
	Color     string `json:"color,omitempty"`
	Icon      string `json:"icon,omitempty"`
	ProjectID string `json:"project_id,omitempty"`
}

// SearchResponse wraps the list of search results.
type SearchResponse struct {
	Results []*SearchResultResponse `json:"results"`
}

func NewSearchResponse(results []*domain.SearchResult) *SearchResponse {
	items := make([]*SearchResultResponse, len(results))
	for i, r := range results {
		items[i] = &SearchResultResponse{
			ID:        r.ID,
			Type:      string(r.Type),
			Name:      r.Name,
			Subtitle:  r.Subtitle,
			URL:       r.URL,
			Color:     r.Color,
			Icon:      r.Icon,
			ProjectID: r.ProjectID,
		}
	}
	return &SearchResponse{Results: items}
}
