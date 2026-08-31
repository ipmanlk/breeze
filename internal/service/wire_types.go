// Package service contains business logic. This file holds wire-type
// definitions shared by comment and message broadcasting; the service layer
// builds typed broadcast payloads and pushes them through the port.Broadcaster
// interface without depending on the WS transport layer.
package service

import (
	"time"

	"ipmanlk/plume/internal/domain"
)

// ---------------------------------------------------------------------------
// Shared wire types (used by message and comment broadcast payloads)
// ---------------------------------------------------------------------------

type wireSender struct {
	ID        string  `json:"id"`
	Name      string  `json:"name"`
	Email     string  `json:"email"`
	AvatarURL *string `json:"avatar_url"`
	Role      string  `json:"role"`
}

type wireAttachment struct {
	ID          string `json:"id"`
	FileName    string `json:"file_name"`
	FileSize    int64  `json:"file_size"`
	ContentType string `json:"content_type"`
	URL         string `json:"url"`
	CreatedAt   string `json:"created_at"`
}

type wireReaction struct {
	Emoji   string   `json:"emoji"`
	Count   int      `json:"count"`
	UserIDs []string `json:"user_ids"`
	Mine    bool     `json:"mine"`
}

type wireTaskMention struct {
	Title     string `json:"title"`
	ProjectID string `json:"project_id"`
}

type wireMentions struct {
	Users    map[string]string          `json:"users,omitempty"`
	Projects map[string]string          `json:"projects,omitempty"`
	Tasks    map[string]wireTaskMention `json:"tasks,omitempty"`
	Channels map[string]string          `json:"channels,omitempty"`
}

type wireForwardedSender struct {
	ID   string `json:"id"`
	Name string `json:"name"`
}

type wireForwardedMessage struct {
	ID        string               `json:"id"`
	Content   string               `json:"content"`
	CreatedAt string               `json:"created_at"`
	Sender    *wireForwardedSender `json:"sender"`
}

type wireMessage struct {
	ID               string                `json:"id"`
	ConversationID   string                `json:"conversation_id"`
	OrgID            string                `json:"org_id"`
	SenderID         string                `json:"sender_id"`
	Content          string                `json:"content"`
	Pinned           bool                  `json:"pinned"`
	CreatedAt        string                `json:"created_at"`
	ParentID         string                `json:"parent_id,omitempty"`
	PinnedAt         string                `json:"pinned_at,omitempty"`
	EditedAt         string                `json:"edited_at,omitempty"`
	Sender           *wireSender           `json:"sender"`
	Attachments      []wireAttachment      `json:"attachments"`
	Reactions        []wireReaction        `json:"reactions"`
	Mentions         *wireMentions         `json:"mentions"`
	ForwardedMessage *wireForwardedMessage `json:"forwarded_message"`
}

type messageCreatedPayload struct {
	Message        wireMessage `json:"message"`
	ConversationID string      `json:"conversation_id"`
}

