package service

import (
	"context"
	"crypto/rand"
	"encoding/base64"
	"errors"
	"fmt"
	"sort"
	"testing"
	"time"

	"ipmanlk/plume/internal/apperr"
	"ipmanlk/plume/internal/auth"
	"ipmanlk/plume/internal/domain"

	"golang.org/x/crypto/argon2"
)

type mockSessionRepo struct {
	sessionsByID    map[string]*domain.Session
	deleteByUserErr error
}

func newMockSessionRepo() *mockSessionRepo {
	return &mockSessionRepo{
		sessionsByID: make(map[string]*domain.Session),
	}
}

func (m *mockSessionRepo) Create(ctx context.Context, session *domain.Session) error {
	s := *session
	s.CreatedAt = time.Now().UTC()
	m.sessionsByID[s.ID] = &s
	return nil
}

func (m *mockSessionRepo) GetByID(ctx context.Context, id string) (*domain.Session, error) {
	s, ok := m.sessionsByID[id]
	if !ok {
		return nil, errors.New("not found")
	}
	return s, nil
}

func (m *mockSessionRepo) Revoke(ctx context.Context, id string) error {
	s, ok := m.sessionsByID[id]
	if !ok {
		return errors.New("not found")
	}
	now := time.Now()
	s.RevokedAt = &now
	return nil
}

func (m *mockSessionRepo) Delete(ctx context.Context, id string) error {
	delete(m.sessionsByID, id)
	return nil
}

func (m *mockSessionRepo) DeleteByUser(ctx context.Context, userID string) error {
	if m.deleteByUserErr != nil {
		return m.deleteByUserErr
	}
	for id, s := range m.sessionsByID {
		if s.UserID == userID {
			delete(m.sessionsByID, id)
		}
	}
	return nil
}

func (m *mockSessionRepo) ListByUser(ctx context.Context, userID string) ([]*domain.Session, error) {
	var out []*domain.Session
	now := time.Now()
	for _, s := range m.sessionsByID {
		if s.UserID == userID && s.ExpiresAt.After(now) {
			cp := *s
			out = append(out, &cp)
		}
	}
	sort.Slice(out, func(i, j int) bool {
		return out[i].CreatedAt.After(out[j].CreatedAt)
	})
	return out, nil
}

func (m *mockSessionRepo) DeleteExpired(ctx context.Context) error {
	for id, s := range m.sessionsByID {
		if time.Now().After(s.ExpiresAt) {
			delete(m.sessionsByID, id)
		}
	}
	return nil
}

func addTestUser(repo *mockUserRepo, accountRepo *mockAccountRepo, email, password string) *domain.User {
	hash, _ := auth.HashPassword(password)
	accountID := "acct-" + email
	accountRepo.accountsByEmail[email] = &domain.Account{ID: accountID, Email: email, PasswordHash: hash}
	accountRepo.accountsByID[accountID] = accountRepo.accountsByEmail[email]
	u := &domain.User{
		ID:        "user-" + email,
		AccountID: accountID,
		OrgID:     "org-1",
		Email:     email,
		Name:      "Test User",
		Role:      domain.RoleMember,
		IsActive:  true,
	}
	repo.usersByID[u.ID] = u
	repo.usersByEmail[email] = u
	return u
}

const testJWTSecret = "test-secret-key-for-jwt-signing-32bytes"

func TestAuthService_Login_Success(t *testing.T) {
	userRepo := newMockUserRepo()
	accountRepo := newMockAccountRepo()
	sessionRepo := newMockSessionRepo()
	svc := NewAuthService(accountRepo, userRepo, sessionRepo, newMockPasswordResetRepo(), newMockMailer(false), "", testJWTSecret, nil)

	addTestUser(userRepo, accountRepo, "alice@test.com", "secure-password")

	account, memberships, token, err := svc.Login(context.Background(), domain.LoginParams{Email: "alice@test.com", Password: "secure-password"})
	if err != nil {
		t.Fatalf("Login() error = %v", err)
	}
	if account == nil {
		t.Fatal("Login() account is nil")
	}
	if account.Email != "alice@test.com" {
		t.Errorf("account.Email = %q, want %q", account.Email, "alice@test.com")
	}
	if len(memberships) != 1 {
		t.Errorf("memberships count = %d, want 1", len(memberships))
	}
	if token == "" {
		t.Error("Login() token is empty")
	}
}

