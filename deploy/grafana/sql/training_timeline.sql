SELECT $__timeInterval(occurred_at) AS time,
       countIf(event_name='training_started') AS started,
       countIf(event_name='training_completed') AS completed
FROM knowledge_analytics.events_effective
WHERE $__timeFilter(occurred_at)
  AND event_name IN ('training_started','training_completed')
  AND mode!='mock' AND (${mode:sqlstring}='__all' OR mode=${mode:sqlstring}) AND (${algorithm:sqlstring}='__all' OR algorithm_version=${algorithm:sqlstring})
GROUP BY time
ORDER BY time
