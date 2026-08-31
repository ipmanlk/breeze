package config

import (
	"log/slog"
	"net/http"
	"os"
	"strconv"
	"strings"
	"time"

	"github.com/joho/godotenv"
)

type Config struct {
	AppEnv            string
	Port              string
	DBPath            string
	UploadDir         string
	JWTSecret         string
	CORSOrigins       []string
	SameSite          string   // lax (default), none, or strict
	TrustedProxyCIDRs []string // CIDR ranges of trusted reverse proxies
	// LogLevel controls structured logging verbosity: debug, info, warn, error.
	LogLevel slog.Level
	// MaxUploadSize is the maximum allowed request body size for file uploads
	// (avatars, task/message attachments) in bytes. 0 disables the limit.
	MaxUploadSize int64
	// AuditRetention is how long audit log entries are kept before purge.
	// 0 means retention purging is disabled (keep entries forever).
	AuditRetention time.Duration
	// AuditCleanupInterval is how often the retention purge runs.
	AuditCleanupInterval time.Duration
	WebSocket            WebSocketConfig
	Voice                VoiceConfig
	SMTP                 SMTPConfig
	VAPID                VAPIDConfig
}

// App environment values.
const (
	EnvDevelopment = "development"
	EnvProduction  = "production"
)

// SMTPConfig holds optional outbound email settings. When Host is empty the
// mailer is a no-op (air-gapped friendly) and all email-dependent features
// (password reset delivery, invite emails, email notifications) fall back to
// their existing server-side log behavior.
type SMTPConfig struct {
	Host     string
	Port     int
	User     string
	Pass     string
	From     string
	FromName string
	// AppURL is the public base URL used to build links in emails (e.g.
	// https://plume.example.com). Falls back to the request Host when empty.
	AppURL string
}

// VAPIDConfig holds Voluntary Application Server Identification keys used to
// authenticate Web Push requests (RFC 8292). When PublicKey is empty, browser
// push notifications are disabled (no subscriptions accepted, no push sent).
type VAPIDConfig struct {
	// PublicKey is the uncompressed P-256 public key, base64url (no padding).
	// Served to the browser for pushManager.subscribe({ applicationServerKey }).
	PublicKey string
	// PrivateKey is the P-256 private key, base64url (no padding).
	PrivateKey string
	// Subject is a contact URL or mailto: included in the VAPID JWT.
	Subject string
}

// WebSocketConfig holds WebSocket connection limits and typing-indicator
// behavior. These protect against resource exhaustion and broadcast
// amplification.
type WebSocketConfig struct {
	// MaxConnectionsPerUser caps concurrent WS connections per user across
	// all tabs/devices. 0 = unlimited. Defends against a single account
	// opening thousands of connections (memory exhaustion).
	MaxConnectionsPerUser int
	// MaxConnectionsGlobal caps total concurrent WS connections server-wide.
	// 0 = unlimited.
	MaxConnectionsGlobal int
	// TypingDebounce is the minimum interval between re-broadcasting a
	// typing_start event for the same user+conversation. Rapid duplicate
	// typing_start messages (e.g. on each keystroke from a chatty client)
	// are dropped to prevent N x M broadcast amplification. 0 = no debounce.
	TypingDebounce time.Duration
}

type VoiceConfig struct {
	STUNURLs    []string
	TurnEnabled bool
	TurnHost    string
	TurnPort    int
	TurnUser    string
	TurnPass    string
	// TurnSecret enables ephemeral per-user TURN credentials (RFC 5766 REST
	// API / coturn `use-auth-secret`). Preferred over static TurnUser/Pass.
	TurnSecret        string
	TurnCredentialTTL time.Duration
	// TurnURLs is an explicit list of TURN URI(s) to advertise to clients and
	// use for the SFU's own peer connections (e.g.
	// `turn:host:3478?transport=tcp`, `turns:host:443?transport=tcp`). When set
	// it takes precedence over TurnHost/TurnPort, which only build a plain
	// `turn:host:port` (UDP) URL. Use this to support TCP/TLS transports for
	// clients behind restrictive firewalls (RFC 5766 REST API recommends
	// advertising multiple URIs).
	TurnURLs []string
	// MaxParticipants caps concurrent participants per voice channel.
	MaxParticipants int
}

