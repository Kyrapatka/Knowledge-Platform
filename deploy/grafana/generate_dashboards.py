"""Generate the two provisioned dashboards and inspectable SQL (stdlib only)."""
import json
from pathlib import Path
ROOT = Path(__file__).resolve().parent
PROM = {"type": "prometheus", "uid": "knowledge-prometheus"}
CH = {"type": "grafana-clickhouse-datasource", "uid": "knowledge-clickhouse"}

class Dashboard:
    def __init__(self, title, uid, datasource):
        self.ds, self.panels, self.y = datasource, [], 0
        self.data = dict(title=title, uid=uid, schemaVersion=39, version=1, editable=False,
                         timezone="utc", refresh="30s", tags=["knowledge-platform"],
                         time={"from":"now-7d" if datasource==CH else "now-1h","to":"now"},
                         panels=self.panels, templating={"list":[]})
    def row(self, title):
        self.panels.append(dict(id=len(self.panels)+1,type="row",title=title,collapsed=False,
                                gridPos=dict(x=0,y=self.y,w=24,h=1),panels=[]))
        self.y+=1
    def panel(self, title, queries=None, kind="timeseries", unit="short", description="", width=12, x=0, height=8, text=None):
        p=dict(id=len(self.panels)+1,type=kind,title=title,description=description,
               gridPos=dict(x=x,y=self.y,w=width,h=height),
               fieldConfig={"defaults":{"unit":unit,"noValue":"No observations","color":{"mode":"palette-classic"}},"overrides":[]},
               options={"legend":{"displayMode":"table","placement":"bottom"},"tooltip":{"mode":"multi"}})
        if kind=="stat": p["options"]={"reduceOptions":{"calcs":["lastNotNull"],"fields":"","values":False},"colorMode":"value","graphMode":"none"}
        if kind=="table": p["options"]={"showHeader":True,"cellHeight":"sm"}
        if text is not None: p["options"]={"mode":"markdown","content":text}
        else:
            p["datasource"]=self.ds
            p["targets"]=[]
            for i,(legend,query) in enumerate(queries.items()):
                t=dict(refId=chr(65+i),datasource=self.ds)
                if self.ds==PROM: t.update(expr=query,legendFormat=legend,range=kind!="stat",instant=kind=="stat")
                else: t.update(rawSql=query,queryType="sql",editorType="sql",format=1)
                p["targets"].append(t)
        self.panels.append(p)
        if x+width>=24:self.y+=height
        return p
    def save(self,name):
        (ROOT/'dashboards').mkdir(exist_ok=True)
        (ROOT/'dashboards'/name).write_text(json.dumps(self.data,indent=2)+'\n',encoding='utf-8')

def sql(name,query):
    q=query.strip()
    (ROOT/'sql').mkdir(exist_ok=True)
    (ROOT/'sql'/f'{name}.sql').write_text(q+'\n',encoding='utf-8')
    return q

s=Dashboard('Knowledge Platform - System','knowledge-system',PROM)
s.row('Backend overview')
sel='{job="knowledge-backend",route!~"/health|/live|/ready"}'
requests='http_requests_total'+sel
rps=f'sum(rate({requests}[5m]))'
err5='sum(rate(http_requests_total{job="knowledge-backend",status=~"5..",route!~"/health|/live|/ready"}[5m])) or vector(0)'
lat=lambda p:f'histogram_quantile({p}, sum by (le) (rate(http_request_duration_seconds_bucket{sel}[5m])))'
for i,(title,q,unit) in enumerate([('Requests / second',rps,'reqps'),('5xx / second',err5,'reqps'),('p95 latency',lat(.95),'s'),('In-flight requests','sum(http_requests_in_flight{job="knowledge-backend"})','short')]):
    s.panel(title,{title:q},'stat',unit,width=6,x=i*6,height=4)