func TestAuthService_Login_WrongPassword(t *testing.T) {
	userRepo := newMockUserRepo()
	accountRepo := newMockAccountRepo()
	sessionRepo := newMockSessionRepo()
	svc := NewAuthService(accountRepo, userRepo, sessionRepo, newMockPasswordResetRepo(), newMockMailer(false), "", testJWTSecret, nil)

	addTestUser(userRepo, accountRepo, "bob@test.com", "correct-password")

	_, _, _, err := svc.Login(context.Background(), domain.LoginParams{Email: "bob@test.com", Password: "wrong-password"})
	if !errors.Is(err, apperr.ErrInvalidCreds) {
		t.Errorf("Login() error = %v, want ErrInvalidCreds", err)
	}
}

func TestAuthService_Login_UserNotFound(t *testing.T) {
	userRepo := newMockUserRepo()
	accountRepo := newMockAccountRepo()
	sessionRepo := newMockSessionRepo()
	svc := NewAuthService(accountRepo, userRepo, sessionRepo, newMockPasswordResetRepo(), newMockMailer(false), "", testJWTSecret, nil)

	_, _, _, err := svc.Login(context.Background(), domain.LoginParams{Email: "nonexistent@test.com", Password: "password"})
	if !errors.Is(err, apperr.ErrInvalidCreds) {
		t.Errorf("Login() error = %v, want ErrInvalidCreds", err)
	}
}

func TestAuthService_ValidateSession_Success(t *testing.T) {
	userRepo := newMockUserRepo()
	accountRepo := newMockAccountRepo()
	sessionRepo := newMockSessionRepo()
	svc := NewAuthService(accountRepo, userRepo, sessionRepo, newMockPasswordResetRepo(), newMockMailer(false), "", testJWTSecret, nil)

	addTestUser(userRepo, accountRepo, "charlie@test.com", "password")
	_, _, token, err := svc.Login(context.Background(), domain.LoginParams{Email: "charlie@test.com", Password: "password"})
	if err != nil {
		t.Fatalf("Login() error = %v", err)
	}

	session, err := svc.ValidateSession(context.Background(), token)
	if err != nil {
		t.Fatalf("ValidateSession() error = %v", err)
	}
	if session == nil {
		t.Fatal("ValidateSession() session is nil")
	}
	if session.UserID != "user-charlie@test.com" {
		t.Errorf("session.UserID = %q, want %q", session.UserID, "user-charlie@test.com")
	}
}

func TestAuthService_ValidateSession_Expired(t *testing.T) {
	userRepo := newMockUserRepo()
	accountRepo := newMockAccountRepo()
	sessionRepo := newMockSessionRepo()
	svc := NewAuthService(accountRepo, userRepo, sessionRepo, newMockPasswordResetRepo(), newMockMailer(false), "", testJWTSecret, nil)

	jwtSvc := auth.NewJWTService(testJWTSecret)
	session := &domain.Session{
		ID:        "expired-session",
		UserID:    "user-1",
		OrgID:     "org-1",
		Role:      domain.RoleMember,
		ExpiresAt: time.Date(2020, 1, 1, 0, 0, 0, 0, time.UTC),
	}
	sessionRepo.Create(context.Background(), session)

	token, err := jwtSvc.GenerateToken(session)
	if err != nil {
		t.Fatalf("GenerateToken() error = %v", err)
	}

	_, err = svc.ValidateSession(context.Background(), token)
	if !errors.Is(err, apperr.ErrSessionNotFound) {
		t.Errorf("ValidateSession() error = %v, want ErrSessionNotFound (JWT expired)", err)
	}
}

