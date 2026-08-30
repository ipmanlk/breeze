package service

import (
	"context"
	"fmt"
	"log/slog"
	"strings"
	"sync"
	"time"

	"ipmanlk/breeze/internal/apperr"
	"ipmanlk/breeze/internal/auth"
	"ipmanlk/breeze/internal/domain"
	"ipmanlk/breeze/internal/port"

	"github.com/google/uuid"
)

// OrganizationService implements port.OrganizationService. It owns org +
// account (credential) + membership creation, the workspace switcher list,
// and workspace switching (new session scoped to the target org).
//
// Setup (Create) and CreateWorkspace both create an org + an owner
// membership. The difference: setup creates a brand-new account (first-run,
// email is new), while CreateWorkspace links an existing authenticated
// account to a new org.
type OrganizationService struct {
	orgRepo     port.OrganizationRepository
	userRepo    port.UserRepository
	accountRepo port.AccountRepository
	sessionRepo port.SessionRepository
	jwt         *auth.JWTService
	postCreate  func(ctx context.Context, orgID, userID string) error
	// setupMu serializes first-run setup (see Create).
	setupMu sync.Mutex
}

var _ port.OrganizationService = (*OrganizationService)(nil)

func NewOrganizationService(
	orgRepo port.OrganizationRepository,
	userRepo port.UserRepository,
	accountRepo port.AccountRepository,
	sessionRepo port.SessionRepository,
	jwtSecret string,
) *OrganizationService {
	return &OrganizationService{
		orgRepo:     orgRepo,
		userRepo:    userRepo,
		accountRepo: accountRepo,
		sessionRepo: sessionRepo,
		jwt:         auth.NewJWTService(jwtSecret),
	}
}

// SetPostCreateHook registers a callback invoked immediately after the org and
// owner membership are persisted. Used by app.go to auto-seed a #general
// channel.
func (s *OrganizationService) SetPostCreateHook(fn func(ctx context.Context, orgID, userID string) error) {
	s.postCreate = fn
}

// Create is the first-run setup path: create a new account (the email is new
// at setup), an org, and an owner membership. Guarded by orgRepo.Exists()
// ("any org yet?") at the handler/middleware layer.
func (s *OrganizationService) Create(ctx context.Context, name, adminName, adminEmail, adminPassword string) (*domain.Organization, *domain.User, error) {
	// Serialize the exists-check and the create so two concurrent setup
	// requests can't both pass the check (check-then-act). The mutex works
	// because Breeze is a single process; combined with the unique email
	// constraint this closes the setup race.
	adminEmail = domain.NormalizeEmail(adminEmail)

	s.setupMu.Lock()
	defer s.setupMu.Unlock()

	exists, err := s.orgRepo.Exists(ctx)
	if err != nil {
		return nil, nil, err
	}
	if exists {
		return nil, nil, apperr.ErrSetupComplete
	}

	orgID := uuid.New().String()
	slug := slugify(name, "org")

	passwordHash, err := auth.HashPassword(adminPassword)
	if err != nil {
		return nil, nil, err
	}

	accountID := uuid.New().String()
	userID := uuid.New().String()

	// Create org, account, and user atomically.
	if err := s.orgRepo.CreateOrgWithAccountAndUser(ctx, &domain.Organization{
		ID:   orgID,
		Name: name,
		Slug: slug,
	}, accountID, userID, passwordHash, adminEmail, adminName); err != nil {
		return nil, nil, err
	}

	org := &domain.Organization{
		ID:   orgID,
		Name: name,
		Slug: slug,
	}

	user := &domain.User{
		ID:        userID,
		AccountID: accountID,
		OrgID:     orgID,
		Email:     adminEmail,
		Name:      adminName,
		Role:      domain.RoleOwner,
		IsActive:  true,
	}

	if s.postCreate != nil {
		if err := s.postCreate(ctx, orgID, userID); err != nil {
			return org, user, fmt.Errorf("post-create: %w", err)
		}
	}

	return org, user, nil
}

func (s *OrganizationService) GetByID(ctx context.Context, id string) (*domain.Organization, error) {
	return s.orgRepo.GetByID(ctx, id)
}

// Update renames an organization and sets its message edit window. The slug
// is recomputed from the new name; on a collision with a different org a
// short suffix is appended (mirroring CreateWorkspace). The edit window is
// clamped to 0-10080 minutes (0 = no limit).
func (s *OrganizationService) Update(ctx context.Context, orgID, name string, messageEditWindowMinute int) (*domain.Organization, error) {
	name = strings.TrimSpace(name)
	if len(name) < 2 || len(name) > 64 {
		return nil, apperr.InvalidInput("name must be between 2 and 64 characters")
	}
	if messageEditWindowMinute < 0 {
		messageEditWindowMinute = 0
	}
	if messageEditWindowMinute > 10080 {
		messageEditWindowMinute = 10080
	}

	org, err := s.orgRepo.GetByID(ctx, orgID)
	if err != nil {
		return nil, err
	}

	slug := slugify(name, "org")
	if slug != org.Slug {
		// Ensure slug uniqueness; append a short suffix on collision with a
		// different org rather than failing.
		if existing, err := s.orgRepo.GetBySlug(ctx, slug); err == nil && existing != nil && existing.ID != org.ID {
			slug = slugify(slug+"-"+org.ID[:8], "org")
		}
	}

	org.Name = name
	org.Slug = slug
	org.MessageEditWindowMinute = messageEditWindowMinute
	if err := s.orgRepo.Update(ctx, org); err != nil {
		return nil, err
	}
	return org, nil
}

