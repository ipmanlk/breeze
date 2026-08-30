package dto

import (
	"fmt"
	"time"

	"ipmanlk/breeze/internal/domain"
)

type CreateConversationRequest struct {
	Type       string   `json:"type" validate:"required,oneof=direct group channel voice category"`
	Name       string   `json:"name" validate:"max=100"`
	Topic      string   `json:"topic,omitempty" validate:"max=500"`
	ParentID   *string  `json:"parent_id,omitempty"`
	MemberIDs  []string `json:"member_ids,omitempty"`
	TargetID   *string  `json:"target_id,omitempty"`
	ProjectIDs []string `json:"project_ids,omitempty"`
}

type UpdateConversationRequest struct {
	Name  string  `json:"name,omitempty" validate:"max=100"`
	Topic *string `json:"topic,omitempty" validate:"omitempty,max=500"`
}

type UpdateChannelPositionRequest struct {
	ParentID    *string `json:"parent_id,omitempty"`
	PositionKey string  `json:"position_key"`
}

type AddMemberRequest struct {
	UserIDs []string `json:"user_ids" validate:"required,min=1"`
}

type MuteRequest struct {
	Muted bool `json:"muted"`
}

type NotificationLevelRequest struct {
	Level string `json:"level" validate:"required,oneof=all mentions nothing"`
}

type ConversationResponse struct {
	ID                string           `json:"id"`
	OrgID             string           `json:"org_id"`
	ParentID          *string          `json:"parent_id,omitempty"`
	Name              string           `json:"name"`
	Topic             string           `json:"topic,omitempty"`
	Type              string           `json:"type"`
	CreatedBy         string           `json:"created_by"`
	PositionKey       string           `json:"position_key"`
	CreatedAt         string           `json:"created_at"`
	UpdatedAt         string           `json:"updated_at"`
	UnreadCount       int              `json:"unread_count"`
	MemberCount       int              `json:"member_count"`
	Muted             bool             `json:"muted"`
	NotificationLevel string           `json:"notification_level"`
	LastMessage       *MessageResponse `json:"last_message,omitempty"`
	PartnerUserID     string           `json:"partner_user_id,omitempty"`
	PartnerName       string           `json:"partner_name,omitempty"`
	ProjectIDs        []string         `json:"project_ids,omitempty"`
}

func NewConversationResponse(c *domain.Conversation) *ConversationResponse {
	r := &ConversationResponse{
		ID:                c.ID,
		OrgID:             c.OrgID,
		ParentID:          c.ParentID,
		Name:              c.Name,
		Topic:             c.Topic,
		Type:              string(c.Type),
		CreatedBy:         c.CreatedBy,
		PositionKey:       c.PositionKey,
		CreatedAt:         c.CreatedAt.Format(time.RFC3339),
		UpdatedAt:         c.UpdatedAt.Format(time.RFC3339),
		UnreadCount:       c.UnreadCount,
		MemberCount:       c.MemberCount,
		Muted:             c.Muted,
		NotificationLevel: string(c.NotifLevel),
		PartnerUserID:     c.PartnerUserID,
		PartnerName:       c.PartnerName,
		ProjectIDs:        c.ProjectIDs,
	}
	if c.LastMessage != nil {
		r.LastMessage = NewMessageResponse(c.LastMessage)
	}
	return r
}

type ConversationMemberResponse struct {
	ID         string  `json:"id"`
	Name       string  `json:"name"`
	Email      string  `json:"email"`
	AvatarURL  *string `json:"avatar_url,omitempty"`
	Role       string  `json:"role"`
	JoinedAt   string  `json:"joined_at"`
	LastReadAt string  `json:"last_read_at"`
	Muted      bool    `json:"muted"`
	Online     bool    `json:"online"`
	Presence   string  `json:"presence,omitempty"`
}

func NewConversationMemberResponse(m *domain.ConversationMember) *ConversationMemberResponse {
	r := &ConversationMemberResponse{
		ID:         m.UserID,
		Name:       m.User.Name,
		Email:      m.User.Email,
		Role:       string(m.User.Role),
		JoinedAt:   m.JoinedAt.Format(time.RFC3339),
		LastReadAt: m.LastReadAt.Format(time.RFC3339),
		Muted:      m.Muted,
	}
	r.AvatarURL = publicAvatarURL(m.User.ID, m.User.AvatarURL)
	return r
}

type SendMessageRequest struct {
	Content            string   `json:"content" validate:"omitempty,max=50000"`
	ParentID           *string  `json:"parent_id,omitempty"`
	ForwardedMessageID *string  `json:"forwarded_message_id,omitempty"`
	AttachmentIDs      []string `json:"attachment_ids,omitempty"`
}

