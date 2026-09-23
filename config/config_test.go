package config

import (
	"testing"
	"time"
)

func TestShutdownTimeout(t *testing.T) {
	t.Setenv("DATABASE_URL", "unused")
	t.Setenv("JWT_SECRET", "test")
	t.Setenv("ACCESS_TOKEN_TTL", "")
	t.Setenv("REFRESH_TOKEN_TTL", "")
	t.Setenv("LOG_LEVEL", "")
	t.Setenv("LOG_FORMAT", "")
	for _, tc := range []struct {
		value   string
		want    time.Duration
		invalid bool
	}{{"", 15 * time.Second, false}, {"2m", 2 * time.Minute, false}, {"0s", 0, true}, {"-1s", 0, true}, {"oops", 0, true}} {
		t.Run(tc.value, func(t *testing.T) {
			t.Setenv("HTTP_SHUTDOWN_TIMEOUT", tc.value)
			cfg, err := Load()
			if (err != nil) != tc.invalid {
				t.Fatalf("error: %v", err)
			}
			if err == nil && cfg.HTTPShutdownTimeout != tc.want {
				t.Fatal(cfg.HTTPShutdownTimeout)
			}
		})
	}
}

func TestLoggingConfig(t *testing.T) {
	t.Setenv("DATABASE_URL", "unused")
	t.Setenv("JWT_SECRET", "test")
	t.Setenv("HTTP_SHUTDOWN_TIMEOUT", "")
	t.Setenv("ACCESS_TOKEN_TTL", "")
	t.Setenv("REFRESH_TOKEN_TTL", "")
	for _, tc := range []struct {
		level, format, wantLevel, wantFormat string
		invalid                              bool
	}{
		{"", "", "info", "text", false}, {"debug", "text", "debug", "text", false}, {"info", "json", "info", "json", false}, {"warn", "text", "warn", "text", false}, {"error", "json", "error", "json", false},
		{"test", "text", "", "", true}, {"info", "xml", "", "", true},
	} {
		t.Run(tc.level+"/"+tc.format, func(t *testing.T) {
			t.Setenv("LOG_LEVEL", tc.level)
			t.Setenv("LOG_FORMAT", tc.format)
			cfg, err := Load()
			if (err != nil) != tc.invalid {
				t.Fatalf("error=%v", err)
			}
			if err == nil && (cfg.LogLevel != tc.wantLevel || cfg.LogFormat != tc.wantFormat) {
				t.Fatalf("logging config=%s/%s", cfg.LogLevel, cfg.LogFormat)
			}
		})
	}
}
