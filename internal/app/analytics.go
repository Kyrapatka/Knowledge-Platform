package app

import (
	"context"
	"sync"
	"time"

	"github.com/Kyrapatka/knowledge-platform/config"
	"github.com/Kyrapatka/knowledge-platform/internal/platform/analytics"
	"github.com/Kyrapatka/knowledge-platform/internal/platform/metrics"
	"log/slog"
)

func startAnalytics(cfg config.Analytics, m *metrics.Metrics, logger *slog.Logger) (analytics.Publisher, func() error, error) {
	if !cfg.Enabled {
		return analytics.NoopPublisher{}, nil, nil
	}
	sink, err := analytics.NewClickHouse(cfg.ClickHouse)
	if err != nil {
		return nil, nil, err
	}
	w, err := analytics.NewWorker(sink, cfg.Worker, m, logger)
	if err != nil {
		_ = sink.Close()
		return nil, nil, err
	}
	timeout := cfg.ShutdownTimeout
	if timeout <= 0 {
		timeout = 10 * time.Second
	}
	var once sync.Once
	var closeErr error
	return w, func() error {
		once.Do(func() {
			ctx, cancel := context.WithTimeout(context.Background(), timeout)
			defer cancel()
			closeErr = w.Shutdown(ctx)
		})
		return closeErr
	}, nil
}
