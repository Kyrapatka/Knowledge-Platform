SELECT mode,count() AS started,countIf(last_status='training_completed') AS completed,
countIf(last_status='training_abandoned') AS abandoned,completed/nullIf(started,0) AS completion_rate,
avgOrNull(answers) AS answers_per_session,
avgOrNullIf(dateDiff('second',started_at,ended_at),last_status IN ('training_completed','training_abandoned')) AS closed_session_seconds
FROM knowledge_analytics.sessions WHERE mode!='mock' AND $__timeFilter(started_at) AND (${mode:sqlstring}='__all' OR mode=${mode:sqlstring}) AND (${algorithm:sqlstring}='__all' OR algorithm_version=${algorithm:sqlstring}) AND (${experiment:sqlstring}='__all' OR experiment_group=${experiment:sqlstring}) GROUP BY mode
