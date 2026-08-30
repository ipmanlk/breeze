package config

import (
	"errors"
	"log/slog"
	"os"
	"testing"
)

func TestLoad_RequiresJWTSecret(t *testing.T) {
	os.Unsetenv("JWT_SECRET")
	_, err := Load()
	if err == nil {
		t.Fatal("Load() with no JWT_SECRET: want error, got nil")
	}
	var le *LoadError
	if !errors.As(err, &le) {
		t.Fatalf("error type = %T, want *LoadError", err)
	}
	if len(le.Missing) == 0 || le.Missing[0] != "JWT_SECRET" {
		t.Errorf("Missing = %v, want [JWT_SECRET]", le.Missing)
	}
}

func TestLoad_RejectsShortJWTSecret(t *testing.T) {
	t.Setenv("JWT_SECRET", "too-short")
	_, err := Load()
	if err == nil {
		t.Fatal("Load() with short JWT_SECRET: want error, got nil")
	}
	var le *LoadError
	if !errors.As(err, &le) {
		t.Fatalf("error type = %T, want *LoadError", err)
	}
	if len(le.Weak) == 0 {
		t.Errorf("Weak = %v, want at least one strength warning", le.Weak)
	}
}

func TestLoad_RejectsPlaceholderJWTSecret(t *testing.T) {
	t.Setenv("JWT_SECRET", "change-me-to-a-random-secret")
	_, err := Load()
	if err == nil {
		t.Fatal("Load() with placeholder JWT_SECRET: want error, got nil")
	}
}

func TestLoad_AcceptsStrongJWTSecret(t *testing.T) {
	t.Setenv("JWT_SECRET", "a-very-long-random-secret-that-is-definitely-32+bytes-long")
	cfg, err := Load()
	if err != nil {
		t.Fatalf("Load() with strong secret: unexpected error %v", err)
	}
	if cfg.JWTSecret != "a-very-long-random-secret-that-is-definitely-32+bytes-long" {
		t.Errorf("JWTSecret = %q", cfg.JWTSecret)
	}
}

func TestBoolEnv(t *testing.T) {
	cases := []struct {
		name string
		set  string
		def  bool
		want bool
	}{
		// strconv.ParseBool accepts: 1, t, T, TRUE, true, True, 0, f, F, FALSE, false, False
		{"true", "true", false, true},
		{"True", "True", false, true},
		{"TRUE", "TRUE", false, true},
		{"1", "1", false, true},
		{"t", "t", false, true},
		{"false", "false", true, false},
		{"0", "0", true, false},
		{"empty uses default", "", true, true},
		{"garbage uses default", "yes", true, true},
	}
	for _, tc := range cases {
		t.Run(tc.name, func(t *testing.T) {
			if tc.set == "" {
				os.Unsetenv("TEST_BOOL_FLAG")
			} else {
				t.Setenv("TEST_BOOL_FLAG", tc.set)
			}
			if got := boolEnv("TEST_BOOL_FLAG", tc.def); got != tc.want {
				t.Errorf("boolEnv(%q) = %v, want %v", tc.set, got, tc.want)
			}
		})
	}
}

func TestVoiceConfig_TurnEnabledAcceptsBoolForms(t *testing.T) {
	// TURN_ENABLED previously only accepted the literal "true". Now it uses
	// strconv.ParseBool, so operators can use 1/0, True/False, etc.
	for _, val := range []string{"true", "True", "TRUE", "1", "t"} {
		t.Run("enable_"+val, func(t *testing.T) {
			t.Setenv("TURN_ENABLED", val)
			t.Setenv("JWT_SECRET", "a-very-long-random-secret-that-is-definitely-32+bytes-long")
			cfg, err := Load()
			if err != nil {
				t.Fatalf("Load: %v", err)
			}
			if !cfg.Voice.TurnEnabled {
				t.Errorf("TURN_ENABLED=%q: want TurnEnabled=true", val)
			}
		})
	}
	for _, val := range []string{"false", "0", "False", ""} {
		t.Run("disable_"+val, func(t *testing.T) {
			if val == "" {
				os.Unsetenv("TURN_ENABLED")
			} else {
				t.Setenv("TURN_ENABLED", val)
			}
			t.Setenv("JWT_SECRET", "a-very-long-random-secret-that-is-definitely-32+bytes-long")
			cfg, err := Load()
			if err != nil {
				t.Fatalf("Load: %v", err)
			}
			if cfg.Voice.TurnEnabled {
				t.Errorf("TURN_ENABLED=%q: want TurnEnabled=false", val)
			}
		})
	}
}

