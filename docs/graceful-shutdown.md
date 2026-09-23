# Graceful shutdown

## Lifecycle

Previously Gin owned the listener through `router.Run`, and `log.Fatalf` could exit before deferred database cleanup.

`cmd/api/main.go` now owns startup, SIGINT/SIGTERM, shutdown deadline and process exit status. `serve` waits for either a signal context or the buffered HTTP result channel. It drains HTTP for `HTTP_SHUTDOWN_TIMEOUT` (default `15s`), attempts force close after any drain failure, joins errors, waits for the listener goroutine and closes PostgreSQL last. The drain context is independent of the signal context. Active request contexts are not canceled on the first signal.

`internal/app` owns the standard `http.Server`; Gin is its Handler. `Run` normalizes `http.ErrServerClosed` to success. `Shutdown` disables atomic readiness before draining HTTP. `ForceClose` closes HTTP connections. `Close` releases infrastructure only; PostgreSQL's underlying sql.DB already supports repeated Close calls. Startup token-manager failures also retain database cleanup errors.

No application background workers were found. No lifecycle framework, dependencies, frontend changes or migrations were added.

## Probes and settings

- `/health` retains HTTP 200 and `{"status":"ok"}`.
- `/live` returns the same lightweight response.
- `/ready` returns 200 after dependency/router initialization and 503 before initialization or once shutdown begins. It does not ping the database on every request. During shutdown, after the listener closes, probes may get a connection refusal rather than 503.
- `HTTP_SHUTDOWN_TIMEOUT`: positive Go duration; default 15s. Invalid, zero and negative values fail configuration loading.
- Header read timeout: 5s; entire request read timeout: 60s; idle keep-alive timeout: 60s; response write timeout: 5m; maximum headers: 1 MiB. The longer response budget accommodates transactional JSON imports. These are transport deadlines, not cancellation deadlines for business operations.

Readiness describes completed application initialization, not ongoing DB health. The startup log says "starting HTTP listener" deliberately: ListenAndServe has not yet confirmed binding at that point. A bind failure is returned and cleaned up immediately.

## Tests

- `internal/app/lifecycle_test.go`: controlled active request drain, preserved response, infrastructure-close ordering, expired deadline and force-close request cancellation, occupied port, readiness and liveness, normal ErrServerClosed.
- `cmd/api/main_test.go`: signal-triggered shutdown, listener error, database close after HTTP, deadline fallback, preservation of all cleanup errors, independent drain context.
- `config/config_test.go`: default/custom/invalid shutdown timeout.
- `internal/app/training_test.go`: existing protected routes, real PostgreSQL initialization, compatible health/live/ready responses, repeated resource close.
- `internal/core/interview/graph/selector_test.go`: existing depth-limit test now uses configured maximum; pending Mock Interview work changed the default from 10 to 20. Algorithm unchanged.
- `go fmt ./...` also formats existing test code; no business behavior changed there.

## Operational limits

Force-closing HTTP connections cancels their request contexts, but Go cannot forcibly stop arbitrary handler goroutines that ignore cancellation. Existing DB calls must cooperate with request cancellation; sql.DB.Close can wait for outstanding queries. Hijacked connections are not currently used and would require explicit ownership if added. `/ready` does not detect a database outage after startup.

OS signal wiring uses standard signal.NotifyContext. Tests simulate its canceled context rather than delivering real process-level SIGTERM on Windows.

## Future improvements (not implemented)

- Second signal causing immediate termination; currently the configured shutdown deadline controls fallback.
- Deployment-specific drain delay/preStop.
- Structured lifecycle logging and Prometheus metrics.
- Explicit worker cancellation/join before DB close if background workers are introduced.

## Verification results — 2026-09-22

- `go fmt ./...`: passed.
- `go test ./...`: passed, including PostgreSQL fixtures enabled through TRAINING_TEST_DATABASE_URL.
- `go test -race ./...`: passed, no races reported. After final startup-error-message cleanup the changed cmd/api, config and internal/app packages were checked under race again and passed.
- `go vet ./...`: passed.
- `go build -o bin/knowledge-api-lifecycle.exe ./cmd/api`: passed.
- Frontend unchanged by this task; frontend checks were not run.
- Initial full test run exposed the pre-existing hardcoded depth boundary described above; the final full run passed after updating that test.

Files changed for this task: cmd/api/main.go, cmd/api/main_test.go, config/config.go, config/config_test.go, .env.example, internal/app/app.go, internal/app/lifecycle.go, internal/app/lifecycle_test.go, internal/app/training_test.go, internal/core/interview/graph/selector_test.go and this document. Running the requested repository-wide formatter also formatted internal/core/dashboard/query_test.go, internal/core/training/repository/postgres/progress_test.go, internal/core/interview/graph/mock_test.go and internal/core/training/handler/mock_interview_test.go. Existing pending Mock Interview implementation and frontend edits were preserved.
