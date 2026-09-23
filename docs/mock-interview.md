# Mock Interview

Mock Interview extends the existing interview graph, session, presentation,
action receipt, event, and Undo infrastructure. It does not create a TrainingPlan.
Long-term, Cram and the existing plan-backed graph API keep their semantics.

## API and storage

- `POST /api/v1/training/mock-interviews`: start or resume an identical active
  mock. Body: `command_id`, `sources: [{folder_id, topics?}]`, `config`.
- `POST /api/v1/training/mock-interviews/preview`: same sources/config, no command
  ID required. Returns eligible topic shares, area capacities and root slots.
- `GET /api/v1/training/mock-interviews/active`: resume on another browser;
  404 means no active mock.
- Existing session GET, `/actions`, `/undo` and `/finish` endpoints are reused.
  `next_route` remains the wire action for the UI's **Next Root** button.

Config adds `interview_mode: real | balanced | custom | deep`,
`depth_level: 1 | 2 | 3`, and `custom_weights: {go,sql,http,architecture,messaging,ops,other}`.
Defaults are Real, depth 1, 24 answers. Profile, interview level and ready/draft
filters still apply. Deep difficulty is a scoring preference, not an extra
eligibility filter. Invalid modes, depths, weights or source lists return 400;
unowned sources return 404. Unknown custom keys and negative/non-finite values
are rejected. At least one **available** topic needs a positive custom weight.

An active mock with different settings returns 409 `active_mock_interview` with
a resume/end explanation. It never returns an SRS-plan overlap conflict.
Commands are idempotent under the existing per-user transaction lock.

Migration 23 permits NULL `plan_id` in the existing session, event and undo
tables, constrains planless sessions to non-combined interview graphs, and
enforces one active planless mock per user. A planless event cannot receive
review credit or change a progress version. Internally `uuid.Nil` represents
SQL NULL (the session JSON currently uses the zero UUID). A lightweight
in-memory adapter supplies existing graph/presentation interfaces; it is never
inserted into `training_plans`. No new session/history tables are needed.

The existing `interview_graph_session_state.state` JSON holds configuration,
sources, seeded random position, immutable initial `interview_plan`, shown,
skipped and completed root IDs, recent root concepts/areas and answer counts.
Undo snapshots restore all these together. Migration 23 also links each graph
answer event to its exact selection event: repeated presentations of a tiny-bank
question must not multiply statistics. Legacy events retain a fallback lookup.
Rollback refuses to erase planless history; archive/export it deliberately
before attempting a down migration.

## Root and branch lifecycle

A root starts a connected branch. Answer concepts/hooks guide a correct answer;
wrong-fallback/prerequisite concepts guide remediation. Primary/tested concepts
are a fallback. The existing concept-edge relevance and score helpers are reused.
Follow-ups must connect semantically (or share the derived area), have not been
shown, and remain inside the selected sources. Wrong-answer remediation does
not select a harder/more-specific question merely because Deep is enabled.

`questions_asked` counts presentations, not answers. `answered_questions` counts
Correct/Wrong only. `shown_root_ids`, `skipped_root_ids`, `completed_root_ids`
distinguish the three lifecycle events (lists can contain repeated IDs after
exhaustion). Completed branches alone advance the planned root slot.

Next Root records replacement and chooses a new root for **the same slot**. It
does not increase answers/completed branches or run the normal answer-limit
completion check. Even 100 skips before answering leave slot 1 active. If the
user skips after some answers, those answers remain recorded but the abandoned
branch is not marked complete. A branch completes after actual answers when its
budget/depth is reached or coherent follow-ups are exhausted. The session ends
at its answer budget, completed root target, explicit End, or loss of all eligible
roots through external edits/deletion. Small banks can produce shorter branches.

Root selection retains the existing positive `root_weight` rule. It prefers
unseen IDs in the planned topic, then unseen IDs in other positively weighted
selected topics. Only after exhaustion does it recycle IDs, avoiding the current
root if an alternative exists. Reusing a previously seen area gets a strong
penalty; overlap with recent root concepts gets an additional penalty. These
are session-local and relax naturally in small banks, not permanent concept bans.
Areas derive from primary concept, then subtopic/category, then topic, then ID;
no new root-area entity is introduced. History and the fixed seeded random
position make resumed/undone selections reproducible.

## Distribution and depth

