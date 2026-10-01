"""Semantic SQL tests in a uniquely named temporary ClickHouse database.
Synthetic dates exist ONLY here; the database is dropped in finally.
"""
import base64, json, os, re, uuid
from datetime import datetime, timedelta, timezone
from pathlib import Path
from urllib.request import Request, urlopen
ROOT=Path(__file__).resolve().parent
DB='analytics_test_'+uuid.uuid4().hex
URL='http://127.0.0.1:'+os.getenv('KP_CLICKHOUSE_PORT','8123')+'/?date_time_input_format=best_effort'
AUTH='Basic '+base64.b64encode(('analytics_admin:'+os.getenv('KP_CLICKHOUSE_ADMIN_PASSWORD','local-clickhouse-admin-only')).encode()).decode()
def query(sql):
    with urlopen(Request(URL,data=sql.encode(),headers={'Authorization':AUTH}),timeout=30) as r:
        return r.read().decode()
def rows(sql):return [json.loads(x) for x in query(sql+' FORMAT JSONEachRow').splitlines()]
now=datetime.now(timezone.utc)
def event(name,days,**kw):
    e=dict(event_id=str(uuid.uuid4()),event_name=name,occurred_at=(now-timedelta(days=days)).isoformat(),user_id='user-a',session_id='session-a',plan_id='plan-a',mode='long_term',algorithm_version='v1',experiment_group='control',material_id='material-a',folder_id='folder-a',template='english_words')
    e.update(kw);return e
def panel_sql(target, mode='__all'):
    q=target['rawSql'].replace('knowledge_analytics',DB)
    for name,value in [('mode',mode),('algorithm','__all')]:
        q=q.replace('${'+name+':sqlstring}', "'"+value+"'")
    end=int(now.timestamp()); start=end-90*86400
    q=re.sub(r'\$__timeFilter\(([^)]+)\)',lambda m:f"({m[1]} BETWEEN toDateTime({start}) AND toDateTime({end}))",q)
    q=re.sub(r'\$__timeInterval\(([^)]+)\)',r'toStartOfHour(\1)',q)
    return q.replace('$__toTime',f'toDateTime({end})').replace('$__fromTime',f'toDateTime({start})')

dashboard=json.loads((ROOT/'grafana/dashboards/learning.json').read_text())
panels={p['title']:p for p in dashboard['panels'] if p.get('targets')}
def panel_rows(title,mode='__all'):
    return rows(panel_sql(panels[title]['targets'][0],mode))

