# Knowledge Platform

A learning workspace with Go/Gin, PostgreSQL, React/TypeScript, normal training and Mock/Deep Interview. Local operations use Docker Compose, Prometheus, Grafana and ClickHouse.

## Full local stack (Docker)

With Docker Desktop/Engine and Compose available, run from this directory:

```powershell
make grafana
```

The first run builds the frontend/API, applies PostgreSQL and ClickHouse migrations,
and provisions Grafana automatically. Open the [app](http://localhost:8080),
[Prometheus targets](http://localhost:9090/targets), or [Grafana](http://localhost:3000).
Grafana local credentials: `admin` / `local-grafana-only`.
The **Knowledge Platform - System** and **Knowledge Platform - Learning Analytics**
dashboards are in the Knowledge Platform folder. ClickHouse HTTP is on port 8123;
backend metrics are at [localhost:8080/metrics](http://localhost:8080/metrics).

All published ports bind to loopback; bundled passwords are local development
values. Set local `KP_*` overrides from `.env.example` when needed.
Compose enables analytics by default (`KP_ANALYTICS_ENABLED=true`) independently
of host `ANALYTICS_ENABLED`. Named volumes retain data across restarts.
Use `make deploy-local` after changing application code; `make grafana` reuses the current image. Neither command runs tests. Docker Desktop/Engine must already be running.
See [observability setup, definitions and checks](docs/observability.md).

The [local deployment guide](docs/local-deployment.md) explains image versions, failure diagnostics, safe rollback and an isolated clean start. [CI](docs/ci.md) runs backend, frontend, strict PostgreSQL integration and application E2E on push/PR. [Swagger](http://localhost:8080/swagger/) is bundled with the app.

## Run locally (without Docker)

Requirements: Go 1.25+, Node.js 22.12+ (the project was checked with 22.16), and a running PostgreSQL database.

1. Copy `.env.example` to `.env` if you do not already have one. Set `DATABASE_URL` and a private, random `JWT_SECRET`. Never commit `.env`.
2. From the repository root, install and build the frontend:

   ```powershell
   npm.cmd --prefix web ci
   npm.cmd --prefix web run build
   ```

3. Apply the database migrations, then run the server:

   ```powershell
   go run ./cmd/migrate
   go run ./cmd/api
   ```

4. Open **http://localhost:8080**, create an account, and add a folder. A username is 3–16 characters; a password is 8–128 characters.

On macOS/Linux, use `npm` instead of `npm.cmd`. Run the server from the repository root so it can find `.env` and `web/dist`. The Go server serves both the frontend and API; no second server is needed for manual testing.

The default migration command applies upward migrations and does not reset your data. `make migrate-down` explicitly attempts one rollback; consult [rollback classifications](docs/migrations.md) first. Each migration and its version record are committed together. It also understands the existing `schema_migrations` table used by golang-migrate. Do not run different migration tools concurrently. If a database was previously marked dirty, investigate it before continuing.

## Development

Run `go run ./cmd/api` in one terminal and `npm.cmd --prefix web run dev` in another. Open the Vite address, usually http://127.0.0.1:5173. `/api` is proxied to the Go server. Set `API_URL` if the API runs elsewhere; the proxy preserves the original host for browser authentication.

## What is included

- Registration, login, refresh and logout. Refresh credentials use an HttpOnly, SameSite cookie; access tokens stay in memory.
- Responsive library with folders, search by folder name, selection and training across multiple folders.
- Material creation and editing, one topic per material, topic/search filters, sorting and pagination.
- Plan-specific progress, main/recovery review dates, learning settings, early Final Review and Skip Rehab.
- Mixed training across vocabulary, interview and formula plans. Every material retains its own algorithm and schedule.
- Automatic answer persistence and continuation, with safe retry of uncertain answer submissions. No manual pause, finish or ordinary skip controls.
- Editing during training while preserving the displayed card snapshot.
- Markdown, code blocks and math rendering. Formula exercises can be added and edited in material details.
- Real activity statistics, including answers, materials reviewed, daily activity and stage promotions.

Create a formula material, open its details and add a practice exercise before training it. Topic selection only limits the training sources; it does not alter the topic or the material's schedule. A new Interview plan defaults to a 150-day long-term horizon, which can be changed before starting. CRAM defaults to 5 days.

The last training selection is remembered in this browser, separately for each account. Plans, progress and answers live on the server. There is no offline answer queue; a network failure shows a retry action using the same command ID.

Folder copying from other users, global discovery/search and Russian localization are outside this first frontend version. Operational and learning analytics are available through the provisioned Grafana dashboards.

### Training refinements

- Correct/Wrong are available before revealing the answer; cards swipe right/left after a confirmed response.
- English starts with a random foreign/native side and alternates after each answer. Direction is recorded on the server and survives reloads.
- Examples have a separate reveal control. Exact occurrences of the foreign word or phrase are masked (case insensitive, whole-word boundaries) until clicked.
- Folder settings define active content/organization fields and which appear as questions or answers. Existing shown cards keep their snapshot; future presentations use updated folder fields.
- All displayed times and statistics use Europe/Moscow. Charts support hover, focus and tap details, alongside daily counts.
- Explicit early review is available when the nearest review is less than three hours away. It counts as an ordinary review and moves only that event, preserving other timers.
- Future review intervals are shortened by 30 minutes, including recovery and final reviews. Individual target dates stay intact. Immediate reviews stay immediate. Migration 17 adjusts existing future review dates once; it never repeatedly shifts dates on refresh.

Apply all current migrations and restart the API before testing these server features. Frontend-only checks with mocked API responses: `npm.cmd --prefix web run test:e2e -- refinement.spec.ts`.

## Verification

```powershell
npm.cmd --prefix web run build
go build ./...
npm.cmd --prefix web run test:e2e
```

Browser tests require the API at http://127.0.0.1:8080 with a built frontend and current migrations. They use installed Microsoft Edge by default. Set `PLAYWRIGHT_CHANNEL=chrome` to use installed Chrome, and `E2E_BASE_URL` for a different address. Tests register separate randomly named QA accounts and create their own content; they do not modify existing users' content. Screenshots/traces go to the ignored `web/test-results` directory.

Training/dashboard integration tests use `TRAINING_TEST_DATABASE_URL` as a PostgreSQL URL and create/drop only their own random schemas:

```powershell
go test ./internal/core/... ./internal/app ./internal/webui ./cmd/...
go test ./internal/auth/handler ./internal/auth/service
```

Full integration checks use `TEST_DATABASE_URL` and `TRAINING_TEST_DATABASE_URL`. Both authentication repository and training suites create/drop their own randomly named schemas; use a dedicated test database/account with schema permissions. See the Compose verification commands in [observability documentation](docs/observability.md#verification).

API contract: [OpenAPI](docs/openapi.yaml), [Swagger UI](http://localhost:8080/swagger/) and [usage/validation](docs/api.md). Additional references: [Training API](internal/core/training/TRAINING_API.md) and [Frontend API additions](docs/frontend-api.md).

## Developer checks and deployment origin

The Makefile uses the existing Go migration runner and npm lockfile. It never embeds database credentials.

| Command | Purpose |
| --- | --- |
| `make run` | Run the Go API; build `web/dist` first |
| `make fmt` / `make fmt-check` | Fix formatting / nonmutating tracked Go format check |
| `make test` / `make vet` / `make test-race` | Backend checks individually |
| `make check` | Go formatting, vet, tests and race checks |
| `make test-cover` | Go coverage in `coverage.out` |
| `make test-integration` | Real PostgreSQL tests; fails on missing database variables or skipped tests |
| `make migrate-test` | All migrations in a fresh random schema, then empty-schema 23 down/up |
| `make frontend-check` | TypeScript, Prettier, OpenAPI validation and production build |
| `make openapi-check` | Validate separate OpenAPI spec and references |
| `make observability` / `make grafana` | Start backend + full observability stack, no tests |
| `make observability-check` | Quick live probes, provisioning and metrics check |
| `make observability-full-check` | All panel SQL, fixtures and resilience checks; requires real activity |
| `make observability-down` | Stop stack, retain all named volumes |
| `make deploy-local` | Preserve previous image, build, migrate once, start and await readiness |
| `make rollback-local` | Restore previous compatible app image, no DB downgrade |
| `make deployment-status` | Compose state, deployed image/version and HTTP probes |
| `make test-e2e` | Playwright against an already running API |
| `make test-all` | All of the above checks, including integration and E2E |
| `make dev` | Compose stack including backend; no second Go process |
| `make docker-ps`, `make docker-logs`, `make docker-down` | Inspect or stop the stack; volumes are retained |
| `make migrate-up`, `make migrate-status`, `make migrate-down` | Apply, inspect or explicitly roll back one migration using DATABASE_URL |

Ordinary `go test ./...` may skip PostgreSQL tests without their environment. For strict integration, set both `TEST_DATABASE_URL` and `TRAINING_TEST_DATABASE_URL` to dedicated test PostgreSQL databases with schema creation permissions. The suites isolate data in random schemas. Migration smoke uses only the latter variable. These checks load the existing `.env` convention.

Frontend formatting: `npm --prefix web run format`; checks: `typecheck`, `format:check`, `build`. There is no separate frontend unit framework or ESLint installation. Playwright is intentionally outside the fast check. Observability E2E cases additionally need the stack described in [observability.md](docs/observability.md).

For a TLS tunnel that rewrites Host, set `HTTP_PUBLIC_ORIGIN=https://your-public-host` and restart the API. This explicitly allows that browser origin while preserving direct local development. Do not use a wildcard origin. HTTPS browser responses use Secure, HttpOnly, SameSite refresh cookies.

`HTTP_TRUSTED_PROXIES` is a comma-separated list of the actual immediate proxy IPs/CIDRs; empty means no trusted proxies. Configure it only for your deployment. Forwarded origin and client-IP headers from arbitrary clients are ignored. Without trusted proxy configuration, rate limits apply to the socket peer (tunnel clients may share one limit). `AUTH_LOGIN_LIMIT`, `AUTH_REGISTER_LIMIT`, `AUTH_REFRESH_LIMIT`, `AUTH_RATE_WINDOW`, and `AUTH_RATE_MAX_KEYS` configure bounded process-local limits; they are not shared between replicas.

See [diagnostic logging](docs/logging.md) for `HTTP_SLOW_REQUEST_THRESHOLD` and log priorities.
