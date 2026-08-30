package dto

// UserPreferencesResponse is the JSON shape returned by GET /settings/preferences.
type UserPreferencesResponse struct {
	Theme                string `json:"theme"`
	Language             string `json:"language"`
	Timezone             string `json:"timezone"`
	EmailNotifications   bool   `json:"email_notifications"`
	DesktopNotifications bool   `json:"desktop_notifications"`
	NotificationSounds   bool   `json:"notification_sounds"`
	SidebarCollapsed     bool   `json:"sidebar_collapsed"`
	MotionSettings       string `json:"motion_settings"`
}

// UpdateUserPreferencesRequest is the JSON body accepted by PATCH /settings/preferences.
// All fields are optional; only provided fields are updated.
type UpdateUserPreferencesRequest struct {
	Theme                *string `json:"theme,omitempty"`
	Language             *string `json:"language,omitempty"`
	Timezone             *string `json:"timezone,omitempty"`
	EmailNotifications   *bool   `json:"email_notifications,omitempty"`
	DesktopNotifications *bool   `json:"desktop_notifications,omitempty"`
	NotificationSounds   *bool   `json:"notification_sounds,omitempty"`
	SidebarCollapsed     *bool   `json:"sidebar_collapsed,omitempty"`
	MotionSettings       *string `json:"motion_settings,omitempty"`
}
