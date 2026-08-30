package middleware

import (
	"bytes"
	"encoding/json"
	"io"
	"net/http"
	"net/http/httptest"
	"testing"
	"time"
)

// TestRateLimitLoginByEmail_SameIP_DifferentEmails verifies that two
// different emails from the same IP each get their own rate-limit budget:
// email A exhausts its limit, email B from the same IP still succeeds.
func TestRateLimitLoginByEmail_SameIP_DifferentEmails(t *testing.T) {
	mw := RateLimitLoginByEmail(3, time.Hour)
	handler := mw(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		// Verify the body is still readable (peekEmail didn't consume it)
		body, err := io.ReadAll(r.Body)
		if err != nil {
			t.Errorf("body not readable: %v", err)
		}
		var req struct {
			Email string `json:"email"`
		}
		if err := json.Unmarshal(body, &req); err != nil {
			t.Errorf("body unmarshal failed: %v", err)
		}
		if req.Email == "" {
			t.Error("email field was consumed or empty")
		}
		w.WriteHeader(http.StatusOK)
	}))

	// Helper: send a login request for a given email+IP and return status.
	send := func(email, ip string) int {
		body := mustMarshal(map[string]string{"email": email, "password": "wrong"})
		r := httptest.NewRequest(http.MethodPost, "/api/auth/login", bytes.NewReader(body))
		r.Header.Set("Content-Type", "application/json")
		r.RemoteAddr = ip + ":12345"
		w := httptest.NewRecorder()
		handler.ServeHTTP(w, r)
		return w.Code
	}

	// Exhaust emailA's limit (3 attempts).
	for i := 0; i < 3; i++ {
		if code := send("alice@example.com", "10.0.0.1"); code != http.StatusOK {
			t.Fatalf("attempt %d for alice: expected 200, got %d", i+1, code)
		}
	}
	// alice's 4th attempt should be 429.
	if code := send("alice@example.com", "10.0.0.1"); code != http.StatusTooManyRequests {
		t.Errorf("4th alice attempt: expected 429, got %d", code)
	}

	// bob from the same IP should still succeed (independent budget).
	for i := 0; i < 3; i++ {
		if code := send("bob@example.com", "10.0.0.1"); code != http.StatusOK {
			t.Fatalf("attempt %d for bob: expected 200, got %d", i+1, code)
		}
	}
	// bob's 4th should also be 429.
	if code := send("bob@example.com", "10.0.0.1"); code != http.StatusTooManyRequests {
		t.Errorf("4th bob attempt: expected 429, got %d", code)
	}
}

// TestRateLimitLoginByEmail_BodyPeekPreservesBody verifies that the
// middleware reads the email without consuming the request body: the
// downstream handler can still read the full JSON body.
func TestRateLimitLoginByEmail_BodyPeekPreservesBody(t *testing.T) {
	mw := RateLimitLoginByEmail(10, time.Hour)
	handler := mw(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		body, err := io.ReadAll(r.Body)
		if err != nil {
			t.Errorf("body not readable: %v", err)
		}
		var req struct {
			Email    string `json:"email"`
			Password string `json:"password"`
		}
		if err := json.Unmarshal(body, &req); err != nil {
			t.Errorf("body unmarshal failed: %v", err)
		}
		if req.Email != "alice@example.com" {
			t.Errorf("expected email alice@example.com, got %q", req.Email)
		}
		if req.Password != "correct-horse-battery-staple" {
			t.Errorf("expected password, got %q", req.Password)
		}
		w.WriteHeader(http.StatusOK)
	}))

	body := mustMarshal(map[string]string{
		"email":    "alice@example.com",
		"password": "correct-horse-battery-staple",
	})
	r := httptest.NewRequest(http.MethodPost, "/api/auth/login", bytes.NewReader(body))
	r.Header.Set("Content-Type", "application/json")
	r.RemoteAddr = "10.0.0.1:12345"
	w := httptest.NewRecorder()
	handler.ServeHTTP(w, r)
	if w.Code != http.StatusOK {
		t.Errorf("expected 200, got %d", w.Code)
	}
}

