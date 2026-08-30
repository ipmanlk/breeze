package store

import (
	"context"
	"database/sql"

	"ipmanlk/breeze/internal/domain"
	"ipmanlk/breeze/internal/port"
	"ipmanlk/breeze/internal/store/sqlc"
)

type ProjectStore struct {
	q  *sqlc.Queries
	db *sql.DB
}

func NewProjectStore(q *sqlc.Queries, db *sql.DB) *ProjectStore {
	return &ProjectStore{q: q, db: db}
}

var _ port.ProjectRepository = (*ProjectStore)(nil)

// projectFields is the common shape extracted from every project query row.
// All sqlc-generated project row types share the same fields; each query
// method extracts them into this struct so a single mapper covers all.
type projectFields struct {
	ID                     string
	OrgID                  string
	Name                   string
	Description            string
	Slug                   string
	Color                  string
	Icon                   string
	CreatedBy              string
	CycleDuration          *int64
	AutoGenerateCycles     bool
	IncompleteTaskHandling string
	StartsAt               *string
	EndsAt                 *string
	IsArchived             bool
	CreatedAt              string
	UpdatedAt              string
}

func mapProject(p projectFields) domain.Project {
	var cd *int
	if p.CycleDuration != nil {
		v := int(*p.CycleDuration)
		cd = &v
	}
	return domain.Project{
		ID:                     p.ID,
		OrgID:                  p.OrgID,
		Name:                   p.Name,
		Description:            p.Description,
		Slug:                   p.Slug,
		Color:                  p.Color,
		Icon:                   p.Icon,
		CreatedBy:              p.CreatedBy,
		CycleDuration:          cd,
		AutoGenerateCycles:     p.AutoGenerateCycles,
		IncompleteTaskHandling: domain.CycleCompletionHandling(p.IncompleteTaskHandling),
		StartsAt:               parseTimePtr(p.StartsAt),
		EndsAt:                 parseTimePtr(p.EndsAt),
		IsArchived:             p.IsArchived,
		CreatedAt:              parseTime(p.CreatedAt),
		UpdatedAt:              parseTime(p.UpdatedAt),
	}
}

func (s *ProjectStore) List(ctx context.Context, orgID string) ([]*domain.Project, error) {
	return s.list(ctx, orgID, false)
}

func (s *ProjectStore) ListIncludingArchived(ctx context.Context, orgID string) ([]*domain.Project, error) {
	return s.list(ctx, orgID, true)
}

func (s *ProjectStore) list(ctx context.Context, orgID string, includeArchived bool) ([]*domain.Project, error) {
	rows, err := s.q.ListProjectsByOrg(ctx, sqlc.ListProjectsByOrgParams{
		OrgID:           orgID,
		IncludeArchived: includeArchived,
	})
	if err != nil {
		return nil, err
	}
	projects := make([]*domain.Project, len(rows))
	for i, r := range rows {
		d := mapProject(projectFields{
			ID: r.ID, OrgID: r.OrgID, Name: r.Name, Description: r.Description,
			Slug: r.Slug, Color: r.Color, Icon: r.Icon, CreatedBy: r.CreatedBy,
			CycleDuration: r.CycleDuration, AutoGenerateCycles: r.AutoGenerateCycles,
			IncompleteTaskHandling: r.IncompleteTaskHandling, StartsAt: r.StartsAt, EndsAt: r.EndsAt,
			IsArchived: r.IsArchived, CreatedAt: r.CreatedAt, UpdatedAt: r.UpdatedAt,
		})
		projects[i] = &d
	}
	return projects, nil
}

func (s *ProjectStore) ListForUser(ctx context.Context, orgID, userID string) ([]*domain.Project, error) {
	return s.listForUser(ctx, orgID, userID, false)
}

func (s *ProjectStore) ListForUserIncludingArchived(ctx context.Context, orgID, userID string) ([]*domain.Project, error) {
	return s.listForUser(ctx, orgID, userID, true)
}

func (s *ProjectStore) listForUser(ctx context.Context, orgID, userID string, includeArchived bool) ([]*domain.Project, error) {
	rows, err := s.q.ListProjectsByMembership(ctx, sqlc.ListProjectsByMembershipParams{
		UserID:          userID,
		OrgID:           orgID,
		IncludeArchived: includeArchived,
	})
	if err != nil {
		return nil, err
	}
	projects := make([]*domain.Project, len(rows))
	for i, r := range rows {
		d := mapProject(projectFields{
			ID: r.ID, OrgID: r.OrgID, Name: r.Name, Description: r.Description,
			Slug: r.Slug, Color: r.Color, Icon: r.Icon, CreatedBy: r.CreatedBy,
			CycleDuration: r.CycleDuration, AutoGenerateCycles: r.AutoGenerateCycles,
			IncompleteTaskHandling: r.IncompleteTaskHandling, StartsAt: r.StartsAt, EndsAt: r.EndsAt,
			IsArchived: r.IsArchived, CreatedAt: r.CreatedAt, UpdatedAt: r.UpdatedAt,
		})
		projects[i] = &d
	}
	return projects, nil
}

func (s *ProjectStore) GetByID(ctx context.Context, orgID, id string) (*domain.Project, error) {
	r, err := s.q.GetProjectByID(ctx, sqlc.GetProjectByIDParams{ID: id, OrgID: orgID})
	if err != nil {
		return nil, mapScanErr(err)
	}
	d := mapProject(projectFields{
		ID: r.ID, OrgID: r.OrgID, Name: r.Name, Description: r.Description,
		Slug: r.Slug, Color: r.Color, Icon: r.Icon, CreatedBy: r.CreatedBy,
		CycleDuration: r.CycleDuration, AutoGenerateCycles: r.AutoGenerateCycles,
		IncompleteTaskHandling: r.IncompleteTaskHandling, StartsAt: r.StartsAt, EndsAt: r.EndsAt,
		IsArchived: r.IsArchived, CreatedAt: r.CreatedAt, UpdatedAt: r.UpdatedAt,
	})
	return &d, nil
}

