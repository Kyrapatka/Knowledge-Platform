-- SQL-client examples. Apply 001/002/003 first; all days are UTC.
-- Canonical Grafana panel SQL (with time/filter macros): ../grafana/sql/*.sql.
-- events_unique chooses the latest occurrence; events_effective removes undo.

SELECT uniqExactIf(user_id,occurred_at>=now()-INTERVAL 1 DAY) AS dau,
       uniqExactIf(user_id,occurred_at>=now()-INTERVAL 7 DAY) AS wau,
       uniqExactIf(user_id,occurred_at>=now()-INTERVAL 30 DAY) AS mau
FROM knowledge_analytics.events_unique WHERE user_id!='' AND event_name!='login_failed';

SELECT mode,count() AS started,countIf(last_status='training_completed') AS completed,
       countIf(last_status='training_abandoned') AS abandoned,
       completed/nullIf(started,0) AS completion_rate,avg(answers) AS answers_per_session
FROM knowledge_analytics.sessions GROUP BY mode;

SELECT mode,algorithm_version,experiment_group,horizon,count() AS eligible,
       countIf(observed) AS reviewed,countIf(observed AND first_result='correct') AS correct,
       correct/nullIf(reviewed,0) AS learning_retention,reviewed/nullIf(eligible,0) AS coverage
FROM knowledge_analytics.learning_retention
GROUP BY mode,algorithm_version,experiment_group,horizon;

SELECT n.days,uniqExact(c.user_id) AS eligible,uniqExactIf(c.user_id,a.day IS NOT NULL) AS returned,
       returned/nullIf(eligible,0) AS product_retention
FROM knowledge_analytics.user_cohorts c
CROSS JOIN (SELECT arrayJoin([1,7,30]) AS days) n
LEFT JOIN knowledge_analytics.user_activity a ON a.user_id=c.user_id AND a.day=addDays(c.joined,n.days)
WHERE addDays(c.joined,n.days)<toDate(now('UTC')) GROUP BY n.days
SETTINGS join_use_nulls=1;