func TestLogLevel(t *testing.T) {
	cases := []struct {
		name   string
		env    string
		appEnv string
		want   slog.Level
	}{
		{"debug", "debug", "production", slog.LevelDebug},
		{"info", "info", "production", slog.LevelInfo},
		{"warn", "warn", "production", slog.LevelWarn},
		{"error", "error", "production", slog.LevelError},
		{"warning", "warning", "production", slog.LevelWarn},
		{"empty prod defaults to info", "", "production", slog.LevelInfo},
		{"empty dev defaults to debug", "", "development", slog.LevelDebug},
		{"unknown defaults to info", "verbose", "production", slog.LevelInfo},
	}
	for _, tc := range cases {
		t.Run(tc.name, func(t *testing.T) {
			if tc.env == "" {
				os.Unsetenv("LOG_LEVEL")
			} else {
				t.Setenv("LOG_LEVEL", tc.env)
			}
			t.Setenv("APP_ENV", tc.appEnv)
			if got := logLevel(); got != tc.want {
				t.Errorf("logLevel() = %v, want %v", got, tc.want)
			}
		})
	}
}

func TestMaxUploadSize(t *testing.T) {
	cases := []struct {
		name string
		env  string
		want int64
	}{
		{"empty default", "", 50 << 20},
		{"plain bytes", "1048576", 1 << 20},
		{"KB suffix", "1024KB", 1 << 20},
		{"MB suffix", "10MB", 10 << 20},
		{"GB suffix", "1GB", 1 << 30},
		{"K suffix", "1024K", 1 << 20},
		{"M suffix", "5M", 5 << 20},
		{"G suffix", "2G", 2 << 30},
		{"spaces trimmed", " 10MB ", 10 << 20},
		{"garbage default", "big", 50 << 20},
	}
	for _, tc := range cases {
		t.Run(tc.name, func(t *testing.T) {
			if tc.env == "" {
				os.Unsetenv("MAX_UPLOAD_SIZE")
			} else {
				t.Setenv("MAX_UPLOAD_SIZE", tc.env)
			}
			if got := maxUploadSize(); got != tc.want {
				t.Errorf("maxUploadSize() = %d, want %d", got, tc.want)
			}
		})
	}
}

func TestConfig_UploadSizeLimit(t *testing.T) {
	if got := (&Config{MaxUploadSize: 10 << 20}).UploadSizeLimit(); got != 10<<20 {
		t.Errorf("UploadSizeLimit() = %d, want %d", got, 10<<20)
	}
	// Zero means unlimited; ensure we still return a usable MaxInt64 sentinel.
	if got := (&Config{MaxUploadSize: 0}).UploadSizeLimit(); got <= 0 {
		t.Errorf("UploadSizeLimit() with MaxUploadSize=0 should be positive, got %d", got)
	}
}

func TestVoiceConfig_TurnURLsParsed(t *testing.T) {
	t.Setenv("JWT_SECRET", "a-very-long-random-secret-that-is-definitely-32+bytes-long")
	t.Setenv("TURN_ENABLED", "true")
	t.Setenv("TURN_URLS", "turn:turn.example.com:3478, turn:turn.example.com:3478?transport=tcp , turns:turn.example.com:443?transport=tcp")
	cfg, err := Load()
	if err != nil {
		t.Fatalf("Load: %v", err)
	}
	want := []string{
		"turn:turn.example.com:3478",
		"turn:turn.example.com:3478?transport=tcp",
		"turns:turn.example.com:443?transport=tcp",
	}
	if len(cfg.Voice.TurnURLs) != len(want) {
		t.Fatalf("TurnURLs = %v, want %v", cfg.Voice.TurnURLs, want)
	}
	for i, w := range want {
		if cfg.Voice.TurnURLs[i] != w {
			t.Errorf("TurnURLs[%d] = %q, want %q", i, cfg.Voice.TurnURLs[i], w)
		}
	}
}
