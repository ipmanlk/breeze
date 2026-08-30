package service

import (
	"context"
	"errors"
	"testing"

	"ipmanlk/breeze/internal/apperr"
	"ipmanlk/breeze/internal/domain"
)

func TestOrganizationService_Create_Success(t *testing.T) {
	orgRepo := newMockOrgRepo(false)
	userRepo := newMockUserRepo()
	accountRepo := newMockAccountRepo()
	sessionRepo := newMockSessionRepo()
	svc := NewOrganizationService(orgRepo, userRepo, accountRepo, sessionRepo, testJWTSecret)

	org, user, err := svc.Create(context.Background(), "My Org", "Alice", "alice@example.com", "secure-password")
	if err != nil {
		t.Fatalf("Create() error = %v", err)
	}

	if org == nil {
		t.Fatal("Create() org is nil")
	}
	if org.Name != "My Org" {
		t.Errorf("Create() org.Name = %q, want %q", org.Name, "My Org")
	}
	if org.Slug != "my-org" {
		t.Errorf("Create() org.Slug = %q, want %q", org.Slug, "my-org")
	}

	if user == nil {
		t.Fatal("Create() user is nil")
	}
	if user.Email != "alice@example.com" {
		t.Errorf("Create() user.Email = %q, want %q", user.Email, "alice@example.com")
	}
	if user.Role != domain.RoleOwner {
		t.Errorf("Create() user.Role = %q, want %q", user.Role, domain.RoleOwner)
	}

	if orgRepo.called < 1 {
		t.Errorf("orgRepo expected >= 1 calls, got %d", orgRepo.called)
	}
}

func TestOrganizationService_Create_AlreadyExists(t *testing.T) {
	orgRepo := newMockOrgRepo(true)
	userRepo := newMockUserRepo()
	accountRepo := newMockAccountRepo()
	sessionRepo := newMockSessionRepo()
	svc := NewOrganizationService(orgRepo, userRepo, accountRepo, sessionRepo, testJWTSecret)

	_, _, err := svc.Create(context.Background(), "My Org", "Alice", "alice@example.com", "secure-password")
	if !errors.Is(err, apperr.ErrSetupComplete) {
		t.Errorf("Create() error = %v, want ErrSetupComplete", err)
	}
}

func TestOrganizationService_Create_Slugifies(t *testing.T) {
	tests := []struct {
		name     string
		orgName  string
		wantSlug string
	}{
		{name: "simple", orgName: "My Org", wantSlug: "my-org"},
		{name: "multiple spaces", orgName: "Hello   World", wantSlug: "hello-world"},
		{name: "special chars", orgName: "Hello! World?", wantSlug: "hello-world"},
		{name: "numbers", orgName: "Team 42", wantSlug: "team-42"},
		{name: "underscores", orgName: "my_org_name", wantSlug: "my_org_name"},
		{name: "mixed case", orgName: "MyBig Org", wantSlug: "mybig-org"},
		{name: "only special chars", orgName: "@@@!!!", wantSlug: "org"},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			orgRepo := newMockOrgRepo(false)
			svc := NewOrganizationService(orgRepo, newMockUserRepo(), newMockAccountRepo(), newMockSessionRepo(), testJWTSecret)
			org, _, err := svc.Create(context.Background(), tt.orgName, "Admin", "admin@test.com", "password")
			if err != nil {
				t.Fatalf("Create() error = %v", err)
			}
			if org.Slug != tt.wantSlug {
				t.Errorf("Slug = %q, want %q", org.Slug, tt.wantSlug)
			}
		})
	}
}

