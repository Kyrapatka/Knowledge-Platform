package app

import (
	"context"
	"errors"
	"io"
	"net"
	"net/http"
	"net/http/httptest"
	"sync"
	"testing"
	"time"

	"github.com/gin-gonic/gin"
)

func TestGracefulDrain(t *testing.T) {
	entered, release, finished := make(chan struct{}), make(chan struct{}), make(chan struct{})
	var once sync.Once
	defer once.Do(func() { close(release) })
	server := httptest.NewUnstartedServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		close(entered)
		select {
		case <-release:
		case <-r.Context().Done():
			t.Error("request canceled during drain")
		}
		w.Write([]byte("completed"))
		close(finished)
	}))
	a := &App{logger: testLogger(), server: server.Config, closeDatabase: func() error {
		select {
		case <-finished:
		default:
			t.Error("database closed before handler completed")
		}
		return nil
	}}
	a.readiness.SetReady(true)
	server.Start()
	defer func() { once.Do(func() { close(release) }); a.ForceClose(); server.Close() }()
	clientDone := make(chan error, 1)
	go func() {
		res, err := server.Client().Get(server.URL)
		if err == nil {
			defer res.Body.Close()
			var body []byte
			body, err = io.ReadAll(res.Body)
			if string(body) != "completed" {
				err = errors.New("incomplete response")
			}
		}
		clientDone <- err
	}()
	select {
	case <-entered:
	case <-time.After(3 * time.Second):
		t.Fatal("handler did not start")
	}
	ctx, cancel := context.WithTimeout(context.Background(), 3*time.Second)
	defer cancel()
	draining := make(chan struct{})
	a.server.RegisterOnShutdown(func() { close(draining) })
	drained := make(chan error, 1)
	go func() { drained <- a.Shutdown(ctx) }()
	select {
	case <-draining:
	case <-ctx.Done():
		t.Fatal(ctx.Err())
	}
	if a.readiness.IsReady() {
		t.Fatal("readiness not disabled before drain")
	}

	select {
	case err := <-drained:
		t.Fatalf("drain returned before handler: %v", err)
	default:
	}
	once.Do(func() { close(release) })
	if err := <-drained; err != nil {
		t.Fatal(err)
	}
	if err := <-clientDone; err != nil {
		t.Fatal(err)
	}
	if err := a.Close(); err != nil {
		t.Fatal(err)
	}
	if err := a.Run(); err != nil {
		t.Fatalf("ErrServerClosed must be normal: %v", err)
	}
}
func TestDrainDeadlineAndForceClose(t *testing.T) {
	entered, canceled := make(chan struct{}), make(chan struct{})
	server := httptest.NewUnstartedServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) { close(entered); <-r.Context().Done(); close(canceled) }))
	a := &App{logger: testLogger(), server: server.Config}
	a.readiness.SetReady(true)
	server.Start()
	defer func() { a.ForceClose(); server.Close() }()
	done := make(chan struct{})
	go func() {
		defer close(done)
		res, err := server.Client().Get(server.URL)
		if err == nil {
			res.Body.Close()
		}
	}()
	select {
	case <-entered:
	case <-time.After(3 * time.Second):
		t.Fatal("request not started")
	}
	ctx, cancel := context.WithDeadline(context.Background(), time.Now().Add(-time.Second))
	defer cancel()
	if err := a.Shutdown(ctx); !errors.Is(err, context.DeadlineExceeded) {
		t.Fatalf("expected deadline: %v", err)
	}
	if a.readiness.IsReady() {
		t.Fatal("still ready")
	}
	select {
	case <-canceled:
		t.Fatal("graceful shutdown canceled request")
	default:
	}
	if err := a.ForceClose(); err != nil {
		t.Fatal(err)
	}
	select {
	case <-canceled:
	case <-time.After(3 * time.Second):
		t.Fatal("force close did not cancel request")
	}
	<-done
}
func TestRunOccupiedPort(t *testing.T) {
	listener, err := net.Listen("tcp", "127.0.0.1:0")
	if err != nil {
		t.Fatal(err)
	}
	defer listener.Close()
	a := &App{logger: testLogger(), server: newHTTPServer(listener.Addr().String(), http.NewServeMux())}
	a.readiness.SetReady(true)
	if err := a.Run(); err == nil {
		t.Fatal("expected bind error")
	}
	if a.readiness.IsReady() {
		t.Fatal("failed server is ready")
	}
}
func TestReadinessProbes(t *testing.T) {
	router := gin.New()
	a := &App{logger: testLogger(), server: newHTTPServer("", router)}
	a.registerProbes(router)
	for _, ready := range []bool{false, true, false} {
		a.readiness.SetReady(ready)
		for _, path := range []string{"/live", "/ready"} {
			w := httptest.NewRecorder()
			router.ServeHTTP(w, httptest.NewRequest("GET", path, nil))
			want := 200
			if path == "/ready" && !ready {
				want = 503
			}
			if w.Code != want {
				t.Fatalf("%s: %d", path, w.Code)
			}
		}
	}
}
