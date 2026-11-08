#!/usr/bin/env python3
"""Recheck physical work, memory traffic, policy-identical inputs and outputs."""
import csv,json,pathlib,sys,math

def audit(root):
 root=pathlib.Path(root);manifest=json.loads((root/'run_manifest.json').read_text());groups={};reports=[];errors=[]
 for case in manifest['cases']:
  if case['status']!='PASS':errors.append(case['case_id']+': '+case['status']);continue
  d=root/case['case_id'];result=json.loads((d/'result.json').read_text())
  if not case['timing_mode']:continue
  counts={i:dict(instructions=0,dram_transactions=0) for i in range(1,case['gpus']+1)}
  metrics=list(csv.DictReader((d/'metrics.csv').open(),skipinitialspace=True))
  if not any(m['where']=='Driver' and m['what']=='kernel_time' and float(m['value'])>0 for m in metrics):errors.append(case['case_id']+': missing or zero kernel timing')
  for m in metrics:
   for i,v in counts.items():
    if m['where'].startswith(f'GPU[{i}].'):
     if m['what']=='cu_inst_count':v['instructions']+=int(float(m['value']))
     if '.DRAM[' in m['where'] and m['what'] in ['read_trans_count','write_trans_count']:v['dram_transactions']+=int(float(m['value']))
  if any(not c['instructions'] or not c['dram_transactions'] for c in counts.values()):errors.append(case['case_id']+': inactive work or memory '+str(counts))
  phases=result.get('phases') or []
  if result['workload']=='gather':
   for phase in phases:
    if phase['Name']=='private':
     sets=[set(v) for v in phase['InputPages'].values()]
     if any(a&b for i,a in enumerate(sets) for b in sets[i+1:]):errors.append(case['case_id']+': private input page overlap')
  if result['workload']=='gather' and result['profile'] in ['all-shared-read','phase-shift'] and case['gpus']>1 and case['policy']!='off':
   intended=set()
   for phase in phases:
    if phase['Name']=='all-shared-read':
     sets=[set(v) for v in phase['InputPages'].values()]
     if sets:intended.update(set.intersection(*sets))
   seen={}
   for event in csv.DictReader((d/'transitions.csv').open()):
    vpn=int(event['vpn']);requester=int(event['gpu']);owner=int(event['owner'])
    seen.setdefault(vpn,set()).add(requester)
    # Local hits need no placement event. A peer request to a live page owned
    # by another active GPU is direct evidence of cross-GPU page sharing.
    if requester!=owner and owner in counts and requester in counts:seen[vpn].add(owner)
   observed=[p for p in intended if len(seen.get(p,set()))>1]
   if not observed:errors.append(case['case_id']+': no actual shared INPUT page transitions')
  key=case['case_id'].rsplit('-',1)[0] if case['case_id'].endswith('-'+case['policy']) else case['case_id'];other=groups.setdefault(key,result)
  if other['sha256']!=result['sha256']:errors.append(key+': policy input mismatch')
  if other.get('output_uint')!=result.get('output_uint'):errors.append(key+': integer output mismatch')
  a,b=other.get('output_float') or [],result.get('output_float') or []
  if len(a)!=len(b) or any(not math.isfinite(x) or abs(x-y)>1e-5+1e-4*abs(y) for x,y in zip(a,b)):errors.append(key+': float output mismatch')
  reports.append(dict(case_id=case['case_id'],physical_gpus=counts))
 result=dict(pass_=not errors,checked_timing_cases=len(reports),errors=errors,cases=reports)
 (root/'audit.json').write_text(json.dumps(result,indent=2));print(f'{root}: {len(reports)} timing cases audited, {len(errors)} errors')
 for e in errors:print(e)
 return bool(errors)
if __name__=='__main__':sys.exit(audit(sys.argv[1]))
