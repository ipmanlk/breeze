package voice

import (
	"testing"
	"time"
)

// TestTurnAuth_GenerateProducesVerifiableCredentials checks the RFC 5766 TURN
// REST API credential scheme: username is "<unix-expiry>:<userID>", password
// is base64(HMAC-SHA1(secret, username)). The generated credential must be
// verifiable and bound to the user + expiry.
func TestTurnAuth_GenerateProducesVerifiableCredentials(t *testing.T) {
	auth := NewTurnAuth("super-secret", time.Hour)
	now := time.Unix(1700000000, 0)

	username, password, expiry := auth.Generate("user-42", now)

	// Username format: "<expiry-unix>:<userID>"
	wantUser := "1700003600:user-42"
	if username != wantUser {
		t.Errorf("username = %q, want %q", username, wantUser)
	}
	if !expiry.Equal(now.Add(time.Hour)) {
		t.Errorf("expiry = %v, want %v", expiry, now.Add(time.Hour))
	}
	if password == "" {
		t.Fatal("password should not be empty")
	}

	// The credential must verify.
	if !auth.Verify(username, password, now) {
		t.Error("generated credential failed to verify")
	}
}

// TestTurnAuth_DifferentUsersGetDifferentPasswords confirms the credential is
// bound to a specific user (a credential issued to user-A cannot be used by
// user-B). This is the security property that makes ephemeral creds superior
// to a single static password shared across all clients.
func TestTurnAuth_DifferentUsersGetDifferentPasswords(t *testing.T) {
	auth := NewTurnAuth("secret", time.Hour)
	now := time.Now()

	_, passA, _ := auth.Generate("user-A", now)
	_, passB, _ := auth.Generate("user-B", now)

	if passA == passB {
		t.Error("different users must get different passwords")
	}
}

// TestTurnAuth_RejectsTamperedPassword confirms a credential with a wrong
// password (e.g. an attacker trying to forge a TURN login) is rejected.
func TestTurnAuth_RejectsTamperedPassword(t *testing.T) {
	auth := NewTurnAuth("secret", time.Hour)
	now := time.Now()

	username, _, _ := auth.Generate("user-A", now)
	if auth.Verify(username, "wrong-password", now) {
		t.Error("tampered password should not verify")
	}
}

// TestTurnAuth_RejectsExpiredCredential confirms the credential becomes
// invalid after its TTL elapses (no long-lived reusable creds).
func TestTurnAuth_RejectsExpiredCredential(t *testing.T) {
	auth := NewTurnAuth("secret", time.Hour)
	issued := time.Now()

	username, password, _ := auth.Generate("user-A", issued)

	// Valid now.
	if !auth.Verify(username, password, issued) {
		t.Error("credential should be valid immediately after issue")
	}
	// Invalid after TTL.
	if auth.Verify(username, password, issued.Add(2*time.Hour)) {
		t.Error("expired credential should not verify")
	}
}

// TestTurnAuth_RejectsWrongSecret confirms that a credential generated under
// one secret cannot be verified under another (server-side secret rotation
// invalidates old creds).
func TestTurnAuth_RejectsWrongSecret(t *testing.T) {
	auth1 := NewTurnAuth("secret-one", time.Hour)
	auth2 := NewTurnAuth("secret-two", time.Hour)
	now := time.Now()

	username, password, _ := auth1.Generate("user-A", now)
	if auth2.Verify(username, password, now) {
		t.Error("credential verified under a different secret")
	}
}

// TestTurnAuth_RejectsMalformedUsername confirms garbage usernames don't
// accidentally verify.
func TestTurnAuth_RejectsMalformedUsername(t *testing.T) {
	auth := NewTurnAuth("secret", time.Hour)
	now := time.Now()
	cases := []string{"", "no-colon", ":userid", "notanumber:userid"}
	for _, u := range cases {
		if auth.Verify(u, "anything", now) {
			t.Errorf("malformed username %q should not verify", u)
		}
	}
}

