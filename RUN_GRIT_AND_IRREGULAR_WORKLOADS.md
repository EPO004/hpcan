# Running GRIT and the irregular workloads

This workspace contains `mgpusim/` and its local `akita/` dependency. Run the
commands below in Bash. These are simulated GPUs; physical GPU hardware and ROCm
are not required. Normal builds use the included compiled GCN3 kernels.

## 1. Build

From this directory (the workspace root):

```bash
cd mgpusim
mkdir -p bin
go build -o bin/gritmicro ./samples/gritmicro
go build -o bin/irregular ./samples/irregular
./bin/irregular -h
```

All remaining commands assume you are in `mgpusim/`. Keep the existing local
Akita replacement in `go.mod`; no dependency upgrade is needed.

Alternatively, build the irregular executable and run its package tests:

```bash
bash scripts/irregular/build.sh
```

## 2. Check GRIT on four GPUs

Create a fresh output directory and run the shared-page microbenchmark:

```bash
mkdir -p results/manual
OUT=$(mktemp -d "results/manual/$(date -u +%Y%m%dT%H%M%SZ)-XXXXXX")

./bin/gritmicro \
  -timing -unified-gpus=1,2,3,4 -use-unified-memory \
  -grit-mode=grit -verify -report-all -disable-rtm \
  -metric-file-name="$OUT/gritmicro" \
  -grit-trace="$OUT/gritmicro-transitions.csv"
```

Successful output includes `GRIT microbenchmark verification: Passed!`.
`$OUT` is reused by the examples below, with different case names.

| Policy flag | Behavior |
|---|---|
| `-grit-mode=grit` | Adaptive GRIT page placement |
| `-grit-mode=ot` | Fixed on-touch migration |
| `-grit-mode=ac` | Fixed access-counter policy |
| `-grit-mode=dup` | Fixed read duplication with write collapse |
| `-grit-mode=off` | GRIT disabled; see the separate baseline command below |

Enabled GRIT requires `-timing`, `-use-unified-memory`, and
`-unified-gpus=1,2` or more GPUs. Use consecutive IDs starting at 1. The GRIT
coordinator uses the serial simulator engine; omit `-parallel` and
`-magic-memory-copy`.

## 3. Run every irregular workload on four physical GPUs

Define this helper once in the same Bash session. It uses four physical GPU
shards, verifies the result, saves the command and log, and records CSV/JSON
metrics and the GRIT transition trace. A nonzero simulator exit remains a failure.

```bash
set -euo pipefail

COMMON=(
  -timing -unified-gpus=1,2,3,4 -use-unified-memory
  -schedule=physical -verify -report-all -disable-rtm -seed=17
)

run_irregular() {
  local name="$1"
  shift
  local cmd=(./bin/irregular "${COMMON[@]}"
    "-grit-mode=${POLICY:-grit}" "$@"
    "-metric-file-name=$OUT/$name"
    "-result-json=$OUT/$name-result.json"
    "-grit-trace=$OUT/$name-transitions.csv")
  printf '%q ' "${cmd[@]}" > "$OUT/$name-command.txt"
  printf '\n' >> "$OUT/$name-command.txt"
  "${cmd[@]}" 2>&1 | tee "$OUT/$name.log"
}
```

Run all six workloads:

```bash
run_irregular bfs      -workload=bfs      -generator=skewed -n=64 -source=0
run_irregular spmv     -workload=spmv     -generator=skewed -n=64
run_irregular pagerank -workload=pagerank -generator=skewed -n=64 -iterations=10 -damping=0.85
run_irregular gather   -workload=gather   -n=64 -profile=phase-shift
run_irregular sssp     -workload=sssp     -generator=skewed -n=64 -source=0
run_irregular cc       -workload=cc       -generator=skewed -n=64
```

| Workload | GPU calculation | Important options |
|---|---|---|
| `bfs` | Shortest-hop levels from a source | `-source`, `-generator`, `-input` |
| `spmv` | CSR sparse matrix-vector multiplication | `-input` supports rectangular Matrix Market matrices |
| `pagerank` | Synchronous pull PageRank, including dangling vertices | `-iterations` defaults to 10; `-damping` defaults to 0.85 |
| `gather` | Indirect reads and synchronized producer/consumer phases | `-profile`; `-n` is the number of accesses |
| `sssp` | Positive-integer weighted shortest paths | `-source`; positive integer weights only |
| `cc` | Weak connected components with minimum-ID labels | Directed input is symmetrized |