func TestAuthService_ValidateSession_DeactivatedUser(t *testing.T) {
	userRepo := newMockUserRepo()
	accountRepo := newMockAccountRepo()
	sessionRepo := newMockSessionRepo()
	svc := NewAuthService(accountRepo, userRepo, sessionRepo, newMockPasswordResetRepo(), newMockMailer(false), "", testJWTSecret, nil)

	addTestUser(userRepo, accountRepo, "deact@test.com", "password")
	_, _, token, err := svc.Login(context.Background(), domain.LoginParams{Email: "deact@test.com", Password: "password"})
	if err != nil {
		t.Fatalf("Login() error = %v", err)
	}

	// 1. Active user → session validates.
	session, err := svc.ValidateSession(context.Background(), token)
	if err != nil {
		t.Fatalf("ValidateSession() with active user error = %v", err)
	}
	if session == nil {
		t.Fatal("ValidateSession() session is nil for active user")
	}

	// 2. Deactivate the user → ValidateSession must reject.
	userRepo.usersByID["user-deact@test.com"].IsActive = false
	_, err = svc.ValidateSession(context.Background(), token)
	if !errors.Is(err, apperr.ErrUserDeactivated) {
		t.Errorf("ValidateSession() after deactivation error = %v, want ErrUserDeactivated", err)
	}

	// 3. Deleted user (removed from repo) → ValidateSession must reject.
	delete(userRepo.usersByID, "user-deact@test.com")
	_, err = svc.ValidateSession(context.Background(), token)
	if !errors.Is(err, apperr.ErrUserDeactivated) {
		t.Errorf("ValidateSession() after user deletion error = %v, want ErrUserDeactivated", err)
	}
}

func TestAuthService_ValidateSession_NotFound(t *testing.T) {
	userRepo := newMockUserRepo()
	accountRepo := newMockAccountRepo()
	sessionRepo := newMockSessionRepo()
	svc := NewAuthService(accountRepo, userRepo, sessionRepo, newMockPasswordResetRepo(), newMockMailer(false), "", testJWTSecret, nil)

	_, err := svc.ValidateSession(context.Background(), "nonexistent-token")
	if !errors.Is(err, apperr.ErrSessionNotFound) {
		t.Errorf("ValidateSession() error = %v, want ErrSessionNotFound", err)
	}
}

func TestAuthService_Logout(t *testing.T) {
	userRepo := newMockUserRepo()
	accountRepo := newMockAccountRepo()
	sessionRepo := newMockSessionRepo()
	svc := NewAuthService(accountRepo, userRepo, sessionRepo, newMockPasswordResetRepo(), newMockMailer(false), "", testJWTSecret, nil)

	addTestUser(userRepo, accountRepo, "dave@test.com", "password")
	_, _, token, err := svc.Login(context.Background(), domain.LoginParams{Email: "dave@test.com", Password: "password"})
	if err != nil {
		t.Fatalf("Login() error = %v", err)
	}

	session, err := svc.ValidateSession(context.Background(), token)
	if err != nil {
		t.Fatalf("ValidateSession() error = %v", err)
	}

	if err := svc.Logout(context.Background(), session.ID); err != nil {
		t.Fatalf("Logout() error = %v", err)
	}

	_, err = svc.ValidateSession(context.Background(), token)
	if !errors.Is(err, apperr.ErrSessionExpired) {
		t.Errorf("after logout, ValidateSession() error = %v, want ErrSessionExpired", err)
	}
}