// Delete removes an organization. Full cascade delete of all org data is
// out of scope for the initial release; this is a hard DELETE of the organizations row (the
// "danger zone" action). The service enforces the type-to-confirm guard
// (confirmName must match the org name) before deleting.
func (s *OrganizationService) Delete(ctx context.Context, orgID, confirmName string) error {
	org, err := s.orgRepo.GetByID(ctx, orgID)
	if err != nil {
		return err
	}
	if confirmName != org.Name {
		return apperr.InvalidInput("type the org name to confirm")
	}
	return s.orgRepo.Delete(ctx, orgID)
}

func (s *OrganizationService) Exists(ctx context.Context) (bool, error) {
	return s.orgRepo.Exists(ctx)
}

// ListForAccount returns every workspace (org + the account's role) the
// account is a member of, newest membership first. Powers the switcher list.
func (s *OrganizationService) ListForAccount(ctx context.Context, accountID string) ([]*domain.Workspace, error) {
	return s.orgRepo.ListForAccount(ctx, accountID)
}

// CreateWorkspace lets an authenticated account spin up a new workspace. It
// creates an org and an owner membership linking the calling account, then
// runs the post-create hook (#general channel). displayName/email/avatarURL
// are copied from the caller's current membership so the new membership's
// display columns are populated without a separate account lookup.
func (s *OrganizationService) CreateWorkspace(ctx context.Context, accountID, name, displayName, email string, avatarURL *string) (*domain.Organization, *domain.User, error) {
	email = domain.NormalizeEmail(email)
	orgID := uuid.New().String()
	slug := slugify(name, "org")

	// Ensure slug uniqueness; append a short suffix on collision rather than
	// failing. Setup's single-org assumption made this a non-issue before.
	if existing, err := s.orgRepo.GetBySlug(ctx, slug); err == nil && existing != nil {
		slug = slugify(slug+"-"+orgID[:8], "org")
	}

	// Create org and user atomically.
	userID := uuid.New().String()
	if err := s.orgRepo.CreateOrgWithUser(ctx, &domain.Organization{
		ID:   orgID,
		Name: name,
		Slug: slug,
	}, userID, accountID, displayName, email, avatarURL); err != nil {
		return nil, nil, err
	}

	org := &domain.Organization{
		ID:   orgID,
		Name: name,
		Slug: slug,
	}

	user := &domain.User{
		ID:        userID,
		AccountID: accountID,
		OrgID:     orgID,
		Email:     email,
		Name:      displayName,
		Role:      domain.RoleOwner,
		AvatarURL: avatarURL,
		IsActive:  true,
	}

	if s.postCreate != nil {
		if err := s.postCreate(ctx, orgID, user.ID); err != nil {
			return org, user, fmt.Errorf("post-create: %w", err)
		}
	}

	return org, user, nil
}

// SwitchWorkspace verifies the account is an active member of orgID, revokes
// the caller's current session, and issues a new session (JWT) scoped to the
// target org with that membership's role. Returns the new session + token.
func (s *OrganizationService) SwitchWorkspace(ctx context.Context, accountID, orgID, currentSessionID string) (*domain.Session, string, error) {
	membership, err := s.userRepo.GetByOrgAndAccount(ctx, orgID, accountID)
	if err != nil {
		return nil, "", apperr.ErrNotFound
	}
	if membership == nil || !membership.IsActive {
		return nil, "", apperr.ErrForbidden
	}

	// Revoke the current (source) session so the old workspace's JWT can no
	// longer be used. Best-effort: a missing/expired session is fine.
	if currentSessionID != "" {
		if err := s.sessionRepo.Revoke(ctx, currentSessionID); err != nil {
			// A failure here means the source workspace's session may remain
			// valid. Log it so operators can detect stale-session issues; the
			// new session is still issued so the switch itself doesn't fail.
			slog.Warn("revoke source session on workspace switch failed", "session_id", currentSessionID, "error", err)
		}
	}

	session := &domain.Session{
		ID:        uuid.New().String(),
		UserID:    membership.ID,
		OrgID:     orgID,
		Role:      membership.Role,
		ExpiresAt: sessionExpiry(),
	}
	if err := s.sessionRepo.Create(ctx, session); err != nil {
		return nil, "", fmt.Errorf("create session: %w", err)
	}

	token, err := s.jwt.GenerateToken(session)
	if err != nil {
		return nil, "", fmt.Errorf("generate token: %w", err)
	}
	return session, token, nil
}

// sessionExpiry returns the absolute expiry for a newly issued session. Mirrors
// the 7-day TTL used by AuthService.Login (sessionTTL in auth.go) so workspace
// switches produce sessions with the same lifetime.
func sessionExpiry() time.Time {
	return time.Now().UTC().Add(7 * 24 * time.Hour)
}

func slugify(name, fallback string) string {
	lower := strings.ToLower(name)
	var slug strings.Builder
	lastDash := false
	for _, r := range lower {
		if (r >= 'a' && r <= 'z') || (r >= '0' && r <= '9') || r == '_' {
			slug.WriteRune(r)
			lastDash = false
		} else if r == '-' || r == ' ' {
			if !lastDash {
				slug.WriteRune('-')
			}
			lastDash = true
		}
	}
	result := strings.Trim(slug.String(), "-_")
	if result == "" {
		result = fallback
	}
	return result
}
