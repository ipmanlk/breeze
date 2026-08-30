package service

import (
	"context"
	"fmt"
	"slices"

	"ipmanlk/breeze/internal/domain"
	"ipmanlk/breeze/internal/port"
)

// validThemePresets is the set of preset IDs the frontend recognises.
var validThemePresets = []string{
	"light", "paper", "dark", "noir",
	"github-dark",
	"solarized-light", "solarized-dark",
	"dracula",
	"nord",
	"monokai",
	"catppuccin-latte", "catppuccin-mocha",
	"tokyo-night",
	"one-dark",
	"gruvbox", "gruvbox-light",
	"rose-pine", "rose-pine-dawn",
}

type UserPreferencesService struct {
	repo port.UserPreferencesRepository
}

var _ port.UserPreferencesService = (*UserPreferencesService)(nil)

func NewUserPreferencesService(repo port.UserPreferencesRepository) *UserPreferencesService {
	return &UserPreferencesService{repo: repo}
}

func (s *UserPreferencesService) Get(ctx context.Context, userID string) (*domain.UserPreferences, error) {
	return s.repo.Get(ctx, userID)
}

func (s *UserPreferencesService) Update(ctx context.Context, userID string, params domain.UpdateUserPreferencesParams) (*domain.UserPreferences, error) {
	prefs, err := s.repo.Get(ctx, userID)
	if err != nil {
		return nil, fmt.Errorf("get preferences: %w", err)
	}

	if params.Theme != nil {
		if !slices.Contains(validThemePresets, *params.Theme) {
			return nil, fmt.Errorf("invalid theme: %s", *params.Theme)
		}
		prefs.Theme = *params.Theme
	}
	if params.Language != nil {
		prefs.Language = *params.Language
	}
	if params.Timezone != nil {
		prefs.Timezone = *params.Timezone
	}
	if params.EmailNotifications != nil {
		prefs.EmailNotifications = *params.EmailNotifications
	}
	if params.DesktopNotifications != nil {
		prefs.DesktopNotifications = *params.DesktopNotifications
	}
	if params.NotificationSounds != nil {
		prefs.NotificationSounds = *params.NotificationSounds
	}
	if params.SidebarCollapsed != nil {
		prefs.SidebarCollapsed = *params.SidebarCollapsed
	}
	if params.MotionSettings != nil {
		prefs.MotionSettings = *params.MotionSettings
	}

	if err := s.repo.Upsert(ctx, prefs); err != nil {
		return nil, fmt.Errorf("upsert preferences: %w", err)
	}

	return prefs, nil
}
