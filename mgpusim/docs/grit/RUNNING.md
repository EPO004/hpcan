# Running GRIT

Use the matching `grit-hpca24` branches in both `mgpusim/` and `akita/`.
MGPUSim's existing `go.mod` replacement resolves Akita to `../akita`; no workspace
file is required. The `configurable-dram-mb` changes remain included.

## Build and check a shared-page workload

From `mgpusim/`:

```sh
go build -o /tmp/gritmicro ./samples/gritmicro
/tmp/gritmicro -timing -unified-gpus=1,2,3 -use-unified-memory \
  -grit-mode=grit -verify -disable-rtm \
  -metric-file-name=/tmp/gritmicro -grit-trace=/tmp/gritmicro.trace.csv
```

The microbenchmark alternates compiled GPU load/store kernels across the physical
GPUs, writes into a previously read-shared page, and verifies host-visible values.
Use consecutive physical GPU IDs beginning at 1 for this microbenchmark. Ordinary
benchmarks continue using the runner's GPU selection and unified-device scheduler.

Files:

- `/tmp/gritmicro.csv`: existing runner metrics plus GRIT counters.
- `/tmp/gritmicro.grit.json`: configuration, kernel cycles/time, admitted faults,
  completed migrations/copies/collapses, traffic, PA cache, predictor, eviction,
  peak-frame, AC-interrupt and L2-TLB-miss metrics.
- `/tmp/gritmicro.trace.csv`: transition events and mapping generations.

A successful completed report has `active_requests=0` and
`pending_operations=0`. `-max-inst` can stop an experiment early; do not use such
an incomplete report for performance comparisons.

A normal benchmark also works:

```sh
go build -o /tmp/fir-grit ./samples/fir
/tmp/fir-grit -timing -unified-gpus=1,2,3,4 -use-unified-memory \
  -grit-mode=grit -length=4096 -verify -disable-rtm \
  -metric-file-name=/tmp/fir-grit
```

FIR mostly accesses private data. Use `gritmicro` to check that duplication,
collapse and dynamic policy decisions actually occur.

## Uniform policies, comparisons and ablations

`-grit-mode=ot|ac|dup|grit` selects uniform OT, uniform AC, uniform duplication,
or adaptive GRIT. All four modes use the same GRIT platform hardware settings.
`-grit-mode=off` retains the existing platform and memory behavior.

```sh
python3 scripts/grit/compare.py /tmp/gritmicro \
  --gpus=1,2,3 --output=/tmp/grit-comparison --ablations
```

The output directory must be new. The script saves commands, logs, commit IDs,
working-tree status and reports. `comparison.csv` uses kernel time and computes
`speedup = OT_time / policy_time`, with improvement `(speedup - 1) * 100`.
Pass benchmark-specific flags after `--`, for example:

```sh
python3 scripts/grit/compare.py /tmp/fir-grit \
  --gpus=1,2,3,4 --output=/tmp/fir-comparison -- --length=4096
```

`--ablations` adds PA-only, PA+cache and PA+prediction runs. PA access costs remain
modeled when its cache is disabled. Threshold sensitivity is available through
`-grit-fault-threshold=2|4|8|16`; AC defaults to 256 transactions per 64-KB region.
For a short AC-threshold diagnostic, pass `-grit-ac-threshold=2` directly to the
microbenchmark. Restore 256 for experiments using the paper's default.

## Capacity and oversubscription

Physical memory still uses `-gpu-dram-size-mb`, including its existing power-of-two
constraint. To enforce a smaller managed resident budget independently:

```sh
/tmp/gritmicro -timing -unified-gpus=1,2 -use-unified-memory \
  -grit-mode=dup -verify -disable-rtm -grit-resident-pages=1 \
  -metric-file-name=/tmp/grit-oversubscribed
```

Alternatively, give the application footprint in bytes. The default ratio is 0.70:

