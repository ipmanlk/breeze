package service

import (
	"context"
	"ipmanlk/breeze/internal/i18n"
	"log/slog"
	"testing"
	"time"

	"ipmanlk/breeze/internal/domain"
	"ipmanlk/breeze/internal/port"
)

var notifTestBundle = i18n.NewBundle()

func TestNotificationService_Notify_CreatesAndBroadcasts(t *testing.T) {
	notifRepo := newMockNotifRepo()
	prefRepo := newMockNotifPrefRepo()
	broadcaster := newMockBroadcaster()

	svc := NewNotificationService(notifRepo, prefRepo, newMockUserPreferencesRepo(), newMockUserRepo(), broadcaster, newMockMailer(false), newMockPushService(false), notifTestBundle, testLogger)
	err := svc.Notify(context.Background(), "org-1", "user-1", domain.NotifTaskAssigned,
		"Assigned to: Test task", "You were assigned", "/projects/p1?task=t1",
		"task", "t1", "actor-1")
	if err != nil {
		t.Fatalf("Notify() error = %v", err)
	}
	if len(notifRepo.notifications) != 1 {
		t.Fatalf("expected 1 notification created, got %d", len(notifRepo.notifications))
	}
	if notifRepo.notifications[0].UserID != "user-1" {
		t.Errorf("expected user-1, got %s", notifRepo.notifications[0].UserID)
	}
	if notifRepo.notifications[0].Type != domain.NotifTaskAssigned {
		t.Errorf("expected task_assigned, got %s", notifRepo.notifications[0].Type)
	}
	if len(broadcaster.messages) != 1 {
		t.Fatalf("expected 1 broadcast, got %d", len(broadcaster.messages))
	}
	if broadcaster.messages[0].roomKey != "org:org-1:user:user-1" {
		t.Errorf("expected org:org-1:user:user-1, got %s", broadcaster.messages[0].roomKey)
	}
	if broadcaster.messages[0].eventType != "notification_new" {
		t.Errorf("expected notification_new, got %s", broadcaster.messages[0].eventType)
	}
}

func TestNotificationService_Notify_RespectsPreferences(t *testing.T) {
	notifRepo := newMockNotifRepo()
	prefRepo := newMockNotifPrefRepo()
	broadcaster := newMockBroadcaster()

	prefRepo.prefs[string(domain.NotifTaskAssigned)] = false

	svc := NewNotificationService(notifRepo, prefRepo, newMockUserPreferencesRepo(), newMockUserRepo(), broadcaster, newMockMailer(false), newMockPushService(false), notifTestBundle, testLogger)
	err := svc.Notify(context.Background(), "org-1", "user-1", domain.NotifTaskAssigned,
		"Assigned to: Test", "Body", "/link", "task", "t1", "actor-1")
	if err != nil {
		t.Fatalf("Notify() error = %v", err)
	}
	if len(notifRepo.notifications) != 0 {
		t.Errorf("expected 0 notifications created, got %d", len(notifRepo.notifications))
	}
	if len(broadcaster.messages) != 0 {
		t.Errorf("expected 0 broadcasts, got %d", len(broadcaster.messages))
	}
}

func TestNotificationService_GetPreferences_MergesDefaults(t *testing.T) {
	notifRepo := newMockNotifRepo()
	prefRepo := newMockNotifPrefRepo()
	broadcaster := newMockBroadcaster()

	prefRepo.prefs[string(domain.NotifTaskOverdue)] = false

	svc := NewNotificationService(notifRepo, prefRepo, newMockUserPreferencesRepo(), newMockUserRepo(), broadcaster, newMockMailer(false), newMockPushService(false), notifTestBundle, testLogger)
	prefs, err := svc.GetPreferences(context.Background(), "user-1")
	if err != nil {
		t.Fatalf("GetPreferences() error = %v", err)
	}
	if len(prefs) != len(domain.AllNotificationTypes) {
		t.Fatalf("expected %d preferences, got %d", len(domain.AllNotificationTypes), len(prefs))
	}

	found := false
	for _, p := range prefs {
		if p.Type == domain.NotifTaskOverdue {
			found = true
			if p.Enabled {
				t.Errorf("expected overdue to be disabled")
			}
		}
	}
	if !found {
		t.Errorf("overdue not found in preferences")
	}
}

