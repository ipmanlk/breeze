package service

import (
	"context"
	"testing"
	"time"

	"ipmanlk/breeze/internal/apperr"
	"ipmanlk/breeze/internal/auth"
	"ipmanlk/breeze/internal/domain"
	"ipmanlk/breeze/internal/storage"
)

func TestInviteService_Create_RoleCap(t *testing.T) {
	userRepo := newMockUserRepo()
	inviteRepo := newMockInviteRepo()
	svc := NewInviteService(userRepo, newMockAccountRepo(), inviteRepo, newMockProjectMemberRepo(), newMockOrgRepo(false), newMockProjectRepo(), newMockMailer(false), "", nil)

	cases := []struct {
		name       string
		callerRole domain.Role
		targetRole domain.Role
		wantErr    error
	}{
		{"owner invites admin", domain.RoleOwner, domain.RoleAdmin, nil},
		{"owner invites owner", domain.RoleOwner, domain.RoleOwner, nil},
		{"admin invites admin", domain.RoleAdmin, domain.RoleAdmin, nil},
		{"admin invites owner", domain.RoleAdmin, domain.RoleOwner, apperr.ErrForbidden},
		{"member invites member", domain.RoleMember, domain.RoleMember, nil},
		{"member invites admin", domain.RoleMember, domain.RoleAdmin, apperr.ErrForbidden},
		{"member invites viewer", domain.RoleMember, domain.RoleViewer, nil},
		{"viewer invites anyone", domain.RoleViewer, domain.RoleMember, apperr.ErrForbidden},
	}

	for _, tc := range cases {
		t.Run(tc.name, func(t *testing.T) {
			_, _, err := svc.Create(context.Background(), domain.CreateInviteParams{
				OrgID: "org-1", InvitedBy: "u1", Role: tc.targetRole,
			}, tc.callerRole)
			if err != tc.wantErr {
				t.Errorf("Create() err = %v, want %v", err, tc.wantErr)
			}
		})
	}
}

func TestInviteService_Accept_Success(t *testing.T) {
	userRepo := newMockUserRepo()
	inviteRepo := newMockInviteRepo()
	svc := NewInviteService(userRepo, newMockAccountRepo(), inviteRepo, newMockProjectMemberRepo(), newMockOrgRepo(false), newMockProjectRepo(), newMockMailer(false), "", nil)

	invite, token, err := svc.Create(context.Background(), domain.CreateInviteParams{
		OrgID: "org-1", InvitedBy: "u1", Role: domain.RoleMember,
	}, domain.RoleOwner)
	if err != nil {
		t.Fatalf("Create() err = %v", err)
	}

	user, _, err := svc.Accept(context.Background(), domain.AcceptInviteParams{
		Token:    token,
		Name:     "Jane",
		Email:    "jane@test.com",
		Password: "password123",
	})
	if err != nil {
		t.Fatalf("Accept() err = %v", err)
	}
	if user == nil {
		t.Fatal("Accept() user is nil")
	}
	if user.Name != "Jane" {
		t.Errorf("Name = %q, want Jane", user.Name)
	}
	if user.Role != domain.RoleMember {
		t.Errorf("Role = %q, want member", user.Role)
	}
	if !user.IsActive {
		t.Error("IsActive = false, want true")
	}

	// invite should be consumed
	i, err := inviteRepo.GetByTokenHash(context.Background(), invite.TokenHash)
	if err != nil {
		t.Fatalf("GetByTokenHash() err = %v", err)
	}
	if i.UseCount != 1 {
		t.Errorf("UseCount = %d, want 1", i.UseCount)
	}
}

