SELECT 'folder' AS dimension,
       folder_id AS value,
       count() AS answers,
       countIf(result='correct')/count() AS correct_rate
FROM knowledge_analytics.training_answers
WHERE $__timeFilter(occurred_at) AND (${mode:sqlstring}='__all' OR mode=${mode:sqlstring}) AND (${algorithm:sqlstring}='__all' OR algorithm_version=${algorithm:sqlstring})
GROUP BY value
UNION ALL
SELECT 'template' AS dimension,
       template AS value,
       count() AS answers,
       countIf(result='correct')/count() AS correct_rate
FROM knowledge_analytics.training_answers
WHERE $__timeFilter(occurred_at) AND (${mode:sqlstring}='__all' OR mode=${mode:sqlstring}) AND (${algorithm:sqlstring}='__all' OR algorithm_version=${algorithm:sqlstring})
GROUP BY value
UNION ALL
SELECT 'topic' AS dimension,
       ifNull(topic,'(unknown)') AS value,
       count() AS answers,
       countIf(result='correct')/count() AS correct_rate
FROM knowledge_analytics.training_answers
WHERE $__timeFilter(occurred_at) AND (${mode:sqlstring}='__all' OR mode=${mode:sqlstring}) AND (${algorithm:sqlstring}='__all' OR algorithm_version=${algorithm:sqlstring})
GROUP BY value
ORDER BY dimension,
       answers DESC
LIMIT 15 BY dimension
