# Knowledge Platform — hardening и responsive Interview

Отчёт по изменениям относительно исходного коммита b7e6bf9. Часть изменений уже включена пользователем в 85021e4; этот коммит не переписывался. API routes, JSON contracts, training algorithms и production-схема не менялись. Последующие исправления находятся в рабочем дереве.

## 1. Architecture

**Interview before:** Handler создавал Store из GORM DB; Store одновременно проверял ready/content/version, нормализовал concepts, координировал imports и выполнял SQL.

**Interview after:** Handler → Service → Repository/Transaction → PostgreSQL. Handler разбирает HTTP и отображает ошибки. Service владеет готовностью question/answer, статусами/версиями, типом папки, concept validation и import orchestration. Store сохраняет GORM, загрузку, CRUD и атомарную транзакцию с account lock. Folder importer присоединяется к уже открытой транзакции через TransactionStore; вложенного независимого commit нет. Composition root создаёт зависимости явно.

**Training before:** один Service и широкий RuntimeStore/Tx. Фактические use cases: plan/config/horizon, session/current/answer/recovery/undo, combined, mock/graph Interview, formula exercises, material progress views и чистые algorithms.

**Training after:** отдельно ExerciseService (5 transactional CRUD operations) и ProgressService (4 read operations). Facade сохраняет текущие вызовы Handler. Adapters сужают доступ без изменения account lock/transaction semantics. Выделение проверялось постепенно.

**Сознательно оставлено:** session/progress/undo/command receipt orchestration вместе, так как одна команда меняет их атомарно. Algorithms не имеют HTTP/GORM/SQL dependencies и не менялись. Generic repository, новый auth framework и лишняя инфраструктура не добавлялись.

## 2. Makefile / developer experience

Targets: run, fmt, vet, test, test-race, test-cover, check, docker-up/down/ps/logs, migrate-up/down/status/version/test, dev, test-integration, frontend-check, test-e2e/frontend-e2e, test-all. Все command targets PHONY.

DB URL: существующий DATABASE_URL через config/.env, без credentials в Makefile. Используется Go migration CLI проекта. dev запускает Compose, который уже содержит backend и migration step; второй go run не запускается. docker-down сохраняет volumes. check последовательно запускает fmt/vet/test/race; тяжёлые E2E остаются в test-all.

## 3. Tests

- Unit/domain: existing pure algorithms, interview graph/seed, material validation, config, middleware, analytics.
- Service: Interview ready invariant/stale version/folder validation; existing auth rotation/reuse, training retries/undo/recovery.
- Handler/HTTP и cross-module: existing PostgreSQL suites для training, ownership, combined, CRAM/long-term, graph, mock, imports/reimport, stale/concurrent commands.
- Repository: auth теперь проверяется на реальных migrations 1–2; добавлена concurrent refresh CAS/invalidated-session проверка. Training progress проверяется на PostgreSQL.
- Migration-sensitive: runner pair/order audit и rollback guards 21/23 на реальной БД.
- Frontend: TypeScript/build/Prettier; отдельного unit framework нет. Browser/UI тесты — существующий Playwright.
- Новые полезные assertions: font hierarchy, горизонтальный overflow, длинный код/списки/ответы, локальный Next scroll, сохранение позиции уже видимого вопроса; screenshots служат ручной проверке, snapshot-baselines не добавлены.

Обычный go test может skip PostgreSQL без TEST_DATABASE_URL/TRAINING_TEST_DATABASE_URL. Строгий test-integration требует обе переменные, принудительно запускает -count=1 и завершает с ошибкой при любом skip. Проверено, что отсутствие env даёт понятную ошибку и ненулевой exit.

Observability Playwright требует OBSERVABILITY_E2E=true и ClickHouse/Grafana Compose stack; без него два теста skipped, а не PASS. Для проверок использована отдельная локальная PostgreSQL на 55439 и отдельный E2E API на 18080; данные приложения не сбрасывались.

## 4. Migrations

23 up/down пары; новых миграций нет. [Подробная классификация всех 23](migrations.md).

- Fully reversible для произвольных production-данных: нет; index-only операции обратимы, но соответствующие пары содержат и data changes.
- Conditional: 10, 12, 15, 18, 19, 21, 22, 23; часть из них дополнительно data-sensitive.
- Data-sensitive/practically irreversible: 1–9, 11, 13, 14, 16, 17, 20, плюс отмеченные conditional пары.
- Migration 21 больше не DELETE-ит новые graph roles ради успешного down: проверяет несовместимые links/imports/bank metadata и отказывает до удаления данных.
- Runner использует advisory lock и транзакционное DDL+version, отвергает dirty state. Smoke удаляет только созданную UUID schema; cleanup failure теперь ошибка.
- Query/index audit: существующие progress/session/receipt/undo/material/auth/interview indexes покрывают проверенные запросы. Новых indexes без подтверждённого query reason не добавлено. Возможное prefix-overlap alias index оставлено до EXPLAIN на реальных объёмах.

