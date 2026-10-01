SELECT interview_mode,
       count() AS started,
       countIf(last_status='training_completed') AS completed,
       completed/nullIf(started,0) AS completion_rate,
       countIf(last_status='training_abandoned') AS abandoned,
       avgOrNull(answers) AS questions_answered,
       avgOrNullIf(dateDiff('second',started_at,ended_at),last_status='training_completed') AS completed_seconds
FROM knowledge_analytics.sessions
WHERE mode='mock' AND $__timeFilter(started_at) AND (${mode:sqlstring}='__all' OR mode=${mode:sqlstring}) AND (${algorithm:sqlstring}='__all' OR algorithm_version=${algorithm:sqlstring})
GROUP BY interview_mode
