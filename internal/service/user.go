package service

import (
	"context"
	"fmt"
	"io"
	"log/slog"
	"path/filepath"
	"strings"

	"ipmanlk/breeze/internal/apperr"
	"ipmanlk/breeze/internal/auth"
	"ipmanlk/breeze/internal/domain"
	"ipmanlk/breeze/internal/port"
	"ipmanlk/breeze/internal/storage"

	"github.com/google/uuid"
)

const (
	maxNameLength     = 64
	minPasswordLength = 8
	maxAvatarBytes    = 2 << 20 // 2MB
	maxOwners         = 5
)

type UserService struct {
	userRepo    port.UserRepository
	sessionRepo port.SessionRepository
	accountRepo port.AccountRepository
	storage     storage.Storage
}

var _ port.UserService = (*UserService)(nil)

func NewUserService(
	userRepo port.UserRepository,
	sessionRepo port.SessionRepository,
	accountRepo port.AccountRepository,
	storage storage.Storage,
) *UserService {
	return &UserService{
		userRepo:    userRepo,
		sessionRepo: sessionRepo,
		accountRepo: accountRepo,
		storage:     storage,
	}
}

func (s *UserService) ListUsers(ctx context.Context, orgID string, filter domain.UserFilter) (*domain.UserListResult, error) {
	return s.userRepo.ListUsers(ctx, orgID, filter)
}

func (s *UserService) GetByID(ctx context.Context, orgID, id string) (*domain.User, error) {
	return s.userRepo.GetByID(ctx, orgID, id)
}

func (s *UserService) UpdateRole(ctx context.Context, orgID, id string, newRole domain.Role, callerRole domain.Role, callerID string) error {
	if id == callerID {
		return apperr.ErrForbidden
	}

	target, err := s.userRepo.GetByID(ctx, orgID, id)
	if err != nil {
		return err
	}

	if !domain.HasPermission(callerRole, domain.PermOrgMembersManage) {
		return apperr.ErrForbidden
	}

	if callerRole == domain.RoleAdmin && newRole == domain.RoleOwner {
		return apperr.ErrForbidden
	}

	if callerRole == domain.RoleAdmin && target.Role == domain.RoleOwner {
		return apperr.ErrForbidden
	}

	if err := s.userRepo.RunInTransaction(ctx, func(repo port.UserRepository) error {
		// Re-check owner counts inside the transaction so two concurrent
		// demotions/promotions can't both pass the check-then-act window.
		if target.Role == domain.RoleOwner && newRole != domain.RoleOwner {
			ownerCount, err := repo.CountOwners(ctx, orgID)
			if err != nil {
				return err
			}
			if ownerCount <= 1 {
				return fmt.Errorf("cannot demote the last owner")
			}
		}
		if newRole == domain.RoleOwner && target.Role != domain.RoleOwner {
			ownerCount, err := repo.CountOwners(ctx, orgID)
			if err != nil {
				return err
			}
			if ownerCount >= maxOwners {
				return fmt.Errorf("maximum number of owners reached")
			}
		}
		return repo.UpdateRole(ctx, orgID, id, newRole)
	}); err != nil {
		return err
	}

	if err := s.sessionRepo.DeleteByUser(ctx, id); err != nil {
		slog.Warn("revoke sessions after role/active change failed", "user_id", id, "error", err)
	}
	return nil
}

func (s *UserService) UpdateActive(ctx context.Context, orgID, id string, active bool, callerID string) error {
	if id == callerID {
		return apperr.ErrForbidden
	}

	target, err := s.userRepo.GetByID(ctx, orgID, id)
	if err != nil {
		return err
	}

	err = s.userRepo.RunInTransaction(ctx, func(repo port.UserRepository) error {
		if !active && target.Role == domain.RoleOwner {
			ownerCount, err := repo.CountOwners(ctx, orgID)
			if err != nil {
				return err
			}
			if ownerCount <= 1 {
				return fmt.Errorf("cannot deactivate the last owner")
			}
		}
		return repo.UpdateActive(ctx, orgID, id, active)
	})
	if err != nil {
		return err
	}

	// Revoke sessions only when deactivating; activating a user should not
	// invalidate their existing sessions. If session deletion fails, return
	// the error so the caller knows the deactivation is incomplete.
	if !active {
		if err := s.sessionRepo.DeleteByUser(ctx, id); err != nil {
			return fmt.Errorf("revoke sessions after deactivation: %w", err)
		}
	}

	return nil
}