s.panel('Scope',kind='text',width=24,height=4,text='Traffic/latency exclude readiness and liveness probes. **4xx are client outcomes, not automatically backend failures.** No requests means no latency observations. Analytics counters measure best-effort delivery; published is queue acceptance, not a ClickHouse ACK. Core health is independent of ClickHouse.')
s.row('Traffic, client errors and backend failures')
s.panel('RPS by route / method',{'{{method}} {{route}}':f'sum by(route,method)(rate({requests}[5m]))'},unit='reqps')
s.panel('4xx and 5xx rates',{'4xx':'sum(rate(http_requests_total{job="knowledge-backend",status=~"4.."}[5m])) or vector(0)','5xx':err5},unit='reqps',x=12)
s.panel('Error percentage',{'4xx':'100 * (sum(rate(http_requests_total{job="knowledge-backend",status=~"4..",route!~"/health|/live|/ready"}[5m])) or vector(0)) / '+rps,'5xx':f'100 * ({err5}) / ({rps})'},unit='percent')
s.panel('Errors in selected range',{'4xx':'sum(increase(http_requests_total{job="knowledge-backend",status=~"4.."}[$__range])) or vector(0)','5xx':'sum(increase(http_requests_total{job="knowledge-backend",status=~"5.."}[$__range])) or vector(0)'},'stat',x=12)
s.panel('Latency percentiles',{'p50':lat(.5),'p95':lat(.95),'p99':lat(.99)},unit='s')
s.panel('p95 by route',{'{{route}}':f'histogram_quantile(0.95,sum by(le,route)(rate(http_request_duration_seconds_bucket{sel}[5m])))'},unit='s',x=12)
s.row('Go runtime, memory and CPU')
s.panel('Goroutines / threads / GOMAXPROCS',{name:f'{name}{{job="knowledge-backend"}}' for name in ['go_goroutines','go_threads','go_sched_gomaxprocs_threads']})
s.panel('Memory: heap and RSS',{name:f'{metric}{{job="knowledge-backend"}}' for name,metric in [('Allocated','go_memstats_alloc_bytes'),('Heap','go_memstats_heap_alloc_bytes'),('RSS','process_resident_memory_bytes')]},unit='bytes',x=12)
s.panel('GC cycles / second',{'cycles':'sum(rate(go_gc_duration_seconds_count{job="knowledge-backend"}[5m]))'})
s.panel('GC pauses',{'p50':'go_gc_duration_seconds{job="knowledge-backend",quantile="0.5"}','max':'go_gc_duration_seconds{job="knowledge-backend",quantile="1"}','mean':'sum(rate(go_gc_duration_seconds_sum{job="knowledge-backend"}[5m])) / sum(rate(go_gc_duration_seconds_count{job="knowledge-backend"}[5m]))'},unit='s',x=12,description='Go collector summary: quantile 1 is the maximum pause, not p99. ')
s.panel('Process CPU (cores)',{'CPU cores':'sum(rate(process_cpu_seconds_total{job="knowledge-backend"}[5m]))'},description='1.0 means one fully occupied logical CPU.')
s.panel('Backend scrape health',{'UP':'up{job="knowledge-backend"}'},'stat',x=12)
s.row('Analytics pipeline health')
s.panel('Published / second',{'accepted':'sum(rate(analytics_events_published_total{job="knowledge-backend"}[5m]))'})
s.panel('Dropped events by reason',{'{{reason}}':'sum by(reason)(increase(analytics_events_dropped_total{job="knowledge-backend"}[$__range]))'},'stat',x=12)
s.panel('Batch flushes / second',{'attempts':'sum(rate(analytics_batch_flush_total{job="knowledge-backend"}[5m]))','errors':'sum(rate(analytics_batch_flush_errors_total{job="knowledge-backend"}[5m]))'})
s.panel('Flush errors in selected range',{'errors':'sum(increase(analytics_batch_flush_errors_total{job="knowledge-backend"}[$__range]))'},'stat',x=12)
s.save('system.json')

l=Dashboard('Knowledge Platform - Learning Analytics','knowledge-learning',CH)
for name,column in [('mode','mode'),('algorithm','algorithm_version'),('experiment','experiment_group')]:
    l.data['templating']['list'].append(dict(name=name,label=name.title(),type='query',datasource=CH,
        query=f"SELECT DISTINCT {column} FROM knowledge_analytics.events_unique WHERE {column}!='' ORDER BY {column}",
        refresh=1,multi=False,includeAll=True,allValue="'__all'",current={'text':'All','value':'$__all'},options=[]))
