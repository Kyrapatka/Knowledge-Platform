-- Examples, not automatic migrations. UTC calendar days; best-effort data.
-- Deduplicate lifecycle events (undo/recompletion can share event_id).
CREATE VIEW IF NOT EXISTS knowledge_analytics.events_unique AS
SELECT * FROM knowledge_analytics.analytics_events ORDER BY occurred_at LIMIT 1 BY event_id;

-- Rolling 24h / 7d / 30d active users with recorded product activity; not page views.
SELECT uniqExactIf(user_id, occurred_at >= now() - INTERVAL 1 DAY) AS dau,
       uniqExactIf(user_id, occurred_at >= now() - INTERVAL 7 DAY) AS wau,
       uniqExactIf(user_id, occurred_at >= now() - INTERVAL 30 DAY) AS mau
FROM knowledge_analytics.events_unique WHERE user_id != '';

-- Registrations per day.
SELECT toDate(occurred_at) AS day, uniqExact(user_id) AS registrations
FROM knowledge_analytics.events_unique WHERE event_name = 'user_registered'
GROUP BY day ORDER BY day;

-- Exact calendar D1/D7/D30 retention, denominator restricted to mature cohorts.
WITH cohorts AS (
 SELECT user_id, toDate(min(occurred_at)) AS joined
 FROM knowledge_analytics.events_unique WHERE event_name='user_registered' GROUP BY user_id
), activity AS (
 SELECT DISTINCT user_id, toDate(occurred_at) AS day
 FROM knowledge_analytics.events_unique WHERE user_id!='' AND event_name!='user_registered'
)
SELECT offset_days,
 uniqExactIf(c.user_id, a.user_id!='') / nullIf(uniqExact(c.user_id),0) AS retention
FROM cohorts c CROSS JOIN (SELECT arrayJoin([1,7,30]) AS offset_days) offsets
LEFT JOIN activity a ON a.user_id=c.user_id AND a.day=addDays(c.joined,offset_days)
WHERE addDays(c.joined,offset_days)<toDate(now('UTC'))
GROUP BY offset_days;

-- Per-session outcomes; unique session ids avoid double counting specialized events.
SELECT mode, uniqExactIf(session_id,event_name='training_started') AS started,
 uniqExactIf(session_id,event_name='training_completed') AS completed,
 completed / nullIf(started,0) AS completion_rate,
 countIf(event_name='training_answered') / nullIf(started,0) AS answers_per_session
FROM knowledge_analytics.events_unique
WHERE event_name IN ('training_started','training_completed','training_answered')
GROUP BY mode;

-- Learning dimensions. Exclude explicitly undone answers and distinguish review credit.
SELECT mode, algorithm_version, experiment_group, topic, difficulty, stage_before,
 count() AS answers, countIf(result='correct')/count() AS correct_rate,
 countIf(result='wrong')/count() AS wrong_rate, avg(answer_time_ms) AS mean_answer_ms,
 countIf(rehab_active_before=false AND rehab_active_after=true)/count() AS rehab_entry_rate,
 countIf(rehab_active_before=true AND rehab_active_after=false AND result='correct' AND review_kind='rehab') AS rehab_recoveries
FROM knowledge_analytics.events_unique
WHERE event_name='training_answered' AND result IN ('correct','wrong')
 AND toString(event_id) NOT IN (
  SELECT related_event_id FROM knowledge_analytics.events_unique
  WHERE event_name='training_rollback' AND result='undo'
 )
GROUP BY mode, algorithm_version, experiment_group, topic, difficulty, stage_before;

-- Observed exposures/time until first learned transition; historical learning
-- before instrumentation and lost events are not reconstructed.
WITH answers AS (
 SELECT * FROM knowledge_analytics.events_unique
 WHERE event_name='training_answered' AND review_credit=true
 AND toString(event_id) NOT IN (SELECT related_event_id FROM knowledge_analytics.events_unique
  WHERE event_name='training_rollback' AND result='undo')
), learned AS (
 SELECT user_id,material_id,mode,min(occurred_at) AS learned_at FROM answers
 WHERE learned_before=false AND learned_after=true GROUP BY user_id,material_id,mode
)
SELECT a.user_id,a.material_id,a.mode,countIf(a.occurred_at<=l.learned_at) AS exposures,
 dateDiff('second',min(a.occurred_at),l.learned_at) AS observed_seconds_until_learned
FROM answers a INNER JOIN learned l USING(user_id,material_id,mode)
GROUP BY a.user_id,a.material_id,a.mode,l.learned_at;

-- Content usage/difficulty: rank by sample size as well as error rate.
SELECT template,topic,material_id,folder_id,count() AS samples,
 countIf(result='wrong')/count() AS error_rate
FROM knowledge_analytics.events_unique WHERE event_name='training_answered'
 AND result IN ('correct','wrong')
GROUP BY template,topic,material_id,folder_id HAVING samples>=20 ORDER BY error_rate DESC;

-- First-use funnel. Folder imports qualify as the first folder.
WITH per_user AS (
 SELECT user_id,
 nullIf(minIf(occurred_at,event_name='user_registered'),toDateTime64(0,3,'UTC')) AS registered,
 nullIf(minIf(occurred_at,event_name IN ('folder_created','folder_imported')),toDateTime64(0,3,'UTC')) AS folder,
 nullIf(minIf(occurred_at,event_name='material_created'),toDateTime64(0,3,'UTC')) AS material,
 nullIf(minIf(occurred_at,event_name='training_started'),toDateTime64(0,3,'UTC')) AS training,
 nullIf(minIf(occurred_at,event_name='training_completed'),toDateTime64(0,3,'UTC')) AS completed
 FROM knowledge_analytics.events_unique WHERE user_id!='' GROUP BY user_id
)
SELECT countIf(registered IS NOT NULL) AS registrations,
 countIf(folder>=registered) AS first_folder,
 countIf(folder>=registered AND material>=folder) AS first_material,
 countIf(folder>=registered AND material>=folder AND training>=material) AS first_training,
 countIf(folder>=registered AND material>=folder AND training>=material AND completed>=training) AS first_completion
FROM per_user;
