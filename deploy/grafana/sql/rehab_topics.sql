SELECT topic,
        count() AS answers,
       countIf(rehab_active_before=false AND rehab_active_after=true) AS entries,
       entries / nullIf(answers,0) AS rehab_rate
FROM knowledge_analytics.training_answers
WHERE review_credit=true AND $__timeFilter(occurred_at) AND (${mode:sqlstring}='__all' OR mode=${mode:sqlstring}) AND (${algorithm:sqlstring}='__all' OR algorithm_version=${algorithm:sqlstring})
GROUP BY topic
HAVING answers>=3
ORDER BY rehab_rate DESC,
       entries DESC
LIMIT 20
