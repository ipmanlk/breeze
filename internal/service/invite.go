package service

import (
	"context"
	"fmt"
	"time"

	"ipmanlk/plume/internal/apperr"
	"ipmanlk/plume/internal/auth"
	"ipmanlk/plume/internal/domain"
	"ipmanlk/plume/internal/i18n"
	"ipmanlk/plume/internal/port"

	"github.com/google/uuid"
)

const inviteTTL = 7 * 24 * time.Hour

type InviteService struct {
	userRepo    port.UserRepository
	accountRepo port.AccountRepository
	inviteRepo  port.UserInviteRepository
	pmRepo      port.ProjectMemberRepository
	projRepo    port.ProjectRepository
	mailer      port.Mailer
	appURL      string
	orgRepo     port.OrganizationRepository
	token       *auth.TokenService
	i18n        *i18n.Bundle
}

var _ port.UserInviteService = (*InviteService)(nil)

// localize is a nil-safe wrapper around i18n.Bundle.MustLocalize.
func (s *InviteService) localize(locale, messageID string, data map[string]any, pluralCount any) string {
	if s.i18n == nil {
		return messageID
	}
	return s.i18n.MustLocalize(locale, messageID, data, pluralCount)
}

func NewInviteService(userRepo port.UserRepository, accountRepo port.AccountRepository, inviteRepo port.UserInviteRepository,
	pmRepo port.ProjectMemberRepository, orgRepo port.OrganizationRepository, projRepo port.ProjectRepository,
	mailer port.Mailer, appURL string, i18nBundle *i18n.Bundle) *InviteService {
	return &InviteService{
		userRepo:    userRepo,
		accountRepo: accountRepo,
		inviteRepo:  inviteRepo,
		pmRepo:      pmRepo,
		projRepo:    projRepo,
		mailer:      mailer,
		appURL:      appURL,
		orgRepo:     orgRepo,
		token:       auth.NewTokenService(),
		i18n:        i18nBundle,
	}
}

func (s *InviteService) Create(ctx context.Context, params domain.CreateInviteParams, callerRole domain.Role) (*domain.UserInvite, string, error) {
	if params.Email != nil {
		e := domain.NormalizeEmail(*params.Email)
		params.Email = &e
	}
	if !canInviteRole(callerRole, params.Role) {
		return nil, "", apperr.ErrForbidden
	}

	token, tokenHash, err := s.token.GenerateRandomToken()
	if err != nil {
		return nil, "", fmt.Errorf("generate token: %w", err)
	}

	expiresAt := time.Now().UTC().Add(inviteTTL)
	if !params.ExpiresAt.IsZero() && params.ExpiresAt.After(time.Now().UTC()) {
		expiresAt = params.ExpiresAt
	}

	invite := &domain.UserInvite{
		ID:        uuid.New().String(),
		OrgID:     params.OrgID,
		Email:     params.Email,
		Role:      params.Role,
		TokenHash: tokenHash,
		InvitedBy: params.InvitedBy,
		ExpiresAt: expiresAt,
	}

	if err := s.inviteRepo.Create(ctx, invite); err != nil {
		return nil, "", fmt.Errorf("create invite: %w", err)
	}

	// Store project assignments if provided
	for _, a := range params.ProjectAssignments {
		if a.Role != domain.RoleAdmin && a.Role != domain.RoleMember && a.Role != domain.RoleViewer && a.Role != domain.RoleGuest {
			return nil, "", fmt.Errorf("invalid project role: %s", a.Role)
		}
		// Validate the project belongs to the invite's org. Without this, a
		// malicious inviter could grant membership in a foreign-org project;
		// the project_members row would reference a project outside this org.
		if s.projRepo != nil {
			if _, err := s.projRepo.GetByID(ctx, params.OrgID, a.ProjectID); err != nil {
				return nil, "", apperr.InvalidInput("project " + a.ProjectID + " is not in this organization")
			}
		}
		if err := s.inviteRepo.AddInviteProject(ctx, invite.ID, a.ProjectID, a.Role); err != nil {
			return nil, "", fmt.Errorf("add invite project: %w", err)
		}
	}

	// Best-effort invite email. The token is always returned in the API
	// response so the inviter can share it manually if email is not
	// configured or delivery fails.
	if s.mailer != nil && s.mailer.Enabled() && invite.Email != nil && *invite.Email != "" {
		inviterName := ""
		if params.InvitedBy != "" {
			if inviter, err := s.userRepo.GetByID(ctx, params.OrgID, params.InvitedBy); err == nil && inviter != nil {
				inviterName = inviter.Name
			}
		}
		orgName := ""
		if org, err := s.orgRepo.GetByID(ctx, params.OrgID); err == nil && org != nil {
			orgName = org.Name
		}
		tmpl := InviteEmail(inviterName, orgName, joinURL(s.appURL, token))
		// Localize the email subject via the i18n bundle.
		locale := i18n.LocaleFromContext(ctx)
		if localized := s.localize(locale, "InviteSubject", map[string]any{"Inviter": inviterName, "Org": orgName}, nil); localized != "InviteSubject" {
			tmpl.Subject = localized
		}
		_ = s.mailer.Send(ctx, *invite.Email, tmpl.Subject, tmpl.HTML, tmpl.Text)
	}

	return invite, token, nil
}