For graph workloads, `-n` sets the vertex/row count. BFS/SSSP/CC default to up to
`n` synchronous passes; an insufficient cap can fail verification. BFS unreachable
levels and SSSP unreachable distances use uint32 `INF=0x3fffffff` in JSON.

Available generators: `path`, `cycle`, `star`, `grid`, `disconnected`, `empty`,
`uniform`, `skewed`, `powerlaw`, `high-locality`, `low-locality`. Use the same
input, seed, size, GPU count and schedule across policy comparisons. Start small:
PageRank's current dangling-mass scan is O(n²) per iteration, and detailed timing
simulation can become expensive.

## Run one workload independently

Each block below is independent: run only the block for the workload you want.
Run it from any directory inside this Git checkout. It locates the repository
root, builds the executable, creates a fresh result
directory, and runs on four physical simulated GPUs with GRIT and CPU verification.
No `COMMON` array, helper function, or earlier `OUT` variable is required.
The `git rev-parse` command makes these examples independent of the clone path.

### BFS only

```bash
cd "$(git rev-parse --show-toplevel)/mgpusim"
mkdir -p bin results/irregular
go build -o bin/irregular ./samples/irregular
OUT=$(mktemp -d "results/irregular/bfs-4gpu-XXXXXX")

./bin/irregular \
  -workload=bfs -generator=skewed -n=64 -source=0 \
  -seed=17 -schedule=physical \
  -timing -unified-gpus=1,2,3,4 -use-unified-memory \
  -grit-mode=grit -verify -report-all -disable-rtm \
  -metric-file-name="$OUT/bfs" \
  -result-json="$OUT/bfs-result.json" \
  -grit-trace="$OUT/bfs-transitions.csv"
```

### SpMV only

```bash
cd "$(git rev-parse --show-toplevel)/mgpusim"
mkdir -p bin results/irregular
go build -o bin/irregular ./samples/irregular
OUT=$(mktemp -d "results/irregular/spmv-4gpu-XXXXXX")

./bin/irregular \
  -workload=spmv -generator=skewed -n=64 \
  -seed=17 -schedule=physical \
  -timing -unified-gpus=1,2,3,4 -use-unified-memory \
  -grit-mode=grit -verify -report-all -disable-rtm \
  -metric-file-name="$OUT/spmv" \
  -result-json="$OUT/spmv-result.json" \
  -grit-trace="$OUT/spmv-transitions.csv"
```

### PageRank only

```bash
cd "$(git rev-parse --show-toplevel)/mgpusim"
mkdir -p bin results/irregular
go build -o bin/irregular ./samples/irregular
OUT=$(mktemp -d "results/irregular/pagerank-4gpu-XXXXXX")

./bin/irregular \
  -workload=pagerank -generator=skewed -n=64 -iterations=10 -damping=0.85 \
  -seed=17 -schedule=physical \
  -timing -unified-gpus=1,2,3,4 -use-unified-memory \
  -grit-mode=grit -verify -report-all -disable-rtm \
  -metric-file-name="$OUT/pagerank" \
  -result-json="$OUT/pagerank-result.json" \
  -grit-trace="$OUT/pagerank-transitions.csv"
```

### Gather only

```bash
cd "$(git rev-parse --show-toplevel)/mgpusim"
mkdir -p bin results/irregular
go build -o bin/irregular ./samples/irregular
OUT=$(mktemp -d "results/irregular/gather-4gpu-XXXXXX")

./bin/irregular \
  -workload=gather -n=64 -profile=phase-shift \
  -seed=17 -schedule=physical \
  -timing -unified-gpus=1,2,3,4 -use-unified-memory \
  -grit-mode=grit -verify -report-all -disable-rtm \
  -metric-file-name="$OUT/gather" \
  -result-json="$OUT/gather-result.json" \
  -grit-trace="$OUT/gather-transitions.csv"
```

### SSSP only

