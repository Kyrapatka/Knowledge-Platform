SELECT avgOrNullIf(dateDiff('second',started_at,ended_at),last_status IN ('training_completed','training_abandoned')) AS value
FROM knowledge_analytics.sessions
WHERE mode!='mock' AND $__timeFilter(started_at) AND (${mode:sqlstring}='__all' OR mode=${mode:sqlstring}) AND (${algorithm:sqlstring}='__all' OR algorithm_version=${algorithm:sqlstring})
