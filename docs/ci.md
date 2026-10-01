# Continuous integration

`.github/workflows/ci.yml` runs on push and pull request, with read-only repository
permissions, Go/npm caches and cancellation of older runs for the same ref.

| Job | Checks |
| --- | --- |
| backend | Go from go.mod; nonmutating tracked-file fmt-check, vet, test, race |
| frontend | Node 22, npm ci, TypeScript, Prettier, OpenAPI, production build |
| integration | PostgreSQL 16; both DB variables, strict runner without skipped tests, migration smoke |
| e2e | After other jobs; dedicated PostgreSQL 16, migrations, built UI, API, /ready, managed Chromium, application Playwright tests |

The backend job has no DB; ordinary Go tests may skip DB suites there. The strict
integration job runs them against real isolated schemas and fails on any skip.
Migration smoke uses a fresh empty schema for the complete up/down/up chain; it
does not prove that destructive downs are safe on populated user data.

E2E uses only disposable test credentials and raises auth limits on that dedicated
instance. Failure reports, traces, screenshots and API logs are retained for seven
days. API shutdown runs even after failure. Browser installation follows
[Playwright CI guidance](https://playwright.dev/docs/ci). Locally Edge remains the
default; `CI=true` selects managed Chromium unless PLAYWRIGHT_CHANNEL overrides it.

Observability E2E remains opt-in via OBSERVABILITY_E2E=true and is excluded from
ordinary CI. There is no remote deployment, publishing or production secret use.
Local equivalents do not certify GitHub-hosted runs; inspect jobs after pushing.