F=" AND ".join(f"(${{{v}:sqlstring}}='__all' OR {c}=${{{v}:sqlstring}})" for v,c in [('mode','mode'),('algorithm','algorithm_version'),('experiment','experiment_group')])
T=lambda col:f'$__timeFilter({col})'
def panel(name,title,query,description='',width=12,x=0,kind='table',height=8,unit='short'):
    return l.panel(title,{'A':sql(name,query)},kind,unit,description,width,x,height)
l.row('Overview - observed events, UTC')
l.panel('Definitions and coverage',kind='text',width=24,height=6,text='**Best-effort analytics, no historical backfill.** Counts use deduplicated events and exclude undone answers. Normal training and mock answers stay separate. Sessions are component sessions (combined training can create several). Abandonment means explicit cancellation, not closing a tab. Answer time includes idle/network time. User return retention != learned-material review success. Blank rates mean no observations, not 0%. Filters apply to training/mock/algorithm panels; user cohorts and funnel remain global. A/B groups stay unassigned until a real assignment mechanism exists.')
panel('active','Active users (rolling to range end)',f"SELECT uniqExactIf(user_id,occurred_at>=toDateTime($__toTime)-INTERVAL 1 DAY) AS DAU, uniqExactIf(user_id,occurred_at>=toDateTime($__toTime)-INTERVAL 7 DAY) AS WAU, uniqExact(user_id) AS MAU FROM knowledge_analytics.events_unique WHERE user_id!='' AND event_name!='login_failed' AND occurred_at BETWEEN toDateTime($__toTime)-INTERVAL 30 DAY AND $__toTime",kind='stat',width=8,height=5)
panel('overview_sessions','Normal training sessions',f"SELECT count() AS Started,countIf(last_status='training_completed') AS Completed,countIf(last_status='training_abandoned') AS Abandoned FROM knowledge_analytics.sessions WHERE mode!='mock' AND {T('started_at')} AND {F}",kind='stat',width=8,x=8,height=5)
panel('overview_correct','Normal answer correctness',f"SELECT countIf(result='correct')/nullIf(count(),0) AS CorrectRate FROM knowledge_analytics.training_answers WHERE {T('occurred_at')} AND {F}",kind='stat',unit='percentunit',width=8,x=16,height=5)
l.row('Training usage and learning effectiveness')
panel('sessions','Sessions and completion by mode',f"""SELECT mode,count() AS started,countIf(last_status='training_completed') AS completed,
countIf(last_status='training_abandoned') AS abandoned,completed/nullIf(started,0) AS completion_rate,
avgOrNull(answers) AS answers_per_session,
avgOrNullIf(dateDiff('second',started_at,ended_at),last_status IN ('training_completed','training_abandoned')) AS closed_session_seconds
FROM knowledge_analytics.sessions WHERE mode!='mock' AND {T('started_at')} AND {F} GROUP BY mode""",'Start cohort in selected range; outcomes observed through now. Open sessions have no duration.',width=24)
panel('answers','Answers, correctness and elapsed time',f"SELECT mode,count() AS answers,countIf(result='correct') AS correct,countIf(result='wrong') AS wrong,correct/nullIf(answers,0) AS correct_rate,avgOrNull(answer_time_ms)/1000 AS answer_seconds FROM knowledge_analytics.training_answers WHERE {T('occurred_at')} AND {F} GROUP BY mode",width=24)
parts=[]
for dim,expr in [('stage','toString(stage_before)'),('difficulty','difficulty'),('topic','topic'),('template','template')]:
    parts.append(f"SELECT '{dim}' AS dimension,ifNull({expr},'(unknown)') AS value,count() AS answers,countIf(result='correct')/count() AS correct_rate,countIf(result='wrong')/count() AS wrong_rate,avgOrNull(answer_time_ms)/1000 AS answer_seconds FROM knowledge_analytics.training_answers WHERE {T('occurred_at')} AND {F} GROUP BY value")