type EditMessageRequest struct {
	Content string `json:"content" validate:"required,max=50000"`
}

type ReactionRequest struct {
	Emoji string `json:"emoji" validate:"required,max=100"`
}

type UploadAttachmentResponse struct {
	ID          string `json:"id"`
	FileName    string `json:"file_name"`
	FileSize    int64  `json:"file_size"`
	ContentType string `json:"content_type"`
	URL         string `json:"url"`
}

// TaskMentionResponse is the serializable DTO for a resolved task mention.
type TaskMentionResponse struct {
	Title     string `json:"title"`
	ProjectID string `json:"project_id"`
}

type MentionsResponse struct {
	Users    map[string]string              `json:"users,omitempty"`
	Projects map[string]string              `json:"projects,omitempty"`
	Tasks    map[string]TaskMentionResponse `json:"tasks,omitempty"`
	Channels map[string]string              `json:"channels,omitempty"`
}

// ToMentionsResponse converts a domain.Mentions payload into the wire DTO.
// Shared by chat messages and task comments so both render mention chips
// identically on the frontend.
func ToMentionsResponse(m *domain.Mentions) *MentionsResponse {
	if m == nil {
		return nil
	}
	r := &MentionsResponse{
		Users:    m.Users,
		Projects: m.Projects,
		Channels: m.Channels,
	}
	r.Tasks = make(map[string]TaskMentionResponse, len(m.Tasks))
	for k, v := range m.Tasks {
		r.Tasks[k] = TaskMentionResponse{Title: v.Title, ProjectID: v.ProjectID}
	}
	return r
}

type MessageResponse struct {
	ID                       string                   `json:"id"`
	ConversationID           string                   `json:"conversation_id"`
	OrgID                    string                   `json:"org_id"`
	SenderID                 string                   `json:"sender_id"`
	Content                  string                   `json:"content"`
	ParentID                 *string                  `json:"parent_id,omitempty"`
	ForwardedMessageID       *string                  `json:"forwarded_message_id,omitempty"`
	Pinned                   bool                     `json:"pinned"`
	PinnedAt                 *string                  `json:"pinned_at,omitempty"`
	EditedAt                 *string                  `json:"edited_at,omitempty"`
	CreatedAt                string                   `json:"created_at"`
	Sender                   *UserResponse            `json:"sender,omitempty"`
	ForwardedMessage         *MessageResponse         `json:"forwarded_message,omitempty"`
	ForwardedFromName        string                   `json:"forwarded_from_name,omitempty"`
	ForwardedFromType        string                   `json:"forwarded_from_type,omitempty"`
	ForwardedFromProjectName string                   `json:"forwarded_from_project_name,omitempty"`
	Attachments              []*MessageAttachmentResp `json:"attachments,omitempty"`
	Reactions                []*ReactionGroupResp     `json:"reactions,omitempty"`
	Mentions                 *MentionsResponse        `json:"mentions,omitempty"`
}

type ReactionGroupResp struct {
	Emoji   string   `json:"emoji"`
	Count   int      `json:"count"`
	UserIDs []string `json:"user_ids"`
	Mine    bool     `json:"mine"`
}

type MessageAttachmentResp struct {
	ID          string `json:"id"`
	FileName    string `json:"file_name"`
	FileSize    int64  `json:"file_size"`
	ContentType string `json:"content_type"`
	URL         string `json:"url"`
	CreatedAt   string `json:"created_at"`
}