try:
    for path in sorted((ROOT/'clickhouse').glob('[0-9]*.sql')):
        # SQL files use no semicolons inside literals; remove full-line comments.
        script='\n'.join(l for l in path.read_text().splitlines() if not l.lstrip().startswith('--'))
        for statement in script.split(';'):
            if statement.strip():query(statement.replace('knowledge_analytics',DB))
    for panel in panels.values():
        for target in panel['targets']:
            assert not re.search(r'SELECT\s+\*',target['rawSql'],re.I)
            rows(panel_sql(target))
    assert panel_rows('Training Sessions Started')==[{'value':0}]
    assert panel_rows('Correct Rate')==[{'value':None}]
    assert panel_rows('Average Session Duration')==[{'value':None}]
    events=[event('user_registered',45),event('user_logged_in',44),event('user_logged_in',38),event('user_logged_in',15),event('training_started',43),
        event('training_answered',42,result='wrong',review_credit=True,rehab_active_before=False,rehab_active_after=True,stage_before=1,stage_after=9,review_kind='stage'),
        event('training_answered',41.9,result='correct',review_credit=True,rehab_active_before=True,rehab_active_after=True,stage_before=1,review_kind='rehab'),
        event('training_answered',41.8,result='correct',review_credit=True,rehab_active_before=True,rehab_active_after=False,stage_before=1,review_kind='rehab'),
        event('training_answered',40,result='correct',review_credit=True,learned_before=False,learned_after=True,stage_before=5),
        event('training_answered',33,result='correct',review_credit=True),
        event('training_answered',32.9,result='wrong',review_credit=True),
        event('training_answered',10,result='wrong',review_credit=True),
        event('training_answered',33.1,result='wrong',review_credit=True,plan_id='another-plan'),
        event('training_started',2,session_id='mock-session',mode='mock',interview_mode='deep'),
        event('interview_question_answered',1.9,session_id='mock-session',mode='mock',result='correct',interview_depth=2)]
    undone=event('training_answered',39,result='wrong',review_credit=True)
    events += [undone,event('training_rollback',38.9,result='undo',related_event_id=undone['event_id'],mode='',algorithm_version='')]
    completed=event('training_completed',40)
    events += [completed,dict(completed,occurred_at=(now-timedelta(days=38.8)).isoformat())]
    # Duplicate delivery cannot double-count answers.
    events += [events[8]]
    # A learned material with no day-7/day-30 review is eligible, NOT a failure.
    events += [event('training_answered',40,result='correct',review_credit=True,learned_before=False,learned_after=True,material_id='unobserved')]
    query(f'INSERT INTO {DB}.analytics_events FORMAT JSONEachRow\n'+'\n'.join(json.dumps(e) for e in events))
    assert rows(f'SELECT count() AS n FROM {DB}.events_unique')[0]['n']==len(events)-2
    assert rows(f"SELECT count() AS n FROM {DB}.training_answers WHERE toString(event_id)='{undone['event_id']}'")[0]['n']==0
    session=rows(f"SELECT mode,last_status,answers FROM {DB}.sessions WHERE session_id='session-a'")[0]
    assert session['mode']=='long_term' and session['last_status']=='training_completed',session
    rehab=rows(f'SELECT attempts,recovered FROM {DB}.rehab_episodes')
    assert rehab==[{'attempts':2,'recovered':True}],rehab
    learned=rows(f"SELECT attempts_to_learned FROM {DB}.learned_materials WHERE material_id='material-a'")
    assert learned==[{'attempts_to_learned':4}],learned
    retention=rows(f'SELECT horizon,count() AS eligible,countIf(observed) AS reviewed,countIf(observed AND first_result=\'correct\') AS correct FROM {DB}.learning_retention GROUP BY horizon ORDER BY horizon')
    assert retention==[{'horizon':7,'eligible':2,'reviewed':1,'correct':1},{'horizon':30,'eligible':2,'reviewed':1,'correct':0}],retention
    mock=rows(f'SELECT count() AS n,avg(interview_depth) AS depth FROM {DB}.mock_answers')
    assert mock==[{'n':1,'depth':2}],mock
    for panel in panels.values():
        for target in panel['targets']: rows(panel_sql(target))
    assert panel_rows('Training Sessions Started')==[{'value':1}]
    assert panel_rows('Training Sessions Completed')==[{'value':1}]
    assert panel_rows('Completion Rate')==[{'value':1}]
    assert panel_rows('Answers Total')==[{'value':9}]
    assert abs(panel_rows('Correct Rate')[0]['value']-5/9)<1e-9
    assert abs(panel_rows('Wrong Rate')[0]['value']-4/9)<1e-9
    assert panel_rows('Answers Total','mock')==[{'value':0}]
    assert panel_rows('Recovery of observed rehab episodes')[0]['attempts_before_recovery']==2
    assert panel_rows('Mock answers and actual follow-up depth')[0]['answers']==1
    stage=[r for r in panel_rows('Correct / wrong rate by stage, difficulty, topic and template') if r['dimension']=='stage' and r['value']=='1']
    assert stage and stage[0]['answers']==3,stage
    assert panel_rows('Materials most often entering rehab')[0]['rehab_entries']==1
    timeline=panel_rows('Answers over time: total, correct and wrong')
    assert sum(r['answers'] for r in timeline)==9
    assert sum(r['correct'] for r in timeline)==5
    # Answers from older sessions must not disappear from mode/version comparisons.
    orphan=event('training_answered',1,session_id='started-before-window',mode='cram',algorithm_version='v2',result='wrong',review_credit=True)
    query(f'INSERT INTO {DB}.analytics_events FORMAT JSONEachRow\n'+json.dumps(orphan))
    for title in ['Cram / long-term: answers, rehab, sessions and learning','Version / group effectiveness']:
        comparison=panel_rows(title,'cram')
        assert len(comparison)==1 and comparison[0]['sessions']==0 and comparison[0]['answers']==1,comparison
        assert comparison[0]['wrong_rate']==1,comparison
    print(f"Dashboard SQL: {len(panels)} panels PASS on empty and populated isolated database; counts/rates/stage/mode/mock/rehab/time buckets verified.")
    print('SQL semantics: PASS (dedup, undo/recompletion, modes, rehab episodes, plan boundaries, mature 7/30-day observed retention, mock depth).')
finally:
    query(f'DROP DATABASE IF EXISTS {DB} SYNC')
