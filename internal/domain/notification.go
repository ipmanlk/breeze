package domain

import "time"

type NotificationType string

const (
	NotifTaskAssigned      NotificationType = "task_assigned"
	NotifTaskStatusChanged NotificationType = "task_status_changed"
	NotifTaskComment       NotificationType = "task_comment"
	NotifTaskDueSoon       NotificationType = "task_due_soon"
	NotifTaskOverdue       NotificationType = "task_overdue"
	NotifChatDM            NotificationType = "chat_dm"
	NotifChatMention       NotificationType = "chat_mention"
)

var DefaultNotificationPreferences = map[NotificationType]bool{
	NotifTaskAssigned:      true,
	NotifTaskStatusChanged: true,
	NotifTaskComment:       true,
	NotifTaskDueSoon:       true,
	NotifTaskOverdue:       true,
	NotifChatDM:            true,
	NotifChatMention:       true,
}

var AllNotificationTypes = []NotificationType{
	NotifTaskAssigned,
	NotifTaskStatusChanged,
	NotifTaskComment,
	NotifTaskDueSoon,
	NotifTaskOverdue,
	NotifChatDM,
	NotifChatMention,
}

type Notification struct {
	ID          string
	OrgID       string
	UserID      string
	Type        NotificationType
	Title       string
	Body        string
	Link        string
	EntityType  string
	EntityID    string
	ProjectSlug string
	ActorID     string
	IsRead      bool
	ReadAt      *time.Time
	CreatedAt   time.Time
	Actor       *User
}

type NotificationFilter struct {
	Cursor     string
	UnreadOnly bool
	Limit      int
}

type NotificationListResult struct {
	Items      []*Notification
	NextCursor string
	HasMore    bool
}

type NotificationPreference struct {
	Type    NotificationType
	Enabled bool
}

type DueTaskRow struct {
	TaskID      string
	Title       string
	DueAt       string
	StatusID    string
	AssigneeID  string
	OrgID       string
	ProjectID   string
	ProjectSlug string
}
