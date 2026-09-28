// Package httpmiddleware provides request correlation and safe HTTP access logs.
package httpmiddleware

import (
	"context"
	"errors"
	"log/slog"
	"net/http"
	"runtime/debug"
	"strings"
	"time"

	"github.com/gin-gonic/gin"
	"github.com/google/uuid"
)

const RequestIDHeader = "X-Request-ID"

type contextKey struct{}
type requestState struct {
	id        string
	logger    *slog.Logger
	stack     string
	recovered bool
}

// ID returns the server-generated ID, or an empty string outside HTTP requests.
func ID(ctx context.Context) string {
	if state, ok := ctx.Value(contextKey{}).(*requestState); ok {
		return state.id
	}
	return ""
}

// Logger returns the derived request logger, or the explicitly supplied fallback.
func Logger(ctx context.Context, fallback *slog.Logger) *slog.Logger {
	if state, ok := ctx.Value(contextKey{}).(*requestState); ok {
		return state.logger
	}
	return fallback
}

// RequestID always creates a fresh ID: client headers are untrusted input.
func RequestID(logger *slog.Logger) gin.HandlerFunc {
	return func(c *gin.Context) {
		id := uuid.NewString()
		state := &requestState{id: id, logger: logger.With(slog.String("request_id", id))}
		c.Request = c.Request.WithContext(context.WithValue(c.Request.Context(), contextKey{}, state))
		c.Header(RequestIDHeader, id)
		c.Next()
	}
}

// AccessLog must follow RequestID and precede Recovery. Only allowlisted fields
// are logged. Unmatched paths exclude the query; request data and error text are never serialized.
func AccessLog(logger *slog.Logger, thresholds ...time.Duration) gin.HandlerFunc {
	slow := time.Second
	if len(thresholds) > 0 && thresholds[0] > 0 {
		slow = thresholds[0]
	}
	return func(c *gin.Context) {
		start := time.Now()
		c.Next()
		route := c.FullPath()
		if route == "" {
			route = "unmatched"
		}
		method := c.Request.Method
		switch method {
		case "GET", "HEAD", "POST", "PUT", "PATCH", "DELETE", "CONNECT", "OPTIONS", "TRACE":
		default:
			method = "OTHER"
		}
		duration := time.Since(start)
		attrs := []slog.Attr{
			slog.String("method", method), slog.String("route", route),
			slog.Int("status", c.Writer.Status()),
			slog.Float64("duration_ms", float64(duration)/float64(time.Millisecond)),
			slog.Int("response_bytes", max(0, c.Writer.Size())),
		}
		canceled := errors.Is(c.Request.Context().Err(), context.Canceled)
		deadline := errors.Is(c.Request.Context().Err(), context.DeadlineExceeded)
		for _, entry := range c.Errors {
			canceled = canceled || errors.Is(entry.Err, context.Canceled)
			deadline = deadline || errors.Is(entry.Err, context.DeadlineExceeded)
		}
		level, reason := slog.LevelInfo, "normal"
		switch {
		case c.Writer.Status() >= 500 && !canceled && !deadline:
			level, reason = slog.LevelError, "server_error"
		case deadline:
			level, reason = slog.LevelWarn, "deadline_exceeded"
		case canceled:
			reason = "context_canceled"
		case route == "unmatched":
			level, reason = slog.LevelWarn, "unmatched"
		case c.Writer.Status() == 404 && strings.HasPrefix(c.Request.URL.Path, "/api/"):
			level, reason = slog.LevelWarn, "api_not_found"
		case duration >= slow:
			level, reason = slog.LevelWarn, "slow"
		}
		if route == "unmatched" || reason == "api_not_found" {
			attrs = append(attrs, slog.String("path", c.Request.URL.Path))
		}
		if state, ok := c.Request.Context().Value(contextKey{}).(*requestState); ok && state.recovered {
			level, reason = slog.LevelError, "panic"
			attrs = append(attrs, slog.Bool("panic_recovered", true), slog.String("panic", "recovered"), slog.String("stack", state.stack))
		}
		attrs = append(attrs, slog.String("reason", reason))
		Logger(c.Request.Context(), logger).LogAttrs(c.Request.Context(), level, "http request completed", attrs...)
	}
}

// Recovery suppresses Gin's unsafe request/panic dump. Panic diagnostics are
// attached to the single completion record, preserving Gin's broken-pipe handling.
func Recovery() gin.HandlerFunc {
	return gin.RecoveryWithWriter(nil, func(c *gin.Context, _ any) {
		if state, ok := c.Request.Context().Value(contextKey{}).(*requestState); ok {
			state.recovered = true
			state.stack = string(debug.Stack())
		}
		c.AbortWithStatus(http.StatusInternalServerError)
	})
}