type LoadError struct {
	Missing []string
	Weak    []string
}

func (e *LoadError) Error() string {
	if len(e.Missing) > 0 {
		return "missing required environment variables: " + strings.Join(e.Missing, ", ")
	}
	return "insecure configuration: " + strings.Join(e.Weak, ", ")
}

func Load() (Config, error) {
	_ = godotenv.Load()

	var errs LoadError

	cfg := Config{
		AppEnv:            optional("APP_ENV", "development"),
		Port:              optional("PORT", "8080"),
		DBPath:            optional("DB_PATH", "./data/plume.db"),
		UploadDir:         optional("UPLOAD_DIR", "./data/uploads"),
		JWTSecret:         required("JWT_SECRET", &errs),
		CORSOrigins:       corsOrigins(),
		SameSite:          optional("COOKIE_SAME_SITE", "lax"),
		TrustedProxyCIDRs: proxyCIDRs(),
		LogLevel:          logLevel(),
		MaxUploadSize:     maxUploadSize(),
		Voice:             voiceConfig(),
		SMTP:              smtpConfig(),
		VAPID:             vapidConfig(),
	}

	// Enforce JWT secret strength: a short or placeholder secret allows offline
	// brute-force of token signatures, enabling session forgery. 32 bytes is
	// the OWASP minimum for HMAC-SHA256 (the algorithm used in internal/auth).
	if cfg.JWTSecret != "" && len(cfg.JWTSecret) < minJWTSecretBytes {
		errs.Weak = append(errs.Weak, "JWT_SECRET must be at least 32 characters (use a random secret)")
	}
	if cfg.JWTSecret == "change-me-to-a-random-secret" {
		errs.Weak = append(errs.Weak, "JWT_SECRET is set to the example placeholder; set a unique random secret")
	}

	// Audit log retention. 0 = keep forever.
	if r := os.Getenv("AUDIT_RETENTION"); r != "" {
		if d, err := time.ParseDuration(r); err == nil && d > 0 {
			cfg.AuditRetention = d
		}
	}
	if i := os.Getenv("AUDIT_CLEANUP_INTERVAL"); i != "" {
		if d, err := time.ParseDuration(i); err == nil && d > 0 {
			cfg.AuditCleanupInterval = d
		}
	}
	cfg.WebSocket = webSocketConfig()

	if len(errs.Missing) > 0 || len(errs.Weak) > 0 {
		return Config{}, &errs
	}

	return cfg, nil
}

func (c *Config) AuthCookieSameSite() http.SameSite {
	switch strings.ToLower(c.SameSite) {
	case "none":
		return http.SameSiteNoneMode
	case "strict":
		return http.SameSiteStrictMode
	default:
		return http.SameSiteLaxMode
	}
}

func corsOrigins() []string {
	raw := os.Getenv("CORS_ORIGINS")
	if raw == "" {
		return []string{"http://localhost:5173", "http://localhost:4173"}
	}
	return strings.Split(raw, ",")
}

func proxyCIDRs() []string {
	raw := os.Getenv("TRUSTED_PROXY_CIDRS")
	if raw == "" {
		return nil
	}
	return strings.Split(raw, ",")
}

func required(key string, errs *LoadError) string {
	v := os.Getenv(key)
	if v == "" {
		errs.Missing = append(errs.Missing, key)
	}
	return v
}
func optional(key, defaultVal string) string {
	if v := os.Getenv(key); v != "" {
		return v
	}
	return defaultVal
}

