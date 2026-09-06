# Training implementation status

The latest conversation rules supersede conflicting examples in the supplied
specification. The current implementation contains:

- Material difficulty through HTTP, service, domain and PostgreSQL (default medium).
- Progress with per-material learning start/target, separate rehab counters and
  separate Stage, rehab and extra-review dates.
- Pure English basic/adaptive v1 transitions, manual actions and version registry.
- Long-term day-marker tables and nearest-template selection (not yet its answer algorithm).
- Folder training defaults, resolved plan/card snapshots and active-source conflict checks.
- Plans, persistent sessions/pools/presentations, atomic events and command receipts.
- Authenticated English Training HTTP endpoints, including recovery skip while waiting.

See [TRAINING_API.md](TRAINING_API.md) for endpoint contracts and a frontend flow.

## Recovery rules

An algorithm first handles the Stage result/rollback and establishes the main
schedule. It can then start recovery without changing that schedule:

1. Reinforce in the pool now until that day's mastery is reached.
2. Skip the following day. Reinforce again two days after first-day mastery.
3. If the main scheduled interval is strictly greater than 30 days, schedule a
   separate extra check ten days after second-day mastery.

Recovery has its own consecutive-correct counter; it never advances Stage
mastery. Historical correct/wrong totals include recovery answers. A wrong rehab
answer resets only that day's mastery. A wrong extra check restarts reinforcement.
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

## Persistence and verification

Migrations 000007, 000008 and 000009 must be applied before running the updated
API. They have only been exercised against an isolated test cluster, not the
application database. Deletion of referenced materials/plans is restricted to
protect progress; a user-facing soft-delete/history policy is still pending.

The Progress repository increments Version using a conditional update. The
answer service saves Progress, TrainingEvent, command receipt and pool changes
in one transaction. PostgreSQL row locks serialize training commands per user;
this covers plan overlap, absent progress creation, session creation/cancellation
and concurrent answers. This deliberately simple MVP approach can serialize
independent sessions belonging to one user, while different users remain independent.

The runtime candidate query is scoped to the plan's owned source folders and
filters untrainable cards. Earlier due materials precede new ones; ties use a
stable creation/ID order. Answered materials move to the end of the pool. Empty
pools remain resumable and do not complete a continuous English plan. A plan
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
Stage priority and manual skip between rehab days.

## Remaining implementation stages

1. Interview CRAM/long-term transitions, per-material final review and plan completion.
2. Formula exercises, presentation modes and algorithm.
3. Algorithm switching with KEEP/RESET/SET and the library deletion/history policy.

The Training API currently enables only English basic/adaptive v1. Existing
progress with another algorithm causes a conflict instead of silently migrating
it. Public-folder copying, content deduplication and search are outside this stage.
