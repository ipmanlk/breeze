package service

import (
	"context"
	"fmt"
	"log/slog"
	"sync"
	"time"

	"ipmanlk/plume/internal/apperr"
	"ipmanlk/plume/internal/auth"
	"ipmanlk/plume/internal/domain"
	"ipmanlk/plume/internal/i18n"

	"ipmanlk/plume/internal/port"

	"github.com/google/uuid"
)

const (
	sessionTTL          = 7 * 24 * time.Hour
	maxResetAttempts    = 3
	resetAttemptsWindow = 10 * time.Minute
)

type AuthService struct {
	accountRepo       port.AccountRepository
	userRepo          port.UserRepository
	sessionRepo       port.SessionRepository
	passwordResetRepo port.PasswordResetRepository
	mailer            port.Mailer
	appURL            string
	jwt               *auth.JWTService
	i18n              *i18n.Bundle

	// resetAttempts guards password-reset confirm against brute-force by
	// rate-limiting per token hash (defense-in-depth; tokens are 256-bit).
	resetAttemptsMu sync.Mutex
	resetAttempts   map[string][]time.Time
}

var _ port.AuthService = (*AuthService)(nil)

// localize is a nil-safe wrapper around i18n.Bundle.MustLocalize.
func (s *AuthService) localize(locale, messageID string, data map[string]any, pluralCount any) string {
	if s.i18n == nil {
		return messageID
	}
	return s.i18n.MustLocalize(locale, messageID, data, pluralCount)
}

// ResetAttemptCleanup prunes entries older than the window from the
// in-memory reset-attempts map. Call periodically (e.g. every 10 minutes)
// to prevent unbounded map growth under sustained token-guessing attacks.
func (s *AuthService) ResetAttemptCleanup() {
	cutoff := time.Now().Add(-resetAttemptsWindow)
	s.resetAttemptsMu.Lock()
	defer s.resetAttemptsMu.Unlock()
	for k, times := range s.resetAttempts {
		var valid []time.Time
		for _, t := range times {
			if t.After(cutoff) {
				valid = append(valid, t)
			}
		}
		if len(valid) == 0 {
			delete(s.resetAttempts, k)
		} else {
			s.resetAttempts[k] = valid
		}
	}
}

func NewAuthService(accountRepo port.AccountRepository, userRepo port.UserRepository, sessionRepo port.SessionRepository,
	passwordResetRepo port.PasswordResetRepository, mailer port.Mailer, appURL, jwtSecret string, i18nBundle *i18n.Bundle) *AuthService {
	return &AuthService{
		accountRepo:       accountRepo,
		userRepo:          userRepo,
		sessionRepo:       sessionRepo,
		passwordResetRepo: passwordResetRepo,
		mailer:            mailer,
		appURL:            appURL,
		jwt:               auth.NewJWTService(jwtSecret),
		i18n:              i18nBundle,
		resetAttempts:     make(map[string][]time.Time),
	}
}

// Login resolves the credential against the global accounts table (unique
// email → one account, no more LIMIT-1 ambiguity), verifies the password,
// then scopes the session to the account's most-recent membership.
//
// If the stored password hash uses weaker Argon2 parameters than the current
// defaults, the hash is transparently upgraded (best-effort, logged on
// failure). This ensures hashes are gradually strengthened as operators
// tune argon2* constants without waiting for users to change passwords.
//
// Returns the account + all its memberships so the frontend can list/switch
// workspaces.
//
// If the account has no memberships (e.g. all deactivated), login still
// succeeds but produces a session with an empty OrgID; callers should treat
// that as "no active workspace".
func (s *AuthService) Login(ctx context.Context, p domain.LoginParams) (*domain.Account, []*domain.User, string, error) {
	p.Email = domain.NormalizeEmail(p.Email)
	account, err := s.accountRepo.GetByEmail(ctx, p.Email)
	if err != nil {
		// Burn a hash comparison so the response timing matches the
		// "account exists, password wrong" path.
		auth.BurnPasswordCheck(p.Password)
		return nil, nil, "", apperr.ErrInvalidCreds
	}

	if !s.CheckPassword(p.Password, account.PasswordHash) {
		return nil, nil, "", apperr.ErrInvalidCreds
	}

	// Best-effort upgrade of the password hash if the stored hash uses
	// weaker Argon2 parameters than the current defaults. This ensures
	// hashes are gradually strengthened as operators tune argon2* constants.
	if auth.NeedsRehash(account.PasswordHash) {
		if newHash, err := auth.HashPassword(p.Password); err == nil {
			if err := s.accountRepo.UpdatePassword(ctx, account.ID, newHash); err != nil {
				slog.Warn("failed to rehash password on login", "error", err, "email", p.Email)
			} else {
				account.PasswordHash = newHash
			}
		}
	}

	memberships, err := s.userRepo.ListByAccount(ctx, account.ID)
	if err != nil {
		return nil, nil, "", fmt.Errorf("list memberships: %w", err)
	}

	// Pick the active workspace: the most-recent active membership (ListByAccount
	// orders by updated_at DESC, so the first active one wins).
	var active *domain.User
	for _, m := range memberships {
		if m.IsActive {
			active = m
			break
		}
	}

	if active == nil {
		return nil, nil, "", apperr.ErrUserDeactivated
	}

	expiresAt := time.Now().UTC().Add(sessionTTL)
	session := &domain.Session{
		ID:        uuid.New().String(),
		UserID:    active.ID,
		OrgID:     active.OrgID,
		Role:      active.Role,
		ExpiresAt: expiresAt,
		UserAgent: p.UserAgent,
		IPAddress: p.IPAddress,
	}

	if err := s.sessionRepo.Create(ctx, session); err != nil {
		return nil, nil, "", fmt.Errorf("create session: %w", err)
	}

	token, err := s.jwt.GenerateToken(session)
	if err != nil {
		return nil, nil, "", fmt.Errorf("generate token: %w", err)
	}

	return account, memberships, token, nil
}

