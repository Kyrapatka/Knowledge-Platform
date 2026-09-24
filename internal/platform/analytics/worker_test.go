package analytics

import (
	"context"
	"errors"
	"io"
	"log/slog"
	"sync"
	"testing"
	"time"

	"github.com/Kyrapatka/knowledge-platform/internal/platform/metrics"
	"github.com/google/uuid"
	"github.com/prometheus/client_golang/prometheus"
	dto "github.com/prometheus/client_model/go"
)

type testSink struct {
	writes chan []Event
	block  <-chan struct{}
	err    error
	closed chan struct{}
}

func (s *testSink) Write(ctx context.Context, b []Event) error {
	s.writes <- append([]Event(nil), b...)
	if s.block != nil {
		select {
		case <-s.block:
		case <-ctx.Done():
			return ctx.Err()
		}
	}
	return s.err
}
func (s *testSink) Close() error { close(s.closed); return nil }
func counter(c prometheus.Counter) float64 {
	v := &dto.Metric{}
	_ = c.Write(v)
	return v.GetCounter().GetValue()
}
func worker(t *testing.T, sink Sink, buffer, batch int, interval time.Duration) (*Worker, *metrics.Metrics) {
	t.Helper()
	m := metrics.New()
	w, err := NewWorker(sink, Options{BufferSize: buffer, BatchSize: batch, FlushInterval: interval, WriteTimeout: time.Second, AppVersion: "test"}, m, slog.New(slog.NewJSONHandler(io.Discard, nil)))
	if err != nil {
		t.Fatal(err)
	}
	t.Cleanup(func() {
		ctx, cancel := context.WithTimeout(context.Background(), 2*time.Second)
		defer cancel()
		_ = w.Shutdown(ctx)
	})
	return w, m
}
func receive(t *testing.T, ch <-chan []Event) []Event {
	t.Helper()
	select {
	case b := <-ch:
		return b
	case <-time.After(3 * time.Second):
		t.Fatal("flush did not occur")
		return nil
	}
}
func TestBatchTimerAndShutdown(t *testing.T) {
	for _, reason := range []string{"batch", "timer", "shutdown"} {
		t.Run(reason, func(t *testing.T) {
			sink := &testSink{writes: make(chan []Event, 5), closed: make(chan struct{})}
			batch, interval := 2, time.Hour
			if reason == "timer" {
				interval = 10 * time.Millisecond
			}
			w, m := worker(t, sink, 10, batch, interval)
			w.Publish(context.Background(), New(UserRegistered, uuid.New()))
			if reason == "batch" {
				w.Publish(context.Background(), New(UserLoggedIn, uuid.New()))
			}
			if reason == "shutdown" {
				if err := w.Shutdown(context.Background()); err != nil {
					t.Fatal(err)
				}
			}
			b := receive(t, sink.writes)
			want := 1
			if reason == "batch" {
				want = 2
			}
			if len(b) != want || b[0].AppVersion != "test" {
				t.Fatal(b)
			}
			if counter(m.Published) != float64(want) {
				t.Fatal("accepted counter")
			}
		})
	}
}
func TestFullBufferNeverWaitsAndCountsDrops(t *testing.T) {
	release := make(chan struct{})
	sink := &testSink{writes: make(chan []Event, 5), closed: make(chan struct{}), block: release}
	w, m := worker(t, sink, 1, 1, time.Hour)
	w.Publish(context.Background(), New(UserRegistered, uuid.New()))
	receive(t, sink.writes)
	w.Publish(context.Background(), New(UserRegistered, uuid.New())) // fills queue while Write is blocked
	done := make(chan struct{})
	go func() { w.Publish(context.Background(), New(UserRegistered, uuid.New())); close(done) }()
	select {
	case <-done:
	case <-time.After(time.Second):
		t.Fatal("Publish blocked")
	}
	if counter(m.Dropped.WithLabelValues("buffer_full")) != 1 {
		t.Fatal("missing drop")
	}
	close(release)
	if err := w.Shutdown(context.Background()); err != nil {
		t.Fatal(err)
	}
	w.Publish(context.Background(), New(UserRegistered, uuid.New()))
	if counter(m.Dropped.WithLabelValues("closed")) != 1 {
		t.Fatal("closed publish not counted")
	}
}
func TestFailureDoesNotStopWorker(t *testing.T) {
	sink := &testSink{writes: make(chan []Event, 5), closed: make(chan struct{}), err: errors.New("secret server response")}
	w, m := worker(t, sink, 10, 1, time.Hour)
	for range 2 {
		w.Publish(context.Background(), New(UserRegistered, uuid.New()))
		receive(t, sink.writes)
	}
	if err := w.Shutdown(context.Background()); err != nil {
		t.Fatal(err)
	}
	if counter(m.FlushErrors) != 2 || counter(m.Dropped.WithLabelValues("flush_error")) != 2 {
		t.Fatal("failure counters")
	}
}
func TestShutdownCancelsBlockedFlush(t *testing.T) {
	sink := &testSink{writes: make(chan []Event, 5), closed: make(chan struct{}), block: make(chan struct{})}
	w, _ := worker(t, sink, 4, 1, time.Hour)
	w.Publish(context.Background(), New(UserRegistered, uuid.New()))
	receive(t, sink.writes)
	ctx, cancel := context.WithCancel(context.Background())
	cancel()
	if !errors.Is(w.Shutdown(ctx), context.Canceled) {
		t.Fatal("shutdown must return cancellation")
	}
	select {
	case <-w.done:
	case <-time.After(time.Second):
		t.Fatal("worker did not exit")
	}
	select {
	case <-sink.closed:
	default:
		t.Fatal("transport not closed")
	}
}
func TestConcurrentPublishAndShutdown(t *testing.T) {
	sink := &testSink{writes: make(chan []Event, 2000), closed: make(chan struct{})}
	w, _ := worker(t, sink, 32, 8, time.Hour)
	var wg sync.WaitGroup
	for range 10 {
		wg.Add(1)
		go func() {
			defer wg.Done()
			for range 100 {
				w.Publish(context.Background(), New(UserLoggedIn, uuid.New()))
			}
		}()
	}
	if err := w.Shutdown(context.Background()); err != nil {
		t.Fatal(err)
	}
	wg.Wait()
	NoopPublisher{}.Publish(context.Background(), Event{})
}
