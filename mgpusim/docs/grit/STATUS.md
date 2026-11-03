# GRIT implementation status

The `grit-hpca24` branches now run OT, AC, DUP and dynamic GRIT through the live
GPU memory hierarchy. See [RUNNING.md](RUNNING.md) for commands, configuration,
validation and explicit timing/model assumptions.

## Connected execution paths

- Allocation/PID lifecycle, host canonical PTEs and per-GPU residency/permissions.
- Access admission before vector/scalar cache lookup, request leases until actual
  completion, generation validation on TLB hits and managed MMU translation.
- Real local/protection fault admission, fourth-fault epochs, timed PA cache/table
  service, prediction and next-fault adoption.
- Global quiescence, correlated cache-flush ACKs, real full-page PMC transfers,
  publication after transfer completion, safe frame retirement and request replay.
- Actual RDMA transaction counting, regional AC thresholds, separate threshold
  migrations and coherent remote read/write observations.
- DUP independent frames, write protection/collapse and transitions between modes.
- Allocation oversubscription, deterministic victim selection, host backing,
  real DRAM swap requests, resident budgets, PID-safe whole-buffer free and reuse.
- Separate configured NVLink/PCIe fabric, bounded walkers/walk cache, TLB latency,
  runtime flags, CSV/JSON metrics, traces, microbenchmark and comparison tooling.

## Verification

- `python3 scripts/grit/validate.py --race`: passes with only the user's three
  specified driver specs excluded.
- Compiled GPU-kernel tests pass in all four modes on three GPUs, checking actual
  values after read sharing, writes, migrations and collapse.
- One-frame-per-GPU oversubscription, host overwrite/revocation and same-VPN
  different-PID free tests pass.
- Four-GPU FIR (`length=4096`, timing/UVM, verification) passes.
- Standalone shared-page microbenchmark and all four policy/three extra ablation
  comparisons complete with verified data and no unfinished GRIT operations.

Testing exposed a driver wakeup race: an enqueue arriving between an empty
`Engine.Run` and clearing `engineRunning` could strand commands. A regression test
and a narrowly staged fix are committed. The pre-existing migration modifications
in `driver/driver.go` remain uncommitted and preserved; only the wakeup fix was
staged in that file.

## Reproduction boundaries

Whole-cache flushing, serialized placement/coherence barriers, ideal CPU backing
and PA service, scalar/instruction hierarchy sharing, and v3 work-group partitioning
remain explicit modeling assumptions. The eight exact paper workload/input variants
and reported speedups have not been reproduced. Do not treat microbenchmark speedups
as paper results. These limitations are described in detail in RUNNING.md.

MGPUSim retains the `configurable-dram-mb` history, original local Akita module
replacement and user's existing driver changes. Historical starting revisions and
that original driver patch remain in `BASELINE.txt` and `preexisting-driver.patch`.