// TestEngine_ICEServersForUser_StaticWhenNoSecret confirms that without a
// TURN secret, the per-user list equals the static list (STUN + static TURN).
func TestEngine_ICEServersForUser_StaticWhenNoSecret(t *testing.T) {
	e, err := NewEngine(Config{
		STUNURLs:    []string{"stun:stun.example.com:19302"},
		TurnEnabled: true,
		TurnHost:    "turn.example.com",
		TurnPort:    3478,
		TurnUser:    "static-user",
		TurnPass:    "static-pass",
	}, nil)
	if err != nil {
		t.Fatalf("NewEngine: %v", err)
	}

	static := e.ICEServers()
	perUser := e.ICEServersForUser("someone")
	if len(static) != len(perUser) {
		t.Fatalf("without secret, per-user should equal static: %d vs %d", len(static), len(perUser))
	}
}

// TestEngine_ICEServersForUser_EphemeralWhenSecret confirms that with a TURN
// secret, each user gets STUN + an ephemeral TURN server whose username is
// bound to the user and whose password differs per user.
func TestEngine_ICEServersForUser_EphemeralWhenSecret(t *testing.T) {
	e, err := NewEngine(Config{
		STUNURLs:    []string{"stun:stun.example.com:19302"},
		TurnEnabled: true,
		TurnHost:    "turn.example.com",
		TurnPort:    3478,
		TurnSecret:  "shared-turn-secret",
	}, nil)
	if err != nil {
		t.Fatalf("NewEngine: %v", err)
	}

	serversA := e.ICEServersForUser("user-A")
	serversB := e.ICEServersForUser("user-B")

	if len(serversA) != 2 || len(serversB) != 2 {
		t.Fatalf("expected STUN + TURN (2 servers), got %d/%d", len(serversA), len(serversB))
	}

	// STUN is shared (first entry).
	if serversA[0].Username != "" {
		t.Errorf("STUN should have no username, got %q", serversA[0].Username)
	}

	// TURN is per-user (second entry).
	turnA := serversA[1]
	turnB := serversB[1]
	if turnA.Username == "" || turnA.Credential == "" {
		t.Fatal("expected ephemeral TURN username + credential")
	}
	if turnA.Username == turnB.Username {
		t.Error("TURN username must differ per user")
	}
	if turnA.Credential == turnB.Credential {
		t.Error("TURN credential must differ per user")
	}
	// Username embeds the userID.
	if !contains(turnA.Username, "user-A") {
		t.Errorf("TURN username %q should contain user-A", turnA.Username)
	}
}

// TestEngine_MaxParticipantsDefaults checks that the SFU reports the cap.
func TestEngine_MaxParticipantsDefaults(t *testing.T) {
	e, err := NewEngine(Config{MaxParticipants: 25}, nil)
	if err != nil {
		t.Fatalf("NewEngine: %v", err)
	}
	if e.MaxParticipants() != 25 {
		t.Errorf("MaxParticipants = %d, want 25", e.MaxParticipants())
	}
}

func contains(s, substr string) bool {
	for i := 0; i+len(substr) <= len(s); i++ {
		if s[i:i+len(substr)] == substr {
			return true
		}
	}
	return false
}

// TestEngine_ICEServersForUser_MultipleTurnURLs verifies that when TurnURLs
// is set (e.g. UDP + TCP + TLS), all of them are advertised to the client with
// the same per-user ephemeral credential. This is the RFC 5766 recommendation
// (advertise multiple URIs so clients can fall back across transports for
// restrictive NATs/firewalls).
func TestEngine_ICEServersForUser_MultipleTurnURLs(t *testing.T) {
	e, err := NewEngine(Config{
		STUNURLs:    []string{"stun:stun.example.com:19302"},
		TurnEnabled: true,
		TurnURLs: []string{
			"turn:turn.example.com:3478",
			"turn:turn.example.com:3478?transport=tcp",
			"turns:turn.example.com:443?transport=tcp",
		},
		TurnSecret: "shared-turn-secret",
	}, nil)
	if err != nil {
		t.Fatalf("NewEngine: %v", err)
	}

	servers := e.ICEServersForUser("user-A")
	// Expect STUN + 1 TURN server entry (the TURN entry carries all 3 URLs).
	if len(servers) != 2 {
		t.Fatalf("expected STUN + TURN (2 entries), got %d", len(servers))
	}
	if len(servers[0].Username) != 0 {
		t.Errorf("STUN should have no username, got %q", servers[0].Username)
	}
	turn := servers[1]
	if len(turn.URLs) != 3 {
		t.Fatalf("expected 3 TURN URLs (UDP/TCP/TLS), got %d: %v", len(turn.URLs), turn.URLs)
	}
	wantURLs := []string{
		"turn:turn.example.com:3478",
		"turn:turn.example.com:3478?transport=tcp",
		"turns:turn.example.com:443?transport=tcp",
	}
	for i, w := range wantURLs {
		if turn.URLs[i] != w {
			t.Errorf("TURN URL[%d] = %q, want %q", i, turn.URLs[i], w)
		}
	}
	if turn.Username == "" || turn.Credential == "" {
		t.Fatal("expected ephemeral TURN username + credential")
	}
	if !contains(turn.Username, "user-A") {
		t.Errorf("TURN username %q should contain user-A", turn.Username)
	}
}

