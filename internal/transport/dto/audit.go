package dto

import "ipmanlk/breeze/internal/domain"

// AuditEntryResponse is a single audit-log row for the admin UI.
type AuditEntryResponse struct {
	ID         string `json:"id"`
	ActorID    string `json:"actor_id"`
	ActorName  string `json:"actor_name"`
	ActorEmail string `json:"actor_email"`
	Action     string `json:"action"`
	EntityType string `json:"entity_type"`
	EntityID   string `json:"entity_id"`
	Metadata   string `json:"metadata,omitempty"`
	CreatedAt  string `json:"created_at"`
}

// AuditLogResponse is the paginated envelope for GET /api/audit-log.
type AuditLogResponse struct {
	Items  []*AuditEntryResponse `json:"items"`
	Total  int                   `json:"total"`
	Limit  int                   `json:"limit"`
	Offset int                   `json:"offset"`
}

func NewAuditEntryResponse(e *domain.AuditEntry) *AuditEntryResponse {
	return &AuditEntryResponse{
		ID:         e.ID,
		ActorID:    e.ActorID,
		ActorName:  e.ActorName,
		ActorEmail: e.ActorEmail,
		Action:     string(e.Action),
		EntityType: e.EntityType,
		EntityID:   e.EntityID,
		Metadata:   e.Metadata,
		CreatedAt:  e.CreatedAt.Format("2006-01-02T15:04:05Z07:00"),
	}
}
