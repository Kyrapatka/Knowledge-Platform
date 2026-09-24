# Observability and analytics foundation

## Boundaries

PostgreSQL remains the sole transactional source of truth. Prometheus measures
technical behavior, ClickHouse stores best-effort event history, JSON stderr logs
are ready for a Loki collector. No external stack is automatically deployed.
No secrets, nicknames, emails, request bodies, answers, auth headers or arbitrary
payload maps are copied into events. Topic/subtopic are explicitly selected
content taxonomy metadata (maximum 200 bytes), not full material metadata.

## Configuration and rollout

1. Keep `ANALYTICS_ENABLED=false` (default) until a ClickHouse database is ready.
2. Apply `deploy/clickhouse/001_analytics_events.sql` with an administrator.
   If changing `CLICKHOUSE_DATABASE`, adjust SQL database qualifiers too.
3. Create a dedicated INSERT-only application account and a separate SELECT-only
   Grafana account. Manage credentials outside source control.
4. Set `CLICKHOUSE_ADDR` to an HTTP(S) endpoint, e.g. `https://host:8443`, not
   native port 9000. TLS verification uses the system CA pool; no insecure skip.
   HTTP is intended for local development only. Credentials use Basic Auth,
   never URL query parameters. Redirects are refused.
5. Set `CLICKHOUSE_DATABASE`, `CLICKHOUSE_USER`, `CLICKHOUSE_PASSWORD`,
   `APP_VERSION` and enable analytics. No startup Ping or automatic DDL occurs.
6. Configure Prometheus to scrape the API's `/metrics`; keep this route private
   at the reverse proxy/firewall in production. It is unauthenticated locally.

Defaults: buffer 4096 events, batch 256, flush interval 5s, write timeout 5s,
shutdown timeout 10s. Config keys are `ANALYTICS_BUFFER_SIZE`,
`ANALYTICS_BATCH_SIZE`, `ANALYTICS_FLUSH_INTERVAL`, `ANALYTICS_WRITE_TIMEOUT`,
`ANALYTICS_SHUTDOWN_TIMEOUT`. Invalid sizes/durations/booleans fail configuration
without echoing secret values. Disabled mode does not create a ClickHouse client
or worker. Enable `LOG_FORMAT=json` for collection; all existing slog level and
request ID semantics remain unchanged.

## Prometheus

Each App owns an independent registry, including Go/process collectors:

| Metric | Labels / meaning |
| --- | --- |
| `http_requests_total` | method, route, status |
| `http_request_duration_seconds` | histogram, same labels, seconds |
| `http_requests_in_flight` | active handlers, no labels |
| `analytics_events_published_total` | accepted into memory, not delivery ACKs |
| `analytics_events_dropped_total` | reason: buffer_full, closed, flush_error, shutdown_timeout, invalid |
| `analytics_batch_flush_total` | nonempty insert attempts |
| `analytics_batch_flush_errors_total` | failed/ambiguous attempts |

Methods are allowlisted (`OTHER` fallback); routes are Gin templates or
`unmatched`, never raw paths. Status is bounded to HTTP codes. No user, request,
folder, material or session ID labels. `/metrics` scrapes are excluded from HTTP
metrics, while probes remain measurable. Gin automatic redirects and requests
rejected before routing are outside the middleware boundary, as with access logs.
Middleware precedes recovery so recovered panics count their resulting status.
No speculative `db_errors_total`: existing DB calls have no unified error owner.

## Event model and catalog

All events have `event_id`, `event_name`, `occurred_at` (UTC), `user_id`,
`session_id`, `app_version`, `algorithm_version`, `experiment_group`. Unknown
identifiers/versions are empty strings. Experiment group is currently empty;
there is no A/B assignment framework. Events use a closed typed schema, not `any`.

| Events | Publishing boundary |
| --- | --- |
| user_registered | after committed user creation, even if later token issuance fails |
| user_logged_in | successful session issuance |
| login_failed | invalid credentials / blocked user; anonymous, no submitted identifier |
| folder_created, folder_deleted | successful standalone service operations |
| folder_imported | outer import commit; seed import after successful changed outcome |
| material_created, material_deleted | standalone operations; imports/bulk emit genuinely created IDs after outer commit |
| training_started, training_completed, training_abandoned | real session creation / transition, including combined runs |
| training_answered | committed correct/wrong actions; never receipt replay |
| training_skipped, training_rollback | skip recovery, graph navigation, rollback and undo |
| rehab_started, rehab_completed | entering rehab / successful correct-answer exit, not a manual skip |
| cram_started, cram_completed | additional lifecycle events for cram sessions |
| interview_started, interview_question_answered, interview_completed | additional graph-interview events |

