package service

import (
	"context"
	"errors"
	"fmt"
	"log/slog"

	"ipmanlk/plume/internal/domain"
	"ipmanlk/plume/internal/port"

	"github.com/google/uuid"
)

// pushService implements port.PushService, wrapping WebPush encryption +
// delivery with subscription storage.
type pushService struct {
	repo   port.PushSubscriptionRepository
	push   *WebPush
	logger *slog.Logger
}

var _ port.PushService = (*pushService)(nil)

// NewPushService builds a port.PushService. The underlying WebPush sender
// determines whether push is enabled (VAPID configured).
func NewPushService(repo port.PushSubscriptionRepository, push *WebPush, logger *slog.Logger) port.PushService {
	return &pushService{repo: repo, push: push, logger: logger}
}

func (s *pushService) Enabled() bool {
	return s.push.Enabled()
}

func (s *pushService) PublicKey() string {
	return s.push.PublicKey()
}

func (s *pushService) Subscribe(ctx context.Context, userID, orgID, endpoint, p256dh, auth string) error {
	if !s.Enabled() {
		return errors.New("push notifications are not enabled on this server")
	}
	if endpoint == "" || p256dh == "" || auth == "" {
		return fmt.Errorf("endpoint, p256dh, and auth are required")
	}
	_, err := s.repo.Upsert(ctx, &domain.PushSubscription{
		ID:       uuid.New().String(),
		UserID:   userID,
		OrgID:    orgID,
		Endpoint: endpoint,
		P256dh:   p256dh,
		Auth:     auth,
	})
	if err != nil {
		return fmt.Errorf("upsert push subscription: %w", err)
	}
	return nil
}

func (s *pushService) Unsubscribe(ctx context.Context, userID, endpoint string) error {
	_, err := s.repo.Delete(ctx, userID, endpoint)
	if err != nil {
		return fmt.Errorf("delete push subscription: %w", err)
	}
	return nil
}

// Send delivers a push notification to all of a user's subscriptions.
// Best-effort: errors for individual subscriptions are logged, and stale
// subscriptions (410 Gone) are pruned.
func (s *pushService) Send(ctx context.Context, userID string, payload domain.PushPayload) error {
	if !s.Enabled() {
		return nil
	}
	subs, err := s.repo.ListByUser(ctx, userID)
	if err != nil {
		return fmt.Errorf("list push subscriptions: %w", err)
	}
	for _, sub := range subs {
		if err := s.push.Push(ctx, Subscription{
			Endpoint: sub.Endpoint,
			P256dh:   sub.P256dh,
			Auth:     sub.Auth,
		}, PushPayload{
			Title: payload.Title,
			Body:  payload.Body,
			Link:  payload.Link,
			Tag:   payload.Tag,
			Data:  payload.Data,
		}); err != nil {
			if errors.Is(err, ErrPushSubscriptionGone) {
				// Prune the stale subscription.
				if _, derr := s.repo.Delete(ctx, userID, sub.Endpoint); derr != nil {
					s.logger.Warn("prune stale push subscription", "error", derr, "endpoint", sub.Endpoint)
				}
				continue
			}
			s.logger.Warn("push notification", "error", err, "endpoint", sub.Endpoint)
		}
	}
	return nil
}