// TestEngine_TurnURLs_TakesPrecedenceOverHostPort verifies that when both
// TurnURLs and TurnHost/TurnPort are set, TurnURLs wins (no duplicate plain
// UDP URL is added).
func TestEngine_TurnURLs_TakesPrecedenceOverHostPort(t *testing.T) {
	e, err := NewEngine(Config{
		STUNURLs:    []string{"stun:stun.example.com:19302"},
		TurnEnabled: true,
		TurnHost:    "ignored.example.com",
		TurnPort:    9999,
		TurnURLs:    []string{"turn:turn.example.com:3478?transport=tcp"},
		TurnSecret:  "shared-turn-secret",
	}, nil)
	if err != nil {
		t.Fatalf("NewEngine: %v", err)
	}

	servers := e.ICEServersForUser("user-A")
	for _, s := range servers {
		for _, u := range s.URLs {
			if contains(u, "ignored.example.com") || contains(u, "9999") {
				t.Errorf("TurnHost/TurnPort should be ignored when TurnURLs is set, but found %q", u)
			}
		}
	}
}

// TestEngine_TurnURLs_StaticCreds verifies that static long-term credentials
// (no TurnSecret) are applied to every TURN URL when multiple are configured.
func TestEngine_TurnURLs_StaticCreds(t *testing.T) {
	e, err := NewEngine(Config{
		TurnEnabled: true,
		TurnURLs: []string{
			"turn:turn.example.com:3478",
			"turns:turn.example.com:443?transport=tcp",
		},
		TurnUser: "static-user",
		TurnPass: "static-pass",
	}, nil)
	if err != nil {
		t.Fatalf("NewEngine: %v", err)
	}

	static := e.ICEServers()
	// Each TURN URL becomes its own static ICEServer entry (no ephemeral cred
	// to bundle them under one username).
	turnCount := 0
	for _, s := range static {
		for _, u := range s.URLs {
			if contains(u, "turn:") || contains(u, "turns:") {
				turnCount++
			}
		}
		if s.Username != "" {
			if s.Username != "static-user" || s.Credential != "static-pass" {
				t.Errorf("static TURN creds = %q/%q, want static-user/static-pass", s.Username, s.Credential)
			}
		}
	}
	if turnCount != 2 {
		t.Errorf("expected 2 static TURN URLs, got %d", turnCount)
	}
}

// TestEngine_TurnURLs_FallsBackToHostPort verifies that when TurnURLs is NOT
// set, the engine derives a plain UDP turn:host:port URL (backward compat).
func TestEngine_TurnURLs_FallsBackToHostPort(t *testing.T) {
	e, err := NewEngine(Config{
		TurnEnabled: true,
		TurnHost:    "turn.example.com",
		TurnPort:    3478,
		TurnSecret:  "shared-turn-secret",
	}, nil)
	if err != nil {
		t.Fatalf("NewEngine: %v", err)
	}

	servers := e.ICEServersForUser("user-A")
	var turnURLs []string
	for _, s := range servers {
		if s.Username != "" {
			turnURLs = s.URLs
		}
	}
	if len(turnURLs) != 1 || turnURLs[0] != "turn:turn.example.com:3478" {
		t.Errorf("expected fallback to turn:turn.example.com:3478, got %v", turnURLs)
	}
}
