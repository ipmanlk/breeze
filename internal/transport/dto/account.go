package dto

// UpdateProfileRequest is the body for PATCH /api/account; changes the
// caller's display name. Avatar is set via the separate upload endpoint.
type UpdateProfileRequest struct {
	Name string `json:"name" validate:"required,min=1,max=64"`
}

// ChangePasswordRequest is the body for POST /api/account/change-password.
// On success the current session is revoked and the client must re-login.
type ChangePasswordRequest struct {
	CurrentPassword string `json:"current_password" validate:"required"`
	NewPassword     string `json:"new_password" validate:"required,min=8"`
}
