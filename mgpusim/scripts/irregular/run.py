#!/usr/bin/env python3
"""Bounded real simulator runs; every case preserves command, log and status."""
import shutil
import argparse, csv, datetime, hashlib, json, os, pathlib, subprocess, sys
ROOT=pathlib.Path(__file__).resolve().parents[2]
def digest(p): return hashlib.sha256(p.read_bytes()).hexdigest()
def main():
 p=argparse.ArgumentParser();p.add_argument('--suite',choices=['smoke','full','edge','stress'],default='smoke');p.add_argument('--output');p.add_argument('--timeout',type=int,default=int(os.getenv('TIMEOUT','120')));p.add_argument('--n',type=int);p.add_argument('--gpus',default=os.getenv('GPUS','1,2,4'));p.add_argument('--policies',default=os.getenv('POLICIES','ot,ac,dup,grit'));a=p.parse_args()
 os.chdir(ROOT);stamp=datetime.datetime.now(datetime.timezone.utc).strftime('%Y%m%dT%H%M%S.%fZ');out=pathlib.Path(a.output or f'results/irregular/{stamp}-{a.suite}').resolve();out.mkdir(parents=True,exist_ok=False)
 manifest={'run_id':stamp,'suite':a.suite,'git_commit':subprocess.check_output(['git','rev-parse','HEAD'],text=True).strip(),'git_status':subprocess.check_output(['git','status','--short'],text=True),'akita_commit':subprocess.check_output(['git','-C','../akita','rev-parse','HEAD'],text=True).strip(),'binary_sha256':digest(ROOT/'bin/irregular'),'kernel_sha256':digest(ROOT/'benchmarks/irregular/kernels.hsaco'),'go_version':subprocess.check_output(['go','version'],text=True).strip(),'cases':[]}
 shutil.copy2(ROOT/'bin/irregular',out/'irregular')
 manifest['source_sha256']={str(x.relative_to(ROOT)):digest(x) for directory in ['benchmarks/irregular','samples/irregular','scripts/irregular'] for x in (ROOT/directory).rglob('*') if x.is_file() and x.suffix in ['.go','.cl','.hsaco','.py','.sh']}
 manifest['source_sha256'].update({str(x):digest(ROOT/x) for x in [pathlib.Path('samples/runner/report.go'),pathlib.Path('benchmarks/shoc/spmv/irregular.go')]})
 (out/'run_manifest.json').write_text(json.dumps(manifest,indent=2))
 cases=[]
 if a.suite in ['smoke','full']:
  n=a.n or (9 if a.suite=='smoke' else 1025);seeds=[17] if a.suite=='smoke' else [17,29,43];topos=['disconnected','skewed'] if a.suite=='smoke' else ['uniform','skewed']
  for w in ['bfs','spmv','pagerank','gather','sssp','cc']:
   for topo in topos:
    for seed in seeds:
     base=[f'-workload={w}',f'-n={n}',f'-generator={topo}',f'-seed={seed}']
     if w=='gather':base+=['-profile=phase-shift' if topo=='skewed' else '-profile=all-shared-read']
     cases.append((f'{w}-{topo}-s{seed}-emu',base+['-gpus=1'],False,1,'off'))
     for count in map(int,a.gpus.split(',')):
      ids=','.join(map(str,range(1,count+1)))
      for policy in (['off'] if count==1 else a.policies.split(',')):
       cases.append((f'{w}-{topo}-s{seed}-g{count}-{policy}',base+['-timing',f'-unified-gpus={ids}','-use-unified-memory',f'-grit-mode={policy}'],True,count,policy))
  # Each controlled profile also runs independently, not only within phase shift.
  for profile in ['private','producer-consumer','alternating-readwrite','skewed']:
   for policy in a.policies.split(','):
    cases.append((f'gather-{profile}-g2-{policy}',['-workload=gather',f'-profile={profile}','-n=65','-timing','-unified-gpus=1,2','-use-unified-memory',f'-grit-mode={policy}'],True,2,policy))
 elif a.suite=='stress':
  for policy in a.policies.split(','):
   cases.append((f'gather-shared-stress-{policy}',['-workload=gather',f'-n={a.n or 65537}','-profile=all-shared-read','-timing','-unified-gpus=1,2','-use-unified-memory',f'-grit-mode={policy}'],True,2,policy))
 else:
  for w in ['bfs','spmv','pagerank','sssp','cc']:
   for n in [1,3,65]:
    for topo in ['empty','path','cycle','star','grid','disconnected','low-locality']:
     cases.append((f'{w}-{topo}-n{n}',[f'-workload={w}',f'-n={n}',f'-generator={topo}','-gpus=1','-verify'],False,1,'off'))
  for n in [1023,1024,1025]:
   for profile in ['private','all-shared-read','phase-shift']:
    cases.append((f'gather-{profile}-n{n}',['-workload=gather',f'-n={n}',f'-profile={profile}','-timing','-unified-gpus=1,2','-use-unified-memory','-grit-mode=grit'],True,2,'grit'))
 failures=0
 for name,args,timing,count,policy in cases:
  d=out/name;d.mkdir();cmd=[str(out/'irregular'),*args,'-verify','-disable-rtm','-report-all',f'-metric-file-name={d}/metrics',f'-result-json={d}/result.json']
  if policy!='off':cmd += [f'-grit-trace={d}/transitions.csv']
  if policy=='off':
   cmd=[x for x in cmd if x!='-use-unified-memory'];cmd=[x.replace('-unified-gpus=','-gpus=') for x in cmd]
  entry={'case_id':name,'command':cmd,'timing_mode':timing,'gpus':count,'policy':policy,'status':'RUNNING'};(d/'case.json').write_text(json.dumps(entry,indent=2))
  try:
   with (d/'stdout.log').open('w') as log:r=subprocess.run(cmd,stdout=log,stderr=subprocess.STDOUT,timeout=a.timeout)
   entry['exit_code']=r.returncode
   if r.returncode:raise ValueError(f'exit {r.returncode}')
   result=json.loads((d/'result.json').read_text())
   if not result.get('verify_pass'):raise ValueError('no CPU verification')
   if timing:
    with (d/'metrics.csv').open() as f:metrics=list(csv.DictReader(f,skipinitialspace=True))
    activity={i:0 for i in range(1,count+1)}
    for m in metrics:
     if m['what']=='cu_inst_count':
      for i in activity:
       if m['where'].startswith(f'GPU[{i}].'):activity[i]+=int(float(m['value']))
    if not any(m['where']=='Driver' and m['what']=='kernel_time' and float(m['value'])>0 for m in metrics):raise ValueError('missing or zero kernel timing')
    if any(v<=0 for v in activity.values()):raise ValueError(f'inactive physical GPU: {activity}')
    entry['physical_gpu_instructions']=activity
    if policy!='off':
     report=json.loads((d/'metrics.grit.json').read_text())
     if report['active_requests'] or report['pending_operations']:raise ValueError('unfinished GRIT requests')
     # Shared-read gather must show actual same-page transitions on both GPUs.
     if result['workload']=='gather' and count>1:
      events=list(csv.DictReader((d/'transitions.csv').open()));pages={}
      for e in events:pages.setdefault(e['vpn'],set()).add(int(e['gpu']))
      entry['shared_pages_observed']=sum(len(gs)>1 for gs in pages.values())
      if result['profile'] in ['all-shared-read','phase-shift'] and not entry['shared_pages_observed']:raise ValueError('no actual cross-GPU shared-page evidence')
   entry['status']='PASS'
  except (ValueError,OSError,subprocess.TimeoutExpired) as e:entry['status']='FAIL';entry['notes']=str(e);failures+=1
  (d/'case.json').write_text(json.dumps(entry,indent=2));manifest['cases'].append(entry)
  (out/'run_manifest.json').write_text(json.dumps(manifest,indent=2));print(name,entry['status'],entry.get('notes',''),flush=True)
 subprocess.run([sys.executable,str(ROOT/'scripts/irregular/aggregate.py'),str(out)],check=True)
 audit=subprocess.run([sys.executable,str(ROOT/'scripts/irregular/audit.py'),str(out)])
 print(f'{len(cases)-failures}/{len(cases)} passed: {out}');return bool(failures or audit.returncode)
if __name__=='__main__':sys.exit(main())
