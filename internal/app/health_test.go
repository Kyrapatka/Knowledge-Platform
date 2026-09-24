package app

import (
	"bytes"
	"context"
	"errors"
	"log/slog"
	"net/http"
	"net/http/httptest"
	"strings"
	"sync"
	"testing"

	"github.com/Kyrapatka/knowledge-platform/config"
	"github.com/Kyrapatka/knowledge-platform/internal/platform/httpmiddleware"
	"github.com/gin-gonic/gin"
)

func checkProbe(t *testing.T, handler http.Handler, path string, status int, value string) {
	t.Helper()
	w := httptest.NewRecorder()
	handler.ServeHTTP(w, httptest.NewRequest(http.MethodGet, path, nil))
	if w.Code != status || w.Body.String() != `{"status":"`+value+`"}` {
		t.Fatalf("%s: %d %s", path, w.Code, w.Body.String())
	}
	if !strings.HasPrefix(w.Header().Get("Content-Type"), "application/json") {
		t.Fatal("missing JSON content type")
	}
}

func TestReadinessState(t *testing.T) {
	var state Readiness
	if state.IsReady() {
		t.Fatal("zero value is ready")
	}
	for _, value := range []bool{true, false, false} {
		state.SetReady(value)
		if state.IsReady() != value {
			t.Fatalf("want readiness %v", value)
		}
	}
}

func TestHealthContractsWithoutDependencies(t *testing.T) {
	var logs bytes.Buffer
	logger := slog.New(slog.NewJSONHandler(&logs, nil))
	router := gin.New()
	router.Use(httpmiddleware.RequestID(logger), httpmiddleware.AccessLog(logger), httpmiddleware.Recovery())
	a := &App{logger: logger} // No database or server needed by health handlers.
	a.registerProbes(router)
	for _, ready := range []bool{false, true, false} {
		a.readiness.SetReady(ready)
		checkProbe(t, router, "/health", 200, "ok")
		checkProbe(t, router, "/live", 200, "ok")
		if ready {
			checkProbe(t, router, "/ready", 200, "ok")
		} else {
			checkProbe(t, router, "/ready", 503, "not_ready")
		}
	}
	if strings.Count(logs.String(), "\n") != 9 || strings.Count(logs.String(), `"msg":"http request completed"`) != 9 {
		t.Fatal("probes must emit access logs only")
	}
}

func TestConcurrentHealthReadsAndReadinessWrites(t *testing.T) {
	router := gin.New()
	a := &App{}
	a.registerProbes(router)
	start := make(chan struct{})
	var wg sync.WaitGroup
	wg.Add(1)
	go func() {
		defer wg.Done()
		<-start
		for i := 0; i < 1000; i++ {
			a.readiness.SetReady(i%2 == 0)
		}
		a.readiness.SetReady(false)
	}()
	for range 12 {
		wg.Add(1)
		go func() {
			defer wg.Done()
			<-start
			for range 100 {
				w := httptest.NewRecorder()
				router.ServeHTTP(w, httptest.NewRequest("GET", "/ready", nil))
				if !((w.Code == 200 && w.Body.String() == `{"status":"ok"}`) || (w.Code == 503 && w.Body.String() == `{"status":"not_ready"}`)) {
					t.Errorf("inconsistent response: %d %s", w.Code, w.Body.String())
				}
			}
		}()
	}
	close(start)
	wg.Wait()
	checkProbe(t, router, "/ready", 503, "not_ready")
}

func TestStartupDatabaseFailureDoesNotPublishApp(t *testing.T) {
	a, err := New(config.Config{DatabaseURL: "postgres://%invalid"}, testLogger())
	if err == nil || a != nil {
		t.Fatal("failed initialization published an App")
	}
}

func TestCleanupDisablesReadinessBeforeResources(t *testing.T) {
	for _, closeErr := range []error{nil, errors.New("close failed")} {
		a := &App{logger: testLogger()}
		a.readiness.SetReady(true)
		a.closeDatabase = func() error {
			if a.readiness.IsReady() {
				t.Error("database cleanup started while ready")
			}
			return closeErr
		}
		for range 2 {
			if err := a.Close(); !errors.Is(err, closeErr) {
				t.Fatal(err)
			}
			if a.readiness.IsReady() {
				t.Fatal("cleanup left readiness enabled")
			}
		}
	}
}

func TestShutdownBeforeRunNeverEnablesReadiness(t *testing.T) {
	a := &App{logger: testLogger(), server: newHTTPServer("127.0.0.1:0", http.NewServeMux())}
	a.readiness.SetReady(true)
	for range 2 {
		if err := a.Shutdown(context.Background()); err != nil {
			t.Fatal(err)
		}
	}
	if err := a.Run(); err != nil {
		t.Fatal(err)
	}
	if a.readiness.IsReady() {
		t.Fatal("readiness re-enabled after shutdown")
	}
}
