package service

import (
	"context"
	"errors"
	"testing"
	"time"

	"ipmanlk/breeze/internal/apperr"
	"ipmanlk/breeze/internal/domain"
)

func TestAuthService_RequestPasswordReset_UnknownEmail(t *testing.T) {
	userRepo := newMockUserRepo()
	accountRepo := newMockAccountRepo()
	sessionRepo := newMockSessionRepo()
	resetRepo := newMockPasswordResetRepo()
	svc := NewAuthService(accountRepo, userRepo, sessionRepo, resetRepo, newMockMailer(false), "", testJWTSecret, nil)

	// Unknown email must NOT error (avoids leaking account existence).
	token, err := svc.RequestPasswordReset(context.Background(), "nobody@test.com")
	if err != nil {
		t.Fatalf("RequestPasswordReset() unknown email error = %v, want nil", err)
	}
	if token != "" {
		t.Errorf("token = %q, want empty for unknown email", token)
	}
	if len(resetRepo.tokens) != 0 {
		t.Errorf("expected 0 stored resets, got %d", len(resetRepo.tokens))
	}
}

func TestAuthService_RequestPasswordReset_KnownEmail(t *testing.T) {
	userRepo := newMockUserRepo()
	accountRepo := newMockAccountRepo()
	sessionRepo := newMockSessionRepo()
	resetRepo := newMockPasswordResetRepo()
	svc := NewAuthService(accountRepo, userRepo, sessionRepo, resetRepo, newMockMailer(false), "", testJWTSecret, nil)

	addTestUser(userRepo, accountRepo, "alice@test.com", "old-password")

	token, err := svc.RequestPasswordReset(context.Background(), "alice@test.com")
	if err != nil {
		t.Fatalf("RequestPasswordReset() error = %v", err)
	}
	if token == "" {
		t.Fatal("token is empty for known email")
	}
	if len(resetRepo.tokens) != 1 {
		t.Fatalf("expected 1 stored reset, got %d", len(resetRepo.tokens))
	}
}

func TestAuthService_ConfirmPasswordReset_Success(t *testing.T) {
	userRepo := newMockUserRepo()
	accountRepo := newMockAccountRepo()
	sessionRepo := newMockSessionRepo()
	resetRepo := newMockPasswordResetRepo()
	svc := NewAuthService(accountRepo, userRepo, sessionRepo, resetRepo, newMockMailer(false), "", testJWTSecret, nil)

	addTestUser(userRepo, accountRepo, "alice@test.com", "old-password")
	token, _ := svc.RequestPasswordReset(context.Background(), "alice@test.com")

	if err := svc.ConfirmPasswordReset(context.Background(), token, "new-password-123"); err != nil {
		t.Fatalf("ConfirmPasswordReset() error = %v", err)
	}

	// The token must now be marked used.
	for _, r := range resetRepo.tokens {
		if r.UsedAt == nil {
			t.Error("reset not marked as used after confirm")
		}
	}

	// The new password must work for login.
	_, _, _, err := svc.Login(context.Background(), domain.LoginParams{Email: "alice@test.com", Password: "new-password-123"})
	if err != nil {
		t.Errorf("Login() with new password error = %v", err)
	}

	// The old password must no longer work.
	_, _, _, err = svc.Login(context.Background(), domain.LoginParams{Email: "alice@test.com", Password: "old-password"})
	if !errors.Is(err, apperr.ErrInvalidCreds) {
		t.Errorf("Login() with old password error = %v, want ErrInvalidCreds", err)
	}
}

func TestAuthService_ConfirmPasswordReset_TooShort(t *testing.T) {
	svc := NewAuthService(newMockAccountRepo(), newMockUserRepo(), newMockSessionRepo(), newMockPasswordResetRepo(), newMockMailer(false), "", testJWTSecret, nil)

	err := svc.ConfirmPasswordReset(context.Background(), "any-token", "short")
	if !errors.Is(err, apperr.ErrInvalidInput) {
		t.Errorf("ConfirmPasswordReset() short password error = %v, want ErrInvalidInput", err)
	}
}

func TestAuthService_ConfirmPasswordReset_InvalidToken(t *testing.T) {
	svc := NewAuthService(newMockAccountRepo(), newMockUserRepo(), newMockSessionRepo(), newMockPasswordResetRepo(), newMockMailer(false), "", testJWTSecret, nil)

	err := svc.ConfirmPasswordReset(context.Background(), "nonexistent-token", "new-password-123")
	if !errors.Is(err, apperr.ErrInvalidInput) {
		t.Errorf("ConfirmPasswordReset() invalid token error = %v, want ErrInvalidInput", err)
	}
}

