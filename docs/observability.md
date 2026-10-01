# Observability and learning analytics

## Boundaries

PostgreSQL remains the sole transactional source of truth. Prometheus measures
technical behavior, ClickHouse stores best-effort event history. Compose deploys
Prometheus, ClickHouse and provisioned Grafana alongside PostgreSQL and the API.
Logs remain structured JSON on stderr; no logging or tracing backend is installed.
No secrets, nicknames, emails, request bodies, answers, auth headers or arbitrary
payload maps are copied into events. Topic/subtopic are explicitly selected
content taxonomy metadata (maximum 200 bytes), not full material metadata.

## One-command local stack

```powershell
docker compose up -d
```

Requirements: Docker Engine/Desktop running, Compose v2, available host ports and
internet on first build for images, npm/Go dependencies and the Grafana plugin.
The multi-stage Dockerfile builds React and the Go API/migrator; the non-root runtime
serves both. PostgreSQL migrations run before the API. An idempotent, one-shot
`clickhouse-init` applies `001`, `002`, `003` and provisions least-privilege accounts.
It exits with code 0 normally. Backend depends only on healthy PostgreSQL: ClickHouse
initialization/outages do not gate core readiness. Events during first initialization
can be lost; wait for `clickhouse-init` to finish before exercising analytics.

| Service | Local address | Purpose |
| --- | --- | --- |
| backend | http://localhost:8080 | App/API; `/metrics`, `/live`, `/ready` |
| postgres | localhost:55432 | Separate Compose PostgreSQL database |
| clickhouse | http://localhost:8123 | HTTP SQL endpoint; `/ping` |
| prometheus | http://localhost:9090/targets | 15-second backend scrape, 30-day storage |
| grafana | http://localhost:3000 | Provisioned dashboards/datasources |

Grafana defaults: **admin / local-grafana-only**. ClickHouse accounts:
`analytics_admin / local-clickhouse-admin-only`, INSERT-only
`analytics_writer / local-writer-only`, SELECT-only `grafana_reader / local-reader-only`.
PostgreSQL defaults: database `knowledge_platform`, user `knowledge`, password
`local-postgres-only`. All are local development values, with host ports bound to
127.0.0.1. Change secrets and apply appropriate TLS/network access controls before
any shared deployment. Do not commit production passwords.

Overrides in `.env` or the shell: `KP_BACKEND_PORT`, `KP_POSTGRES_PORT`,
`KP_CLICKHOUSE_PORT`, `KP_PROMETHEUS_PORT`, `KP_GRAFANA_PORT`, `KP_POSTGRES_PASSWORD`,
`KP_JWT_SECRET`, `KP_CLICKHOUSE_ADMIN_PASSWORD`, `KP_CLICKHOUSE_WRITER_PASSWORD`,
`KP_CLICKHOUSE_READER_PASSWORD`, `KP_GRAFANA_USER`, `KP_GRAFANA_PASSWORD`.
Passwords embedded in the PostgreSQL URL must be URL-safe (or percent-encoded in
an override DATABASE_URL). `KP_ANALYTICS_ENABLED` defaults to true in Compose;
the existing host `.env` value `ANALYTICS_ENABLED=false` does not override it.
Host port changes do not affect internal datasource/scrape addresses.

Named volumes preserve PostgreSQL, ClickHouse, Grafana and Prometheus data.
`docker compose down` retains them; do not use `down -v` unless intentionally
resetting data. PostgreSQL and Grafana bootstrap credentials initialize new volumes;
changing their environment variables alone does not rotate existing database/admin
passwords. Rotate through each service's supported administration commands.
Writer/reader password changes apply on the next `clickhouse-init` run, with backend
and Grafana recreated using matching environment values.

After application edits use `docker compose up -d --build`. After SQL edits use
`docker compose run --rm clickhouse-init`. Dashboard files are watched every 15s;
restart Grafana after datasource/provisioning configuration edits. Regenerate JSON
and inspectable panel SQL with `python deploy/grafana/generate_dashboards.py`.
JSON/SQL outputs are committed, so Python is not required to start the stack.

### Existing external ClickHouse / host development

The existing `go run ./cmd/migrate` / `go run ./cmd/api` workflow is unchanged.
For an existing ClickHouse, apply numeric SQL files with an administrator, provision
an INSERT-only writer and SELECT-only reader, then set host `CLICKHOUSE_ADDR` to the
HTTP(S) endpoint, `CLICKHOUSE_DATABASE`, `CLICKHOUSE_USER`, `CLICKHOUSE_PASSWORD` and
`ANALYTICS_ENABLED=true`. SQL and dashboards qualify `knowledge_analytics`; adjust
all qualifiers together if using another database. Reader settings should include
`readonly=1`, `join_use_nulls=1` and `max_execution_time=30 CHANGEABLE_IN_READONLY`.
Use the system CA trust for TLS; the writer refuses redirects and never skips TLS
verification. Keep `/metrics` private at the production reverse proxy/firewall.

