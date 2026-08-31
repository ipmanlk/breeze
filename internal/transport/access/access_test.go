package access

import (
	"context"
	"errors"
	"testing"

	"ipmanlk/plume/internal/domain"
	"ipmanlk/plume/internal/port"
)

// pmMock is a minimal ProjectMemberRepository whose Get returns a fixed
// membership (or none). Only Get is exercised by ResolveEffectiveRole.
type pmMock struct {
	member *domain.ProjectMember
	err    error
}

func (m *pmMock) List(context.Context, string, string, domain.UserFilter) (*domain.ProjectMemberListResult, error) {
	return nil, errors.New("not implemented")
}

func (m *pmMock) Get(_ context.Context, _, _, _ string) (*domain.ProjectMember, error) {
	if m.err != nil {
		return nil, m.err
	}
	return m.member, nil
}

func (m *pmMock) Add(context.Context, string, string, domain.Role) error        { return nil }
func (m *pmMock) Remove(context.Context, string, string) error                  { return nil }
func (m *pmMock) UpdateRole(context.Context, string, string, domain.Role) error { return nil }
func (m *pmMock) ListByUser(context.Context, string, string) ([]*domain.UserProjectMembership, error) {
	return nil, nil
}

func (m *pmMock) SetMemberships(_ context.Context, _ string, _ string, _ []domain.ProjectAssignment) error {
	return nil
}

type projMock struct{}

func (m *projMock) GetByID(_ context.Context, _, _ string) (*domain.Project, error) {
	return &domain.Project{}, nil
}

func (m *projMock) List(context.Context, string) ([]*domain.Project, error) {
	return nil, errors.New("not implemented")
}

func (m *projMock) Create(context.Context, *domain.Project) error {
	return errors.New("not implemented")
}

func (m *projMock) ListForUser(context.Context, string, string) ([]*domain.Project, error) {
	return nil, errors.New("not implemented")
}

func (m *projMock) ListIncludingArchived(context.Context, string) ([]*domain.Project, error) {
	return nil, errors.New("not implemented")
}

func (m *projMock) ListForUserIncludingArchived(context.Context, string, string) ([]*domain.Project, error) {
	return nil, errors.New("not implemented")
}

func (m *projMock) ListByIDs(context.Context, string, []string) ([]*domain.Project, error) {
	return nil, errors.New("not implemented")
}

func (m *projMock) Update(context.Context, *domain.Project) error {
	return errors.New("not implemented")
}

func (m *projMock) Delete(context.Context, string, string) error {
	return errors.New("not implemented")
}

func (m *projMock) SetArchived(_ context.Context, _ string, _ string, _ bool) error {
	return errors.New("not implemented")
}

func (m *projMock) GetBySlug(context.Context, string, string) (*domain.Project, error) {
	return nil, errors.New("not implemented")
}

func (m *projMock) CreateWithStatuses(_ context.Context, _ *domain.Project, _ []*domain.TaskStatus) error {
	return errors.New("not implemented")
}

var _ port.ProjectRepository = (*projMock)(nil)

func ctxWith(role, userID string) context.Context {
	ctx := context.Background()
	ctx = context.WithValue(ctx, domain.CtxRole, role)
	ctx = context.WithValue(ctx, domain.CtxUserID, userID)
	ctx = context.WithValue(ctx, domain.CtxOrgID, "test-org")
	return ctx
}

func TestIsOrgElevated(t *testing.T) {
	cases := map[domain.Role]bool{
		domain.RoleOwner:  true,
		domain.RoleAdmin:  true,
		domain.RoleMember: true,
		domain.RoleViewer: false,
		domain.RoleGuest:  false,
	}
	for role, want := range cases {
		if got := IsOrgElevated(role); got != want {
			t.Errorf("IsOrgElevated(%s) = %v, want %v", role, got, want)
		}
	}
}

func TestEffectiveRoleFor(t *testing.T) {
	// Org-elevated users always keep their org role; the project role is
	// ignored (no confusing override for admins/owners).
	if got := EffectiveRoleFor(domain.RoleAdmin, domain.RoleViewer); got != domain.RoleAdmin {
		t.Errorf("admin override = %s, want admin (ignored)", got)
	}
	// Project-scoped users use their per-project role.
	if got := EffectiveRoleFor(domain.RoleGuest, domain.RoleAdmin); got != domain.RoleAdmin {
		t.Errorf("guest->project admin = %s, want admin", got)
	}
}

func TestResolveEffectiveRole(t *testing.T) {
	t.Run("org elevated uses org role without db lookup", func(t *testing.T) {
		// pmRepo is nil; elevated roles must not touch it.
		for _, role := range []domain.Role{domain.RoleOwner, domain.RoleAdmin, domain.RoleMember} {
			got, err := ResolveEffectiveRole(ctxWith(string(role), "u1"), nil, &projMock{}, "p1")
			if err != nil {
				t.Fatalf("%s: unexpected err %v", role, err)
			}
			if got != role {
				t.Errorf("%s: got %s, want %s", role, got, role)
			}
		}
	})

	t.Run("viewer with membership returns project role", func(t *testing.T) {
		repo := &pmMock{member: &domain.ProjectMember{Role: domain.RoleAdmin}}
		got, err := ResolveEffectiveRole(ctxWith("viewer", "u1"), repo, &projMock{}, "p1")
		if err != nil {
			t.Fatalf("unexpected err %v", err)
		}
		if got != domain.RoleAdmin {
			t.Errorf("got %s, want admin (project override)", got)
		}
	})

	t.Run("guest without membership is rejected", func(t *testing.T) {
		repo := &pmMock{member: nil}
		_, err := ResolveEffectiveRole(ctxWith("guest", "u1"), repo, &projMock{}, "p1")
		if !errors.Is(err, ErrNotProjectMember) {
			t.Fatalf("got %v, want ErrNotProjectMember", err)
		}
	})

	t.Run("viewer with empty project role is rejected", func(t *testing.T) {
		repo := &pmMock{member: &domain.ProjectMember{Role: ""}}
		_, err := ResolveEffectiveRole(ctxWith("viewer", "u1"), repo, &projMock{}, "p1")
		if !errors.Is(err, ErrNotProjectMember) {
			t.Fatalf("got %v, want ErrNotProjectMember", err)
		}
	})

	t.Run("missing role in context is forbidden", func(t *testing.T) {
		_, err := ResolveEffectiveRole(context.Background(), &pmMock{}, &projMock{}, "p1")
		if !errors.Is(err, ErrForbidden) {
			t.Fatalf("got %v, want ErrForbidden", err)
		}
	})
}

// Compile-time check that pmMock satisfies the port interface.
var _ port.ProjectMemberRepository = (*pmMock)(nil)
