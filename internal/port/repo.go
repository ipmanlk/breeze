package port

import (
	"context"
	"time"

	"ipmanlk/breeze/internal/domain"
)

type UserRepository interface {
	GetByID(ctx context.Context, orgID, id string) (*domain.User, error)
	GetByEmail(ctx context.Context, orgID, email string) (*domain.User, error)
	// RunInTransaction runs fn against a transaction-scoped UserRepository.
	RunInTransaction(ctx context.Context, fn func(UserRepository) error) error
	ListUsers(ctx context.Context, orgID string, filter domain.UserFilter) (*domain.UserListResult, error)
	ListByIDs(ctx context.Context, ids []string) ([]*domain.User, error)
	ListByAccount(ctx context.Context, accountID string) ([]*domain.User, error)
	GetByOrgAndAccount(ctx context.Context, orgID, accountID string) (*domain.User, error)
	Create(ctx context.Context, user *domain.User) error
	Update(ctx context.Context, user *domain.User) error
	UpdateProfileByAccount(ctx context.Context, accountID, name string, avatarURL *string) error
	UpdateRole(ctx context.Context, orgID, id string, role domain.Role) error
	UpdateActive(ctx context.Context, orgID, id string, active bool) error
	CountOwners(ctx context.Context, orgID string) (int, error)
}

type AccountRepository interface {
	GetByEmail(ctx context.Context, email string) (*domain.Account, error)
	GetByID(ctx context.Context, id string) (*domain.Account, error)
	Create(ctx context.Context, account *domain.Account) error
	UpdatePassword(ctx context.Context, id, passwordHash string) error
	Exists(ctx context.Context) (bool, error)
}

type UserInviteRepository interface {
	Create(ctx context.Context, invite *domain.UserInvite) error
	GetByTokenHash(ctx context.Context, hash string) (*domain.UserInvite, error)
	ListByOrg(ctx context.Context, orgID string, limit int) ([]*domain.UserInvite, error)
	Delete(ctx context.Context, orgID, id string) error
	IncrementUseCount(ctx context.Context, id string) error
	RecordAcceptance(ctx context.Context, inviteID, userID string) error
	AddInviteProject(ctx context.Context, inviteID, projectID string, role domain.Role) error
	ListInviteProjects(ctx context.Context, inviteID string) ([]*domain.InviteProjectAssignment, error)
	DeleteInviteProjects(ctx context.Context, inviteID string) error
	AcceptInvite(ctx context.Context, inviteID, userID string) error
}

type OrganizationRepository interface {
	GetByID(ctx context.Context, id string) (*domain.Organization, error)
	GetBySlug(ctx context.Context, slug string) (*domain.Organization, error)
	Exists(ctx context.Context) (bool, error)
	Count(ctx context.Context) (int64, error)
	Create(ctx context.Context, org *domain.Organization) error
	Update(ctx context.Context, org *domain.Organization) error
	Delete(ctx context.Context, id string) error
	ListForAccount(ctx context.Context, accountID string) ([]*domain.Workspace, error)
	// CreateOrgWithAccountAndUser atomically creates an org, account, and user in a single transaction.
	CreateOrgWithAccountAndUser(ctx context.Context, org *domain.Organization, accountID, userID, passwordHash, adminEmail, adminName string) error
	// CreateOrgWithUser atomically creates an org and user (for an existing account) in a single transaction.
	CreateOrgWithUser(ctx context.Context, org *domain.Organization, userID, accountID, displayName, email string, avatarURL *string) error
}

type SessionRepository interface {
	Create(ctx context.Context, session *domain.Session) error
	GetByID(ctx context.Context, id string) (*domain.Session, error)
	ListByUser(ctx context.Context, userID string) ([]*domain.Session, error)
	Revoke(ctx context.Context, id string) error
	Delete(ctx context.Context, id string) error
	DeleteByUser(ctx context.Context, userID string) error
	DeleteExpired(ctx context.Context) error
}

type PushSubscriptionRepository interface {
	Upsert(ctx context.Context, sub *domain.PushSubscription) (*domain.PushSubscription, error)
	ListByUser(ctx context.Context, userID string) ([]*domain.PushSubscription, error)
	Delete(ctx context.Context, userID, endpoint string) (int64, error)
	DeleteByUser(ctx context.Context, userID string) (int64, error)
}