// boolEnv reads a boolean environment variable using strconv.ParseBool, the
// Go stdlib convention (accepts 1, t, T, TRUE, true, True, 0, f, F, FALSE,
// false, False). Falls back to defaultVal when unset or unparseable. This is
// the single, consistent way boolean env vars are parsed across the app;
// prefer it over `== "true"` string checks so operators can use any of the
// accepted forms (1/0 is common in 12-factor/docker setups).
func boolEnv(key string, defaultVal bool) bool {
	if v, err := strconv.ParseBool(os.Getenv(key)); err == nil {
		return v
	}
	return defaultVal
}

// minJWTSecretBytes is the minimum length enforced for JWT_SECRET. 32 bytes
// matches the HMAC-SHA256 output size and is the OWASP recommendation.
const minJWTSecretBytes = 32

func voiceConfig() VoiceConfig {
	cfg := VoiceConfig{
		STUNURLs:        []string{"stun:stun.l.google.com:19302"},
		TurnEnabled:     boolEnv("TURN_ENABLED", false),
		TurnHost:        "localhost",
		TurnPort:        3478,
		MaxParticipants: 25, // practical default to bound SFU cost
	}
	if urls := os.Getenv("STUN_URLS"); urls != "" {
		cfg.STUNURLs = strings.Split(urls, ",")
	}
	if host := os.Getenv("TURN_HOST"); host != "" {
		cfg.TurnHost = host
	}
	if port := os.Getenv("TURN_PORT"); port != "" {
		if p, err := strconv.Atoi(port); err == nil {
			cfg.TurnPort = p
		}
	}
	cfg.TurnUser = os.Getenv("TURN_USER")
	cfg.TurnPass = os.Getenv("TURN_PASS")
	// Ephemeral credential secret (coturn `static-auth-secret`). When set,
	// each voice join receives time-limited, per-user TURN credentials.
	cfg.TurnSecret = os.Getenv("TURN_SECRET")
	if ttl := os.Getenv("TURN_CREDENTIAL_TTL"); ttl != "" {
		if d, err := time.ParseDuration(ttl); err == nil {
			cfg.TurnCredentialTTL = d
		}
	}
	// Explicit TURN URI list; supports TCP/TLS transports
	// (turn:host:3478?transport=tcp, turns:host:443?transport=tcp). When set,
	// takes precedence over the TurnHost/TurnPort-derived UDP URL.
	if urls := os.Getenv("TURN_URLS"); urls != "" {
		for _, u := range strings.Split(urls, ",") {
			if u = strings.TrimSpace(u); u != "" {
				cfg.TurnURLs = append(cfg.TurnURLs, u)
			}
		}
	}
	if max := os.Getenv("VOICE_MAX_PARTICIPANTS"); max != "" {
		if n, err := strconv.Atoi(max); err == nil && n > 0 {
			cfg.MaxParticipants = n
		}
	}
	return cfg
}

// webSocketConfig loads WebSocket connection limits and typing debounce.
// All are optional with sane defaults; 0 disables the corresponding limit.
func webSocketConfig() WebSocketConfig {
	cfg := WebSocketConfig{
		MaxConnectionsPerUser: 10,   // multi-tab friendly; caps abuse
		MaxConnectionsGlobal:  5000, // per-instance backstop
		TypingDebounce:        1 * time.Second,
	}
	if n := os.Getenv("WS_MAX_CONNECTIONS_PER_USER"); n != "" {
		if v, err := strconv.Atoi(n); err == nil && v >= 0 {
			cfg.MaxConnectionsPerUser = v
		}
	}
	if n := os.Getenv("WS_MAX_CONNECTIONS_GLOBAL"); n != "" {
		if v, err := strconv.Atoi(n); err == nil && v >= 0 {
			cfg.MaxConnectionsGlobal = v
		}
	}
	if d := os.Getenv("WS_TYPING_DEBOUNCE"); d != "" {
		if v, err := time.ParseDuration(d); err == nil && v >= 0 {
			cfg.TypingDebounce = v
		}
	}
	return cfg
}

