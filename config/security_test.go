package config

import "testing"

func TestHTTPHardeningConfigValidation(t *testing.T) {
	t.Setenv("DATABASE_URL", "test")
	t.Setenv("JWT_SECRET", "test-secret-with-at-least-32-characters")
	for _, tt := range []struct{ key, value string }{
		{"HTTP_TRUSTED_PROXIES", "*"}, {"HTTP_PUBLIC_ORIGIN", "https://user:secret@example.com"},
		{"HTTP_PUBLIC_ORIGIN", "https://example.com/path"}, {"HTTP_SLOW_REQUEST_THRESHOLD", "0s"},
		{"AUTH_LOGIN_LIMIT", "0"}, {"AUTH_RATE_MAX_KEYS", "100001"}, {"AUTH_RATE_WINDOW", "no"},
	} {
		t.Run(tt.key+tt.value, func(t *testing.T) {
			t.Setenv(tt.key, tt.value)
			if _, err := Load(); err == nil {
				t.Fatal("invalid configuration accepted")
			}
		})
	}
}