```bash
cd "$(git rev-parse --show-toplevel)/mgpusim"
mkdir -p bin results/irregular
go build -o bin/irregular ./samples/irregular
OUT=$(mktemp -d "results/irregular/sssp-4gpu-XXXXXX")

./bin/irregular \
  -workload=sssp -generator=skewed -n=64 -source=0 \
  -seed=17 -schedule=physical \
  -timing -unified-gpus=1,2,3,4 -use-unified-memory \
  -grit-mode=grit -verify -report-all -disable-rtm \
  -metric-file-name="$OUT/sssp" \
  -result-json="$OUT/sssp-result.json" \
  -grit-trace="$OUT/sssp-transitions.csv"
```

### Connected components (CC) only

```bash
cd "$(git rev-parse --show-toplevel)/mgpusim"
mkdir -p bin results/irregular
go build -o bin/irregular ./samples/irregular
OUT=$(mktemp -d "results/irregular/cc-4gpu-XXXXXX")

./bin/irregular \
  -workload=cc -generator=skewed -n=64 \
  -seed=17 -schedule=physical \
  -timing -unified-gpus=1,2,3,4 -use-unified-memory \
  -grit-mode=grit -verify -report-all -disable-rtm \
  -metric-file-name="$OUT/cc" \
  -result-json="$OUT/cc-result.json" \
  -grit-trace="$OUT/cc-transitions.csv"
```

Each run prints its verification result and writes its own metrics and outputs
under the fresh `$OUT` directory. Change `-n` to adjust size,
`-unified-gpus=1,2` for two GPUs, or `-grit-mode=ot|ac|dup` to choose one fixed
policy (use one value, for example `-grit-mode=dup`). Gather's `-profile` selects
the access pattern; the available profiles are listed below.

### Run all Gather profiles

Using the helper above:

```bash
for profile in private all-shared-read producer-consumer alternating-readwrite \
               phase-shift uniform skewed hotset high-locality low-locality; do
  run_irregular "gather-$profile" -workload=gather -n=65 -profile="$profile"
done
```

Private regions are page-aligned and disjoint. Shared-read consumers read the
same input region; producer/consumer and alternating-write phases synchronize
before reads. Phase shift runs private, shared-read, then alternating read/write.
Atomic scatter is not implemented. Gather requires `-schedule=physical`.

### Use local input files

```bash
run_irregular bfs-file -workload=bfs \
  -input=datasets/irregular/sink_graph.el -n=6 -source=0
run_irregular spmv-file -workload=spmv \
  -input=datasets/irregular/rectangular.mtx
run_irregular pagerank-file -workload=pagerank \
  -input=datasets/irregular/sink_graph.el -n=6 -iterations=10
run_irregular sssp-file -workload=sssp \
  -input=datasets/irregular/tiny_weighted.wel -n=6 -source=0
run_irregular cc-file -workload=cc \
  -input=datasets/irregular/symmetric.mtx
```

`.el` lines contain zero-based `from to`; `.wel` lines contain `from to weight`.
Supply `-n` to retain isolated vertices. Matrix Market dimensions are taken from
its header; indexing is converted from one-based, symmetric entries are expanded,
and duplicate entries are retained. SpMV sums duplicate entries; PageRank counts
parallel edges. Graph workloads require square inputs. The supplied fixtures are
original synthetic data dedicated to the public domain under CC0-1.0; no dataset
download is needed. Generated graphs and Gather phase inputs have SHA-256 hashes
in their result metadata.

## 4. Compare GRIT with fixed policies

For example, run the same four-GPU SpMV input under all four policies:

```bash
for policy in ot ac dup grit; do
  POLICY="$policy" run_irregular "spmv-$policy" \
    -workload=spmv -generator=skewed -n=64
done
```

Replace `spmv` with any of the six workload names and keep its options identical
across policies. Each case must have a distinct output name.

For the existing GRIT microbenchmark comparison tool, including ablations:

```bash
python3 scripts/grit/compare.py "$PWD/bin/gritmicro" \
  --gpus=1,2,3,4 --output="$OUT/grit-comparison" --ablations
```

### GRIT-disabled and single-GPU baselines

Use static device memory for the disabled-GRIT baseline:

```bash
./bin/irregular -workload=spmv -generator=skewed -n=64 \
  -timing -gpus=1,2,3,4 -schedule=physical -grit-mode=off \
  -verify -report-all -disable-rtm \
  -metric-file-name="$OUT/spmv-off-4gpu"

./bin/irregular -workload=bfs -generator=path -n=64 -source=0 \
  -timing -gpus=1 -grit-mode=off -verify -report-all -disable-rtm \
  -metric-file-name="$OUT/bfs-off-1gpu"
```