// buildWireMessage converts a domain.Message into the wire format expected by
// WebSocket clients (enriched with full sender, attachment, reaction objects).
func buildWireMessage(msg *domain.Message) wireMessage {
	m := wireMessage{
		ID:             msg.ID,
		ConversationID: msg.ConversationID,
		OrgID:          msg.OrgID,
		SenderID:       msg.SenderID,
		Content:        msg.Content,
		Pinned:         msg.Pinned,
		CreatedAt:      msg.CreatedAt.Format(time.RFC3339),
	}
	if msg.ParentID != nil {
		m.ParentID = *msg.ParentID
	}
	if msg.PinnedAt != nil {
		m.PinnedAt = msg.PinnedAt.Format(time.RFC3339)
	}
	if msg.EditedAt != nil {
		m.EditedAt = msg.EditedAt.Format(time.RFC3339)
	}
	if msg.Sender != nil {
		m.Sender = &wireSender{
			ID:        msg.Sender.ID,
			Name:      msg.Sender.Name,
			Email:     msg.Sender.Email,
			AvatarURL: msg.Sender.AvatarURL,
			Role:      string(msg.Sender.Role),
		}
	}
	if len(msg.Attachments) > 0 {
		m.Attachments = make([]wireAttachment, len(msg.Attachments))
		for i, a := range msg.Attachments {
			m.Attachments[i] = wireAttachment{
				ID:          a.ID,
				FileName:    a.FileName,
				FileSize:    a.FileSize,
				ContentType: a.ContentType,
				URL:         a.URL,
				CreatedAt:   a.CreatedAt.Format(time.RFC3339),
			}
		}
	}
	if len(msg.Reactions) > 0 {
		m.Reactions = make([]wireReaction, len(msg.Reactions))
		for i, r := range msg.Reactions {
			m.Reactions[i] = wireReaction{
				Emoji:   r.Emoji,
				Count:   r.Count,
				UserIDs: r.UserIDs,
				Mine:    r.Mine,
			}
		}
	}
	if msg.Mentions != nil {
		wm := &wireMentions{
			Users:    msg.Mentions.Users,
			Projects: msg.Mentions.Projects,
			Tasks:    make(map[string]wireTaskMention, len(msg.Mentions.Tasks)),
			Channels: msg.Mentions.Channels,
		}
		for id, t := range msg.Mentions.Tasks {
			wm.Tasks[id] = wireTaskMention{Title: t.Title, ProjectID: t.ProjectID}
		}
		m.Mentions = wm
	}
	if msg.ForwardedMessage != nil {
		fwd := &wireForwardedMessage{
			ID:        msg.ForwardedMessage.ID,
			Content:   msg.ForwardedMessage.Content,
			CreatedAt: msg.ForwardedMessage.CreatedAt.Format(time.RFC3339),
		}
		if msg.ForwardedMessage.Sender != nil {
			fwd.Sender = &wireForwardedSender{
				ID:   msg.ForwardedMessage.Sender.ID,
				Name: msg.ForwardedMessage.Sender.Name,
			}
		}
		m.ForwardedMessage = fwd
	}
	return m
}

// ---------------------------------------------------------------------------
// Comment wire types
// ---------------------------------------------------------------------------

type wireComment struct {
	ID              string        `json:"id"`
	TaskID          string        `json:"task_id"`
	ProjectID       string        `json:"project_id"`
	AuthorID        string        `json:"author_id"`
	Content         string        `json:"content"`
	ParentID        string        `json:"parent_id,omitempty"`
	CreatedAt       string        `json:"created_at"`
	UpdatedAt       string        `json:"updated_at"`
	EditedAt        string        `json:"edited_at,omitempty"`
	AuthorName      string        `json:"author_name"`
	AuthorEmail     string        `json:"author_email"`
	AuthorAvatarURL string        `json:"author_avatar_url,omitempty"`
	Mentions        *wireMentions `json:"mentions,omitempty"`
}

type commentPayload struct {
	Comment        wireComment `json:"comment"`
	TaskID         string      `json:"task_id"`
	ProjectID      string      `json:"project_id"`
	ConversationID string      `json:"conversation_id"`
}

// buildWireComment converts a domain.Comment into the wire format expected by
// WebSocket clients (enriched with author name/email, mention details).
func buildWireComment(c *domain.Comment) wireComment {
	wc := wireComment{
		ID:          c.ID,
		TaskID:      c.TaskID,
		ProjectID:   c.ProjectID,
		AuthorID:    c.AuthorID,
		Content:     c.Content,
		CreatedAt:   c.CreatedAt.Format(time.RFC3339),
		UpdatedAt:   c.UpdatedAt.Format(time.RFC3339),
		AuthorName:  c.AuthorName,
		AuthorEmail: c.AuthorEmail,
	}
	if c.ParentID != nil {
		wc.ParentID = *c.ParentID
	}
	if c.EditedAt != nil {
		wc.EditedAt = c.EditedAt.Format(time.RFC3339)
	}
	if c.AuthorAvatarURL != nil {
		wc.AuthorAvatarURL = *c.AuthorAvatarURL
	}
	if c.Mentions != nil {
		wm := &wireMentions{
			Users:    c.Mentions.Users,
			Projects: c.Mentions.Projects,
			Tasks:    make(map[string]wireTaskMention, len(c.Mentions.Tasks)),
			Channels: c.Mentions.Channels,
		}
		for id, t := range c.Mentions.Tasks {
			wm.Tasks[id] = wireTaskMention{Title: t.Title, ProjectID: t.ProjectID}
		}
		wc.Mentions = wm
	}
	return wc
}
