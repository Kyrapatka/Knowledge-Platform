WITH u AS (
SELECT user_id,
nullIf(minIf(occurred_at,event_name='user_registered'),toDateTime64(0,3,'UTC')) AS registered,
nullIf(minIf(occurred_at,event_name IN ('folder_created','folder_imported')),toDateTime64(0,3,'UTC')) AS folder,
nullIf(minIf(occurred_at,event_name IN ('material_created','folder_imported')),toDateTime64(0,3,'UTC')) AS material,
nullIf(minIf(occurred_at,event_name='training_started' AND mode!='mock'),toDateTime64(0,3,'UTC')) AS training,
nullIf(minIf(occurred_at,event_name='training_completed' AND mode!='mock'),toDateTime64(0,3,'UTC')) AS completed,
groupArrayIf(occurred_at,event_name!='user_registered') AS activity
FROM knowledge_analytics.events_unique WHERE user_id!='' AND occurred_at<=$__toTime GROUP BY user_id),
f AS (SELECT *,folder>=registered AND material>=folder AND training>=material AND completed>=training AS finished FROM u WHERE $__timeFilter(registered))
SELECT count() AS registrations,countIf(folder>=registered) AS first_folder,countIf(folder>=registered AND material>=folder) AS first_content,
countIf(folder>=registered AND material>=folder AND training>=material) AS first_training,countIf(finished) AS first_completion,
countIf(finished AND completed<toDateTime(addDays(toDate(registered),2)) AND addDays(toDate(registered),1)<toDate($__toTime)) AS d1_eligible,
countIf(finished AND completed<toDateTime(addDays(toDate(registered),2)) AND addDays(toDate(registered),1)<toDate($__toTime) AND arrayExists(t -> t>=completed AND toDate(t)=addDays(toDate(registered),1),activity)) AS d1_return,
countIf(finished AND completed<toDateTime(addDays(toDate(registered),2)) AND addDays(toDate(registered),7)<toDate($__toTime) AND arrayExists(t -> t>=completed AND toDate(t)=addDays(toDate(registered),1),activity)) AS d7_eligible,
countIf(finished AND completed<toDateTime(addDays(toDate(registered),2)) AND addDays(toDate(registered),7)<toDate($__toTime) AND arrayExists(t -> t>=completed AND toDate(t)=addDays(toDate(registered),1),activity) AND arrayExists(t -> t>=completed AND toDate(t)=addDays(toDate(registered),7),activity)) AS d7_return FROM f