```text
-grit-footprint-bytes=APPLICATION_BYTES -grit-capacity-ratio=0.70
```

The per-GPU frame budget is `floor(footprint * ratio / 4096)`. Use either the
footprint option or an explicit page budget. Without either, actual physical
capacity is the limit; the simulator cannot infer the paper's application
footprint automatically. Leave enough physical memory for non-UVM code, kernel
arguments and other ordinary allocations. UVM allocations that cannot initially
fit start in host backing and fault into GPU memory on access.

Eviction retains an authoritative image: independent read replicas can be dropped;
a resident authoritative replica can become owner; otherwise the page is copied
to host backing. GPU reads/writes for swap use actual DRAM requests and ACKs.
Host transfer time is charged at the configured host bandwidth before publication.
Freed dirty buffers are flushed before frames can be reused. Free is PID-specific
and releases all pages and replicas in the buffer.

## Timing and hardware assumptions

Implemented defaults include 64 CUs/GPU at 1 GHz, 16/16/32-KB vector/scalar/instruction
L1 caches, 256-KB aggregate L2 with 16 ways, 32-entry L1 TLBs, 512-entry/16-way
L2 TLBs with pipelined 10-cycle lookup, and eight walkers per GPU with a 64-request
queue, four levels at 100 cycles per uncached level, and a 128-entry LRU cache of
intermediate walk entries. The four-level assumption is configurable in the Akita
MMU builder; leaf PTE reads always observe current mappings.

Peer transfers use the existing Akita NVLink connector. The full-mesh topology
splits the nominal aggregate peer bandwidth among `N-1` links. Host traffic uses
a distinct PCIe path. Nominal bandwidths are configurable:

```text
-grit-peer-bandwidth-gbps=300 -grit-host-bandwidth-gbps=32
-grit-pa-cache-cycles=1 -grit-pa-memory-cycles=100
```

These are configured rates, not a claim of measured end-to-end link throughput.
The existing connector's routers, headers and buffering affect achieved bandwidth.

The following assumptions materially affect timing and must accompany results:

- Placement transitions serialize globally and quiesce accepted memory requests.
  Cache flushes cover all data caches rather than only one page. This is
  conservative and may overestimate coherence/transition costs.
- AC aliases are writable mappings to one authoritative physical image. Conflicting
  accesses are ordered; writes and cross-GPU observations use conservative cache
  coherence barriers. Such barriers do not increment the PA fault counter.
- At an AC threshold, the triggering hot 4-KB page migrates; the destination's
  64-KB regional counter resets. This target-selection rule is an assumption.
- PA backing accesses use fixed latency and logical read/write accounting, without
  a contending CPU DRAM component. The fault cannot retire before PA service finishes.
- Host backing is ideal storage after modeled transfer delay and real GPU DRAM
  requests. Host backing addresses are reused; host DRAM/cache contention is absent.
- Scalar/instruction memory hierarchy and work-group partitioning retain the v3
  shader-array/scheduler structure. Scalar/instruction TLBs remain shared within a
  shader array; the paper's fully CU-private placement is not reproduced for them.
- CPU overwrites, explicit remaps and frees should occur between kernels. The
  parallel event engine is rejected for GRIT. Current validation targets a serial
  event engine, with actual parallel GPU memory traffic between transitions.
- Missing host PTEs prevent predictor promotion across holes. Majority votes count
  at least five matching pages/groups; minority subranges need not already be groups.

This is a runnable implementation of the policies and dynamic placement mechanism.
It is not a calibrated reproduction of the paper's reported performance numbers.
The authors' exact inputs/application versions, scheduler, topology and additional
hardware calibration are needed before making quantitative paper-reproduction claims.

## Validation

```sh
python3 scripts/grit/validate.py --race
```

This runs both modules' tests, vets the changed paths, and runs managed-memory
race tests. Only the three driver specs explicitly excluded by the user are skipped;
the other driver tests and a lost-wakeup regression test still run.
