package service

import (
	"context"
	"fmt"
	"log/slog"
	"time"

	"ipmanlk/breeze/internal/apperr"
	"ipmanlk/breeze/internal/domain"
	"ipmanlk/breeze/internal/port"

	"github.com/google/uuid"
)

const maxCommentLength = 10000

type CommentService struct {
	commentRepo  port.CommentRepository
	taskRepo     port.TaskRepository
	projectRepo  port.ProjectRepository
	convRepo     port.ConversationRepository
	userRepo     port.UserRepository
	notifSvc     port.NotificationService
	broadcaster  port.Broadcaster
	log          *slog.Logger
	mentions     mentionHydrator
	access       port.AccessChecker
	activityRepo port.TaskActivityRepository
}

var _ port.CommentService = (*CommentService)(nil)

func NewCommentService(
	commentRepo port.CommentRepository,
	taskRepo port.TaskRepository,
	projectRepo port.ProjectRepository,
	convRepo port.ConversationRepository,
	userRepo port.UserRepository,
	notifSvc port.NotificationService,
	broadcaster port.Broadcaster,
	log *slog.Logger,
	access port.AccessChecker,
	activityRepo port.TaskActivityRepository,
) *CommentService {
	return &CommentService{
		commentRepo:  commentRepo,
		taskRepo:     taskRepo,
		projectRepo:  projectRepo,
		convRepo:     convRepo,
		userRepo:     userRepo,
		notifSvc:     notifSvc,
		broadcaster:  broadcaster,
		log:          log,
		mentions:     newMentionHydrator(userRepo, projectRepo, taskRepo, convRepo),
		access:       access,
		activityRepo: activityRepo,
	}
}

func (s *CommentService) ListByTask(ctx context.Context, orgID, taskID, projectID, beforeCursor string, limit int) (*domain.CommentListResult, error) {
	// Verify the task belongs to the caller's org + the URL's project before
	// returning its comments. This closes a latent IDOR where a caller with
	// access to project A could pass A's project ID + B's task ID to read an
	// unrelated task's comment thread (the SQL was also not org-scoped; now
	// fixed with c.org_id filter + this ownership check).
	if _, err := s.taskRepo.GetByID(ctx, orgID, taskID, projectID); err != nil {
		return nil, apperr.ErrNotFound
	}
	result, err := s.commentRepo.ListByTask(ctx, domain.CommentFilter{
		TaskID:       taskID,
		OrgID:        orgID,
		BeforeCursor: beforeCursor,
		Limit:        limit,
	})
	if err != nil {
		return nil, err
	}
	s.hydrateComments(ctx, orgID, result.Items)
	return result, nil
}

