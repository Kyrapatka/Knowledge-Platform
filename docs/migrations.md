# PostgreSQL migration audit

There are 23 contiguous up/down pairs. The runner validates their numbering and pairs, serializes itself with a PostgreSQL advisory lock, and commits each DDL operation with its version record. A dirty version stops the runner. Do not mix migration tools concurrently: other tools may use different locks.

A down file is not a promise of data-safe rollback. Categories below concern retained user data, not merely whether SQL executes on an empty database. No migration was renumbered; no new database schema was introduced by this hardening pass.

| Migration | Classification | Rollback implications |
| --- | --- | --- |
| 01 users | Data-sensitive | Drops accounts |
| 02 auth sessions | Data-sensitive | Drops active/revoked session history |
| 03 folders | Data-sensitive | Drops library folders |
| 04 folder config | Data-sensitive | Removes configured fields |
| 05 materials | Data-sensitive | Drops learning content |
| 06 folder config version | Data-sensitive | Removes concurrency version history |
| 07 material difficulty | Data-sensitive | Loses user-selected difficulty |
| 08 training progress | Data-sensitive | Drops learning progress |
| 09 training runtime | Data-sensitive | Drops plans, sessions, events, receipts and configuration |
| 10 interview final | Conditional | Old CHECK constraints reject start_final history |
| 11 formula training | Data-sensitive | Drops exercises and event practice metadata |
| 12 deletion/change journal | Conditional, data-sensitive | Guard rejects soft-deleted content/change history; plan version column is removed |
| 13 combined training | Data-sensitive | Removes selection/combined session metadata |
| 14 library topics | Practically irreversible data migration | Drops index but intentionally retains topic definition/data |
| 15 early review | Conditional | Old CHECK constraints reject review_early history |
| 16 English direction | Data-sensitive | Drops recorded answer direction |
| 17 review lead | Practically irreversible data migration | Down deliberately preserves adjusted dates; later answers prevent reconstruction |
| 18 undo | Conditional, data-sensitive | Guard rejects undone answers; pending undo snapshots are still dropped |
| 19 interview graph | Conditional, data-sensitive | Guard rejects active graph history, but graph/profile/concept data would be dropped |
| 20 graph runtime | Data-sensitive | Indexes reversible; explicit selection order is lost |
| 21 question bank | Conditional, data-sensitive | Refuses bank imports/policies/memberships/profiles/new roles/nondefault metadata; restores legacy seed keys |
| 22 interview navigation | Conditional | Old action/event-mode constraints reject navigation history |
| 23 mock sessions | Conditional, data-sensitive | NOT NULL restoration rejects planless sessions/events/undo; graph-selection event references are dropped |

**Fully reversible with arbitrary production data:** none of these pairs can honestly be given that blanket guarantee. Individual index-only operations are reversible, but the migrations containing them also change data or constraints. Empty-schema down/up is a smoke check, not a production rollback certification.

Migration 21 previously deleted answer/wrong_fallback links to force the old constraint to fit. Its down now refuses incompatible data before dropping anything, including bank profiles, seed-managed aliases and topic/subtopic metadata. This avoids silently deleting user information. Older destructive downs are documented rather than rewritten speculatively; restore a backup or design an explicit forward fix when preserving their data is required.

## Constraints and query indexes

Existing ownership FKs, session/plan/material FKs, unique command receipts, progress identity constraints, positive/monotonic version checks and algorithm/action CHECK constraints remain intact. Migration 21 backfills owner_id before NOT NULL, retains legacy duplicate seed keys, then creates the owner/seed partial unique index. Migration 23 permits planless mock history only under mock-specific CHECK constraints. No ORM schema generation is used for the production-schema auth repository checks.

The inspected queries already have relevant indexes: active materials by folder, active folders by owner, plan/material progress identity and due work, active sessions, command receipt uniqueness, undo by user/id descending, graph selection by session/order with undone filtering, ready profiles by folder, bank filter by owner/status/domain/topic, memberships by profile/material, and auth session token identity/ownership. No new index was added without an actual uncovered query.

`interview_aliases_concept` overlaps the leading concept_id of the existing alias uniqueness index. It was not removed without query-plan/production-size evidence. Further performance tuning should use representative EXPLAIN ANALYZE results rather than guessed composite indexes.

## Safe smoke and strict tests

Set `TRAINING_TEST_DATABASE_URL` to a dedicated test database and run `make migrate-test`. It creates a UUID-named schema, applies all 23 up migrations, tests 23 down/up only on that empty schema, then removes only that schema. Cleanup failure fails the command. It does not reset the application database. The allowlist requires review when a new migration is added.

PostgreSQL tests also verify that rolling back migration 23 with planless history fails atomically and retains version 23, and that migration 21 refuses bank metadata without erasing it. `make test-integration` requires both test database variables and rejects any skipped test in its selected suites.

## Local application rollback

`make rollback-local` restores only the recorded previous application image, with
API-only startup. It never runs the older migrator or a down migration. Automatic
rollback currently requires the same clean migration version recorded with that
image; any schema version change is refused pending manual compatibility review,
even if additive. See [local deployment](local-deployment.md). Prefer expand/contract
changes and keep old application readers/writers compatible during transitions.
