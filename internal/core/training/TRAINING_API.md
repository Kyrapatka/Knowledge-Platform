# Training API

## Interview plans and deadlines

Create with `algorithm_key: "interview_cram"` and `horizon_days: 1..7`, or
`algorithm_key: "interview_long_term"` and `horizon_days: 7..365`.
The horizon is required for Interview; English and Formula accept only zero/omitted horizon.
Track is inferred from the algorithm. Default Interview pool size is 5.

Each material's target is learning admission time plus horizon days. Existing
long-term progress keeps its own target when resumed in another plan; a new
plan's horizon applies to newly admitted materials. CRAM progress is isolated
per plan, so multiple CRAM plans can use the same sources independently.

Mastery is 1/2/3 consecutive correct answers for easy/medium/hard. Three
consecutive wrong Stage answers upgrade easy to a user-specific medium override.
CRAM wrong resets mastery without rollback or rehab. Long-term wrong rolls back
one stage for an incoming gap <=30 days, two for a gap >30 days, with Stage 1
as the floor. The new main timer uses the rolled-back stage's gap. Rehab then
uses its own mastery on Day 0 and Day 2; an extra check is scheduled ten days
after the second mastery only if that new main interval exceeds 30 days.

CRAM 1/2 days uses an 8-hour intermediate review. Longer CRAM schedules use
the agreed day-marker gaps. Long-term uses the nearest table, ties shorter,
and replaces its last marker with the requested horizon. Stage 1 starts now;
later nonfinal reviews use the gap from actual mastery. All main dates are
capped at the material's fixed target; the final stage is scheduled at target.
At an overdue target, Final Review takes precedence even after a rollback.
Wrong on Final Review keeps the final task due until mastery; it does not
extend the target or send the material back into recovery.

Early final review is available only after reaching the last stage. Read
`GET /training/sessions/:sessionID/materials/:materialID/progress` for the current
`version`, `target_at`, `stage_review_at`, `next_review_at`, `completed_at` and
`can_start_final`. A material not admitted yet returns 404 without creating progress.
Then POST to `.../materials/:materialID/start-final` with:

```json
{"command_id":"<new-uuid>","expected_version":3}
```

This cancels recovery, makes the final stage due now and returns the usual
action result with a `start_final` event. It does not count as an answer or
complete the material. Retry with the same command ID and body; changed/stale
commands return 409. Presentations mark final tasks with `final_review: true`.
Answer them through the existing actions endpoint. CRAM rejects `advance` and
`rollback`; long-term supports manual one-stage changes without answer counts.

When all currently trainable source materials are completed, the finite plan
and its active session complete in the answer transaction. Not-yet-due and
newly added trainable materials prevent completion; empty cards are excluded.
Materials added after plan completion require another plan. Progress and events
remain available. Migration 000010 adds `start_final` events; its down migration
refuses to discard existing such history.

All routes below use `/api/v1` and require `Authorization: Bearer <access_token>`.
Apply migrations through **000012** before starting the updated application.
The application does not apply them automatically.

Enabled algorithms: `english_basic:v1`, `english_adaptive:v1`,
`interview_cram:v1`, `interview_long_term:v1`, `formula_adaptive:v1`.
English and Formula use track `default` without a deadline. Interview uses `cram`
or `long_term`. TemplateKey is provenance, not a compatibility gate: plans validate
the actual card configuration, which must have active question and answer fields.

## Routes

