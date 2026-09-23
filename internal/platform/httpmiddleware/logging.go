// Package httpmiddleware provides request correlation and safe HTTP access logs.
package httpmiddleware

import (
	"context"
	"log/slog"
	"net/http"
	"runtime/debug"
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
// are logged; notably raw URLs, request data and c.Errors are never serialized.
func AccessLog(logger *slog.Logger) gin.HandlerFunc {
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
		attrs := []slog.Attr{
			slog.String("method", method), slog.String("route", route),
			slog.Int("status", c.Writer.Status()),
			slog.Float64("duration_ms", float64(time.Since(start))/float64(time.Millisecond)),
			slog.Int("response_bytes", max(0, c.Writer.Size())),
		}
		level := slog.LevelInfo
		if c.Writer.Status() >= 500 {
			level = slog.LevelError
		}
		if state, ok := c.Request.Context().Value(contextKey{}).(*requestState); ok && state.recovered {
			level = slog.LevelError
			attrs = append(attrs, slog.Bool("panic_recovered", true), slog.String("stack", state.stack))
		}
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
