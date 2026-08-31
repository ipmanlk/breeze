package handler

import (
	"context"
	"log/slog"
	"net/http"
	"net/http/httptest"
	"strings"
	"testing"
	"time"

	"ipmanlk/plume/internal/domain"
	"ipmanlk/plume/internal/port"
	"ipmanlk/plume/internal/service"

	"github.com/go-chi/chi/v5"
)

// slugProjectService is a minimal ProjectService mock whose only configurable
// behavior is GetBySlug. Other methods are no-ops: GetBySlug is the only one
// the GetBySlug handler exercises.
type slugProjectService struct {
	project *domain.Project
	err     error
}

func (s *slugProjectService) List(context.Context, string) ([]*domain.Project, error) {
	return nil, nil
}
func (s *slugProjectService) ListIncludingArchived(context.Context, string) ([]*domain.Project, error) {
	return nil, nil
}
func (s *slugProjectService) ListForUser(context.Context, string, string) ([]*domain.Project, error) {
	return nil, nil
}
func (s *slugProjectService) ListForUserIncludingArchived(context.Context, string, string) ([]*domain.Project, error) {
	return nil, nil
}
func (s *slugProjectService) ListForCaller(context.Context, string, string, domain.Role, bool) ([]*domain.Project, error) {
	return nil, nil
}
func (s *slugProjectService) GetByID(context.Context, string, string) (*domain.Project, error) {
	return nil, nil
}
func (s *slugProjectService) GetBySlug(context.Context, string, string) (*domain.Project, error) {
	return s.project, s.err
}
func (s *slugProjectService) Create(context.Context, string, string, string, *int, bool, domain.CycleCompletionHandling, *time.Time, *time.Time) (*domain.Project, error) {
	return nil, nil
}
func (s *slugProjectService) Update(context.Context, *domain.Project) error   { return nil }
func (s *slugProjectService) Delete(context.Context, string, string) error    { return nil }
func (s *slugProjectService) Archive(context.Context, string, string) error   { return nil }
func (s *slugProjectService) Unarchive(context.Context, string, string) error { return nil }

var _ port.ProjectService = (*slugProjectService)(nil)

// TestProjectHandler_GetBySlug_DeniesNonMemberViewer verifies that a
// viewer (project-scoped role) who is NOT a project member must get 403
// when fetching a project by slug, even though the project exists in their
// org. Previously GetBySlug returned the project to any org member.
func TestProjectHandler_GetBySlug_DeniesNonMemberViewer(t *testing.T) {
	proj := &domain.Project{ID: "proj-1", OrgID: "org-1", Slug: "secret"}
	svc := &slugProjectService{project: proj}
	// Wire a real AccessService so the viewer (non-member) path consults
	// pmRepo.Get (returns not-a-member) and denies access.
	projRepo := &fakeProjectRepo{byOrgID: map[string]*domain.Project{
		"org-1/proj-1": proj,
	}}
	pmRepo := &fakeProjectMemberRepo{members: map[string]*domain.ProjectMember{}} // no membership
	accessSvc := service.NewAccessService(pmRepo, projRepo, nil, nil)
	h := NewProjectHandler(svc, accessSvc, noopAuditService{}, slog.Default())

	r := httptest.NewRequest(http.MethodGet, "/api/projects/by-slug/secret", nil)
	// Viewer role → EnsureProjectAccess consults pmRepo.Get (returns not-a-member).
	ctx := context.WithValue(r.Context(), domain.CtxUserID, "viewer-1")
	ctx = context.WithValue(ctx, domain.CtxOrgID, "org-1")
	ctx = context.WithValue(ctx, domain.CtxRole, string(domain.RoleViewer))
	r = r.WithContext(ctx)
	r = addChiURLParamProject(r, "slug", "secret")

	w := httptest.NewRecorder()
	h.GetBySlug(w, r)

	// A non-member viewer must NOT receive the project (200). The access layer
	// returns ErrNotFound (mapped to 404) to avoid leaking the project's
	// existence: either 403 or 404 is an acceptable denial, but not 200.
	if w.Code == http.StatusOK {
		t.Fatalf("non-member viewer should be denied, got 200: %s", w.Body.String())
	}
	// The project's name/slug must not appear in the error body.
	if strings.Contains(w.Body.String(), "secret") {
		t.Errorf("response leaked project data: %s", w.Body.String())
	}
}

// TestProjectHandler_GetBySlug_AllowsMemberViewer confirms a viewer who IS a
// project member can fetch by slug (no regression).
func TestProjectHandler_GetBySlug_AllowsMemberViewer(t *testing.T) {
	proj := &domain.Project{ID: "proj-1", OrgID: "org-1", Slug: "general"}
	svc := &slugProjectService{project: proj}
	// Viewer IS a project member → access allowed.
	projRepo := &fakeProjectRepo{byOrgID: map[string]*domain.Project{
		"org-1/proj-1": proj,
	}}
	pmRepo := &fakeProjectMemberRepo{members: map[string]*domain.ProjectMember{
		"org-1/proj-1/viewer-1": {Role: domain.RoleViewer},
	}}
	accessSvc := service.NewAccessService(pmRepo, projRepo, nil, nil)
	h := NewProjectHandler(svc, accessSvc, noopAuditService{}, slog.Default())

	r := httptest.NewRequest(http.MethodGet, "/api/projects/by-slug/general", nil)
	ctx := context.WithValue(r.Context(), domain.CtxUserID, "viewer-1")
	ctx = context.WithValue(ctx, domain.CtxOrgID, "org-1")
	ctx = context.WithValue(ctx, domain.CtxRole, string(domain.RoleViewer))
	r = r.WithContext(ctx)
	r = addChiURLParamProject(r, "slug", "general")

	w := httptest.NewRecorder()
	h.GetBySlug(w, r)

	if w.Code != http.StatusOK {
		t.Fatalf("expected 200 for member viewer, got %d: %s", w.Code, w.Body.String())
	}
}

// addChiURLParamProject sets a chi URL param without importing the helper
// name-collision concerns (the package already has addChiURLParam).
func addChiURLParamProject(r *http.Request, key, value string) *http.Request {
	routeCtx := chi.NewRouteContext()
	routeCtx.URLParams.Add(key, value)
	return r.WithContext(context.WithValue(r.Context(), chi.RouteCtxKey, routeCtx))
}
