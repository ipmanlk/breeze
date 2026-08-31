package service

import (
	"context"
	"errors"
	"fmt"
	"strconv"
	"time"

	"ipmanlk/plume/internal/apperr"
	"ipmanlk/plume/internal/domain"
	"ipmanlk/plume/internal/port"

	"github.com/google/uuid"
)

type ProjectService struct {
	projRepo   port.ProjectRepository
	statusRepo port.TaskStatusRepository
	viewRepo   port.ViewRepository
	postCreate func(ctx context.Context, orgID, projectID, userID string) error
}

var _ port.ProjectService = (*ProjectService)(nil)

func NewProjectService(projRepo port.ProjectRepository, statusRepo port.TaskStatusRepository, viewRepo port.ViewRepository) *ProjectService {
	return &ProjectService{projRepo: projRepo, statusRepo: statusRepo, viewRepo: viewRepo}
}

// SetPostCreateHook registers a callback invoked after the project is persisted.
// Used by app.go to auto-seed a #general channel scoped to the project.
func (s *ProjectService) SetPostCreateHook(fn func(ctx context.Context, orgID, projectID, userID string) error) {
	s.postCreate = fn
}

var defaultStatuses = []struct {
	Name     string
	Color    string
	Position int
	Category string
}{
	{"Backlog", "#94a3b8", 0, domain.StatusCategoryTodo},
	{"Todo", "#3b82f6", 1, domain.StatusCategoryTodo},
	{"In Progress", "#f59e0b", 2, domain.StatusCategoryInProgress},
	{"In Review", "#8b5cf6", 3, domain.StatusCategoryInProgress},
	{"Done", "#22c55e", 4, domain.StatusCategoryDone},
	{"Canceled", "#ef4444", 5, domain.StatusCategoryCanceled},
}

func (s *ProjectService) Create(ctx context.Context, orgID, name string, createdBy string, cycleDuration *int, autoGenerateCycles bool, incompleteTaskHandling domain.CycleCompletionHandling, startsAt, endsAt *time.Time) (*domain.Project, error) {
	if startsAt != nil && endsAt != nil && endsAt.Before(*startsAt) {
		return nil, apperr.InvalidInput("project end date must be after start date")
	}
	id := uuid.New().String()
	slug, err := s.uniqueSlug(ctx, orgID, "", slugify(name, "project"))
	if err != nil {
		return nil, err
	}

	now := time.Now()
	project := &domain.Project{
		ID:                     id,
		OrgID:                  orgID,
		Name:                   name,
		Slug:                   slug,
		Color:                  "oklch(0.6 0.15 250)",
		Icon:                   "FolderIcon",
		CreatedBy:              createdBy,
		CycleDuration:          cycleDuration,
		AutoGenerateCycles:     autoGenerateCycles,
		IncompleteTaskHandling: incompleteTaskHandling,
		StartsAt:               startsAt,
		EndsAt:                 endsAt,
		CreatedAt:              now,
		UpdatedAt:              now,
	}

	// Build default statuses
	statuses := make([]*domain.TaskStatus, len(defaultStatuses))
	for i, ds := range defaultStatuses {
		statuses[i] = &domain.TaskStatus{
			ID:        uuid.New().String(),
			ProjectID: id,
			Name:      ds.Name,
			Color:     ds.Color,
			Position:  ds.Position,
			Category:  ds.Category,
			Default:   true,
		}
	}

	if err := s.projRepo.CreateWithStatuses(ctx, project, statuses); err != nil {
		return nil, err
	}

	if s.postCreate != nil {
		if err := s.postCreate(ctx, orgID, id, createdBy); err != nil {
			return project, fmt.Errorf("post-create: %w", err)
		}
	}

	return s.projRepo.GetByID(ctx, orgID, id)
}

func (s *ProjectService) List(ctx context.Context, orgID string) ([]*domain.Project, error) {
	return s.projRepo.List(ctx, orgID)
}