// ListSessions returns only the caller's sessions, newest first, excluding
// expired rows.
func TestAuthService_ListSessions_ScopedToCaller(t *testing.T) {
	userRepo := newMockUserRepo()
	accountRepo := newMockAccountRepo()
	sessionRepo := newMockSessionRepo()
	svc := NewAuthService(accountRepo, userRepo, sessionRepo, newMockPasswordResetRepo(), newMockMailer(false), "", testJWTSecret, nil)

	addTestUser(userRepo, accountRepo, "alice@test.com", "pw")
	addTestUser(userRepo, accountRepo, "bob@test.com", "pw")

	_, aMemberships, _, err := svc.Login(context.Background(), domain.LoginParams{Email: "alice@test.com", Password: "pw"})
	if err != nil {
		t.Fatalf("alice login: %v", err)
	}
	_, _, _, err = svc.Login(context.Background(), domain.LoginParams{Email: "alice@test.com", Password: "pw"})
	if err != nil {
		t.Fatalf("alice second login: %v", err)
	}
	_, bMemberships, _, err := svc.Login(context.Background(), domain.LoginParams{Email: "bob@test.com", Password: "pw"})
	if err != nil {
		t.Fatalf("bob login: %v", err)
	}

	aID := aMemberships[0].ID
	bID := bMemberships[0].ID

	aSessions, err := svc.ListSessions(context.Background(), aID)
	if err != nil {
		t.Fatalf("ListSessions alice: %v", err)
	}
	if len(aSessions) != 2 {
		t.Errorf("alice sessions = %d, want 2", len(aSessions))
	}
	// Newest first.
	if len(aSessions) == 2 && !aSessions[0].CreatedAt.After(aSessions[1].CreatedAt) && !aSessions[0].CreatedAt.Equal(aSessions[1].CreatedAt) {
		t.Errorf("sessions not ordered newest-first")
	}
	for _, s := range aSessions {
		if s.UserID != aID {
			t.Errorf("leak: alice list contains session for %q", s.UserID)
		}
	}

	bSessions, _ := svc.ListSessions(context.Background(), bID)
	if len(bSessions) != 1 {
		t.Errorf("bob sessions = %d, want 1", len(bSessions))
	}
}

// RevokeSession refuses to revoke a session owned by another user (no
// cross-user revocation via ID alone) and returns ErrNotFound.
func TestAuthService_RevokeSession_ForeignSessionRejected(t *testing.T) {
	userRepo := newMockUserRepo()
	accountRepo := newMockAccountRepo()
	sessionRepo := newMockSessionRepo()
	svc := NewAuthService(accountRepo, userRepo, sessionRepo, newMockPasswordResetRepo(), newMockMailer(false), "", testJWTSecret, nil)

	addTestUser(userRepo, accountRepo, "alice@test.com", "pw")
	addTestUser(userRepo, accountRepo, "bob@test.com", "pw")

	_, aMemberships, aToken, err := svc.Login(context.Background(), domain.LoginParams{Email: "alice@test.com", Password: "pw"})
	if err != nil {
		t.Fatalf("alice login: %v", err)
	}
	_, bMemberships, _, err := svc.Login(context.Background(), domain.LoginParams{Email: "bob@test.com", Password: "pw"})
	if err != nil {
		t.Fatalf("bob login: %v", err)
	}

	aSession, err := svc.ValidateSession(context.Background(), aToken)
	if err != nil {
		t.Fatalf("validate alice: %v", err)
	}

	// Bob tries to revoke Alice's session.
	err = svc.RevokeSession(context.Background(), bMemberships[0].ID, aSession.ID)
	if !errors.Is(err, apperr.ErrNotFound) {
		t.Errorf("RevokeSession foreign = %v, want ErrNotFound", err)
	}
	// Alice's session must still be valid.
	if _, err := svc.ValidateSession(context.Background(), aToken); err != nil {
		t.Errorf("alice session invalidated by foreign user: %v", err)
	}

	// Alice revoking her own session succeeds.
	if err := svc.RevokeSession(context.Background(), aMemberships[0].ID, aSession.ID); err != nil {
		t.Errorf("RevokeSession own = %v, want nil", err)
	}
	if _, err := svc.ValidateSession(context.Background(), aToken); !errors.Is(err, apperr.ErrSessionExpired) {
		t.Errorf("after own revoke, validate = %v, want ErrSessionExpired", err)
	}
}

// hashWithCustomParams creates a valid argon2id hash using the given custom
// parameters. Used to simulate a password hash created with weaker settings.
func hashWithCustomParams(password string, memory, time, keyLen uint32, threads uint8) string {
	salt := make([]byte, 32)
	if _, err := rand.Read(salt); err != nil {
		panic(fmt.Errorf("rand.Read: %w", err))
	}
	hash := argon2.IDKey([]byte(password), salt, time, memory, threads, keyLen)
	b64Salt := base64.RawStdEncoding.EncodeToString(salt)
	b64Hash := base64.RawStdEncoding.EncodeToString(hash)
	return fmt.Sprintf("$argon2id$v=19$m=%d,t=%d,p=%d$%s$%s", memory, time, threads, b64Salt, b64Hash)
}

