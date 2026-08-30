package service

import (
	"context"
	"errors"
	"strings"
	"testing"

	"ipmanlk/breeze/internal/apperr"
	"ipmanlk/breeze/internal/auth"
	"ipmanlk/breeze/internal/domain"
	"ipmanlk/breeze/internal/storage"
)

// newTestUserService wires a UserService with the standard mocks + a temp-dir
// local storage backend. accountRepo and userRepo are returned so the caller
// can seed data.
func newTestUserService(t *testing.T) (svc *UserService, userRepo *mockUserRepo, accountRepo *mockAccountRepo, sessionRepo *mockSessionRepo) {
	t.Helper()
	userRepo = newMockUserRepo()
	accountRepo = newMockAccountRepo()
	sessionRepo = newMockSessionRepo()
	store := storage.NewLocal(t.TempDir())
	svc = NewUserService(userRepo, sessionRepo, accountRepo, store)
	return
}

func TestUserService_UpdateProfile_Success(t *testing.T) {
	svc, userRepo, accountRepo, _ := newTestUserService(t)

	user, acct := addTestUserWithAccount(userRepo, accountRepo, "acct-1", "user-1", "org-1", "Original", "orig@x.com", "pw")

	updated, err := svc.UpdateProfile(context.Background(), user.OrgID, user.ID, "New Name", nil)
	if err != nil {
		t.Fatalf("UpdateProfile() error = %v", err)
	}
	if updated.Name != "New Name" {
		t.Errorf("updated.Name = %q, want New Name", updated.Name)
	}

	// The display name must be synced across ALL of the account's memberships.
	for _, u := range userRepo.usersByID {
		if u.AccountID == acct.ID && u.Name != "New Name" {
			t.Errorf("membership %s name = %q, want synced New Name", u.ID, u.Name)
		}
	}
}

func TestUserService_UpdateProfile_EmptyName(t *testing.T) {
	svc, userRepo, _, _ := newTestUserService(t)
	addTestUserWithAccount(userRepo, newMockAccountRepo(), "acct-1", "user-1", "org-1", "Original", "o@x.com", "pw")

	_, err := svc.UpdateProfile(context.Background(), "org-1", "user-1", "   ", nil)
	if !errors.Is(err, apperr.ErrInvalidInput) {
		t.Errorf("UpdateProfile() error = %v, want ErrInvalidInput", err)
	}
}

func TestUserService_UpdateProfile_NotFound(t *testing.T) {
	svc, _, _, _ := newTestUserService(t)

	_, err := svc.UpdateProfile(context.Background(), "org-1", "missing", "Name", nil)
	if err == nil {
		t.Error("UpdateProfile() expected error for missing user")
	}
}

func TestUserService_ChangePassword_Success(t *testing.T) {
	svc, userRepo, accountRepo, sessionRepo := newTestUserService(t)
	user, acct := addTestUserWithAccount(userRepo, accountRepo, acctID("a"), "user-1", "org-1", "Alice", "a@x.com", "old-password")
	originalHash := acct.PasswordHash

	// Seed an existing session for the user that should be revoked.
	sessionRepo.Create(context.Background(), &domain.Session{ID: "sess-1", UserID: user.ID, OrgID: "org-1"})

	if err := svc.ChangePassword(context.Background(), user.OrgID, user.ID, "old-password", "newpass123"); err != nil {
		t.Fatalf("ChangePassword() error = %v", err)
	}

	// Password hash must have changed.
	updated, _ := accountRepo.GetByID(context.Background(), acct.ID)
	if updated.PasswordHash == originalHash {
		t.Error("password hash was not updated")
	}
	// New password verifies.
	if !auth.CheckPassword("newpass123", updated.PasswordHash) {
		t.Error("new password does not verify against the updated hash")
	}
}

func TestUserService_ChangePassword_WrongCurrent(t *testing.T) {
	svc, userRepo, accountRepo, _ := newTestUserService(t)
	user, _ := addTestUserWithAccount(userRepo, accountRepo, acctID("a"), "user-1", "org-1", "Alice", "a@x.com", "correct")

	err := svc.ChangePassword(context.Background(), user.OrgID, user.ID, "wrong-current", "newpass123")
	if !errors.Is(err, apperr.ErrInvalidCreds) {
		t.Errorf("ChangePassword() error = %v, want ErrInvalidCreds", err)
	}
}

