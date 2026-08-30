package voice

import "time"

// Config holds voice/SFU configuration. It is populated by the app's config
// layer (internal/config) from environment variables and passed into
// NewEngine, rather than read directly via os.Getenv here.
type Config struct {
	STUNURLs []string

	// TURN server (coturn or compatible).
	TurnEnabled bool
	TurnHost    string
	TurnPort    int
	// TurnStaticUser/Pass: legacy long-term credentials (static). Prefer
	// TurnSecret for ephemeral per-user credentials.
	TurnUser string
	TurnPass string
	// TurnSecret enables RFC 5766 TURN REST API ephemeral credentials.
	// When set, each join receives time-limited username/password bound to
	// the user (industry standard; coturn `use-auth-secret`).
	TurnSecret string
	// TurnCredentialTTL is the lifetime of ephemeral TURN credentials.
	// Defaults to 12h when zero.
	TurnCredentialTTL time.Duration

	// TurnURLs is an explicit list of TURN URI(s) to advertise to clients and
	// use for the SFU's own peer connections, e.g.
	// `turn:host:3478?transport=tcp`, `turns:host:443?transport=tcp`. When set
	// it takes precedence over TurnHost/TurnPort (which only build a plain
	// UDP `turn:host:port` URL). Use this to support TCP/TLS transports for
	// clients behind restrictive firewalls; RFC 5766 recommends advertising
	// multiple URIs. Credentials generated from TurnSecret apply to every URL.
	TurnURLs []string

	// MaxParticipants caps the number of concurrent participants per voice
	// channel. We default to 25 to bound SFU cost (O(n²) subscriber peer
	// connections), since each participant adds a bidirectional connection.
	MaxParticipants int
}
