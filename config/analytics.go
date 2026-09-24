package config

import (
	"errors"
	"os"
	"strconv"
	"time"

	"github.com/Kyrapatka/knowledge-platform/internal/platform/analytics"
)

type Analytics struct {
	Enabled         bool
	ClickHouse      analytics.ClickHouseConfig
	Worker          analytics.Options
	ShutdownTimeout time.Duration
}

func analyticsFromEnv() (Analytics, error) {
	var cfg Analytics
	enabled, err := choiceFromEnv("ANALYTICS_ENABLED", "false", []string{"true", "false"})
	if err != nil {
		return cfg, err
	}
	cfg.Enabled = enabled == "true"
	cfg.ClickHouse = analytics.ClickHouseConfig{Address: envOr("CLICKHOUSE_ADDR", "http://127.0.0.1:8123"), Database: envOr("CLICKHOUSE_DATABASE", "knowledge_analytics"), User: envOr("CLICKHOUSE_USER", "default"), Password: os.Getenv("CLICKHOUSE_PASSWORD")}
	cfg.Worker.AppVersion = envOr("APP_VERSION", "dev")
	if cfg.Worker.BufferSize, err = boundedInt("ANALYTICS_BUFFER_SIZE", 4096, 100000); err != nil {
		return cfg, err
	}
	if cfg.Worker.BatchSize, err = boundedInt("ANALYTICS_BATCH_SIZE", 256, 10000); err != nil {
		return cfg, err
	}
	if cfg.Worker.FlushInterval, err = durationFromEnv("ANALYTICS_FLUSH_INTERVAL", 5*time.Second); err != nil {
		return cfg, err
	}
	if cfg.Worker.WriteTimeout, err = durationFromEnv("ANALYTICS_WRITE_TIMEOUT", 5*time.Second); err != nil {
		return cfg, err
	}
	if cfg.ShutdownTimeout, err = durationFromEnv("ANALYTICS_SHUTDOWN_TIMEOUT", 10*time.Second); err != nil {
		return cfg, err
	}
	if cfg.Enabled {
		c, e := analytics.NewClickHouse(cfg.ClickHouse)
		if e != nil {
			return cfg, e
		}
		_ = c.Close()
	}
	return cfg, nil
}
func envOr(key, fallback string) string {
	if v := os.Getenv(key); v != "" {
		return v
	}
	return fallback
}
func boundedInt(key string, fallback, max int) (int, error) {
	v := os.Getenv(key)
	if v == "" {
		return fallback, nil
	}
	n, err := strconv.Atoi(v)
	if err != nil || n < 1 || n > max {
		return 0, errors.New(key + " is outside the supported positive range")
	}
	return n, nil
}
