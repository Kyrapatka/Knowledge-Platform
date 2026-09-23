package app

import (
	"net/http/httptest"
	"os"
	"testing"
	"time"

	"github.com/Kyrapatka/knowledge-platform/config"
	"github.com/google/uuid"
)

func TestTrainingRoutesAreWiredAndProtected(t *testing.T) {
	dsn := os.Getenv("TRAINING_TEST_DATABASE_URL")
	if dsn == "" {
		t.Skip("set TRAINING_TEST_DATABASE_URL")
	}
	a, err := New(config.Config{DatabaseURL: dsn, HTTPAddress: "127.0.0.1:0", JWTSecret: "training-integration-test-secret", JWTIssuer: "test", AccessTokenTTL: time.Minute, RefreshTokenTTL: time.Hour}, testLogger())
	if err != nil {
		t.Fatal(err)
	}
	defer a.Close()
	for _, path := range []string{"/health", "/live", "/ready"} {
		w := httptest.NewRecorder()
		a.server.Handler.ServeHTTP(w, httptest.NewRequest("GET", path, nil))
		if w.Code != 200 || w.Body.String() != `{"status":"ok"}` {
			t.Fatalf("%s: %d %s", path, w.Code, w.Body.String())
		}
	}
	t.Cleanup(func() {
		if err := a.Close(); err != nil {
			t.Error(err)
		}
	})

	id := uuid.New().String()
	for _, path := range []string{"/api/v1/training/plans", "/api/v1/training/sessions/" + id, "/api/v1/folders/" + id + "/training-config", "/api/v1/materials/" + id + "/exercises"} {
		r := httptest.NewRequest("GET", path, nil)
		w := httptest.NewRecorder()
		a.server.Handler.ServeHTTP(w, r)
		if w.Code != 401 {
			t.Fatalf("%s: expected auth middleware, got %d", path, w.Code)
		}
	}
}
