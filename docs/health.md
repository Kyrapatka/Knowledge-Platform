# Health, liveness and readiness

## HTTP contract

All three endpoints are public GET routes, without authentication or dependency
calls. Their existing JSON contracts are preserved:

| Endpoint | Status | Body | Meaning |
| --- | --- | --- | --- |
| `/health` | 200 | `{"status":"ok"}` | Backward-compatible liveness |
| `/live` | 200 | `{"status":"ok"}` | Process serves HTTP |
| `/ready` | 200 | `{"status":"ok"}` | Startup complete, not shutting down |
| `/ready` | 503 | `{"status":"not_ready"}` | Not initialized or shutting down |

The existing frontend development proxy still supports `/health`. Probe handlers
do not emit domain logs; ordinary structured HTTP access logging is unchanged.

## Ownership and lifecycle

`App` owns one zero-value `Readiness` containing an `atomic.Bool` (initially false).
It is not global and must not be copied after use. `health.go` handlers read that
same App-owned state; there is no service/repository or separate probe state.

```text
config → DB opened and validated → repositories/services/router initialized
       → ready=true → Run / serve traffic

SIGINT/SIGTERM → Shutdown: ready=false → HTTP drain → DB close → exit
listener failure → ready=false → cleanup
```

`New` sets true only at its last step, before returning the App. Initialization
errors return no App. `Run` never sets true, including when called after shutdown.
`Shutdown` and `ForceClose` clear readiness before calling the HTTP server;
`Run` clears it on server exit. `Close` clears it before closing resources, even
on cleanup failure. Repeated false writes are safe. Only construction enables
readiness, so concurrent shutdown cannot be undone by a delayed Run call.

## PostgreSQL decision

The existing `OpenPostgres` validates the connection using `PingContext` during
startup. A failed connection prevents successful App construction. Probes only
read in-memory lifecycle state: no per-request Ping, polling, background checker
or connection-pool statistics are introduced. Pool size is not evidence of
database health. This avoids probe-induced dependency load and keeps liveness
independent from transient database failures.

## Shutdown and limitations

`http.Server.Shutdown` closes listeners before waiting for active requests.
The probe handler returns 503 as soon as shutdown disables readiness, while
active requests can still finish. A new network connection after listener closure
can receive connection refusal instead of a 503 response. No artificial delay or
separate management listener is added to create a probe-observation window.

Readiness is an initialization/lifecycle signal, not a continuous guarantee of
DB availability, schema compatibility, capacity or business-operation success.
It becomes true before binding; a bind error clears it and triggers cleanup.
Successful startup does not imply later database outages will turn readiness off.
The health endpoints are served on the same HTTP listener as application traffic.

## Tests

`health_test.go` checks zero/default state, toggling, idempotent false writes,
exact public contracts, access-only logging, concurrent requests and writes,
failed DB initialization, cleanup failures and shutdown before Run.
`lifecycle_test.go` synchronizes a real active HTTP request and shutdown with
channels: readiness becomes false before drain finishes, the same router returns
503 for `/ready` and 200 for `/live` and `/health`, then the active request completes.
It also covers occupied-port failure and forced drain. Existing App integration
tests verify successful startup and all three endpoints with PostgreSQL.

## Verification (2026-09-24)

- `go fmt ./...`: passed.
- `go test ./...`: passed.
- `go test -race ./...`: passed, no races reported.
- `go vet ./...`: passed.

Both `TRAINING_TEST_DATABASE_URL` and `TEST_DATABASE_URL` were configured for
PostgreSQL integration tests. Unchanged packages reused Go's test cache; the
modified App package ran successfully in both normal and race modes.
