package port

import (
	"context"
	"io"
	"time"

	"ipmanlk/breeze/internal/domain"
)

// AccessChecker provides defense-in-depth authorization for service-level
// operations. It duplicates the checks that HTTP middleware performs, so that
// business-logic operations are still authorized when called outside an HTTP
// request context (e.g. from background jobs, WS handlers, or tests).
//
// Services should be updated to call the appropriate check method before
// mutating a resource. A nil AccessChecker, or one with nil repos, FAILS
// CLOSED: it returns apperr.ErrForbidden (denies all access). This is a
// deliberate safety measure so a miss-wired checker can never silently
// allow unauthorized operations.
type AccessChecker interface {
	// RequireProjectAccess checks that the user has the given permission for
	// the specified project. Returns nil if allowed, or apperr.ErrForbidden.
	// A nil receiver or nil deps returns apperr.ErrForbidden (fail-closed).
	RequireProjectAccess(ctx context.Context, userID, orgID, projectID string, perm domain.Permission) error

	// RequireTaskAccess resolves taskID → projectID and checks the user has
	// `perm` on that project. Used by task-scoped services (comments,
	// attachments, time entries) where the caller knows the task
	// but not necessarily the project.
	RequireTaskAccess(ctx context.Context, userID, orgID, taskID string, perm domain.Permission) error

	// RequireOrgAccess checks that the user's org-level role has the given
	// permission. Used for org-scoped resources such as labels.
	RequireOrgAccess(ctx context.Context, userID, orgID string, perm domain.Permission) error
}

type TokenService interface {
	GenerateRandomToken() (string, string, error)
	HashToken(token string) string
}

type AuthService interface {
	Login(ctx context.Context, p domain.LoginParams) (*domain.Account, []*domain.User, string, error)
	ValidateSession(ctx context.Context, tokenString string) (*domain.Session, error)
	// ValidateSessionByID re-checks a session without a JWT (used by live
	// WebSocket connections to notice revocation, deactivation, or role
	// changes that happened after the connection was upgraded).
	ValidateSessionByID(ctx context.Context, sessionID string) (*domain.Session, error)
	Logout(ctx context.Context, sessionID string) error
	ListSessions(ctx context.Context, userID string) ([]*domain.Session, error)
	RevokeSession(ctx context.Context, userID, sessionID string) error
	HashPassword(password string) (string, error)
	CheckPassword(password, encodedHash string) bool
	RequestPasswordReset(ctx context.Context, email string) (string, error)
	ConfirmPasswordReset(ctx context.Context, token, newPassword string) error
	ValidateResetToken(ctx context.Context, token string) error
}

type OrganizationService interface {
	GetByID(ctx context.Context, id string) (*domain.Organization, error)
	Create(ctx context.Context, name, adminName, adminEmail, adminPassword string) (*domain.Organization, *domain.User, error)
	Exists(ctx context.Context) (bool, error)
	ListForAccount(ctx context.Context, accountID string) ([]*domain.Workspace, error)
	CreateWorkspace(ctx context.Context, accountID, name, displayName, email string, avatarURL *string) (*domain.Organization, *domain.User, error)
	SwitchWorkspace(ctx context.Context, accountID, orgID, currentSessionID string) (*domain.Session, string, error)
	Update(ctx context.Context, orgID, name string, messageEditWindowMinute int) (*domain.Organization, error)
	// Delete removes an organization. The caller must provide the org's name
	// as confirmName to confirm the action (type-to-confirm guard).
	Delete(ctx context.Context, orgID, confirmName string) error
}

