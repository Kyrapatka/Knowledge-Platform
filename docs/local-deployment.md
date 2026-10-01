# Local deployment and rollback

Run commands from the repository root with Go 1.25+, Make and an already running
Docker Engine/Desktop with Compose v2. Node 22.12+ is needed for host frontend
checks; Docker builds its own Node/Go stages. The helper never launches Desktop.
An unavailable daemon produces a clear error and a nonzero exit. Python 3 is
needed only for observability verification. No registry or remote CD is used.

## Deploy

```powershell
make deploy-local
make deployment-status
```

The helper records the healthy running backend's image ID and migration version,
tags it `<compose-project>-backend:previous`, builds source as `:candidate`, then
tags it `:current`. Frontend and Swagger are inside that same application image.
The image version label defaults to UTC timestamp + Git short revision; set
`LOCAL_VERSION` for an explicit label. Uncommitted source is included, so a Git
revision alone is not an exact build identifier. Immutable image IDs identify it.

Compose starts the existing stack and waits for PostgreSQL health. The image
command runs `./migrate && exec ./api`: migrations run once before the API, with
no second helper migrator. Success requires the expected image and HTTP `/ready`
200. Readiness timeout defaults to two minutes (`LOCAL_READY_TIMEOUT=3m` overrides
it). No tests run. The command prints service, probe, metrics and Swagger URLs.

Ignored `.cache/deployment-<project>.json` stores current/previous/pending image IDs,
version labels and migration versions, with no credentials. The helper also writes
a minimal `.cache/runtime-<project>.json` Compose override. Preserve these files
and protected images for rollback. A lock prevents concurrent helper mutations;
check for running helpers before removing a stale `.lock` after a crash.

## Failure and rollback

```powershell
docker compose ps -a
docker compose logs --tail=100 backend
make rollback-local
make deployment-status
```

Failed builds leave the running backend untouched. Startup/readiness failure
returns nonzero, leaves a pending deployment and preserves the previous image.
Rollback is explicit. `observability` refuses pending deployments; retry deploy
or roll back before starting the stack through that command.

Rollback restores the previous **application image**, including frontend, recreates
only backend and waits for readiness. It overrides startup with `./api`, so the
old migration runner never runs. It does not rebuild, downgrade schema or delete
data/volumes. Subsequent `observability` preserves API-only startup until deploy.

Automatic rollback currently requires the **same clean migration version** as
the recorded previous image. Missing, dirty or changed versions are refused before
container mutation. Even additive migrations need manual compatibility review;
see [migration classifications](migrations.md). Version checks cannot detect
manual DDL changes. `migrate-down` is a separate explicit operation and may destroy
data. A first deployment without a previous working image has no rollback target.
Repeated rollback restores the same protected previous image.

Use helper commands consistently for deployment history. Direct Compose startup
remains supported but does not record rollback state. Helper tags are scoped to
`COMPOSE_PROJECT_NAME`; helpers use this repository's `compose.yaml`, not arbitrary
`COMPOSE_FILE` overlays. Do not prune current/previous images.

## Observability without regression tests

```powershell
make grafana
make observability-check
make observability-down
```

`grafana` aliases `observability`: PostgreSQL, backend, ClickHouse, ClickHouse init,
Prometheus and Grafana. It reuses the current image, building only if absent.
Use `deploy-local` to build changed source. It waits for backend readiness,
successful ClickHouse init and service health without running tests. A fresh
installation can correctly show empty learning panels until activity is created.

Quick check validates health, provisioned dashboards/data sources and live metrics.
Full check additionally runs every panel query with required events, isolated SQL
fixtures and ClickHouse outage/disabled-analytics checks. It temporarily stops
ClickHouse and restores it in a finally block. See [observability](observability.md).
`observability-down` and `docker-down` retain all named volumes.

## Isolated clean start

Use a separate PowerShell session, another project and unused ports:

```powershell
$env:COMPOSE_PROJECT_NAME = "knowledge-platform-fresh"
$env:KP_BACKEND_PORT = "18081"
$env:KP_POSTGRES_PORT = "55433"
$env:KP_CLICKHOUSE_PORT = "18123"
$env:KP_PROMETHEUS_PORT = "19090"
$env:KP_GRAFANA_PORT = "13000"
make deploy-local
make grafana
make observability-check
make deployment-status
make observability-down
```

App/Swagger are at localhost:18081 and /swagger/; Grafana at localhost:13000.
Separate volumes, image tags and state preserve the main stack. Down retains
these volumes too. Close that shell or remove overrides before normal commands.