type ProjectRepository interface {
	List(ctx context.Context, orgID string) ([]*domain.Project, error)
	ListIncludingArchived(ctx context.Context, orgID string) ([]*domain.Project, error)
	ListForUser(ctx context.Context, orgID, userID string) ([]*domain.Project, error)
	ListForUserIncludingArchived(ctx context.Context, orgID, userID string) ([]*domain.Project, error)
	ListByIDs(ctx context.Context, orgID string, ids []string) ([]*domain.Project, error)
	GetByID(ctx context.Context, orgID, id string) (*domain.Project, error)
	GetBySlug(ctx context.Context, orgID, slug string) (*domain.Project, error)
	Create(ctx context.Context, p *domain.Project) error
	CreateWithStatuses(ctx context.Context, project *domain.Project, statuses []*domain.TaskStatus) error
	Update(ctx context.Context, p *domain.Project) error
	SetArchived(ctx context.Context, orgID, id string, archived bool) error
	Delete(ctx context.Context, orgID, id string) error
}

type TaskStatusRepository interface {
	ListByProject(ctx context.Context, projectID string) ([]*domain.TaskStatus, error)
	GetByID(ctx context.Context, id string) (*domain.TaskStatus, error)
	Create(ctx context.Context, s *domain.TaskStatus) error
	Update(ctx context.Context, s *domain.TaskStatus) error
	Delete(ctx context.Context, id, projectID string) error
	CountTasksByStatus(ctx context.Context, statusID, projectID string) (int64, error)
	ReassignTasks(ctx context.Context, toStatusID, fromStatusID, projectID string) error
}

type TaskDependencyRepository interface {
	Add(ctx context.Context, taskID, blocksTaskID string) error
	Remove(ctx context.Context, taskID, blocksTaskID string) error
	ListBlocking(ctx context.Context, taskID string) ([]*domain.Task, error)
	ListBlocked(ctx context.Context, taskID string) ([]*domain.Task, error)
}

type AuditRepository interface {
	Create(ctx context.Context, entry *domain.AuditEntry) error
	List(ctx context.Context, orgID string, limit, offset int, action, actorID *string) ([]*domain.AuditEntry, error)
	Count(ctx context.Context, orgID string, action, actorID *string) (int, error)
	// DeleteOlderThan removes audit entries older than the cutoff (all orgs).
	// Used by the periodic retention cleanup.
	DeleteOlderThan(ctx context.Context, cutoff time.Time) (int64, error)
}

type TaskActivityRepository interface {
	Create(ctx context.Context, entry *domain.TaskActivity) error
	List(ctx context.Context, taskID string, filter domain.TaskActivityFilter) (*domain.TaskActivityResult, error)
}