type ProjectService interface {
	List(ctx context.Context, orgID string) ([]*domain.Project, error)
	ListIncludingArchived(ctx context.Context, orgID string) ([]*domain.Project, error)
	ListForUser(ctx context.Context, orgID, userID string) ([]*domain.Project, error)
	ListForUserIncludingArchived(ctx context.Context, orgID, userID string) ([]*domain.Project, error)
	// ListForCaller returns projects scoped to the caller's org role.
	// Elevated roles (owner/admin/member) see all org projects; viewer/guest
	// see only projects they have explicit membership in.
	ListForCaller(ctx context.Context, orgID, userID string, role domain.Role, includeArchived bool) ([]*domain.Project, error)
	GetByID(ctx context.Context, orgID, id string) (*domain.Project, error)
	GetBySlug(ctx context.Context, orgID, slug string) (*domain.Project, error)
	Create(ctx context.Context, orgID, name string, createdBy string, cycleDuration *int, autoGenerateCycles bool, incompleteTaskHandling domain.CycleCompletionHandling, startsAt, endsAt *time.Time) (*domain.Project, error)
	Update(ctx context.Context, p *domain.Project) error
	Delete(ctx context.Context, orgID, id string) error
	Archive(ctx context.Context, orgID, id string) error
	Unarchive(ctx context.Context, orgID, id string) error
}

type TaskStatusService interface {
	List(ctx context.Context, projectID string) ([]*domain.TaskStatus, error)
	GetByID(ctx context.Context, id string) (*domain.TaskStatus, error)
	Create(ctx context.Context, userID, orgID string, params domain.CreateTaskStatusParams) (*domain.TaskStatus, error)
	Update(ctx context.Context, userID, orgID string, s *domain.TaskStatus) error
	Delete(ctx context.Context, userID, orgID, id, projectID string) error
}

type TaskService interface {
	List(ctx context.Context, orgID, projectID string, filter domain.TaskFilter) ([]*domain.Task, error)
	ListTasks(ctx context.Context, orgID, userID string, role domain.Role, filter domain.TaskListFilter) (*domain.TaskListResult, error)
	ListSubtasks(ctx context.Context, orgID, projectID, parentID string) ([]*domain.Task, error)
	ReorderSubtasks(ctx context.Context, orgID, projectID, parentID string, ops []domain.ReorderOp) error
	GetByID(ctx context.Context, orgID, id, projectID string) (*domain.Task, error)
	Create(ctx context.Context, params domain.CreateTaskParams) (*domain.Task, error)
	Update(ctx context.Context, actorID string, t *domain.Task) error
	BatchUpdate(ctx context.Context, orgID string, params domain.BatchUpdateParams, actorID string) ([]*domain.Task, error)
	Duplicate(ctx context.Context, orgID, taskID, projectID string, includeSubtasks bool, actorID string) (*domain.Task, error)
	MoveToProject(ctx context.Context, orgID, taskID, fromProjectID, toProjectID, toStatusID string, actorID string) (*domain.Task, error)
	Delete(ctx context.Context, orgID, id, projectID string, mode domain.DeleteSubtaskMode, actorID string) error
	Move(ctx context.Context, actorID, orgID, id, projectID, statusID, positionKey string) error
	Reorder(ctx context.Context, orgID, projectID string, ops []domain.ReorderOp) error
	ListActivity(ctx context.Context, orgID, projectID, taskID string, filter domain.TaskActivityFilter) (*domain.TaskActivityResult, error)
}

type TaskDependencyService interface {
	Add(ctx context.Context, userID, orgID, taskID, blocksTaskID string) error
	Remove(ctx context.Context, userID, orgID, taskID, blocksTaskID string) error
	ListBlocking(ctx context.Context, userID, orgID, taskID string) ([]*domain.Task, error)
	ListBlocked(ctx context.Context, userID, orgID, taskID string) ([]*domain.Task, error)
}

type AuditService interface {
	Record(ctx context.Context, orgID, actorID string, action domain.AuditAction, entityType, entityID string, metadata any)
	List(ctx context.Context, orgID string, limit, offset int, action, actorID *string) ([]*domain.AuditEntry, int, error)
}

// BackupService provides database backup (download) and restore (validate +
// stage + restart-swap) for self-hosted operators. Backup uses VACUUM INTO
// for an atomic snapshot without closing the live DB. Restore stages a
// validated file to <DBPath>.restore-pending; the swap happens at startup
// before any connection is opened.
type BackupService interface {
	// DownloadBackup creates a VACUUM INTO snapshot and returns a ReadCloser
	// over the snapshot file plus a suggested filename. The caller must close
	// the reader (which removes the temp file).
	DownloadBackup(ctx context.Context) (io.ReadCloser, string, error)
	// StageRestore reads an uploaded backup, validates it is a SQLite DB with
	// the expected schema, and writes it to the staging path. Returns the
	// staging path. The server must be restarted to apply the restore.
	StageRestore(ctx context.Context, reader io.Reader) (string, error)
	// HasPendingRestore reports whether a staged restore file exists on disk.
	HasPendingRestore() bool
	// PendingRestoreInfo returns the staging path and file size when a pending
	// restore exists (ok=true), or ok=false otherwise.
	PendingRestoreInfo() (path string, size int64, ok bool)
	// ClearPendingRestore removes a staged restore file (admin cancelled).
	ClearPendingRestore() error
}

