package app

import (
	"bytes"
	"context"
	"errors"
	"github.com/Kyrapatka/knowledge-platform/config"
	"github.com/gin-gonic/gin"
	"io"
	"log/slog"
	"net/http"
	"net/http/httptest"
	"os"
	"strings"
	"testing"
	"time"
)

func testLogger() *slog.Logger { return slog.New(slog.NewTextHandler(io.Discard, nil)) }
func TestNewRequiresLoggerBeforeOpeningDatabase(t *testing.T) {
	if _, err := New(config.Config{DatabaseURL: "must-not-connect"}, nil); err == nil || !strings.Contains(err.Error(), "logger") {
		t.Fatalf("nil logger: %v", err)
	}
}
func TestInfrastructureLogsAndErrorOwnership(t *testing.T) {
	var output bytes.Buffer
	a := &App{logger: slog.New(slog.NewJSONHandler(&output, nil)), server: newHTTPServer("", http.NewServeMux()), closeDatabase: func() error { return nil }}
	if err := a.Shutdown(context.Background()); err != nil {
		t.Fatal(err)
	}
	if err := a.Close(); err != nil {
		t.Fatal(err)
	}
	for _, message := range []string{"http server drained", "database closed"} {
		if strings.Count(output.String(), `"msg":"`+message+`"`) != 1 {
			t.Fatal(output.String())
		}
	}
	output.Reset()
	sentinel := errors.New("close failed")
	a.closeDatabase = func() error { return sentinel }
	if err := a.Close(); !errors.Is(err, sentinel) {
		t.Fatal(err)
	}
	if output.Len() != 0 {
		t.Fatal("App must return failure to its owner, not log it again")
	}
}

func TestGinLoggingKeepsAccessAndRecoveryWithoutCredentials(t *testing.T) {
	dsn := os.Getenv("TRAINING_TEST_DATABASE_URL")
	if dsn == "" {
		t.Skip("set TRAINING_TEST_DATABASE_URL")
	}
	var access, lifecycle bytes.Buffer
	previous := gin.DefaultWriter
	gin.DefaultWriter = &access
	defer func() { gin.DefaultWriter = previous }()
	log := slog.New(slog.NewJSONHandler(&lifecycle, nil))
	a, err := New(config.Config{DatabaseURL: dsn, HTTPAddress: "127.0.0.1:0", JWTSecret: "test-secret-with-at-least-32-characters", JWTIssuer: "test", AccessTokenTTL: time.Minute, RefreshTokenTTL: time.Hour}, log)
	if err != nil {
		t.Fatal(err)
	}
	defer a.Close()
	router := a.server.Handler.(*gin.Engine)
	const secret = "CREDENTIAL_MUST_NOT_APPEAR_IN_LOGS"
	router.GET("/_logging-private", func(c *gin.Context) { _ = c.Error(errors.New(secret)); c.Status(500) })
	router.POST("/_logging-panic", func(c *gin.Context) { panic(secret) })
	access.Reset()
	lifecycle.Reset()
	for _, path := range []string{"/_logging-private", "/_logging-panic"} {
		method := "GET"
		if path == "/_logging-panic" {
			method = "POST"
		}
		req := httptest.NewRequest(method, path+"?password="+secret, strings.NewReader(secret))
		req.Header.Set("Authorization", "Bearer "+secret)
		req.Header.Set("Cookie", "refresh_token="+secret)
		w := httptest.NewRecorder()
		router.ServeHTTP(w, req)
		if w.Code != 500 {
			t.Fatalf("lost recovery status: %d", w.Code)
		}
		if w.Header().Get("X-Request-ID") == "" {
			t.Fatal("missing request ID")
		}
	}
	if strings.Contains(access.String()+lifecycle.String(), secret) {
		t.Fatal("sensitive request/error/panic content leaked")
	}
	if access.Len() != 0 {
		t.Fatal("duplicate Gin access log")
	}
	if strings.Count(lifecycle.String(), `"msg":"http request completed"`) != 2 {
		t.Fatal("access logging lost")
	}
	if !strings.Contains(lifecycle.String(), `"panic_recovered":true`) || !strings.Contains(lifecycle.String(), `"stack":`) {
		t.Fatal("panic diagnostics lost")
	}
}
