# First frontend MVP contracts

All learning/library routes are under `/api/v1` and require a Bearer access token. Frontend text is English.

## Browser authentication

`POST /auth/browser/register` and `/auth/browser/login` accept `{nickname,password}` and return `{user,access_token,expires_at}`. An HttpOnly `kp_refresh` cookie is scoped to `/api/v1/auth/browser`. `POST /auth/browser/refresh` rotates it; `POST /auth/browser/logout` revokes it and removes the cookie. Browser POSTs require a matching Origin. Existing JSON-token auth endpoints remain available to other clients.

## Library

`GET /library` returns `{folders,totals}`. Each folder retains its ordinary configuration and adds `material_count`, `due_count`, `learning_count`, `completed_count`, `topics:[{name,count}]`, and `selected_plan`.

`GET /library/folders/:folderID/materials` accepts `q`, `topic`, `plan_id`, `sort`, `direction`, `limit`, `offset`. Sort keys: `created_at`, `question`, `topic`, `stage`, `next_review_at`. The response is `{items,total,limit,offset,topics,selected_plan,plans}`. Items contain material fields plus `topic` and nullable `progress`. No progress is created by reading a library page. A selected CRAM plan reads its own progress; persistent plans read their own track.

Topics use `metadata.topic`; legacy `metadata.category` is a fallback. One material has one topic. Empty topic selection means all topics; `__none__` selects materials with no topic. The material editor shows only active metadata fields configured by the folder.

## Combined training

`POST /training/combined`:

```json
{
  "sources": [
    {"folder_id":"<folder-uuid>", "topics":["SQL"]},
    {"folder_id":"<other-folder-uuid>", "plan_id":"<plan-uuid>"}
  ]
}
```

Sources may include `algorithm_key`, `horizon_days` and `pool_size` to configure a new plan. Existing plan settings must be changed explicitly; a launch never silently resets progress. Sources are resolved transactionally and filtered on the server. An existing compatible plan and session are reused.

Response:

```text
{
  sessions: [{session, plan, summary, pool_size}],
  current: {session_id, plan_id, algorithm_key, presentation} | null,
  summary,
  next_review_at,
  empty_reason
}
```

`POST /training/combined/current` accepts `{session_ids:[...]}` and refreshes all child sessions. Finished runs can resume when new work becomes available. Always retain returned session IDs. `current` contains exactly the presentation to show. Answers go to the existing `/training/sessions/:sessionID/actions` endpoint, followed by a combined refresh. Reuse the exact command ID and request body after an uncertain network result.

The UI does not expose technical child sessions or a finish button. When no work is currently available it shows “All done for now”; the material schedule remains active. Empty reasons include `no_matching_materials`, `not_due` and `completed`.

## Plan settings

The existing `/training/plans/:id/algorithm` endpoint additionally accepts `pool_size` and `end_active_session:true`. The latter lets the settings operation close the current internal run atomically. No manual session finish is needed. Pool-only KEEP updates preserve stage, recovery and partial mastery. Moving between CRAM and long-term uses a separate plan because those tracks hold independent progress.

## Statistics

`GET /statistics?days=30&timezone=Europe/Moscow` returns `{days,timezone,from,to,totals,daily}`. Answer counts include only `correct`/`wrong`, not administrative actions. Material counts are distinct. Sessions count sessions containing answers; empty technical runs are excluded. No elapsed active study time is inferred from session timestamps. History remains in statistics after content deletion.
# Training refinement additions

`POST /training/combined/current` accepts `review_early: true` with a stable `command_id` and the existing `session_ids`. The server validates ownership, current sessions and the strictly less than three-hour window. Replaying the command returns its receipt. The selected event is brought forward; other Stage/recovery dates are preserved.

English presentations optionally include `direction` (`foreign` or `native`), `example` and `foreign_word`. The first direction is random and subsequent answered presentations alternate per material, including after Wrong. Reloading an unanswered presentation preserves it. The direction is included in answer history.