type CycleService interface {
	List(ctx context.Context, projectID string) ([]*domain.Cycle, error)
	GetByID(ctx context.Context, id, projectID string) (*domain.Cycle, error)
	GetActive(ctx context.Context, projectID string) (*domain.Cycle, error)
	Create(ctx context.Context, params domain.CreateCycleParams) (*domain.Cycle, error)
	Update(ctx context.Context, userID, orgID string, c *domain.Cycle) error
	Delete(ctx context.Context, userID, orgID, id, projectID string) error
	Activate(ctx context.Context, userID, orgID, id, projectID string) (*domain.Cycle, error)
	Complete(ctx context.Context, userID, orgID, id, projectID string, moveToCycleID string) (*domain.Cycle, error)
}

type TimeEntryService interface {
	List(ctx context.Context, orgID, taskID, projectID string) ([]*domain.TimeEntry, error)
	Start(ctx context.Context, orgID, taskID, projectID, userID, description string) ([]*domain.TimeEntry, error)
	Stop(ctx context.Context, orgID, taskID, projectID, userID string) ([]*domain.TimeEntry, error)
	Create(ctx context.Context, params domain.CreateTimeEntryParams) ([]*domain.TimeEntry, error)
	Update(ctx context.Context, callerUserID string, callerRole domain.Role, params domain.UpdateTimeEntryParams) ([]*domain.TimeEntry, error)
	Delete(ctx context.Context, callerUserID string, callerRole domain.Role, orgID, id, taskID, projectID string) error
}

type AttachmentService interface {
	List(ctx context.Context, orgID, taskID, projectID string) ([]*domain.Attachment, error)
	Create(ctx context.Context, params domain.CreateAttachmentParams) (*domain.Attachment, error)
	Delete(ctx context.Context, userID, orgID, id, taskID, projectID string) error
	Get(ctx context.Context, id string) (*domain.Attachment, error)
	Download(ctx context.Context, orgID, id string) (io.ReadCloser, string, string, string, error)
}

type UserService interface {
	ListUsers(ctx context.Context, orgID string, filter domain.UserFilter) (*domain.UserListResult, error)
	GetByID(ctx context.Context, orgID, id string) (*domain.User, error)
	UpdateRole(ctx context.Context, orgID, id string, role domain.Role, callerRole domain.Role, callerID string) error
	UpdateActive(ctx context.Context, orgID, id string, active bool, callerID string) error
	UpdateProfile(ctx context.Context, orgID, userID, name string, avatarURL *string) (*domain.User, error)
	UploadAvatar(ctx context.Context, orgID, userID string, file io.Reader, filename, contentType string, size int64) (*domain.User, error)
	ChangePassword(ctx context.Context, orgID, userID, currentPassword, newPassword string) error
}

type UserPreferencesService interface {
	Get(ctx context.Context, userID string) (*domain.UserPreferences, error)
	Update(ctx context.Context, userID string, params domain.UpdateUserPreferencesParams) (*domain.UserPreferences, error)
}

type UserInviteService interface {
	Create(ctx context.Context, params domain.CreateInviteParams, callerRole domain.Role) (*domain.UserInvite, string, error)
	List(ctx context.Context, orgID string, limit int) ([]*domain.UserInvite, error)
	Revoke(ctx context.Context, orgID, id string) error
	Validate(ctx context.Context, token string) (*domain.UserInvite, error)
	Accept(ctx context.Context, params domain.AcceptInviteParams) (*domain.User, string, error)
}

type Broadcaster interface {
	Broadcast(roomKey string, eventType string, payload any) error
}