func (s *CommentService) Create(ctx context.Context, orgID, taskID, authorID, content string, parentID *string) (*domain.Comment, error) {
	if content == "" || len(content) > maxCommentLength {
		return nil, apperr.InvalidInput("comment must be between 1 and 10000 characters")
	}

	if s.access != nil {
		if err := s.access.RequireTaskAccess(ctx, authorID, orgID, taskID, domain.PermTaskCreate); err != nil {
			return nil, err
		}
	}

	task, err := s.taskRepo.GetByIDAndOrg(ctx, orgID, taskID)
	if err != nil {
		return nil, fmt.Errorf("task not found: %w", err)
	}

	// Validate parent belongs to the same task if this is a reply.
	if parentID != nil && *parentID != "" {
		parent, err := s.commentRepo.GetByID(ctx, orgID, *parentID)
		if err != nil {
			return nil, apperr.NotFound("parent comment", err)
		}
		if parent.TaskID != taskID {
			return nil, apperr.InvalidInput("parent comment does not belong to this task")
		}
	}

	comment := &domain.Comment{
		ID:        uuid.New().String(),
		OrgID:     orgID,
		TaskID:    taskID,
		ProjectID: task.ProjectID,
		AuthorID:  authorID,
		Content:   content,
		ParentID:  parentID,
	}
	if err := s.commentRepo.Create(ctx, comment); err != nil {
		return nil, err
	}

	comment, err = s.commentRepo.GetByID(ctx, orgID, comment.ID)
	if err != nil {
		return nil, err
	}
	s.hydrateComments(ctx, orgID, []*domain.Comment{comment})

	s.sendNotifications(ctx, comment, task)
	s.broadcastComment(ctx, string(domain.WsTypeCommentNew), comment)

	// Record activity best-effort.
	if s.activityRepo != nil {
		_ = s.activityRepo.Create(ctx, &domain.TaskActivity{
			ID:        uuid.New().String(),
			TaskID:    task.ID,
			OrgID:     comment.OrgID,
			ProjectID: comment.ProjectID,
			ActorID:   comment.AuthorID,
			Action:    domain.ActivityCommentAdded,
			Field:     "",
			OldValue:  "",
			NewValue:  "",
			CreatedAt: time.Now(),
		})
		if s.broadcaster != nil {
			_ = s.broadcaster.Broadcast(
				domain.RoomKeyProject(comment.OrgID, comment.ProjectID),
				string(domain.WsTypeTaskActivityRecorded),
				map[string]any{"task_id": task.ID},
			)
		}
	}

	return comment, nil
}

func (s *CommentService) Update(ctx context.Context, orgID, id, authorID, content string) (*domain.Comment, error) {
	if content == "" || len(content) > maxCommentLength {
		return nil, apperr.InvalidInput("comment must be between 1 and 10000 characters")
	}
	existing, err := s.commentRepo.GetByID(ctx, orgID, id)
	if err != nil {
		return nil, err
	}
	if existing.AuthorID != authorID {
		return nil, apperr.Forbidden("you can only edit your own comments")
	}
	if s.access != nil {
		if err := s.access.RequireTaskAccess(ctx, authorID, orgID, existing.TaskID, domain.PermTaskEdit); err != nil {
			return nil, err
		}
	}
	existing.Content = content
	if err := s.commentRepo.Update(ctx, existing); err != nil {
		return nil, err
	}
	comment, err := s.commentRepo.GetByID(ctx, orgID, id)
	if err != nil {
		return nil, err
	}
	s.hydrateComments(ctx, orgID, []*domain.Comment{comment})
	s.broadcastComment(ctx, string(domain.WsTypeCommentUpdated), comment)
	return comment, nil
}

func (s *CommentService) Delete(ctx context.Context, orgID, id, authorID string) error {
	existing, err := s.commentRepo.GetByID(ctx, orgID, id)
	if err != nil {
		return err
	}
	if existing.AuthorID != authorID {
		return apperr.Forbidden("you can only delete your own comments")
	}
	if s.access != nil {
		if err := s.access.RequireTaskAccess(ctx, authorID, orgID, existing.TaskID, domain.PermTaskEdit); err != nil {
			return err
		}
	}
	if err := s.commentRepo.SoftDelete(ctx, orgID, id); err != nil {
		return err
	}
	if s.broadcaster != nil {
		_ = s.broadcaster.Broadcast(
			domain.RoomKeyProject(orgID, existing.ProjectID),
			string(domain.WsTypeCommentDeleted),
			map[string]any{
				"comment_id": id,
				"task_id":    existing.TaskID,
			},
		)
	}
	return nil
}