// TestRateLimitLoginByEmail_MalformedBodyFallback verifies that a
// non-JSON body falls back to IP-only keying: the IP is limited even
// though no email could be extracted.
func TestRateLimitLoginByEmail_MalformedBodyFallback(t *testing.T) {
	mw := RateLimitLoginByEmail(2, time.Hour)
	handler := mw(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.WriteHeader(http.StatusOK)
	}))

	send := func(bodyContent string, ip string) int {
		r := httptest.NewRequest(http.MethodPost, "/api/auth/login", bytes.NewReader([]byte(bodyContent)))
		r.Header.Set("Content-Type", "application/json")
		r.RemoteAddr = ip + ":12345"
		w := httptest.NewRecorder()
		handler.ServeHTTP(w, r)
		return w.Code
	}

	// Use a malformed body: should still be rate-limited by IP.
	if code := send("not-json", "10.0.0.1"); code != http.StatusOK {
		t.Fatalf("attempt 1 (malformed): expected 200, got %d", code)
	}
	if code := send("still-not-json", "10.0.0.1"); code != http.StatusOK {
		t.Fatalf("attempt 2 (malformed): expected 200, got %d", code)
	}
	// 3rd attempt from same IP should be 429 even with no email.
	if code := send("nope", "10.0.0.1"); code != http.StatusTooManyRequests {
		t.Errorf("attempt 3 (malformed): expected 429, got %d", code)
	}

	// A different IP should not be affected.
	if code := send("not-json", "10.0.0.2"); code != http.StatusOK {
		t.Errorf("different IP: expected 200, got %d", code)
	}
}

// TestRateLimitLogin_SameIPSharedBudget verifies the IP-only limiter
// still works: same IP shares budget regardless of email.
func TestRateLimitLogin_SameIPSharedBudget(t *testing.T) {
	mw := RateLimitLogin(2, time.Hour)
	handler := mw(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.WriteHeader(http.StatusOK)
	}))

	send := func(email, ip string) int {
		body := mustMarshal(map[string]string{"email": email, "password": "x"})
		r := httptest.NewRequest(http.MethodPost, "/api/auth/login", bytes.NewReader(body))
		r.Header.Set("Content-Type", "application/json")
		r.RemoteAddr = ip + ":12345"
		w := httptest.NewRecorder()
		handler.ServeHTTP(w, r)
		return w.Code
	}

	if code := send("alice@example.com", "10.0.0.1"); code != http.StatusOK {
		t.Fatalf("alice 1: expected 200, got %d", code)
	}
	if code := send("bob@example.com", "10.0.0.1"); code != http.StatusOK {
		t.Fatalf("bob 1: expected 200, got %d", code)
	}
	// Same IP, 3rd attempt (different email) should be 429.
	if code := send("charlie@example.com", "10.0.0.1"); code != http.StatusTooManyRequests {
		t.Errorf("charlie 1 (same IP): expected 429, got %d", code)
	}
}

func TestRateLimitLoginByEmail_EmailNormalization(t *testing.T) {
	mw := RateLimitLoginByEmail(2, time.Hour)
	callCount := 0
	handler := mw(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		callCount++
		w.WriteHeader(http.StatusOK)
	}))

	send := func(email, ip string) int {
		body := mustMarshal(map[string]string{"email": email, "password": "x"})
		r := httptest.NewRequest(http.MethodPost, "/api/auth/login", bytes.NewReader(body))
		r.Header.Set("Content-Type", "application/json")
		r.RemoteAddr = ip + ":12345"
		w := httptest.NewRecorder()
		handler.ServeHTTP(w, r)
		return w.Code
	}

	// Same email, different cases: should share the same budget.
	if code := send("Alice@Example.com", "10.0.0.1"); code != http.StatusOK {
		t.Fatalf("Alice@Example.com: expected 200, got %d", code)
	}
	if code := send("alice@example.com", "10.0.0.1"); code != http.StatusOK {
		t.Fatalf("alice@example.com: expected 200, got %d", code)
	}
	// 3rd attempt with another case variant should be 429 (same budget).
	if code := send("ALICE@EXAMPLE.COM", "10.0.0.1"); code != http.StatusTooManyRequests {
		t.Errorf("ALICE@EXAMPLE.COM: expected 429, got %d", code)
	}
	// Different email still has its own budget.
	if code := send("bob@example.com", "10.0.0.1"); code != http.StatusOK {
		t.Errorf("bob@example.com: expected 200, got %d", code)
	}

	// Verify the downstream handler was called 3 times (not 4).
	if callCount != 3 {
		t.Errorf("expected 3 successful handler calls, got %d", callCount)
	}
}

// TestRateLimitPasswordResetConfirm_SameTokenSameBudget verifies that
// the same token from different IPs shares one per-token budget: 3rd
// attempt (regardless of IP) returns 429.
func TestRateLimitPasswordResetConfirm_SameTokenSameBudget(t *testing.T) {
	mw := RateLimitPasswordResetConfirm(2, time.Hour)
	callCount := 0
	handler := mw(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		callCount++
		w.WriteHeader(http.StatusOK)
	}))

	send := func(token, ip string) int {
		body := mustMarshal(map[string]string{"token": token, "new_password": "newPass123"})
		r := httptest.NewRequest(http.MethodPost, "/api/auth/password-reset/confirm", bytes.NewReader(body))
		r.Header.Set("Content-Type", "application/json")
		r.RemoteAddr = ip + ":12345"
		w := httptest.NewRecorder()
		handler.ServeHTTP(w, r)
		return w.Code
	}

	token := "valid-reset-token-abc123"

	// Two attempts with token from different IPs should succeed (budget = 2).
	if code := send(token, "10.0.0.1"); code != http.StatusOK {
		t.Fatalf("token attempt 1 (IP1): expected 200, got %d", code)
	}
	if code := send(token, "10.0.0.2"); code != http.StatusOK {
		t.Fatalf("token attempt 2 (IP2): expected 200, got %d", code)
	}
	// 3rd attempt from a third IP: same token, shared budget; should 429.
	if code := send(token, "10.0.0.3"); code != http.StatusTooManyRequests {
		t.Errorf("token attempt 3 (IP3): expected 429, got %d", code)
	}

	// Downstream handler called exactly twice (third request was rejected).
	if callCount != 2 {
		t.Errorf("expected 2 successful handler calls, got %d", callCount)
	}
}