type TaskRepository interface {
	ListByProject(ctx context.Context, orgID, projectID string, filter domain.TaskFilter) ([]*domain.Task, error)
	ListByUser(ctx context.Context, orgID, userID string, filter domain.TaskListFilter) (*domain.TaskListResult, error)
	ListByIDs(ctx context.Context, orgID string, ids []string) ([]*domain.Task, error)
	GetByID(ctx context.Context, orgID, id, projectID string) (*domain.Task, error)
	GetByIDAndOrg(ctx context.Context, orgID, id string) (*domain.Task, error)
	ListSubtasks(ctx context.Context, orgID, parentID string) ([]*domain.Task, error)
	Create(ctx context.Context, task *domain.Task) error
	Update(ctx context.Context, task *domain.Task) error
	Move(ctx context.Context, orgID, id, projectID, statusID, positionKey string) error
	MoveToProject(ctx context.Context, orgID, id, fromProjectID, toProjectID, toStatusID, positionKey string, completedAt *time.Time) error
	Delete(ctx context.Context, orgID, id, projectID string) error
	DeleteSubtasks(ctx context.Context, orgID, parentID string) error
	PromoteSubtasks(ctx context.Context, orgID, parentID string) error
	CountSubtasks(ctx context.Context, orgID, parentID string) (int64, error)
	GetLastSubtaskPosition(ctx context.Context, orgID, parentID string) (string, error)
	// GenerateSubtaskPositionKey atomically reads the last subtask_position and
	// generates the next one (appends at end). Transaction-wrapped to prevent
	// the lexorank race on concurrent subtask creation.
	GenerateSubtaskPositionKey(ctx context.Context, orgID, parentID string) (string, error)
	ReorderSubtasks(ctx context.Context, orgID, parentID string, ops []domain.ReorderOp) error
	SetAssignees(ctx context.Context, taskID string, userIDs []string) error
	ListAssigneesByTaskIDs(ctx context.Context, taskIDs []string) (map[string][]domain.TaskAssignee, error)
	GetLastPositionKey(ctx context.Context, orgID, projectID, statusID string) (string, error)
	// GeneratePositionKey atomically reads the last position key for the given
	// status and generates the next one (appends at end). Uses a transaction to
	// prevent the lexorank race where concurrent reads produce duplicate keys.
	GeneratePositionKey(ctx context.Context, orgID, projectID, statusID string) (string, error)
	GetPositionKeyNeighbors(ctx context.Context, orgID, taskID string) (prev, next string, err error)
	UnassignCycleFromTasks(ctx context.Context, orgID, cycleID string) error
	MoveTasksToCycle(ctx context.Context, orgID, fromCycleID, toCycleID string) error
	MoveIncompleteTasksToCycle(ctx context.Context, orgID, fromCycleID, toCycleID string) error
	UnassignCycleFromIncompleteTasks(ctx context.Context, orgID, cycleID string) error
	ListByCycle(ctx context.Context, orgID, cycleID string) ([]*domain.Task, error)
	// RunInTransaction runs fn against a transaction-scoped TaskRepository.
	// If fn returns nil the transaction commits; otherwise it rolls back.
	// Used by BatchUpdate to make multi-task mutations atomic. A nil
	// receiver falls back to running fn against the non-transactional repo
	// so callers degrade gracefully rather than panic.
	RunInTransaction(ctx context.Context, fn func(TaskRepository) error) error
	// ListByIDsFull is the full-projection batch fetch (org-scoped). Used by
	// BatchUpdate which needs complete task rows to mutate safely. ListByIDs
	// (minimal) is for mention hydration.
	ListByIDsFull(ctx context.Context, orgID string, ids []string) ([]*domain.Task, error)
}

type ProjectMemberRepository interface {
	List(ctx context.Context, orgID, projectID string, filter domain.UserFilter) (*domain.ProjectMemberListResult, error)
	Get(ctx context.Context, orgID, projectID, userID string) (*domain.ProjectMember, error)
	Add(ctx context.Context, projectID, userID string, role domain.Role) error
	Remove(ctx context.Context, projectID, userID string) error
	UpdateRole(ctx context.Context, projectID, userID string, role domain.Role) error
	ListByUser(ctx context.Context, orgID, userID string) ([]*domain.UserProjectMembership, error)
	// SetMemberships atomically replaces all project memberships for a user
	// with the given assignments. This is the transactional version of
	// (ListByUser + diff + Add/Remove/UpdateRole loop) and avoids race
	// conditions under concurrent access.
	SetMemberships(ctx context.Context, orgID, userID string, assignments []domain.ProjectAssignment) error
}

type CycleRepository interface {
	ListByProject(ctx context.Context, projectID string) ([]*domain.Cycle, error)
	GetByID(ctx context.Context, id, projectID string) (*domain.Cycle, error)
	GetActiveByProject(ctx context.Context, projectID string) (*domain.Cycle, error)
	Create(ctx context.Context, c *domain.Cycle) error
	Update(ctx context.Context, c *domain.Cycle) error
	Delete(ctx context.Context, id, projectID string) error
	DeactivateAll(ctx context.Context, projectID string) error
	SetActive(ctx context.Context, id, projectID string) error
	// ActivateCycle deactivates all cycles and activates the given one in a
	// single transaction.
	ActivateCycle(ctx context.Context, id, projectID string) error
	CountTasksByCycle(ctx context.Context, id string) (total, completed int64, err error)
	CountTasksByCycles(ctx context.Context, projectID string) (map[string]domain.CycleTaskCount, error)
	// CompleteCycle executes all DB mutations for cycle completion in a single
	// transaction. The service computes the plan; the store runs it atomically.
	CompleteCycle(ctx context.Context, plan domain.CycleCompletionPlan) error
}

type AttachmentRepository interface {
	ListByTask(ctx context.Context, taskID string) ([]*domain.Attachment, error)
	GetByID(ctx context.Context, id string) (*domain.Attachment, error)
	Create(ctx context.Context, a *domain.Attachment) error
	Delete(ctx context.Context, id, taskID string) error
}

