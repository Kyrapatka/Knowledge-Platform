SELECT countIf(result='correct') / nullIf(count(),0) AS value
FROM knowledge_analytics.training_answers
WHERE $__timeFilter(occurred_at) AND (${mode:sqlstring}='__all' OR mode=${mode:sqlstring}) AND (${algorithm:sqlstring}='__all' OR algorithm_version=${algorithm:sqlstring})