Omit `-use-unified-memory` for the multi-GPU mode-off baseline: legacy UVM failed
shared physical-shard output checks, and that combination is explicitly rejected.
Static-memory mode off is a conventional-memory baseline, not an identical-memory-
model comparison with the four GRIT policies.

## 5. GPU scheduling and memory capacity

The default `-schedule=physical` explicitly partitions destination rows across
physical GPUs. GRIT still requires the `-unified-gpus` platform flag. To use two
GPUs instead of four, change it to `-unified-gpus=1,2`.

`-schedule=unified` instead uses the existing virtual-device workgroup scheduler
for BFS, SpMV, PageRank, SSSP and CC. Small virtual launches can run entirely on
GPU 1: with 64-workitem groups and 64 CUs/GPU, two-GPU activity requires more than
64 groups, and four-GPU activity requires more than 192 groups. Physical sharding
is preferable for the tiny examples above. Confirm activity from per-GPU counters;
a GPU-selection flag alone is not proof.

Physical DRAM defaults to 32 MiB/GPU. `-gpu-dram-size-mb` accepts a positive power
of two. To stress managed residency, append **one** of the following to an enabled
GRIT run:

```text
-grit-resident-pages=1
```

or, supplying your actual application footprint in bytes:

```text
-grit-footprint-bytes=1048576 -grit-capacity-ratio=0.70
```

The second example supplies a 1-MiB footprint and limits residency to
`floor(1048576 * 0.70 / 4096)` pages per GPU. It does not measure the workload's
footprint automatically. Leave physical space for driver/kernel allocations.
Other controls include `-grit-ac-threshold=256`, `-grit-fault-threshold=4`,
`-grit-peer-bandwidth-gbps=300`, and `-grit-host-bandwidth-gbps=32`.

## 6. Automated verification and results

```bash
# Build, then test all workloads/policies with the main cases on four GPUs.
# Additional controlled Gather-profile cases use two GPUs by design.
bash scripts/irregular/smoke.sh --gpus=4 --policies=ot,ac,dup,grit

# Default matrix: emulator, one-GPU mode off, and two/four-GPU policies.
bash scripts/irregular/smoke.sh

# Boundary, fixture, malformed-input, and scheduler checks.
python3 scripts/irregular/run.py --suite edge
python3 scripts/irregular/validate.py --quick
python3 scripts/irregular/validate.py

# Above-L2 shared Gather stress test.
python3 scripts/irregular/run.py --suite stress --policies=dup --timeout=180

# Optional larger multi-seed study; tune size and timeout before running.
bash scripts/irregular/full.sh --n=129 --timeout=180
```

The runner accepts `--n`, `--gpus`, `--policies`, `--timeout`, and a new `--output`
directory. Defaults also come from `GPUS`, `POLICIES`, and `TIMEOUT`. It preserves
failed cases and exits nonzero if cases or audits fail. The default full matrix
uses seeds 17/29/43 and 1025 rows unless overridden.

Each automated run creates a timestamped directory under `results/irregular/`
containing commands, executable/source hashes, logs, verified outputs, raw metrics,
`summary.csv`, `comparisons.csv`, and `audit.json`. Manual runs above save files
under `$OUT`. A passing result includes `verification: Passed!`; completed GRIT
JSON reports must have `active_requests=0` and `pending_operations=0`.

Per-GPU `cu_inst_count` and DRAM transaction counters show physical work. The
independent audit also checks nonzero kernel time, matching policy inputs/outputs,
and actual shared input-page events or peer-request/owner pairs. Computed Gather
page sets are predictions, not executed memory traces. Driver kernel/total times
are simulated seconds in raw CSV; normalized summaries use nanoseconds. Kernel
time and end-to-end time differ because setup/transfers are included in the latter.
Speedup is baseline simulated kernel time divided by GRIT simulated kernel time.
Small smoke cases establish correctness, not representative performance.

`go test ./...` still reproduces three pre-existing driver-spec failures involving
RDMA drain, page-migration responses, and GPU restart responses. Workload package
and GRIT/runner checks passed. The larger research grid is optional; these commands do not establish
paper-reproduction performance claims.