type TimeEntryRepository interface {
	ListByTask(ctx context.Context, taskID string) ([]*domain.TimeEntry, error)
	GetActiveTimer(ctx context.Context, taskID, userID string) (*domain.TimeEntry, error)
	GetActiveTimerByUser(ctx context.Context, userID string) ([]*domain.TimeEntry, error)
	StartTimer(ctx context.Context, id, taskID, userID, description string) error
	StopTimer(ctx context.Context, id, userID string) error
	StartTimerAtomic(ctx context.Context, id, taskID, userID, description string) error
	Create(ctx context.Context, entry *domain.TimeEntry) error
	Update(ctx context.Context, entry *domain.TimeEntry) error
	Delete(ctx context.Context, id, taskID string) error
	TotalTimeByTask(ctx context.Context, taskID string) (int64, error)
}

type NotificationRepository interface {
	Create(ctx context.Context, n *domain.Notification) error
	List(ctx context.Context, orgID, userID string, filter domain.NotificationFilter) (*domain.NotificationListResult, error)
	GetByID(ctx context.Context, id, userID string) (*domain.Notification, error)
	MarkRead(ctx context.Context, id, userID string) error
	MarkAllRead(ctx context.Context, userID string) error
	CountUnread(ctx context.Context, userID string) (int64, error)
}

type NotificationPreferenceRepository interface {
	List(ctx context.Context, userID string) ([]*domain.NotificationPreference, error)
	GetByType(ctx context.Context, userID string, notifType domain.NotificationType) (*domain.NotificationPreference, error)
	Set(ctx context.Context, userID, notifType string, enabled bool) error
	FindDueNotifications(ctx context.Context, nowMinus1h, now, nowPlus24h time.Time, dueSoonType, overdueType string) ([]domain.DueTaskRow, error)
}

type ConversationRepository interface {
	ListByUser(ctx context.Context, orgID, userID string, filter domain.ConversationFilter) (*domain.ConversationListResult, error)
	ListByParent(ctx context.Context, orgID, parentID, userID string, includeProjectLinked bool) ([]*domain.Conversation, error)
	GetByID(ctx context.Context, orgID, id string) (*domain.Conversation, error)
	GetByIDWithMember(ctx context.Context, orgID, id, userID string) (*domain.Conversation, error)
	GetDMByUsers(ctx context.Context, orgID, requesterID, recipientID string) (*domain.Conversation, error)
	ListByIDs(ctx context.Context, orgID string, ids []string) ([]*domain.Conversation, error)
	Create(ctx context.Context, conv *domain.Conversation) error
	CreateWithMembers(ctx context.Context, conv *domain.Conversation, memberIDs []string) error
	Update(ctx context.Context, conv *domain.Conversation) error
	UpdateParent(ctx context.Context, orgID, id string, parentID *string, positionKey string) error
	UpdatePositionKey(ctx context.Context, orgID, id string, positionKey string) error
	ListCategories(ctx context.Context, orgID string) ([]*domain.Conversation, error)
	ListSiblingPositionKeys(ctx context.Context, orgID string, parentID *string) ([]string, error)
	Delete(ctx context.Context, orgID, id string) error
	SoftDeleteByParent(ctx context.Context, orgID, parentID string) error
	AddMember(ctx context.Context, orgID, convID, userID string) error
	RemoveMember(ctx context.Context, convID, userID string) error
	GetMembers(ctx context.Context, convID string) ([]*domain.ConversationMember, error)
	IsMember(ctx context.Context, convID, userID string) (bool, error)
	UpdateReadState(ctx context.Context, convID, userID string) error
	UnreadCount(ctx context.Context, convID, userID string) (int, error)
	UnreadCounts(ctx context.Context, userID string, convIDs []string) (map[string]int, error)
	GetLastMessage(ctx context.Context, convID string) (*domain.Message, error)
	ListPinnedMessages(ctx context.Context, convID string, limit int) ([]*domain.Message, error)
	CountMembers(ctx context.Context, convID string) (int, error)
}

type ChannelProjectLinkRepository interface {
	Create(ctx context.Context, channelID, projectID string) error
	Delete(ctx context.Context, channelID, projectID string) error
	DeleteByChannel(ctx context.Context, channelID string) error
	GetByChannel(ctx context.Context, channelID string) ([]string, error)
	GetByProject(ctx context.Context, projectID string) ([]string, error)
	SetProjectLinks(ctx context.Context, channelID string, projectIDs []string) error
	GetUsersWithProjectAccess(ctx context.Context, orgID, projectID string) ([]*domain.User, error)
}

