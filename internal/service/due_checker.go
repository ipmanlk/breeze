package service

import (
	"context"
	"log/slog"
	"time"

	"ipmanlk/breeze/internal/port"
)

type DueChecker struct {
	notifSvc      port.NotificationService
	logger        *slog.Logger
	checkInterval time.Duration
}

func NewDueChecker(notifSvc port.NotificationService, logger *slog.Logger, checkInterval time.Duration) *DueChecker {
	return &DueChecker{
		notifSvc:      notifSvc,
		logger:        logger,
		checkInterval: checkInterval,
	}
}

func (c *DueChecker) Run(ctx context.Context) {
	c.runCheck(ctx)
	ticker := time.NewTicker(c.checkInterval)
	defer ticker.Stop()

	for {
		select {
		case <-ticker.C:
			c.runCheck(ctx)
		case <-ctx.Done():
			c.logger.Info("due checker shutting down")
			return
		}
	}
}

func (c *DueChecker) runCheck(ctx context.Context) {
	c.logger.Debug("running due notification check")
	if err := c.notifSvc.ProcessDueNotifications(ctx); err != nil {
		c.logger.Warn("due notification check", "error", err)
	}
}
