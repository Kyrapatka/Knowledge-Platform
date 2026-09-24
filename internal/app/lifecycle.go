package app

import (
	"context"
	"errors"
	"fmt"
	"log/slog"
	"net/http"
	"time"
)

func newHTTPServer(address string, handler http.Handler) *http.Server {
	return &http.Server{Addr: address, Handler: handler,
		ReadHeaderTimeout: 5 * time.Second, ReadTimeout: 60 * time.Second,
		// Imports may perform many transactional writes after reading the request.
		WriteTimeout: 5 * time.Minute, IdleTimeout: 60 * time.Second, MaxHeaderBytes: 1 << 20}
}
func (a *App) Run() error {
	// ListenAndServe has not confirmed a bind yet; do not claim it has started.
	a.logger.Info("http server starting", slog.String("address", a.server.Addr))
	err := a.server.ListenAndServe()
	a.readiness.SetReady(false)
	if err != nil && !errors.Is(err, http.ErrServerClosed) {
		return fmt.Errorf("serve HTTP: %w", err)
	}
	return nil
}
func (a *App) Shutdown(ctx context.Context) error {
	a.readiness.SetReady(false)
	if err := a.server.Shutdown(ctx); err != nil {
		return fmt.Errorf("drain HTTP: %w", err)
	}
	a.logger.Info("http server drained")
	return nil
}
func (a *App) ForceClose() error {
	a.readiness.SetReady(false)
	if err := a.server.Close(); err != nil {
		return fmt.Errorf("force close HTTP: %w", err)
	}
	return nil
}

// Close releases infrastructure only; the owner must drain or close HTTP first.
// database/sql.DB.Close is already safe to call repeatedly.
func (a *App) Close() error {
	// Even cleanup without a preceding Run/Shutdown must not leave readiness on.
	a.readiness.SetReady(false)
	// HTTP has drained (or was force-closed) before this cleanup boundary.
	// Analytics is best-effort and must not prevent PostgreSQL cleanup.
	if a.closeAnalytics != nil {
		if err := a.closeAnalytics(); err != nil {
			a.logger.Warn("analytics shutdown deadline exceeded")
		}
	}
	if a.closeDatabase != nil {
		if err := a.closeDatabase(); err != nil {
			return fmt.Errorf("close database: %w", err)
		}
		a.logger.Info("database closed")
	}
	return nil
}
