package dto

import "strings"

// publicAvatarURL converts an internal storage path (e.g. /uploads/avatars/...)
// into an auth-protected avatar endpoint URL. Returns nil when the user has no
// avatar or when userID is empty.
func publicAvatarURL(userID string, rawURL *string) *string {
	if rawURL == nil || *rawURL == "" || userID == "" {
		return nil
	}
	return publicAvatarURLString(userID, *rawURL)
}

// publicAvatarURLString is the same as publicAvatarURL but accepts a string
// value (used by DTOs backed by domains that store the URL as a plain string).
func publicAvatarURLString(userID, rawURL string) *string {
	if rawURL == "" || userID == "" {
		return nil
	}
	if strings.HasPrefix(rawURL, "/api/avatars/") {
		endpoint := rawURL
		return &endpoint
	}
	endpoint := "/api/avatars/" + userID
	return &endpoint
}
