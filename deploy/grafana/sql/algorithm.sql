WITH a AS (
SELECT mode,algorithm_version,experiment_group,count() AS answers,countIf(result='correct')/count() AS correct_rate,countIf(result='wrong')/count() AS wrong_rate,avgOrNull(answer_time_ms)/1000 AS answer_seconds,countIf(review_credit=true AND rehab_active_before=false AND rehab_active_after=true)/nullIf(countIf(review_credit=true),0) AS rehab_rate
FROM knowledge_analytics.training_answers
WHERE $__timeFilter(occurred_at) AND (${mode:sqlstring}='__all' OR mode=${mode:sqlstring}) AND (${algorithm:sqlstring}='__all' OR algorithm_version=${algorithm:sqlstring})
GROUP BY mode,algorithm_version,experiment_group),
s AS (
SELECT mode,algorithm_version,experiment_group,count() AS sessions,countIf(last_status='training_completed')/count() AS completion_rate
FROM knowledge_analytics.sessions
WHERE mode!='mock' AND $__timeFilter(started_at) AND (${mode:sqlstring}='__all' OR mode=${mode:sqlstring}) AND (${algorithm:sqlstring}='__all' OR algorithm_version=${algorithm:sqlstring})
GROUP BY mode,algorithm_version,experiment_group),
g AS (
SELECT mode,algorithm_version,experiment_group,avgOrNull(attempts_to_learned) AS attempts_to_learned,avgOrNull(seconds_to_learned) AS time_to_learned
FROM knowledge_analytics.learned_materials
WHERE $__timeFilter(learned_at) AND (${mode:sqlstring}='__all' OR mode=${mode:sqlstring}) AND (${algorithm:sqlstring}='__all' OR algorithm_version=${algorithm:sqlstring})
GROUP BY mode,algorithm_version,experiment_group),
keys AS (
SELECT mode,algorithm_version,experiment_group
FROM s UNION DISTINCT
SELECT mode,algorithm_version,experiment_group
FROM a UNION DISTINCT
SELECT mode,algorithm_version,experiment_group
FROM g)
SELECT k.mode,
       k.algorithm_version,
       if(k.experiment_group='','(unassigned)',k.experiment_group) AS experiment_group,
       ifNull(s.sessions,0) AS sessions,
       s.completion_rate,
       ifNull(a.answers,0) AS answers,
       a.correct_rate,
       a.wrong_rate,
       a.answer_seconds,
       a.rehab_rate,
       g.attempts_to_learned,
       g.time_to_learned
FROM keys k
LEFT JOIN s USING(mode,algorithm_version,experiment_group)
LEFT JOIN a USING(mode,algorithm_version,experiment_group)
LEFT JOIN g USING(mode,algorithm_version,experiment_group)