func TestLogin_RehashOnStaleParams(t *testing.T) {
	userRepo := newMockUserRepo()
	accountRepo := newMockAccountRepo()
	sessionRepo := newMockSessionRepo()
	svc := NewAuthService(accountRepo, userRepo, sessionRepo, newMockPasswordResetRepo(), newMockMailer(false), "", testJWTSecret, nil)

	const (
		email    = "alice@test.com"
		password = "hunter2"
	)

	// Create an account with a hash using weaker Argon2 params.
	weakHash := hashWithCustomParams(password, 32768, 2, 32, 1)
	acctID := "acct-" + email
	accountRepo.accountsByEmail[email] = &domain.Account{ID: acctID, Email: email, PasswordHash: weakHash}
	accountRepo.accountsByID[acctID] = accountRepo.accountsByEmail[email]
	userRepo.usersByID["user-"+email] = &domain.User{
		ID:        "user-" + email,
		AccountID: acctID,
		OrgID:     "org-1",
		Email:     email,
		Name:      "Alice",
		Role:      domain.RoleMember,
		IsActive:  true,
	}
	userRepo.usersByEmail[email] = userRepo.usersByID["user-"+email]

	// Login should succeed and upgrade the hash.
	_, _, token, err := svc.Login(context.Background(), domain.LoginParams{Email: email, Password: password})
	if err != nil {
		t.Fatalf("Login() error = %v", err)
	}

	// The account's stored hash should now be the current params.
	stored := accountRepo.accountsByID[acctID].PasswordHash
	if auth.NeedsRehash(stored) {
		t.Error("stored hash still needs rehash after upgrade")
	}

	// login still works with token.
	if _, err := svc.ValidateSession(context.Background(), token); err != nil {
		t.Errorf("ValidateSession after rehash = %v, want nil", err)
	}

	// A second login also succeeds (rehash idempotent).
	if _, _, _, err := svc.Login(context.Background(), domain.LoginParams{Email: email, Password: password}); err != nil {
		t.Errorf("second login after rehash = %v, want nil", err)
	}
}

func TestLogin_RehashFailureIsBestEffort(t *testing.T) {
	userRepo := newMockUserRepo()
	accountRepo := newMockAccountRepo()
	accountRepo.updatePasswordErr = errors.New("db unavailable")
	sessionRepo := newMockSessionRepo()
	svc := NewAuthService(accountRepo, userRepo, sessionRepo, newMockPasswordResetRepo(), newMockMailer(false), "", testJWTSecret, nil)

	const (
		email    = "bob@test.com"
		password = "s3cret!"
	)

	weakHash := hashWithCustomParams(password, 32768, 2, 32, 1)
	acctID := "acct-" + email
	accountRepo.accountsByEmail[email] = &domain.Account{ID: acctID, Email: email, PasswordHash: weakHash}
	accountRepo.accountsByID[acctID] = accountRepo.accountsByEmail[email]
	userRepo.usersByID["user-"+email] = &domain.User{
		ID:        "user-" + email,
		AccountID: acctID,
		OrgID:     "org-1",
		Email:     email,
		Name:      "Bob",
		Role:      domain.RoleMember,
		IsActive:  true,
	}
	userRepo.usersByEmail[email] = userRepo.usersByID["user-"+email]

	// Login must succeed despite the rehash failure (best-effort).
	_, _, token, err := svc.Login(context.Background(), domain.LoginParams{Email: email, Password: password})
	if err != nil {
		t.Fatalf("Login() error = %v, want nil (best-effort rehash)", err)
	}

	// The stored hash should still be the weak one (rehash failed).
	stored := accountRepo.accountsByID[acctID].PasswordHash
	if !auth.NeedsRehash(stored) {
		t.Error("stored hash was upgraded despite UpdatePassword error")
	}

	// Session should still be valid.
	if _, err := svc.ValidateSession(context.Background(), token); err != nil {
		t.Errorf("ValidateSession after failed rehash = %v, want nil", err)
	}
}

