# Structured logging foundation

## Ownership and configuration

The API entrypoint (`cmd/api/main.go`, through `run`) loads config, creates one
`*slog.Logger` with `internal/platform/logger.New`, and passes it explicitly to
`app.New(cfg, logger)` and the process lifecycle coordinator. App rejects nil;
it does not construct a fallback. No `slog.SetDefault`, logging interfaces,
third-party logging packages, new environment modes or domain refactors are used.

```env
LOG_LEVEL=info
LOG_FORMAT=text
```

Supported levels: `debug`, `info`, `warn`, `error`. Formats: `text`, `json`.
Unset/empty environment variables use the defaults above, matching the existing
config style. Nonempty unsupported values fail startup (exit 1); they are never
silently mapped to info/text or echoed back into logs. The factory also validates
direct callers. It uses `slog.TextHandler` or `slog.JSONHandler` with an explicit
level. All records include `service=knowledge-platform` and go to stderr.
An optional factory `Output io.Writer` makes tests independent of global output.

Configuration/logger initialization failures occur before a configured logger
exists. They produce one plain stderr bootstrap diagnostic and a nonzero exit,
not a second default/global logger. Never log the entire config object.

## Lifecycle and error ownership

- INFO: application starting, database connected, HTTP server starting, shutdown
  started, HTTP server drained, database closed, application stopped.
- WARN: graceful-drain deadline exceeded; force-close recovery is attempted.
- ERROR: initialization, listener, drain (other than deadline), force-close or
  database-close failure. Each operation's error is logged once by its owner.
- DEBUG: reserved for deliberate technical diagnostics; normal lifecycle is INFO.

The HTTP message intentionally says **starting**, not **started**:
`ListenAndServe` has not confirmed that the socket is bound at that point.
No lifecycle behavior was changed just to manufacture a “started” event.
`http.ErrServerClosed` remains a normal termination. “Drained”/“database closed”
are only emitted after the corresponding call succeeds.

App returns infrastructure failures with `%w`; the entrypoint/coordinator logs
them with `slog.Any("error", err)`. There is no additional “application failed”
echo of already logged cleanup/listener errors. The coordinator still joins and
returns all causes and preserves graceful-shutdown ordering. The final INFO
record has `clean_shutdown=true|false`; exit status remains authoritative.

Repositories and services do not receive a logger or log each propagated error.
Future domain events should be added selectively at their owning boundary.

Illustrative text output:

```text
time=2026-09-23T12:00:00.000Z level=INFO msg="http server starting" service=knowledge-platform address=:8080
```

The same record with JSON:

```json
{"time":"2026-09-23T12:00:00Z","level":"INFO","msg":"http server starting","service":"knowledge-platform","address":":8080"}
```

## Existing logging and secret safety

Gin's built-in access logger has been removed. The middleware order is
`RequestID -> AccessLog -> Recovery -> handler`, from
`internal/platform/httpmiddleware`. Access logs use the application's logger,
output format and level threshold, with no second plaintext access record.

Every request entering this chain gets a fresh server-generated UUID returned in
`X-Request-ID`. Incoming IDs are deliberately ignored, even valid UUIDs: there is
no configured trusted proxy boundary. `ID(ctx)` exposes the ID and
`Logger(ctx, fallback)` exposes the derived logger through `c.Request.Context()`.
Outside an HTTP request they return an empty ID and the explicit fallback.

One `http request completed` record contains `request_id`, allowlisted `method`
(nonstandard methods become `OTHER`), route template, actual `status`,
`duration_ms` and `response_bytes`. Unmatched routes, including SPA fallback,
use `route=unmatched`, never the raw path. 5xx and recovered panics are ERROR;
other responses, including expected 401/404, are INFO. LOG_LEVEL can therefore
suppress normal access records. No raw path, query, headers, body, client IP,
user agent, path parameter or `c.Errors` text is logged.

