"""Explicit local Compose resilience check. Stops/restarts only this stack's ClickHouse.
Creates its own QA users and a temporary Noop backend; no production data is deleted.
"""
import json, os, re, subprocess, time, uuid
from urllib.request import Request, urlopen
BASE='http://127.0.0.1:'+os.getenv('KP_BACKEND_PORT','8080')
def call(base,path,data=None,token=None):
    headers={'Content-Type':'application/json'}
    if token:headers['Authorization']='Bearer '+token
    with urlopen(Request(base+path,data=json.dumps(data).encode() if data is not None else None,headers=headers),timeout=30) as r:
        raw=r.read().decode();return json.loads(raw) if raw else None

def metric(base,name):
    with urlopen(base+'/metrics',timeout=10) as r:text=r.read().decode()
    match=re.search(r'^'+re.escape(name)+r' ([\d.e+\-]+)$',text,re.M)
    assert match,'Missing metric '+name
    return float(match.group(1))
def wait(fn,seconds=35):
    end=time.monotonic()+seconds
    while time.monotonic()<end:
        try:
            if fn():return
        except Exception:pass
        time.sleep(1)
    raise AssertionError('Condition did not become true')
def cmd(*args):return subprocess.check_output(['docker',*args],text=True).strip()
def register(base):
    return call(base,'/api/v1/auth/register',{'nickname':'obs'+uuid.uuid4().hex[:10],'password':'ResilienceTest2026!'})['access_token']

before=metric(BASE,'analytics_batch_flush_errors_total')
try:
    cmd('compose','stop','clickhouse')
    token=register(BASE)
    folder=call(BASE,'/api/v1/folders',{'title':'Outage verification','template_key':'english_words'},token)
    call(BASE,f"/api/v1/folders/{folder['id']}/materials",{'values':{'foreign':'resilient','native':'устойчивый'},'metadata':{'topic':'Observability'},'difficulty':'easy'},token)
    view=call(BASE,'/api/v1/training/combined',{'sources':[{'folder_id':folder['id']}]},token)
    current=view['current'];card=current['presentation']
    call(BASE,f"/api/v1/training/sessions/{current['session_id']}/actions",{'command_id':str(uuid.uuid4()),'presentation_id':card['id'],'expected_version':card['progress_version'],'action':'correct'},token)
    call(BASE,'/ready')
    wait(lambda:metric(BASE,'analytics_batch_flush_errors_total')>before)
    call(BASE,'/ready')
    print('ClickHouse outage: auth + folder/material + training answer + core readiness PASS; flush errors increased.')
finally:
    cmd('compose','start','clickhouse')
wait(lambda:call(BASE,'/ready') is not None)
# A separate disposable process exercises the exact same binary with analytics off.
name='knowledge-noop-check-'+uuid.uuid4().hex[:8]
try:
    cmd('compose','run','--no-deps','--rm','-d','--name',name,'-e','ANALYTICS_ENABLED=false','-p','127.0.0.1:0:8080','backend')
    port=cmd('port',name,'8080/tcp').split(':')[-1]
    noop='http://127.0.0.1:'+port
    wait(lambda:call(noop,'/ready') is not None)
    register(noop)
    assert metric(noop,'analytics_events_published_total')==0
    assert metric(noop,'analytics_batch_flush_total')==0
    print('ANALYTICS_ENABLED=false: real registration + readiness PASS, published=0, flushes=0.')
finally:
    subprocess.run(['docker','stop',name],stdout=subprocess.DEVNULL,check=False)
