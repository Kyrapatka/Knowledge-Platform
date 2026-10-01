SELECT folder_id,
       material_id,
       topic,
       count() AS answers,
       countIf(rehab_active_before=false AND rehab_active_after=true) AS rehab_entries,
       rehab_entries / nullIf(answers,0) AS rehab_rate
FROM knowledge_analytics.training_answers
WHERE review_credit=true AND $__timeFilter(occurred_at) AND (${mode:sqlstring}='__all' OR mode=${mode:sqlstring}) AND (${algorithm:sqlstring}='__all' OR algorithm_version=${algorithm:sqlstring})
GROUP BY folder_id,
       material_id,
       topic
HAVING rehab_entries>0
ORDER BY rehab_entries DESC,
       rehab_rate DESC
LIMIT 30
