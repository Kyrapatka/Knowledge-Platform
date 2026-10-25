# Quick Import: existing-folder material import

Status: implementation in progress. This operation never creates or renames folders. Existing Folder Import and its 5000-item contract remain unchanged.

## Architecture and contract

Pure TypeScript parser/model under `web/src/features/quick-import` adapts the supplied Universal Parser v0.4 and retains its regressions. Text (1 MiB) and strict UTF-8 JSON (10 MiB) produce one ImportDraft, maximum 1000 cards. English uses current active folder fields, `foreign`, `native`, and newline-joined `example`; pronunciation/transcription are never imported. Interview text uses explicit question/answer labels and current schema; missing answers are not invented.

Draft stores original source, UTF-16 ranges suitable for textarea selection (including CRLF), automatic snapshot, edited cards, ordered examples, unresolved fragments, explanations, boundary/example alternatives, explicit exclusions and edit history. Content-derived IDs with occurrence suffixes remain stable across revalidation. Scores are relative evidence, never probabilities. Every nonblank source line is represented by a field, example or diagnostic. Ambiguity requires a decision. Slash-separated values remain whole.

JSON supports arrays and format_version=1 Folder Import envelopes with matching template, without applying folder name. Aliases: word→foreign, translation→native, full_answer→answer, source→sources, notes→note. Duplicate JSON keys are errors with paths; unknown fields and pronunciation remain review diagnostics until explicitly excluded or reassigned. Metadata and profile options use the actual schemas and bounded typed fields.

Review: paginated list (25), All/Issues/Ready, counts, field/example editing, fragment exclusion/assignment, example reassignment/copy, merge/split and exact source navigation. Revalidation operates on edits. Reparse requires confirmation if edits would be replaced. Mobile uses list/editor drilldown. Close/Escape warns about unsaved work. Bounded local storage is scoped to authenticated user and folder, expires after 7 days, offers restore, and is removed only on confirmed successful commit (or explicit discard).

## Server

`POST /api/v1/folders/:folderID/materials/import/validate` is read-only; `POST .../import` commits. Both use auth and ownership and enforce current active folder schema, type/field/count/size limits. JSON request: command_id UUID, duplicate_policy (skip default, conflict, allow), items [{client_id, values, metadata, difficulty, optional profile, optional duplicate_policy}]. Difficulty defaults medium. Validation result includes per-item issues, batch/existing duplicate IDs, created/skipped predictions and valid flag. Commit returns confirmed created/skipped/failed counts and material IDs.

Duplicate key is lowercase trimmed whitespace-collapsed foreign/question; no stemming or splitting. Compare all active materials in the folder and the accepted batch. Skip is explicit in preview/result; conflict aborts; allow requires an explicit user decision. Conflicting translations are not merged automatically.

One transaction locks the user row, checks the receipt, reads current folder and duplicate state, validates every item, creates materials through CreateMany, persists interview profiles, then writes receipt. A validation failure rolls back the entire submitted set. Existing write paths share the user-row lock. Migration 26 adds only material_import_commands (user_id, command_id unique, request hash, response, timestamp), separate from bulk commands to prevent cross-operation replay. Hash includes operation, folder and request. Same command/payload replays its result; changed payload conflicts. Receipt has no expiry while materials can still exist. Down refuses loss of receipts. Analytics publish once after successful commit; replay emits nothing.

Interview profile creation joins the same transaction. Explicit profile inputs are validated by the interview domain. Answered questions receive a ready profile with deterministic topic concepts; unanswered questions remain draft with an explicit warning, preserving their question without invented answers. Failure to save a profile aborts import.

## Client commit safety

Before commit, server validation runs on the selected set and confirmation lists create/skip/exclude counts. Import all requires all included cards ready and all unassigned fragments resolved. Import valid only explicitly confirms omitted invalid cards/fragments. Persist an immutable command snapshot before sending; double-submit guard and frozen form survive uncertain network errors and draft restoration. Retry uses exactly the same UUID/body. Only a confirmed server result clears the draft and refreshes library/material counts.

## Limits

1000 items, 1 MiB text, 10 MiB JSON/request, 64 fields per values/metadata map, 100000 UTF-8 bytes per field, 8 MiB total values, 64 concepts, JSON nesting at most 32. Unsupported template, stale/inactive schema, null values, unknown fields, invalid difficulty/profile are rejected. IndexedDB draft storage is bounded to 40 MiB per draft, 80 MiB total across at most 5 recent drafts; failure is visible and does not pretend to save.

## Test/acceptance plan

RED records behavioral assertions failing against stubs, then GREEN records parser/model/backend assertions. Retain all 53 prototype regressions; independently compare all 14 real-note cards/12 examples and the exact сатл fragment. New diverse 500-record corpus compares values/examples/ranges with a separate oracle; damaged corpus verifies missing translations, unknown fragments and duplicates. Report time and heap deltas, not accuracy percentages.

AC01 button/old import isolation; AC02 mixed English formats; AC03 global examples/token boundaries; AC04 range-preserving diagnostics and boundary alternatives; AC05 interview schema/profile; AC06 strict JSON/unified draft; AC07 edit/revalidate/merge/split/source; AC08 responsive review/500 rows; AC09 draft recovery and pending command; AC10 duplicate policies/all-folder/race; AC11 atomicity/ownership/idempotency; AC12 explicit valid-only/results/refresh; AC13 no pronunciation/external NLP; AC14 regression/security limits; AC15 stress independent oracle; AC16 documentation and honest PASS/FAIL/BLOCKED evidence.

PostgreSQL integration is gated by TRAINING_TEST_DATABASE_URL. Docker Desktop is not started per the user's latest preference. Browser tests with mocked API are reported separately from real PostgreSQL end-to-end verification.
