SELECT topic,
       difficulty,
       count() AS answers,
       countIf(result='wrong') AS wrong,
       countIf(result='correct')/count() AS correct_rate,
       countIf(result='wrong')/count() AS wrong_rate,
       avgOrNull(answer_time_ms)/1000 AS answer_seconds
FROM knowledge_analytics.mock_answers
WHERE $__timeFilter(occurred_at) AND (${mode:sqlstring}='__all' OR mode=${mode:sqlstring}) AND (${algorithm:sqlstring}='__all' OR algorithm_version=${algorithm:sqlstring})
GROUP BY topic,
       difficulty
ORDER BY wrong DESC
LIMIT 30
