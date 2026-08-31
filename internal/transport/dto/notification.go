package dto

import (
	"ipmanlk/plume/internal/domain"
)

type UserBrief struct {
	ID        string  `json:"id"`
	Name      string  `json:"name"`
	Email     string  `json:"email"`
	AvatarURL *string `json:"avatar_url,omitempty"`
}

func NewUserBrief(u *domain.User) *UserBrief {
	return &UserBrief{
		ID:        u.ID,
		Name:      u.Name,
		Email:     u.Email,
		AvatarURL: publicAvatarURL(u.ID, u.AvatarURL),
	}
}

type NotificationResponse struct {
	ID          string     `json:"id"`
	Type        string     `json:"type"`
	Title       string     `json:"title"`
	Body        string     `json:"body"`
	Link        string     `json:"link"`
	EntityType  string     `json:"entity_type"`
	EntityID    string     `json:"entity_id"`
	ProjectSlug string     `json:"project_slug"`
	Actor       *UserBrief `json:"actor,omitempty"`
	IsRead      bool       `json:"is_read"`
	CreatedAt   string     `json:"created_at"`
}

type PaginatedNotificationsResponse struct {
	Items      []*NotificationResponse `json:"items"`
	NextCursor string                  `json:"next_cursor,omitempty"`
	HasMore    bool                    `json:"has_more"`
}

type UnreadCountResponse struct {
	Count int `json:"count"`
}

type NotificationPreferenceResponse struct {
	Type    string `json:"type"`
	Enabled bool   `json:"enabled"`
}

type SetNotificationPreferenceRequest struct {
	Enabled *bool `json:"enabled" validate:"required"`
}

func NewNotificationResponse(n *domain.Notification) *NotificationResponse {
	r := &NotificationResponse{
		ID:          n.ID,
		Type:        string(n.Type),
		Title:       n.Title,
		Body:        n.Body,
		Link:        n.Link,
		EntityType:  n.EntityType,
		EntityID:    n.EntityID,
		ProjectSlug: n.ProjectSlug,
		IsRead:      n.IsRead,
		CreatedAt:   n.CreatedAt.Format("2006-01-02T15:04:05Z07:00"),
	}
	if n.Actor != nil {
		r.Actor = NewUserBrief(n.Actor)
	}
	return r
}
