package config

import "testing"

func TestAnalyticsConfig(t *testing.T) {
	for _, key := range []string{"ANALYTICS_ENABLED", "ANALYTICS_BUFFER_SIZE", "ANALYTICS_BATCH_SIZE", "ANALYTICS_FLUSH_INTERVAL", "ANALYTICS_WRITE_TIMEOUT", "ANALYTICS_SHUTDOWN_TIMEOUT", "CLICKHOUSE_ADDR", "CLICKHOUSE_DATABASE"} {
		t.Setenv(key, "")
	}
	cfg, err := analyticsFromEnv()
	if err != nil || cfg.Enabled || cfg.Worker.BufferSize != 4096 {
		t.Fatal(cfg, err)
	}
	for _, tc := range []struct{ key, value string }{{"ANALYTICS_ENABLED", "yes"}, {"ANALYTICS_BUFFER_SIZE", "0"}, {"ANALYTICS_BATCH_SIZE", "1000000"}, {"ANALYTICS_FLUSH_INTERVAL", "-1s"}} {
		t.Run(tc.key, func(t *testing.T) {
			t.Setenv(tc.key, tc.value)
			if _, err := analyticsFromEnv(); err == nil {
				t.Fatal("accepted invalid config")
			}
		})
	}
	t.Setenv("ANALYTICS_ENABLED", "true")
	t.Setenv("CLICKHOUSE_ADDR", "https://localhost:8443")
	if _, err = analyticsFromEnv(); err != nil {
		t.Fatal("configuration must not connect", err)
	}
}
