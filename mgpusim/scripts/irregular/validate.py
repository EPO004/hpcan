#!/usr/bin/env python3
"""Fixture, negative, real scheduler and oversubscription regression tests."""
import datetime,json,pathlib,subprocess,sys,os,shutil,hashlib,csv
ROOT=pathlib.Path(__file__).resolve().parents[2];os.chdir(ROOT)
out=ROOT/'results/irregular'/('fixtures-'+datetime.datetime.now(datetime.timezone.utc).strftime('%Y%m%dT%H%M%S.%fZ'));out.mkdir();shutil.copy2(ROOT/"bin/irregular",out/"irregular")
cases=[
 ('weighted',['-workload=sssp','-input=datasets/irregular/tiny_weighted.wel','-n=6'],True),
 ('sinks',['-workload=pagerank','-input=datasets/irregular/sink_graph.el','-n=6'],True),
 ('rectangular',['-workload=spmv','-input=datasets/irregular/rectangular.mtx'],True),
 ('symmetric',['-workload=cc','-input=datasets/irregular/symmetric.mtx'],True),
 ('source-middle',['-workload=bfs','-generator=path','-n=9','-source=4'],True),
 ('source-isolated',['-workload=bfs','-generator=empty','-n=9','-source=4'],True),
 ('invalid-source',['-workload=bfs','-n=4','-source=4'],False),
 ('invalid-n',['-n=0'],False),('invalid-workload',['-workload=madeup'],False),
 ('invalid-profile',['-workload=gather','-profile=madeup'],False),
 ('invalid-gpu',['-gpus=2,1'],False),('invalid-iteration',['-iterations=-1'],False),
 ('truncated-bfs',['-workload=bfs','-generator=path','-n=9','-iterations=1'],False),
 ('truncated-cc',['-workload=cc','-generator=path','-n=9','-iterations=1'],False),
 ('nonsquare-graph',['-workload=bfs','-input=datasets/irregular/rectangular.mtx'],False),
 ('nan-damping',['-workload=pagerank','-damping=NaN'],False),
]
for name,args,expected in list(cases[:6]):
 cases.append((name+'-timing2',args+['-timing','-unified-gpus=1,2','-use-unified-memory','-grit-mode=grit'],True))
for work in ([] if '--quick' in sys.argv else ['bfs','spmv','pagerank','sssp','cc']):
 cases.append((work+'-virtual',['-workload='+work,'-n=4097','-generator=skewed','-schedule=unified','-timing','-unified-gpus=1,2','-use-unified-memory','-grit-mode=grit']+(['-iterations=1'] if work=='pagerank' else []),True))
# Fewer destinations than GPUs: outputs checked, idle GPUs expected and disclosed.
cases.append(('tiny-shards',['-workload=cc','-n=3','-timing','-unified-gpus=1,2,3,4','-use-unified-memory','-grit-mode=grit'],True))
cases.append(('oversubscription',['-workload=gather','-n=65','-profile=phase-shift','-timing','-unified-gpus=1,2','-use-unified-memory','-grit-mode=dup','-grit-resident-pages=1'],True))
cases.append(('off-virtual-flag',['-workload=gather','-n=65','-profile=phase-shift','-timing','-unified-gpus=1,2','-grit-mode=off'],True))
cases.append(('legacy-off-uvm-rejected',['-workload=gather','-timing','-unified-gpus=1,2','-use-unified-memory','-grit-mode=off'],False))
results=[]
for name,args,expected in cases:
 d=out/name;d.mkdir();cmd=[str(out/'irregular'),*args,'-verify','-disable-rtm','-report-inst-count',f'-metric-file-name={d}/metrics',f'-result-json={d}/result.json']
 try:
  with (d/'stdout.log').open('w') as f:r=subprocess.run(cmd,stdout=f,stderr=subprocess.STDOUT,timeout=120)
  passed=(r.returncode==0)==expected
  if expected:
   passed=passed and json.loads((d/'result.json').read_text())['verify_pass']
   if '-timing' in args and name!='tiny-shards':
    activity={1:0,2:0}
    metrics=list(csv.DictReader((d/'metrics.csv').open(),skipinitialspace=True))
    passed=passed and any(m['where']=='Driver' and m['what']=='kernel_time' and float(m['value'])>0 for m in metrics)
    for m in metrics:
     if m['what']=='cu_inst_count':
      for i in activity:
       if m['where'].startswith(f'GPU[{i}].'):activity[i]+=int(float(m['value']))
    passed=passed and all(activity.values())
  entry=dict(name=name,command=cmd,expected_success=expected,exit_code=r.returncode,status='PASS' if passed else 'FAIL')
 except Exception as e:entry=dict(name=name,command=cmd,status='FAIL',notes=str(e))
 results.append(entry);print(name,entry['status'],flush=True)
(out/'provenance.json').write_text(json.dumps(dict(binary_sha256=hashlib.sha256((out/'irregular').read_bytes()).hexdigest(),git_commit=subprocess.check_output(['git','rev-parse','HEAD'],text=True).strip()),indent=2))
(out/'validation.json').write_text(json.dumps(results,indent=2));print(out);sys.exit(any(x['status']!='PASS' for x in results))