`graph/plan.go` centralizes classification, presets, normalization and allocation.
Canonical domain takes priority, followed by topic/subtopic, concepts, keywords
and folder name. Token boundaries avoid substring mistakes; unknown metadata
goes to Other (baseline weight 5). Ordinary imported interview materials without
a graph profile use metadata and conservative defaults. There is no import-only
branch and the seed bank is not rewritten.

The initial **product preset**, not a universal empirical interview standard:

| Group | Relative weight |
| --- | ---: |
| Go/runtime/concurrency | 45 |
| SQL/databases | 20 |
| HTTP/API/networks | 12 |
| Architecture/system design | 10 |
| Messaging/distributed systems | 8 |
| Testing/operations | 5 |

Only eligible, selected groups participate. Normalize each weight by their sum:
Go + HTTP gives 45/57 = 78.95% and 12/57 = 21.05%; Go + SQL + HTTP gives
58.44%, 25.97%, 15.58%. Balanced uses equal weights; Custom uses nonnegative
relative numbers, so 5/3/2 and 10/6/4 both give 50/30/20. Zero excludes a group.

Weights allocate **root branches**, not individual questions. Capped largest
remainders allocate every root slot, randomly break equal remainders using the
session seed, and redistribute shortages to topics with spare distinct-area
capacity. A smooth weighted-deficit schedule mixes topics. Preview uses a fixed
seed for stable display; actual starts can assign tied remainders differently.
Skipped/exhausted topics may dynamically fall back to other selected topics;
the initial plan stays available for comparison and Undo.

Standard root target is `min(questionLimit, ceil(questionLimit / 6) + 2)`, capped
by available distinct root areas: 12→4, 18→5, 24→6, 30→7, 40→9. The remaining
answer budget is divided across remaining slots, normally giving 3–5 answers
per standard branch. The old `max_roots` config remains for legacy graph clients;
new mock targets are derived from session size/mode and shown in the preview.

| 24-answer mode | Root target | Target branch | Difficulty bonus | Rarity bonus | Follow probability | Concept hops |
| --- | ---: | ---: | ---: | ---: | ---: | ---: |
| Standard | 6 | ~4 | 0 | 0 | .80 | 1 |
| Deep 1 | 4 | ~6 | 1.2 | .7 | .93 | 1 |
| Deep 2 | 3 | ~8 | 2.4 | 1.4 | .96 | 2 |
| Deep 3 | 2 | ~12 | 3.6 | 2.1 | .99 | 2 |

Deep uses Real topic shares but different traversal parameters. Difficulty and
rarity bonuses scale normalized profile values; two-hop expansion is discounted
and must stay connected. Root-switch penalties .4/.8/1.2 further reduce optional
early switching. Budgets, explicit maximum depth, no-repeat history and available
connections still bound every branch. Lack of hard questions never empties the
pool just because Deep is selected.

## Practice-only boundary and performance

Planless candidate queries never join `user_material_progress`. Presentation,
answers and Undo skip all progress reads/creates/updates, and every selection has
`review_credit=false`. Existing stage/due/stability/rehab/counters and plans are
unchanged. Normal TrainingPlan progress is not merged, duplicated or cancelled.
Old plan-backed graph endpoints are kept for API compatibility; the new Mock UI
does not resume those legacy SRS-capable sessions.

Candidate material/profile metadata is loaded in one bounded query (up to 5000),
then concepts and profile memberships in two bulk queries. There is no per-question
query. Sources are ownership-checked at startup; queries continue to enforce owner,
template and deletion filters. No cache or parallel question-bank store is added.

## Checks

Unit tests cover normalization, scale invariance, capped/fair allocation,
root scaling, 100 replacements, area/ID novelty, single-root fallback, Deep
parameters/difficulty, follow-up history and completion. PostgreSQL/HTTP tests
cover existing overlapping plans, all modes, exact whole-row SRS/plan invariance,
Undo, resume, receipts, source ownership, invalid sources/config and active conflicts.
Playwright tests exercise the real API, seeded folders, independent SRS plans,
all four modes/three depths, selected-only controls, errors, 20 replacements,
dismissal/resume, 320/360/390/430/768/1440 layouts and mobile folder actions.

Run `go fmt ./...`, `go test ./...`, `go test -race ./...` with the configured
isolated-schema test DSNs; frontend `npm.cmd --prefix web run build` and
`npm.cmd --prefix web run test:e2e` (the `test` script also invokes Playwright).
There is no lint script. Browser tests require the migrated API on port 8080.
