#!/usr/bin/env python3
"""Standalone Matplotlib figures from verified summary.csv, never estimates."""
import csv
import pathlib
import sys
import matplotlib
matplotlib.use('Agg')
import matplotlib.pyplot as plt
root=pathlib.Path(sys.argv[1])
rows=[r for r in csv.DictReader((root/'summary.csv').open())
      if r['status']=='PASS' and r['timing_mode']=='True' and r['kernel_time_ns']]
colors={'ot':'#8c6d31','ac':'#3182bd','dup':'#756bb1','grit':'#31a354','off':'#636363'}
fig,ax=plt.subplots(figsize=(12,max(4,len(rows)*.20)))
ax.barh([r['case_id'] for r in rows],[float(r['kernel_time_ns']) for r in rows],
        color=[colors[r['policy']] for r in rows])
ax.set_xlabel('Simulated kernel busy time (ns)');ax.invert_yaxis()
ax.set_title('Verified timing cases — raw data: summary.csv')
fig.tight_layout();fig.savefig(root/'kernel-time.svg');plt.close(fig)
if (root/'comparisons.csv').exists():
 pairs=list(csv.DictReader((root/'comparisons.csv').open()))
 fig,ax=plt.subplots(figsize=(12,max(4,len(pairs)*.20)))
 ax.barh([p['case_id']+' / '+p['baseline'] for p in pairs],[float(p['speedup']) for p in pairs])
 ax.axvline(1,color='black',linestyle='--');ax.set_xlabel('Baseline kernel time / GRIT kernel time');ax.invert_yaxis()
 fig.tight_layout();fig.savefig(root/'speedup.svg');plt.close(fig)
print(root/'kernel-time.svg')
