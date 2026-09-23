// Package logger configures the application's single structured logger.
package logger

import (
	"errors"
	"io"
	"log/slog"
	"os"
)

type Config struct {
	Level  string
	Format string
	// Output defaults to stderr. Supplying a writer supports deterministic tests
	// and lets the process owner choose its destination without global state.
	Output io.Writer
}

func New(cfg Config) (*slog.Logger, error) {
	var level slog.Level
	switch cfg.Level {
	case "debug":
		level = slog.LevelDebug
	case "info":
		level = slog.LevelInfo
	case "warn":
		level = slog.LevelWarn
	case "error":
		level = slog.LevelError
	default:
		return nil, errors.New("LOG_LEVEL must be debug, info, warn or error")
	}
	output := cfg.Output
	if output == nil {
		output = os.Stderr
	}
	options := &slog.HandlerOptions{Level: level}
	var handler slog.Handler
	switch cfg.Format {
	case "text":
		handler = slog.NewTextHandler(output, options)
	case "json":
		handler = slog.NewJSONHandler(output, options)
	default:
		return nil, errors.New("LOG_FORMAT must be text or json")
	}
	return slog.New(handler).With(slog.String("service", "knowledge-platform")), nil
}
