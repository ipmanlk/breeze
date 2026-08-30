package service

import (
	"context"
	"errors"
	"fmt"
	"log/slog"
	"time"

	"ipmanlk/breeze/internal/apperr"
	"ipmanlk/breeze/internal/domain"
	"ipmanlk/breeze/internal/i18n"
	"ipmanlk/breeze/internal/port"

	"github.com/google/uuid"
)

type NotificationService struct {
	repo         port.NotificationRepository
	prefRepo     port.NotificationPreferenceRepository
	userPrefRepo port.UserPreferencesRepository
	userRepo     port.UserRepository
	broadcaster  port.Broadcaster
	mailer       port.Mailer
	push         port.PushService
	i18n         *i18n.Bundle
	logger       *slog.Logger
}

func NewNotificationService(
	repo port.NotificationRepository,
	prefRepo port.NotificationPreferenceRepository,
	userPrefRepo port.UserPreferencesRepository,
	userRepo port.UserRepository,
	broadcaster port.Broadcaster,
	mailer port.Mailer,
	push port.PushService,
	i18nBundle *i18n.Bundle,
	logger *slog.Logger,
) *NotificationService {
	return &NotificationService{
		repo:         repo,
		prefRepo:     prefRepo,
		userPrefRepo: userPrefRepo,
		userRepo:     userRepo,
		broadcaster:  broadcaster,
		mailer:       mailer,
		push:         push,
		i18n:         i18nBundle,
		logger:       logger,
	}
}

// localize is a nil-safe wrapper around i18n.Bundle.MustLocalize. When the
// bundle is nil (e.g. in tests), returns the messageID as a no-op fallback.
func (s *NotificationService) localize(locale, messageID string, data map[string]any, pluralCount any) string {
	if s.i18n == nil {
		return messageID
	}
	return s.i18n.MustLocalize(locale, messageID, data, pluralCount)
}

var _ port.NotificationService = (*NotificationService)(nil)

// localeForUser returns the recipient's preferred locale (BCP-47), falling
// back to the source locale if the preference can't be read, the repo
// returns nil, or the repo is not configured (e.g. in tests).
func (s *NotificationService) localeForUser(ctx context.Context, userID string) string {
	if s.userPrefRepo == nil {
		return i18n.SourceLocale
	}
	prefs, err := s.userPrefRepo.Get(ctx, userID)
	if err != nil || prefs == nil {
		return i18n.SourceLocale
	}
	return i18n.Normalize(prefs.Language)
}

func (s *NotificationService) Notify(ctx context.Context, orgID, recipientID string, notifType domain.NotificationType, title, body, link, entityType, entityID, actorID string) error {
	if recipientID == "" {
		return nil
	}

	enabled, err := s.isTypeEnabled(ctx, recipientID, notifType)
	if err != nil {
		return fmt.Errorf("check preferences: %w", err)
	}
	if !enabled {
		return nil
	}

	now := time.Now()
	n := &domain.Notification{
		ID:         uuid.New().String(),
		OrgID:      orgID,
		UserID:     recipientID,
		Type:       notifType,
		Title:      title,
		Body:       body,
		Link:       link,
		EntityType: entityType,
		EntityID:   entityID,
		ActorID:    actorID,
		CreatedAt:  now,
	}

	if err := s.repo.Create(ctx, n); err != nil {
		return fmt.Errorf("create notification: %w", err)
	}

	if err := s.broadcaster.Broadcast(
		domain.RoomKeyUser(orgID, recipientID),
		string(domain.WsTypeNotificationNew),
		map[string]any{
			"id":          n.ID,
			"type":        string(n.Type),
			"title":       n.Title,
			"body":        n.Body,
			"link":        n.Link,
			"entity_type": n.EntityType,
			"entity_id":   n.EntityID,
			"actor_id":    n.ActorID,
			"is_read":     false,
			"created_at":  n.CreatedAt.Format(time.RFC3339),
		},
	); err != nil {
		s.logger.Warn("broadcast notification", "error", err, "user_id", recipientID)
	}

	// Best-effort delivery of email + browser push. Email fires when the
	// mailer is configured AND the recipient has email_notifications on.
	// Push fires when VAPID is configured AND the recipient has
	// desktop_notifications on. Both are best-effort: failures are logged
	// and never block the in-app notification that was already created.
	s.maybeSendEmail(ctx, orgID, recipientID, title, body, link)
	s.maybeSendPush(ctx, recipientID, title, body, link)

	return nil
}

// maybeSendEmail delivers an email copy of a notification when the mailer is
// configured and the recipient has email notifications enabled. Best-effort:
// errors are logged and never propagated.
func (s *NotificationService) maybeSendEmail(ctx context.Context, orgID, userID, title, body, link string) {
	if s.mailer == nil || !s.mailer.Enabled() {
		return
	}
	emailEnabled, err := s.isEmailEnabled(ctx, userID)
	if err != nil || !emailEnabled {
		return
	}
	user, err := s.userRepo.GetByID(ctx, orgID, userID)
	if err != nil || user == nil || user.Email == "" {
		return
	}
	tmpl := NotificationEmail(title, body, link)
	if err := s.mailer.Send(ctx, user.Email, tmpl.Subject, tmpl.HTML, tmpl.Text); err != nil {
		s.logger.Warn("email notification", "error", err, "user_id", userID)
	}
}