// Mailer sends transactional email. When SMTP is not configured the
// implementation is a no-op: Enabled() returns false and Send is a silent
// no-op, so callers can unconditionally invoke Send without branching.
type Mailer interface {
	// Enabled reports whether outbound email is configured. Callers should
	// check this before doing work that only makes sense with email (e.g.
	// building a reset link to email rather than log).
	Enabled() bool
	// Send delivers an email. It must be safe to call when Enabled() is
	// false (no-op). Errors are logged by the implementation and not
	// returned to the caller, so a mailer failure never blocks the action
	// that triggered it (best-effort, like audit logging).
	Send(ctx context.Context, to, subject, htmlBody, textBody string) error
}

// PushService sends browser push notifications to a user's registered Web
// Push subscriptions. When VAPID is not configured it is a no-op.
type PushService interface {
	// Enabled reports whether Web Push (VAPID) is configured.
	Enabled() bool
	// PublicKey returns the VAPID public key (base64url) for the browser to
	// pass to pushManager.subscribe, or "" when disabled.
	PublicKey() string
	// Subscribe registers a push subscription endpoint for a user.
	Subscribe(ctx context.Context, userID, orgID, endpoint, p256dh, auth string) error
	// Unsubscribe removes a push subscription by endpoint.
	Unsubscribe(ctx context.Context, userID, endpoint string) error
	// Send delivers a push notification to all of a user's subscriptions.
	// Best-effort: stale subscriptions (410 Gone) are pruned silently.
	Send(ctx context.Context, userID string, payload domain.PushPayload) error
}

type ProjectMemberService interface {
	List(ctx context.Context, orgID, projectID string, filter domain.UserFilter) (*domain.ProjectMemberListResult, error)
	Add(ctx context.Context, orgID, projectID, userID string, role domain.Role) error
	Remove(ctx context.Context, orgID, projectID, userID string) error
	UpdateRole(ctx context.Context, orgID, projectID, userID string, role domain.Role) error
	ListByUser(ctx context.Context, orgID, userID string) ([]*domain.UserProjectMembership, error)
	SetMemberships(ctx context.Context, orgID, userID string, assignments []domain.ProjectAssignment) error
}

type NotificationService interface {
	List(ctx context.Context, orgID, userID string, filter domain.NotificationFilter) (*domain.NotificationListResult, error)
	CountUnread(ctx context.Context, userID string) (int, error)
	MarkRead(ctx context.Context, id, userID string) error
	MarkAllRead(ctx context.Context, userID string) error
	GetPreferences(ctx context.Context, userID string) ([]*domain.NotificationPreference, error)
	SetPreference(ctx context.Context, userID string, notifType domain.NotificationType, enabled bool) error
	Notify(ctx context.Context, orgID, recipientID string, notifType domain.NotificationType, title, body, link, entityType, entityID, actorID string) error
	ProcessDueNotifications(ctx context.Context) error
}

type ConversationService interface {
	ListMyConversations(ctx context.Context, orgID, userID string, filter domain.ConversationFilter) (*domain.ConversationListResult, error)
	ListByParent(ctx context.Context, orgID, parentID, userID string, role domain.Role, includeProjectLinked bool) ([]*domain.Conversation, error)
	GetByID(ctx context.Context, orgID, id, userID string) (*domain.Conversation, error)
	CreateChannel(ctx context.Context, params domain.CreateConversationParams) (*domain.Conversation, error)
	CreateDM(ctx context.Context, orgID, createdBy, targetUserID string) (*domain.Conversation, error)
	CreateGroupDM(ctx context.Context, orgID, createdBy string, memberIDs []string) (*domain.Conversation, error)
	UpdateConversation(ctx context.Context, conv *domain.Conversation) error
	UpdateChannelParent(ctx context.Context, orgID, id string, parentID *string, positionKey string) error
	DeleteConversation(ctx context.Context, orgID, id, userID string, callerRole domain.Role) error
	AddMembers(ctx context.Context, orgID, convID, adderID string, memberIDs []string) error
	RemoveMember(ctx context.Context, orgID, convID, removerID, targetID string) error
	GetMembers(ctx context.Context, convID string) ([]*domain.ConversationMember, error)
	MarkRead(ctx context.Context, convID, userID string) error
	SetMuted(ctx context.Context, orgID, convID, userID string, muted bool) error
	SetNotificationLevel(ctx context.Context, orgID, convID, userID string, level domain.NotificationLevel) error
	GetPinnedMessages(ctx context.Context, convID string, limit int) ([]*domain.Message, error)
	EnsureGeneralChannel(ctx context.Context, orgID, userID string) error
	GetChannelProjectLinks(ctx context.Context, channelID string) ([]string, error)
	SetChannelProjectLinks(ctx context.Context, channelID string, projectIDs []string) error
	ListAccess(ctx context.Context, orgID, channelID string) ([]*domain.ChannelAccessEntry, error)
}

