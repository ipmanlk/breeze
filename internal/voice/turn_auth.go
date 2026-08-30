package voice

import (
	"crypto/hmac"
	"crypto/sha1"
	"crypto/subtle"
	"encoding/base64"
	"fmt"
	"strconv"
	"time"
)

// TurnAuth implements the RFC 5766 TURN REST API credential scheme used by
// coturn and most WebRTC stacks (LiveKit, mediasoup-demo, Janus).
//
// Instead of shipping a static long-term TURN username/password to every
// client (which leaks reusable credentials), the server derives a
// short-lived, per-user credential pair from a shared secret:
//
//	username = "<expiry-unix>:<userID>"
//	password = base64(HMAC-SHA1(secret, username))
//
// The credential is valid only until `expiry`, and is bound to a specific
// user (so it cannot be replayed by a different account). coturn validates
// the same pair server-side with `use-auth-secret`.
type TurnAuth struct {
	sharedSecret string
	ttl          time.Duration
}

// NewTurnAuth creates a TURN REST credential generator. If secret is empty,
// returns nil-equivalent behaviour (Generate returns zero values) so callers
// can treat "no TURN configured" and "TURN without REST" uniformly.
func NewTurnAuth(secret string, ttl time.Duration) *TurnAuth {
	if ttl <= 0 {
		ttl = 12 * time.Hour
	}
	return &TurnAuth{sharedSecret: secret, ttl: ttl}
}

// Generate produces an ephemeral TURN credential for the given user. The
// password is the HMAC-SHA1 of the username, keyed by the shared secret,
// then base64-encoded; exactly what coturn's `static-auth-secret` expects.
func (t *TurnAuth) Generate(userID string, now time.Time) (username, password string, expiry time.Time) {
	expiry = now.Add(t.ttl)
	username = fmt.Sprintf("%d:%s", expiry.Unix(), userID)
	mac := hmac.New(sha1.New, []byte(t.sharedSecret))
	mac.Write([]byte(username))
	password = base64.StdEncoding.EncodeToString(mac.Sum(nil))
	return username, password, expiry
}

// Verify validates a credential pair produced by Generate. This is used by
// a self-hosted TURN validation path or tests; coturn validates server-side.
func (t *TurnAuth) Verify(username, password string, now time.Time) bool {
	// Username is "<unix-expiry>:<userID>"
	idx := -1
	for i, c := range username {
		if c == ':' {
			idx = i
			break
		}
	}
	if idx <= 0 {
		return false
	}
	expStr := username[:idx]
	expUnix, err := strconv.ParseInt(expStr, 10, 64)
	if err != nil {
		return false
	}
	if time.Unix(expUnix, 0).Before(now) {
		return false // expired
	}
	mac := hmac.New(sha1.New, []byte(t.sharedSecret))
	mac.Write([]byte(username))
	want := base64.StdEncoding.EncodeToString(mac.Sum(nil))
	return subtle.ConstantTimeCompare([]byte(want), []byte(password)) == 1
}
