package handler

import (
	"context"
	"errors"
	"log/slog"
	"testing"

	"ipmanlk/plume/internal/domain"
	"ipmanlk/plume/internal/port"
)

// Minimal, behavior-driven fakes for exercising the real wsRoomAccessChecker
// (not the denyAll mock). These exist so we can assert the project access path
// actually grants access to members and denies non-members: a regression
// guard for the bug where CanAccessProject passed context.Background() to
// access.ResolveEffectiveRole (which reads identity from the context), causing
// every project room subscription to be denied.

type fakeProjectRepo struct {
	byOrgID map[string]*domain.Project // keyed by orgID+"/"+id
}

func (f *fakeProjectRepo) List(ctx context.Context, orgID string) ([]*domain.Project, error) {
	return nil, nil
}
func (f *fakeProjectRepo) ListForUser(ctx context.Context, orgID, userID string) ([]*domain.Project, error) {
	return nil, nil
}
func (f *fakeProjectRepo) ListIncludingArchived(ctx context.Context, orgID string) ([]*domain.Project, error) {
	return nil, nil
}
func (f *fakeProjectRepo) ListForUserIncludingArchived(ctx context.Context, orgID, userID string) ([]*domain.Project, error) {
	return nil, nil
}
func (f *fakeProjectRepo) ListByIDs(ctx context.Context, orgID string, ids []string) ([]*domain.Project, error) {
	return nil, nil
}
func (f *fakeProjectRepo) GetByID(ctx context.Context, orgID, id string) (*domain.Project, error) {
	p, ok := f.byOrgID[orgID+"/"+id]
	if !ok {
		return nil, errors.New("not found")
	}
	return p, nil
}
func (f *fakeProjectRepo) GetBySlug(ctx context.Context, orgID, slug string) (*domain.Project, error) {
	return nil, nil
}
func (f *fakeProjectRepo) Create(ctx context.Context, p *domain.Project) error      { return nil }
func (f *fakeProjectRepo) Update(ctx context.Context, p *domain.Project) error      { return nil }
func (f *fakeProjectRepo) Delete(ctx context.Context, orgID, id string) error       { return nil }
func (f *fakeProjectRepo) SetArchived(_ context.Context, _, _ string, _ bool) error { return nil }
func (f *fakeProjectRepo) CreateWithStatuses(_ context.Context, _ *domain.Project, _ []*domain.TaskStatus) error {
	return nil
}

var _ port.ProjectRepository = (*fakeProjectRepo)(nil)

type fakeProjectMemberRepo struct {
	members map[string]*domain.ProjectMember // keyed by orgID+"/"+projectID+"/"+userID
}

func (f *fakeProjectMemberRepo) List(ctx context.Context, orgID, projectID string, filter domain.UserFilter) (*domain.ProjectMemberListResult, error) {
	return &domain.ProjectMemberListResult{}, nil
}
func (f *fakeProjectMemberRepo) Get(ctx context.Context, orgID, projectID, userID string) (*domain.ProjectMember, error) {
	pm, ok := f.members[orgID+"/"+projectID+"/"+userID]
	if !ok {
		return nil, errors.New("not a member")
	}
	return pm, nil
}
func (f *fakeProjectMemberRepo) Add(ctx context.Context, projectID, userID string, role domain.Role) error {
	return nil
}
func (f *fakeProjectMemberRepo) Remove(ctx context.Context, projectID, userID string) error {
	return nil
}
func (f *fakeProjectMemberRepo) UpdateRole(ctx context.Context, projectID, userID string, role domain.Role) error {
	return nil
}
func (f *fakeProjectMemberRepo) ListByUser(ctx context.Context, orgID, userID string) ([]*domain.UserProjectMembership, error) {
	return nil, nil
}

func (f *fakeProjectMemberRepo) SetMemberships(_ context.Context, _ string, _ string, _ []domain.ProjectAssignment) error {
	return nil
}

var _ port.ProjectMemberRepository = (*fakeProjectMemberRepo)(nil)

// TestWSRoomAccessChecker_CanAccessProject_AllowsMember is the regression test
// for the room-access checker: CanAccessProject must GRANT access to a user who is a
// member of the project. Previously it delegated to ResolveEffectiveRole with
// context.Background(), which has no user/org/role values, so EVERY project
// subscription was denied: silently breaking real-time task updates.
func TestWSRoomAccessChecker_CanAccessProject_AllowsMember(t *testing.T) {
	projRepo := &fakeProjectRepo{byOrgID: map[string]*domain.Project{
		"org-1/proj-1": {ID: "proj-1", OrgID: "org-1"},
	}}
	pmRepo := &fakeProjectMemberRepo{members: map[string]*domain.ProjectMember{
		"org-1/proj-1/user-1": {ProjectID: "proj-1", UserID: "user-1", Role: domain.RoleViewer},
	}}
	chk := NewWSRoomAccessChecker(nil, nil, pmRepo, projRepo, slog.Default())
	ctx := context.Background()

	// Viewer (project-scoped role) WITH a membership row → allowed.
	if !chk.CanAccessProject(ctx, "org-1", "proj-1", "user-1", domain.RoleViewer) {
		t.Fatal("viewer member was denied project room access; this breaks real-time task subscriptions")
	}

	// Elevated org role → allowed (implicit org-wide access).
	if !chk.CanAccessProject(ctx, "org-1", "proj-1", "user-2", domain.RoleMember) {
		t.Fatal("org member was denied project room access; elevated roles have implicit access")
	}
}

// TestWSRoomAccessChecker_CanAccessProject_DeniesNonMember ensures a guest
// with no project_members row cannot subscribe to the project room.
func TestWSRoomAccessChecker_CanAccessProject_DeniesNonMember(t *testing.T) {
	projRepo := &fakeProjectRepo{byOrgID: map[string]*domain.Project{
		"org-1/proj-1": {ID: "proj-1", OrgID: "org-1"},
	}}
	pmRepo := &fakeProjectMemberRepo{members: map[string]*domain.ProjectMember{}}
	chk := NewWSRoomAccessChecker(nil, nil, pmRepo, projRepo, slog.Default())
	ctx := context.Background()

	// Guest with no membership row → denied.
	if chk.CanAccessProject(ctx, "org-1", "proj-1", "user-3", domain.RoleGuest) {
		t.Fatal("guest with no project membership was granted project room access")
	}

	// Cross-org attempt: project exists only in org-1; admin of org-2 must be
	// denied (defense in depth, even though the room key embeds the orgID).
	if chk.CanAccessProject(ctx, "org-2", "proj-1", "user-4", domain.RoleOwner) {
		t.Fatal("owner of a different org was granted project room access")
	}
}