// maybeSendPush delivers a browser push notification when Web Push is
// configured and the recipient has desktop_notifications enabled. The push
// is only meaningful when the user's browser tab is closed (the service
// worker can't display anything if the tab is focused; the in-app toast +
// badge already cover that). Best-effort: errors are logged.
func (s *NotificationService) maybeSendPush(ctx context.Context, userID, title, body, link string) {
	if s.push == nil || !s.push.Enabled() {
		return
	}
	desktopEnabled, err := s.isDesktopEnabled(ctx, userID)
	if err != nil || !desktopEnabled {
		return
	}
	if err := s.push.Send(ctx, userID, domain.PushPayload{
		Title: title,
		Body:  body,
		Link:  link,
		Tag:   "breeze-notification",
	}); err != nil {
		s.logger.Warn("push notification", "error", err, "user_id", userID)
	}
}

// isEmailEnabled reads the user's email_notifications preference. Defaults
// to true when no explicit override exists (matching the domain default).
func (s *NotificationService) isEmailEnabled(ctx context.Context, userID string) (bool, error) {
	prefs, err := s.userPrefRepo.Get(ctx, userID)
	if err != nil {
		return true, nil
	}
	return prefs.EmailNotifications, nil
}

// isDesktopEnabled reads the user's desktop_notifications preference.
// Defaults to true when no explicit override exists (matching the domain
// default). Browser push only fires when this is on.
func (s *NotificationService) isDesktopEnabled(ctx context.Context, userID string) (bool, error) {
	prefs, err := s.userPrefRepo.Get(ctx, userID)
	if err != nil {
		return true, nil
	}
	return prefs.DesktopNotifications, nil
}

func (s *NotificationService) isTypeEnabled(ctx context.Context, userID string, notifType domain.NotificationType) (bool, error) {
	defaultEnabled, ok := domain.DefaultNotificationPreferences[notifType]
	if !ok {
		return true, nil
	}

	pref, err := s.prefRepo.GetByType(ctx, userID, notifType)
	if err != nil {
		// No explicit override for this type: fall back to the default.
		if errors.Is(err, apperr.ErrNotFound) {
			return defaultEnabled, nil
		}
		return false, err
	}
	return pref.Enabled, nil
}

func (s *NotificationService) List(ctx context.Context, orgID, userID string, filter domain.NotificationFilter) (*domain.NotificationListResult, error) {
	return s.repo.List(ctx, orgID, userID, filter)
}

func (s *NotificationService) CountUnread(ctx context.Context, userID string) (int, error) {
	count, err := s.repo.CountUnread(ctx, userID)
	return int(count), err
}

func (s *NotificationService) MarkRead(ctx context.Context, id, userID string) error {
	return s.repo.MarkRead(ctx, id, userID)
}

func (s *NotificationService) MarkAllRead(ctx context.Context, userID string) error {
	return s.repo.MarkAllRead(ctx, userID)
}

func (s *NotificationService) GetPreferences(ctx context.Context, userID string) ([]*domain.NotificationPreference, error) {
	overrides, err := s.prefRepo.List(ctx, userID)
	if err != nil {
		return nil, err
	}

	overrideMap := make(map[domain.NotificationType]bool)
	for _, o := range overrides {
		overrideMap[o.Type] = o.Enabled
	}

	prefs := make([]*domain.NotificationPreference, 0, len(domain.AllNotificationTypes))
	for _, t := range domain.AllNotificationTypes {
		enabled, ok := overrideMap[t]
		if !ok {
			enabled = domain.DefaultNotificationPreferences[t]
		}
		prefs = append(prefs, &domain.NotificationPreference{
			Type:    t,
			Enabled: enabled,
		})
	}

	return prefs, nil
}

func (s *NotificationService) SetPreference(ctx context.Context, userID string, notifType domain.NotificationType, enabled bool) error {
	if err := s.prefRepo.Set(ctx, userID, string(notifType), enabled); err != nil {
		return fmt.Errorf("set preference: %w", err)
	}
	return nil
}

func (s *NotificationService) ProcessDueNotifications(ctx context.Context) error {
	now := time.Now().UTC()
	// Widen the overdue window to 24h to match the dedup subquery window.
	// This catches tasks that became overdue while the server was down
	// (up to 24h ago); the dedup subquery prevents double-notification.
	nowMinus24h := now.Add(-24 * time.Hour)
	nowPlus24h := now.Add(24 * time.Hour)

	rows, err := s.prefRepo.FindDueNotifications(ctx, nowMinus24h, now, nowPlus24h, string(domain.NotifTaskDueSoon), string(domain.NotifTaskOverdue))
	if err != nil {
		return fmt.Errorf("find due notifications: %w", err)
	}

	for _, row := range rows {
		var notifType domain.NotificationType
		dueAt, dueErr := time.Parse("2006-01-02 15:04:05", row.DueAt)
		if dueErr == nil && dueAt.Before(now) {
			notifType = domain.NotifTaskOverdue
		} else {
			notifType = domain.NotifTaskDueSoon
		}

		// Resolve the assignee's locale for localized notification strings.
		loc := s.localeForUser(ctx, row.AssigneeID)

		title := s.localize(loc, "TaskDueSoonTitle", map[string]any{"Title": row.Title}, nil)
		if notifType == domain.NotifTaskOverdue {
			title = s.localize(loc, "TaskOverdueTitle", map[string]any{"Title": row.Title}, nil)
		}
		body := s.localize(loc, "TaskDueSoonBody", map[string]any{"Title": row.Title, "DueAt": row.DueAt}, nil)
		if notifType == domain.NotifTaskOverdue {
			body = s.localize(loc, "TaskOverdueBody", map[string]any{"Title": row.Title, "DueAt": row.DueAt}, nil)
		}

		if err := s.Notify(ctx, row.OrgID, row.AssigneeID, notifType, title, body, fmt.Sprintf("/projects/%s?task=%s", row.ProjectSlug, row.TaskID), "task", row.TaskID, ""); err != nil {
			s.logger.Warn("process due notification", "error", err, "task", row.TaskID, "assignee", row.AssigneeID)
		}
	}

	return nil
}
