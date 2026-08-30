package domain

import (
	"regexp"
	"time"
)

// mentionTokenRe matches mention tokens embedded in message content: <@type:id> or <@everyone>.
var mentionTokenRe = regexp.MustCompile(`<@(?:[^:>]+(?::[^>]+)?|everyone)>`)

// mentionTokenPartsRe captures the type and optional id from a <@type:id> or
// <@everyone> token. It is the single source of truth for parsing mention
// tokens; used by the mention hydrator, the message service, and the
// notification extractor.
var mentionTokenPartsRe = regexp.MustCompile(`<@([^:>]+)(?::([^>]+))?>`)

// MentionToken represents a single parsed mention token from message content.
type MentionToken struct {
	Type string
	ID   string // empty for "everyone"
}

// ParseMentionTokens extracts all mention tokens from content, returning
// Type and ID for each. For <@everyone>, Type is "everyone" and ID is "".
func ParseMentionTokens(content string) []MentionToken {
	matches := mentionTokenPartsRe.FindAllStringSubmatch(content, -1)
	tokens := make([]MentionToken, 0, len(matches))
	for _, m := range matches {
		if len(m) < 2 {
			continue
		}
		id := ""
		if len(m) >= 3 {
			id = m[2]
		}
		tokens = append(tokens, MentionToken{Type: m[1], ID: id})
	}
	return tokens
}

// Display-format regexes for FormatMentionsForDisplay. Each converts a raw
// mention token into a short human-readable label.
var (
	displayEveryoneRe = regexp.MustCompile(`<@everyone>`)
	displayUserRe     = regexp.MustCompile(`<@user:([^>]+)>`)
	displayChannelRe  = regexp.MustCompile(`<@channel:([^>]+)>`)
	displayProjectRe  = regexp.MustCompile(`<@project:([^>]+)>`)
	displayTaskRe     = regexp.MustCompile(`<@task:([^>]+)>`)
)

// StripMentionTokens removes mention tokens (<@user:123>, <@everyone>, etc.)
// from content, producing plain text suitable for FTS indexing. This ensures
// search snippets contain clean human-readable text instead of raw token syntax.
func StripMentionTokens(content string) string {
	return mentionTokenRe.ReplaceAllString(content, "")
}

// FormatMentionsForDisplay converts mention tokens to human-readable format
// for notifications and previews. Converts <@everyone> to @everyone and
// <@user:id> to @user, etc.
func FormatMentionsForDisplay(content string) string {
	content = displayEveryoneRe.ReplaceAllString(content, "@everyone")
	content = displayUserRe.ReplaceAllString(content, "@user")
	content = displayChannelRe.ReplaceAllString(content, "@channel")
	content = displayProjectRe.ReplaceAllString(content, "@project")
	content = displayTaskRe.ReplaceAllString(content, "@task")
	return content
}

type ConversationType string

const (
	ConvDirect   ConversationType = "direct"
	ConvGroup    ConversationType = "group"
	ConvChannel  ConversationType = "channel"
	ConvVoice    ConversationType = "voice"
	ConvCategory ConversationType = "category"
)

type NotificationLevel string

const (
	NotifLevelAll      NotificationLevel = "all"
	NotifLevelMentions NotificationLevel = "mentions"
	NotifLevelNothing  NotificationLevel = "nothing"
)

type PresenceStatus string

const (
	PresenceOnline  PresenceStatus = "online"
	PresenceAway    PresenceStatus = "away"
	PresenceDnd     PresenceStatus = "dnd"
	PresenceOffline PresenceStatus = "offline"
)

type Conversation struct {
	ID          string
	OrgID       string
	ParentID    *string
	Name        string
	Topic       string
	Type        ConversationType
	CreatedBy   string
	PositionKey string
	CreatedAt   time.Time
	UpdatedAt   time.Time
	DeletedAt   *time.Time

	LastMessage   *Message
	UnreadCount   int
	MemberCount   int
	NotifLevel    NotificationLevel
	Muted         bool
	PartnerUserID string
	PartnerName   string

	ProjectIDs []string
}

type CreateConversationParams struct {
	OrgID       string
	ParentID    *string
	Name        string
	Topic       string
	Type        ConversationType
	CreatedBy   string
	PositionKey string
	ProjectIDs  []string
}

type ConversationMember struct {
	ConversationID string
	UserID         string
	OrgID          string
	JoinedAt       time.Time
	LastReadAt     time.Time
	Muted          bool

	User *User
}

type Message struct {
	ID                 string
	ConversationID     string
	OrgID              string
	SenderID           string
	Content            string
	ParentID           *string
	ForwardedMessageID *string
	Pinned             bool
	PinnedAt           *time.Time
	PinnedBy           *string
	EditedAt           *time.Time
	DeletedAt          *time.Time
	CreatedAt          time.Time

	Sender                          *User
	ForwardedMessage                *Message
	ForwardedMessageConvName        string
	ForwardedMessageConvType        ConversationType
	ForwardedMessageConvProjectID   *string
	ForwardedMessageConvProjectName string
	Attachments                     []MessageAttachment
	Reactions                       []ReactionGroup
	Mentions                        *Mentions
}

type CreateMessageParams struct {
	ConversationID     string
	OrgID              string
	SenderID           string
	Content            string
	ParentID           *string
	ForwardedMessageID *string
	AttachmentIDs      []string
}

// EditMessageParams contains all parameters for editing a message.
// Use this struct to avoid long parameter lists when calling MessageService.EditMessage.
type EditMessageParams struct {
	MsgID    string
	ConvID   string
	OrgID    string
	EditorID string
	Content  string
}

