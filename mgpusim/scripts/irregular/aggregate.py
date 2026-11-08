#!/usr/bin/env python3
"""Summarize real raw results, retaining failed cases and unavailable metrics."""
import csv,json,pathlib,sys

def aggregate(root):
 root=pathlib.Path(root);manifest=json.loads((root/'run_manifest.json').read_text());rows=[]
 for e in manifest['cases']:
  d=root/e['case_id'];r=json.loads((d/'result.json').read_text()) if (d/'result.json').exists() else {};g=json.loads((d/'metrics.grit.json').read_text()) if (d/'metrics.grit.json').exists() else {};placement=g.get('placement',{});fault=g.get('faults',{})
  kernel='';rdma='';total=''
  if (d/'metrics.csv').exists():
   ms=list(csv.DictReader((d/'metrics.csv').open(),skipinitialspace=True))
   for m in ms:
    if m['where']=='Driver' and m['what']=='kernel_time':kernel=float(m['value'])*1e9
    if m['where']=='Driver' and m['what']=='total_time':total=float(m['value'])*1e9
   rdma=sum(float(m['value']) for m in ms if m['what']=='outgoing_trans_count') if any(m['what']=='outgoing_trans_count' for m in ms) else ''
  rows.append(dict(run_id=manifest['run_id'],case_id=e['case_id'],git_commit=manifest['git_commit'],workload=r.get('workload',''),profile=r.get('profile',''),dataset=next((x.split('=',1)[1] for x in e['command'] if x.startswith('-generator=') or x.startswith('-input=')),''),sha256=r.get('sha256',''),seed=r.get('seed',''),topology=next((x.split('=',1)[1] for x in e['command'] if x.startswith('-generator=')),''),n=r.get('n',''),m=r.get('m',''),gpus=e['gpus'],gpu_ids=','.join(map(str,range(1,e['gpus']+1))),unified_gpu=any(x.startswith('-unified-gpus=') for x in e['command']),schedule=r.get('schedule',''),policy=e['policy'],page_size_bytes=4096,dram_bytes_per_gpu=33554432,footprint_bytes=r.get('footprint_bytes',''),iterations=r.get('iterations',''),timing_mode=e['timing_mode'],sim_time_ns=total,kernel_time_ns=kernel,verify_pass=r.get('verify_pass',False),max_abs_error=r.get('max_abs_error',''),max_rel_error=r.get('max_rel_error',''),rdma_requests=rdma,migrations=placement.get('Migrations',''),duplicates=placement.get('Copies',''),write_collapses=placement.get('Collapses',''),page_faults=(fault.get('LocalPageFaults',0)+fault.get('ProtectionFaults',0)) if fault else '',local_page_faults=fault.get('LocalPageFaults',''),protection_faults=fault.get('ProtectionFaults',''),grit_scheme_switches='',physical_gpu_instructions=json.dumps(e.get('physical_gpu_instructions',{})),shared_pages_observed=e.get('shared_pages_observed',''),status=e['status'],notes=e.get('notes','')))
 if rows:
  with (root/'summary.csv').open('w') as f:w=csv.DictWriter(f,fieldnames=list(rows[0]));w.writeheader();w.writerows(rows)
 # Compare only verified pairs with identical input, GPU count and launch mode.
 pairs=[]
 for r in rows:
  if r['policy']!='grit' or r['status']!='PASS' or not r['kernel_time_ns']:continue
  for b in rows:
   if b['policy'] in ['ot','ac','dup'] and b['status']=='PASS' and all(r[k]==b[k] for k in ['workload','dataset','sha256','seed','gpus','iterations','schedule']) and r['case_id'].rsplit('-',1)[0]==b['case_id'].rsplit('-',1)[0]:
    speed=b['kernel_time_ns']/r['kernel_time_ns'];pairs.append(dict(case_id=r['case_id'],baseline=b['policy'],speedup=speed,runtime_reduction_percent=100*(1-1/speed)))
 if pairs:
  with (root/'comparisons.csv').open('w') as f:w=csv.DictWriter(f,fieldnames=list(pairs[0]));w.writeheader();w.writerows(pairs)
 return rows
if __name__=='__main__':aggregate(sys.argv[1])
