package service

import (
	"context"
	"errors"
	"testing"

	"ipmanlk/breeze/internal/apperr"
	"ipmanlk/breeze/internal/domain"
	"ipmanlk/breeze/internal/port"
)

// testRepos holds minimal mock repos for AccessChecker tests.
type testProjRepo struct {
	port.ProjectRepository
	getByIDFn func(ctx context.Context, orgID, id string) (*domain.Project, error)
}

func (r *testProjRepo) GetByID(ctx context.Context, orgID, id string) (*domain.Project, error) {
	return r.getByIDFn(ctx, orgID, id)
}

type testUserRepo struct {
	port.UserRepository
	getByIDFn func(ctx context.Context, orgID, id string) (*domain.User, error)
}

func (r *testUserRepo) GetByID(ctx context.Context, orgID, id string) (*domain.User, error) {
	return r.getByIDFn(ctx, orgID, id)
}

type testPMRepo struct {
	port.ProjectMemberRepository
	getFn func(ctx context.Context, orgID, projectID, userID string) (*domain.ProjectMember, error)
}

func (r *testPMRepo) Get(ctx context.Context, orgID, projectID, userID string) (*domain.ProjectMember, error) {
	return r.getFn(ctx, orgID, projectID, userID)
}

type testTaskRepo struct {
	port.TaskRepository
	getByIDAndOrgFn func(ctx context.Context, orgID, id string) (*domain.Task, error)
}

func (r *testTaskRepo) GetByIDAndOrg(ctx context.Context, orgID, id string) (*domain.Task, error) {
	return r.getByIDAndOrgFn(ctx, orgID, id)
}

func proj(projectID, orgID string) *domain.Project {
	return &domain.Project{ID: projectID, OrgID: orgID}
}

func userWithRole(userID, orgID string, role domain.Role) *domain.User {
	return &domain.User{ID: userID, OrgID: orgID, Role: role}
}

func member(projectID, userID string, role domain.Role) *domain.ProjectMember {
	return &domain.ProjectMember{ProjectID: projectID, UserID: userID, Role: role}
}

func task(id, orgID, projectID string) *domain.Task {
	return &domain.Task{ID: id, OrgID: orgID, ProjectID: projectID}
}

func TestRequireTaskAccess_ElevatedRole_Allowed(t *testing.T) {
	chk := &accessCheckerImpl{
		projRepo: &testProjRepo{
			getByIDFn: func(ctx context.Context, orgID, id string) (*domain.Project, error) {
				return proj(id, orgID), nil
			},
		},
		userRepo: &testUserRepo{
			getByIDFn: func(ctx context.Context, orgID, id string) (*domain.User, error) {
				return userWithRole(id, orgID, domain.RoleMember), nil
			},
		},
		taskRepo: &testTaskRepo{
			getByIDAndOrgFn: func(ctx context.Context, orgID, id string) (*domain.Task, error) {
				return task(id, orgID, "proj-1"), nil
			},
		},
	}

	err := chk.RequireTaskAccess(context.Background(), "user-1", "org-1", "task-1", domain.PermTaskEdit)
	if err != nil {
		t.Fatalf("RequireTaskAccess for elevated role: %v", err)
	}
}

func TestRequireProjectAccess_ElevatedRole_Allowed(t *testing.T) {
	chk := &accessCheckerImpl{
		projRepo: &testProjRepo{
			getByIDFn: func(ctx context.Context, orgID, id string) (*domain.Project, error) {
				return proj(id, orgID), nil
			},
		},
		userRepo: &testUserRepo{
			getByIDFn: func(ctx context.Context, orgID, id string) (*domain.User, error) {
				return userWithRole(id, orgID, domain.RoleMember), nil
			},
		},
		taskRepo: nil, // not needed for this check
	}

	err := chk.RequireProjectAccess(context.Background(), "user-1", "org-1", "proj-1", domain.PermTaskEdit)
	if err != nil {
		t.Fatalf("RequireProjectAccess for elevated role: %v", err)
	}
}

