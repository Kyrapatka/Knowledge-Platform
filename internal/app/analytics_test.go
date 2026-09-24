package app

import (
	"context"
	"errors"
	"net/http/httptest"
	"os"
	"reflect"
	"strings"
	"testing"
	"time"

	"github.com/Kyrapatka/knowledge-platform/config"
	"github.com/Kyrapatka/knowledge-platform/internal/platform/analytics"
)

func TestAnalyticsCleanupBeforePostgresEvenOnFailure(t *testing.T) {
	var calls []string
	a := &App{logger: testLogger(), closeAnalytics: func() error { calls = append(calls, "analytics"); return context.DeadlineExceeded }, closeDatabase: func() error { calls = append(calls, "postgres"); return nil }}
	a.readiness.SetReady(true)
	if err := a.Close(); err != nil {
		t.Fatal(err)
	}
	if a.readiness.IsReady() || !reflect.DeepEqual(calls, []string{"analytics", "postgres"}) {
		t.Fatal(calls)
	}
}
func TestOptionalAnalyticsAndMetricsWiring(t *testing.T) {
	dsn := os.Getenv("TRAINING_TEST_DATABASE_URL")
	if dsn == "" {
		t.Skip("set TRAINING_TEST_DATABASE_URL")
	}
	cfg := config.Config{DatabaseURL: dsn, JWTSecret: "test-secret-with-at-least-32-characters", JWTIssuer: "test", AccessTokenTTL: time.Minute, RefreshTokenTTL: time.Hour}
	for _, enabled := range []bool{false, true} {
		cfg.Analytics = config.Analytics{Enabled: enabled, ClickHouse: analytics.ClickHouseConfig{Address: "http://127.0.0.1:1", Database: "analytics"}, Worker: analytics.Options{BufferSize: 2, BatchSize: 2, FlushInterval: time.Hour, WriteTimeout: time.Second}, ShutdownTimeout: time.Second}
		a, err := New(cfg, testLogger())
		if err != nil {
			t.Fatal("optional ClickHouse blocked startup", err)
		}
		checkProbe(t, a.server.Handler, "/ready", 200, "ok")
		w := httptest.NewRecorder()
		a.server.Handler.ServeHTTP(w, httptest.NewRequest("GET", "/metrics", nil))
		if w.Code != 200 || !strings.Contains(w.Body.String(), "http_requests_total") {
			t.Fatal("metrics not wired")
		}
		if err = a.Shutdown(context.Background()); err != nil {
			t.Fatal(err)
		}
		if err = a.Close(); err != nil && !errors.Is(err, context.DeadlineExceeded) {
			t.Fatal(err)
		}
	}
}
