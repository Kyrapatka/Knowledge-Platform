SELECT n.days,uniqExact(c.user_id) AS eligible_users,uniqExactIf(c.user_id,a.day=addDays(c.joined,n.days)) AS returned_users,returned_users/nullIf(eligible_users,0) AS retention
FROM knowledge_analytics.user_cohorts c CROSS JOIN (SELECT arrayJoin([1,7,30]) AS days) n
LEFT JOIN knowledge_analytics.user_activity a ON c.user_id=a.user_id
WHERE $__timeFilter(registered_at) AND addDays(c.joined,n.days)<toDate($__toTime,'UTC')
GROUP BY n.days ORDER BY n.days