func TestRequireProjectAccess_ViewerWithMembership_Allowed(t *testing.T) {
	chk := &accessCheckerImpl{
		projRepo: &testProjRepo{
			getByIDFn: func(ctx context.Context, orgID, id string) (*domain.Project, error) {
				return proj(id, orgID), nil
			},
		},
		userRepo: &testUserRepo{
			getByIDFn: func(ctx context.Context, orgID, id string) (*domain.User, error) {
				return userWithRole(id, orgID, domain.RoleViewer), nil
			},
		},
		pmRepo: &testPMRepo{
			getFn: func(ctx context.Context, orgID, projectID, userID string) (*domain.ProjectMember, error) {
				return member(projectID, userID, domain.RoleViewer), nil
			},
		},
		taskRepo: nil,
	}

	// Viewer with project membership checking PermProjectView (viewer HAS this perm).
	err := chk.RequireProjectAccess(context.Background(), "user-1", "org-1", "proj-1", domain.PermProjectView)
	if err != nil {
		t.Fatalf("RequireProjectAccess for viewer with membership: %v", err)
	}
}

func TestRequireProjectAccess_ViewerWithoutMembership_Denied(t *testing.T) {
	chk := &accessCheckerImpl{
		projRepo: &testProjRepo{
			getByIDFn: func(ctx context.Context, orgID, id string) (*domain.Project, error) {
				return proj(id, orgID), nil
			},
		},
		userRepo: &testUserRepo{
			getByIDFn: func(ctx context.Context, orgID, id string) (*domain.User, error) {
				return userWithRole(id, orgID, domain.RoleViewer), nil
			},
		},
		pmRepo: &testPMRepo{
			getFn: func(ctx context.Context, orgID, projectID, userID string) (*domain.ProjectMember, error) {
				return nil, errors.New("not found")
			},
		},
		taskRepo: nil,
	}

	err := chk.RequireProjectAccess(context.Background(), "user-1", "org-1", "proj-1", domain.PermTaskView)
	if err == nil {
		t.Fatal("RequireProjectAccess for viewer without membership: expected error")
	}
	if !errors.Is(err, apperr.ErrForbidden) {
		t.Fatalf("expected ErrForbidden, got %v", err)
	}
}

func TestRequireProjectAccess_ProjectNotInOrg_Denied(t *testing.T) {
	chk := &accessCheckerImpl{
		projRepo: &testProjRepo{
			getByIDFn: func(ctx context.Context, orgID, id string) (*domain.Project, error) {
				return nil, errors.New("not found")
			},
		},
		userRepo: &testUserRepo{
			getByIDFn: func(ctx context.Context, orgID, id string) (*domain.User, error) {
				return userWithRole(id, orgID, domain.RoleMember), nil
			},
		},
	}

	err := chk.RequireProjectAccess(context.Background(), "user-1", "org-1", "proj-unknown", domain.PermTaskEdit)
	if err == nil {
		t.Fatal("RequireProjectAccess for project not in org: expected error")
	}
	if !errors.Is(err, apperr.ErrForbidden) {
		t.Fatalf("expected ErrForbidden, got %v", err)
	}
}

func TestRequireOrgAccess_ElevatedRole_Allowed(t *testing.T) {
	chk := &accessCheckerImpl{
		userRepo: &testUserRepo{
			getByIDFn: func(ctx context.Context, orgID, id string) (*domain.User, error) {
				return userWithRole(id, orgID, domain.RoleAdmin), nil
			},
		},
	}

	err := chk.RequireOrgAccess(context.Background(), "user-1", "org-1", domain.PermProjectCreate)
	if err != nil {
		t.Fatalf("RequireOrgAccess for elevated role: %v", err)
	}
}

// TestRequireOrgAccess_MemberDeniedOrgPermission guards against the old
// elevated-role shortcut that let `member` pass ANY org permission check.
func TestRequireOrgAccess_MemberDeniedOrgPermission(t *testing.T) {
	chk := &accessCheckerImpl{
		userRepo: &testUserRepo{
			getByIDFn: func(ctx context.Context, orgID, id string) (*domain.User, error) {
				return userWithRole(id, orgID, domain.RoleMember), nil
			},
		},
	}

	// Members can create tasks but must never pass an org-management check.
	if err := chk.RequireOrgAccess(context.Background(), "user-1", "org-1", domain.PermTaskCreate); err != nil {
		t.Fatalf("member should keep PermTaskCreate: %v", err)
	}
	err := chk.RequireOrgAccess(context.Background(), "user-1", "org-1", domain.PermOrgDelete)
	if !errors.Is(err, apperr.ErrForbidden) {
		t.Fatalf("member with PermOrgDelete should be forbidden, got %v", err)
	}
}

