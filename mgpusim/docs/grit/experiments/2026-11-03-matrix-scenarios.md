# Matrix multiplication scenario sweep — 3 November 2026

The expanded sweep attempted 15 runs: 10 passed full CPU-reference verification, two failed verification, one configuration was rejected at startup, and two exceeded wall-clock limits. Every successfully verified run ended with zero active requests and pending GRIT operations. The failures mean this benchmark/platform combination cannot be described as working across all scenarios.

## Setup and reproducibility

Use the same corrected-verifier binary as the preceding matrix experiment: SHA-256 `735b156affe35c30eca64e6c4bad1e3fae4e73a4bc1cd79b5c35bb700434cec7`. This keeps comparisons with that experiment meaningful. Source provenance is preserved in the raw metadata. No simulator or kernel behavior was changed during this sweep.

Except for the explicit GRIT-disabled control, physical DRAM is 1 MiB per GPU. Managed resident limits are separate from physical DRAM. The serial simulator, timed memory copies, default PA fault threshold 4 and AC threshold 256, shared UVM, and full benchmark verification are used throughout. Two independent simulator processes may run concurrently; only simulated kernel time is compared, never host wall-clock performance.

Matrix notation below is A rows × inner dimension × B columns. Wide = 32×32×4096; deeper = 32×64×4096; tall = 128×32×1024. All three shapes have 131,072 output values and 128 workgroups under unified two-GPU scheduling. Deeper doubles the multiply-accumulate count and has a 1,544-KiB matrix footprint versus 1,028 KiB for wide. Tall has a 656-KiB footprint but exposes a kernel correctness bug.

The diagnostic runner is `scripts/grit/matrix_scenarios.py`. It intentionally retains failing and unsupported configurations and exits nonzero when any scenario fails. It now includes the three supplemental controls used to investigate the initial results. The main sweep used a 300-second per-case wall-clock limit; the supplemental single-active-GPU pressure run used 180 seconds. Its command and status preserve that distinction.

```sh
go build -o /tmp/matrixmultiplication-grit ./samples/matrixmultiplication
python3 scripts/grit/matrix_scenarios.py \
  --binary /tmp/matrixmultiplication-grit \
  --out /tmp/grit-matrix-scenarios --timeout 300
```

Rebuilding from the current source will have a different binary hash. The original commands, logs, statuses, metric files, transitions, binary/source metadata and combined summary CSV are retained in `/home/epo/hpcan/experiments/grit-matrix-scenarios/`. Matching earlier measurements are retained in `/home/epo/hpcan/experiments/grit-matrix-light/`.

## Correct, completed runs

All kernel times are simulated microseconds. Budgets are per GPU.

| Scenario | Mode | Budget KiB | Kernel µs | Faults | Copies | Evictions | Swap KiB |
|---|---|---:|---:|---:|---:|---:|---:|
| Wide, repeat | GRIT | 716 | 395.930 | 260 | 1 | 0 | 312 |
| Wide | fixed AC | 716 | 4,592.171 | 262 | 0 | 46 | 604 |
| Wide | fixed DUP | 716 | 397.598 | 257 | 65 | 23 | 412 |
| Wide, intermediate pressure | OT | 512 | 1,860.664 | 1,136 | 0 | 63 | 1,012 |
| Wide, intermediate pressure | GRIT | 512 | 389.888 | 260 | 1 | 1 | 520 |
| Deeper | OT | 716 | 2,544.656 | 1,461 | 0 | 138 | 1,812 |
| Deeper | GRIT | 716 | 578.973 | 392 | 2 | 28 | 940 |
| Wide, prediction disabled | GRIT | 256 | 1,862.487 | 788 | 1 | 657 | 5,512 |
| Wide, PA cache disabled | GRIT | 716 | 393.644 | 260 | 1 | 0 | 312 |
| Single active GPU, 32×32×512 | GRIT | 256 | 77.780 | 33 | 0 | 0 | 0 |

