package main

import (
	"bytes"
	"context"
	"errors"
	"log/slog"
	"reflect"
	"strings"
	"testing"
	"time"
)

type fakeApplication struct {
	calls                                   []string
	stopped                                 chan struct{}
	runErr, shutdownErr, forceErr, closeErr error
}

func (a *fakeApplication) Run() error {
	if a.runErr != nil {
		return a.runErr
	}
	<-a.stopped
	return nil
}
func (a *fakeApplication) Shutdown(ctx context.Context) error {
	if ctx.Err() != nil {
		return errors.New("drain inherited canceled signal")
	}
	a.calls = append(a.calls, "shutdown")
	close(a.stopped)
	return a.shutdownErr
}
func (a *fakeApplication) ForceClose() error { a.calls = append(a.calls, "force"); return a.forceErr }
func (a *fakeApplication) Close() error      { a.calls = append(a.calls, "database"); return a.closeErr }
func TestLifecycle(t *testing.T) {
	serverErr, forceErr, dbErr := errors.New("bind"), errors.New("force"), errors.New("database")
	for _, tc := range []struct {
		name                     string
		run, shutdown, force, db error
		want                     []string
	}{
		{name: "signal", want: []string{"shutdown", "database"}},
		{name: "startup failure", run: serverErr, want: []string{"shutdown", "database"}},
		{name: "timeout and cleanup errors", shutdown: context.DeadlineExceeded, force: forceErr, db: dbErr, want: []string{"shutdown", "force", "database"}},
	} {
		t.Run(tc.name, func(t *testing.T) {
			a := &fakeApplication{stopped: make(chan struct{}), runErr: tc.run, shutdownErr: tc.shutdown, forceErr: tc.force, closeErr: tc.db}
			ctx, cancel := context.WithCancel(context.Background())
			defer cancel()
			if tc.run == nil {
				cancel()
			}
			var logs bytes.Buffer
			logger := slog.New(slog.NewJSONHandler(&logs, nil))
			err := serve(ctx, a, time.Second, logger)
			for _, cause := range []error{tc.run, tc.shutdown, tc.force, tc.db} {
				if cause != nil && !errors.Is(err, cause) {
					t.Errorf("lost error %v in %v", cause, err)
				}
			}
			if tc.run == nil && tc.shutdown == nil && tc.db == nil && err != nil {
				t.Fatal(err)
			}
			if !reflect.DeepEqual(a.calls, tc.want) {
				t.Fatalf("order %v", a.calls)
			}
			for _, message := range []string{"shutdown started", "application stopped"} {
				if strings.Count(logs.String(), `"msg":"`+message+`"`) != 1 {
					t.Fatalf("missing/duplicate lifecycle event: %s", logs.String())
				}
			}
			for _, message := range []struct {
				err error
				msg string
			}{{tc.run, "http server failed"}, {tc.force, "failed to force close http server"}, {tc.db, "failed to close database"}} {
				want := 0
				if message.err != nil {
					want = 1
				}
				if strings.Count(logs.String(), `"msg":"`+message.msg+`"`) != want {
					t.Fatalf("duplicate/missing error: %s", logs.String())
				}
			}
			if tc.shutdown == context.DeadlineExceeded && !strings.Contains(logs.String(), `"level":"WARN"`) {
				t.Fatal("timeout must warn before force close")
			}
		})
	}
}

func TestRunRejectsInvalidLoggingBeforeConnecting(t *testing.T) {
	t.Setenv("DATABASE_URL", "must-not-connect")
	t.Setenv("JWT_SECRET", "test")
	t.Setenv("HTTP_SHUTDOWN_TIMEOUT", "")
	t.Setenv("ACCESS_TOKEN_TTL", "")
	t.Setenv("REFRESH_TOKEN_TTL", "")
	for _, tc := range []struct{ level, format string }{{"test", "text"}, {"info", "invalid"}} {
		t.Setenv("LOG_LEVEL", tc.level)
		t.Setenv("LOG_FORMAT", tc.format)
		if code := run(); code != 1 {
			t.Fatalf("invalid configuration exited %d", code)
		}
	}
}