// smtpConfig loads optional outbound email settings. When SMTP_HOST is unset
// the returned config has an empty Host, which the mailer treats as disabled
// (no-op) so the app stays air-gapped friendly.
func smtpConfig() SMTPConfig {
	cfg := SMTPConfig{
		Host:     os.Getenv("SMTP_HOST"),
		User:     os.Getenv("SMTP_USER"),
		Pass:     os.Getenv("SMTP_PASS"),
		From:     os.Getenv("SMTP_FROM"),
		FromName: optional("SMTP_FROM_NAME", "Plume"),
		AppURL:   strings.TrimRight(os.Getenv("APP_URL"), "/"),
		Port:     587,
	}
	if port := os.Getenv("SMTP_PORT"); port != "" {
		if p, err := strconv.Atoi(port); err == nil && p > 0 {
			cfg.Port = p
		}
	}
	return cfg
}

// vapidConfig loads optional Web Push VAPID keys. When VAPID_PUBLIC_KEY is
// unset browser push is disabled (the UI hides the opt-in and the backend
// rejects subscriptions).
func vapidConfig() VAPIDConfig {
	return VAPIDConfig{
		PublicKey:  os.Getenv("VAPID_PUBLIC_KEY"),
		PrivateKey: os.Getenv("VAPID_PRIVATE_KEY"),
		Subject:    optional("VAPID_SUBJECT", "mailto:noreply@plume.local"),
	}
}

// logLevel parses LOG_LEVEL (debug/info/warn/error). Unset or unknown values
// fall back to info in production and debug in development for safe defaults.
func logLevel() slog.Level {
	raw := strings.ToLower(os.Getenv("LOG_LEVEL"))
	switch raw {
	case "debug":
		return slog.LevelDebug
	case "warn", "warning":
		return slog.LevelWarn
	case "error":
		return slog.LevelError
	case "info":
		return slog.LevelInfo
	case "":
		if optional("APP_ENV", "development") == EnvProduction {
			return slog.LevelInfo
		}
		return slog.LevelDebug
	default:
		return slog.LevelInfo
	}
}

// maxUploadSize parses MAX_UPLOAD_SIZE in bytes. It accepts plain integers and
// SI suffixes (K, M, G, KB, MB, GB) to match common ops conventions. A zero or
// negative value disables the upload limit; the default is 50 MiB to align
// with the documented configuration reference.
func maxUploadSize() int64 {
	raw := strings.TrimSpace(os.Getenv("MAX_UPLOAD_SIZE"))
	if raw == "" {
		return 50 << 20
	}

	// Plain integer bytes.
	if n, err := strconv.ParseInt(raw, 10, 64); err == nil {
		return n
	}

	var mult int64 = 1
	switch {
	case strings.HasSuffix(raw, "GB"):
		mult = 1 << 30
		raw = strings.TrimSuffix(raw, "GB")
	case strings.HasSuffix(raw, "MB"):
		mult = 1 << 20
		raw = strings.TrimSuffix(raw, "MB")
	case strings.HasSuffix(raw, "KB"):
		mult = 1 << 10
		raw = strings.TrimSuffix(raw, "KB")
	case strings.HasSuffix(raw, "G"):
		mult = 1 << 30
		raw = strings.TrimSuffix(raw, "G")
	case strings.HasSuffix(raw, "M"):
		mult = 1 << 20
		raw = strings.TrimSuffix(raw, "M")
	case strings.HasSuffix(raw, "K"):
		mult = 1 << 10
		raw = strings.TrimSuffix(raw, "K")
	}

	raw = strings.TrimSpace(raw)
	n, err := strconv.ParseInt(raw, 10, 64)
	if err != nil {
		return 50 << 20
	}
	return n * mult
}

// UploadSizeLimit returns a positive upload limit for use with middleware.
// A configured value of 0 means "unlimited"; this helper returns math.MaxInt64
// in that case so callers can still pass a numeric cap to MaxBytesReader.
func (c *Config) UploadSizeLimit() int64 {
	if c.MaxUploadSize <= 0 {
		return int64(^uint(0) >> 1)
	}
	return c.MaxUploadSize
}