Training's service-local transaction wrapper collects explicit business events,
then publishes only after `RuntimeStore.Transact` succeeds. Repositories contain
no publishers and algorithms are unchanged. Failed callbacks/commits discard
pending events. Existing receipts bypass those event creation sites on replay.
Folder/material services inside generic imports retain their zero-value no-op
publisher; only the outer service publishes after commit. Custom callers wrapping
standalone services in their own transaction must follow the same rule.

`training_answered` includes folder/material IDs, available template/topic/subtopic,
difficulty, mode, result, before/after stage, consecutive-correct and wrong counts,
rehab flags, next-review timestamps and learned flags. It includes review credit
to separate graph practice from schedule-affecting reviews. `answer_time_ms` is
server elapsed time from presentation creation to action (includes idle/network
time), not a client stopwatch. No answer content is copied. Retrievability and
stability remain NULL: current progress has neither. Mock sessions with no
persistent progress also have NULL progress snapshots. Template is populated
from folders already read by the transaction; unavailable dimensions stay empty
or NULL without additional analytics-only DB queries. Manual `advance` is not
misclassified as an answer. Undo references the original event ID.

## Async delivery and lifecycle

```text
committed operation → Publish → bounded channel → one worker → batch HTTP INSERT

shutdown → readiness=false → HTTP drain/force close → stop accepting events
         → flush remaining queue/batch → close ClickHouse transport → close PostgreSQL
```

`Publisher.Publish(context.Context, Event)` returns no error because delivery is
best-effort and must not change business outcomes. It does no network/JSON/log I/O.
It uses a short local lock to serialize channel closure, never waits for queue
capacity, and ignores request cancellation after a successful commit. Event
snapshots must not be mutated after publishing. The application always injects
a worker or `NoopPublisher`; configuration branches are not spread through domain
methods. A future durable publisher can implement the same interface.

Flush occurs at batch size, interval and shutdown. Failed batches are counted and
discarded, with no retries. The worker continues processing later batches. Drop
WARNs are aggregated at most once a minute (plus shutdown summary); failed-insert
WARNs are similarly throttled and exclude exception text/response bodies.
An outage never changes readiness, auth or training responses. Shutdown gets a
separate bounded analytics budget after HTTP drain; expiration cancels in-flight
HTTP I/O, discards remaining queued events with counters and joins the worker.
Sink implementations must honor context cancellation. PostgreSQL cleanup still
runs when analytics shutdown times out. Forced-close handlers that ignore request
cancellation may publish too late; those events are rejected safely.

This is best-effort, at-most-one application insert attempt, not durable or
exactly-once delivery. Crashes lose memory; timeout may mean ClickHouse committed
even though the client observed failure. Metrics cannot settle that ambiguity.

## ClickHouse storage

`MergeTree`, monthly partitions `toYYYYMM(occurred_at)`, 365-day TTL. Sort key:
`(event_name, toDate(occurred_at), template, user_id, occurred_at, event_id)`.
It favors event/time/template slices and user cohorts; topic is a typed nullable
dimension filtered within those slices, not a primary sort prefix. Common
dimensions and learning fields are typed columns. Nothing operational lives here.
TTL deletion is asynchronous. Legal/user-deletion workflows need a separately
defined retention/erasure policy; internal UUIDs are still pseudonymous data.

Lifecycle IDs are stable per session/event name (undo can reopen a completed
session); MergeTree itself does not deduplicate. `queries.sql` provides an
`events_unique` view and sample queries. This first-stage view sorts on reads;
large deployments may need preaggregation after measuring actual workloads.

## Analytics and Grafana