func TestRequireOrgAccess_ViewerWithoutPermission_Denied(t *testing.T) {
	chk := &accessCheckerImpl{
		userRepo: &testUserRepo{
			getByIDFn: func(ctx context.Context, orgID, id string) (*domain.User, error) {
				return userWithRole(id, orgID, domain.RoleViewer), nil
			},
		},
	}

	// Viewer does NOT have PermProjectCreate.
	err := chk.RequireOrgAccess(context.Background(), "user-1", "org-1", domain.PermProjectCreate)
	if err == nil {
		t.Fatal("RequireOrgAccess for viewer without permission: expected error")
	}
	if !errors.Is(err, apperr.ErrForbidden) {
		t.Fatalf("expected ErrForbidden, got %v", err)
	}
}

func TestRequireProjectAccess_NilChecker_Denied(t *testing.T) {
	var chk *accessCheckerImpl

	err := chk.RequireProjectAccess(context.Background(), "user-1", "org-1", "proj-1", domain.PermTaskEdit)
	if err == nil {
		t.Fatal("nil checker must return ErrForbidden, got nil")
	}
	if !errors.Is(err, apperr.ErrForbidden) {
		t.Fatalf("nil checker: expected ErrForbidden, got %v", err)
	}
}

func TestRequireProjectAccess_NilRepos_Denied(t *testing.T) {
	chk := &accessCheckerImpl{}

	err := chk.RequireProjectAccess(context.Background(), "user-1", "org-1", "proj-1", domain.PermTaskEdit)
	if err == nil {
		t.Fatal("checker with nil repos must return ErrForbidden, got nil")
	}
	if !errors.Is(err, apperr.ErrForbidden) {
		t.Fatalf("checker with nil repos: expected ErrForbidden, got %v", err)
	}
}

func TestRequireOrgAccess_NilChecker_Denied(t *testing.T) {
	var chk *accessCheckerImpl

	err := chk.RequireOrgAccess(context.Background(), "user-1", "org-1", domain.PermProjectCreate)
	if err == nil {
		t.Fatal("nil checker must return ErrForbidden, got nil")
	}
	if !errors.Is(err, apperr.ErrForbidden) {
		t.Fatalf("nil checker: expected ErrForbidden, got %v", err)
	}
}

func TestRequireOrgAccess_NilUserRepo_Denied(t *testing.T) {
	chk := &accessCheckerImpl{}

	err := chk.RequireOrgAccess(context.Background(), "user-1", "org-1", domain.PermProjectCreate)
	if err == nil {
		t.Fatal("checker with nil repos must return ErrForbidden, got nil")
	}
	if !errors.Is(err, apperr.ErrForbidden) {
		t.Fatalf("checker with nil repos: expected ErrForbidden, got %v", err)
	}
}

func TestEnsureProjectAccessCachedRoleIsProjectScoped(t *testing.T) {
	orgID := "org-1"
	userID := "u-1"
	sourceProject := "proj-source"
	targetProject := "proj-target"

	// Viewer/guest org role: access comes from project_members only.
	calls := 0
	pmRepo := &testPMRepo{
		getFn: func(ctx context.Context, orgID, projectID, userID string) (*domain.ProjectMember, error) {
			calls++
			if projectID == sourceProject {
				return member(projectID, userID, domain.RoleAdmin), nil
			}
			return nil, apperr.ErrNotFound
		},
	}
	projRepo := &testProjRepo{getByIDFn: func(ctx context.Context, orgID, id string) (*domain.Project, error) {
		return proj(id, orgID), nil
	}}

	svc := NewAccessService(pmRepo, projRepo, nil, nil)

	// Simulate the middleware having resolved the role for the SOURCE project.
	ctx := context.WithValue(context.Background(), domain.CtxEffectiveRole, domain.RoleAdmin)
	ctx = context.WithValue(ctx, domain.CtxEffectiveRoleProjectID, sourceProject)

	if err := svc.EnsureProjectAccess(ctx, orgID, userID, domain.RoleViewer, sourceProject); err != nil {
		t.Fatalf("source project (cached) should fast-path: %v", err)
	}

	err := svc.EnsureProjectAccess(ctx, orgID, userID, domain.RoleViewer, targetProject)
	if err == nil {
		t.Fatal("target project must not be authorized by a role cached for a different project")
	}
	if calls == 0 {
		t.Fatal("expected the pm repo to be consulted for the uncached target project")
	}
}