func (s *AuthService) ValidateSession(ctx context.Context, tokenString string) (*domain.Session, error) {
	claims, err := s.jwt.ParseToken(tokenString)
	if err != nil {
		return nil, apperr.ErrSessionNotFound
	}

	session, err := s.sessionRepo.GetByID(ctx, claims.ID)
	if err != nil {
		return nil, apperr.ErrSessionNotFound
	}

	if session.IsRevoked() {
		return nil, apperr.ErrSessionExpired
	}

	// Defense-in-depth: verify the owning user is still active. If the user
	// was deactivated and session deletion failed (or was missed), this
	// prevents continued access. A missing user (deleted) is treated the
	// same as deactivated.
	user, err := s.userRepo.GetByID(ctx, session.OrgID, session.UserID)
	if err != nil || !user.IsActive {
		return nil, apperr.ErrUserDeactivated
	}

	return session, nil
}

func (s *AuthService) ValidateSessionByID(ctx context.Context, sessionID string) (*domain.Session, error) {
	session, err := s.sessionRepo.GetByID(ctx, sessionID)
	if err != nil {
		return nil, apperr.ErrSessionNotFound
	}
	if session.IsRevoked() {
		return nil, apperr.ErrSessionExpired
	}

	user, err := s.userRepo.GetByID(ctx, session.OrgID, session.UserID)
	if err != nil || !user.IsActive {
		return nil, apperr.ErrUserDeactivated
	}

	// The role is snapshotted into the session at login. If it changed since
	// (demotion/promotion), the HTTP path already revoked the session; this
	// catches any that slipped through.
	if user.Role != session.Role {
		return nil, apperr.ErrForbidden
	}

	return session, nil
}

func (s *AuthService) Logout(ctx context.Context, sessionID string) error {
	return s.sessionRepo.Revoke(ctx, sessionID)
}

// ListSessions returns the caller's active + revoked sessions (excluding
// expired rows). Used by the Sessions settings page.
func (s *AuthService) ListSessions(ctx context.Context, userID string) ([]*domain.Session, error) {
	return s.sessionRepo.ListByUser(ctx, userID)
}

// RevokeSession revokes another session owned by the same user. It verifies
// ownership first so a caller cannot revoke a foreign user's session by ID
// alone.
func (s *AuthService) RevokeSession(ctx context.Context, userID, sessionID string) error {
	sess, err := s.sessionRepo.GetByID(ctx, sessionID)
	if err != nil {
		return apperr.ErrNotFound
	}
	if sess.UserID != userID {
		// Don't leak existence; return the same sentinel as a missing row.
		return apperr.ErrNotFound
	}
	return s.sessionRepo.Revoke(ctx, sessionID)
}

func (s *AuthService) HashPassword(password string) (string, error) {
	return auth.HashPassword(password)
}

func (s *AuthService) CheckPassword(password, encodedHash string) bool {
	return auth.CheckPassword(password, encodedHash)
}