panel('dimensions','Correct / wrong rate by stage, difficulty, topic and template','\nUNION ALL\n'.join(parts)+'\nORDER BY dimension,answers DESC',width=24,height=11)
panel('learned','Observed attempts and time until learned',f"SELECT mode,algorithm_version,experiment_group,count() AS learned_materials,avgOrNull(attempts_to_learned) AS attempts,avgOrNull(seconds_to_learned) AS observed_seconds FROM knowledge_analytics.learned_materials WHERE {T('learned_at')} AND {F} GROUP BY mode,algorithm_version,experiment_group",'First observed false -> true learned transition in a plan. Attempts/time begin at first recorded effective answer, not necessarily first-ever study.',width=24)
panel('learning_retention','Learning retention: first observed review in day 7 / day 30 window',f"SELECT horizon AS days,mode,algorithm_version,experiment_group,count() AS eligible_materials,countIf(observed) AS reviewed_materials,countIf(observed AND first_result='correct')/nullIf(reviewed_materials,0) AS correct_rate,reviewed_materials/nullIf(eligible_materials,0) AS observation_coverage FROM knowledge_analytics.learning_retention WHERE {T('learned_at')} AND {F} GROUP BY horizon,mode,algorithm_version,experiment_group",'Same user/material/plan/version; first review in [N,N+1) days, only fully mature windows. Unobserved materials are not wrong. No synthetic/backdated production data.',width=24,height=9)
l.row('Rehab and cram vs long-term')
panel('rehab','Rehab entry and wrong rate by stage',f"SELECT stage_before AS stage,mode,count() AS answers,countIf(rehab_active_before=false AND rehab_active_after=true) AS entries,entries/count() AS entry_rate,countIf(result='wrong')/count() AS wrong_rate FROM knowledge_analytics.training_answers WHERE review_credit=true AND {T('occurred_at')} AND {F} GROUP BY stage,mode ORDER BY entries DESC",width=12)
panel('rehab_recovery','Recovery of observed rehab episodes',f"SELECT mode,entry_stage,count() AS entries,countIf(recovered) AS completions,completions/nullIf(entries,0) AS recovery_rate,avgOrNull(attempts) AS attempts_per_episode FROM knowledge_analytics.rehab_episodes WHERE {T('entered_at')} AND {F} GROUP BY mode,entry_stage",'Cohort of observed entries in range; recovery requires a correct rehab exit. Manual skip is not recovery. Open episodes remain in the denominator.',x=12)
panel('mode_comparison','Cram / long-term: answers, rehab, sessions and learning',f"""WITH a AS (SELECT mode,count() AS answers,countIf(result='correct')/count() AS correct_rate,avgOrNull(answer_time_ms)/1000 AS answer_seconds,countIf(rehab_active_before=false AND rehab_active_after=true)/count() AS rehab_rate FROM knowledge_analytics.training_answers WHERE {T('occurred_at')} AND {F} GROUP BY mode),
s AS (SELECT mode,count() AS sessions,countIf(last_status='training_completed')/count() AS completion_rate FROM knowledge_analytics.sessions WHERE mode!='mock' AND {T('started_at')} AND {F} GROUP BY mode),
g AS (SELECT mode,avgOrNull(attempts_to_learned) AS attempts_to_learned,avgOrNull(seconds_to_learned) AS time_to_learned FROM knowledge_analytics.learned_materials WHERE {T('learned_at')} AND {F} GROUP BY mode)
SELECT s.mode,s.sessions,s.completion_rate,a.answers,a.correct_rate,a.answer_seconds,a.rehab_rate,g.attempts_to_learned,g.time_to_learned FROM s LEFT JOIN a USING(mode) LEFT JOIN g USING(mode)""",'Observational comparison, not a randomized causal estimate. Retention comparison is in the learning-retention table.',width=24)
l.row('Mock Interview - practice-only events')
panel('mock_sessions','Mock sessions and deep usage',f"SELECT interview_mode,count() AS started,countIf(last_status='training_completed') AS completed,countIf(last_status='training_abandoned') AS abandoned,avgOrNull(answers) AS questions_answered,avgOrNullIf(dateDiff('second',started_at,ended_at),last_status='training_completed') AS completed_seconds FROM knowledge_analytics.sessions WHERE mode='mock' AND {T('started_at')} AND {F} GROUP BY interview_mode",'Mode comes from actual start configuration. Empty means events before instrumentation; never inferred from answer text.',width=24)
panel('mock_answers','Mock answers and actual follow-up depth',f"SELECT count() AS answers,countIf(result='correct') AS correct,countIf(result='wrong') AS wrong,correct/nullIf(answers,0) AS correct_rate,avgOrNull(answer_time_ms)/1000 AS answer_seconds,avgOrNull(interview_depth) AS mean_depth,countIf(interview_depth>0) AS followup_answers FROM knowledge_analytics.mock_answers WHERE {T('occurred_at')} AND {F}",width=24)
panel('mock_failures','Mock failure hotspots by topic / difficulty',f"SELECT topic,difficulty,count() AS answers,countIf(result='wrong') AS wrong,countIf(result='correct')/count() AS correct_rate,avgOrNull(answer_time_ms)/1000 AS answer_seconds FROM knowledge_analytics.mock_answers WHERE {T('occurred_at')} AND {F} GROUP BY topic,difficulty ORDER BY wrong DESC LIMIT 30",width=24)
l.row('Content usage - IDs and typed taxonomy only')
parts=[]
for dim,expr in [('folder','folder_id'),('template','template'),('topic',"ifNull(topic,'(unknown)')")]:
    parts.append(f"SELECT '{dim}' AS dimension,{expr} AS value,count() AS answers,countIf(result='correct')/count() AS correct_rate FROM knowledge_analytics.training_answers WHERE {T('occurred_at')} AND {F} GROUP BY value")