The single-active-GPU control configures two physical GPUs but dispatches work only to GPU 2 using `-unified-gpus=2`. Its 132-KiB footprint fits the 256-KiB resident budget. A platform containing only one physical GPU is rejected, as recorded below.

## Analysis

The wide GRIT repeat reproduces the earlier 395.930-µs time and 260 faults exactly, providing a consistency check. At the matched 512-KiB budget, GRIT is **4.772× faster than OT**, with 77.11% fewer faults and 63 versus one eviction. Like the earlier 716-KiB case, selective duplication of the shared A page avoids ownership ping-pong.

Doubling the inner dimension preserves the benefit: GRIT is **4.395× faster than OT**. There are now two shared A pages and two copies. This input also increases footprint and memory pressure, so the timing increase combines additional arithmetic, input traffic and capacity effects.

Fixed DUP is close in time to GRIT on wide, but creates 65 replicas rather than one and incurs 23 evictions. GRIT therefore achieves comparable timing with less replication on this input. Fixed AC is much slower, and records **6,400 remote cache-line accesses and 24 threshold interrupts**. Unlike the earlier adaptive runs, this directly exercises the remote-access counter trigger. It is not an exhaustive validation of every counter/coherence case.

Compared with the earlier matched 256-KiB GRIT run (1,769.639 µs), disabling prediction raises kernel time by **5.25%**. Nevertheless, faults decrease from 828 to 788, evictions from 697 to 657, and swap from 5,832 to 5,512 KiB. Placement and scheduling affect the critical path; fewer faults alone do not guarantee a shorter run. This single input is not enough to generalize prediction's benefit.

Disabling the PA cache produces a 0.58% lower kernel time even though PA stall cycles increase from 45,260 to 52,000. The baseline has only three PA hits out of 260 lookups. Timing changes affect subsequent event ordering, so this case does not establish a PA-cache performance benefit or justify attributing whole-kernel time directly to PA stall cycles.

Small timing differences across budgets should likewise not be treated as monotonic memory-capacity effects. Fair policy comparisons use matched shapes, hardware, budgets and flags. The current whole-cache invalidations and serialized placement amplify migration costs; these speedups are simulator observations, not reproductions of the paper's results.

## Failures, unsupported cases and time limits

| Scenario | Outcome | Evidence |
|---|---|---|
| Tall, OT, 256 KiB/GPU | Verification failed | First mismatch `[0,32]`: expected 6.696725, got 9.100811 |
| Small tall control, GRIT off, 32 MiB/GPU | Verification failed | 64×32×64 input; first mismatch `[0,32]`: expected 7.595318, got 9.947590 |
| Tall, GRIT, 256 KiB/GPU | Exceeded 300 seconds wall time | No complete metrics or correctness result; transition trace continued advancing |
| One physical GPU, GRIT | Rejected at startup | Platform requires 4-KiB pages, at least two GPUs, serial engine and timed copies |
| Wide, only GPU 2 active, 256 KiB | Exceeded 180 seconds wall time | No complete metrics or correctness result; transition trace continued advancing |

The tall failures have a concrete kernel-level explanation independent of GRIT: in `MatrixMultiplication_Kernels.cl`, `globalPosA` uses `lIdY` to select A rows but omits the row workgroup offset. Workgroups after the first row tile consequently reuse the first 32 rows of A while writing later C rows. The GRIT-disabled control fails at exactly the row-32 boundary as well. The expanded full-output verifier detects this issue; the old verifier would have missed it. These failed results are excluded from speedup comparisons. The kernel source and embedded HSACO have not been repaired in this experiment.

The two timed-out traces show repeated placements and increasing generations, consistent with severe thrashing. They do not prove a deadlock or successful eventual completion. In particular, the one-active-GPU pressure case cannot be called verified just because the smaller fitting control passes. The tall timeout is additionally confounded by the already demonstrated kernel bug.

No completed run exercised DUP write collapse: output partitions do not deliberately share writable pages. Repeated kernels, deliberately shared writes, other benchmarks and comprehensive coherence behavior remain separate validation work. No pre-existing driver tests were run as part of this sweep; the matrix verifier regression test passed.