| Method | Route | Result |
|---|---|---|
| POST | `/materials/:materialID/exercises` | Create Formula exercise (201) |
| GET | `/materials/:materialID/exercises?limit=50&offset=0` | Own exercises, limit 1..100 |
| PUT | `/materials/:materialID/exercises/:exerciseID` | Replace exercise with expected version |
| DELETE | `/materials/:materialID/exercises/:exerciseID?expected_version=1` | Delete exercise (204) |
| GET | `/folders/:folderID/training-config` | Defaults and their version |
| PATCH | `/folders/:folderID/training-config` | Replace defaults using expected version |
| POST | `/training/plans` | Create plan (201) |
| GET | `/training/plans?limit=50&offset=0` | Own plans, stable pagination, limit 1..100 |
| GET | `/training/plans/:planID` | Plan and config snapshot |
| POST | `/training/plans/:planID/cancel` | Cancel plan and active session; retain progress/history |
| POST | `/training/plans/:planID/algorithm` | Explicit KEEP/RESET/SET change after finishing the session |
| GET | `/training/plans/:planID/changes?limit=50&offset=0` | Own plan's change journal |
| POST | `/training/plans/:planID/sessions` | Start or return the existing active session (200) |
| GET | `/training/sessions/:sessionID` | Resume session, refresh due state and refill pool |
| GET | `/training/sessions/:sessionID/current` | Same session view |
| GET | `/training/sessions/:sessionID/result` | Same view including summary; finished sessions are read-only |
| POST | `/training/sessions/:sessionID/actions` | Act on the exact current presentation |
| POST | `/training/sessions/:sessionID/materials/:materialID/skip-rehab` | Skip pending recovery, including before its date |
| GET | `/training/sessions/:sessionID/materials/:materialID/progress` | Existing material progress, version, dates and `can_start_final` |
| POST | `/training/sessions/:sessionID/materials/:materialID/start-final` | Bring a reached final stage forward |
| POST | `/training/sessions/:sessionID/finish` | Finish session, retain partial mastery |
| POST | `/training/sessions/:sessionID/cancel` | Cancel session, retain committed answers |

GET session endpoints may refill an active pool and persist a new presentation;
repeat reads return the same presentation until it is answered or superseded.
Clients should fetch these intentionally when resuming training, not prefetch them.

## Formula exercises and training

Create a plan with `algorithm_key: "formula_adaptive"`. Formula folder defaults
use pool size 5. The plan uses active card/schema fields and exercises attached
to each material; no particular TemplateKey is required. At least one exercise
is required for a material to enter the Formula pool. This includes early stages.
Adding an exercise later makes the material eligible on the next refill.

Exercise POST body (PUT also requires `expected_version`):

```json
{
  "problem": "A car travels 100 km in 2 hours. Find its average speed.",
  "answer": "50 km/h",
  "solution": "v = s/t = 100/2 = 50 km/h",
  "hint": "Divide distance by travel time."
}
```

Problem, answer and solution are required nonblank strings (maximum 64000 UTF-8
bytes each); hint is optional (maximum 16000 bytes). Responses include `id`,
`material_id`, `version`, `created_at`, `updated_at` and the four text fields.
PUT replaces all four fields, including clearing an omitted hint. Stale versions
return 409; another user's material/exercise or an ID attached to another material
returns 404. DELETE requires the current `expected_version` query parameter.

Formula presentations add `exercise_id`, `exercise_version` and `practice_mode`:

| Mode | Before attempting (`question`) | Used for |
|---|---|---|
| `worked` | Problem, formula/card content, explanations, full solution and answer | Stage 1 and first recovery day |
| `faded` | Problem and optional hint | Stage 2 and refresher after wrong S4-S6 |
| `independent` | Problem only | Stage 3, second recovery day and extra check |
| `mixed` | Problem only; formulas are mixed across the pool | Stages 4-9 |
| `maintenance` | Problem only | Stage 10 |

The `answer` array contains the formula context, solution and answer to reveal
after the attempt. Independent modes do not place formula names or hints in
`question`; the frontend should also avoid showing a material title separately.
The response is a self-check payload, not a secured exam: solutions are already
present in `answer`. Submit `correct` or `wrong` through the existing actions API.
Worked-mode success means the learner understood the demonstrated solution.

Mastery is 1/2/3 consecutive successes for easy/medium/hard. Wrong S1-S3 resets
mastery without rollback. Wrong S4-S6 enables faded support, removed after the
next correct answer. Wrong S7+ starts a worked refresher, followed by independent
practice two days after its mastery. If the second-day or extra check is wrong,
the material rolls back one stage and restarts recovery. Recovery never advances
Stage mastery. After the second recovery mastery, a separate check is scheduled
in ten days only if the main interval exceeds 30 days. Stage checks take priority;
skip preserves the main timer.

Stage markers are 0, 1, 3, 7, 14, 30, 60, 120, 240, 365 days. Each subsequent
gap starts at actual mastery. Successful Stage 10 reviews repeat after 365 days;
the plan stays active and has no Final Review. Manual advance/rollback changes
one stage without incrementing answer counts; rollback starts recovery. A manual
rollback to Stage 1 uses a one-day main timer.