// UpdateProfile changes the caller's display name (and optionally avatar) for
// the current org's membership. Because display columns are denormalized
// copies of the account identity, the update is propagated across ALL of the
// account's memberships so every workspace stays in sync.
func (s *UserService) UpdateProfile(ctx context.Context, orgID, userID, name string, avatarURL *string) (*domain.User, error) {
	name = strings.TrimSpace(name)
	if name == "" {
		return nil, apperr.InvalidInput("name is required")
	}
	if len(name) > maxNameLength {
		return nil, apperr.InvalidInput(fmt.Sprintf("name must be at most %d characters", maxNameLength))
	}

	user, err := s.userRepo.GetByID(ctx, orgID, userID)
	if err != nil {
		return nil, err
	}

	if err := s.userRepo.UpdateProfileByAccount(ctx, user.AccountID, name, avatarURL); err != nil {
		return nil, err
	}

	// Re-fetch the current org's membership to return the updated row.
	return s.userRepo.GetByID(ctx, orgID, userID)
}

// UploadAvatar stores an avatar image and links it to the account. The avatar
// URL is propagated across all of the account's memberships via
// UpdateProfileByAccount. The existing display name is preserved.
func (s *UserService) UploadAvatar(ctx context.Context, orgID, userID string, file io.Reader, filename, contentType string, size int64) (*domain.User, error) {
	if !strings.HasPrefix(contentType, "image/") {
		return nil, apperr.InvalidInput("avatar must be an image")
	}
	if size > maxAvatarBytes {
		return nil, apperr.InvalidInput(fmt.Sprintf("avatar must be at most %d bytes", maxAvatarBytes))
	}

	user, err := s.userRepo.GetByID(ctx, orgID, userID)
	if err != nil {
		return nil, err
	}

	ext := filepath.Ext(filename)
	storagePath := filepath.Join("avatars", user.AccountID, uuid.New().String()+ext)
	if err := s.storage.Save(ctx, storagePath, file); err != nil {
		return nil, fmt.Errorf("save avatar: %w", err)
	}

	avatarURL, err := s.storage.URL(ctx, storagePath)
	if err != nil {
		return nil, fmt.Errorf("avatar url: %w", err)
	}

	// Preserve the existing name; only the avatar changes.
	if err := s.userRepo.UpdateProfileByAccount(ctx, user.AccountID, user.Name, &avatarURL); err != nil {
		return nil, err
	}

	return s.userRepo.GetByID(ctx, orgID, userID)
}

// ChangePassword verifies the current credential, updates the account's
// password hash, then revokes the caller's sessions so they must re-authenticate.
// The frontend receives a 401 on the next call and redirects to login.
func (s *UserService) ChangePassword(ctx context.Context, orgID, userID, currentPassword, newPassword string) error {
	if len(newPassword) < minPasswordLength {
		return apperr.InvalidInput(fmt.Sprintf("new password must be at least %d characters", minPasswordLength))
	}

	user, err := s.userRepo.GetByID(ctx, orgID, userID)
	if err != nil {
		return err
	}

	account, err := s.accountRepo.GetByID(ctx, user.AccountID)
	if err != nil {
		return err
	}

	if !auth.CheckPassword(currentPassword, account.PasswordHash) {
		return apperr.ErrInvalidCreds
	}

	hash, err := auth.HashPassword(newPassword)
	if err != nil {
		return fmt.Errorf("hash password: %w", err)
	}

	if err := s.accountRepo.UpdatePassword(ctx, account.ID, hash); err != nil {
		return err
	}

	// Revoke the caller's sessions so the password change takes effect for
	// all active clients. The frontend logs out + redirects to /login.
	if err := s.sessionRepo.DeleteByUser(ctx, userID); err != nil {
		slog.Warn("revoke sessions after password change failed", "user_id", userID, "error", err)
	}
	return nil
}
