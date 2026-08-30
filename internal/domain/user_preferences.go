package domain

import "time"

// UserPreferences stores per-user account preferences (theme, notifications,
// editor options, etc.). One row per user, created on first access with
// sensible defaults.
type UserPreferences struct {
	UserID               string
	Theme                string // preset ID, e.g. "dark", "noir", "github-dark"
	Language             string // BCP-47 tag, e.g. "en"
	Timezone             string // IANA tz, e.g. "UTC"
	EmailNotifications   bool
	DesktopNotifications bool
	NotificationSounds   bool
	SidebarCollapsed     bool
	MotionSettings       string // JSON blob of frontend motion/animation preferences
	CreatedAt            time.Time
	UpdatedAt            time.Time
}

// DefaultUserPreferences returns a new UserPreferences with production
// defaults. Callers set UserID before persisting.
func DefaultUserPreferences(userID string) *UserPreferences {
	now := time.Now()
	return &UserPreferences{
		UserID:               userID,
		Theme:                "dark",
		Language:             "en",
		Timezone:             "UTC",
		EmailNotifications:   true,
		DesktopNotifications: true,
		NotificationSounds:   true,
		SidebarCollapsed:     false,
		MotionSettings:       "{}",
		CreatedAt:            now,
		UpdatedAt:            now,
	}
}

// UpdateUserPreferencesParams is the service-layer input for a partial
// preferences update. Pointer fields allow distinguishing "not sent" from
// "set to zero value".
type UpdateUserPreferencesParams struct {
	Theme                *string `json:"theme,omitempty"`
	Language             *string `json:"language,omitempty"`
	Timezone             *string `json:"timezone,omitempty"`
	EmailNotifications   *bool   `json:"email_notifications,omitempty"`
	DesktopNotifications *bool   `json:"desktop_notifications,omitempty"`
	NotificationSounds   *bool   `json:"notification_sounds,omitempty"`
	SidebarCollapsed     *bool   `json:"sidebar_collapsed,omitempty"`
	MotionSettings       *string `json:"motion_settings,omitempty"`
}