Migration smoke: PASS. Все up → empty-schema down23 → up23. Отдельные tests подтверждают отказ rollback при planless history и bank metadata.

## 5. Security

**Already good:** Argon2id; refresh hashes вместо raw tokens; browser rotation через atomic compare-and-swap; logout/revoke; owner-scoped queries; основные import byte/item/UTF-8/JSON limits; CSP, nosniff и Referrer-Policy в webui. Это сохранено.

**Found / fixed:** отсутствовавшие bounded auth limits; неявное доверие forwarded headers; недостаточно строгий JSON reader Interview; deployment-origin mismatch через туннель.

- Login/register/refresh: отдельные configurable fixed windows, bounded map + lazy cleanup, 429 и Retry-After. Native/browser endpoints делят лимит операции.
- Gin и origin logic используют единый явный HTTP_TRUSTED_PROXIES; по умолчанию никому не доверяют.
- HTTP_PUBLIC_ORIGIN задаёт известный публичный origin для TLS tunnel с переписанным Host. Значение валидируется. В локальной .env выставлено https://platform.loca.lt; credentials не менялись.
- Refresh cookies сохраняют HttpOnly/SameSite/Path/expiry; Secure включён для принятого HTTPS origin, direct HTTP development работает.
- Interview JSON: 2 MiB, UTF-8, неизвестные поля/несколько JSON values отклоняются; answers/sources имеют field bounds.
- Passwords, Authorization, токены, cookies, DSN credentials, payloads не добавлялись в logs.

Локальная диагностика: readiness 200; POST /api/v1/auth/browser/refresh с разрешённым origin и без cookie → ожидаемый 401 invalid_refresh_token; чужой origin → 403 invalid_origin. Повторная внешняя проверка получила HTTP 503 — Tunnel Unavailable. Процесс localtunnel отсутствует. Его запуск отклонён автоматической проверкой безопасности, поскольку требуется явное согласие пользователя на публикацию API через сторонний tunnel. Обход блокировки не выполнялся.

**Remaining:** trusted peer CIDRs зависят от реального deployment. Без них tunnel clients делят rate bucket socket peer. In-memory limiter имеет process scope; распределённый limiter в эту задачу не добавлялся. HSTS не включался для mixed local HTTP setup.

## 6. Logging

Одна существующая structured система; один основной record на request. Поля method/route/status/duration/request_id. Query string отсутствует.

| Case | Result |
| --- | --- |
| unmatched | WARN, reason=unmatched, URL.Path отдельным log field |
| API 404 | WARN, без второго warning |
| slow | WARN, HTTP_SLOW_REQUEST_THRESHOLD, default 1s |
| context canceled | INFO, не ERROR по самому факту cancellation |
| deadline | WARN |
| unexpected 5xx | ERROR |
| panic | ERROR + safe marker + stack; приоритет над остальными |
| analytics buffer_full | Aggregated WARN: count/queue_len/queue_capacity |
| invalid | Aggregated WARN без payload |
| closed | DEBUG до финального worker report; после exit только metrics |
| shutdown_timeout | ERROR при реальной потере: pending_events/timeout/queue |
| flush_error | Throttled ERROR: batch_size/safe error class; retry не существует |

Реальные unmatched paths: /favicon.svg, /interview, /train, /assets/index-*.js и /assets/markdown-*.js. Успешные SPA/assets ответы тоже проходят через fallback и дают unmatched; это не доказательство недоступности API. Raw path/request ID/UUID/error string не добавлены в Prometheus labels.

## 7. Comments / docs

Удалены устаревшие/декоративные комментарии composition root и старое обещание будущего answer service. Русские technical comments в material/folder/workshop/auth переведены или удалены, когда повторяли код. Новые WHY объясняют transaction reuse, receipt atomicity, ограничение proxy trust, bounded limiter, безопасный rollback и отсутствие payload в logs. Добавлены package docs internal/core/interview/doc.go и internal/core/training/doc.go. Обновлены README, training README, logging docs; добавлена migration classification.

## 8. Frontend tooling и responsive Interview

Package manager: npm (package-lock.json). Scripts: typecheck, build, format, format:check, check, test/test:e2e. Prettier существовал; добавлены scripts/ignore, существующие неформатированные файлы приведены к его правилам. ESLint отсутствовал и не добавлен. Fast frontend-check не запускает Playwright.

Изменены RecallCard/TrainingPage, InterviewRun, scoped styles и общий hook question-scroll. Отдельный mobile component не создан. Компактной навигации добавлены aria-label, поскольку на tablet скрытый текст лишал links доступного имени. Полный E2E обнаружил рассинхронизацию initialConfig: добавлены неактивные keywords/concepts, которые backend требует сохранять при обновлении шаблона.