// ListIncludingArchived returns all projects including archived ones. Used by
// the project list handler when the archived=true query param is set so users
// can discover and restore archived projects.
func (s *ProjectService) ListIncludingArchived(ctx context.Context, orgID string) ([]*domain.Project, error) {
	return s.projRepo.ListIncludingArchived(ctx, orgID)
}

func (s *ProjectService) ListForUser(ctx context.Context, orgID, userID string) ([]*domain.Project, error) {
	return s.projRepo.ListForUser(ctx, orgID, userID)
}

// ListForUserIncludingArchived returns all projects the project-scoped user is
// a member of, including archived ones.
func (s *ProjectService) ListForUserIncludingArchived(ctx context.Context, orgID, userID string) ([]*domain.Project, error) {
	return s.projRepo.ListForUserIncludingArchived(ctx, orgID, userID)
}

// ListForCaller returns projects scoped to the caller's org role.
// Elevated roles (owner/admin/member) see all org projects; viewer/guest
// see only projects they have explicit membership in.
func (s *ProjectService) ListForCaller(ctx context.Context, orgID, userID string, role domain.Role, includeArchived bool) ([]*domain.Project, error) {
	if domain.IsOrgElevatedRole(role) {
		if includeArchived {
			return s.projRepo.ListIncludingArchived(ctx, orgID)
		}
		return s.projRepo.List(ctx, orgID)
	}
	if includeArchived {
		return s.projRepo.ListForUserIncludingArchived(ctx, orgID, userID)
	}
	return s.projRepo.ListForUser(ctx, orgID, userID)
}

func (s *ProjectService) GetByID(ctx context.Context, orgID, id string) (*domain.Project, error) {
	return s.projRepo.GetByID(ctx, orgID, id)
}

func (s *ProjectService) GetBySlug(ctx context.Context, orgID, slug string) (*domain.Project, error) {
	return s.projRepo.GetBySlug(ctx, orgID, slug)
}

func (s *ProjectService) Update(ctx context.Context, p *domain.Project) error {
	if p.StartsAt != nil && p.EndsAt != nil && p.EndsAt.Before(*p.StartsAt) {
		return apperr.InvalidInput("project end date must be after start date")
	}
	existing, err := s.projRepo.GetByID(ctx, p.OrgID, p.ID)
	if err != nil {
		return fmt.Errorf("get existing project: %w", err)
	}
	if p.Slug != existing.Slug && p.Slug != "" {
		p.Slug, err = s.uniqueSlug(ctx, p.OrgID, p.ID, p.Slug)
		if err != nil {
			return err
		}
	}
	p.UpdatedAt = time.Now()
	return s.projRepo.Update(ctx, p)
}

func (s *ProjectService) Delete(ctx context.Context, orgID, id string) error {
	return s.projRepo.Delete(ctx, orgID, id)
}

// Archive marks a project as archived. Archived projects are hidden from the
// default project list but remain accessible (read-only) for reference.
func (s *ProjectService) Archive(ctx context.Context, orgID, id string) error {
	return s.projRepo.SetArchived(ctx, orgID, id, true)
}

// Unarchive restores an archived project to the active list.
func (s *ProjectService) Unarchive(ctx context.Context, orgID, id string) error {
	return s.projRepo.SetArchived(ctx, orgID, id, false)
}

// uniqueSlug returns a project slug that is unique within the org. If the
// desired slug is already taken by another project, it appends a numeric
// suffix. An empty excludeID means "ignore no project" (used on create).
func (s *ProjectService) uniqueSlug(ctx context.Context, orgID, excludeID, desired string) (string, error) {
	if desired == "" {
		return "", apperr.InvalidInput("slug cannot be empty")
	}
	slug := desired
	for i := 2; i <= 1000; i++ {
		existing, err := s.projRepo.GetBySlug(ctx, orgID, slug)
		if err != nil {
			if errors.Is(err, apperr.ErrNotFound) {
				return slug, nil
			}
			return "", fmt.Errorf("check slug: %w", err)
		}
		if existing.ID == excludeID {
			return slug, nil
		}
		slug = desired + "-" + strconv.Itoa(i)
	}
	return "", apperr.InvalidInput("could not generate a unique slug")
}
