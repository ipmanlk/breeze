package middleware

// Rate limiting here is in-process (an in-memory map guarded by a mutex).
// Breeze ships as a single self-hosted binary, so this is correct for the
// documented deployment. If multi-instance horizontal scaling is ever added,
// this limiter must be backed by shared storage (e.g. Redis); otherwise the
// effective limit becomes limit × instances.

import (
	"bytes"
	"encoding/json"
	"io"
	"net/http"
	"strings"
	"sync"
	"time"

	"ipmanlk/breeze/internal/transport"
)

type rateLimiter struct {
	mu       sync.Mutex
	attempts map[string][]time.Time
	limit    int
	window   time.Duration
	stop     chan struct{}
	stopOnce sync.Once
}

func newRateLimiter(limit int, window time.Duration) *rateLimiter {
	rl := &rateLimiter{
		attempts: make(map[string][]time.Time),
		limit:    limit,
		window:   window,
		stop:     make(chan struct{}),
	}
	go rl.cleanup(5 * time.Minute)
	return rl
}

// Stop signals the cleanup goroutine to exit. Currently not wired to app
// lifecycle (the process exits anyway), but makes the goroutine stoppable
// for graceful shutdown if the lifecycle is wired in the future.
// Safe to call multiple times; only the first call closes the channel.
func (rl *rateLimiter) Stop() {
	rl.stopOnce.Do(func() {
		close(rl.stop)
	})
}

func (rl *rateLimiter) cleanup(interval time.Duration) {
	ticker := time.NewTicker(interval)
	defer ticker.Stop()
	for {
		select {
		case <-rl.stop:
			return
		case <-ticker.C:
			rl.mu.Lock()
			cutoff := time.Now().Add(-rl.window)
			for key, times := range rl.attempts {
				var valid []time.Time
				for _, t := range times {
					if t.After(cutoff) {
						valid = append(valid, t)
					}
				}
				if len(valid) == 0 {
					delete(rl.attempts, key)
				} else {
					rl.attempts[key] = valid
				}
			}
			rl.mu.Unlock()
		}
	}
}

func (rl *rateLimiter) allow(key string) bool {
	rl.mu.Lock()
	defer rl.mu.Unlock()

	now := time.Now()
	cutoff := now.Add(-rl.window)

	times := rl.attempts[key]
	var valid []time.Time
	for _, t := range times {
		if t.After(cutoff) {
			valid = append(valid, t)
		}
	}

	if len(valid) >= rl.limit {
		rl.attempts[key] = valid
		return false
	}

	rl.attempts[key] = append(valid, now)
	return true
}

type loginEmailBody struct {
	Email string `json:"email"`
}

// passwordResetConfirmBody is used to peek at the token from the request body
// for per-token rate limiting on the password-reset confirm endpoint.
type passwordResetConfirmBody struct {
	Token string `json:"token"`
}

// RateLimitLogin rate-limits by client IP only. Suitable for routes where
// the request body does not carry a user-specific identifier (e.g.
// password-reset confirm, invite accept).
func RateLimitLogin(limit int, window time.Duration) func(http.Handler) http.Handler {
	rl := newRateLimiter(limit, window)
	return func(next http.Handler) http.Handler {
		return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
			ip := transport.ClientIP(r)
			if !rl.allow(ip) {
				w.Header().Set("Retry-After", "60")
				http.Error(w, `{"error":{"code":"rate_limited","message":"too many requests"}}`, http.StatusTooManyRequests)
				return
			}
			next.ServeHTTP(w, r)
		})
	}
}

// RateLimitLoginByEmail rate-limits by a composite of client IP + email from
// the request body. This prevents one user's failed logins from locking out
// other users behind the same IP (corporate NAT). If the email cannot be
// parsed from the body, it falls back to IP-only keying so a malformed body
// still gets basic protection.
func RateLimitLoginByEmail(limit int, window time.Duration) func(http.Handler) http.Handler {
	rl := newRateLimiter(limit, window)
	return func(next http.Handler) http.Handler {
		return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
			ip := transport.ClientIP(r)

			// Peek at the email from the request body without consuming it.
			email := peekEmail(r)

			key := ip
			if email != "" {
				key = "login:" + ip + ":" + email
			}

			if !rl.allow(key) {
				w.Header().Set("Retry-After", "60")
				http.Error(w, `{"error":{"code":"rate_limited","message":"too many requests"}}`, http.StatusTooManyRequests)
				return
			}
			next.ServeHTTP(w, r)
		})
	}
}

// peekEmail reads the request body, resets it, and extracts the "email" field
// from the JSON payload. Returns empty string on any read/parse error.
// The email is lowercased and trimmed so that "User@x.com" and "user@x.com"
// share the same rate-limit bucket, preventing case-variation bypass.
func peekEmail(r *http.Request) string {
	body, err := io.ReadAll(r.Body)
	if err != nil {
		return ""
	}
	r.Body = io.NopCloser(bytes.NewReader(body))

	var req loginEmailBody
	if err := json.Unmarshal(body, &req); err != nil {
		return ""
	}
	return strings.ToLower(strings.TrimSpace(req.Email))
}

// RateLimitPasswordResetConfirm rate-limits by the reset token from the
// request body. This prevents an attacker rotating source IPs from brute-
// forcing a single token by exhausting a shared per-token budget. A limit
// of 3 is recommended since legitimate use is exactly 1 attempt per token.
// On read/parse error or empty token, falls back to IP-only keying so a
// malformed body still gets basic protection.
func RateLimitPasswordResetConfirm(limit int, window time.Duration) func(http.Handler) http.Handler {
	rl := newRateLimiter(limit, window)
	return func(next http.Handler) http.Handler {
		return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
			token := peekToken(r)

			key := transport.ClientIP(r)
			if token != "" {
				key = "reset-confirm:" + token
			}

			if !rl.allow(key) {
				w.Header().Set("Retry-After", "60")
				http.Error(w, `{"error":{"code":"rate_limited","message":"too many requests"}}`, http.StatusTooManyRequests)
				return
			}
			next.ServeHTTP(w, r)
		})
	}
}

// peekToken reads the request body, resets it, and extracts the "token"
// field from the JSON payload. Returns empty string on any read/parse error.
func peekToken(r *http.Request) string {
	body, err := io.ReadAll(r.Body)
	if err != nil {
		return ""
	}
	r.Body = io.NopCloser(bytes.NewReader(body))

	var req passwordResetConfirmBody
	if err := json.Unmarshal(body, &req); err != nil {
		return ""
	}
	return strings.TrimSpace(req.Token)
}