func (s *ProjectStore) GetBySlug(ctx context.Context, orgID, slug string) (*domain.Project, error) {
	r, err := s.q.GetProjectBySlug(ctx, sqlc.GetProjectBySlugParams{OrgID: orgID, Slug: slug})
	if err != nil {
		return nil, mapScanErr(err)
	}
	d := mapProject(projectFields{
		ID: r.ID, OrgID: r.OrgID, Name: r.Name, Description: r.Description,
		Slug: r.Slug, Color: r.Color, Icon: r.Icon, CreatedBy: r.CreatedBy,
		CycleDuration: r.CycleDuration, AutoGenerateCycles: r.AutoGenerateCycles,
		IncompleteTaskHandling: r.IncompleteTaskHandling, StartsAt: r.StartsAt, EndsAt: r.EndsAt,
		IsArchived: r.IsArchived, CreatedAt: r.CreatedAt, UpdatedAt: r.UpdatedAt,
	})
	return &d, nil
}

func (s *ProjectStore) Create(ctx context.Context, p *domain.Project) error {
	return s.q.CreateProject(ctx, sqlc.CreateProjectParams{
		ID:                     p.ID,
		OrgID:                  p.OrgID,
		Name:                   p.Name,
		Description:            p.Description,
		Slug:                   p.Slug,
		Color:                  p.Color,
		Icon:                   p.Icon,
		CreatedBy:              p.CreatedBy,
		CycleDuration:          int64Ptr(p.CycleDuration),
		AutoGenerateCycles:     p.AutoGenerateCycles,
		IncompleteTaskHandling: string(p.IncompleteTaskHandling),
		StartsAt:               formatTimePtr(p.StartsAt),
		EndsAt:                 formatTimePtr(p.EndsAt),
	})
}

// CreateWithStatuses creates a project and its default task statuses atomically.
func (s *ProjectStore) CreateWithStatuses(ctx context.Context, project *domain.Project, statuses []*domain.TaskStatus) error {
	tx, err := s.db.BeginTx(ctx, nil)
	if err != nil {
		return err
	}
	defer tx.Rollback()

	q := s.q.WithTx(tx)

	if err := q.CreateProject(ctx, sqlc.CreateProjectParams{
		ID:                     project.ID,
		OrgID:                  project.OrgID,
		Name:                   project.Name,
		Description:            project.Description,
		Slug:                   project.Slug,
		Color:                  project.Color,
		Icon:                   project.Icon,
		CreatedBy:              project.CreatedBy,
		CycleDuration:          int64Ptr(project.CycleDuration),
		AutoGenerateCycles:     project.AutoGenerateCycles,
		IncompleteTaskHandling: string(project.IncompleteTaskHandling),
		StartsAt:               formatTimePtr(project.StartsAt),
		EndsAt:                 formatTimePtr(project.EndsAt),
	}); err != nil {
		return err
	}

	for _, st := range statuses {
		if err := q.CreateStatus(ctx, sqlc.CreateStatusParams{
			ID:        st.ID,
			ProjectID: st.ProjectID,
			Name:      st.Name,
			Color:     st.Color,
			Position:  int64(st.Position),
			Category:  st.Category,
			IsDefault: st.Default,
		}); err != nil {
			return err
		}
	}

	return tx.Commit()
}

func (s *ProjectStore) Update(ctx context.Context, p *domain.Project) error {
	return s.q.UpdateProject(ctx, sqlc.UpdateProjectParams{
		Name:                   p.Name,
		Description:            p.Description,
		Color:                  p.Color,
		Icon:                   p.Icon,
		CycleDuration:          int64Ptr(p.CycleDuration),
		AutoGenerateCycles:     p.AutoGenerateCycles,
		IncompleteTaskHandling: string(p.IncompleteTaskHandling),
		StartsAt:               formatTimePtr(p.StartsAt),
		EndsAt:                 formatTimePtr(p.EndsAt),
		ID:                     p.ID,
		OrgID:                  p.OrgID,
	})
}

func (s *ProjectStore) SetArchived(ctx context.Context, orgID, id string, archived bool) error {
	return s.q.SetProjectArchived(ctx, sqlc.SetProjectArchivedParams{
		IsArchived: archived,
		ID:         id,
		OrgID:      orgID,
	})
}

func (s *ProjectStore) Delete(ctx context.Context, orgID, id string) error {
	return s.q.DeleteProject(ctx, sqlc.DeleteProjectParams{ID: id, OrgID: orgID})
}

func (s *ProjectStore) ListByIDs(ctx context.Context, orgID string, ids []string) ([]*domain.Project, error) {
	if len(ids) == 0 {
		return nil, nil
	}
	rows, err := s.q.GetProjectsByIDs(ctx, ids)
	if err != nil {
		return nil, err
	}
	projects := make([]*domain.Project, 0, len(rows))
	for _, row := range rows {
		// Filter by orgID for security; the SQL fetches by IDs only
		if row.OrgID != orgID {
			continue
		}
		d := &domain.Project{
			ID:    row.ID,
			Name:  row.Name,
			OrgID: row.OrgID,
			Icon:  row.Icon,
			Color: row.Color,
		}
		projects = append(projects, d)
	}
	return projects, nil
}