type MessageService interface {
	ListMessages(ctx context.Context, orgID, convID string, filter domain.MessageFilter) (*domain.MessageListResult, error)
	ListReplies(ctx context.Context, orgID, convID, parentID string, filter domain.MessageFilter) (*domain.MessageListResult, error)
	SearchMessages(ctx context.Context, orgID, userID string, filter domain.MessageSearchFilter) (*domain.MessageSearchListResult, error)
	SendMessage(ctx context.Context, params domain.CreateMessageParams) (*domain.Message, error)
	EditMessage(ctx context.Context, params domain.EditMessageParams) (*domain.Message, error)
	DeleteMessage(ctx context.Context, orgID, msgID, convID, deleterID string) error
	PinMessage(ctx context.Context, orgID, msgID, convID, pinnerID string) error
	UnpinMessage(ctx context.Context, orgID, msgID, convID string) error
	AddReaction(ctx context.Context, params domain.AddReactionParams) error
	RemoveReaction(ctx context.Context, params domain.RemoveReactionParams) error
}

type MentionService interface {
	Search(ctx context.Context, orgID, userID, userRole, query string, types []domain.MentionType, limit int) ([]*domain.MentionResult, error)
}

type SearchService interface {
	Search(ctx context.Context, params domain.SearchParams) ([]*domain.SearchResult, error)
}

type DashboardService interface {
	GetDashboard(ctx context.Context, params domain.GetDashboardParams) (*domain.DashboardData, error)
	SetVisibility(ctx context.Context, params domain.SetVisibilityParams) (*domain.DashboardVisibility, error)
}

type PresenceService interface {
	SetStatus(ctx context.Context, orgID, userID string, status domain.PresenceStatus) error
	ListForOrg(ctx context.Context, orgID string) ([]*domain.UserPresence, error)
}

type ChannelPermissionService interface {
	ResolvePermissions(ctx context.Context, orgID, channelID, userID string, userRole domain.Role) (*domain.ChannelPermissions, error)
	ResolveRolePermissions(ctx context.Context, orgID, channelID string) ([]*domain.EffectivePermission, error)
	GetPermissions(ctx context.Context, channelID string) ([]*domain.PermissionRule, error)
	SetPermissions(ctx context.Context, channelID string, rules []*domain.PermissionRule) error
	GetUserOverrides(ctx context.Context, channelID string) ([]*domain.UserPermissionOverride, error)
	SetUserOverrides(ctx context.Context, channelID string, overrides []*domain.UserPermissionOverride) error
	UserHasAccess(ctx context.Context, orgID, channelID, userID string, userRole domain.Role) (bool, error)
	GetUsersWithProjectAccess(ctx context.Context, orgID, projectID string) ([]*domain.User, error)
	ListAccess(ctx context.Context, orgID, channelID string) ([]*domain.ChannelAccessEntry, error)
}

type ViewService interface {
	Create(ctx context.Context, params domain.CreateViewParams) (*domain.View, error)
	Update(ctx context.Context, userID string, params domain.UpdateViewParams) (*domain.View, error)
	Delete(ctx context.Context, userID, orgID, id string) error
	GetByID(ctx context.Context, orgID, id string) (*domain.View, error)
	ListByProject(ctx context.Context, orgID, projectID string) ([]*domain.View, error)
	ListGlobal(ctx context.Context, orgID string) ([]*domain.View, error)
	ListPinned(ctx context.Context, userID string) ([]*domain.View, error)
	Pin(ctx context.Context, orgID, viewID, userID string) error
	Unpin(ctx context.Context, orgID, viewID, userID string) error
}