Gin's automatic slash/path redirects happen before its middleware chain; those
redirects, and requests rejected by net/http before routing, are outside this
logging boundary. Existing redirect behavior is unchanged.

Gin recovery is retained via `gin.RecoveryWithWriter` with a safe callback. The
default debug panic dump masks Authorization but exposes Cookie and the panic
value. We disable that dump and attempt HTTP 500; the single completion ERROR
record includes `panic_recovered=true` plus `debug.Stack()` (function/file/line stack, not request headers/body,
source-line contents or panic value). Gin's broken-connection handling remains in
place. If headers were already written, the actual status is retained and the
panic marker still makes the record ERROR. Gin debug route announcements remain Gin-owned; deploy with
the existing `GIN_MODE=release` setting when debug announcements are undesirable.

Default GORM logging is disabled with its existing Discard logger: interpolated
SQL can expose auth hashes and refresh tokens, even on failed INSERTs. Query
errors still propagate normally. This deliberately avoids duplicate repository
logs rather than routing potentially sensitive SQL through slog.

Database connection errors get a safe rendering with their original cause
preserved by `Unwrap` for `errors.Is/As`. Logs expose a category (invalid config,
timeout, canceled, generic connection failure) and a validated SQLSTATE when
available, never full driver text or DATABASE_URL. This also protects the separate
migration CLI's connection-error diagnostic; that CLI otherwise retains its
existing human-readable output and does not share an API-process logger.

This is not an arbitrary-secret redaction framework. Call sites must not add
passwords/hashes, tokens/JWTs, Authorization/Cookie values, raw config/DSNs,
request bodies or untrusted error text as log attributes. Request-scoped loggers
provide correlation, not automatic sanitization of arbitrary attributes.

## Tests and verification

- Logger: both handler types, all four thresholds, constant service field,
  JSON records, invalid inputs, no echoed config values, concurrent derived loggers.
- Config/entrypoint: defaults, explicit settings, invalid levels/formats and
  nonzero startup exit before database initialization.
- App/lifecycle: required logger, successful infrastructure events, failures
  returned without duplicate logs, WARN timeout fallback, original cleanup order.
- PostgreSQL/Gin integration: real App wiring; access/recovery preserved while
  query/header/body/internal-error/panic credentials are absent from output.
- HTTP middleware: response/context/log correlation, untrusted ID replacement,
  concurrent unique IDs, level filtering, route templates, 401/404/503, panic
  before and after response commitment, one completion event and secret safety.
- Connection errors: malformed credential-bearing DSNs, safe SQLSTATE, retained
  cause identity/type and timeout/cancel classification.

Repository checks: `go fmt ./...`, `go test ./...`, `go test -race ./...`,
`go vet ./...`. Integration tests use the existing isolated PostgreSQL schema
fixtures configured by `TRAINING_TEST_DATABASE_URL` and `TEST_DATABASE_URL`.
The API dependency graph and DB schema are unchanged by this logging task.

Verified on 2026-09-23: all four commands passed, with both PostgreSQL test DSNs
enabled; the race run reported no races. Real process smoke checks verified
text/JSON output, invalid logging config exit 1, credential-free malformed DSN
diagnostics, and occupied-port cleanup with exactly one listener ERROR.
An optional ordinary build encountered a Windows module-cache metadata write
warning; `go build -buildvcs=false -o .cache/api-logging-check.exe ./cmd/api`
completed without that warning and was used for the process smoke checks.

## Scope

Request IDs and structured HTTP logging are implemented without new dependencies,
API response-body changes, metrics or tracing. Domain logging remains unchanged.

Follow-up verification on 2026-09-23: `go test ./...` and `go vet ./...`
passed with both database test variables configured. Targeted race checks passed
for `internal/platform/httpmiddleware`, `internal/app` and `cmd/api`; the App
integration used PostgreSQL. Changed Go files were formatted with gofmt.