| Typography | До | После mobile | После desktop |
| --- | --- | --- | --- |
| Recall Interview question | общий lead 30–42px; mobile 24px | 19px / 1.4, weight 600 | 24px / 1.4, weight 600 |
| Recall short answer | общий тяжёлый lead | 16px / 1.5, weight 500 | 18px / 1.5 |
| Recall detailed answer | мог наследовать lead 30–42px | 15px / 1.6, weight 400 | 16px / 1.6 |
| Mock question | clamp 25–34px, mobile 25px | 19px / 1.4 | 24px / 1.4 |
| Mock detailed answer | 16px / 1.75 | 15px / 1.6 | 16px / 1.6 |
| Mock short answer | body hierarchy | 16px / 1.5 | 17px / 1.5 |
| Secondary | часть labels 9–11px | ключевые labels 12px; sources 13px | metadata 12–13px |
| Recall action buttons | 18px/64px high; mobile17px/62px | 16px, min-height56px | 16px, min-height56px |

Spacing: mobile recall card 20px18px → 16px, face24px12px → 16px10px; section gap24px → 18px desktop/14px mobile. Interview mobile использует всю доступную ширину main с внутренним padding, вместо дополнительного calc(100%-32px). Mock conversation28px32px →24px desktop,20px16px→16px mobile; question vertical34→24 desktop и26→18 mobile; min-height230→160 desktop и200→140 mobile; reference padding22→18 desktop/14 mobile; gap24→18 desktop/16 mobile. Prose ограничен70ch, длинный code/слова переносятся, существующий local answer scroll сохранён.

Scroll transition: hook отслеживает смену question ID и после render измеряет начало контейнера. Если оно уже видно — ничего не делает. Иначе локальный scrollIntoView(block:start) с учётом header/scroll-margin; smooth только без prefers-reduced-motion. Initial render и раскрытие ответа не вызывают безусловный scroll. window.scrollTo(0,0) не добавлен.

Проверены short/long question, short/detailed answer, lists, code, newlines, длинные слова, mobile portrait320/375/390/430, tablet768, desktop1440/1920, Next после длинного ответа. Desktop сохранил отдельную typography. Visual review screenshots390/1440 и browser assertions подтверждают читаемость/отсутствие horizontal overflow.

## 9. Files changed

- `.env.example`
- `Makefile`
- `README.md`
- `cmd/migrate/main.go`
- `cmd/migrate/main_test.go`
- `cmd/test-integration/main.go`
- `compose.yaml`
- `config/config.go`
- `config/security_test.go`
- `docs/hardening-report.md`
- `docs/logging.md`
- `docs/migrations.md`
- `internal/app/app.go`
- `internal/auth/handler/browser.go`
- `internal/auth/handler/browser_test.go`
- `internal/auth/repository/postgres/repository_test.go`
- `internal/auth/repository/postgres/rotation_test.go`
- `internal/auth/service/logout.go`
- `internal/core/folder/importer/service.go`
- `internal/core/folder/repository/postgres/repository.go`
- `internal/core/folder/repository/repository.go`
- `internal/core/folder/workshop/service/service.go`
- `internal/core/folder/workshop/validator.go`
- `internal/core/interview/doc.go`
- `internal/core/interview/handler.go`
- `internal/core/interview/handler_test.go`
- `internal/core/interview/import.go`
- `internal/core/interview/profile.go`
- `internal/core/interview/repository.go`
- `internal/core/interview/service.go`
- `internal/core/interview/service_test.go`
- `internal/core/interview/store.go`
- `internal/core/interview/store_transaction.go`
- `internal/core/material/service/service.go`
- `internal/core/training/README.md`
- `internal/core/training/doc.go`
- `internal/core/training/handler/interview_bank_test.go`
- `internal/core/training/handler/interview_graph_test.go`
- `internal/core/training/handler/interview_normal_test.go`
- `internal/core/training/handler/interview_reimport_test.go`
- `internal/core/training/repository/exercise.go`
- `internal/core/training/repository/postgres/progress.go`
- `internal/core/training/repository/progress_view.go`
- `internal/core/training/repository/runtime.go`
- `internal/core/training/service/exercise.go`
- `internal/core/training/service/material_progress.go`
- `internal/core/training/service/service.go`
- `internal/platform/analytics/diagnostics_test.go`
- `internal/platform/analytics/worker.go`
- `internal/platform/httpmiddleware/diagnostics_test.go`
- `internal/platform/httpmiddleware/logging.go`
- `internal/platform/httpmiddleware/logging_test.go`
- `internal/platform/httpmiddleware/security.go`
- `internal/platform/httpmiddleware/security_test.go`
- `migrations/000021_interview_question_bank.down.sql`
- `web/.prettierignore`
- `web/package.json`
- `web/src/App.tsx`
- `web/src/editors.tsx`
- `web/src/fields.tsx`
- `web/src/import-folder.tsx`
- `web/src/interview-editor.tsx`
- `web/src/interview-types.ts`
- `web/src/interview.css`
- `web/src/interview.tsx`
- `web/src/question-scroll.ts`
- `web/src/recall-card.tsx`
- `web/src/styles.css`
- `web/src/training.tsx`
- `web/tests/interview-bank.spec.ts`
- `web/tests/interview-normal.spec.ts`
- `web/tests/interview-responsive.spec.ts`
- `web/tests/mobile-collection-recall.spec.ts`
- `web/tests/mobile-editor-speech.spec.ts`
- `web/tests/mock-interview.spec.ts`
- `web/tests/mvp.spec.ts`
- `web/tests/observability.spec.ts`
- `web/tests/seed-reimport.spec.ts`

