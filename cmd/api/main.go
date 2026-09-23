package main

import (
	"context"
	"errors"
	"fmt"
	"log/slog"
	"os"
	"os/signal"
	"syscall"
	"time"

	"github.com/Kyrapatka/knowledge-platform/config"
	"github.com/Kyrapatka/knowledge-platform/internal/app"
	applog "github.com/Kyrapatka/knowledge-platform/internal/platform/logger"
)

func main() {
	os.Exit(run())
}

func run() int {
	cfg, err := config.Load()
	if err != nil {
		// Bootstrap diagnostics: no configured logger exists yet. Errors from
		// config deliberately name keys, never their supplied values.
		fmt.Fprintln(os.Stderr, "load config:", err)
		return 1
	}
	logger, err := applog.New(applog.Config{Level: cfg.LogLevel, Format: cfg.LogFormat})
	if err != nil {
		fmt.Fprintln(os.Stderr, "initialize logger:", err)
		return 1
	}
	logger.Info("application starting")
	signalCtx, stop := signal.NotifyContext(context.Background(), os.Interrupt, syscall.SIGTERM)
	defer stop()
	application, err := app.New(cfg, logger)
	if err != nil {
		logger.Error("application initialization failed", slog.Any("error", err))
		return 1
	}
	if err := serve(signalCtx, application, cfg.HTTPShutdownTimeout, logger); err != nil {
		return 1
	}
	return 0
}

// This boundary permits lifecycle tests without PostgreSQL or process signals.
type applicationLifecycle interface {
	Run() error
	Shutdown(context.Context) error
	ForceClose() error
	Close() error
}

func serve(signalCtx context.Context, application applicationLifecycle, timeout time.Duration, logger *slog.Logger) (result error) {
	defer func() {
		if err := application.Close(); err != nil {
			logger.Error("failed to close database", slog.Any("error", err))
			result = errors.Join(result, err)
		}
		logger.Info("application stopped", slog.Bool("clean_shutdown", result == nil))
	}()
	serverErr := make(chan error, 1)
	go func() { serverErr <- application.Run() }()
	serverExited := false
	reason := "server_exit"
	select {
	case <-signalCtx.Done():
		reason = "signal"
	case err := <-serverErr:
		serverExited = true
		result = err
		if err != nil {
			logger.Error("http server failed", slog.Any("error", err))
		}
	}
	// A signal must not cancel the drain context or active request contexts.
	ctx, cancel := context.WithTimeout(context.Background(), timeout)
	defer cancel()
	logger.Info("shutdown started", slog.String("reason", reason), slog.Duration("timeout", timeout))
	if err := application.Shutdown(ctx); err != nil {
		if errors.Is(err, context.DeadlineExceeded) {
			logger.Warn("graceful shutdown timed out; forcing http close", slog.Any("error", err))
		} else {
			logger.Error("graceful shutdown failed; forcing http close", slog.Any("error", err))
		}
		forceErr := application.ForceClose()
		if forceErr != nil {
			logger.Error("failed to force close http server", slog.Any("error", forceErr))
		}
		result = errors.Join(result, err, forceErr)
	}
	if !serverExited {
		err := <-serverErr
		if err != nil {
			logger.Error("http server failed", slog.Any("error", err))
		}
		result = errors.Join(result, err)
	}
	return result
}