func TestNotificationService_EmptyRecipient_NoOp(t *testing.T) {
	notifRepo := newMockNotifRepo()
	prefRepo := newMockNotifPrefRepo()
	broadcaster := newMockBroadcaster()

	svc := NewNotificationService(notifRepo, prefRepo, newMockUserPreferencesRepo(), newMockUserRepo(), broadcaster, newMockMailer(false), newMockPushService(false), notifTestBundle, testLogger)
	err := svc.Notify(context.Background(), "org-1", "", domain.NotifTaskAssigned,
		"Title", "Body", "/link", "task", "t1", "actor-1")
	if err != nil {
		t.Fatalf("Notify() error = %v", err)
	}
	if len(notifRepo.notifications) != 0 {
		t.Errorf("expected 0 notifications created for empty recipient")
	}
}

var _ port.NotificationPreferenceRepository = (*mockNotifPrefRepo)(nil)
var _ port.NotificationRepository = (*mockNotifRepo)(nil)

var testLogger = slog.Default()

func TestNotificationService_Notify_SendsEmailWhenEnabled(t *testing.T) {
	notifRepo := newMockNotifRepo()
	prefRepo := newMockNotifPrefRepo()
	broadcaster := newMockBroadcaster()
	mailer := newMockMailer(true)
	userRepo := newMockUserRepo()
	userRepo.usersByID["user-1"] = &domain.User{ID: "user-1", OrgID: "org-1", Email: "user1@test.com", Name: "User One"}
	svc := NewNotificationService(notifRepo, prefRepo, newMockUserPreferencesRepo(), userRepo, broadcaster, mailer, newMockPushService(false), notifTestBundle, testLogger)

	err := svc.Notify(context.Background(), "org-1", "user-1", domain.NotifTaskAssigned,
		"Assigned: Task", "Body", "/projects/p?task=t", "task", "t", "actor-1")
	if err != nil {
		t.Fatalf("Notify() error = %v", err)
	}
	if len(mailer.sent) != 1 {
		t.Fatalf("expected 1 email sent, got %d", len(mailer.sent))
	}
	if mailer.sent[0].To != "user1@test.com" {
		t.Errorf("email To = %q, want user1@test.com", mailer.sent[0].To)
	}
}

func TestNotificationService_Notify_SkipsEmailWhenMailerDisabled(t *testing.T) {
	notifRepo := newMockNotifRepo()
	prefRepo := newMockNotifPrefRepo()
	broadcaster := newMockBroadcaster()
	mailer := newMockMailer(false) // disabled
	svc := NewNotificationService(notifRepo, prefRepo, newMockUserPreferencesRepo(), newMockUserRepo(), broadcaster, mailer, newMockPushService(false), notifTestBundle, testLogger)

	err := svc.Notify(context.Background(), "org-1", "user-1", domain.NotifTaskAssigned,
		"Assigned: Task", "Body", "/link", "task", "t", "actor-1")
	if err != nil {
		t.Fatalf("Notify() error = %v", err)
	}
	if len(mailer.sent) != 0 {
		t.Fatalf("expected 0 emails when mailer disabled, got %d", len(mailer.sent))
	}
}

func TestNotificationService_Notify_SendsPushWhenEnabled(t *testing.T) {
	notifRepo := newMockNotifRepo()
	prefRepo := newMockNotifPrefRepo()
	broadcaster := newMockBroadcaster()
	push := newMockPushService(true)
	userRepo := newMockUserRepo()
	userRepo.usersByID["user-1"] = &domain.User{ID: "user-1", OrgID: "org-1", Email: "u@t.com"}
	svc := NewNotificationService(notifRepo, prefRepo, newMockUserPreferencesRepo(), userRepo, broadcaster, newMockMailer(false), push, notifTestBundle, testLogger)

	err := svc.Notify(context.Background(), "org-1", "user-1", domain.NotifTaskAssigned,
		"Assigned: Task", "Body", "/link", "task", "t", "actor-1")
	if err != nil {
		t.Fatalf("Notify() error = %v", err)
	}
	if len(push.sent) != 1 {
		t.Fatalf("expected 1 push sent, got %d", len(push.sent))
	}
	if push.sent[0].Title != "Assigned: Task" {
		t.Errorf("push title = %q", push.sent[0].Title)
	}
}

