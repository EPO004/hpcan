# Light matrix-multiplication experiment — 3 November 2026

All five measured runs passed the corrected CPU-reference verification over all 131,072 output elements. Every report ended with zero active requests and zero pending GRIT operations.

## Configuration

- A: 32 × 32; B: 32 × 4096; C: 32 × 4096, float32.
- 4,194,304 multiply-accumulates, approximately 8.39 million floating-point operations.
- UVM footprint: 1,052,672 bytes = 1,028 KiB = 257 base pages.
- Two GPUs, 64 CUs/GPU, serial event engine, shared UVM.
- Physical DRAM reduced from the default 32 MiB/GPU to 1 MiB/GPU (32-fold).
- Default GRIT thresholds: four faults and 256 remote cache-line accesses.
- 70% budget: floor(1,052,672 × 0.70 / 4096) = 179 pages = 716 KiB/GPU.
- Severe budget: 64 pages = 256 KiB/GPU. All OT/GRIT pairs use identical hardware and budgets.

The wide, shallow shape is deliberate: it supplies 128 workgroups, so the existing scheduler assigns 64 to each GPU. A preliminary 64/32/64 input had only two workgroups and ran almost entirely on GPU 1. The measured rectangular input avoids that misleading multi-GPU test. Both GPUs have substantial measured kernel activity.

## Results

Times below are simulated kernel time, not host wall-clock duration.

| Case | Mode | Kernel µs | Local faults | Decisions | Migrations | Copies | Evictions | Swap KiB |
|---|---|---:|---:|---:|---:|---:|---:|---:|
| control-grit-32mb | grit | 412.334 | 260 | 1 | 131 | 1 | 0 | 0 |
| lowdram-ot-70pct | ot | 1747.024 | 1056 | 0 | 966 | 0 | 0 | 312 |
| balanced-70pct | grit | 395.930 | 260 | 1 | 169 | 1 | 0 | 312 |
| pressure-ot-64pages | ot | 4816.540 | 3077 | 0 | 3044 | 0 | 2459 | 19928 |
| pressure-grit-64pages | grit | 1769.639 | 828 | 112 | 794 | 1 | 697 | 5832 |

Migrations include completed page moves/restores; swap counts actual host-backing bytes in both directions. Initially allocated host-backed pages can produce swap-in traffic without an eviction event.

## Interpretation

At 716 KiB/GPU, GRIT is **4.412× faster than uniform OT** in this simulator run: 395.930 versus 1,747.024 µs. Local faults decrease from 1,056 to 260 (75.38% reduction), and migrations decrease from 966 to 169.

The trace identifies the actual adaptive event at simulated time 0.000153075 s: PID 1, VPN 1 changes from scheme 1 (OT) to scheme 3 (DUP), following the fourth read fault. This is matrix A, a 4-KiB read-shared page. GRIT creates one independent 4-KiB copy. A can then remain readable on both GPUs, avoiding repeated ownership transfers. No predictor promotion is needed in this case.

The 70% case has no runtime evictions: after distributing private B/C portions across the GPUs, the working set fits. Its 312 KiB of swap traffic comes from restoring pages that began in host backing. Therefore that case proves adaptive read duplication and initial host backing, not sustained eviction behavior.

At 256 KiB/GPU, **real oversubscription** occurs. GRIT still passes complete verification with 697 evictions and 5,832 KiB (5.695 MiB) of swap traffic. Uniform OT incurs 2,459 evictions and 19,928 KiB (19.461 MiB) of swap traffic. GRIT is **2.722× faster than OT**, and local faults decrease from 3,077 to 828 (73.09%).

The tight budget increases GRIT kernel time to 1,769.639 µs, about 4.47× its 716-KiB result. GRIT reduces memory-management overhead; it does not eliminate the cost of insufficient memory. The trace/report show 112 threshold decisions, 35 OT→AC and 39 OT→DUP transitions, and two eight-page predictor promotions. Some decisions follow capacity faults on private pages, highlighting the fault-based classifier's sensitivity to oversubscription.

There are zero protection faults, zero write collapses and zero AC-threshold interrupts in these runs. The output partitions do not share writable pages, and the AC pages generate no counted remote transactions. These experiments therefore demonstrate adaptive duplication, policy/prediction activity and backing/eviction, but do not independently validate the 256-access AC migration trigger or DUP write-collapse path.

The 32-MiB control passes at 412.334 µs with no swap. Its small timing difference versus the 1-MiB/716-KiB case should not be interpreted as a benefit of reducing DRAM: initial residency, transfer sources and modeled DRAM geometry differ. Policy speedups above compare OT with GRIT under the same low-memory configuration.

These are single-input simulator results, not the paper's reported speedups. Whole-cache flushing and serialized placement/coherence barriers amplify migration costs; see RUNNING.md for model assumptions. Host wall-clock durations are stored separately, and some cases ran concurrently, so they were not used for performance ratios.

## Reproduce

From `mgpusim/`:

```sh
go build -o /tmp/matrixmultiplication-grit ./samples/matrixmultiplication
/tmp/matrixmultiplication-grit -timing -unified-gpus=1,2 \
  -use-unified-memory -gpu-dram-size-mb=1 \
  -grit-mode=grit -x=32 -y=32 -z=4096 -verify -disable-rtm \
  -grit-footprint-bytes=1052672 -metric-file-name=/tmp/mm-grit-70pct
```

For severe pressure, replace `-grit-footprint-bytes=1052672` with `-grit-resident-pages=64`. Replace `-grit-mode=grit` with `-grit-mode=ot` for a matched baseline.

Raw logs, commands, reports, transitions, wall-clock statuses, binary hash/build metadata and summary CSV are retained at:

`/home/epo/hpcan/experiments/grit-matrix-light/`

The benchmark verifier previously checked only its first output row because the inner loop advanced the wrong variable. That was corrected and regression-tested before accepting these results, in commit `6c7befc`. The original user driver edits were preserved.
