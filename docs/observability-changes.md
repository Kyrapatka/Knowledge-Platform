# Observability change inventory

Paths are relative to the repository root. Existing uncommitted health/readiness
files from the preceding task are preserved, not counted as newly implemented
observability features. `cmd/api/main.go` already drains HTTP before calling
`App.Close`, so it did not need a new lifecycle coordinator.

## Foundation and configuration

| File | Change and purpose |
| --- | --- |
| `.env.example` | Optional ClickHouse/worker/version settings with safe disabled defaults |
| `config/config.go` | Load analytics settings into App configuration |
| `config/analytics.go` | Validate booleans, sizes, durations and HTTP(S) endpoint without network I/O |
| `config/analytics_test.go` | Defaults, invalid inputs and offline startup configuration |
| `go.mod` | Official Prometheus client and resolved dependency classification |
| `go.sum` | Checksums for the resolved dependencies |
| `internal/platform/metrics/metrics.go` | App-local registry, technical counters, histogram, gauge, HTTP middleware and exposition |
| `internal/platform/metrics/metrics_test.go` | Count, latency, in-flight, panic and normalized-label checks |
| `internal/platform/analytics/event.go` | Closed typed envelope and required event catalog |
| `internal/platform/analytics/emitter.go` | Composition-time publisher dependency with safe zero-value behavior |
| `internal/platform/analytics/worker.go` | Nonblocking bounded queue, batches, timer, aggregated drops and cancellation-aware shutdown |
| `internal/platform/analytics/worker_test.go` | Queue saturation, flush triggers, failures, cancellation and concurrent close/publish |
| `internal/platform/analytics/clickhouse.go` | Batch JSONEachRow HTTP transport, TLS verification, timeouts and safe errors |
| `internal/platform/analytics/clickhouse_test.go` | HTTP transport protocol/authentication/error tests without external ClickHouse |
| `internal/app/analytics.go` | Noop/worker construction and once-only bounded analytics shutdown |
| `internal/app/analytics_test.go` | Cleanup ordering and App startup/metrics with disabled or unreachable ClickHouse |
| `internal/app/app.go` | Registry, /metrics, middleware and publisher wiring |
| `internal/app/lifecycle.go` | Analytics flush/transport closure before PostgreSQL cleanup |

## Business boundaries (no algorithm/schema changes)

| File | Change and purpose |
| --- | --- |
| `internal/auth/service/service.go` | Publisher dependency |
| `internal/auth/service/register.go` | Registered event after user persistence |
| `internal/auth/service/login.go` | Successful login and anonymous credential-failure events |
| `internal/auth/service/analytics_test.go` | Privacy, infrastructure failure and panic classification |
| `internal/core/folder/service/service.go` | Standalone creation/deletion events after successful persistence |
| `internal/core/folder/importer/service.go` | Collect imported folder/material outcomes and publish after outer commit |
| `internal/core/material/service/service.go` | Standalone creation/deletion events |
| `internal/core/interview/import.go` | Return internal JSON-excluded created IDs; no API response change |
| `internal/core/interview/handler.go` | Publish committed seed/bulk outcomes at application boundary |
| `internal/core/training/service/analytics.go` | Service-local pending events, post-commit publish, typed snapshots and lifecycle builders |
| `internal/core/training/service/analytics_test.go` | Commit/replay/NULL snapshot/privacy/rehab semantics |
| `internal/core/training/service/service.go` | Shared transaction boundary, starts, explicit finishes and cancellation events |
| `internal/core/training/service/action.go` | Queue real before/after answer snapshot inside transaction, publish afterward |
| `internal/core/training/service/combined.go` | Combined component lifecycle and replacement events |
| `internal/core/training/service/change.go` | Abandonment when changing an active plan; shared transaction boundary |
| `internal/core/training/service/pool.go` | Auto-completion event on actual transition |
| `internal/core/training/service/interview_graph.go` | Graph starts/automatic completion |
| `internal/core/training/service/interview_graph_action.go` | Graph answer snapshots without fabricated practice progress |
| `internal/core/training/service/interview_graph_undo.go` | Post-commit undo event referencing the original answer |
| `internal/core/training/service/skip.go` | Skip event with available real progress snapshots |
| `internal/core/training/service/undo.go` | Post-commit combined undo event |
| `internal/core/training/service/early.go` | Use shared transaction boundary for nested lifecycle transitions |
| `internal/core/training/service/final.go` | Use shared transaction boundary for nested lifecycle transitions |
| `internal/core/training/service/exercise.go` | Uniform transaction wrapper; no additional exercise events |
| `internal/core/training/service/material_progress.go` | Uniform transaction wrapper; no new progress-read events |
| `internal/core/training/service/mock_interview.go` | Uniform transaction wrapper; no new preview/read events |
| `internal/core/training/handler/analytics_test.go` | Real PostgreSQL deferred commit failure, successful publish and receipt replay |

## Operations artifacts

| File | Purpose |
| --- | --- |
| `deploy/clickhouse/001_analytics_events.sql` | Explicit admin-applied MergeTree database/table DDL with typed dimensions and TTL |
| `deploy/clickhouse/queries.sql` | Deduplicated view, product/learning/content/cohort/funnel examples |
| `docs/observability.md` | Rollout, semantics, failure cases, four dashboards, Loki/Tempo extension points and limitations |
| `docs/observability-changes.md` | This per-file implementation inventory |
