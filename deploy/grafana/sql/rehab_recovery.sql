SELECT mode,
       entry_stage,
       count() AS entries,
       countIf(recovered) AS completions,
       completions/nullIf(entries,0) AS recovery_rate,
       avgOrNullIf(attempts,recovered) AS attempts_before_recovery
FROM knowledge_analytics.rehab_episodes
WHERE $__timeFilter(entered_at) AND (${mode:sqlstring}='__all' OR mode=${mode:sqlstring}) AND (${algorithm:sqlstring}='__all' OR algorithm_version=${algorithm:sqlstring})
GROUP BY mode,
       entry_stage