func TestAuthService_ValidateResetToken_Success(t *testing.T) {
	userRepo := newMockUserRepo()
	accountRepo := newMockAccountRepo()
	resetRepo := newMockPasswordResetRepo()
	sessionRepo := newMockSessionRepo()
	svc := NewAuthService(accountRepo, userRepo, sessionRepo, resetRepo, newMockMailer(false), "", testJWTSecret, nil)

	tokenSvc := auth.NewTokenService()
	token, tokenHash, err := tokenSvc.GenerateRandomToken()
	if err != nil {
		t.Fatalf("GenerateRandomToken() error = %v", err)
	}
	err = resetRepo.Create(context.Background(), &domain.PasswordReset{
		ID:        "reset-1",
		AccountID: "acct-user",
		TokenHash: tokenHash,
		ExpiresAt: time.Now().UTC().Add(1 * time.Hour),
		CreatedAt: time.Now().UTC(),
	})
	if err != nil {
		t.Fatalf("resetRepo.Create() error = %v", err)
	}

	if err := svc.ValidateResetToken(context.Background(), token); err != nil {
		t.Errorf("ValidateResetToken() for valid token = %v, want nil", err)
	}
}

func TestAuthService_ValidateResetToken_Expired(t *testing.T) {
	userRepo := newMockUserRepo()
	accountRepo := newMockAccountRepo()
	resetRepo := newMockPasswordResetRepo()
	sessionRepo := newMockSessionRepo()
	svc := NewAuthService(accountRepo, userRepo, sessionRepo, resetRepo, newMockMailer(false), "", testJWTSecret, nil)

	tokenSvc := auth.NewTokenService()
	token, tokenHash, err := tokenSvc.GenerateRandomToken()
	if err != nil {
		t.Fatalf("GenerateRandomToken() error = %v", err)
	}
	err = resetRepo.Create(context.Background(), &domain.PasswordReset{
		ID:        "reset-2",
		AccountID: "acct-user",
		TokenHash: tokenHash,
		ExpiresAt: time.Now().UTC().Add(-1 * time.Hour),
		CreatedAt: time.Now().UTC(),
	})
	if err != nil {
		t.Fatalf("resetRepo.Create() error = %v", err)
	}

	if err := svc.ValidateResetToken(context.Background(), token); err == nil {
		t.Error("ValidateResetToken() for expired token = nil, want error")
	}
}

func TestAuthService_ValidateResetToken_Used(t *testing.T) {
	userRepo := newMockUserRepo()
	accountRepo := newMockAccountRepo()
	resetRepo := newMockPasswordResetRepo()
	sessionRepo := newMockSessionRepo()
	svc := NewAuthService(accountRepo, userRepo, sessionRepo, resetRepo, newMockMailer(false), "", testJWTSecret, nil)

	tokenSvc := auth.NewTokenService()
	token, tokenHash, err := tokenSvc.GenerateRandomToken()
	if err != nil {
		t.Fatalf("GenerateRandomToken() error = %v", err)
	}
	now := time.Now().UTC()
	err = resetRepo.Create(context.Background(), &domain.PasswordReset{
		ID:        "reset-3",
		AccountID: "acct-user",
		TokenHash: tokenHash,
		ExpiresAt: now.Add(1 * time.Hour),
		UsedAt:    &now,
		CreatedAt: now,
	})
	if err != nil {
		t.Fatalf("resetRepo.Create() error = %v", err)
	}

	if err := svc.ValidateResetToken(context.Background(), token); err == nil {
		t.Error("ValidateResetToken() for used token = nil, want error")
	}
}

func TestAuthService_ValidateResetToken_Unknown(t *testing.T) {
	userRepo := newMockUserRepo()
	accountRepo := newMockAccountRepo()
	resetRepo := newMockPasswordResetRepo()
	sessionRepo := newMockSessionRepo()
	svc := NewAuthService(accountRepo, userRepo, sessionRepo, resetRepo, newMockMailer(false), "", testJWTSecret, nil)

	if err := svc.ValidateResetToken(context.Background(), "unknown-token"); err == nil {
		t.Error("ValidateResetToken() for unknown token = nil, want error")
	}
}