func NewMessageResponse(m *domain.Message) *MessageResponse {
	r := &MessageResponse{
		ID:                 m.ID,
		ConversationID:     m.ConversationID,
		OrgID:              m.OrgID,
		SenderID:           m.SenderID,
		Content:            m.Content,
		ParentID:           m.ParentID,
		ForwardedMessageID: m.ForwardedMessageID,
		Pinned:             m.Pinned,
		CreatedAt:          m.CreatedAt.Format(time.RFC3339),
	}
	if m.EditedAt != nil {
		s := m.EditedAt.Format(time.RFC3339)
		r.EditedAt = &s
	}
	if m.PinnedAt != nil {
		s := m.PinnedAt.Format(time.RFC3339)
		r.PinnedAt = &s
	}
	if m.Sender != nil {
		r.Sender = NewUserResponse(m.Sender)
	}
	if m.ForwardedMessage != nil {
		r.ForwardedMessage = NewMessageResponse(m.ForwardedMessage)
	}
	if m.ForwardedMessageConvName != "" {
		r.ForwardedFromName = m.ForwardedMessageConvName
		r.ForwardedFromType = string(m.ForwardedMessageConvType)
		if m.ForwardedMessageConvProjectName != "" {
			r.ForwardedFromProjectName = m.ForwardedMessageConvProjectName
		}
	}
	for _, a := range m.Attachments {
		r.Attachments = append(r.Attachments, &MessageAttachmentResp{
			ID:          a.ID,
			FileName:    a.FileName,
			FileSize:    a.FileSize,
			ContentType: a.ContentType,
			URL:         fmt.Sprintf("/api/conversations/%s/attachments/%s/download", m.ConversationID, a.ID),
			CreatedAt:   a.CreatedAt.Format(time.RFC3339),
		})
	}
	for _, rg := range m.Reactions {
		r.Reactions = append(r.Reactions, &ReactionGroupResp{
			Emoji:   rg.Emoji,
			Count:   rg.Count,
			UserIDs: rg.UserIDs,
			Mine:    rg.Mine,
		})
	}
	if m.Mentions != nil {
		r.Mentions = ToMentionsResponse(m.Mentions)
	}
	return r
}

type MentionSearchRequest struct {
	Query string   `json:"q"`
	Types []string `json:"types"`
}

type MentionResultResponse struct {
	ID          string  `json:"id"`
	Type        string  `json:"type"`
	Label       string  `json:"label"`
	AvatarURL   *string `json:"avatar_url,omitempty"`
	Icon        *string `json:"icon,omitempty"`
	IconColor   *string `json:"icon_color,omitempty"`
	ProjectID   *string `json:"project_id,omitempty"`
	ProjectName *string `json:"project_name,omitempty"`
}

func NewMentionResultResponse(m *domain.MentionResult) *MentionResultResponse {
	avatarURL := m.AvatarURL
	if m.Type == domain.MentionUser {
		avatarURL = publicAvatarURL(m.ID, m.AvatarURL)
	}
	return &MentionResultResponse{
		ID:          m.ID,
		Type:        string(m.Type),
		Label:       m.Label,
		AvatarURL:   avatarURL,
		Icon:        m.Icon,
		IconColor:   m.IconColor,
		ProjectID:   m.ProjectID,
		ProjectName: m.ProjectName,
	}
}

type MentionSearchResponse struct {
	Results []*MentionResultResponse `json:"results"`
}

type ConversationListResponse struct {
	Items      []*ConversationResponse `json:"items"`
	NextCursor string                  `json:"next_cursor"`
	HasMore    bool                    `json:"has_more"`
}

type MessageListResponse struct {
	Items      []*MessageResponse `json:"items"`
	NextCursor string             `json:"next_cursor"`
	HasMore    bool               `json:"has_more"`
}

type PinnedMessagesResponse struct {
	Items []*MessageResponse `json:"items"`
}

type MessageSearchResponse struct {
	Items      []*MessageSearchItemResponse `json:"items"`
	NextCursor string                       `json:"next_cursor,omitempty"`
	HasMore    bool                         `json:"has_more"`
}

type MessageSearchItemResponse struct {
	Message          *MessageResponse `json:"message"`
	Rank             float64          `json:"rank"`
	Snippet          string           `json:"snippet"`
	ConversationName string           `json:"conversation_name"`
	ConversationType string           `json:"conversation_type"`
}

func NewMessageSearchResponse(result *domain.MessageSearchListResult) *MessageSearchResponse {
	resp := &MessageSearchResponse{
		Items:      make([]*MessageSearchItemResponse, len(result.Items)),
		NextCursor: result.NextCursor,
		HasMore:    result.HasMore,
	}
	for i, item := range result.Items {
		resp.Items[i] = &MessageSearchItemResponse{
			Message:          NewMessageResponse(item.Message),
			Rank:             item.Rank,
			Snippet:          item.Snippet,
			ConversationName: item.ConversationName,
			ConversationType: string(item.ConversationType),
		}
	}
	return resp
}

// PermissionRuleRequest is the request body for permission rules.
type PermissionRuleRequest struct {
	Role       string `json:"role" validate:"required"`
	Permission string `json:"permission" validate:"required"`
	Allow      bool   `json:"allow"`
}

// PermissionRuleResponse represents a permission rule response.
type PermissionRuleResponse struct {
	Role       string `json:"role"`
	Permission string `json:"permission"`
	Allow      bool   `json:"allow"`
}