func TestAuthService_ConfirmPasswordReset_AlreadyUsed(t *testing.T) {
	userRepo := newMockUserRepo()
	accountRepo := newMockAccountRepo()
	sessionRepo := newMockSessionRepo()
	resetRepo := newMockPasswordResetRepo()
	svc := NewAuthService(accountRepo, userRepo, sessionRepo, resetRepo, newMockMailer(false), "", testJWTSecret, nil)

	addTestUser(userRepo, accountRepo, "alice@test.com", "old-password")
	token, _ := svc.RequestPasswordReset(context.Background(), "alice@test.com")

	// First use succeeds.
	if err := svc.ConfirmPasswordReset(context.Background(), token, "new-password-123"); err != nil {
		t.Fatalf("first ConfirmPasswordReset() error = %v", err)
	}
	// Second use of the same token must fail.
	err := svc.ConfirmPasswordReset(context.Background(), token, "another-password-456")
	if !errors.Is(err, apperr.ErrInvalidInput) {
		t.Errorf("second ConfirmPasswordReset() error = %v, want ErrInvalidInput", err)
	}
}

func TestAuthService_ConfirmPasswordReset_Expired(t *testing.T) {
	userRepo := newMockUserRepo()
	accountRepo := newMockAccountRepo()
	sessionRepo := newMockSessionRepo()
	resetRepo := newMockPasswordResetRepo()
	svc := NewAuthService(accountRepo, userRepo, sessionRepo, resetRepo, newMockMailer(false), "", testJWTSecret, nil)

	addTestUser(userRepo, accountRepo, "alice@test.com", "old-password")
	token, _ := svc.RequestPasswordReset(context.Background(), "alice@test.com")

	// Force the stored reset to be expired.
	for _, r := range resetRepo.tokens {
		r.ExpiresAt = time.Now().UTC().Add(-1 * time.Hour)
	}

	err := svc.ConfirmPasswordReset(context.Background(), token, "new-password-123")
	if !errors.Is(err, apperr.ErrInvalidInput) {
		t.Errorf("ConfirmPasswordReset() expired token error = %v, want ErrInvalidInput", err)
	}
}

// TestAuthService_ConfirmPasswordReset_RevokesExistingSessions verifies that a
// successful password reset invalidates all of the account's existing sessions.
// Without this, a session cookie stolen before the reset stays valid until its
// JWT expiry, defeating the purpose of resetting a compromised password.
func TestAuthService_ConfirmPasswordReset_RevokesExistingSessions(t *testing.T) {
	userRepo := newMockUserRepo()
	accountRepo := newMockAccountRepo()
	sessionRepo := newMockSessionRepo()
	resetRepo := newMockPasswordResetRepo()
	svc := NewAuthService(accountRepo, userRepo, sessionRepo, resetRepo, newMockMailer(false), "", testJWTSecret, nil)

	u := addTestUser(userRepo, accountRepo, "alice@test.com", "old-password")

	// A pre-existing session (e.g. one stolen via XSS) for the account.
	stolen := &domain.Session{
		ID:        "sess-stolen",
		UserID:    u.ID,
		OrgID:     u.OrgID,
		Role:      u.Role,
		ExpiresAt: time.Now().UTC().Add(24 * time.Hour),
	}
	if err := sessionRepo.Create(context.Background(), stolen); err != nil {
		t.Fatalf("seed session: %v", err)
	}

	token, _ := svc.RequestPasswordReset(context.Background(), "alice@test.com")
	if err := svc.ConfirmPasswordReset(context.Background(), token, "new-password-123"); err != nil {
		t.Fatalf("ConfirmPasswordReset() error = %v", err)
	}

	// The stolen session must no longer validate.
	_, err := svc.ValidateSession(context.Background(), mustGenToken(t, svc, stolen.ID, u.ID, u.OrgID, u.Role))
	if err == nil {
		t.Error("stolen session still valid after password reset; expected revocation")
	}
}

// mustGenToken signs a JWT for the given session via the service's own JWT
// helper. We reach into the package-level auth helper because the service does
// not expose token generation publicly.
func mustGenToken(t *testing.T, svc *AuthService, sessionID, userID, orgID string, role domain.Role) string {
	t.Helper()
	tok, err := svc.jwt.GenerateToken(&domain.Session{
		ID:        sessionID,
		UserID:    userID,
		OrgID:     orgID,
		Role:      role,
		ExpiresAt: time.Now().UTC().Add(24 * time.Hour),
	})
	if err != nil {
		t.Fatalf("generate token: %v", err)
	}
	return tok
}
