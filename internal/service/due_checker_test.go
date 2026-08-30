package service

import (
	"context"
	"log/slog"
	"testing"
	"time"
)

func TestDueChecker_CallsProcessDueNotifications(t *testing.T) {
	var called bool
	svc := &mockNotificationService{
		processFn: func(ctx context.Context) error {
			called = true
			return nil
		},
	}
	checker := NewDueChecker(svc, slog.Default(), 1*time.Hour)
	checker.runCheck(context.Background())
	if !called {
		t.Error("expected ProcessDueNotifications to be called")
	}
}