func TestInviteService_Accept_EmailMismatch(t *testing.T) {
	userRepo := newMockUserRepo()
	inviteRepo := newMockInviteRepo()
	svc := NewInviteService(userRepo, newMockAccountRepo(), inviteRepo, newMockProjectMemberRepo(), newMockOrgRepo(false), newMockProjectRepo(), newMockMailer(false), "", nil)

	email := "restricted@test.com"
	_, token, err := svc.Create(context.Background(), domain.CreateInviteParams{
		OrgID: "org-1", InvitedBy: "u1", Role: domain.RoleMember, Email: &email,
	}, domain.RoleOwner)
	if err != nil {
		t.Fatalf("Create() err = %v", err)
	}

	_, _, err = svc.Accept(context.Background(), domain.AcceptInviteParams{
		Token:    token,
		Name:     "Jane",
		Email:    "other@test.com",
		Password: "password123",
	})
	if err != apperr.ErrForbidden {
		t.Errorf("Accept() err = %v, want ErrForbidden", err)
	}
}

func TestInviteService_Accept_AlreadyExists(t *testing.T) {
	userRepo := newMockUserRepo()
	inviteRepo := newMockInviteRepo()
	svc := NewInviteService(userRepo, newMockAccountRepo(), inviteRepo, newMockProjectMemberRepo(), newMockOrgRepo(false), newMockProjectRepo(), newMockMailer(false), "", nil)

	// seed existing user
	userRepo.usersByID["u1"] = &domain.User{ID: "u1", AccountID: "acct-1", OrgID: "org-1", Email: "exists@test.com", Name: "Existing", Role: domain.RoleMember, IsActive: true}
	userRepo.usersByEmail["exists@test.com"] = userRepo.usersByID["u1"]

	_, token, err := svc.Create(context.Background(), domain.CreateInviteParams{
		OrgID: "org-1", InvitedBy: "u1", Role: domain.RoleMember,
	}, domain.RoleOwner)
	if err != nil {
		t.Fatalf("Create() err = %v", err)
	}

	_, _, err = svc.Accept(context.Background(), domain.AcceptInviteParams{
		Token:    token,
		Name:     "Jane",
		Email:    "exists@test.com",
		Password: "password123",
	})
	if err != apperr.ErrAlreadyExists {
		t.Errorf("Accept() err = %v, want ErrAlreadyExists", err)
	}
}

func TestInviteService_Accept_Expired(t *testing.T) {
	userRepo := newMockUserRepo()
	inviteRepo := newMockInviteRepo()
	svc := NewInviteService(userRepo, newMockAccountRepo(), inviteRepo, newMockProjectMemberRepo(), newMockOrgRepo(false), newMockProjectRepo(), newMockMailer(false), "", nil)

	_, token, err := svc.Create(context.Background(), domain.CreateInviteParams{
		OrgID: "org-1", InvitedBy: "u1", Role: domain.RoleMember,
	}, domain.RoleOwner)
	if err != nil {
		t.Fatalf("Create() err = %v", err)
	}

	// manually expire
	invite, _ := inviteRepo.GetByTokenHash(context.Background(), svc.token.HashToken(token))
	invite.ExpiresAt = invite.ExpiresAt.Add(-8 * 24 * time.Hour) // expired a day ago

	_, _, err = svc.Accept(context.Background(), domain.AcceptInviteParams{
		Token:    token,
		Name:     "Jane",
		Email:    "jane@test.com",
		Password: "password123",
	})
	if err != apperr.ErrSessionExpired {
		t.Errorf("Accept() err = %v, want ErrSessionExpired", err)
	}
}