// sendNotifications mirrors the chat message flow: notify @mentioned users,
// task assignees, and the parent comment's author: never the comment author.
func (s *CommentService) sendNotifications(ctx context.Context, comment *domain.Comment, task *domain.Task) {
	authorName := comment.AuthorName
	if authorName == "" {
		authorName = "Someone"
	}

	// Build the task link with the project slug (not the UUID) so it matches
	// every other task notification and the SPA router resolves it directly.
	// Falls back to the project ID if the project can't be loaded.
	projectKey := task.ProjectID
	if proj, err := s.projectRepo.GetByID(ctx, comment.OrgID, task.ProjectID); err == nil && proj.Slug != "" {
		projectKey = proj.Slug
	}
	taskLink := fmt.Sprintf("/projects/%s?task=%s", projectKey, task.ID)

	notified := map[string]bool{comment.AuthorID: true}

	// 1. Explicitly @mentioned users (and @everyone → all org members).
	mentionedUserIDs := extractMentionedUserIDs(comment.Content)
	hasEveryone := containsEveryoneMention(comment.Content)
	if hasEveryone {
		users, err := s.userRepo.ListUsers(ctx, comment.OrgID, domain.UserFilter{})
		if err == nil {
			for _, u := range users.Users {
				mentionedUserIDs[u.ID] = true
			}
		}
	}

	for userID := range mentionedUserIDs {
		if notified[userID] {
			continue
		}
		s.notifyComment(ctx, comment, task, userID, authorName, taskLink,
			"mentioned you in a comment",
			fmt.Sprintf("%s mentioned you on %s", authorName, task.Title),
		)
		notified[userID] = true
	}

	// 2. Task assignees (excluding the author).
	for _, a := range task.Assignees {
		if notified[a.ID] {
			continue
		}
		s.notifyComment(ctx, comment, task, a.ID, authorName, taskLink,
			"commented on a task you're assigned to",
			fmt.Sprintf("%s commented on %s", authorName, task.Title),
		)
		notified[a.ID] = true
	}

	// 3. Parent comment author (reply notification); excluding the author.
	if comment.ParentID != nil && *comment.ParentID != "" {
		parent, err := s.commentRepo.GetByID(ctx, comment.OrgID, *comment.ParentID)
		if err == nil && parent.AuthorID != comment.AuthorID && !notified[parent.AuthorID] {
			s.notifyComment(ctx, comment, task, parent.AuthorID, authorName, taskLink,
				"replied to your comment",
				fmt.Sprintf("%s replied to your comment on %s", authorName, task.Title),
			)
		}
	}
}

func (s *CommentService) notifyComment(ctx context.Context, comment *domain.Comment, task *domain.Task, recipientID, authorName, taskLink, bodyTitle, body string) {
	_ = s.notifSvc.Notify(ctx, comment.OrgID, recipientID,
		domain.NotifTaskComment,
		bodyTitle,
		domain.FormatMentionsForDisplay(body),
		taskLink,
		// entity_type "task" (not "task_comment") so the notifications query's
		// tasks↔projects JOIN resolves project_slug for the inbox link.
		"task", task.ID, comment.AuthorID,
	)
}

// broadcastComment pushes a comment event to the project room so any open task
// detail dialog updates live (mirrors chat message_new broadcasts).
func (s *CommentService) broadcastComment(ctx context.Context, eventType string, comment *domain.Comment) {
	wc := buildWireComment(comment)
	cp := commentPayload{Comment: wc, TaskID: comment.TaskID, ProjectID: comment.ProjectID, ConversationID: comment.TaskID}
	if err := s.broadcaster.Broadcast(domain.RoomKeyProject(comment.OrgID, comment.ProjectID), eventType, cp); err != nil {
		s.log.Warn("broadcast comment", "error", err, "event", eventType, "comment_id", comment.ID)
	}
}

// hydrateComments resolves <@type:id> tokens in comment content into a
// Mentions payload via the shared mentionHydrator.
func (s *CommentService) hydrateComments(ctx context.Context, orgID string, comments []*domain.Comment) {
	if len(comments) == 0 {
		return
	}
	contents := make([]string, len(comments))
	for i, c := range comments {
		contents[i] = c.Content
	}
	resolved := s.mentions.hydrateMany(ctx, orgID, contents)
	for i, c := range comments {
		c.Mentions = resolved[i]
	}
}
