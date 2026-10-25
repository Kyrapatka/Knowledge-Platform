# Quick Import UX redesign

Status: implementation specification. Scope is frontend only. The existing parser, backend, migration and API contract remain unchanged.

## User flow

Input: open from the current folder, choose Text or JSON, paste or upload, continue to Review. Returning to Input shows the current source buffer; viewing the original never reparses or removes reviewed edits. If source changes, a new parse requires explicit confirmation before replacing edits. Restored drafts show source, input kind, edits and decisions.

Review: desktop shows a compact summary, All/Issues/Ready filter, paginated material list (roughly 35% width), one selected editor, contextual diagnostics, navigation between problems and a pinned import action. Mobile shows list or editor with one main scroll and a pinned action above the safe area. Basic word/translation or question/answer fields and examples appear first; Advanced contains metadata, difficulty, profile, merge/split, example reassignment, duplicate override and parser details.

Problems: one derived diagnostic collection merges parser issues, missing fields, local validation, server issues and duplicate information. Each item has a stable identity, severity (error/warning/info), source, card and field when known, range, explanation, action and resolution. Server issues match `client_id` first, then a path index against the exact submitted command. Skip duplicates are informational. Counters distinguish affected materials from individual diagnostics; resolved or excluded material does not count as an active problem. Filters and navigation consume the same collection.

Import: selecting the main action sends validation for an immutable snapshot. The preview is bound to a revision and command ID; a later edit invalidates it, and a stale response cannot replace current diagnostics. Confirmation states actual predicted created, skipped, intentionally excluded and unresolved fragments. A pending command is written before commit, freezes edits and Undo/Redo, and retry uses the same body/UUID. The result is shown only after backend confirmation; confirmed success clears the draft and refreshes library state.

Undo/Redo: keep five reversible snapshots of editable cards and issue decisions. Sequential typing in the same field groups into one action until blur or a short idle pause. A new edit clears redo. Revalidation runs on restored state. Source ranges and stable IDs remain intact. Automatic parser output and source are referenced once, not copied into every history item. Local history need not survive reload, but the current edited draft must.

Recovery: ordinary drafts expire after seven days and are scoped to user/folder. An unconfirmed pending command must never be evicted by time/size cleanup or a second tab; if storage is full or stale, show an error and do not send a new command. A confirmed command may be cleared. Older v1 drafts remain readable; their source is recovered from `draft.source` when the saved input is absent, while an intentionally emptied input is represented explicitly.

## Layout and accessibility

Desktop modal up to 1100 px. Review has one main scroll; the material list is paginated and should not add a competing scrollbar. Actions remain visible at the bottom. Mobile 320–430 px and tablet are checked in portrait and landscape; list/editor drilldown preserves filter, page and selection. Controls use >=44 px touch targets where practical. The action area uses `env(safe-area-inset-bottom)`, and keyboard focus moves to the selected problem/field without hiding it behind the virtual keyboard. Existing Modal keyboard behavior and dark-theme variables are reused.

## Acceptance and tests

FR-01..25 and AC-01..35 from the attached request are in scope. The 16 themes are a traceability grouping, not a separate numbered source list. RED tests cover unified diagnostic counts/grouping, stale preview, server field mapping, skip duplicates, source restore, five-step Undo/Redo with typing grouping, filters/navigation, pinned action, 320/360/390/430 px, pending recovery and old Folder Import. GREEN requires these and existing parser/API tests. Browser tests with mocked API do not prove PostgreSQL writes; report them separately.