func TestUserService_UpdateRole_Guards(t *testing.T) {
	userRepo := newMockUserRepo()
	sessionRepo := newMockSessionRepo()
	accountRepo := newMockAccountRepo()
	store := storage.NewLocal(t.TempDir())
	svc := NewUserService(userRepo, sessionRepo, accountRepo, store)

	// seed: owner1, admin1, member1
	owner := &domain.User{ID: "owner1", OrgID: "org-1", Name: "Owner", Email: "o@test.com", Role: domain.RoleOwner, IsActive: true}
	admin := &domain.User{ID: "admin1", OrgID: "org-1", Name: "Admin", Email: "a@test.com", Role: domain.RoleAdmin, IsActive: true}
	member := &domain.User{ID: "member1", OrgID: "org-1", Name: "Member", Email: "m@test.com", Role: domain.RoleMember, IsActive: true}
	for _, u := range []*domain.User{owner, admin, member} {
		userRepo.usersByID[u.ID] = u
		userRepo.usersByEmail[u.Email] = u
	}

	cases := []struct {
		name       string
		targetID   string
		callerID   string
		callerRole domain.Role
		newRole    domain.Role
		wantErr    bool
	}{
		{"self-change rejected", "admin1", "admin1", domain.RoleAdmin, domain.RoleMember, true},
		{"admin demotes owner rejected", "owner1", "admin1", domain.RoleAdmin, domain.RoleMember, true},
		{"admin promotes to owner rejected", "member1", "admin1", domain.RoleAdmin, domain.RoleOwner, true},
		{"owner promotes member to admin", "member1", "owner1", domain.RoleOwner, domain.RoleAdmin, false},
		{"owner demotes admin to member", "admin1", "owner1", domain.RoleOwner, domain.RoleMember, false},
		{"admin promotes member to admin", "member1", "admin1", domain.RoleAdmin, domain.RoleAdmin, false},
	}

	for _, tc := range cases {
		t.Run(tc.name, func(t *testing.T) {
			err := svc.UpdateRole(context.Background(), "org-1", tc.targetID, tc.newRole, tc.callerRole, tc.callerID)
			if tc.wantErr {
				if err == nil {
					t.Error("expected error, got nil")
				}
			} else {
				if err != nil {
					t.Errorf("unexpected error: %v", err)
				}
			}
		})
	}
}

func TestUserService_UpdateActive_Guards(t *testing.T) {
	userRepo := newMockUserRepo()
	sessionRepo := newMockSessionRepo()
	accountRepo := newMockAccountRepo()
	store := storage.NewLocal(t.TempDir())
	svc := NewUserService(userRepo, sessionRepo, accountRepo, store)

	owner := &domain.User{ID: "owner1", OrgID: "org-1", Name: "Owner", Email: "o@test.com", Role: domain.RoleOwner, IsActive: true}
	userRepo.usersByID["owner1"] = owner
	userRepo.usersByEmail["o@test.com"] = owner

	err := svc.UpdateActive(context.Background(), "org-1", "owner1", false, "owner1")
	if err != apperr.ErrForbidden {
		t.Errorf("self-deactivation err = %v, want ErrForbidden", err)
	}

	// can't deactivate last owner
	err = svc.UpdateActive(context.Background(), "org-1", "owner1", false, "admin1")
	if err == nil {
		t.Error("expected error deactivating last owner, got nil")
	}
}

func TestAuthService_Login_Deactivated(t *testing.T) {
	userRepo := newMockUserRepo()
	accountRepo := newMockAccountRepo()
	sessionRepo := newMockSessionRepo()
	svc := NewAuthService(accountRepo, userRepo, sessionRepo, newMockPasswordResetRepo(), newMockMailer(false), "", "test-secret", nil)

	hash, _ := auth.HashPassword("pw")
	accountRepo.accountsByEmail["d@test.com"] = &domain.Account{ID: "acct-1", Email: "d@test.com", PasswordHash: hash}
	accountRepo.accountsByID["acct-1"] = accountRepo.accountsByEmail["d@test.com"]
	userRepo.usersByID["u1"] = &domain.User{ID: "u1", AccountID: "acct-1", OrgID: "org-1", Email: "d@test.com", Name: "Deactivated", Role: domain.RoleMember, IsActive: false}
	userRepo.usersByEmail["d@test.com"] = userRepo.usersByID["u1"]

	_, _, _, err := svc.Login(context.Background(), domain.LoginParams{Email: "d@test.com", Password: "pw"})
	if err != apperr.ErrUserDeactivated {
		t.Errorf("Login() err = %v, want ErrUserDeactivated", err)
	}
}

type mockInviteRepo struct {
	invitesByHash map[string]*domain.UserInvite
	invitesByID   map[string]*domain.UserInvite
}