type VoiceSFU interface {
	CreatePublisher(ctx context.Context, orgID, userID, connID, convID string) (sdp string, err error)
	CreateSubscriber(ctx context.Context, orgID, subscriberID, subscriberConnID, publisherID, convID string) (sdp string, err error)
	HandleAnswer(ctx context.Context, userID, convID, sdp string) error
	HandleSubscriberAnswer(ctx context.Context, subscriberID, publisherID, convID, sdp string) error
	HandleICECandidate(ctx context.Context, userID, convID, candidateJSON string) error
	HandleSubscriberICECandidate(ctx context.Context, userID, convID, publisherID, candidateJSON string) error
	RemoveParticipant(ctx context.Context, userID, convID string) error
	SetMuted(ctx context.Context, userID, convID string, muted bool) error
	ICEServers() []domain.ICEServer
}

type VoiceService interface {
	Join(ctx context.Context, orgID, userID string, callerRole domain.Role, connID, convID string) (*domain.VoiceJoinResult, error)
	Leave(ctx context.Context, orgID, userID, convID string) error
	LeaveByConnection(ctx context.Context, orgID, userID, connID string) error
	SetMute(ctx context.Context, orgID, userID, convID string, muted bool) error
	SetDeafen(ctx context.Context, orgID, userID, convID string, deafened bool) error
	Kick(ctx context.Context, orgID, callerUserID string, callerRole domain.Role, convID, targetUserID string) error
	ListParticipants(ctx context.Context, orgID, convID string) ([]domain.VoiceParticipantInfo, error)
	HandleSignal(ctx context.Context, orgID, userID, connID, convID string, msg domain.VoiceSignalMsg) error
}

type LabelService interface {
	List(ctx context.Context, orgID string) ([]*domain.Label, error)
	Create(ctx context.Context, userID, orgID, name, color string) (*domain.Label, error)
	Update(ctx context.Context, userID, orgID, id, name, color string) (*domain.Label, error)
	Delete(ctx context.Context, userID, orgID, id string) error
	SetTaskLabels(ctx context.Context, userID, orgID, taskID string, labelIDs []string) error
	GetTaskLabels(ctx context.Context, userID, orgID, taskID string) ([]*domain.Label, error)
	ListLabelsByTaskIDs(ctx context.Context, taskIDs []string) (map[string][]*domain.Label, error)
}

type CommentService interface {
	ListByTask(ctx context.Context, orgID, taskID, projectID, beforeCursor string, limit int) (*domain.CommentListResult, error)
	Create(ctx context.Context, orgID, taskID, authorID, content string, parentID *string) (*domain.Comment, error)
	Update(ctx context.Context, orgID, id, authorID, content string) (*domain.Comment, error)
	Delete(ctx context.Context, orgID, id, authorID string) error
}

type TaskTemplateService interface {
	List(ctx context.Context, orgID, projectID string) ([]*domain.TaskTemplate, error)
	Get(ctx context.Context, orgID, projectID, id string) (*domain.TaskTemplate, error)
	Create(ctx context.Context, p domain.CreateTaskTemplateParams) (*domain.TaskTemplate, error)
	Update(ctx context.Context, p domain.UpdateTaskTemplateParams) (*domain.TaskTemplate, error)
	Delete(ctx context.Context, orgID, projectID, id string) error
	Instantiate(ctx context.Context, orgID, projectID, id, createdBy string) (*domain.Task, error)
	ProcessDueRecurring(ctx context.Context) error
}

type CustomFieldService interface {
	List(ctx context.Context, orgID, projectID string) ([]*domain.CustomField, error)
	Create(ctx context.Context, userID string, p domain.CreateCustomFieldParams) (*domain.CustomField, error)
	Update(ctx context.Context, userID string, p domain.UpdateCustomFieldParams) (*domain.CustomField, error)
	Delete(ctx context.Context, userID, orgID, projectID, id string) error
	GetTaskValues(ctx context.Context, userID, orgID, taskID string) (map[string]string, error)
	SetTaskValue(ctx context.Context, userID, orgID, taskID, fieldID, value string) error
}