// AddReactionParams contains all parameters for adding a reaction to a message.
// Use this struct to avoid long parameter lists when calling MessageService.AddReaction.
type AddReactionParams struct {
	MsgID  string
	ConvID string
	UserID string
	OrgID  string
	Emoji  string
}

// RemoveReactionParams contains all parameters for removing a reaction from a message.
// Use this struct to avoid long parameter lists when calling MessageService.RemoveReaction.
type RemoveReactionParams struct {
	MsgID  string
	ConvID string
	UserID string
	OrgID  string
	Emoji  string
}

type MessageAttachment struct {
	ID          string
	MessageID   string
	FileName    string
	FileSize    int64
	ContentType string
	StoragePath string
	URL         string
	CreatedAt   time.Time
}

type PendingAttachment struct {
	ID             string
	ConversationID string
	FileName       string
	FileSize       int64
	ContentType    string
	StoragePath    string
	UploadedBy     string
	URL            string
	CreatedAt      time.Time
}

type Reaction struct {
	MessageID string
	UserID    string
	Emoji     string
	CreatedAt time.Time
}

type ReactionGroup struct {
	Emoji   string
	Count   int
	UserIDs []string
	Mine    bool
}

type MentionType string

const (
	MentionUser     MentionType = "user"
	MentionProject  MentionType = "project"
	MentionTask     MentionType = "task"
	MentionChannel  MentionType = "channel"
	MentionEveryone MentionType = "everyone"
)

// MentionUserResult is a search result for user mentions.
type MentionUserResult struct {
	ID        string
	Name      string
	AvatarURL *string
}

// MentionChannelResult is a search result for channel mentions.
type MentionChannelResult struct {
	ID   string
	Name string
}

// MentionProjectResult is a search result for project mentions.
type MentionProjectResult struct {
	ID    string
	Name  string
	Icon  string
	Color string
}

// MentionTaskResult is a search result for task mentions.
type MentionTaskResult struct {
	ID          string
	Title       string
	ProjectID   string
	ProjectName string
}

type MentionResult struct {
	ID          string
	Type        MentionType
	Label       string
	AvatarURL   *string
	Icon        *string
	IconColor   *string
	ProjectID   *string
	ProjectName *string
}

// MessageMention represents a single mention in a message content.
type MessageMention struct {
	ID    string
	Type  MentionType
	Label string
}

// TaskMention holds resolved data for a task mention.
type TaskMention struct {
	Title     string
	ProjectID string
}

// Mentions holds resolved mention labels for all types found in message content.
type Mentions struct {
	Users    map[string]string
	Projects map[string]string
	Tasks    map[string]TaskMention
	Channels map[string]string
}

type MessageFilter struct {
	Before string
	Limit  int
}

type MessageListResult struct {
	Items      []*Message
	NextCursor string
	HasMore    bool
}

type MessageSearchScope string

const (
	MessageSearchScopeAll       MessageSearchScope = "all"
	MessageSearchScopeWorkspace MessageSearchScope = "workspace"
	MessageSearchScopeProject   MessageSearchScope = "project"
	MessageSearchScopeDM        MessageSearchScope = "dm"
)

type MessageSearchFilter struct {
	Query                string
	Scope                MessageSearchScope
	ConversationID       string
	SenderID             string
	HasAttachment        bool
	HasLink              bool
	IsPinned             bool
	After                string
	Before               string
	Sort                 string
	Cursor               string
	Limit                int
	IncludeProjectLinked bool
}

type MessageSearchResult struct {
	Message          *Message
	Rank             float64
	Snippet          string
	ConversationName string
	ConversationType ConversationType
}

type MessageSearchListResult struct {
	Items      []*MessageSearchResult
	NextCursor string
	HasMore    bool
}

type ConversationFilter struct {
	Cursor               string
	Limit                int
	Scope                *string
	IncludeProjectLinked bool
}

type ConversationListResult struct {
	Items      []*Conversation
	NextCursor string
	HasMore    bool
}

type UserPresence struct {
	UserID   string
	OrgID    string
	Status   PresenceStatus
	LastSeen time.Time

	User *User
}

type UserChannelPreference struct {
	UserID            string
	ConversationID    string
	OrgID             string
	NotificationLevel NotificationLevel
	Muted             bool
	LastReadAt        time.Time
}

// PermissionRule represents a permission rule for a channel (including category-type channels).
// Role can be "everyone", "owner", "admin", "member", or "viewer".
// Permission is one of: channel:view, channel:send, channel:manage, channel:permissions.
type PermissionRule struct {
	ChannelID  *string
	Role       Role
	Permission Permission
	Allow      bool
}

// UserPermissionOverride represents a per-user override for a channel permission.
type UserPermissionOverride struct {
	ChannelID  string
	UserID     string
	Permission Permission
	Allow      bool
}

// ChannelAccessEntry represents a user who has access to a channel.
// Source indicates how they got access: "explicit" for explicit membership,
// or "project:{name}" for implicit access via a project.
type ChannelAccessEntry struct {
	User   *User
	Source string
}

// ChannelPermissions represents the resolved permissions for a user in a channel.
type ChannelPermissions struct {
	CanView        bool
	CanSend        bool
	CanManage      bool
	CanPermissions bool
}

// EffectivePermission represents the computed effective permission for a specific
// role on a specific channel. Explicit indicates whether this was explicitly set
// at this channel level (vs inherited from a parent category or org default).
type EffectivePermission struct {
	Role       Role       `json:"role"`
	Permission Permission `json:"permission"`
	Allow      bool       `json:"allow"`
	Explicit   bool       `json:"explicit"`
}