type ChannelPermissionRepository interface {
	GetPermissions(ctx context.Context, channelID string) ([]*domain.PermissionRule, error)
	SetPermissions(ctx context.Context, channelID string, rules []*domain.PermissionRule) error
	GetUserOverrides(ctx context.Context, channelID string) ([]*domain.UserPermissionOverride, error)
	SetUserOverrides(ctx context.Context, channelID string, overrides []*domain.UserPermissionOverride) error
}

type MessageRepository interface {
	ListByConversation(ctx context.Context, orgID, convID string, filter domain.MessageFilter) (*domain.MessageListResult, error)
	ListReplies(ctx context.Context, orgID, convID, parentID string, filter domain.MessageFilter) (*domain.MessageListResult, error)
	SearchMessages(ctx context.Context, orgID, userID string, filter domain.MessageSearchFilter) (*domain.MessageSearchListResult, error)
	GetByID(ctx context.Context, id, convID string) (*domain.Message, error)
	GetByIDAnyConv(ctx context.Context, id string) (*domain.Message, error)
	Create(ctx context.Context, msg *domain.Message) error
	Update(ctx context.Context, msg *domain.Message) error
	SoftDelete(ctx context.Context, id, convID string) error
	Pin(ctx context.Context, id, convID, pinnedBy string) error
	Unpin(ctx context.Context, id, convID string) error
	GetConversationLastMessage(ctx context.Context, convID string) (*domain.Message, error)
	GetLastMessagesForConversations(ctx context.Context, convIDs []string) (map[string]*domain.Message, error)
	Count(ctx context.Context, convID string) (int, error)
}

type MessageAttachmentRepository interface {
	Create(ctx context.Context, att *domain.MessageAttachment) error
	ListByMessage(ctx context.Context, messageID string) ([]*domain.MessageAttachment, error)
	ListByMessages(ctx context.Context, messageIDs []string) (map[string][]*domain.MessageAttachment, error)
	GetByID(ctx context.Context, id string) (*domain.MessageAttachment, error)
	GetByIDAndConversation(ctx context.Context, id, conversationID string) (*domain.MessageAttachment, error)
	Delete(ctx context.Context, id string) error
	UpdateMessageID(ctx context.Context, id, messageID string) error
}

type ReactionRepository interface {
	Add(ctx context.Context, orgID, messageID, userID, emoji string) error
	// Remove deletes one reaction and reports whether a row was actually
	// removed (false = nothing existed; idempotent no-op).
	Remove(ctx context.Context, messageID, userID, emoji string) (bool, error)
	ListForMessages(ctx context.Context, messageIDs []string) ([]*domain.Reaction, error)
}

type PendingAttachmentRepository interface {
	Create(ctx context.Context, att *domain.PendingAttachment) error
	// GetByID returns the pending upload only when it was created by the
	// given user; otherwise it behaves as not-found.
	GetByID(ctx context.Context, id, uploadedBy string) (*domain.PendingAttachment, error)
	Delete(ctx context.Context, id string) error
	DeleteOlderThan(ctx context.Context, before time.Time) ([]*domain.PendingAttachment, error)
}

type PresenceRepository interface {
	Upsert(ctx context.Context, orgID, userID string, status domain.PresenceStatus) error
	// Get returns the user's presence row scoped to one organization.
	// Presence is per-membership (composite PK user_id+org_id), so callers
	// must pass the org they are asking about.
	Get(ctx context.Context, orgID, userID string) (*domain.UserPresence, error)
	ListForOrg(ctx context.Context, orgID string) ([]*domain.UserPresence, error)
}

type UserChannelPreferenceRepository interface {
	Upsert(ctx context.Context, pref *domain.UserChannelPreference) error
	SetMuted(ctx context.Context, orgID, userID, convID string, muted bool) error
	SetNotificationLevel(ctx context.Context, orgID, userID, convID string, level domain.NotificationLevel) error
	Get(ctx context.Context, userID, convID string) (*domain.UserChannelPreference, error)
	UpdateLastRead(ctx context.Context, userID, convID string) error
}

type UserPreferencesRepository interface {
	Get(ctx context.Context, userID string) (*domain.UserPreferences, error)
	Upsert(ctx context.Context, prefs *domain.UserPreferences) error
}