func TestNotificationService_Notify_SkipsPushWhenDisabled(t *testing.T) {
	notifRepo := newMockNotifRepo()
	prefRepo := newMockNotifPrefRepo()
	broadcaster := newMockBroadcaster()
	push := newMockPushService(false) // disabled
	svc := NewNotificationService(notifRepo, prefRepo, newMockUserPreferencesRepo(), newMockUserRepo(), broadcaster, newMockMailer(false), push, notifTestBundle, testLogger)

	err := svc.Notify(context.Background(), "org-1", "user-1", domain.NotifTaskAssigned,
		"Assigned: Task", "Body", "/link", "task", "t", "actor-1")
	if err != nil {
		t.Fatalf("Notify() error = %v", err)
	}
	if len(push.sent) != 0 {
		t.Fatalf("expected 0 pushes when disabled, got %d", len(push.sent))
	}
}

func TestNotificationService_ProcessDueNotifications_Window(t *testing.T) {
	const fmt_ = "2006-01-02 15:04:05"
	orgID := "org-1"
	userID := "user-1"

	t.Run("catches_overdue_within_24h_window", func(t *testing.T) {
		notifRepo := newMockNotifRepo()
		prefRepo := newMockNotifPrefRepo()
		bcast := newMockBroadcaster()

		// Task became overdue 5 hours ago: within the 24h window (but
		// beyond the old 1h window that caused missed-overdue bug).
		fiveHoursAgo := time.Now().UTC().Add(-5 * time.Hour).Format(fmt_)
		prefRepo.dueTaskRows = []domain.DueTaskRow{{
			TaskID:      "task-1",
			Title:       "Overdue task",
			DueAt:       fiveHoursAgo,
			StatusID:    "status-1",
			AssigneeID:  userID,
			OrgID:       orgID,
			ProjectID:   "proj-1",
			ProjectSlug: "proj-one",
		}}

		svc := NewNotificationService(notifRepo, prefRepo, newMockUserPreferencesRepo(), newMockUserRepo(), bcast, newMockMailer(true), newMockPushService(true), notifTestBundle, testLogger)
		err := svc.ProcessDueNotifications(context.Background())
		if err != nil {
			t.Fatalf("ProcessDueNotifications() error = %v", err)
		}

		if len(notifRepo.notifications) != 1 {
			t.Fatalf("expected 1 notification created for overdue catch-up, got %d", len(notifRepo.notifications))
		}
		if notifRepo.notifications[0].Type != domain.NotifTaskOverdue {
			t.Errorf("expected NotifTaskOverdue, got %s", notifRepo.notifications[0].Type)
		}
		if notifRepo.notifications[0].Title != "Task overdue: Overdue task" {
			t.Errorf("expected overdue title, got %q", notifRepo.notifications[0].Title)
		}
	})

	t.Run("no_rows_returns_no_notifications", func(t *testing.T) {
		notifRepo := newMockNotifRepo()
		prefRepo := newMockNotifPrefRepo()
		bcast := newMockBroadcaster()

		// No rows returned (simulating SQL filtering for tasks beyond 24h
		// or already notified within dedup window).
		prefRepo.dueTaskRows = nil

		svc := NewNotificationService(notifRepo, prefRepo, newMockUserPreferencesRepo(), newMockUserRepo(), bcast, newMockMailer(true), newMockPushService(true), notifTestBundle, testLogger)
		err := svc.ProcessDueNotifications(context.Background())
		if err != nil {
			t.Fatalf("ProcessDueNotifications() error = %v", err)
		}

		if len(notifRepo.notifications) != 0 {
			t.Errorf("expected 0 notifications when no rows, got %d", len(notifRepo.notifications))
		}
	})
}