func TestOrganizationService_CreateWorkspace_Success(t *testing.T) {
	orgRepo := newMockOrgRepo(false)
	userRepo := newMockUserRepo()
	accountRepo := newMockAccountRepo()
	sessionRepo := newMockSessionRepo()
	svc := NewOrganizationService(orgRepo, userRepo, accountRepo, sessionRepo, testJWTSecret)

	// Seed the caller's existing membership so CreateWorkspace can copy
	// display identity into the new owner membership.
	caller := &domain.User{
		ID: "user-1", AccountID: "acct-1", OrgID: "org-1",
		Email: "owner@test.com", Name: "Owner", Role: domain.RoleOwner, IsActive: true,
	}
	userRepo.usersByID[caller.ID] = caller

	org, user, err := svc.CreateWorkspace(context.Background(), "acct-1", "New Org", "Owner", "owner@test.com", nil)
	if err != nil {
		t.Fatalf("CreateWorkspace() error = %v", err)
	}
	if org == nil || org.Name != "New Org" {
		t.Fatalf("CreateWorkspace() org = %+v", org)
	}
	if user == nil || user.Role != domain.RoleOwner {
		t.Fatalf("CreateWorkspace() user role = %v, want owner", user.Role)
	}
	if user.AccountID != "acct-1" {
		t.Errorf("user.AccountID = %q, want acct-1", user.AccountID)
	}
}

func TestOrganizationService_ListForAccount(t *testing.T) {
	orgRepo := newMockOrgRepo(false)
	userRepo := newMockUserRepo()
	accountRepo := newMockAccountRepo()
	sessionRepo := newMockSessionRepo()
	svc := NewOrganizationService(orgRepo, userRepo, accountRepo, sessionRepo, testJWTSecret)

	// Pre-seed the account so Lookup succeeds.
	accountID := "test-account-123"
	_ = accountRepo.Create(ctx, &domain.Account{ID: accountID, Email: "alice@example.com", PasswordHash: "hash"})

	_, _, err := svc.Create(context.Background(), "First Org", "Alice", "alice@example.com", "pw")
	if err != nil {
		t.Fatalf("Create() error = %v", err)
	}

	// ListForAccount uses the mock which returns every seeded org as an
	// owner workspace.
	list, err := svc.ListForAccount(context.Background(), "any-account-id")
	if err != nil {
		t.Fatalf("ListForAccount() error = %v", err)
	}
	if len(list) == 0 {
		t.Fatal("ListForAccount() returned no workspaces")
	}
	if list[0].Organization.Name != "First Org" {
		t.Errorf("workspace org name = %q, want First Org", list[0].Organization.Name)
	}
	if !list[0].IsOwner {
		t.Error("workspace IsOwner = false, want true")
	}
}

func TestOrganizationService_SwitchWorkspace_NotMember(t *testing.T) {
	orgRepo := newMockOrgRepo(false)
	userRepo := newMockUserRepo()
	accountRepo := newMockAccountRepo()
	sessionRepo := newMockSessionRepo()
	svc := NewOrganizationService(orgRepo, userRepo, accountRepo, sessionRepo, testJWTSecret)

	// No membership seeded for acct-x in org-y.
	_, _, err := svc.SwitchWorkspace(context.Background(), "acct-x", "org-y", "")
	if !errors.Is(err, apperr.ErrNotFound) {
		t.Errorf("SwitchWorkspace() error = %v, want ErrNotFound", err)
	}
}

func TestOrganizationService_SwitchWorkspace_Success(t *testing.T) {
	orgRepo := newMockOrgRepo(false)
	userRepo := newMockUserRepo()
	// Link the orgRepo mock to the userRepo so CreateOrgWithAccountAndUser
	// also seeds the user in the userRepo mock.
	orgRepo.userRepo = userRepo
	accountRepo := newMockAccountRepo()
	sessionRepo := newMockSessionRepo()
	svc := NewOrganizationService(orgRepo, userRepo, accountRepo, sessionRepo, testJWTSecret)

	// Seed an org + owner membership via setup, then switch to that org.
	org, user, err := svc.Create(context.Background(), "My Org", "Alice", "alice@example.com", "pw")
	if err != nil {
		t.Fatalf("Create() error = %v", err)
	}

	session, token, err := svc.SwitchWorkspace(context.Background(), user.AccountID, org.ID, "old-session")
	if err != nil {
		t.Fatalf("SwitchWorkspace() error = %v", err)
	}
	if session == nil || token == "" {
		t.Fatal("SwitchWorkspace() returned nil session or empty token")
	}
	if session.OrgID != org.ID {
		t.Errorf("session.OrgID = %q, want %q", session.OrgID, org.ID)
	}
	if session.Role != domain.RoleOwner {
		t.Errorf("session.Role = %q, want owner", session.Role)
	}
}

