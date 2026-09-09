# Training implementation status

The latest conversation rules supersede conflicting examples in the supplied
specification. The current implementation contains:

- Material difficulty through HTTP, service, domain and PostgreSQL (default medium).
- Progress with per-material learning start/target, separate rehab counters and
  separate Stage, rehab and extra-review dates.
- Pure English basic/adaptive v1 transitions, manual actions and version registry.
- Interview CRAM/long-term v1, individual deadlines, rollback, difficulty mastery and final review.
- Formula adaptive v1, exercise CRUD, worked/faded/independent/mixed/maintenance
  presentations, recovery and yearly maintenance.
- Folder training defaults, resolved plan/card snapshots and active-source conflict checks.
- Plans, persistent sessions/pools/presentations, atomic events and command receipts.
- Transactional KEEP/RESET/SET algorithm changes, individual horizon updates,
  plan configuration versions and an accessible change journal.
- Soft deletion of materials/folders with training history retained.
- Authenticated English/Interview/Formula HTTP endpoints, including recovery skip,
  material progress, exercise versions and early Interview final review.

See [TRAINING_API.md](TRAINING_API.md) for endpoint contracts and a frontend flow.

## Recovery rules

An algorithm first handles the Stage result/rollback and establishes the main
schedule. It can then start recovery without changing that schedule:

1. Reinforce in the pool now until that day's mastery is reached.
2. Skip the following day. Reinforce again two days after first-day mastery.
3. If the main scheduled interval is strictly greater than 30 days, schedule a
   separate extra check ten days after second-day mastery.

Recovery has its own consecutive-correct counter; it never advances Stage
mastery. Historical correct/wrong totals include recovery answers. In English and
Interview, a wrong rehab answer resets only that day's mastery, and a wrong extra
check restarts reinforcement. Formula additionally rolls back one stage when its
independent second-day or extra check fails, then restarts the refresher.
The current English implementation uses its usual 3 or 3/4/5 mastery requirement
for each rehab learning day and one correct answer for the extra check.

An ordinary due Stage review takes priority over due rehab and extra checks,
including older overdue recovery actions. Processing it cancels all outstanding
recovery. If the ordinary answer is wrong, the algorithm may start new recovery.
Skipping recovery also preserves the Stage dates.

NextReviewAt() and the PostgreSQL generated next_review_at return the earliest
timestamp across all active actions. DueReview(now) separately selects the
action with Stage priority. Reading never mutates/cancels state.

Example with a 40-day main interval: first reinforcement Day 0, rest Day 1,
second reinforcement Day 2, extra check Day 12, normal Stage review Day 40.
If actual mastery is completed later, subsequent recovery dates use that actual
completion time. Times are UTC, with day intervals preserving time of day.

## Identity and scheduling

Persistent identity: user + material + track, PlanID nil. CRAM identity: user +
material + PlanID. Each new CRAM starts at Stage 1. Copies with new material IDs
are independent; there is no content deduplication or shared canonical identity.

PlanConfig.HorizonDays is applied afresh when a material starts learning. A
material added on Day 100 with a 150-day horizon gets its own target 150 days
after that start. Plan has no shared TargetAt. NewProgress creates the first
stage due immediately; the first StageLastReviewAt remains nil until reviewed.

Long-term values 1, 2, 5, ... are day markers; their gaps are 1, 3, ... . Day 90
after Day 65 is a 25-day gap, not a 90-day interval. English's separate table
lists Stage cooldowns (1, 1, 1, 6, ...); its final stage repeats yearly.

Formula day markers are 0, 1, 3, 7, 14, 30, 60, 120, 240, 365. Gaps are applied
from actual mastery; successful Stage 10 reviews repeat every 365 days. Formula
plans use persistent default-track progress without a deadline or final review.
Wrong S1-S3 resets mastery; wrong S4-S6 adds a faded refresher until one correct
answer; wrong S7+ starts recovery. Formula uses the latest two-learning-day Rehab
rule, superseding the older Formula-specific one-day example. Stage priority,
the >30-day extra-check threshold and skip semantics remain shared.

For MVP, each Formula practice task requires at least one exercise with a problem,
answer and full solution. Materials without exercises may exist in the library
but are excluded from the pool until an exercise is added. Exercise edits/deletion
do not change a displayed snapshot. New tasks prefer a different exercise from
the previous answered task when another exists. Correctness is self-reported.

## Persistence and verification

Migrations 000007 through 000012 must be applied before running the updated
API. They have only been exercised against an isolated test cluster, not the
application database. Material/folder DELETE marks deleted_at and hides content
from library reads, exercise access and training. Folder deletion also marks its
materials. Progress, exercises, plans and events remain stored. Plan cancellation
retains history; no physical plan-delete API or content-restore API is provided.

The Progress repository increments Version using a conditional update. The
answer service saves Progress, TrainingEvent, command receipt and pool changes
in one transaction. PostgreSQL row locks serialize training commands per user;
this covers plan overlap, absent progress creation, session creation/cancellation
and concurrent answers. Library creation/deletion acquires the same user lock
to serialize material admission, folder deletion and answers. This approach can serialize
independent sessions belonging to one user, while different users remain independent.

The runtime candidate query is scoped to the plan's owned source folders and
filters untrainable cards. English due materials precede new ones, with stable creation/ID ties.
Interview and Formula randomly select eligible materials across all sources.
Answered materials move to the end of the pool. Empty pools remain resumable and
do not complete continuous English/Formula plans. A persistent-track plan
cannot share a source folder with another active plan of the same track; sources
are dynamic, so this also covers materials added to those folders later.

The low-level Progress ListDue method is not the runtime authorization boundary.

Run unit tests: go test ./internal/core/...

Set TRAINING_TEST_DATABASE_URL to run the PostgreSQL integration test. It uses
a unique schema and rolls back all migrations and fixtures; it never truncates
existing application tables. It checks migrations up/down, nullable identity
uniqueness, CRAM isolation, optimistic-lock conflicts, zero/NULL persistence and
generated due dates. HTTP tests also exercise restart/resume, card snapshots,
dynamic pool replenishment, mastery across sessions, two-device conflicts,
concurrent duplicate requests, event failure rollback, cancellation, defaults,
Stage priority and manual skip between rehab days. Formula tests also cover all
day markers, mastery levels, recovery rollback, exercise CRUD ownership/versioning,
content support by mode, exercise rotation, and edits/deletion during a live task.
The final-stage tests cover change rollback on journal failure, elapsed-time
recalculation, horizon updates, retries/versions, deletion during answers, folder
deletion during material creation, and retained history after deletion.

## MVP handoff

The planned Training implementation stages are complete. Apply migrations through
000012 to the target environment before connecting the frontend; tests do not
migrate the working application database. Run the documented frontend scenarios
for English, Interview and Formula when the UI is connected.

The API enables English basic/adaptive, Interview CRAM/long-term and Formula
adaptive v1 (standard exercise/self-check workflow). Changing an active plan's
algorithm requires finishing its session and an explicit KEEP/RESET/SET command.
Creating a plan never silently migrates existing incompatible progress.
Cross-track changes use a new plan. Public-folder copying, content deduplication,
search, configurable Formula practice-style weights and automated grading are
outside this MVP implementation.