func newMockInviteRepo() *mockInviteRepo {
	return &mockInviteRepo{
		invitesByHash: make(map[string]*domain.UserInvite),
		invitesByID:   make(map[string]*domain.UserInvite),
	}
}

func (m *mockInviteRepo) Create(ctx context.Context, invite *domain.UserInvite) error {
	i := *invite
	m.invitesByHash[i.TokenHash] = &i
	m.invitesByID[i.ID] = &i
	return nil
}

func (m *mockInviteRepo) GetByTokenHash(ctx context.Context, hash string) (*domain.UserInvite, error) {
	i, ok := m.invitesByHash[hash]
	if !ok {
		return nil, apperr.ErrNotFound
	}
	return i, nil
}

func (m *mockInviteRepo) ListByOrg(ctx context.Context, orgID string, limit int) ([]*domain.UserInvite, error) {
	return nil, nil
}

func (m *mockInviteRepo) Delete(ctx context.Context, orgID, id string) error {
	delete(m.invitesByID, id)
	return nil
}

func (m *mockInviteRepo) IncrementUseCount(ctx context.Context, id string) error {
	i, ok := m.invitesByID[id]
	if !ok {
		return apperr.ErrNotFound
	}
	i.UseCount++
	return nil
}

func (m *mockInviteRepo) RecordAcceptance(ctx context.Context, inviteID, userID string) error {
	return nil
}

func (m *mockInviteRepo) AddInviteProject(ctx context.Context, inviteID, projectID string, role domain.Role) error {
	return nil
}

func (m *mockInviteRepo) ListInviteProjects(ctx context.Context, inviteID string) ([]*domain.InviteProjectAssignment, error) {
	return nil, nil
}

func (m *mockInviteRepo) DeleteInviteProjects(ctx context.Context, inviteID string) error {
	return nil
}

func (m *mockInviteRepo) AcceptInvite(ctx context.Context, inviteID, userID string) error {
	// Mimic the atomic store method: increment use count + record acceptance
	return m.IncrementUseCount(ctx, inviteID)
}

// TestInviteService_Create_RejectsCrossOrgProjectAssignment verifies that a
// project assignment whose ProjectID belongs to a different org is
// rejected. Without this, a malicious inviter could grant membership in a
// foreign-org project via the invite's project_assignments.
func TestInviteService_Create_RejectsCrossOrgProjectAssignment(t *testing.T) {
	userRepo := newMockUserRepo()
	inviteRepo := newMockInviteRepo()
	projRepo := newMockProjectRepo()
	// proj-1 is in org-1 (the invite's org); proj-2 is in org-2 (foreign).
	projRepo.projectsByID["proj-1"] = &domain.Project{ID: "proj-1", OrgID: "org-1"}
	projRepo.projectsByID["proj-2"] = &domain.Project{ID: "proj-2", OrgID: "org-2"}
	svc := NewInviteService(userRepo, newMockAccountRepo(), inviteRepo, newMockProjectMemberRepo(), newMockOrgRepo(false), projRepo, newMockMailer(false), "", nil)

	// Same-org project assignment → OK.
	_, _, err := svc.Create(context.Background(), domain.CreateInviteParams{
		OrgID: "org-1", InvitedBy: "u1", Role: domain.RoleMember,
		ProjectAssignments: []domain.InviteProjectAssignment{
			{ProjectID: "proj-1", Role: domain.RoleMember},
		},
	}, domain.RoleOwner)
	if err != nil {
		t.Fatalf("same-org assignment should succeed, got %v", err)
	}

	// Cross-org project assignment → rejected.
	_, _, err = svc.Create(context.Background(), domain.CreateInviteParams{
		OrgID: "org-1", InvitedBy: "u1", Role: domain.RoleMember,
		ProjectAssignments: []domain.InviteProjectAssignment{
			{ProjectID: "proj-2", Role: domain.RoleMember}, // foreign org
		},
	}, domain.RoleOwner)
	if err == nil {
		t.Fatal("expected error for cross-org project assignment, got nil")
	}
}
