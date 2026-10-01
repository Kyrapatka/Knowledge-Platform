SELECT $__timeInterval(occurred_at) AS time,
       count() AS answers,
       countIf(result='correct') AS correct,
       countIf(result='wrong') AS wrong
FROM knowledge_analytics.training_answers
WHERE $__timeFilter(occurred_at) AND (${mode:sqlstring}='__all' OR mode=${mode:sqlstring}) AND (${algorithm:sqlstring}='__all' OR algorithm_version=${algorithm:sqlstring})
GROUP BY time
ORDER BY time