func TestOrganizationService_Update_Success(t *testing.T) {
	orgRepo := newMockOrgRepo(false)
	userRepo := newMockUserRepo()
	accountRepo := newMockAccountRepo()
	sessionRepo := newMockSessionRepo()
	svc := NewOrganizationService(orgRepo, userRepo, accountRepo, sessionRepo, testJWTSecret)

	org, _, err := svc.Create(context.Background(), "Original Org", "Alice", "alice@example.com", "pw")
	if err != nil {
		t.Fatalf("Create() error = %v", err)
	}

	updated, err := svc.Update(context.Background(), org.ID, "Renamed Org", 45)
	if err != nil {
		t.Fatalf("Update() error = %v", err)
	}
	if updated.Name != "Renamed Org" {
		t.Errorf("updated.Name = %q, want Renamed Org", updated.Name)
	}
	if updated.Slug != "renamed-org" {
		t.Errorf("updated.Slug = %q, want renamed-org", updated.Slug)
	}
	if updated.MessageEditWindowMinute != 45 {
		t.Errorf("updated.MessageEditWindowMinute = %d, want 45", updated.MessageEditWindowMinute)
	}
}

func TestOrganizationService_Update_NameTooShort(t *testing.T) {
	orgRepo := newMockOrgRepo(false)
	svc := NewOrganizationService(orgRepo, newMockUserRepo(), newMockAccountRepo(), newMockSessionRepo(), testJWTSecret)

	org, _, _ := svc.Create(context.Background(), "My Org", "Alice", "alice@example.com", "pw")

	_, err := svc.Update(context.Background(), org.ID, "x", 0)
	if !errors.Is(err, apperr.ErrInvalidInput) {
		t.Errorf("Update() error = %v, want ErrInvalidInput", err)
	}
}

func TestOrganizationService_Update_NotFound(t *testing.T) {
	orgRepo := newMockOrgRepo(false)
	svc := NewOrganizationService(orgRepo, newMockUserRepo(), newMockAccountRepo(), newMockSessionRepo(), testJWTSecret)

	_, err := svc.Update(context.Background(), "missing-org", "New Name", 0)
	if err == nil {
		t.Error("Update() expected error for missing org")
	}
}

func TestOrganizationService_Delete_Success(t *testing.T) {
	orgRepo := newMockOrgRepo(false)
	svc := NewOrganizationService(orgRepo, newMockUserRepo(), newMockAccountRepo(), newMockSessionRepo(), testJWTSecret)

	org, _, _ := svc.Create(context.Background(), "Doomed Org", "Alice", "alice@example.com", "pw")

	if err := svc.Delete(context.Background(), org.ID, "Doomed Org"); err != nil {
		t.Fatalf("Delete() error = %v", err)
	}

	// The org must be gone from the repo.
	if _, err := orgRepo.GetByID(context.Background(), org.ID); err == nil {
		t.Error("org still exists after Delete")
	}
}

func TestOrganizationService_Delete_NotFound(t *testing.T) {
	orgRepo := newMockOrgRepo(false)
	svc := NewOrganizationService(orgRepo, newMockUserRepo(), newMockAccountRepo(), newMockSessionRepo(), testJWTSecret)

	if err := svc.Delete(context.Background(), "missing-org", "Anything"); err == nil {
		t.Error("Delete() expected error for missing org")
	}
}
