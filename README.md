# Knowledge Platform

First frontend MVP: an English-language, dark learning workspace built with React and TypeScript, backed by the Go API and PostgreSQL.

## Run locally

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

The migration command applies upward migrations only and does not reset your data. Each migration and its version record are committed together. It also understands the existing `schema_migrations` table used by golang-migrate. Do not run different migration tools concurrently. If a database was previously marked dirty, investigate it before continuing.

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

Folder copying from other users, global discovery/search, Russian localization and advanced analytics are outside this first frontend version.

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

**Do not run `go test ./...` against a working database.** The older `internal/auth/repository/postgres` tests truncate their configured users/sessions tables. They require a disposable database and are deliberately excluded from the verification commands above.

API references: [Training API](internal/core/training/TRAINING_API.md) and [Frontend API additions](docs/frontend-api.md).