`deploy/clickhouse/queries.sql` covers DAU/WAU/MAU, registrations, mature-cohort
calendar D1/D7/D30 retention, completion and answers/session, correctness by
mode/version/experiment/stage/topic/difficulty, rehab entry/recovery, observed
exposures and time until learned, difficult content and first-use funnel.
Learning retention after 7/30 days must use later review outcomes of the same
user/material after first learned date, with an explicitly chosen observation
window; it is not the same metric as user return retention. Only observed return
reviews are measurable, not unobserved memory retention. Compare cram and long-term
with cohort controls; mode differences are not automatically causal effects.

Four planned dashboards and sources (no provisioned credentials or services):

1. **Backend Overview — Prometheus**: RPS `sum(rate(http_requests_total[5m]))`;
   p50/p95/p99 `histogram_quantile(0.95, sum by(le)(rate(http_request_duration_seconds_bucket[5m])))`;
   4xx/5xx via status regex; `http_requests_in_flight`. Filter `/health|/live|/ready`
   out of user-facing latency panels where appropriate.
2. **Product Overview — ClickHouse**: active users, registrations, cohort return
   retention, unique training starts/completions, funnel. Use the official Grafana
   ClickHouse datasource plugin with the separate SELECT-only account.
3. **Learning Analytics — ClickHouse**: correct rate by stage/topic/difficulty,
   rehab entry/recovery, exposures until learned, observed 7/30-day review success,
   cram vs long-term and algorithm-version comparison. Exclude undone events and
   separate `review_credit=false` practice from scheduled learning.
4. **Errors / Operations — Prometheus + Loki**: 5xx, rate of analytics drops and
   flush errors; Loki queries for infrastructure DB errors and shutdown events.
   No DB error counter is advertised that the code does not instrument.

Loki collects JSON stderr through an agent, not an in-process client. Use bounded
labels such as service/environment; keep request IDs as searchable fields, not
stream labels. Prometheus and ClickHouse are available now; Loki and Tempo
datasources become usable only after their collectors/backends are deployed.

Tracing is a documented next stage, not fake spans/trace IDs: HTTP middleware can
install an OTel context before business handlers; existing `context.Context`
flows into GORM operations, and request-scoped slog can add valid trace/span IDs.
An SDK/exporter would be App-owned, flushed after HTTP drain with its own budget.
Never turn an arbitrary X-Request-ID into a trace ID. Full DB spans and export to
Tempo are deliberately not implemented in this change.

## Coverage limitations

No historical backfill. Active users mean recorded events, not read-only visits.
Losses and 365-day TTL limit long-term cohorts. Abandonment is explicit cancellation,
not inferred from a closed tab. Combined session counts refer to component sessions.
Seed/bulk upserts carry internal, JSON-excluded created IDs to the application
boundary, so existing/restored materials are not counted as new creations.
Multi-folder starts have no single folder dimension. These limitations must be
displayed on dashboards.
Material-deletion events describe explicit material deletion, not each item
removed by a folder-deletion cascade. Read-only visits and implicit tab closures
are not inferred as events.

References: [official Prometheus Go instrumentation](https://prometheus.io/docs/guides/go-application/)
and [ClickHouse HTTP interface](https://clickhouse.com/docs/interfaces/http).

## Verification — 2026-09-24

- `go fmt ./...`: passed.
- `go test ./...`: passed with both PostgreSQL test DSNs configured.
- `go test -race ./...`: passed, no races (training HTTP suite: 144.886s).
- `go vet ./...`: passed.
- `go mod tidy`: completed; dependencies resolved.
- `go build -buildvcs=false -o .cache/api-observability-check.exe ./cmd/api`: passed.
- `git diff --check`: passed (Windows LF/CRLF advisory warnings only).

New tests cover metrics, queue saturation/nonblocking publish, timer/size/shutdown
flush, sink failures, concurrent shutdown/publish, Noop, safe transport errors,
config validation, optional-dependency startup, typed answer snapshots, anonymous
login failures, and actual PostgreSQL deferred commit failure/receipt replay.
JSON event field names were compared with the ClickHouse DDL column names.

No live ClickHouse instance was available or deployed. Transport tests use an
HTTP test server; SQL DDL and analytical queries have not been executed against
a real ClickHouse server. Before production enablement, apply schema and validate
end-to-end ingestion and queries on the intended ClickHouse version.

The complete per-file inventory is in `observability-changes.md`.