// TestRateLimitPasswordResetConfirm_DifferentTokensSeparateBudgets verifies
// that different tokens have independent rate-limit budgets.
func TestRateLimitPasswordResetConfirm_DifferentTokensSeparateBudgets(t *testing.T) {
	mw := RateLimitPasswordResetConfirm(2, time.Hour)
	handler := mw(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.WriteHeader(http.StatusOK)
	}))

	send := func(token, ip string) int {
		body := mustMarshal(map[string]string{"token": token, "new_password": "newPass123"})
		r := httptest.NewRequest(http.MethodPost, "/api/auth/password-reset/confirm", bytes.NewReader(body))
		r.Header.Set("Content-Type", "application/json")
		r.RemoteAddr = ip + ":12345"
		w := httptest.NewRecorder()
		handler.ServeHTTP(w, r)
		return w.Code
	}

	// Exhaust tokenA's budget.
	tokenA := "token-a"
	tokenB := "token-b"

	if code := send(tokenA, "10.0.0.1"); code != http.StatusOK {
		t.Fatalf("tokenA attempt 1: expected 200, got %d", code)
	}
	if code := send(tokenA, "10.0.0.1"); code != http.StatusOK {
		t.Fatalf("tokenA attempt 2: expected 200, got %d", code)
	}
	// tokenA's 3rd should be 429.
	if code := send(tokenA, "10.0.0.1"); code != http.StatusTooManyRequests {
		t.Errorf("tokenA attempt 3: expected 429, got %d", code)
	}

	// tokenB from same IP should still succeed (independent budget).
	for i := 0; i < 2; i++ {
		if code := send(tokenB, "10.0.0.1"); code != http.StatusOK {
			t.Fatalf("tokenB attempt %d: expected 200, got %d", i+1, code)
		}
	}
	// tokenB's 3rd should also be 429.
	if code := send(tokenB, "10.0.0.1"); code != http.StatusTooManyRequests {
		t.Errorf("tokenB attempt 3: expected 429, got %d", code)
	}
}

// TestRateLimitPasswordResetConfirm_BodyPeekPreservesBody verifies that
// the middleware reads the token without consuming the request body: the
// downstream handler can still read the full JSON payload.
func TestRateLimitPasswordResetConfirm_BodyPeekPreservesBody(t *testing.T) {
	mw := RateLimitPasswordResetConfirm(10, time.Hour)
	handler := mw(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		body, err := io.ReadAll(r.Body)
		if err != nil {
			t.Errorf("body not readable: %v", err)
		}
		var req struct {
			Token       string `json:"token"`
			NewPassword string `json:"new_password"`
		}
		if err := json.Unmarshal(body, &req); err != nil {
			t.Errorf("body unmarshal failed: %v", err)
		}
		if req.Token != "my-reset-token" {
			t.Errorf("expected token 'my-reset-token', got %q", req.Token)
		}
		if req.NewPassword != "securePass1" {
			t.Errorf("expected new_password 'securePass1', got %q", req.NewPassword)
		}
		w.WriteHeader(http.StatusOK)
	}))

	body := mustMarshal(map[string]string{
		"token":        "my-reset-token",
		"new_password": "securePass1",
	})
	r := httptest.NewRequest(http.MethodPost, "/api/auth/password-reset/confirm", bytes.NewReader(body))
	r.Header.Set("Content-Type", "application/json")
	r.RemoteAddr = "10.0.0.1:12345"
	w := httptest.NewRecorder()
	handler.ServeHTTP(w, r)
	if w.Code != http.StatusOK {
		t.Errorf("expected 200, got %d", w.Code)
	}
}

func mustMarshal(v any) []byte {
	b, err := json.Marshal(v)
	if err != nil {
		panic(err)
	}
	return b
}

// TestRateLimiter_Stop_Idempotent verifies that calling Stop() multiple
// times does not panic (sync.Once guards the close).
func TestRateLimiter_Stop_Idempotent(t *testing.T) {
	rl := newRateLimiter(1, time.Minute)
	// First call must not panic.
	rl.Stop()
	// Second call must not panic either.
	rl.Stop()
}