The default Compose file owns its local ClickHouse. To reuse an external one with
Compose, use a local override: disable `clickhouse`/`clickhouse-init` via profiles,
point backend `CLICKHOUSE_*` to the external HTTP endpoint, and mount a matching
Grafana datasource configuration. Do not start a second server against an existing
ClickHouse data directory. No existing ClickHouse was running on the verified host.

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

`training_answered` includes plan/folder/material IDs, available template/topic/subtopic,
difficulty, mode, result, before/after stage, consecutive-correct and wrong counts,
rehab flags, next-review timestamps and learned flags. It includes review credit
to separate graph practice from schedule-affecting reviews. `answer_time_ms` is
server elapsed time from presentation creation to action (includes idle/network
time), not a client stopwatch. No answer content is copied. Retrievability and
stability remain NULL: current progress has neither. Mock sessions with no
persistent progress also have NULL progress snapshots. Template is populated
from folders already read by the transaction; unavailable dimensions stay empty
or NULL without additional analytics-only DB queries. New typed fields are `plan_id`,
`interview_mode` (actual start configuration) and nullable `interview_depth` (actual
shown question depth, not the configured maximum). Manual `advance` is not
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
session); MergeTree itself does not deduplicate. `003_analytics_views.sql` creates
`events_unique` using the latest occurrence, and `events_effective` excludes undone
answers and their derived events. `queries.sql` supplies SQL-client examples.
Views sort/join on reads;
large deployments may need preaggregation after measuring actual workloads.

## Analytics and Grafana

Two automatically provisioned dashboards in folder **Knowledge Platform**:

- [Knowledge Platform - System](http://localhost:3000/d/knowledge-system): RPS,
  4xx and 5xx rates/totals/percentages, histogram p50/p95/p99 and route p95,
  in-flight requests, goroutines/threads/GOMAXPROCS, heap/RSS, GC cycle rate and
  pause summary, CPU usage in cores, scrape health, accepted/dropped/flush/error
  analytics counters. Probes are excluded from business traffic and latency.
- [Knowledge Platform - Learning Analytics](http://localhost:3000/d/knowledge-learning):
  sessions, completion, answers and elapsed time; correctness by stage, difficulty,
  topic and template; learning milestones, observed retention, rehab, cram versus
  long-term, mock interviews, content usage, product cohorts/funnel and algorithm
  comparisons. Rows group related panels; training views support mode/version
  filters. Product user cohorts and funnel remain global.

Stable datasource UIDs: `knowledge-prometheus` (default, Prometheus) and
`knowledge-clickhouse` (official `grafana-clickhouse-datasource`, plugin 4.20.0
installed synchronously by the Grafana container). Both use Docker service names.
Grafana queries ClickHouse through HTTP with the separate SELECT-only account;
reader max_execution_time is changeable in readonly mode as the plugin requires.

### Definitions and denominators

All cohort days and dashboard times are UTC. Missing observations are blank, not
fabricated zero successes. Panel SQL is in `deploy/grafana/sql`, reusable views in
`deploy/clickhouse/003_analytics_views.sql`.

| Measure | Definition / scope |
| --- | --- |
| Session usage | Sessions started in selected range; their latest effective completion/cancellation observed through now. Undo reopens a session. Answers/session includes zero-answer sessions. Duration only for closed sessions. |
| Normal correctness | Effective correct / (correct + wrong) `training_answered`, mode other than mock. |
| Mock correctness | Only `interview_question_answered` with mode=mock. No normal-event duplication. Actual configured interview mode and presented depth are separate dimensions. |
| Learned | First observed credited false-to-true learned transition per user/material/plan/mode/version/group. Attempts/time begin with first recorded effective credited answer. Unknown historical plan IDs are excluded. |
| Learning retention 7/30 | First credited review of that same user/material/plan/mode/version/group in [N,N+1) days after learned. Include only fully mature windows. Correct / observed reviews, alongside eligible count and review coverage. No review is unobserved, not wrong. |
| Rehab entry rate | Answers entering rehab / credited answers. Stage panel also shows wrong rate. |
| Rehab recovery | Recovered episodes / observed entries. Only a correct rehab exit qualifies; manual skip does not. Attempts before recovery average only recovered episodes and count rehab reviews, not the triggering normal wrong answer. |
| Cram vs long-term | Real mode dimension: sessions/completion, correctness, rehab entry, answer time. Learning retention is grouped by mode in its own panel. Observational, not a causal comparison. |
| DAU/WAU/MAU | Distinct users with recorded events in rolling 1/7/30 days ending at selected range end. No read-only page visits. |
| D1/D7/D30 product retention | Return activity on exactly day N after registration (UTC), restricted to cohorts whose entire return day has elapsed. Shows eligible and returning counts. |
| Funnel | Ordered registration, folder, material/import, start, completion, subsequent D1/D7 activity. D1/D7 refer to registration calendar day; return must also occur after completion. Eligible counts expose cohort maturity. |
| Algorithm comparison | Recorded version/group correctness, completion, learned attempts/time, rehab rate and answer time; retention panel also groups by version/group. No assignment mechanism exists: groups remain unassigned. |
| Content difficulty | ID/topic/template/difficulty only; material wrong-rate and low-correctness topic rankings require >=3 answers to reduce one-answer noise. |

No Loki, Tempo, tracing collector or synthetic DB-error counter is introduced.

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

## Verification

Run from the repository root against the local stack:

```powershell
docker compose config --quiet
docker compose up -d
docker compose ps -a
python deploy/verify_observability.py --require-events
python deploy/test_analytics_sql.py
python deploy/verify_analytics_resilience.py
```

`verify_observability.py` checks backend UP, both datasource health endpoints,
provisioned dashboard identities and every panel query (Prometheus API / Grafana
ClickHouse plugin). `--require-events` additionally requires positive accepted and
flush counters, so run it after real activity. SQL tests use a random, separate
`analytics_test_*` database and drop only that database in finally; no synthetic
rows are inserted into product analytics. They cover undo/recompletion, plan
separation, observed/missing D7/D30 review denominators, rehab episodes and mock depth.
The resilience check temporarily stops this stack's ClickHouse, creates QA content,
answers a real training card and checks readiness plus flush errors. It restarts
ClickHouse in finally and verifies a separate temporary Noop backend with real auth.

For browser checks (creates separate QA users/content, imports the Go question bank):

```powershell
$env:OBSERVABILITY_E2E='true'
npm.cmd --prefix web ci
npm.cmd --prefix web run test:e2e -- observability.spec.ts --trace off
```

Installed Edge is the default; set `PLAYWRIGHT_CHANNEL` or `E2E_BASE_URL` as needed.
The test logs in, completes normal training, answers/ends a deep mock interview,
checks real ClickHouse events/depth and opens both Grafana dashboards. Trace capture
is disabled here to avoid retaining authentication exchanges. Python/browser checks
read `KP_*` overrides from the shell (they do not load `.env` automatically).

For Go integration tests, use PostgreSQL test DSNs with schema privileges:

```powershell
$env:TEST_DATABASE_URL='postgres://knowledge:local-postgres-only@127.0.0.1:55432/knowledge_platform?sslmode=disable'
$env:TRAINING_TEST_DATABASE_URL=$env:TEST_DATABASE_URL
go fmt ./...
go test ./...
go test -race ./...
go vet ./...
```

Both repository/handler integration suites isolate their tables in random schemas.
Worker tests cover queue saturation/nonblocking publish, flush timer/size/shutdown,
sink failures, concurrent closure, Noop and optional dependency startup. Service
tests cover actual commit failure/replay and truthful event dimensions. No business
algorithm or mock interview behavior is changed by observability instrumentation.

References: [Grafana ClickHouse datasource configuration](https://grafana.com/docs/plugins/grafana-clickhouse-datasource/latest/configure/),
[query editor](https://grafana.com/docs/plugins/grafana-clickhouse-datasource/latest/query-editor/),
[Grafana Docker configuration](https://grafana.com/docs/grafana/latest/setup-grafana/configure-docker/).

## Learning dashboard: panels and query contract

Open [Knowledge Platform - Learning Analytics](http://localhost:3000/d/knowledge-learning).
The existing file provider loads it automatically from `deploy/grafana/dashboards/learning.json`.
It uses only datasource `knowledge-clickhouse`; the System dashboard remains separate.
Edit `deploy/grafana/generate_dashboards.py`, then regenerate the checked-in JSON/SQL:

```powershell
python deploy/grafana/generate_dashboards.py
```

The eight overview stats are **Training Sessions Started**, **Training Sessions
Completed**, **Completion Rate**, **Answers Total**, **Correct Rate**, **Wrong Rate**,
**Average Session Duration**, and **Average Answers per Session**. Session measures
select the start cohort and use latest observed outcomes; answer measures select
answer timestamps. Duration includes completed/cancelled sessions only. Completion
rate includes still-open starts in the denominator. Combined runs can contain several
component sessions. Empty counts are 0; undefined rates/averages remain NULL.

The rest of the dashboard has eight sections:

1. Training Usage: adaptive time buckets for starts/completions and total/correct/wrong
   answers, plus session and answer statistics by the real mode.
2. Learning Effectiveness: correctness by **stage_before**, difficulty, topic and
   template, with answer time in each dimension; learned milestones and observed
   7/30-day learning retention with observation coverage.
3. Rehab: entries/wrong rate by stage, successful recovery rate, attempts before
   recovery, event-time entries/exits and topics with highest entry rate.
4. Cram vs Long-term: sessions, completion, answers, correct/wrong rates, answer time,
   credited rehab rate, observed attempts/time to learned. Normal track is actually
   emitted as `default`; `cram` and `mock` are separate real modes. No invented
   `long-term` event value is used. Normal training tables exclude mock.
5. Mock Interview: start/completion/rate by configured interview mode, questions,
   correctness, answer time, actual follow-up depth and topic/difficulty breakdown.
6. Content Analytics: top 15 per folder/template/topic dimension, wrong-rate ranking,
   low-correctness topics, and a separate material ranking by rehab entry count.
7. Algorithm Analytics: actual version and emitted experiment_group, including
   answers/correct/wrong/rehab/completion/answer time and observed learning milestones.
   Keys include versions with answers even when their sessions began before the range.
8. Product retention/funnel: rolling DAU/WAU/MAU, exact UTC D1/D7/D30 and ordered
   registration-to-content-to-training-to-return stages.

Only two low-cardinality dropdowns are needed: **Mode** and **Algorithm** (the latter
queries `algorithm_version`). They refresh using the selected time range; All also
includes historical unknown values. There is no user/material dropdown and no empty
A/B selector. The raw group column displays `(unassigned)` until assignments exist.
Product cohorts/funnel/active-user counts intentionally remain global.

Every panel has an explicit temporal contract. Event charts use `$__timeFilter` and
`$__timeInterval`; session, learned and rehab panels select start/learned/entry cohorts.
DAU/WAU/MAU are fixed 1/7/30-day windows ending at `$__toTime` rather than truncated by
the range start. Cohort outcomes are observed through the latest available history;
the dashboard is not an as-of historical replay. Product return-day eligibility is
bounded by the selected range end. All dashboard projections are explicit; no
`SELECT *` or sensitive content is used in panel queries.

The MergeTree sort key starts with event_name/date/template and monthly partitions.
Time/event predicates help event queries; deduplication, undo, first-ever learning
and cross-day cohorts still require historical reads through existing views. No
schema/key change or premature materialized aggregation is added. With large volumes,
measure query_log/read_rows and EXPLAIN indexes before introducing aggregate tables;
this dashboard does not promise bounded scans for full-history cohort calculations.

Missing evidence stays missing: no backfill, no pure attention-time measurement,
no memory-retention estimate for materials without a later answer, and no causal
A/B conclusion. Future experiments require a persistently assigned experiment ID/
group and assignment event, carried into actual training events. Future attention-time
analytics would require an explicit validated client duration measurement; current
answer_time_ms includes idle/network time. Existing fields already support observed
learning retention once enough mature repeat history accumulates; no new events
are necessary for the current dashboard.

Validation: `deploy/test_analytics_sql.py` executes every Learning panel against an
empty and a populated isolated ClickHouse database, checking exact counts/rates,
stage-before attribution, normal/mock separation, rehab recovery, undo and time buckets.
`deploy/verify_observability.py` executes all panel queries through live datasources.
`web/tests/observability.spec.ts` exercises real normal training and a deep mock,
then opens Grafana and scrolls through sections so lazy-loaded queries run.

## Fast local commands

`make observability` (alias `make grafana`) starts the entire existing stack and
waits for backend, ClickHouse initialization, Grafana and Prometheus health. It
runs no tests and reuses the current image; use `make deploy-local` to rebuild.
`make observability-down` stops it while preserving all volumes. Docker must
already be running.

`make observability-check` uses this document's existing verifier in quick mode:
health, provisioned dashboards/data sources and live metrics.
`make observability-full-check` runs require-events queries across both dashboards,
isolated SQL fixtures and real outage/Noop resilience checks. The resilience check
temporarily stops ClickHouse and restores it afterward. Generate learning activity
first, for example with the opt-in observability Playwright scenarios. See
[deployment and rollback](local-deployment.md).
