-- Views deliberately contain no Grafana macros; they can be checked with SQL clients.
-- Latest lifecycle occurrence handles undo/recompletion sharing a stable event_id.
CREATE OR REPLACE VIEW knowledge_analytics.events_unique AS
SELECT * FROM knowledge_analytics.analytics_events
ORDER BY occurred_at DESC, event_name LIMIT 1 BY event_id;

CREATE OR REPLACE VIEW knowledge_analytics.events_effective AS
SELECT * FROM knowledge_analytics.events_unique
WHERE toString(event_id) NOT IN
 (SELECT related_event_id FROM knowledge_analytics.events_unique WHERE event_name='training_rollback' AND result='undo')
AND (related_event_id='' OR related_event_id NOT IN
 (SELECT related_event_id FROM knowledge_analytics.events_unique WHERE event_name='training_rollback' AND result='undo'));

CREATE OR REPLACE VIEW knowledge_analytics.training_answers AS
SELECT * FROM knowledge_analytics.events_effective
WHERE event_name='training_answered' AND mode!='mock' AND result IN ('correct','wrong');

CREATE OR REPLACE VIEW knowledge_analytics.mock_answers AS
SELECT * FROM knowledge_analytics.events_effective
WHERE event_name='interview_question_answered' AND mode='mock' AND result IN ('correct','wrong');

CREATE OR REPLACE VIEW knowledge_analytics.sessions AS
SELECT user_id,session_id,
 argMaxIf(e.mode,occurred_at,e.mode!='') AS mode,
 argMaxIf(e.algorithm_version,occurred_at,e.algorithm_version!='') AS algorithm_version,
 argMaxIf(e.experiment_group,occurred_at,e.experiment_group!='') AS experiment_group,
 argMaxIf(e.interview_mode,occurred_at,e.interview_mode!='') AS interview_mode,
 minIf(occurred_at,event_name='training_started') AS started_at,
 maxIf(occurred_at,event_name IN ('training_completed','training_abandoned')) AS ended_at,
 argMaxIf(event_name,occurred_at,event_name IN ('training_started','training_completed','training_abandoned') OR (event_name='training_rollback' AND result='undo')) AS last_status,
 countIf(event_name IN ('training_answered','interview_question_answered') AND result IN ('correct','wrong')) AS answers
FROM knowledge_analytics.events_effective e
WHERE session_id!='' AND (event_name!='interview_question_answered' OR e.mode='mock')
GROUP BY user_id,session_id
HAVING countIf(event_name='training_started')>0;

CREATE OR REPLACE VIEW knowledge_analytics.learned_materials AS
WITH first_learned AS (
 SELECT user_id,material_id,plan_id,mode,algorithm_version,experiment_group,
 min(occurred_at) AS learned_at
 FROM knowledge_analytics.training_answers
 WHERE review_credit=true AND learned_before=false AND learned_after=true
 -- Historical rows without a plan cannot safely distinguish separate schedules.
 AND plan_id!=''
 GROUP BY user_id,material_id,plan_id,mode,algorithm_version,experiment_group
)
SELECT l.user_id AS user_id,l.material_id AS material_id,l.plan_id AS plan_id,l.mode AS mode,l.algorithm_version AS algorithm_version,l.experiment_group AS experiment_group,l.learned_at AS learned_at,
 countIf(a.occurred_at<=l.learned_at) AS attempts_to_learned,
 dateDiff('second',min(a.occurred_at),l.learned_at) AS seconds_to_learned
FROM first_learned l INNER JOIN knowledge_analytics.training_answers a
 USING(user_id,material_id,plan_id,mode,algorithm_version,experiment_group)
WHERE a.review_credit=true
GROUP BY l.user_id,l.material_id,l.plan_id,l.mode,l.algorithm_version,l.experiment_group,l.learned_at;

-- The first actual review in [N,N+1) days after learning. Missing review != wrong.
-- Only fully mature windows are included; success is conditional on observation.
CREATE OR REPLACE VIEW knowledge_analytics.learning_retention AS
SELECT l.user_id AS user_id,l.material_id AS material_id,l.plan_id AS plan_id,l.mode AS mode,l.algorithm_version AS algorithm_version,l.experiment_group AS experiment_group,l.learned_at AS learned_at,
 n.horizon,
 countIf(a.occurred_at>=addDays(l.learned_at,n.horizon) AND a.occurred_at<addDays(l.learned_at,n.horizon+1))>0 AS observed,
 argMinIf(a.result,a.occurred_at,a.occurred_at>=addDays(l.learned_at,n.horizon) AND a.occurred_at<addDays(l.learned_at,n.horizon+1)) AS first_result
FROM knowledge_analytics.learned_materials l
CROSS JOIN (SELECT arrayJoin([7,30]) AS horizon) n
LEFT JOIN (SELECT * FROM knowledge_analytics.training_answers WHERE review_credit=true) a
 USING(user_id,material_id,plan_id,mode,algorithm_version,experiment_group)
WHERE addDays(l.learned_at,n.horizon+1)<=now()
GROUP BY l.user_id,l.material_id,l.plan_id,l.mode,l.algorithm_version,l.experiment_group,l.learned_at,n.horizon SETTINGS join_use_nulls=1;

CREATE OR REPLACE VIEW knowledge_analytics.rehab_episodes AS
WITH numbered AS (
 SELECT *,sum(toUInt64(ifNull(rehab_active_before=false AND rehab_active_after=true,false))) OVER
  (PARTITION BY user_id,material_id,plan_id ORDER BY occurred_at,event_id ROWS BETWEEN UNBOUNDED PRECEDING AND CURRENT ROW) AS episode
 FROM knowledge_analytics.training_answers WHERE plan_id!='' AND review_credit=true
)
SELECT user_id,material_id,plan_id,episode,
 argMin(mode,occurred_at) AS mode,argMin(algorithm_version,occurred_at) AS algorithm_version,
 argMin(experiment_group,occurred_at) AS experiment_group,
 min(occurred_at) AS entered_at,
 argMin(stage_before,occurred_at) AS entry_stage,
 countIf(review_kind='rehab') AS attempts,
 countIf(rehab_active_before=true AND rehab_active_after=false AND result='correct' AND review_kind='rehab')>0 AS recovered
FROM numbered WHERE episode>0 AND (rehab_active_before=true OR rehab_active_after=true)
GROUP BY user_id,material_id,plan_id,episode;

CREATE OR REPLACE VIEW knowledge_analytics.user_activity AS
SELECT DISTINCT user_id,toDate(occurred_at,'UTC') AS day
FROM knowledge_analytics.events_unique
WHERE user_id!='' AND event_name NOT IN ('login_failed','user_registered');

CREATE OR REPLACE VIEW knowledge_analytics.user_cohorts AS
SELECT user_id,min(occurred_at) AS registered_at,toDate(registered_at,'UTC') AS joined
FROM knowledge_analytics.events_unique WHERE event_name='user_registered' AND user_id!=''
GROUP BY user_id;
