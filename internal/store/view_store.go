package store

import (
	"context"
	"encoding/json"
	"fmt"

	"ipmanlk/breeze/internal/domain"
	"ipmanlk/breeze/internal/port"
	"ipmanlk/breeze/internal/store/sqlc"
)

type ViewStore struct {
	q *sqlc.Queries
}

func NewViewStore(q *sqlc.Queries) *ViewStore {
	return &ViewStore{q: q}
}

var _ port.ViewRepository = (*ViewStore)(nil)

// viewFiltersJSON is the on-disk JSON representation of domain.ViewFilters.
// It lives in the store layer so the domain type stays free of serialization
// concerns.
type viewFiltersJSON struct {
	Search     string `json:"search,omitempty"`
	Priority   string `json:"priority,omitempty"`
	StatusID   string `json:"status_id,omitempty"`
	AssigneeID string `json:"assignee_id,omitempty"`
	CycleID    string `json:"cycle_id,omitempty"`
}

func filtersToJSON(f domain.ViewFilters) (string, error) {
	b, err := json.Marshal(viewFiltersJSON{
		Search:     f.Search,
		Priority:   f.Priority,
		StatusID:   f.StatusID,
		AssigneeID: f.AssigneeID,
		CycleID:    f.CycleID,
	})
	if err != nil {
		return "", fmt.Errorf("marshal filters: %w", err)
	}
	return string(b), nil
}

func filtersFromJSON(s string) (domain.ViewFilters, error) {
	var v viewFiltersJSON
	if s == "" {
		return domain.ViewFilters{}, nil
	}
	if err := json.Unmarshal([]byte(s), &v); err != nil {
		return domain.ViewFilters{}, fmt.Errorf("unmarshal view filters: %w", err)
	}
	return domain.ViewFilters{
		Search:     v.Search,
		Priority:   v.Priority,
		StatusID:   v.StatusID,
		AssigneeID: v.AssigneeID,
		CycleID:    v.CycleID,
	}, nil
}

func (s *ViewStore) toDomain(v sqlc.View) (domain.View, error) {
	filters, err := filtersFromJSON(v.Filters)
	if err != nil {
		return domain.View{}, err
	}

	return domain.View{
		ID:        v.ID,
		OrgID:     v.OrgID,
		ProjectID: v.ProjectID,
		CreatedBy: v.CreatedBy,
		Name:      v.Name,
		Layout:    domain.ViewLayout(v.Layout),
		Filters:   filters,
		CreatedAt: parseTime(v.CreatedAt),
		UpdatedAt: parseTime(v.UpdatedAt),
	}, nil
}

func (s *ViewStore) Create(ctx context.Context, v *domain.View) error {
	filtersJSON, err := filtersToJSON(v.Filters)
	if err != nil {
		return err
	}
	return s.q.CreateView(ctx, sqlc.CreateViewParams{
		ID:        v.ID,
		OrgID:     v.OrgID,
		ProjectID: v.ProjectID,
		CreatedBy: v.CreatedBy,
		Name:      v.Name,
		Layout:    string(v.Layout),
		Filters:   filtersJSON,
	})
}

func (s *ViewStore) Update(ctx context.Context, v *domain.View) error {
	filtersJSON, err := filtersToJSON(v.Filters)
	if err != nil {
		return err
	}
	return s.q.UpdateView(ctx, sqlc.UpdateViewParams{
		Name:    v.Name,
		Layout:  string(v.Layout),
		Filters: filtersJSON,
		ID:      v.ID,
		OrgID:   v.OrgID,
	})
}

func (s *ViewStore) Delete(ctx context.Context, orgID, id string) error {
	return s.q.DeleteView(ctx, sqlc.DeleteViewParams{ID: id, OrgID: orgID})
}

func (s *ViewStore) GetByID(ctx context.Context, orgID, id string) (*domain.View, error) {
	v, err := s.q.GetViewByID(ctx, sqlc.GetViewByIDParams{ID: id, OrgID: orgID})
	if err != nil {
		return nil, mapScanErr(err)
	}
	d, err := s.toDomain(v)
	if err != nil {
		return nil, err
	}
	return &d, nil
}

func (s *ViewStore) ListByProject(ctx context.Context, orgID, projectID string) ([]*domain.View, error) {
	rows, err := s.q.ListViewsByProject(ctx, sqlc.ListViewsByProjectParams{
		OrgID:     orgID,
		ProjectID: &projectID,
	})
	if err != nil {
		return nil, err
	}
	views := make([]*domain.View, len(rows))
	for i, row := range rows {
		d, err := s.toDomain(row)
		if err != nil {
			return nil, err
		}
		views[i] = &d
	}
	return views, nil
}

func (s *ViewStore) ListGlobal(ctx context.Context, orgID string) ([]*domain.View, error) {
	rows, err := s.q.ListGlobalViews(ctx, orgID)
	if err != nil {
		return nil, err
	}
	views := make([]*domain.View, len(rows))
	for i, row := range rows {
		d, err := s.toDomain(row)
		if err != nil {
			return nil, err
		}
		views[i] = &d
	}
	return views, nil
}

func (s *ViewStore) ListPinned(ctx context.Context, userID string) ([]*domain.View, error) {
	rows, err := s.q.ListPinnedViews(ctx, userID)
	if err != nil {
		return nil, err
	}
	views := make([]*domain.View, len(rows))
	for i, row := range rows {
		d, err := s.toDomain(row)
		if err != nil {
			return nil, err
		}
		views[i] = &d
	}
	return views, nil
}

func (s *ViewStore) Pin(ctx context.Context, viewID, userID string) error {
	return s.q.PinView(ctx, sqlc.PinViewParams{ViewID: viewID, UserID: userID})
}

func (s *ViewStore) Unpin(ctx context.Context, viewID, userID string) error {
	return s.q.UnpinView(ctx, sqlc.UnpinViewParams{ViewID: viewID, UserID: userID})
}