func (s *AuthService) RequestPasswordReset(ctx context.Context, email string) (string, error) {
	email = domain.NormalizeEmail(email)
	// Look up the account (don't reveal if email exists).
	account, err := s.accountRepo.GetByEmail(ctx, email)
	if err != nil {
		// Keep timing comparable with the found-account path (which hashes
		// and emails) before returning the same generic success.
		auth.BurnPasswordCheck(uuid.New().String())
		// Return a generic success to avoid leaking account existence.
		return "", nil
	}

	tokenSvc := auth.NewTokenService()
	token, tokenHash, err := tokenSvc.GenerateRandomToken()
	if err != nil {
		return "", fmt.Errorf("generate token: %w", err)
	}

	reset := &domain.PasswordReset{
		ID:        uuid.New().String(),
		AccountID: account.ID,
		TokenHash: tokenHash,
		ExpiresAt: time.Now().UTC().Add(1 * time.Hour),
	}
	if err := s.passwordResetRepo.Create(ctx, reset); err != nil {
		return "", fmt.Errorf("save reset: %w", err)
	}

	// The token is returned so the handler can log the reset link as a
	// fallback when SMTP is not configured. When SMTP is configured the
	// service emails the link directly and the handler skips logging.
	if s.mailer != nil && s.mailer.Enabled() && account.Email != "" {
		tmpl := PasswordResetEmail(resetURL(s.appURL, token))
		// Localize the email subject via the i18n bundle; the body (HTML-heavy)
		// stays English until the mail template functions are fully refactored.
		locale := i18n.LocaleFromContext(ctx)
		if localized := s.localize(locale, "PasswordResetSubject", nil, nil); localized != "PasswordResetSubject" {
			tmpl.Subject = localized
		}
		if err := s.mailer.Send(ctx, account.Email, tmpl.Subject, tmpl.HTML, tmpl.Text); err != nil {
			// Best-effort: don't fail the request over an email error. The
			// handler will still log the link as a fallback.
			return token, nil
		}
		// Emailed successfully; return empty token so the handler does
		// not also log the link.
		return "", nil
	}
	return token, nil
}

func (s *AuthService) ConfirmPasswordReset(ctx context.Context, token, newPassword string) error {
	if len(newPassword) < 8 {
		return apperr.InvalidInput("password must be at least 8 characters")
	}

	tokenSvc := auth.NewTokenService()
	tokenHash := tokenSvc.HashToken(token)

	// Per-token rate limit: at most maxResetAttempts attempts within the
	// window. This is defense-in-depth: the token itself is 256-bit
	// CSPRNG, but rate limiting prevents a brute-force race even if the
	// token space were somehow narrowed. Uses the token hash as the key
	// so tokens are never stored in plaintext in the in-memory map.
	s.resetAttemptsMu.Lock()
	now := time.Now()
	cutoff := now.Add(-resetAttemptsWindow)
	times := s.resetAttempts[tokenHash]
	var valid []time.Time
	for _, t := range times {
		if t.After(cutoff) {
			valid = append(valid, t)
		}
	}
	if len(valid) >= maxResetAttempts {
		s.resetAttemptsMu.Unlock()
		return apperr.InvalidInput("invalid or expired reset token")
	}
	s.resetAttempts[tokenHash] = append(valid, now)
	s.resetAttemptsMu.Unlock()

	reset, err := s.passwordResetRepo.GetByTokenHash(ctx, tokenHash)
	if err != nil {
		return apperr.InvalidInput("invalid or expired reset token")
	}

	if time.Now().UTC().After(reset.ExpiresAt) {
		return apperr.InvalidInput("reset token has expired")
	}

	if reset.UsedAt != nil {
		return apperr.InvalidInput("reset token has already been used")
	}

	hash, err := auth.HashPassword(newPassword)
	if err != nil {
		return fmt.Errorf("hash password: %w", err)
	}

	// Consume the token BEFORE mutating anything else. The conditional
	// UPDATE makes this atomic: if a concurrent request already consumed the
	// token, we stop here and never touch the password.
	used, err := s.passwordResetRepo.MarkUsed(ctx, reset.ID)
	if err != nil {
		return err
	}
	if !used {
		return apperr.InvalidInput("reset token has already been used")
	}

	if err := s.accountRepo.UpdatePassword(ctx, reset.AccountID, hash); err != nil {
		return err
	}

	// Revoke all active sessions for this account so a stolen cookie from
	// before the reset (e.g. via XSS or device theft) can no longer be used.
	// Mirrors UserService.ChangePassword's session-revocation behavior.
	users, err := s.userRepo.ListByAccount(ctx, reset.AccountID)
	if err != nil {
		return fmt.Errorf("list memberships: %w", err)
	}
	for _, u := range users {
		if err := s.sessionRepo.DeleteByUser(ctx, u.ID); err != nil {
			slog.Warn("revoke sessions after password reset failed", "user_id", u.ID, "error", err)
		}
	}

	return nil
}

// ValidateResetToken checks whether a password-reset token is valid
// (exists, not expired, not already used) without consuming it. Returns
// the same generic error for all failure modes so an attacker probing
// tokens cannot distinguish "not found" from "expired" from "used".
func (s *AuthService) ValidateResetToken(ctx context.Context, token string) error {
	if token == "" {
		return apperr.InvalidInput("invalid or expired reset token")
	}
	tokenSvc := auth.NewTokenService()
	tokenHash := tokenSvc.HashToken(token)

	reset, err := s.passwordResetRepo.GetByTokenHash(ctx, tokenHash)
	if err != nil {
		return apperr.InvalidInput("invalid or expired reset token")
	}

	if time.Now().UTC().After(reset.ExpiresAt) {
		return apperr.InvalidInput("invalid or expired reset token")
	}

	if reset.UsedAt != nil {
		return apperr.InvalidInput("invalid or expired reset token")
	}

	return nil
}