type SearchRepository interface {
	SearchProjects(ctx context.Context, orgID, query, projectID string, limit int) ([]*domain.SearchResult, error)
	SearchProjectsForUser(ctx context.Context, orgID, userID, query string, limit int) ([]*domain.SearchResult, error)
	SearchTasks(ctx context.Context, orgID, query, projectID string, limit int) ([]*domain.SearchResult, error)
	SearchTasksForUser(ctx context.Context, orgID, userID, query, projectID string, limit int) ([]*domain.SearchResult, error)
	SearchChannels(ctx context.Context, orgID, userID string, includeProjectLinked bool, query string, limit int) ([]*domain.SearchResult, error)
	SearchDirectMessages(ctx context.Context, orgID, userID, query string, limit int) ([]*domain.SearchResult, error)
	SearchMembers(ctx context.Context, orgID, query string, limit int) ([]*domain.SearchResult, error)
	RecentProjects(ctx context.Context, orgID string, limit int) ([]*domain.SearchResult, error)
	RecentProjectsForUser(ctx context.Context, orgID, userID string, limit int) ([]*domain.SearchResult, error)
	RecentTasks(ctx context.Context, orgID string, limit int) ([]*domain.SearchResult, error)
	RecentTasksForUser(ctx context.Context, orgID, userID string, limit int) ([]*domain.SearchResult, error)
}

type DashboardRepository interface {
	MyTasks(ctx context.Context, orgID, userID string, limit int) ([]*domain.DashboardTask, error)
	DueSoonTasks(ctx context.Context, orgID, userID string, limit int) ([]*domain.DashboardTask, error)
	MyTaskStats(ctx context.Context, orgID, userID string) (*domain.DashboardStats, error)
	RecentActivity(ctx context.Context, orgID, userID string, limit int) ([]*domain.DashboardActivity, error)
	OrgProjects(ctx context.Context, orgID string) ([]*domain.DashboardProject, error)
	OrgProjectsForUser(ctx context.Context, orgID, userID string) ([]*domain.DashboardProject, error)
	GetPreferences(ctx context.Context, orgID, userID string) ([]domain.SectionType, error)
	SetPreferences(ctx context.Context, orgID, userID string, sections []domain.SectionType) error
}

type VoiceParticipantRepository interface {
	ListByConversation(ctx context.Context, orgID, convID string) ([]*domain.VoiceParticipant, error)
	ListByConversationWithUser(ctx context.Context, orgID, convID string) ([]domain.VoiceParticipantInfo, error)
	Join(ctx context.Context, p *domain.VoiceParticipant) error
	Leave(ctx context.Context, orgID, convID, userID string) error
	UpdateFlags(ctx context.Context, orgID, convID, userID string, muted, deafened bool) error
	UpdateConnection(ctx context.Context, orgID, convID, userID, connectionID string) error
	Count(ctx context.Context, orgID, convID string) (int, error)
	Get(ctx context.Context, orgID, convID, userID string) (*domain.VoiceParticipant, error)
	ListActiveVoiceForUser(ctx context.Context, orgID, userID string) ([]*domain.VoiceParticipant, error)
	// DeleteAll removes every row (crash-recovery startup sweep) and returns
	// the number removed.
	DeleteAll(ctx context.Context) (int64, error)
}

type ViewRepository interface {
	Create(ctx context.Context, v *domain.View) error
	Update(ctx context.Context, v *domain.View) error
	Delete(ctx context.Context, orgID, id string) error
	GetByID(ctx context.Context, orgID, id string) (*domain.View, error)
	ListByProject(ctx context.Context, orgID, projectID string) ([]*domain.View, error)
	ListGlobal(ctx context.Context, orgID string) ([]*domain.View, error)
	ListPinned(ctx context.Context, userID string) ([]*domain.View, error)
	Pin(ctx context.Context, viewID, userID string) error
	Unpin(ctx context.Context, viewID, userID string) error
}

type PasswordResetRepository interface {
	Create(ctx context.Context, reset *domain.PasswordReset) error
	GetByTokenHash(ctx context.Context, hash string) (*domain.PasswordReset, error)
	// MarkUsed atomically marks the token used. Returns false if the token
	// was already consumed by a concurrent request.
	MarkUsed(ctx context.Context, id string) (bool, error)
	// DeleteExpired removes used and expired tokens.
	DeleteExpired(ctx context.Context) error
}
