package analytics

import (
	"context"
	"errors"
	"log/slog"
	"sync"
	"sync/atomic"
	"time"

	"github.com/Kyrapatka/knowledge-platform/internal/platform/metrics"
)

type Publisher interface{ Publish(context.Context, Event) }
type NoopPublisher struct{}

func (NoopPublisher) Publish(context.Context, Event) {}

// Sink implementations must honor cancellation and have bounded Close.
type Sink interface {
	Write(context.Context, []Event) error
	Close() error
}
type Options struct {
	BufferSize, BatchSize       int
	FlushInterval, WriteTimeout time.Duration
	AppVersion                  string
}
type Worker struct {
	queue          chan Event
	mu             sync.RWMutex
	closed         bool
	done           chan struct{}
	cancel         context.CancelFunc
	sink           Sink
	options        Options
	metrics        *metrics.Metrics
	logger         *slog.Logger
	drops          [5]atomic.Uint64
	shutdownBudget atomic.Int64
}

func NewWorker(sink Sink, options Options, m *metrics.Metrics, logger *slog.Logger) (*Worker, error) {
	if sink == nil || m == nil || logger == nil || options.BufferSize < 1 || options.BatchSize < 1 || options.FlushInterval <= 0 || options.WriteTimeout <= 0 {
		return nil, errors.New("invalid analytics worker configuration")
	}
	ctx, cancel := context.WithCancel(context.Background())
	w := &Worker{queue: make(chan Event, options.BufferSize), done: make(chan struct{}), cancel: cancel, sink: sink, options: options, metrics: m, logger: logger}
	go w.run(ctx)
	return w, nil
}

var dropReasons = [...]string{"buffer_full", "invalid", "closed", "shutdown_timeout", "flush_error"}

func (w *Worker) drop(reason string, n int) {
	w.metrics.Dropped.WithLabelValues(reason).Add(float64(n))
	for i, key := range dropReasons {
		if reason == key {
			w.drops[i].Add(uint64(n))
			return
		}
	}
}
func (w *Worker) Publish(_ context.Context, e Event) {
	// No serialization, network call, or log I/O on the request path.
	if !e.EventName.valid() || len(e.Template) > 200 || (e.Topic != nil && len(*e.Topic) > 200) || (e.Subtopic != nil && len(*e.Subtopic) > 200) {
		w.drop("invalid", 1)
		return
	}
	e.AppVersion = w.options.AppVersion
	w.mu.RLock()
	defer w.mu.RUnlock()
	if w.closed {
		w.drop("closed", 1)
		return
	}
	select {
	case w.queue <- e:
		w.metrics.Published.Inc()
	default:
		w.drop("buffer_full", 1)
	}
}
func (w *Worker) Shutdown(ctx context.Context) error {
	if deadline, ok := ctx.Deadline(); ok {
		w.shutdownBudget.Store(int64(max(0, time.Until(deadline))))
	}
	w.mu.Lock()
	if !w.closed {
		w.closed = true
		close(w.queue)
	}
	w.mu.Unlock()
	select {
	case <-w.done:
		return nil
	case <-ctx.Done():
		w.cancel()
		// Sink.Write honors context; join the canceled worker so its transport is
		// closed before the caller proceeds to PostgreSQL cleanup.
		<-w.done
		return ctx.Err()
	}
}
func (w *Worker) run(ctx context.Context) {
	defer close(w.done)
	defer w.cancel()
	defer func() {
		if err := w.sink.Close(); err != nil {
			w.logger.Warn("analytics transport close failed")
		}
	}()
	ticker := time.NewTicker(w.options.FlushInterval)
	defer ticker.Stop()
	warnings := time.NewTicker(time.Minute)
	defer warnings.Stop()
	report := func() {
		for i, reason := range dropReasons {
			n := w.drops[i].Swap(0)
			if n == 0 {
				continue
			}
			level := slog.LevelWarn
			if reason == "closed" {
				level = slog.LevelDebug
			}
			if reason == "shutdown_timeout" || reason == "flush_error" {
				level = slog.LevelError
			}
			attrs := []slog.Attr{slog.String("reason", reason), slog.Uint64("count", n), slog.Int("queue_len", len(w.queue)), slog.Int("queue_capacity", cap(w.queue))}
			if reason == "shutdown_timeout" {
				attrs = append(attrs, slog.Uint64("pending_events", n), slog.Duration("timeout", time.Duration(w.shutdownBudget.Load())))
			}
			// Flush failures already have a throttled record with batch size and a safe error class.
			if reason != "flush_error" {
				w.logger.LogAttrs(context.Background(), level, "analytics events dropped", attrs...)
			}
		}
	}
	defer report()
	batch := make([]Event, 0, w.options.BatchSize)
	var lastError time.Time
	flush := func() {
		if len(batch) == 0 {
			return
		}
		w.metrics.Flushes.Inc()
		writeCtx, cancel := context.WithTimeout(ctx, w.options.WriteTimeout)
		err := w.sink.Write(writeCtx, batch)
		cancel()
		if err != nil {
			w.metrics.FlushErrors.Inc()
			if ctx.Err() != nil {
				w.drop("shutdown_timeout", len(batch))
			} else {
				w.drop("flush_error", len(batch))
			}
			if ctx.Err() == nil && time.Since(lastError) >= time.Minute {
				w.logger.Error("analytics batch insert failed", slog.String("reason", "flush_error"), slog.Int("batch_size", len(batch)), slog.String("error", safeSinkError(err)))
				lastError = time.Now()
			}
		}
		clear(batch)
		batch = batch[:0]
	}
	for {
		if ctx.Err() != nil {
			w.drop("shutdown_timeout", len(batch))
			for range w.queue {
				w.drop("shutdown_timeout", 1)
			}
			return
		}
		select {
		case e, ok := <-w.queue:
			if !ok {
				flush()
				return
			}
			batch = append(batch, e)
			if len(batch) >= w.options.BatchSize {
				flush()
			}
		case <-ticker.C:
			flush()
		case <-warnings.C:
			report()
		case <-ctx.Done():
		}
	}
}

// Sink errors may include remote responses or credentials; log only bounded classes.
func safeSinkError(err error) string {
	if errors.Is(err, context.Canceled) {
		return "context_canceled"
	}
	if errors.Is(err, context.DeadlineExceeded) {
		return "deadline_exceeded"
	}
	return "sink_write_failed"
}