Ignored runtime artifacts (.cache, web/dist, screenshots/reports) и локальная .env не включены в tracked diff. Credentials не публикуются.

## 10. Verification

| Check | Result |
| --- | --- |
| go fmt ./... | PASS |
| go vet ./... | PASS |
| go test ./... | PASS, с обеими test DB env |
| go test -race ./... | PASS, с PostgreSQL; поздние изменения migration/analytics повторно проверены с race |
| make test-integration | PASS: 91 tests, no skips |
| missing integration env | PASS: ожидаемый nonzero exit с понятной причиной |
| make migrate-test | PASS: 23 up, down23, up23, cleanup |
| frontend typecheck | PASS |
| frontend build | PASS |
| frontend format check | PASS |
| separate frontend unit suite | NOT RUN: отсутствует в проекте |
| Playwright application/UI | PASS по итогам повторов: 35 разных сценариев |
| Playwright observability | NOT RUN: 2 skipped, нет Compose analytics stack |
| Docker stack | NOT RUN: daemon pipe unavailable |
| local API readiness/origin | PASS: 200 / allowed-origin401 / foreign-origin403 |
| public tunnel | FAIL: HTTP503 Tunnel Unavailable; восстановление требует explicit approval |

Первый полный E2E прогон: 32 passed, 3 failed, 2 skipped. Два падения выявили отсутствующие keywords/concepts в initialConfig; после исправления весь mvp.spec.ts прошёл (3 tests). Третий сценарий прерывал импорт 368 вопросов на 10-секундном ожидании скрытия dialog; теперь он ждёт конкретный HTTP response (до90s), проверяет success и затем UI. Его повтор прошёл за23.4s. Assertions бизнес-поведения не ослаблялись. Все 21 responsive checks прошли в полном прогоне.

Первый backend запуск завершился FAIL из-за отключённой тестовой PostgreSQL; после восстановления выполнен полный успешный make check. Этот failed run не считался PASS.

Логи проверок находятся в ignored .cache: backend-check.log, integration-check.log, migration-smoke.log, frontend-check.log, playwright.log, playwright-recheck.log, playwright-import-recheck.log, final-race-targeted.log. Playwright artifacts при targeted rerun могут заменять предыдущий каталог; текстовые логи сохраняют результаты всех прогонов.

## 11. Remaining recommendations

- После явного согласия пользователя восстановить public localtunnel на platform.loca.lt: сейчас внешний адрес возвращает 503 Tunnel Unavailable. Локальный origin path и regression tests пройдены.
- Выполнить два observability E2E с доступным Docker/ClickHouse/Grafana и OBSERVABILITY_E2E=true. Docker daemon в окружении проверки недоступен; инфраструктурный прогон не заявляется как PASS.
- Перед production rollback использовать migration classification; исторические destructive downs не гарантируют сохранность данных. Возможный alias-index overlap проверять через EXPLAIN на representative dataset.

## Release-candidate follow-up (2026-10-01)

Audit preserved existing proxy trust, exact browser Origin validation, HttpOnly/
SameSite/Secure cookies, bounded per-process authentication limits, safe SQL query
parameters and sorting allowlists, generic internal errors, readiness-before-drain
and worker/database shutdown ordering. HTTP settings remain read-header 5s, read
60s, write 5m, idle 60s and max headers 1 MiB; the longer write timeout supports
large imports. Training/Interview handlers already bound JSON to 1/2 MiB and file
imports to 10 MiB plus multipart overhead.

Added missing ordinary JSON limits: auth 64 KiB; folder/material/workshop 1 MiB.
Known oversized bodies return JSON 413; chunked overflows are bounded by
MaxBytesReader and existing bind errors return 400. Existing import/training
limits remain intact. Tests cover known/chunked input and import exemption.
Bundled Swagger uses local assets and external initializer JavaScript under the
existing CSP, without CDN or inline-script exceptions. Detailed current regression
results are recorded separately in the release-candidate report.