panel('content_usage','Most used folders / templates / topics','\nUNION ALL\n'.join(parts)+'\nORDER BY answers DESC LIMIT 40',width=12,height=11)
panel('hard_materials','Material wrong / rehab rates',f"SELECT folder_id,material_id,topic,count() AS answers,countIf(result='wrong')/count() AS wrong_rate,countIf(rehab_active_before=false AND rehab_active_after=true) AS rehab_entries FROM knowledge_analytics.training_answers WHERE {T('occurred_at')} AND {F} GROUP BY folder_id,material_id,topic HAVING answers>=3 ORDER BY wrong_rate DESC,answers DESC LIMIT 30",'Minimum 3 observations; sample sizes shown. No raw question or answer text.',x=12,height=11)
panel('hard_topics','Topics with lowest correct rate',f"SELECT topic,count() AS answers,countIf(result='correct')/count() AS correct_rate FROM knowledge_analytics.training_answers WHERE {T('occurred_at')} AND {F} GROUP BY topic HAVING answers>=3 ORDER BY correct_rate,answers DESC LIMIT 20",width=24)
l.row('Product retention and ordered first-use funnel')
panel('product_retention','User return retention D1 / D7 / D30',f"""SELECT n.days,uniqExact(c.user_id) AS eligible_users,uniqExactIf(c.user_id,a.day=addDays(c.joined,n.days)) AS returned_users,returned_users/nullIf(eligible_users,0) AS retention
FROM knowledge_analytics.user_cohorts c CROSS JOIN (SELECT arrayJoin([1,7,30]) AS days) n
LEFT JOIN knowledge_analytics.user_activity a ON c.user_id=a.user_id
WHERE {T('registered_at')} AND addDays(c.joined,n.days)<toDate($__toTime,'UTC')
GROUP BY n.days ORDER BY n.days""",'UTC registration-day cohorts; exact calendar return day. Only fully mature cohorts. Recorded activity, not page views.',width=24)
panel('funnel','Registration  ->  folder  ->  content  ->  training  ->  completion  ->  returns',f"""WITH u AS (
SELECT user_id,
nullIf(minIf(occurred_at,event_name='user_registered'),toDateTime64(0,3,'UTC')) AS registered,
nullIf(minIf(occurred_at,event_name IN ('folder_created','folder_imported')),toDateTime64(0,3,'UTC')) AS folder,
nullIf(minIf(occurred_at,event_name IN ('material_created','folder_imported')),toDateTime64(0,3,'UTC')) AS material,
nullIf(minIf(occurred_at,event_name='training_started' AND mode!='mock'),toDateTime64(0,3,'UTC')) AS training,
nullIf(minIf(occurred_at,event_name='training_completed' AND mode!='mock'),toDateTime64(0,3,'UTC')) AS completed,
groupArrayIf(occurred_at,event_name!='user_registered') AS activity
FROM knowledge_analytics.events_unique WHERE user_id!='' AND occurred_at<=$__toTime GROUP BY user_id),
f AS (SELECT *,folder>=registered AND material>=folder AND training>=material AND completed>=training AS finished FROM u WHERE {T('registered')})
SELECT count() AS registrations,countIf(folder>=registered) AS first_folder,countIf(folder>=registered AND material>=folder) AS first_content,
countIf(folder>=registered AND material>=folder AND training>=material) AS first_training,countIf(finished) AS first_completion,
countIf(finished AND completed<toDateTime(addDays(toDate(registered),2)) AND addDays(toDate(registered),1)<toDate($__toTime)) AS d1_eligible,
countIf(finished AND completed<toDateTime(addDays(toDate(registered),2)) AND addDays(toDate(registered),1)<toDate($__toTime) AND arrayExists(t -> t>=completed AND toDate(t)=addDays(toDate(registered),1),activity)) AS d1_return,
countIf(finished AND completed<toDateTime(addDays(toDate(registered),2)) AND addDays(toDate(registered),7)<toDate($__toTime) AND arrayExists(t -> t>=completed AND toDate(t)=addDays(toDate(registered),1),activity)) AS d7_eligible,
countIf(finished AND completed<toDateTime(addDays(toDate(registered),2)) AND addDays(toDate(registered),7)<toDate($__toTime) AND arrayExists(t -> t>=completed AND toDate(t)=addDays(toDate(registered),1),activity) AND arrayExists(t -> t>=completed AND toDate(t)=addDays(toDate(registered),7),activity)) AS d7_return FROM f""",'Ordered first-use stages. Import qualifies as content. D1/D7 are exact UTC return days; D7 is conditional on the preceding D1 stage. Eligible columns prevent immature cohorts being counted as failures.',width=24)
l.row('Algorithm versions and experiment groups')
panel('algorithm','Version / group effectiveness',f"""WITH a AS (SELECT mode,algorithm_version,experiment_group,count() AS answers,countIf(result='correct')/count() AS correct_rate,avgOrNull(answer_time_ms)/1000 AS answer_seconds,countIf(rehab_active_before=false AND rehab_active_after=true)/count() AS rehab_rate FROM knowledge_analytics.training_answers WHERE {T('occurred_at')} AND {F} GROUP BY mode,algorithm_version,experiment_group),
s AS (SELECT mode,algorithm_version,experiment_group,count() AS sessions,countIf(last_status='training_completed')/count() AS completion_rate FROM knowledge_analytics.sessions WHERE mode!='mock' AND {T('started_at')} AND {F} GROUP BY mode,algorithm_version,experiment_group),
g AS (SELECT mode,algorithm_version,experiment_group,avgOrNull(attempts_to_learned) AS attempts_to_learned,avgOrNull(seconds_to_learned) AS time_to_learned FROM knowledge_analytics.learned_materials WHERE {T('learned_at')} AND {F} GROUP BY mode,algorithm_version,experiment_group)
SELECT s.mode,s.algorithm_version,if(s.experiment_group='','(unassigned)',s.experiment_group) AS experiment_group,s.sessions,s.completion_rate,a.answers,a.correct_rate,a.answer_seconds,a.rehab_rate,g.attempts_to_learned,g.time_to_learned FROM s LEFT JOIN a USING(mode,algorithm_version,experiment_group) LEFT JOIN g USING(mode,algorithm_version,experiment_group)""",'No invented v2 or random A/B assignments. Compare actual emitted versions/groups. 7/30-day learning retention by version/group appears above.',width=24,height=10)
# Consistent readable units for tables without multiplying dimension columns.
for p in l.panels:
    if p['type']=='table':
        p['fieldConfig']['overrides']=[
            {'matcher':{'id':'byRegexp','options':'.*(rate|retention|coverage)$'},'properties':[{'id':'unit','value':'percentunit'},{'id':'decimals','value':1}]},
            {'matcher':{'id':'byRegexp','options':'.*(seconds|time_to_learned)$'},'properties':[{'id':'unit','value':'s'}]},
        ]
l.save('learning.json')