Exercise selection avoids the last answered exercise when another exists.
Resuming does not reroll a displayed task. Editing/deleting an exercise preserves
that task and its answer receipt; subsequent tasks use current exercises. If all
exercises are deleted, the already displayed task may be answered, then that
material leaves the pool until an exercise is added. Progress is retained.
Events record `exercise_id` and `practice_mode` even after exercise deletion.
Exercise creation is ordinary CRUD; answer idempotency remains command-based.

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
Use an active plan with that existing algorithm and the explicit change endpoint
below. Creating a plan does not silently reset or migrate progress.

## Change algorithm or horizon

Finish or cancel the active session first, keeping the plan active. Fetch the
plan's `version` (configuration version, distinct from any material's progress
version), then POST `/training/plans/:planID/algorithm`:

```json
{
  "command_id": "<new-uuid-for-this-change>",
  "expected_version": 1,
  "algorithm_key": "english_adaptive",
  "algorithm_version": 1,
  "mode": "keep"
}
```

The target must be registered and use the same track. Supported modes:

| Mode | Stage | Schedule |
|---|---|---|
| `keep` | Preserve each material's current stage | Last Stage review plus target algorithm's interval |
| `reset` | Stage 1 | Due now, restart individual learning start/horizon |
| `set` | Body must include `stage: N` | Last Stage review plus the interval of N |

If no Stage review exists, KEEP/SET use the individual learning start as anchor;
initial Stage 1 is due now. Elapsed time is retained, so shortening an interval
may make a material immediately overdue. A stage outside the target schedule
returns 400 for the entire command; it is never silently clamped.

All modes preserve historical correct/wrong totals and difficulty overrides,
clear partial mastery and recovery, and increment affected progress versions.
KEEP preserves completed progress; RESET/SET reopen it. Only existing progress
of live source materials is changed. New materials start at Stage 1 when admitted,
using the new plan settings. Removed content/history is left intact.

For Interview, optional `horizon_days` changes each material's target relative to
its own learning start. RESET uses now as the new start; KEEP/SET retain the old
start. Omission preserves each existing material's horizon. Plan settings govern
future admissions. Allowed ranges remain CRAM 1..7 and LONG_TERM 7..365. A new
final stage stays anchored to its target, with overdue targets taking priority.

The result contains `plan` (including the next configuration version) and
`changes` (material IDs, before/after stages and progress versions, next dates).
Plan, progress, journal and command receipt commit together. Exact retries return
the original result; stale versions or reused IDs with another body return 409.
An active session or inactive plan also returns 409. Cross-track conversion
returns 400: create a separate plan to start that track independently.

The journal endpoint supports limit 1..100 and nonnegative offset. Each entry
contains ID, plan/user/command IDs, creation time and `details` with the change
request, previous algorithm and committed result. Ordinary training answer
statistics are not incremented by algorithm changes.

## Material and folder deletion

Existing library DELETE routes now perform soft deletion. Deleted materials and
folders return 404 through ordinary library/exercise reads and cannot enter new
training tasks. Folder deletion also marks its materials. Live sessions discard
deleted cards when refreshed. Answering a now-deleted card returns 409; a retry
of a previously committed answer still returns its stored receipt.

Progress, exercise records, plan sources, events and summaries remain in the DB.
Plan/session history remains accessible even if a source folder was deleted.
An empty plan remains available to cancel; deletion does not manufacture learning
completion. Finite-plan completion otherwise considers currently live trainable
materials. Restoration and permanent purging are not exposed by this API.
Migration 000012 down refuses rollback while deleted content or change history
exists, preventing accidental resurrection or history loss.

## Start / resume

POST to the plan's sessions route. The response contains:

- `session`: ID, plan ID, status, start/finish timestamps.
- `current`: presentation or `null`.
- `pool_size`: actual number of active materials, possibly below configured size.
- `summary`: committed session counts.

A presentation has its own `id`, `material_id`, `folder_id`, `kind` (`stage`,
`rehab`, `extra`), `final_review`, `progress_version`, `stage`, counters, `required_correct`,
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
later. English/Formula stay active; Interview finishes only when all currently trainable
source materials have completed their final reviews. Empty source folders alone
do not complete a plan. Materials added to sources are discovered when
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
