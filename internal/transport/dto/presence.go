package dto

type SetStatusRequest struct {
	Status string `json:"status" validate:"required,oneof=online away dnd offline"`
}
