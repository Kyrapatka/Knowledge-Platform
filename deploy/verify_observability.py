"""Validate provisioned dashboards through Grafana, plus Prometheus and ClickHouse.
No synthetic events are inserted. Run after the opt-in browser smoke test for --require-events.
"""
import argparse, base64, json, os, re, sys, time
from pathlib import Path
from urllib.request import Request, urlopen
from urllib.parse import urlencode
from urllib.error import HTTPError
ROOT = Path(__file__).resolve().parent

def request(base, path, body=None, auth=None):
    headers={}
    if auth: headers['Authorization']='Basic '+base64.b64encode(':'.join(auth).encode()).decode()
    if body is not None: headers['Content-Type']='application/json'
    req=Request(base+path,data=json.dumps(body).encode() if body is not None else None,headers=headers)
    try:
        with urlopen(req,timeout=45) as r: return json.load(r)
    except HTTPError as e:
        # Grafana query errors have no application tokens or content bodies.
        message=e.read().decode()
        raise RuntimeError(f'{path}: HTTP {e.code}: {message[:1800]}') from None

def main():
    parser=argparse.ArgumentParser();parser.add_argument('--require-events',action='store_true');parser.add_argument('--quick',action='store_true');args=parser.parse_args()
    grafana='http://127.0.0.1:'+os.getenv('KP_GRAFANA_PORT','3000')
    prom='http://127.0.0.1:'+os.getenv('KP_PROMETHEUS_PORT','9090')
    auth=(os.getenv('KP_GRAFANA_USER','admin'),os.getenv('KP_GRAFANA_PASSWORD','local-grafana-only'))
    targets=request(prom,'/api/v1/targets')['data']['activeTargets']
    backend=[t for t in targets if t['labels'].get('job')=='knowledge-backend']
    assert backend and all(t['health']=='up' for t in backend), 'Backend target is not UP'
    print('Prometheus backend target: UP')
    for uid in ['knowledge-prometheus','knowledge-clickhouse']:
        health=request(grafana,f'/api/datasources/uid/{uid}/health',auth=auth)
        assert health.get('status')=='OK',health
        print(uid+': OK')
    base='http://127.0.0.1:'+os.getenv('KP_BACKEND_PORT','8080')
    def metrics():
        with urlopen(base+'/metrics',timeout=10) as r: return r.read().decode()
    def total(raw,name):
        return sum(float(x) for x in re.findall('^'+name+r'(?:\{[^\n]*\})? ([0-9.e+\-]+)$',raw,re.M))
    for url in [base+'/live',base+'/ready',grafana+'/api/health',prom+'/-/ready','http://127.0.0.1:'+os.getenv('KP_CLICKHOUSE_PORT','8123')+'/ping']:
        with urlopen(url,timeout=10) as r: assert r.status==200,url
    before=metrics()
    with urlopen(base+'/live',timeout=10) as r: assert r.status==200
    after=metrics()
    for name in ['http_requests_total','http_request_duration_seconds_count']:
        assert total(after,name)>total(before,name),name+' did not increase'
    for name in ['http_requests_in_flight','go_goroutines','go_threads','go_sched_gomaxprocs_threads',
                 'go_memstats_alloc_bytes','go_memstats_heap_alloc_bytes','process_resident_memory_bytes',
                 'go_gc_duration_seconds_count','process_cpu_seconds_total','analytics_events_published_total',
                 'analytics_events_dropped_total','analytics_batch_flush_total','analytics_batch_flush_errors_total']:
        assert re.search('^'+name+r'(?:\{| )',after,re.M),name+' missing'
    print('HTTP counter + histogram increase; in-flight, Go/process and analytics metrics present')
    now=int(time.time()*1000);failures=[];checked=0
    for filename in ['system.json','learning.json']:
        d=json.loads((ROOT/'grafana'/'dashboards'/filename).read_text())
        live=request(grafana,'/api/dashboards/uid/'+d['uid'],auth=auth)['dashboard']
        assert live['title']==d['title']
        for panel in ([] if args.quick else d['panels']):
            for target in panel.get('targets',[]):
                try:
                    if 'expr' in target:
                        expr=target['expr'].replace('$__range','1h')
                        result=request(prom,'/api/v1/query?'+urlencode({'query':expr}))
                        assert result['status']=='success'
                    else:
                        target=dict(target)
                        for v in ['mode','algorithm','experiment']:
                            target['rawSql']=target['rawSql'].replace('${'+v+':sqlstring}',"'__all'")
                        target.update(intervalMs=15000,maxDataPoints=500)
                        result=request(grafana,'/api/ds/query',{'from':str(now-90*86400000),'to':str(now),'queries':[target]},auth)
                        for r in result['results'].values():
                            assert not r.get('error'),r.get('error')
                            assert r.get('status',200)==200,r
                    checked+=1
                except Exception as e: failures.append((panel['title'],str(e)))
        print(d['title']+': provisioned')
    if args.require_events:
        for metric in ['analytics_events_published_total','analytics_batch_flush_total']:
            data=request(prom,'/api/v1/query?'+urlencode({'query':metric+'{job="knowledge-backend"}'}))['data']['result']
            assert data and float(data[0]['value'][1])>0,metric+' must be positive after real activity'
    print('Quick health/provisioning checks PASS' if args.quick else 'Full dashboard query checks complete')
    print(f'Queries checked: {checked}; failures: {len(failures)}')
    for title,error in failures: print(title+': '+error)
    if failures: sys.exit(1)
if __name__=='__main__': main()