func TestUserService_ChangePassword_TooShort(t *testing.T) {
	svc, userRepo, accountRepo, _ := newTestUserService(t)
	user, _ := addTestUserWithAccount(userRepo, accountRepo, acctID("a"), "user-1", "org-1", "Alice", "a@x.com", "pw")

	err := svc.ChangePassword(context.Background(), user.OrgID, user.ID, "pw", "short")
	if !errors.Is(err, apperr.ErrInvalidInput) {
		t.Errorf("ChangePassword() error = %v, want ErrInvalidInput", err)
	}
}

// addTestUserWithAccount seeds an account + membership and returns both so
// tests can assert on either. Mirrors the existing addTestUser helper but lets
// the caller control IDs/org.
func addTestUserWithAccount(userRepo *mockUserRepo, accountRepo *mockAccountRepo, accountID, userID, orgID, name, email, password string) (*domain.User, *domain.Account) {
	hash, _ := auth.HashPassword(password)
	acct := &domain.Account{ID: accountID, Email: email, PasswordHash: hash}
	accountRepo.accountsByEmail[email] = acct
	accountRepo.accountsByID[accountID] = acct
	user := &domain.User{
		ID:        userID,
		AccountID: accountID,
		OrgID:     orgID,
		Email:     email,
		Name:      name,
		Role:      domain.RoleMember,
		IsActive:  true,
	}
	userRepo.usersByID[userID] = user
	userRepo.usersByEmail[email] = user
	return user, acct
}

func TestUserService_UpdateActive_DeactivationDeleteSessionsFailure(t *testing.T) {
	svc, userRepo, accountRepo, sessionRepo := newTestUserService(t)
	user, _ := addTestUserWithAccount(userRepo, accountRepo, acctID("da"), "user-da", "org-1", "Deact", "deact@x.com", "pw")

	sessionRepo.deleteByUserErr = errors.New("session db unavailable")

	err := svc.UpdateActive(context.Background(), user.OrgID, user.ID, false, "someone-else")
	if err == nil {
		t.Fatal("UpdateActive() expected error when session deletion fails, got nil")
	}
	if !strings.Contains(err.Error(), "session db unavailable") {
		t.Errorf("UpdateActive() error = %v, want error wrapping 'session db unavailable'", err)
	}

	// The user should still be marked inactive despite the session-revocation
	// failure (UpdateActive completes before DeleteByUser is attempted).
	fetched, err := userRepo.GetByID(context.Background(), user.OrgID, user.ID)
	if err != nil {
		t.Fatalf("GetByID() error = %v", err)
	}
	if fetched.IsActive {
		t.Error("user.IsActive should be false even when session deletion fails")
	}
}

func TestUserService_UpdateActive_ReactivationDoesNotDeleteSessions(t *testing.T) {
	svc, userRepo, accountRepo, sessionRepo := newTestUserService(t)
	user, _ := addTestUserWithAccount(userRepo, accountRepo, acctID("ra"), "user-ra", "org-1", "React", "react@x.com", "pw")

	// Start with an inactive user.
	user.IsActive = false

	// Seed an existing session so we can detect deletion.
	sessionRepo.Create(context.Background(), &domain.Session{ID: "react-sess", UserID: user.ID, OrgID: user.OrgID})

	err := svc.UpdateActive(context.Background(), user.OrgID, user.ID, true, "someone-else")
	if err != nil {
		t.Fatalf("UpdateActive() reactivation error = %v", err)
	}

	// The session should still exist (no DeleteByUser call on activation).
	_, err = sessionRepo.GetByID(context.Background(), "react-sess")
	if err != nil {
		t.Errorf("session was deleted after reactivation: %v", err)
	}

	fetched, err := userRepo.GetByID(context.Background(), user.OrgID, user.ID)
	if err != nil {
		t.Fatalf("GetByID() error = %v", err)
	}
	if !fetched.IsActive {
		t.Error("user.IsActive should be true after reactivation")
	}
}

func acctID(s string) string { return "acct-" + s }