func (s *InviteService) List(ctx context.Context, orgID string, limit int) ([]*domain.UserInvite, error) {
	return s.inviteRepo.ListByOrg(ctx, orgID, limit)
}

func (s *InviteService) Revoke(ctx context.Context, orgID, id string) error {
	return s.inviteRepo.Delete(ctx, orgID, id)
}

func (s *InviteService) Validate(ctx context.Context, token string) (*domain.UserInvite, error) {
	tokenHash := s.token.HashToken(token)
	invite, err := s.inviteRepo.GetByTokenHash(ctx, tokenHash)
	if err != nil {
		return nil, apperr.ErrNotFound
	}
	if invite.IsExpired() {
		return nil, apperr.ErrSessionExpired
	}
	if !invite.HasUsesRemaining() {
		return nil, apperr.ErrForbidden
	}
	return invite, nil
}

func (s *InviteService) Accept(ctx context.Context, params domain.AcceptInviteParams) (*domain.User, string, error) {
	params.Email = domain.NormalizeEmail(params.Email)
	invite, err := s.Validate(ctx, params.Token)
	if err != nil {
		return nil, "", err
	}

	if invite.Email != nil && *invite.Email != params.Email {
		return nil, "", apperr.ErrForbidden
	}

	_, err = s.userRepo.GetByEmail(ctx, invite.OrgID, params.Email)
	if err == nil {
		return nil, "", apperr.ErrAlreadyExists
	}

	passwordHash, err := auth.HashPassword(params.Password)
	if err != nil {
		return nil, "", fmt.Errorf("hash password: %w", err)
	}

	// Find-or-create the global account by email. If the account exists (the
	// person already belongs to another workspace), keep their existing
	// password; otherwise create the account with the new password.
	account, err := s.accountRepo.GetByEmail(ctx, params.Email)
	if err != nil {
		// No existing account; create one.
		accountID := uuid.New().String()
		account = &domain.Account{
			ID:           accountID,
			Email:        params.Email,
			PasswordHash: passwordHash,
		}
		if err := s.accountRepo.Create(ctx, account); err != nil {
			return nil, "", fmt.Errorf("create account: %w", err)
		}
	}

	userID := uuid.New().String()
	user := &domain.User{
		ID:        userID,
		AccountID: account.ID,
		OrgID:     invite.OrgID,
		Email:     params.Email,
		Name:      params.Name,
		Role:      invite.Role,
		IsActive:  true,
	}

	if err := s.userRepo.Create(ctx, user); err != nil {
		return nil, "", fmt.Errorf("create user: %w", err)
	}

	if err := s.inviteRepo.AcceptInvite(ctx, invite.ID, userID); err != nil {
		return nil, "", fmt.Errorf("accept invite: %w", err)
	}

	// Apply project assignments if the invite had any
	projectAssignments, err := s.inviteRepo.ListInviteProjects(ctx, invite.ID)
	if err != nil {
		return nil, "", fmt.Errorf("list invite projects: %w", err)
	}
	for _, a := range projectAssignments {
		if err := s.pmRepo.Add(ctx, a.ProjectID, userID, a.Role); err != nil {
			return nil, "", fmt.Errorf("add project member for invite: %w", err)
		}
	}

	user, err = s.userRepo.GetByID(ctx, invite.OrgID, userID)
	return user, "", err
}

func canInviteRole(callerRole, targetRole domain.Role) bool {
	switch callerRole {
	case domain.RoleOwner:
		return true
	case domain.RoleAdmin:
		return targetRole != domain.RoleOwner
	case domain.RoleMember:
		return targetRole == domain.RoleMember || targetRole == domain.RoleViewer || targetRole == domain.RoleGuest
	default:
		return false
	}
}
