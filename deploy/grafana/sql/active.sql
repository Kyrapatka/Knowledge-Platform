SELECT uniqExactIf(user_id,occurred_at>=toDateTime($__toTime)-INTERVAL 1 DAY) AS DAU,
       uniqExactIf(user_id,occurred_at>=toDateTime($__toTime)-INTERVAL 7 DAY) AS WAU,
       uniqExact(user_id) AS MAU
FROM knowledge_analytics.events_unique
WHERE user_id!='' AND event_name!='login_failed'
  AND occurred_at BETWEEN toDateTime($__toTime)-INTERVAL 30 DAY AND $__toTime
