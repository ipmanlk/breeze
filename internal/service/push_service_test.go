package service

import (
	"context"
	"log/slog"
	"testing"

	"ipmanlk/breeze/internal/config"
	"ipmanlk/breeze/internal/domain"
)

// TestPushService_DisabledWhenNoVAPID verifies the service is a no-op when
// VAPID keys are not configured.
func TestPushService_DisabledWhenNoVAPID(t *testing.T) {
	wp := NewWebPush(config.VAPIDConfig{}, slog.Default())
	if wp.Enabled() {
		t.Fatal("WebPush should be disabled with empty VAPID config")
	}
	if wp.PublicKey() != "" {
		t.Errorf("PublicKey should be empty when disabled, got %q", wp.PublicKey())
	}
}

// TestPushService_SubscribeUnsubscribe verifies the repo round-trip without
// requiring a live push endpoint.
func TestPushService_SubscribeUnsubscribe(t *testing.T) {
	repo := newMockPushSubRepo()
	// Disabled push so Send is a no-op; Subscribe still needs Enabled() true
	// to proceed, so we construct a WebPush with valid VAPID keys.
	priv, err := generateTestVAPIDKey()
	if err != nil {
		t.Fatalf("generate vapid: %v", err)
	}
	wp := NewWebPush(priv, slog.Default())
	if !wp.Enabled() {
		t.Fatal("expected WebPush enabled with valid keys")
	}
	svc := NewPushService(repo, wp, slog.Default())

	ctx := context.Background()
	if err := svc.Subscribe(ctx, "user-1", "org-1", "https://push.example/abc", "p256dh-b64", "auth-b64"); err != nil {
		t.Fatalf("Subscribe: %v", err)
	}
	subs, _ := repo.ListByUser(ctx, "user-1")
	if len(subs) != 1 {
		t.Fatalf("expected 1 subscription, got %d", len(subs))
	}
	if subs[0].Endpoint != "https://push.example/abc" {
		t.Errorf("endpoint = %q", subs[0].Endpoint)
	}

	// Unsubscribe.
	if err := svc.Unsubscribe(ctx, "user-1", "https://push.example/abc"); err != nil {
		t.Fatalf("Unsubscribe: %v", err)
	}
	subs, _ = repo.ListByUser(ctx, "user-1")
	if len(subs) != 0 {
		t.Fatalf("expected 0 subscriptions after unsubscribe, got %d", len(subs))
	}
}

// TestPushService_SubscribeRejectsWhenDisabled verifies Subscribe fails when
// VAPID is not configured.
func TestPushService_SubscribeRejectsWhenDisabled(t *testing.T) {
	repo := newMockPushSubRepo()
	wp := NewWebPush(config.VAPIDConfig{}, slog.Default())
	svc := NewPushService(repo, wp, slog.Default())
	err := svc.Subscribe(context.Background(), "u", "o", "ep", "p", "a")
	if err == nil {
		t.Fatal("expected error when subscribing with push disabled")
	}
}

// TestPushService_SendNoopsWhenDisabled verifies Send does nothing (and
// returns no error) when VAPID is not configured.
func TestPushService_SendNoopsWhenDisabled(t *testing.T) {
	repo := newMockPushSubRepo()
	repo.subs = append(repo.subs, &domain.PushSubscription{
		UserID: "u1", Endpoint: "https://push.example/abc", P256dh: "x", Auth: "y",
	})
	wp := NewWebPush(config.VAPIDConfig{}, slog.Default())
	svc := NewPushService(repo, wp, slog.Default())
	if err := svc.Send(context.Background(), "u1", domain.PushPayload{Title: "Hi"}); err != nil {
		t.Fatalf("Send when disabled returned error: %v", err)
	}
}

// generateTestVAPIDKey produces a valid VAPID config (P-256 key pair) for tests.
func generateTestVAPIDKey() (config.VAPIDConfig, error) {
	cfg, err := generateVAPIDKeyPair()
	if err != nil {
		return config.VAPIDConfig{}, err
	}
	return cfg, nil
}
