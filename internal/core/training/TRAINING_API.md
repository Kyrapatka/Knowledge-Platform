# English Training API

All routes below use `/api/v1` and require `Authorization: Bearer <access_token>`.
Apply migrations through **000009** before starting the updated application.
The application does not apply them automatically.

Currently enabled algorithms: `english_basic:v1`, `english_adaptive:v1`.
Track is `default`; continuous English plans have no deadline. Interview and
Formula defaults are stored for their folders, but those algorithms cannot yet
be launched. TemplateKey is provenance, not a compatibility gate: plans validate
the actual card configuration, which must have active question and answer fields.

## Routes

| Method | Route | Result |
|---|---|---|
| GET | `/folders/:folderID/training-config` | Defaults and their version |
| PATCH | `/folders/:folderID/training-config` | Replace defaults using expected version |
| POST | `/training/plans` | Create plan (201) |
| GET | `/training/plans?limit=50&offset=0` | Own plans, stable pagination, limit 1..100 |
| GET | `/training/plans/:planID` | Plan and config snapshot |
| POST | `/training/plans/:planID/cancel` | Cancel plan and active session; retain progress/history |
| POST | `/training/plans/:planID/sessions` | Start or return the existing active session (200) |
| GET | `/training/sessions/:sessionID` | Resume session, refresh due state and refill pool |
| GET | `/training/sessions/:sessionID/current` | Same session view |
| GET | `/training/sessions/:sessionID/result` | Same view including summary; finished sessions are read-only |
| POST | `/training/sessions/:sessionID/actions` | Act on the exact current presentation |
| POST | `/training/sessions/:sessionID/materials/:materialID/skip-rehab` | Skip pending recovery, including before its date |
| POST | `/training/sessions/:sessionID/finish` | Finish session, retain partial mastery |
| POST | `/training/sessions/:sessionID/cancel` | Cancel session, retain committed answers |

GET session endpoints may refill an active pool and persist a new presentation;
repeat reads return the same presentation until it is answered or superseded.
Clients should fetch these intentionally when resuming training, not prefetch them.

## Folder defaults

PATCH body:

```json
{
  "version": 1,
  "training_config": {
    "default_algorithm_key": "english_basic",
    "pool_size": 8
  }
}
```

Pool size must be 1..50. The response returns the next version. Stale versions
produce 409. This version is separate from Workshop's `config_version`. Changing
defaults does not change existing plans. New English folders start with pool 8.

## Create a plan

```json
{
  "source_folder_ids": ["<folder-uuid>"],
  "algorithm_key": "english_basic",
  "pool_size": 8
}
```

Algorithm and pool size are optional: explicit values override folder defaults.
Conflicting defaults across folders require explicit values (400); array order
does not pick a winner. Use 1..100 distinct source folders. Sources must be owned
by the caller. Card/schema configuration is snapshotted at creation.

Two active plans cannot use the same source folder in the same persistent track,
including when the folder is empty. This protects current and future materials
from conflicting schedules. New IDs copied from elsewhere remain independent.

If previous persistent progress uses another algorithm, creation returns 409.
Algorithm migration is a later stage; the API does not silently reset progress.

## Start / resume

POST to the plan's sessions route. The response contains:

- `session`: ID, plan ID, status, start/finish timestamps.
- `current`: presentation or `null`.
- `pool_size`: actual number of active materials, possibly below configured size.
- `summary`: committed session counts.

A presentation has its own `id`, `material_id`, `folder_id`, `kind` (`stage`,
`rehab`, `extra`), `progress_version`, `stage`, counters, `required_correct`,
effective difficulty, and ordered `question` / `answer` arrays:

```json
{"key":"foreign","label":"Foreign","value":"example"}
```

The frontend initially displays the question and reveals the answer on request.
The answer is included in the payload because this is self-check training, not
an exam. Content/difficulty stay fixed for the displayed presentation even if
the material is edited. A later presentation reads current content using the
plan's card snapshot.

If `current` is null, there is no due card now. The user may finish or resume
later. The plan stays active. Materials added to sources are discovered when
the pool next has room; partial mastery survives ending and reopening sessions.

## Answer / manual actions

```json
{
  "command_id": "<new-uuid-for-this-logical-action>",
  "presentation_id": "<current.id>",
  "expected_version": 1,
  "action": "correct"
}
```

Actions: `correct`, `wrong`, `advance`, `rollback`, `skip_rehab`.
`skip_rehab` on this route requires a current recovery presentation. For future
recovery, use the dedicated route below. Manual advance/rollback do not count as
correct/wrong answers. Advancing above Stage 11 or rolling below Stage 1 is 400.

The result contains `event`, the material's `next_review_at`, and the next
`session` view. Progress, the event, pool changes and this exact result are
committed in one transaction.

Use one command UUID per logical action. On timeout, retry with exactly the same
UUID and body. The stored result is returned, including after session completion
or cancellation. Reusing a command UUID with a different request gives 409.
The command namespace is per user and spans sessions.

Two different commands for one presentation cannot both succeed. On 409, fetch
the session again and use its new presentation/version; do not automatically
reinterpret an old answer against a new card. A Stage becoming due also
invalidates an outstanding rehab presentation.

## Skip recovery while waiting

POST to `/training/sessions/:sessionID/materials/:materialID/skip-rehab`:

```json
{"command_id":"<new-uuid>","expected_version":4}
```

The version is available as `event.progress_version_after` from the last action
on that material. Session/plan must be active, the material must be an owned
source material, and recovery must still exist. An ordinary Stage already due
must be reviewed instead (409). The command clears rehab and the extra check,
preserves the Stage timer and records a manual event without a presentation ID.
It has the same atomicity and retry semantics as an answer.

## Summary / errors

Summary fields: `correct`, `wrong`, `advance`, `rollback`, `skip_rehab`,
`materials_reviewed` (distinct materials with actions), and `stage_promotions`
(promotions earned by correct answers). Recovery answers contribute to answer
totals but not Stage mastery. Finish/cancel never erases these counts.

- 400 `invalid_training_request`: invalid settings or action.
- 401 `unauthorized`: missing/invalid access token.
- 404 `training_not_found`: missing or inaccessible resource.
- 409 `training_conflict`: stale version/presentation, command reuse, overlapping
  active plans, incompatible progress algorithm, or inactive plan/session.
- 500 `internal_error`: transaction failed; no partial answer is committed.

## Verification

With `TRAINING_TEST_DATABASE_URL` set to a test PostgreSQL URL:

```text
go test ./internal/core/... ./internal/app ./cmd/api
```

Integration tests create unique schemas and remove only those schemas. They
cover actual HTTP handlers, database transactions, pool recovery after service
reconstruction, concurrent requests and forced failure of event insertion.
