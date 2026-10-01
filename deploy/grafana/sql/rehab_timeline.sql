SELECT $__timeInterval(occurred_at) AS time,
       countIf(rehab_active_before=false AND rehab_active_after=true) AS entries,
       countIf(rehab_active_before=true AND rehab_active_after=false AND result='correct' AND review_kind='rehab') AS completions
FROM knowledge_analytics.training_answers
WHERE review_credit=true AND $__timeFilter(occurred_at) AND (${mode:sqlstring}='__all' OR mode=${mode:sqlstring}) AND (${algorithm:sqlstring}='__all' OR algorithm_version=${algorithm:sqlstring})
GROUP BY time
ORDER BY time
