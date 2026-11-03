#!/usr/bin/env python3
"""Run verified, light matrix scenarios; retain commands and raw simulator data."""
import argparse
import concurrent.futures
import csv
import hashlib
import json
import pathlib
import subprocess
import time


def main():
    parser = argparse.ArgumentParser(description=__doc__)
    parser.add_argument('--binary', required=True)
    parser.add_argument('--out', required=True)
    parser.add_argument('--timeout', type=int, default=300)
    args = parser.parse_args()
    binary = pathlib.Path(args.binary).resolve()
    out = pathlib.Path(args.out).resolve()
    out.mkdir(parents=True, exist_ok=True)
    cases = []

    def add(name, mode='grit', x=32, y=32, z=4096, pages=179,
            gpus='1,2', dram=1, extra=()):
        cases.append(dict(name=name, mode=mode, x=x, y=y, z=z,
                          pages=pages, gpus=gpus, dram=dram, extra=list(extra)))

    add('wide-repeat')
    add('wide-ac', mode='ac')
    add('wide-dup', mode='dup')
    for mode in ('ot', 'grit'):
        add('intermediate-' + mode, mode=mode, pages=128)
        add('tall-' + mode, mode=mode, y=128, z=1024, pages=64)
        add('deeper-' + mode, mode=mode, x=64)
    add('single-gpu-pressure', pages=64, gpus='1')
    # Keep two physical GPUs for GRIT, but dispatch work only to GPU 2.
    add('single-active-gpu-pressure', pages=64, gpus='2')
    add('single-active-gpu-fit', pages=64, gpus='2', z=512)
    add('prediction-off-pressure', pages=64, extra=('-grit-prediction=false',))
    add('pa-cache-off', extra=('-grit-pa-cache=false',))
    add('tall-kernel-control-off', mode='off', y=64, z=64, dram=32)
    metadata = dict(binary=str(binary), sha256=hashlib.sha256(binary.read_bytes()).hexdigest(),
                    cases=cases,
                    timing='simulated kernel time; concurrent wall times are not compared')
    (out / 'metadata.json').write_text(json.dumps(metadata, indent=2) + '\n')

    def run(case):
        name = case['name']
        command = [str(binary), '-timing', '-unified-gpus=' + case['gpus'],
                   '-use-unified-memory', '-gpu-dram-size-mb=' + str(case['dram']),
                   '-verify', '-disable-rtm',
                   '-grit-mode=' + case['mode'], '-grit-resident-pages=' + str(case['pages']),
                   *['-' + k + '=' + str(case[k]) for k in ('x', 'y', 'z')],
                   *case['extra'], '-metric-file-name=' + str(out / name),
                   '-grit-trace=' + str(out / (name + '.trace.csv'))]
        (out / (name + '.command.json')).write_text(json.dumps(command, indent=2) + '\n')
        start = time.monotonic()
        with (out / (name + '.log')).open('w') as log:
            try:
                result = subprocess.run(command, stdout=log, stderr=subprocess.STDOUT,
                                        timeout=args.timeout)
                status = dict(exit_code=result.returncode)
            except subprocess.TimeoutExpired:
                status = dict(timeout_seconds=args.timeout)
        status['wall_seconds'] = time.monotonic() - start
        status['verified'] = (status.get('exit_code') == 0 and
                              'Passed!' in (out / (name + '.log')).read_text())
        (out / (name + '.status.json')).write_text(json.dumps(status, indent=2) + '\n')
        row = dict(name=name, mode=case['mode'], x=case['x'], y=case['y'], z=case['z'],
                   gpus=case['gpus'], resident_pages=case['pages'], **status)
        report = out / (name + '.grit.json')
        if report.exists():
            data = json.loads(report.read_text())
            row.update(kernel_us=data['kernel_time_ns'] / 1000,
                       faults=data['faults']['LocalPageFaults'],
                       decisions=data['faults']['FaultTriggers'],
                       copies=data['placement']['Copies'], evictions=data['evictions'],
                       swap_kib=data['swap_bytes'] / 1024,
                       remote_accesses=data['placement']['RemoteAccesses'],
                       ac_interrupts=data['ac_threshold_interrupts'],
                       active_requests=data['active_requests'],
                       pending_operations=data['pending_operations'])
            row['verified'] &= data['active_requests'] == 0 and data['pending_operations'] == 0
        else:
            row['verified'] = False
        print(json.dumps(row), flush=True)
        return row

    with concurrent.futures.ThreadPoolExecutor(max_workers=2) as executor:
        rows = list(executor.map(run, cases))
    fields = list(dict.fromkeys(key for row in rows for key in row))
    with (out / 'summary.csv').open('w', newline='') as f:
        writer = csv.DictWriter(f, fieldnames=fields)
        writer.writeheader()
        writer.writerows(rows)
    if not all(row['verified'] for row in rows):
        raise SystemExit('One or more scenarios failed; inspect logs and status files.')


if __name__ == '__main__':
    main()