// NewPermissionRuleResponse creates a response from a domain permission rule.
func NewPermissionRuleResponse(r *domain.PermissionRule) *PermissionRuleResponse {
	return &PermissionRuleResponse{
		Role:       string(r.Role),
		Permission: string(r.Permission),
		Allow:      r.Allow,
	}
}

// EffectivePermissionResponse represents the computed effective permission for
// a specific role on a specific channel.
type EffectivePermissionResponse struct {
	Role       string `json:"role"`
	Permission string `json:"permission"`
	Allow      bool   `json:"allow"`
	Explicit   bool   `json:"explicit"`
}

// SetPermissionsRequest is the request body for setting permissions.
type SetPermissionsRequest struct {
	Rules []PermissionRuleRequest `json:"rules" validate:"required"`
}

// SetProjectLinksRequest is the request body for setting project links.
type SetProjectLinksRequest struct {
	ProjectIDs []string `json:"project_ids"`
}

// UserOverrideRequest is a per-user channel permission override.
type UserOverrideRequest struct {
	UserID     string `json:"user_id" validate:"required"`
	Permission string `json:"permission" validate:"required,oneof=channel:view channel:send channel:manage channel:permissions"`
	Allow      bool   `json:"allow"`
}

// UserOverrideResponse represents a per-user channel permission override.
type UserOverrideResponse struct {
	UserID     string `json:"user_id"`
	Permission string `json:"permission"`
	Allow      bool   `json:"allow"`
}

// NewUserOverrideResponse creates a response from a domain override.
func NewUserOverrideResponse(o *domain.UserPermissionOverride) *UserOverrideResponse {
	return &UserOverrideResponse{
		UserID:     o.UserID,
		Permission: string(o.Permission),
		Allow:      o.Allow,
	}
}

// SetUserOverridesRequest is the request body for setting user overrides.
type SetUserOverridesRequest struct {
	Overrides []UserOverrideRequest `json:"overrides" validate:"required"`
}

// ChannelAccessEntryResponse represents a user with access to a channel.
type ChannelAccessEntryResponse struct {
	User   *UserResponse `json:"user"`
	Source string        `json:"source"`
}

// ChannelPermissionsResponse represents resolved permissions for a user.
type ChannelPermissionsResponse struct {
	CanView        bool `json:"can_view"`
	CanSend        bool `json:"can_send"`
	CanManage      bool `json:"can_manage"`
	CanPermissions bool `json:"can_permissions"`
}

// NewChannelPermissionsResponse creates a response from domain permissions.
func NewChannelPermissionsResponse(p *domain.ChannelPermissions) *ChannelPermissionsResponse {
	return &ChannelPermissionsResponse{
		CanView:        p.CanView,
		CanSend:        p.CanSend,
		CanManage:      p.CanManage,
		CanPermissions: p.CanPermissions,
	}
}

type PresenceListResponse struct {
	Items []*UserPresenceResponse `json:"items"`
}

type UserPresenceResponse struct {
	UserID   string        `json:"user_id"`
	OrgID    string        `json:"org_id"`
	Status   string        `json:"status"`
	LastSeen string        `json:"last_seen"`
	User     *UserResponse `json:"user,omitempty"`
}

func NewUserPresenceResponse(p *domain.UserPresence) *UserPresenceResponse {
	return &UserPresenceResponse{
		UserID:   p.UserID,
		OrgID:    p.OrgID,
		Status:   string(p.Status),
		LastSeen: p.LastSeen.Format(time.RFC3339),
		User:     NewUserResponse(p.User),
	}
}

// VoiceParticipantResponse is the HTTP DTO for a voice channel participant.
type VoiceParticipantResponse struct {
	ID        string  `json:"id"`
	UserID    string  `json:"user_id"`
	Name      string  `json:"name"`
	AvatarURL *string `json:"avatar_url,omitempty"`
	Muted     bool    `json:"muted"`
	Deafened  bool    `json:"deafened"`
	Speaking  bool    `json:"speaking"`
	JoinedAt  string  `json:"joined_at"`
}

func NewVoiceParticipantResponse(p *domain.VoiceParticipantInfo) *VoiceParticipantResponse {
	return &VoiceParticipantResponse{
		ID:        p.ID,
		UserID:    p.UserID,
		Name:      p.Name,
		AvatarURL: publicAvatarURLString(p.UserID, p.AvatarURL),
		Muted:     p.Muted,
		Deafened:  p.Deafened,
		Speaking:  p.Speaking,
		JoinedAt:  p.JoinedAt.Format(time.RFC3339),
	}
}
