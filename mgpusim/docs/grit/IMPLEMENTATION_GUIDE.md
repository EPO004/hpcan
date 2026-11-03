# GRIT (HPCA 2024) — Complete Implementation Blueprint for MGPUSim v3

**Target:** MGPUSim **v3.0.3** (`5d49f873df5279182500c6878935c3207a013e59`), with its matching **Akita v3.0.0** (`11d3f6cae3645bb796fd52c357c5492f0611f3d4`).  
**Paper:** Yueqi Wang, Bingyao Li, Aamer Jaleel, Jun Yang and Xulong Tang, *GRIT: Enhancing Multi-GPU Performance with Fine-Grained Dynamic Page Placement*, HPCA 2024, DOI: [10.1109/HPCA57654.2024.00085](https://doi.org/10.1109/HPCA57654.2024.00085).  
**Date of guide:** 27 October 2026.  
**Purpose:** A source-file-by-source-file engineering specification for implementing and evaluating GRIT, not a claim that MGPUSim v3 already contains GRIT or that the code below has been compiled.

## Read this first — reproduction status and scope

This guide is based on the full 15-page attached paper and inspection of the two source releases above. The repository paths and existing function names marked **EXISTING** were checked against those source releases. The `grit/...` files, types, Go signatures and pseudocode marked **NEW / PROPOSED** describe what you must add. They do **not** already exist in MGPUSim. Apply changes in your own fork; leave upstream packages intact except for controlled integrations described below.

**There is no honest one-line GRIT patch.** In v3, the Akita MMU has one shared logical `vm.PageTable`, and `mmu.pageNeedMigrate` initiates migration when a unified page belongs to a different GPU. `processMigrationReturn` also sets `page.IsPinned = true`. That behavior cannot implement repeated on-touch movement, remote PTE mappings, page replicas and write-protection faults by adjusting a threshold alone. Likewise, a TLB miss is *not* equivalent to GRIT's **local-page fault**. A faithful version needs explicit page residency and coherence state, new fault events, and access permissions on the real memory path.

The paper omits some low-level implementation choices (exact cache lookup latency, how evictions are chosen, transactions during racing faults, interconnect topology, allocation behavior for absent neighboring pages). Those are identified as **engineering assumptions** with tests; do not represent them as specified paper facts. A replication of the reported percentages additionally requires the authors' exact input instances, simulator changes and tuning. The existing MGPUSim v3 benchmark suite is not identical to the eight application versions in Table II.

### A. Paper requirements, at a glance

| Feature | What to model | Paper reference |
|---|---|---|
| Initial policy | Every newly allocated page starts as **on-touch** | §V-C, §VI-A |
| Three policies | `OT` (on-touch migration), `AC` (access-counter remote access + migration), `DUP` (replicate for reads; collapse on write) | §II-B |
| Scheme-change trigger | **4** local-page/protection faults **for the same page, per decision epoch** | §V-B/C; Fig. 21 |
| Policy selection at trigger | Read-only fault history → `DUP`; any write/protection fault → `AC` | §V-C; Fig. 13 |
| Read/write bit | `0=read`, `1=write`; once set, remains sticky until the next epoch | §V-C |
| PA-Table | CPU-memory entry = 45-bit VPN + 1 read/write bit + 2 fault-counter bits = **48 bits** | §V-C/F |
| PA-Cache | 64 entries, 4-way, 16 sets, VPN low 4 bits as set index, LRU, write-back, write-allocate | §V-C; Fig. 12 |
| PA-Cache size | 41-bit tag + 1 write bit + 2 counter bits = 44 bits × 64 = **352 bytes** (excluding realistic metadata) | §V-F |
| AC migration trigger | 256 remote cache-line accesses; counting region **64 KB** | §II-B2; Table I |
| Predictor | Aligned groups of **1, 8, 64, 512** consecutive 4-KB pages | §V-D; Table V |
| Predictor vote | Strictly more than half (`5 of 8`) matching entries/groups | §V-D |
| PTE logical scheme codes | `01=OT`, `10=AC`, `11=DUP`; `00` reserved/uninitialized | §V-C; Table IV |
| Group-size codes | `00=1`, `01=8`, `10=64`, `11=512` pages | §V-D; Table V |
| PTE placement in *paper* | Scheme in bits 9–10; group in host-side bits 52–53 | Fig. 14 |
| Default page size | 4 KB; optional 2 MB sensitivity run | Table I; §VI-B3 |
| Default experiments | 4 GPUs, sensitivity 2/8/16 GPUs | §VI |

**Logical versus physical PTE bits:** Akita v3 represents a page as a Go struct (`vm.Page`), not a 64-bit x86/NVIDIA PTE. You must add `Scheme` and `GroupCode` typed fields or a separate metadata table. Do **not** blindly bit-shift `PAddr` or assume bits 9–10/52–53 exist in `vm.Page`. You can serialize physical bit positions later for a bit-exact hardware research model.

### B. Baseline configuration to reproduce (paper Tables I–II)

| Item | Paper target | MGPUSim v3 action |
|---|---|---|
| GPU compute | 64 CUs/GPU, 1 GHz | Existing builder defaults to 16 shader arrays × 4 CUs; verify CU timing |
| L1 vector | 16 KB, 4-way | Inspect `samples/runner/shaderarray.go`; tune build methods |
| L1 instruction | 32 KB, 4-way | Same |
| L1 scalar | 16 KB, 4-way | Same |
| L2 | 256 KB, 16-way | Inspect `samples/runner/r9nanobuilder.go::buildL2Caches` |
| GPU memory | DRAM capacity = 70% of application footprint per paper's experimental model | Add enforceable GPU resident-frame budget and eviction/backing |
| L1 TLB | 32 entries, fully associative, 1 cycle, CU-private, LRU | Existing shader-array TLB configuration differs; redesign placement if reproducing exactly |
| L2 TLB | 512 entries, 16-way, 10 cycles, CU-shared | Existing builder uses 64 ways and capacity based on DRAM: change |
| GMMU walkers | 8 shared walkers, 100 cycles per level, 128-entry walk cache, 64-entry queue | Existing Akita MMU uses a simplified fixed page-walk latency; extend for fidelity |
| Inter-GPU | 300 GB/s NVLink v2 | Existing platform uses a PCIe connector. Build/tune a distinct GPU–GPU path |
| CPU–GPU | 32 GB/s PCIe v4 | `samples/runner/timingplatform.go::createConnection`; verify achieved bandwidth and latency |
| AC threshold | 256 | New GRIT AC counter |
| Memory page | 4 KB | Existing `log2PageSize=12` |

### C. Where to change the actual code

Paths below are relative to two checkouts: `mgpusim/` and `akita/`. This distinction matters: `github.com/sarchlab/akita/v3` is an **external Go module**, so changes placed only in MGPUSim's `driver` directory cannot modify the Akita MMU or TLB behavior.

| File | Existing function/struct | Required GRIT edit |
|---|---|---|
| **NEW** `mgpusim/grit/types.go` | — | Policies, faults, PID/VPN keys, page/replica directory |
| **NEW** `mgpusim/grit/directory.go` | — | Canonical page state and per-GPU resident mappings |
| **NEW** `mgpusim/grit/pa_table.go` | — | PA-Table, PA-Cache and fault threshold |
| **NEW** `mgpusim/grit/predictor.go` | — | 1→8→64→512 grouping, promotion/degradation |
| **NEW** `mgpusim/grit/policy.go` | — | OT/AC/DUP policy decisions and transitions |
| **NEW** `mgpusim/grit/counters.go` | — | Access-counter 64-KB regions and 256 threshold |
| **NEW** `mgpusim/grit/metrics.go` | — | Fault, migration, replication and timing metrics |
| `akita/mem/vm/pagetable.go` | `vm.Page`, `PageTable` | Metadata-compatible fields/wrapper; avoid changing `PageTable` interface if possible |
| `akita/mem/vm/protocol.go` | `TranslationReq`, `TranslationRsp`, migration messages | Access intent, origin GPU, explicit fault resolution, response identity |
| `akita/mem/vm/mmu/mmu.go` | `finalizePageWalk`, `pageNeedMigrate`, `sendMigrationToDriver`, `processMigrationReturn` | Route on real residency/policy instead of unconditional migration; **remove unconditional post-migration pinning for GRIT pages** |
| `akita/mem/vm/mmu/builder.go` | `Builder` | Inject GRIT controller and configurable page-walk model |
| `akita/mem/vm/tlb/tlb.go` | `lookup`, `parseBottom`, `handleTLBFlush` | Per-GPU valid/permission/generation checks and targeted invalidation |
| `akita/mem/vm/addresstranslator/addresstranslator.go` | `translate`, `parseTranslation`, `createTranslatedReadReq/WriteReq` | Preserve read/write request intent and honor local/replica/remote mapping |
| `akita/mem/cache/...` | vector/scalar cache front ends | Write-protection check must run even on **cache hits**, before any data mutation |
| `mgpusim/driver/driver.go` | `Driver`, `parseFromMMU`, `preparePageForMigration`, shootdown/restart pipeline | GRIT fault dispatcher, three policy states, copy/collapse/invalidation FSM |
| `mgpusim/driver/builder.go` | `Builder.Build` | Instantiate the GRIT controller and expose ports/metrics |
| `mgpusim/driver/internal/memoryallocator.go` | `allocatePageWithGivenVAddr`, `RemovePage`, `Remap` | Reclaim frames; allocate a copy without replacing the only canonical mapping |
| `mgpusim/driver/internal/device.go` | `allocatePage`, `mustHaveSpaceLeft` | DRAM occupancy, out-of-capacity eviction rather than panic |
| `mgpusim/protocol/driverprotocol.go` | Shootdown, restart, migration messages | New fault, replicate, collapse and page-specific ACK messages |
| `mgpusim/timing/pagemigrationcontroller/pmc.go` | `PageMigrationController` | Reuse page transfer; add duplication/copy endpoints and correct retirement |
| `mgpusim/timing/rdma/comp.go` | `processReqFromL1`, `processReqFromOutside` | Remote read/write instrumentation; AC 64-KB counter trigger |
| `mgpusim/timing/cp/commandprocessor.go` | `processShootdownCommand`, flush/restart paths | Reuse barriers; make shootdowns targeted and avoid global flushing unless required |
| `mgpusim/samples/runner/timingplatform.go` | `Build`, `createMMU`, `createConnection` | Wire new modules, tune network and page walk |
| `mgpusim/samples/runner/r9nanobuilder.go` | `buildL2TLB`, `buildL2Caches`, `buildDRAMControllers` | Exact cache/TLB/DRAM parameters and counters |
| `mgpusim/samples/runner/shaderarray.go` | `buildL1*TLB`, `buildL1*Caches` | L1 sizes and protected-access interception |
| `mgpusim/samples/runner/flag.go` | flags | Add `-grit-mode`, threshold, prediction/PA-cache controls |
| `mgpusim/samples/runner/runner.go` | `buildTimingPlatform`, `createUnifiedGPUs` | Route options; require timing+UVM+multi-GPU |
| `mgpusim/samples/runner/report.go` | reporting methods | Output per-mode counters and kernel time |
| `mgpusim/driver/driver.go` | `distributeWGToGPUs` | Match paper's TB scheduling experiment, not just MGPUSim's fixed work-group chunks |

**Note:** the paper's four named phases (fault detection, PA lookup, decision, prediction) do not by themselves implement cache coherence or physical memory copies. You must first get the three uniform policies **correct**; only then enable automatic scheme selection.

## 0. Prepare a reproducible fork before modifying anything

Work on a Linux system with Go **1.22 or later compatible with the v3.0.3 `go.mod`**. Do **not** use MGPUSim v4 or current `amd/`-based documentation paths; v4 reorganized the project. Pin these revisions:

```bash
git clone https://github.com/sarchlab/mgpusim.git mgpusim
cd mgpusim
git checkout 5d49f873df5279182500c6878935c3207a013e59
git switch -c grit-hpca24
cd ..

git clone https://github.com/sarchlab/akita.git akita
cd akita
git checkout 11d3f6cae3645bb796fd52c357c5492f0611f3d4
git switch -c grit-v3
cd ..
```

Make Akita changes visible to MGPUSim using a workspace file **outside** both tracked repositories:

```bash
go work init ./mgpusim ./akita
go work use ./mgpusim ./akita
go work sync
cd mgpusim
go test ./driver/... ./timing/... ./samples/runner/...
```

If `go work` causes dependency conflicts, alternatively add `replace github.com/sarchlab/akita/v3 => ../akita` to your *fork's* MGPUSim `go.mod`. Choose **one** method. `go.mod` in MGPUSim v3.0.3 requests Akita v3.0.0. Always record `go version`, commit hashes, build flags and whether `go.work` is active.

Before any GRIT edit, build and run a tiny deterministic existing benchmark:

```bash
cd mgpusim/samples/fir
go build
./fir -timing -unified-gpus=1,2,3,4 -use-unified-memory -report-all
```

The command is a **smoke-test candidate**, not a guarantee that every benchmark's data layout uses multi-GPU sharing. Run `./fir -h` first and check benchmark-specific args. Save output, kernel cycles, migration counts and any correctness output. Also run `go test ./...` for the baseline once dependencies are available. **Passing compilation is not sufficient to validate migration correctness.**

**Suggested branch milestones:** `baseline-v3`, `grit-directory`, `uniform-ot`, `uniform-ac`, `uniform-dup`, `fault-trigger`, `pa-cache`, `predictor`, `evaluation`. Commit after each green milestone so regressions can be bisected.

## 1. Define a single source of truth for logical pages and replicas

### 1.1 NEW `grit/types.go`: stable identities and states

Do not key metadata on virtual address alone: different processes can reuse the same VPN. Use `(PID, VPN)` where `VPN = virtualAddress >> 12`. GPU IDs are the **real device IDs** (1–N in driver-facing interfaces), not the unified virtual GPU ID and not the zero-based `d.GPUs` slice index.

```go
// PROPOSED interfaces/types; integrate into your own grit package.
package grit

type Scheme uint8
const (
    SchemeInvalid Scheme = iota // 00, never an active policy
    SchemeOnTouch               // 01
    SchemeAccessCounter         // 10
    SchemeDuplication           // 11
)

type AccessKind uint8
const (
    AccessRead AccessKind = iota
    AccessWrite
)

type FaultKind uint8
const (
    FaultLocalPage FaultKind = iota
    FaultWriteProtection
)

type PageKey struct {
    PID uint32 // cast to/from vm.PID at Akita boundary
    VPN uint64
}

type Replica struct {
    GPUID       uint64
    PAddr       uint64
    Valid       bool
    Writable    bool
    Dirty       bool
    Generation  uint64
}

type PageState struct {
    Key         PageKey
    Scheme      Scheme
    HomeGPU     uint64       // authoritative owner / latest data holder
    OwnerPAddr  uint64
    Replicas    map[uint64]Replica
    Version     uint64       // increases on writes and ownership changes
    Epoch       uint64       // increases when a policy decision resets history
    Pending     bool         // serializes conflicting operations on this page
    GroupCode   uint8        // canonical group metadata is base-page only
}
```

`PageState` is the semantic model. `vm.Page` continues to expose the physical address relevant to an individual translation, but the global directory must be able to return a different `PAddr` for each GPU when `DUP` is active. A single `vm.Page.PAddr` cannot represent replicas.

### 1.2 NEW `grit/directory.go`: ownership, permissions, lookup

Implement the following methods (signatures illustrative):

```go
type Directory interface {
    Lookup(key PageKey) (PageState, bool)
    LookupMapping(key PageKey, gpu uint64, access AccessKind) (Replica, bool)
    BeginOperation(key PageKey) (PageState, bool) // false => already locked/pending
    CommitOperation(key PageKey, next PageState)
    AbortOperation(key PageKey)
    OnAllocate(key PageKey, initialGPU uint64, paddr uint64)
    OnFree(key PageKey)
    InvalidateReplica(key PageKey, gpu uint64)
    RecordCommittedWrite(key PageKey, gpu uint64)
}
```

Use either a mutex-protected map (correctness-first serial simulation) or a simulator component with a request/response port (best for modeled contention). If sharing an object across simulator components, do not mutate it behind Akita's event scheduling in a way that skips modeled delays. **Every operation must be ordered by events/ACKs**. Assert invariants:

1. At most one *writable* copy of any `(PID,VPN)` exists at once.
2. Any number of **read-only** copies may exist in `DUP`.
3. The latest data is not discarded until it is copied back or moved to a surviving owner.
4. A cached mapping with an older generation cannot be used after a shootdown.
5. `OT` never retains readable stale replicas on other GPUs.
6. `AC` may map a page remotely; the actual physical address stays at its authoritative owner until threshold migration.
7. `Pending` transitions queue additional conflicting reads/writes/faults rather than dropping them.

At allocation (`driver/internal/memoryallocator.go::allocatePages` and `allocatePageWithGivenVAddr`), register a fresh `PageState` with `SchemeOnTouch`, no duplicates, and an owner. A unified allocation may be physically interleaved across GPUs; map each *actual* backing frame to its actual GPU, not the virtual unified GPU ID. At `RemovePage`, `Free` and `Remap`, revoke and release replicas and invalidate corresponding per-GPU mappings. Test allocation/release separately from migration.

### 1.3 Extend Akita `vm.Page` **without losing mappings**

Edit **EXISTING** `akita/mem/vm/pagetable.go::Page` and add fields such as:

```go
// Added to existing vm.Page; PROPOSED fields.
Scheme       uint8  // paper PTE codes: 1 OT, 2 AC, 3 DUP
GroupCode    uint8  // meaningful on the aligned base PTE only
Generation   uint64
Writable     bool   // for *this* GPU's PTE; do not confuse with global owner
```

However, `vm.PageTable.Find(PID, VAddr)` does **not** take GPUID, so it cannot supply per-GPU `PAddr`, writable state and resident-bit by itself. Keep this as a **host canonical PTE**, and create either:

- **Recommended:** a `grit.GPUPageTable[GPU][PageKey]` (software model of local PTEs) consulted by the MMU/guard; OR
- a wrapper that provides `LookupForGPU(pid, vaddr, gpu, access)` while preserving the old `vm.PageTable` interface for allocation.

Do not break all existing `PageTable` mocks by adding methods to the upstream interface unnecessarily. Prefer a separate GRIT interface, adapter and Go type assertion. For all direct construction/copy paths (`driver/internal/memoryallocator.go`, `driver/driver.go::preparePageForMigration`, `vm.TranslationRsp`), propagate or reconstruct the correct scheme and generation. An allocator returning a new `vm.Page` struct will otherwise silently erase added metadata.

### 1.4 Capacity and physical frames

The v3 allocator can panic when no free physical address remains (`driver/internal/device.go::mustHaveSpaceLeft`). For GRIT, each additional replica consumes another physical frame. Add per-device resident bytes, reserved/inflight frames and a separate backing store for evicted data. A safe `ReserveFrame/CommitFrame/ReleaseFrame` transaction prevents double-allocation:

```text
Reserve target frame → quiesce owner/dirty cache → copy complete →
update owner/replica directory → update PTE generation → release obsolete frame
```

Do not release the source frame before the successful copy ACK. For a read replica, source ownership must remain valid. In oversubscription, choose a deterministic victim policy (e.g. LRU), write back dirty data to host backing as needed, invalidate all stale mappings and account for eviction cost. **Victim-selection policy is not specified by the paper**; document it as an assumption and run sensitivity tests. Model per-GPU DRAM capacity as an actual enforced limit rather than simply allocating a large `mem.Storage` object.

## 2. Introduce *real* page faults and access permissions

This step is the most important departure from unmodified v3. The paper counts **local page faults** (invalid local GPU PTE) and **page protection faults** (a write to a read-only duplicate), **not** ordinary L1 or L2 TLB misses.

### 2.1 Events to distinguish

| Event | Meaning | PA counter? | Resolution |
|---|---|---|---|
| L1 TLB miss | Translation not in L1 TLB | **No** | Search L2 TLB |
| L2 TLB miss, valid GPU-local PTE | Translation not cached, local PTE exists | **No** | Page walk then fill TLB |
| Local-page fault | GPU-local PTE invalid or missing | **Yes** | Ask host UVM / GRIT to install, migrate, map remote, or replicate |
| Write-protection fault | Write targets a read-only local replica/PTE | **Yes** | Invalidate replicas, collapse, establish writable owner |
| Remote cache-line access | Data transaction crosses GPUs | **No** (uses AC counter instead) | RDMA / peer memory route |
| AC threshold reached | 256 counted remote accesses to 64-KB region | **No** | Request migration, separate from PA threshold |

Introduce a fault message in **NEW** `grit/protocol.go`, or in **EXISTING** `mgpusim/protocol/driverprotocol.go` if needed for driver routing:

```go
// PROPOSED; sim.MsgMeta must be initialized with a unique ID, Src, Dst, SendTime.
type PageFaultReq struct {
    sim.MsgMeta
    PID          vm.PID
    VAddr        uint64   // 4-KB-aligned page address
    RequesterGPU uint64   // actual GPU ID (1..N)
    Access       grit.AccessKind
    Kind         grit.FaultKind
    ParentReqID  string   // original stalled access ID
}

type PageFaultRsp struct {
    sim.MsgMeta
    RespondTo   string
    Generation  uint64
    Retry       bool
}
```

Construct responses with an explicit request ID; never resume a different stalled request. Add `FaultReq`/`FaultRsp` buffers and backpressure handling. Deduplicate simultaneous faults for the same `(PID, VPN, GPU, generation)` into a waiter list: multiple requests stalled behind one fault should produce **one** fault-counter increment if the hardware coalesces them. Record whether and where coalescing occurs in your simulator; the paper does not specify the exact coalescing boundary.

### 2.2 Edit Akita `TranslationReq` and translation plumbing

**EXISTING** `akita/mem/vm/protocol.go::TranslationReq`: add `AccessKind` and optionally a `RequestID`/epoch to carry read-vs-write semantics from the source. Extend its builder with `WithAccessKind`. This field must be set **when constructing the request** in **EXISTING** `akita/mem/vm/addresstranslator/addresstranslator.go::translate` from `mem.ReadReq` versus `mem.WriteReq`, and copied in **EXISTING** `akita/mem/vm/tlb/tlb.go::fetchBottom`. Otherwise the MMU has no way to tell a write fault from a read fault.

**Critical:** an access may hit in L1 data cache without visiting the address translator or MMU. Therefore, implement a page-permission guard at the entrance to every relevant **data cache** (vector and scalar; cover instruction fetch separately where applicable) or at a common CU memory gateway. Inspect `akita/mem/cache/writearound/...`, `akita/mem/cache/writethrough/...` and `mgpusim/samples/runner/shaderarray.go::connectComponents`; insert the guard *before any cache hit can accept a write*. The guard checks the GPU-local PTE permissions and generation and stalls/protects the memory request before the cache changes data.

Recommended logical API:

```go
// NEW grit/guard.go; called before a cache can serve/commit access.
type AccessDecision uint8
const (
    AccessAllowed AccessDecision = iota
    NeedLocalFault
    NeedWriteProtectionFault
    WaitForPageTransition
)
func CheckAccess(pid vm.PID, vaddr uint64, gpuID uint64,
                 write bool, cachedGeneration uint64) AccessDecision
```

Once the PTE is valid and permissions permit access, ordinary TLB and cache hit/miss behavior proceeds. On a protection fault, park/retry the original write only **after** the last required invalidation ACK and write ownership is committed. If you skip the cache-hit guard, duplication will silently lose writes.

### 2.3 Edit Akita `MMU` around existing migration logic

The important **EXISTING** Akita v3 functions are:

- `mmu.go::finalizePageWalk` obtains the canonical page; it currently invokes `pageNeedMigrate`.
- `mmu.go::pageNeedMigrate` simply compares `walking.req.DeviceID` and `walking.page.DeviceID`, then checks `Unified` and `IsPinned`.
- `mmu.go::sendMigrationToDriver` builds a `PageMigrationReqToDriver`.
- `mmu.go::processMigrationReturn` unconditionally performs `page.IsPinned = true` after migration.

Create a GRIT-aware branch in `finalizePageWalk`, with the default legacy branch preserved when GRIT is disabled:

```text
IF GRIT disabled:
    use existing Akita v3 logic unchanged
ELSE:
    locate canonical PageKey and GPU-local PTE
    if PageState.Pending: enqueue request as waiter
    else if GPU-local PTE valid and permits AccessKind:
         return GPU-specific physical mapping; fill appropriate TLB
    else if valid but write disallowed:
         generate protection fault and wait for driver resolution
    else:
         generate local-page fault and wait for driver resolution
```

Do **not** call `pageNeedMigrate` just because a GPU-local TLB missed; GRIT's `AC` maps remote pages intentionally. On the GRIT branch, remove the unconditional `IsPinned=true` behavior after completion; only pin pages intentionally (e.g. user-mapped pinned memory). Keep a separate `Pending`/`IsMigrating` indicator for temporary in-flight operations. Ensure the MMU can have queued faults for distinct pages: v3's `isDoingMigration` and `currentOnDemandMigration` serialize migrations globally; you may initially retain serialization for correctness, then allow multiple nonconflicting pages if the pipeline supports it.

After a resolved fault, run a new translation or requery directory state before returning to TLB: **never reuse the pre-migration `page.PAddr`**. Keep `ParentReqID` and generation to ensure the resumed operation is the one that stalled. Changes to Akita's old unit-test mocks may be required if any public builder struct changes.

## 3. Implement all three uniform page-placement modes first

Add **NEW** `grit/policy.go`, **NEW** `grit/counters.go`, and an operations queue/state machine in **EXISTING** `mgpusim/driver/driver.go`. Expose `-grit-mode=ot|ac|dup|grit` but keep `grit` itself disabled until the three individual modes pass correctness tests.

### 3.1 OT: on-touch migration (paper Fig. 2A)

For a local-page fault on GPU `G`:

1. If the page is already locally resident in `G` but its PTE was invalid, install local `PAddr` and allow access; **no migration**.
2. If its authoritative frame is on GPU `H != G`, mark the page `Pending` and block concurrent accesses.
3. Drain relevant outstanding requests, flush/commit dirty cache lines for that page, and invalidate stale PTE/TLB/cache mappings from previous owners. See `driver.go::initiateRDMADrain`, `sendShootDownReqs`, and `timing/cp/commandprocessor.go::processShootdownCommand`.
4. Reserve destination page frame on `G` using the allocator; copy **all 4 KB** over the PMC mechanism; wait for **all data-copy ACKs**, not just an enqueue ACK.
5. Commit `HomeGPU=G`, `OwnerPAddr=destination`, `Version++`; remove/reclaim the obsolete owner frame only after commit; update directory and host canonical PTE.
6. Install a valid **writable** local GPU PTE, `Generation=newVersion`; invalidate any still-stale TLB entries; restart affected GPUs/RDMA only when safe.
7. Send success to MMU, replay stalled operation. Increment **one** page-fault counter event and **one** completed-migration statistic.

**EXISTING path to reuse:** `driver/driver.go::preparePageForMigration`, `sendMigrationReqToCP`, `processPageMigrationRspFromCP`, `preparePageMigrationRspToMMU` and `timing/pagemigrationcontroller/pmc.go`. Currently `preparePageForMigration` allocates and updates the global page table **before the copy finishes**. For GRIT, stage the new frame in the directory and only publish it atomically on completion; otherwise another request may read uninitialized data. Preserve legacy behavior for non-GRIT runs.

Acceptance trace: GPU1 write → GPU2 read → GPU1 write should cause two ownership transfers (assuming first allocation on GPU1), with correct final value, no stale replicas, and one unique writer.

### 3.2 AC: access-counter-based migration (paper Fig. 2B)

On a local-page fault from `G` when page home is `H`:

1. Do **not** automatically move data.
2. Install a valid GPU-local PTE mapping to **remote** `OwnerPAddr` on `H`. It must preserve the actual remote physical location.
3. Subsequent accesses reach the existing multi-GPU RDMA path (`timing/rdma/comp.go`, `createRDMAAddrTable` in `samples/runner/timingplatform.go`). The remote route must be correct for both reads and writes.
4. Count the paper's remote page-group accesses in **64-KB physical/virtual page regions**, using 16×4-KB pages per region. A nonlocal 64-byte cache-line request is one countable remote access; document how fragmented/coalesced simulator transactions correspond to this event.
5. When `count >= 256`, request **one migration** for the appropriate 4-KB page(s). The paper describes 64-KB counter granularity but not a fully explicit multi-page victim/migration selection algorithm; choose and document one policy. **Recommended first implementation:** migrate the hot 4-KB faulting page, reset its destination's 64-KB counter, and do not claim exact vendor behavior.
6. Drain RDMA, shoot down stale mappings, transfer data, update owner/generation, resume stalled traffic and restart access counters according to an explicit policy. For already local pages, count no remote accesses.

Suggested **NEW** counter key/state:

```go
type CounterKey struct {
    PID       uint32
    RegionVPN uint64 // (VPN / 16) * 16, aligned 64-KB region
    GPU       uint64 // GPU performing remote accesses
}
type CounterState struct {
    RemoteAccesses uint16
    MigrationArmed bool
}
```

Do not increment on address-translation misses or local cache hits: increment when an actual **remote cache-line transaction** crosses the GPU boundary. The natural hook is **EXISTING** `rdma/comp.go::processReqFromL1`, before forwarding `mem.ReadReq`/`mem.WriteReq` to the remote endpoint; prevent double counting retransmits and count each originating transaction once. Add an AC-threshold notification message to the driver; the driver must coalesce repeated threshold interrupts while a migration is armed. Keep the 256 count entirely separate from GRIT's threshold of **4 faults**.

**Critical AC correctness:** remote writes must update a single authoritative version. A remote PTE may be marked writable even though its physical target is on another GPU. The system must order writes with outstanding readers and ensure subsequent loads observe the latest committed data; global storage alone does not automatically make independently cached GPU L1/L2 copies coherent. Add invalidation or bypass for shared remote writable pages as needed and test read-after-write across GPUs.

### 3.3 DUP: on-demand read duplication and write collapse (paper Fig. 2C)

On a **read** from GPU `G` where the requested page exists elsewhere:

1. If `G` owns a valid local replica, return it with read permission.
2. Otherwise reserve a destination frame, wait for any dirty authoritative owner state to be committed, copy full page through PMC, and mark the new replica `Valid=true`, `Writable=false`.
3. Set GPU-local PTE to point to the **replica's PAddr**. Retain the original owner frame and all other valid replicas; this is a **copy**, not an OT move.
4. All `DUP` pages that have multiple valid copies should have those copies read-only. A sole exclusive owner may remain writable only if the directory guarantees no outstanding competing readers and permission transitions are atomic.

On a **write** from any GPU `G` to a duplicated/read-only page:

1. Generate a **write-protection fault** before *any* L1 cache hit can mutate data.
2. Lock the page. Drain reads/writes and ensure the latest dirty owner data is committed.
3. Send page-targeted invalidations to **all holders** (including any stale TLB/cache translations), wait for every ACK, and revoke read permissions.
4. If the current authoritative owner is not `G`, **move/copy** the most recent page image to `G` using PMC. Do not use an arbitrary stale replica as the source.
5. Delete/release all other replicas only after their outstanding reads have quiesced. Retain one writable owner, `Version++`; install a writable PTE for `G`.
6. Respond/retry the original write. Subsequent reads from another GPU can replicate the **new** version; another write-collapse occurs only if replicas have been created again.

**EXISTING components to edit:**

- `driver/internal/memoryallocator.go`: `AllocatePageWithGivenVAddr` currently overwrites the one global mapping; add a **separate replica-frame allocator** without altering canonical mapping until commit.
- `timing/pagemigrationcontroller/pmc.go`: expose copy-without-destroying-source and completed transfer ACKs.
- `timing/cp/commandprocessor.go`: adapt shootdown to invalidate the relevant page entries and dirty lines; v3's current flush can invalidate broadly.
- `akita/mem/vm/tlb/tlb.go` and cache guard: enforce read-only permissions for a **cached** translation.

Acceptance trace: GPU1 initializes a page; GPU2 read copies it; GPU3 read copies it; GPU2 write triggers exactly one collapse; GPU1 and GPU3 can no longer read stale copies; GPU3's later read sees GPU2's new value. Check replica occupancy `1 → 2 → 3 → 1 → 2` and that the final physical contents agree across all observed reads.

### 3.4 Transition semantics (uniform policy or dynamic GRIT)

Use one serialized `Transition(key, old, new)` operation. It must be safe even when **new == old** (the paper explicitly notes `AC → AC` events). Do not just rewrite two PTE bits; that is insufficient for `DUP` transitions.

| Old → new | Required physical/logical work |
|---|---|
| OT → OT | Usually no physical change; re-evaluate current GPU's local presence |
| OT → AC | Keep current owner; allow remote GPU PTEs; initialize AC counters; invalidate stale OT-only mappings where necessary |
| OT → DUP | Keep owner; on future remote reads create read replicas; revoke writable permissions before first sharing |
| AC → OT | Stop remote-counter decisions; invalidate stale remote mappings; migrate on next local-page fault; ensure no concurrent remote writes |
| AC → AC | Reset fault-decision epoch; retain current mapping, avoid unnecessary neighbor prediction |
| AC → DUP | Quiesce outstanding remote writers; use authoritative current image; install read-only replicas on demand |
| DUP → OT | **Collapse replicas**, choose one authoritative owner, revoke other mappings, release replica frames; later OT faults may migrate |
| DUP → AC | **Collapse replicas**; leave single authoritative remote-mappable owner and restart AC counters |
| DUP → DUP | Leave copies; start new fault epoch; do not duplicate already-present frames |

Page-group *background prediction* is different: per §V-D/E it updates **host-side future policy metadata** without immediate page copying or PTE/TLB invalidation. Apply the predicted scheme at the **next real page fault**, then perform a safe physical transition. Maintain `EffectiveScheme` versus `PredictedScheme` if needed; do not pretend that rewriting the host policy instantly changes existing GPU mappings.

## 4. Fault-Aware Initiator: schedule fault events through the UVM driver

The decision threshold is **4 total relevant page faults per page**, including local-page faults and page-protection faults. It is *not* a threshold of four remote memory operations, four TLB misses, or four page migrations.

### 4.1 Add a fault-to-driver path (do not overload migration ACKs)

**EXISTING** `mgpusim/driver/driver.go::parseFromMMU` currently only handles `*vm.PageMigrationReqToDriver`; add support for GRIT `PageFaultReq` and `ACMigrationThresholdReq`. Suggested staged queue:

```text
Akita MMU / cache guard
    └─ PageFaultReq(pid, vpn, gpu, R/W, local/protection, original request ID)
         └─ Driver GRIT fault queue
              ├─ Lookup PA-Cache / PA-Table (timed)
              ├─ Read centralized host scheme/prediction
              ├─ [faultCount < 4] use current/predicted scheme
              └─ [faultCount = 4] choose DUP if all-read; AC if any-write
                   └─ Page operation (map, migrate, duplicate, collapse)
                        └─ ACKs → install local mapping → PageFaultRsp
                             └─ Retry original memory request
```

In Akita `MMU::Tick`, add a sender for faults and a receiver for fault resolutions. Existing ports are `Top` and `Migration`. **Either** add a new dedicated `Fault` port in `akita/mem/vm/mmu/builder.go::createPorts` **or** extend the `Migration` message protocol with typed fault events. A dedicated port is easier to reason about; wire it in `samples/runner/timingplatform.go::Build` and `createConnection`, and make sure both driver and MMU ports are attached to the simulated network.

The driver must schedule CPU-side PA lookups and transitions as **simulation events**. Never call Go `time.Sleep` to model latency. Do not directly invoke another component's `Tick` with a made-up time. Use the v3's Akita `sim.TickingComponent` and its pending-send queues. Preserve source/destination ports and request IDs.

### 4.2 Suggested per-page fault handler state machine

```text
Idle
  → LockPage
  → PA_Access_And_HostWalk (may run in parallel)
  → SelectScheme
  → [InstallMapping | ScheduleOTMigration | ScheduleACMapping |
     ScheduleDuplicate | CollapseAndCopy]
  → WaitForDrain/Shootdown/CopyACK (only required stages)
  → CommitDirectoryAndPTE
  → RespondToMMU
  → UnlockAndReplayWaiters
  → Idle
```

Implement a `map[PageKey]*InFlightOperation`. Keep each operation's pending ACK count **per page** and per message type. The existing `Driver` fields `currentPageMigrationReq`, `numShootDownACK`, `numPagesMigratingACK`, `numRestartACK` are **global singleton state**; using them for overlapping GRIT requests can associate a response with the wrong page. The correctness-first approach is a single FIFO operation at a time; the optimized approach uses per-page FSMs and request-ID correlation. Either approach must never return a fault response before mapping state and required ACKs are committed.

### 4.3 Exact counting and lifetime reset

At every **new, non-coalesced** GRIT fault for a key:

1. Atomically obtain its PA entry (timed cache access).
2. If `kind == FaultWriteProtection` or `AccessKind == AccessWrite`, set sticky write flag. (The paper says read/write type is set from the requested access and stays `1` for the scheme lifetime.)
3. Increment the epoch's 2-bit logical counter. When the **fourth** event arrives, trigger a decision, then delete the PA record from **both** cache and backing PA table and begin a new epoch.
4. The first fault after deletion creates a fresh entry, count=1 and write bit reflecting that fault. Store the decision epoch separately in the page directory so stale writebacks from older cache entries cannot resurrect deleted state.

The paper's 2-bit counter `00…11` cannot literally store 4, so **detect `count == 3` before incrementing on the fourth event**. Do not overflow `11→00` without triggering. Tests should check threshold crossing at precisely `[1,2,3,4]`, reset, and then another four faults. The PA-Cache miss latency is not a separate *extra fault*.

If a private page faults once and remains resident, its `count=1` will stay until it is evicted/invalidated or the process terminates. **Do not force a scheme change for private pages.** This is the intended reason GRIT leaves mostly private pages as OT (§V-C).

## 5. Implement PA-Table and 64-entry PA-Cache

### 5.1 NEW `grit/pa_table.go`: canonical representation

The **paper's logical** entry format is 48 bits, but a normal Go struct is much larger because of padding, map headers and metadata. Record **logical hardware accounting** separately from actual simulator memory usage.

```go
// PROPOSED state. Actual Go object size is not the paper's 48-bit hardware size.
type PAEntry struct {
    Key   PageKey
    VPN   uint64
    Count uint8 // invariant: 0..3 until fourth event triggers
    Wrote bool
    Epoch uint64 // simulator correctness field, not charged to paper's 48 bits
}

type PACacheLine struct {
    Valid     bool
    Dirty     bool
    PID       uint32
    VPN       uint64
    Count     uint8
    Wrote     bool
    Epoch     uint64
    LastTouch uint64 // LRU metadata; implementation accounting separately
}

type PAController struct {
    Backing map[PageKey]PAEntry
    Sets    [16][4]PACacheLine
    Clock   uint64
}
```

The paper describes PA-Table entry VPN with no explicit PID field. For multiple simulated processes, you must add PID to the lookup key or isolate the table per process. **Do not merge two different processes that happen to use the same virtual page number.** A paper-size estimate can ignore PID overhead only when modeling a single address space.

Cache mapping:

```go
func paSet(vpn uint64) int { return int(vpn & 0xF) }
func paTag(vpn uint64) uint64 { return vpn >> 4 } // upper 41 VPN bits
```

The cache has exactly **16 sets × 4 ways** and chooses the least-recently-used valid way for eviction. It is write-allocate and write-back: a hit updates the cached entry but **does not** write CPU memory immediately. On a miss, if the selected victim is dirty and current, write its entry to the backing table; then load backing entry if present, or allocate a fresh `Count=0`, `Wrote=false` entry. Apply the arriving fault **exactly once** after the fill. If fourth fault triggers, delete the corresponding entry from both structures and return a `DecisionInfo` to the driver.

### 5.2 Detailed PA lookup/update pseudocode

```text
RecordFault(key, requestIsWrite, kind, now):
    setID = VPN(key) & 0xF
    line = search 4 ways for (PID, VPN, expected Epoch)
    if HIT:
        metric.PACacheHits++
        account cache-hit latency
    else:
        metric.PACacheMisses++
        choose invalid way or LRU victim
        if victim.valid and victim.dirty and victim.epoch=currentEpoch(victim.key):
            schedule CPU writeback, wait for its completion
        schedule PA-Table read (one CPU-memory access on miss)
        if backing entry exists: fill it
        else: initialize {count=0, wrote=false, epoch=current}
    line.wrote = line.wrote OR requestIsWrite OR protectionFault
    if line.count == 3:                    // arriving fault is number 4
        decision = (line.wrote ? AC : DUP)
        invalidate line; delete backing[key]
        increment epoch for this page
        metric.PATriggered++
        return (trigger=true, decision, now+modeledLatency)
    else:
        line.count++
        line.dirty = true
        update LRU
        return (trigger=false, now+modeledLatency)
```

**Ordering caveat:** this pseudocode describes logical behavior, not a Go function you should run instantaneously. For cycle simulation, PA cache lookup, backing memory request, possible dirty-victim writeback, and response must be **staged** in a component/event queue with network/DRAM contention. The paper states PA-Table accesses add memory traffic but does **not** publish a full PA-Cache hit latency table. Keep `paHitCycles`, `paMemReadCycles`, `paMemWriteCycles`, `paWayCount` as tunable knobs; use plausible values only when labeled assumptions. When `PA-Table only` ablation is selected, every fault must access backing memory (not a zero-cost Go map).

### 5.3 Integrate PA access with fault resolution

For any fault, the MMU page-table walk and PA lookup can overlap (§V-C/E). A resolution cannot finish until **both** the page translation and GRIT scheme decision are ready:

```text
decisionReadyAt = max(hostPageWalkReadyAt, PAControllerReadyAt)
faultCanResumeAt = max(decisionReadyAt, requiredPageOperationCompleteAt)
```

This is an ordering equation, not an instruction to directly modify simulator time. Schedule events using the simulator clock. A cache miss may finish behind a page walk; a PA-cache hit may be completely hidden by the page walk. Add independent measures for PA hit/miss latency, total time on the critical path, and backing PA-Table memory traffic. Test a forced slow PA lookup to verify that the page walk does actually wait when the PA result is later.

**Delete stale PA state when freeing a page**. PA-Cache is write-back: `Free` must either write back live dirty entries for accounting or invalidate them consistently, and it must not reinsert freed pages in the background.

### 5.4 Policy decision — NEW `grit/policy.go`

```go
// PROPOSED, logically complete decision rule for GRIT faults.
func DecideOnFourthFault(sawWrite bool) Scheme {
    if sawWrite { return SchemeAccessCounter }
    return SchemeDuplication
}
```

There is **no** explicit `OT` outcome on a fourth fault. OT persists when faults do not reach four; frequent sharing faults eventually favor `DUP` or `AC`. The paper's Table III includes private/producer–consumer categories for analysis, but the actual Fig. 13 decision mechanism reduces to **read bit → DUP, write bit → AC** after the threshold. Do not invent a GPU-count-based classifier unless creating a separately labeled enhancement.

After deciding: update host `PredictedScheme` (and effective mode at the requesting page-fault resolution), clear the epoch's PA entry, and enqueue neighbor analysis **only if the resulting scheme actually differs from the previously adopted scheme**. The paper specifically suppresses group checks when a triggered AC decision equals already-used AC.

## 6. Neighboring-Aware Prediction — exact grouping algorithm

Implement this in **NEW** `grit/predictor.go`. The predictor inspects policy bits, **not GPU data values**, and runs in the background after resolving the original fault. Its updates are to future host policy; they do not copy pages or flush GPU TLBs immediately (§V-D/E).

### 6.1 Alignment and groups

For 4-KB pages, group sizes are powers of eight: `1`, `8`, `64`, `512`. A group comprises **aligned consecutive VPNs**, never arbitrary nearest addresses. The paper's printed base-address equation mixes VPN and byte-address notation; use the dimensionally consistent formula:

```go
func GroupBaseVPN(vpn, groupPages uint64) uint64 {
    return (vpn / groupPages) * groupPages
}
```

For `vpn=1234`, the aligned eight-page group begins at `1232`; the aligned 64-page group begins at `1216`; the aligned 512-page group begins at `1024`. Restrict a 512-page group to one 2-MB page-table region. Do not allow groups to straddle allocation/address-space holes without defining how absent PTEs vote; the recommended strict implementation requires **all eight children to be allocated/present** before promotion (engineering assumption, not specified explicitly by paper). On group creation, write `GroupCode` only to the aligned **base-page PTE**; descendants either carry `0` or an internal membership map, but never contradict the base.

| Level | Page count | Byte span | Group code on base PTE | Children required |
|---|---:|---:|---|---:|
| Leaf | 1 | 4 KB | `00` | — |
| L1 | 8 | 32 KB | `01` | 8 × 1 |
| L2 | 64 | 256 KB | `10` | 8 × 8 |
| L3 | 512 | 2 MB | `11` | 8 × 64 |

### 6.2 Promotion vote (strict majority)

1. When a page's policy changes, inspect the **aligned** eight single-page PTEs containing that VPN.
2. If **at least five** of the eight already have the newly selected scheme (including the changed page), assign the new predicted scheme to all eight and mark an 8-page group.
3. Inspect the aligned 64-page region containing this group. If **at least five** of its eight 8-page child groups already have the new scheme and valid group codes, assign it to all 64 pages and mark the 64-page base PTE.
4. Repeat for the 512-page region, using eight 64-page groups. Stop after 512, after any failing vote, at a missing/unaligned child, or if a promotion would cross a 2-MB host page-table boundary.
5. **Only the host-predicted policy changes in background.** If an affected GPU later faults on one of these pages, its next fault resolution can adopt the predicted mode **without waiting for four faults for that page**. Avoid invalidating active GPU mappings in the prediction phase.

Suggested core algorithm (pseudocode; the `PageMetadata` store is separate from the physical directory):

```text
PromoteIfMajority(changedVPN, newScheme):
    if newScheme == oldEffectiveScheme(changedVPN): return
    for level in [8, 64, 512]:
        base = floor(changedVPN / level) * level
        childSize = level / 8
        children = [base + i*childSize for i in 0..7]
        if any child absent/unallocated: return
        if childSize > 1 and any child's group code != code(childSize): return
        matches = number of child groups whose predicted scheme == newScheme
        if matches < 5: return
        for each page in [base, base+level):
            set host predicted scheme = newScheme
        clear child base codes where subsumed by parent
        set host PTE GroupCode at base = code(level)
        metric.PredictionPromotions[level]++
```

This is the most conservative unambiguous interpretation. The paper says to compare neighboring pages/groups and use more than half; for a simulator with many unallocated neighbors, alternate rules (e.g. ignoring missing PTEs) will affect results. Keep such variants opt-in and document them distinctly.

### 6.3 Demotion / downward degradation

A page in a grouped range may reach its fourth fault and select a different scheme. The current large group must be split:

- If a **64-page** group changes at one page, demote to **eight 8-page groups**. The 8-page subgroup containing the changed page further demotes to **eight single pages** (group code `00`); the seven unaffected subgroups remain valid 8-page groups (`01`) if their schemes remain uniform.
- If a **512-page** group changes at one page, split into eight 64-page groups, recursively split the affected 64-page group into eight 8-page groups, then split only the affected 8-page group into singleton pages.
- If a page changes inside an 8-page group, that group becomes eight single pages.

Pseudocode:

```text
OnActualSchemeChange(vpn, newScheme):
    locate highest existing aligned ancestor group that contains vpn
    if ancestor exists and newScheme != ancestor.scheme:
        recursively split ancestor into eight equal children
        for every unaffected child: preserve old uniform scheme and valid code
        for the child containing vpn: split again until level=1
    set this page's effective/future scheme = newScheme
    schedule PromoteIfMajority(vpn, newScheme) in background
```

Ensure the split affects group metadata and predicted bits correctly. Never split into overlapping, nonaligned regions. Do not free/reallocate GPU frames merely because groups change. The **actual** transition for the faulting page still follows §3.4; other affected pages adopt host-predicted policies at future faults.

### 6.4 Avoid promotion/degradation oscillation

The paper gives a special rule: when the new fourth-fault decision is still **AC**, and the page already uses AC, **do not perform neighboring checks**. Example: within an eight-page AC group, three pages have switched to DUP; an AC page later triggers another AC decision. Promoting back to AC would undo the three DUP choices and create oscillations. Implement the exact `newScheme != previousScheme` guard for predictor invocation. Count suppressed checks in instrumentation.

### 6.5 Predictor correctness invariants

- Every `GroupCode` matches one of `{1,8,64,512}`.
- Group base is correctly aligned.
- Parent groups consist of exactly eight children; no overlapping active parents.
- An active group is uniform in **host-predicted** scheme.
- No group crosses PID, unmapped region, or a 2-MB page-table region.
- A background prediction changes **only** future scheme metadata, not physical owner/PAddr, GPU-local permissions or cache/TLB state.
- If predictions race with a page's migration/collapse, queue them behind the page's directory lock or version-check them before committing. Stale background work must never overwrite a newer fourth-fault decision.

## 7. Wire GRIT into MGPUSim v3's builders and driver

### 7.1 Add runtime flags — `samples/runner/flag.go`

Use the existing `flag` package and add these **PROPOSED** knobs:

```go
var gritModeFlag = flag.String("grit-mode", "off",
    "off, ot, ac, dup, or grit")
var gritFaultThresholdFlag = flag.Int("grit-fault-threshold", 4,
    "Local/protection faults per policy decision")
var gritACThresholdFlag = flag.Int("grit-ac-threshold", 256,
    "Remote cache-line accesses per 64-KB counting region")
var gritPACacheFlag = flag.Bool("grit-pa-cache", true,
    "Enable the 64-entry page-attribute cache")
var gritPredictionFlag = flag.Bool("grit-prediction", true,
    "Enable neighboring-aware prediction")
var gritStatsFlag = flag.Bool("grit-stats", true,
    "Write GRIT CSV statistics")
var gritCapacityRatioFlag = flag.Float64("grit-capacity-ratio", 0.70,
    "Per-GPU resident-frame limit, relative to configured footprint")
```

In `Runner.ParseFlag()` validate `mode ∈ {off, ot, ac, dup, grit}`, positive thresholds, and `0 < capacityRatio <= 1`. For `grit-mode != off`, require `-timing` and a **real multi-GPU** run (`-unified-gpus=1,2,3,4 -use-unified-memory`, or benchmark-specific equivalent); fail loudly rather than silently running functional-only emulation. Expose the chosen options in the CSV header. If `mode=ot`, bypass PA decision logic even though normal faults still occur; for `mode=ac|dup`, use uniform behavior without neighbor prediction; for `mode=grit`, begin OT and apply Fault-Aware decisions.

**Ablation flags:** support `PA-only`, `PA+cache`, `PA+prediction` and `GRIT-full`, or a controlled pair of booleans, to reproduce Fig. 20. Keep the distinction between *PA-table present* and *PA-cache enabled*: disabling PA-cache must still incur backing-memory requests.

### 7.2 Pass configuration through runner/platform to MMU and driver

**EXISTING** `samples/runner/runner.go::buildTimingPlatform`: pass an immutable `grit.Config` to the builder, for example `.WithGRITConfig(config)`. Add the field and method in **EXISTING** `samples/runner/timingplatform.go::R9NanoPlatformBuilder`.

**EXISTING** `R9NanoPlatformBuilder.Build()` currently does roughly:

```text
create engine → create global Storage → create Akita MMU and vm.PageTable →
build Driver → build GPUs and PCIe routes → establish route
```

Change construction to:

```text
engine
  → host canonical page table
  → GRIT directory, PA controller, prediction controller, AC counter module
  → MMU (with GRIT shared interfaces/ports; fallback legacy when off)
  → driver (receives same controllers; no duplicated state)
  → GPUs (individual local PTE/guard interfaces and RDMA counter hooks)
  → CPU/GPU + GPU/GPU networks
  → validate connectivity and start
```

The new `grit.Config` should contain page size, mode, threshold values, cache enabled, predictor enabled, per-GPU capacity, memory timing, and interconnect model. Do not create a new policy directory for every GPU: the authoritative page directory and PA-Table are **host-side shared**. Each GPU gets its own local PTEs, TLBs, local frame tracking and optional access guard.

**EXISTING** `mgpusim/driver/builder.go::Builder` should take `.WithGRIT(directory, paController, config)` and assign the resulting pointer to a new `Driver.grit` field. If you avoid an import cycle by defining `grit.Service` interfaces in `grit`, put message structs in a neutral protocol package rather than importing `driver` from `grit`.

### 7.3 Adapt the driver without breaking current migration commands

**EXISTING** `driver/driver.go::Tick` should service GRIT fault requests, progression of pending per-page operations, and responses in addition to regular command queues. Add methods such as:

```go
// PROPOSED method names and responsibilities; implement as simulator events.
func (d *Driver) processGRITFaults(now sim.VTimeInSec) bool
func (d *Driver) processACThresholds(now sim.VTimeInSec) bool
func (d *Driver) scheduleGRITPageOperation(now sim.VTimeInSec) bool
func (d *Driver) processGRITACKs(now sim.VTimeInSec) bool
func (d *Driver) sendGRITFaultResponses(now sim.VTimeInSec) bool
```

Do **not** call `d.memAllocator.Remap` or the old `preparePageForMigration` unmodified for read replication: they maintain a **single** global PAddr per VPN. Add `ReserveReplicaFrame` and `CommitMapping` methods to the allocator/directory. Tie every CPU/GPU communication to a `sim.Msg` ID and a per-page FSM. If your initial implementation serializes all faults globally, state that in the experiment methodology; it may increase runtime relative to the paper.

### 7.4 Extend existing CP/PMC messages

In **EXISTING** `mgpusim/protocol/driverprotocol.go` add, or carefully reuse, commands for:

- `ShootdownPageReq`/`ShootdownPageRsp`: PID, VPNs, **generation**, intended GPU target; no unrelated address range.
- `PageCopyReq`/`PageCopyRsp`: source GPU + physical address, destination GPU + physical address, size, `KeepSource` bool.
- `PageCollapseReq`/`PageCollapseRsp`: revocation of all read replicas and TLB/cache entries, with ACK barrier.
- `ACThresholdReq`: PID, aligned 64-KB region VPN, target GPU, hot 4-KB page, original count.
- `GRITFaultReq`/`GRITFaultRsp`: as §2, if not placed in `grit/protocol.go`.

Use the existing `protocol.NewShootdownCommand`/`ShootDownCompleteRsp` where possible for full GPU quiescence; if implementing targeted shootdowns, confirm `timing/cp/commandprocessor.go` forwards the exact VPN list and that `akita/mem/vm/tlb/tlb.go::handleTLBFlush` removes only those entries. Separate queue/ACK accounting for memory copy, TLB invalidation and cache flush. An ACK means the relevant step **completed**, not merely that a request entered a send buffer.

**EXISTING** `timing/pagemigrationcontroller/pmc.go` already reads a page from the current owner and writes it to a destination, using `numDataRspPendingForPageMigration`. Reuse its data movement for both OT and DUP by giving it source/destination physical addresses and `KeepSource`. Retaining the old source is a directory/allocator decision; the PMC merely copies bytes and reports completion. Respect its maximum requests in flight and avoid corrupting a destination with simultaneous migrations.

## 8. Model memory access and coherence correctly

### 8.1 Changes to local translation and remote routing

On **every translation response**, return the GPU-appropriate `PAddr`:

- `OT`: mapping points at the current owning GPU after migration.
- `AC`: mapping may point to another GPU's physical page; RDMA routes it to the owner.
- `DUP`: read mapping points to that GPU's local replica; writer maps to its sole writable authoritative copy after collapse.

The `vm.TranslationRsp.Page` consumed by `akita/mem/vm/tlb/tlb.go::parseBottom` is installed into a TLB. Its cached `PAddr`, permission and version can go stale. Make TLB lookup/flush honor `Generation` (or a proper invalidation handshake), and do not let a stale TLB hit circumvent protection on a cached write. A generation check on every hit is a conservative correctness mechanism; if it adds overhead not present in the paper, account for it as simulator instrumentation or replace it with proper targeted invalidation.

**EXISTING** `akita/mem/vm/addresstranslator/addresstranslator.go::createTranslatedReadReq` and `createTranslatedWriteReq` compute `addr=page.PAddr+offset` and use `lowModuleFinder.Find(addr)`. This physical-routing design supports remote memory if the returned `PAddr` really belongs to the remote GPU and the finder routes it correctly. Do **not** add a second page-copy operation just to satisfy an AC remote read. Verify both `ReadReq` and `WriteReq` go through the correct RDMA endpoint and that `Info`/`PID`/request identity are preserved for accounting.

### 8.2 Cache coherence is a separate requirement

The paper conceptually flushes caches/TLBs before migrations or write collapse. MGPUSim v3 offers `timing/cp/commandprocessor.go` shootdown and cache-flush paths, but it is not a free general-purpose multi-GPU coherent cache protocol. For research correctness:

1. Before moving **writable** data, wait for previous writes to retire and flush all dirty cache lines of its owner.
2. For `DUP`, a new local replica must copy the latest memory version, not stale global storage.
3. On write collapse, invalidate every other replica's data caches **and** translations before granting exclusive write permission.
4. On owner change, invalidate old local/remote mappings and cache lines across every GPU that could have cached them.
5. On AC remote writes, either implement necessary cross-GPU coherence/order or conservatively bypass problematic noncoherent caches for writable shared pages (labeled as an engineering approximation). Measure any extra bandwidth/latency.
6. Use single-page, targeted invalidations where safe; if v3 only supports broad drains at first, retain them but **do not compare resulting overhead to the paper as identical**.

Add debug assertions checking page contents and version before and after every transition. A performance number is invalid if the simulator reads stale data.

### 8.3 Source-data correctness when the allocator migrates

MGPUSim v3's `driver.go::preparePageForMigration` calls `AllocatePageWithGivenVAddr` and updates the global `pageTable` early. In your GRIT path:

```text
old = directory.Lookup(key)
newPAddr = allocator.ReserveFrame(destGPU)  // no canonical PTE update yet
send PMC PageCopyReq(old.OwnerPAddr → newPAddr)
wait until all memory write responses received
atomically update directory + canonical host PTE + target local PTE
invalidate stale mappings and increment page generation
release source if OT or collapse; keep source if DUP read copy
send PageFaultRsp only after required coherence ACKs
```

If migration errors, retry/abort without exposing a half-moved page. The old frame remains authoritative until commit. For CPU-backed eviction, model the CPU-side data path and store the latest version before freeing last GPU copy.

## 9. Match the paper's timing and architecture

Correct functional behavior is mandatory, but matching speedups also requires credible component latencies and resource limits. Implement the following independently of GRIT's decision logic so **all four policies** run on an equal baseline.

### 9.1 GPU / cache / TLB sizes

**EXISTING** `samples/runner/timingplatform.go::MakeR9NanoBuilder` already defaults to four GPUs, 16 shader arrays per GPU, four CUs per shader array and a 4-KB page. Preserve those. Check each cache in:

- `samples/runner/shaderarray.go::buildL1VCaches`, `buildL1SCache`, `buildL1ICache` and the `buildL1*TLB` methods.
- `samples/runner/r9nanobuilder.go::buildL2Caches`, `buildL2TLB`.

**Mismatches to fix:** v3's sample `buildL2TLB` uses `numWays := 64` and derives `numSets` from `dramSize`; the paper target is a **512-entry, 16-way L2 TLB**, so change to 16 ways × 32 sets with 10-cycle lookup. v3's shader-array L1 TLB construction includes 64-way single-set instances; the paper target is **32-entry CU-private** L1 TLBs, which may require moving/replicating the TLB within the shader array's CU wiring, not only adjusting a number. Verify L1 caching associations and sizes using the builder definitions before tuning.

Changing `numSets`, `numWays`, cache capacities and lookup delays must preserve valid simulator wiring and avoid impossible per-CU sharing. Run TLB-only microbenchmarks before adding GRIT decisions.

### 9.2 Page walker realism

**EXISTING** `akita/mem/vm/mmu/mmu.go::startWalking` sets one fixed `cycleLeft` from `WithPageWalkingLatency(100)`; this is *not* the paper's full GMMU. Extend it to a service queue with **8 concurrent walkers**, **64 queued requests**, **100 cycles per page-table level**, and **128-entry shared page-walk cache**. A simplified configurable 4-level page walk is acceptable for early correctness tests, but mark it a timing approximation until the page-walk cache, memory traffic and hit rates are modeled. Do not charge all four levels as a flat 400 cycles if the walk cache eliminates some accesses.

Page-table walk requests and PA-Table backing accesses should generate *modeled* CPU memory traffic. Ensure they contend for the appropriate memory/network service. PA access may overlap the page walk (§5.3).

### 9.3 CPU–GPU and GPU–GPU links

**EXISTING** `samples/runner/timingplatform.go::createConnection` uses an Akita `pcie.Connector` with PCIe v4 ×16 and switch latency. The paper instead assumes **PCIe v4 32 GB/s CPU↔GPU** and **NVLink-v2 300 GB/s GPU↔GPU**. A homogeneous PCIe topology is therefore not sufficient for matching the paper.

Recommended design:

1. Retain PCIe for Driver/MMU ↔ GPU control messages and host-backed migration traffic; verify achieved effective throughput in a transfer microbenchmark.
2. Add a separate GPU–GPU transfer/remote-memory network using Akita network components or an explicit modeled-bandwidth/latency route in `timing/rdma/comp.go` and PMC inter-GPU paths. Route according to **source and destination GPU**, not only `PAddr`.
3. Calibrate a 4-KB migration and sustained remote cache-line streams against 300 GB/s aggregate GPU–GPU target. Do **not** claim 300 GB/s per individual direction unless that is how the chosen model defines it; document contention and topology assumptions.
4. Ensure that OT migration, DUP page copying and AC remote requests consume the same GPU–GPU link budget. Counting bandwidth for only one policy unfairly biases comparisons.
5. Extend to 2/8/16 GPUs with a documented topology, contention and routing; run separate bandwidth sanity tests for each size.

### 9.4 Capacity and 70% footprint

**EXISTING** `samples/runner/timingplatform.go::Build` creates large global storage and GPU builders allocate physical frames. This alone does not impose the paper's capacity pressure. Provide a consistent footprint-aware allocation limit. For each benchmark instance, measure **working allocated bytes** and configure `capacityLimitBytes` per GPU according to your interpretation of the paper's 70% rule; record whether it is 70% of *whole application footprint per GPU* or 70% of *its share*. The paper gives the ratio but does not fully describe the simulator's eviction algorithm or the exact per-GPU normalization. Report the chosen formula and test it with a forced-oversubscription case.

Maintain separate counters: original pages, replica pages, allocated GPU frames, backing-store bytes, peak resident bytes, evictions and copy traffic. Never count mere page-table allocations as consuming replica DRAM capacity. On memory pressure, evict a verified clean replica first if possible; dirty-owner eviction requires writeback and retargeting all local PTEs. The paper's oversubscription trends cannot be reproduced if the allocator simply panics at capacity.

### 9.5 Thread-block scheduling

The paper fills CUs in one GPU with TBs round-robin before moving to the next GPU (§III-B). **EXISTING** `mgpusim/driver/driver.go::distributeWGToGPUs` partitions a kernel's work groups into static contiguous chunks based on CU counts, with per-GPU command processors scheduling locally. This is **not guaranteed to implement the same spill-when-full scheduler**. For close reproduction, add a `-grit-scheduler=paper|legacy` option and implement/testing the paper's assignment under current CU occupancy. Keep the same scheduler for OT/AC/DUP/GRIT comparisons; scheduling influences sharing locality and the apparent benefit of GRIT.

## 10. Benchmarks, experiment scripts and metrics

### 10.1 Benchmark inventory: paper versus v3

Paper Table II reports eight workloads and indicative footprints. The MGPUSim v3 tree contains these **candidate** sample implementations; **similar names do not prove identical kernels, inputs or multi-GPU sharing patterns**.

| Paper label | Paper workload and footprint | Existing v3 candidate | Reproduction warning |
|---|---|---|---|
| BFS | SHOC breadth-first search, 32 MB | `samples/bfs/`, `benchmarks/shoc/bfs/` | Match graph, edge distribution, BFS input and work-group layout |
| BS | AMDAPPSDK bitonic sort, 30 MB | `samples/bitonicsort/`, `benchmarks/amdappsdk/bitonicsort/` | Match array length and sort passes |
| C2D | DNN-Mark Convolution 2D, 94 MB | `samples/conv2d/`, `benchmarks/dnn/layer_benchmarks/conv2d/` | **Do not assume** this is the same DNN-Mark kernel/input |
| FIR | Hetero-Mark FIR, 155 MB | `samples/fir/`, `benchmarks/heteromark/fir/` | Match signal length, tap size and access pattern |
| GEMM | AMDAPPSDK GEMM, 16 MB | `benchmarks/dnn/gputensor/gemm.*` or custom/ported GEMM | Inspect GEMM code and input; matrix multiply is **not automatically identical** |
| MM | AMDAPPSDK matrix multiplication, 33 MB | `samples/matrixmultiplication/`, `benchmarks/amdappsdk/matrixmultiplication/` | Match X/Y/Z dimensions, matrix layout |
| SC | AMDAPPSDK simple convolution, 131 MB | `samples/simpleconvolution/`, `benchmarks/amdappsdk/simpleconvolution/` | Match image/filter dimensions |
| ST | SHOC Stencil 2D, 33 MB | `samples/stencil2d/`, `benchmarks/shoc/stencil2d/` | Match stencil shape, step count and tiling |

Before claiming replication, determine exact allocations and compare them to Table II. `samples/conv2d` and `samples/matrixmultiplication` have different defaults, so reproducing a 94-MB or 33-MB footprint requires explicit input flags. For example `samples/fir` accepts `-length`, `samples/bitonicsort` accepts `-length`, `samples/matrixmultiplication` accepts `-x -y -z`, and `samples/bfs` accepts `-node -degree` or `-load-graph`. Use each sample's `-h` output; do not blindly multiply dimensions until measured actual UVM allocation sizes agree.

Every benchmark must exercise **multiple actual GPUs** and share unified pages. The `-unified-gpus=1,2,3,4` flag creates a unified logical device, and `-use-unified-memory` calls the benchmark's `SetUnifiedMemory()` when supported (`samples/runner/runner.go`). A benchmark may nevertheless use private or explicit-copy buffers. Instrument actual `(PID, VPN, requesterGPUID)` traces to verify that the desired sharing patterns occur.

### 10.2 Modes and ablations to execute

Run the same benchmark instance, seeds, GPU count, cache/TLB/DRAM settings and simulated network for every mode:

1. Uniform `OT` (on-touch).
2. Uniform `AC` (256 remote accesses / 64-KB counter region).
3. Uniform `DUP` (read replication + write collapse).
4. `GRIT` with PA-Table only, without PA-Cache or prediction.
5. `GRIT` with PA-Table + PA-Cache, without prediction.
6. `GRIT` with PA-Table + neighbor prediction, without PA-Cache.
7. `GRIT` full (PA-Table + PA-Cache + prediction).

For sensitivity studies: fault thresholds `{2,4,8,16}` (Fig. 21); GPU counts `{2,4,8,16}` with **fixed application input size** (Figs. 22–24); 2-MB pages with **larger input footprints** (Fig. 25). With a 2-MB page, do not also use 4-KB grouping assumptions without explicitly changing the model: 512×4-KB predictor regions and one 2-MB hardware page are fundamentally different. Implement large-page support after 4-KB GRIT is verified.

**Warning on `-magic-memory-copy`:** the flag bypasses modeled data-transfer timing in v3; **do not enable it in paper-reproduction runs**. Use it only as a functional diagnostic. Otherwise migration/duplication may appear artificially cheap.

### 10.3 Add metrics — `grit/metrics.go` and `samples/runner/report.go`

Count these at the exact event that *completes*, and include PID/GPU breakdowns where useful:

| CSV field | Definition |
|---|---|
| `kernel_time_ns` | Simulated kernel completion time (from existing timing tracer) |
| `total_cycles` | Completion time × configured GPU frequency |
| `local_page_faults` | Unique/coalesced local GPU page-fault events admitted to UVM |
| `protection_faults` | Writes to read-only local pages/replicas |
| `grit_fault_triggers` | Fault threshold crossings |
| `scheme_ot_to_ac`, `scheme_ot_to_dup`, etc. | Completed effective transitions by old/new scheme |
| `scheme_unchanged_decisions` | Threshold reached but selected mode already active |
| `pa_cache_hits`, `pa_cache_misses`, `pa_writebacks` | Logical PA cache events |
| `pa_backing_reads`, `pa_backing_writes` | Modeled CPU memory accesses |
| `pa_stall_cycles` | Fault response critical-path delay attributable to PA operations |
| `ac_remote_accesses` | Actual remote cache-line transactions in AC mode |
| `ac_threshold_interrupts` | 256-count crossings accepted, after deduplication |
| `page_migrations`, `page_copies`, `write_collapses` | Successful completed operations |
| `bytes_migrated`, `bytes_duplicated`, `bytes_remote_read`, `bytes_remote_write` | Actual data traffic by cause |
| `tlb_invalidations`, `cache_flushes` | Completed invalidation/flush actions |
| `predict_promote_8`, `predict_promote_64`, `predict_promote_512` | Completed promotions |
| `predict_demote_8`, `predict_demote_64`, `predict_demote_512` | Completed demotions |
| `predict_suppressed_ac_no_change` | Group checks intentionally skipped |
| `predicted_scheme_used_on_fault` | Next fault actually applies host-side prediction |
| `frames_used_peak`, `evictions`, `swap_bytes` | Capacity and oversubscription |
| `stall_time_fault`, `stall_time_migration`, `stall_time_remote`, `stall_time_collapse` | Latency breakdown for Fig. 3-like analyses |

For Fig. 19-style scheme breakdown, the paper counts the fraction of **L2-TLB-missing accesses** served under each mode, not simply the number of currently assigned pages. Instrument an event on **L2 TLB misses after the effective mode is known** and aggregate `OT/AC/DUP` proportions. A single page may contribute to several modes at different times.

Implement `metricsCollector`/tracing integration in **EXISTING** `samples/runner/report.go::defineMetrics`, `reportStats` and `dumpMetrics`, and add an optional `-grit-trace` output of `(simTime,pid,vpn,gpu,event,oldScheme,newScheme,owner,version)`. Keep standard report flags working when GRIT is off.

**Comparison formula:** if `T_OT` is baseline elapsed simulated time and `T_X` is the time of another policy, throughput speedup is `S = T_OT / T_X`. Improvement is `(S-1)*100%`. Do **not** compute `(T_OT-T_X)/T_OT*100%` and label it a speedup; that is a time-reduction percentage.

### 10.4 Paper targets are reference points, not test assertions

The paper reports average GRIT performance improvements over uniform OT, AC and DUP of approximately **60%, 49% and 29%** in its default experiment (§VI-A). It also reports average page-fault reductions of **39%, 55% and 16%** compared with those three schemes in the main results (§VI-A, Fig. 18). For 4 as the fault threshold, speedup over OT is 60%; thresholds 2/8/16 yield 53%/59%/48% (§VI-B1). It reports an average 23% improvement versus OT when using 2-MB pages in the large-input sensitivity study (§VI-B3).

These **are not acceptance criteria** for a fresh fork with different benchmark instances or interconnect modeling. First establish *functional* matches to each algorithmic invariant, then compare directions/trends, then perform paper-specific timing calibration. A GRIT result that is faster but returns wrong data is a failure.

## 11. Test plan: test before running full applications

Create focused Go tests in **NEW** `grit/*_test.go`; integration tests in `driver/*_test.go`, Akita `mem/vm/mmu/*_test.go`, TLB/cache tests, and multi-GPU microbenchmarks under a **NEW** `samples/gritmicro/` folder. Existing `go test ./...` suites must remain green in **`-grit-mode=off`**.

### 11.1 Essential deterministic unit tests

| ID | Test setup | Required observable result |
|---|---|---|
| U01 | Fault stream R,R,R,R on one page | Fourth fault selects DUP, then PA entry disappears |
| U02 | Fault stream R,R,W,R | Fourth selects AC; sticky write remains set |
| U03 | 3 faults then 1 fault | Trigger **only** on fourth; no 2-bit wrap |
| U04 | One fault after an epoch reset | New count=1 and new write flag |
| U05 | Same VPN, two different PIDs | Two independent PA entries |
| U06 | VPNs with same low 4 index bits | Correct 16-set collisions and 4-way LRU |
| U07 | Five colliding keys in same cache set | Dirty LRU victim written back; refilled state correct |
| U08 | 64 distinct keys | Exactly 64 populated cache lines; no accidental direct backing updates on hits |
| U09 | 8 aligned pages, exactly 4 have target scheme | **No** group promotion |
| U10 | 8 aligned pages, exactly 5 have target scheme | Promote to aligned 8-page group |
| U11 | Eight aligned 8-page groups, five match | Promote to 64 pages, then similarly 512 |
| U12 | One page changes inside 64-page group | Seven intact 8-page groups, changed subgroup splits to singles |
| U13 | One page changes inside 512-page group | Recursive split only on affected branch |
| U14 | AC decision unchanged in an AC group | No neighbor check, no oscillating promotion |
| U15 | Group near 2-MB boundary | No promotion across region or allocation hole |
| U16 | Prediction updates host PTE | Existing GPU mapping untouched until next real fault |
| U17 | Concurrent faults for one page | Correct coalescing, no double counting/copying |
| U18 | PA victim writeback after epoch deletion | No stale PA state reappears |

### 11.2 Policy and data-integrity tests

| ID | Event trace | Expected |
|---|---|---|
| P01 | GPU1 writes X=7; GPU2 OT reads; GPU1 OT reads | Two transfers; both read 7; final one owner |
| P02 | GPU1 owns page; GPU2 AC reads 300 remote lines | First 255 remote accesses do not migrate; trigger at 256; values correct |
| P03 | GPU2 AC maps remote PTE; its L2 TLB misses repeatedly | No false local-page-fault increment after valid local PTE exists |
| P04 | GPU1 writes X=1; GPU2/GPU3 DUP read; GPU2 writes X=2 | Protection fault; collapse; no stale reads; GPU3 next read yields 2 |
| P05 | DUP → AC transition with three replicas | Old replicas removed and invalidated; one authoritative owner; remote access works |
| P06 | DUP → OT transition during outstanding load | Load retires or is safely retried; no premature frame release |
| P07 | Two GPUs write same page nearly simultaneously | Serial ownership; no two writable valid replicas |
| P08 | Cache hit attempts write to read-only replica | Fault **before** any data changes |
| P09 | Migration while another GPU holds stale TLB | Stale translation invalidated; retried access uses new generation/PAddr |
| P10 | Memory full, read replica requested | Safe eviction/backing; no allocator panic or data loss |
| P11 | Page freed while PA cache line dirty | PA/directory state removed; later allocation cannot inherit old policy |
| P12 | Page transfer partially complete, another read arrives | Reader stalls; cannot read uninitialized destination frame |
| P13 | Local TLB miss but valid local PTE | **No GRIT fault** (even if TLB refills) |
| P14 | Forced PA-cache miss slower than page walk | Fault response waits for PA result; modeled timing grows |

Use deterministic two-/three-GPU memory kernels that write known integers and read them back. Inspect actual memory bytes through a debug-only reference path **after all in-flight writes drain**. Forbid a test from passing based only on a counter or an absence of simulator panics.

### 11.3 Integration and performance sanity tests

1. **Legacy off:** compare `go test ./...` and one timed benchmark with a clean v3.0.3 baseline. GRIT disabled should not change observed outputs; any timing change must be explained.
2. **Uniform OT:** repeatedly access one shared page alternately across two GPUs; migrations, bytes and owner changes should match the access sequence.
3. **Uniform AC:** repeated remote cache misses should have nonzero RDMA bytes and fewer migrations than OT for a ping-pong workload; verify threshold timing.
4. **Uniform DUP:** read-mostly sharing should create replicas and few collapses; write-heavy sharing should create collapses and invalidate others.
5. **Mixed pages:** one private write-heavy page and one shared read-mostly page. GRIT should usually retain OT for private and move toward DUP for read-shared after fault threshold.
6. **Shared write-heavy:** GRIT should eventually choose AC when the fourth fault epoch includes a write/protection event.
7. **Neighbor pattern:** allocate 512 consecutive VPNs, change five of eight aligned children, verify promotion and next-fault policy use without immediate physical copy.
8. **Oversubscription:** force GPU resident frame demand above budget; no out-of-memory panic, deterministic evictions, correct data and accounted transfer traffic.
9. **Network:** verify transfers consume modeled bandwidth and that heavy concurrent copies serialize/contend as configured.
10. **Scalability:** run 2, 4, 8 and 16 GPUs with the same input, checking total virtual memory footprint stays fixed when required by the paper.

### 11.4 Suggested developer commands

The commands below assume a **completed implementation**; `-grit-*` flags do not exist in unmodified v3.

```bash
cd mgpusim
go test ./grit/...
go test ./driver/... ./timing/...
go test ./...

cd ../akita
go test ./mem/vm/... ./mem/cache/...

cd ../mgpusim/samples/fir
go build -o fir-grit
./fir-grit -h
./fir-grit -timing -unified-gpus=1,2,3,4 -use-unified-memory \
  -grit-mode=ot -length=4096 -report-all
./fir-grit -timing -unified-gpus=1,2,3,4 -use-unified-memory \
  -grit-mode=ac -length=4096 -report-all
./fir-grit -timing -unified-gpus=1,2,3,4 -use-unified-memory \
  -grit-mode=dup -length=4096 -report-all
./fir-grit -timing -unified-gpus=1,2,3,4 -use-unified-memory \
  -grit-mode=grit -grit-fault-threshold=4 -grit-ac-threshold=256 \
  -grit-pa-cache=true -grit-prediction=true -length=4096 -report-all
```

Avoid running four experiments into the **same CSV filename**: set each run's `-metric-file-name` to a distinct name or directory. For full reproduction, replace `-length=4096` with sizes that match the paper footprint, and use a sufficiently long simulation; this small size is for integration smoke testing only.

## 12. Suggested implementation order and completion gates

**Do not skip the gates.** Later GRIT components depend on correct earlier memory semantics.

| Order | Deliverable | Primary files | Gate to continue |
|---|---|---|---|
| 0 | Clean pinned v3 + Akita build | `go.mod`, `go.work` | Baseline tests and one benchmark work |
| 1 | Directory and per-GPU mappings | `grit/types.go`, `directory.go`; Akita `pagetable.go`; allocator | PID isolation; owner/replica invariants |
| 2 | True local/protection faults and guard | Akita `protocol.go`, `mmu.go`, `tlb.go`, `addresstranslator.go`, cache front ends | TLB misses are not counted as faults; cached writes protected |
| 3 | OT uniform policy | Driver, CP, PMC, allocator | P01/P09/P12 pass |
| 4 | AC uniform policy | RDMA, `grit/counters.go`, driver | P02/P03 and coherent remote writes pass |
| 5 | DUP uniform policy | PMC, cache guard, allocator, driver | P04/P06/P08 pass |
| 6 | Cross-policy transitions | `grit/policy.go`, driver | P05/P07 pass; all nine transitions safe |
| 7 | Fault threshold + PA-Table | `grit/pa_table.go`, driver/MMU | U01–U05 and U18 pass |
| 8 | Timed PA-Cache | `grit/pa_table.go`, PA memory path | U06–U08, P14 pass; writeback/latency modeled |
| 9 | Predictor | `grit/predictor.go`, host policy PTE | U09–U16 pass |
| 10 | Network, cache/TLB, memory budget | Runner builders, Akita MMU/cache, allocator | Per-link bandwidth, sizes, oversubscription validated |
| 11 | Reporter + ablation switches | `samples/runner/*.go`, `grit/metrics.go` | Eight counters/latencies cross-check against event traces |
| 12 | Eight workloads and all sensitivity runs | Sample benchmarks and scripts | All runs correct, comparable, metadata recorded |

### One-page acceptance checklist

- [ ] Source releases are pinned; baseline behavior still works with GRIT off.
- [ ] PID/VPN/GPU mapping is unambiguous; unified GPU ID not confused with physical GPU IDs.
- [ ] L1 and L2 TLB misses are not treated as local-page faults.
- [ ] Every write to a read-only DUP replica faults **even if it hits in cache**.
- [ ] OT moves full pages and does not expose destination until copy completion.
- [ ] AC remote access actually traverses RDMA; 256-count trigger is separate from four-fault trigger.
- [ ] DUP creates independent frames, invalidates on write and preserves latest data.
- [ ] Duplicate-to-other transitions safely collapse existing replicas.
- [ ] PA-Table entries reset after fourth fault; PA-Cache uses 16 sets × 4 ways, LRU/write-back/write-allocate.
- [ ] Fault decision is **read → DUP** or **any write → AC**; private pages retain OT without repeated faults.
- [ ] Predictor uses aligned 1/8/64/512 groups, >50% vote and correct recursive demotion.
- [ ] Background group changes do **not** migrate/invalidate active GPU mappings immediately.
- [ ] PA-Table/cache traffic and overlap with page walks are timed.
- [ ] GPU memory capacity and eviction are enforced; no silent over-allocation.
- [ ] GPU–GPU and CPU–GPU networks are separately calibrated.
- [ ] Benchmarks actually share UVM pages across GPUs and produce correct outputs.
- [ ] All mode comparisons use identical hardware/input/scheduling settings.
- [ ] Performance report distinguishes *speedup* from *elapsed-time reduction*.

## 13. Known pitfalls and explicit deviations to record in your paper/report

1. **Akita `IsPinned`:** after standard v3 migration, `processMigrationReturn` sets `IsPinned=true`; retaining that behavior will silently suppress future migrations in GRIT's OT/AC policies.
2. **MMU host table:** `vm.PageTable` is global. Do not assume it directly represents per-GPU PTE residency/permissions or multiple replica physical addresses.
3. **Miss versus fault:** a cache/TLB miss alone is not a host page fault and must not increment the PA fault counter.
4. **Single PAddr update:** using `AllocatePageWithGivenVAddr` unmodified for DUP destroys the distinction between copies. It is only safe for single-owner migration after staged commit.
5. **Write hit:** guarding only translations misses protection faults on cache hits, invalidating DUP semantics.
6. **False 4-count rollover:** counter bits are 2 bits; trigger before 3 overflows.
7. **Counter granularity:** AC's 64-KB counter region is not GRIT's 4-KB PA record or the predictor's 32-KB initial group.
8. **Source data:** early page-table update before data-copy completion can read incomplete destination data.
9. **Global ACK counters:** uncorrelated concurrent faults can make a driver restart the wrong page or GPU.
10. **Coherence:** cache flushes, write permissions and page migration must be ordered. `mem.Storage` does not automatically implement distributed cache coherence.
11. **PTE bit claims:** Go `vm.Page` does not contain literal host PTE bits 9–10 or 52–53. Model logical fields before proposing any packed encoding.
12. **Physical topology:** v3 uses PCIe by default; calling the run 'NVLink' without modeling its bandwidth/contension changes the experiment.
13. **Application equivalence:** `samples/conv2d` is not necessarily the original DNN-Mark Convolution 2D, and a generic GEMM sample is not necessarily AMDAPPSDK GEMM.
14. **Large pages:** one 2-MB hardware page cannot support independent policy for its internal 512 4-KB segments; false sharing will lower benefit, as discussed in the paper.
15. **Scheduling:** original v3 work-group distribution is not proven identical to the paper's occupancy-driven TB scheduling.
16. **PA cache accounting:** 352 bytes is a logical tag/data estimate excluding valid/dirty/LRU/PID metadata and actual Go storage.
17. **Ambiguous paper details:** migration target selection on a 64-KB AC counter threshold, PA exact access latency, and eviction policy require declared engineering assumptions.
18. **Scientific honesty:** do not label code pseudocode in this document as a tested drop-in patch; an implementation must be compiled and verified in the target environment.

## 14. Source references and traceability

**Primary paper**

- Y. Wang et al., [GRIT: Enhancing Multi-GPU Performance with Fine-Grained Dynamic Page Placement](https://doi.org/10.1109/HPCA57654.2024.00085), *2024 IEEE International Symposium on High-Performance Computer Architecture*, pp. 1080–1094. The user-supplied 15-page PDF was used for the section/table/figure annotations above. A [public authors' PDF](https://libingyao.github.io/assets/files/GRIT_HPCA24.pdf) is also available.

**MGPUSim and Akita source versions inspected**

- [MGPUSim v3.0.3 tag](https://github.com/sarchlab/mgpusim/tree/v3.0.3) — exact commit `5d49f873df5279182500c6878935c3207a013e59`. See [driver/driver.go](https://github.com/sarchlab/mgpusim/blob/v3.0.3/driver/driver.go), [driver/internal/memoryallocator.go](https://github.com/sarchlab/mgpusim/blob/v3.0.3/driver/internal/memoryallocator.go), [timing/pagemigrationcontroller/pmc.go](https://github.com/sarchlab/mgpusim/blob/v3.0.3/timing/pagemigrationcontroller/pmc.go), [timing/rdma/comp.go](https://github.com/sarchlab/mgpusim/blob/v3.0.3/timing/rdma/comp.go), [samples/runner/timingplatform.go](https://github.com/sarchlab/mgpusim/blob/v3.0.3/samples/runner/timingplatform.go) and [samples/runner/r9nanobuilder.go](https://github.com/sarchlab/mgpusim/blob/v3.0.3/samples/runner/r9nanobuilder.go).
- [Akita v3.0.0 tag](https://github.com/sarchlab/akita/tree/v3.0.0) — exact commit `11d3f6cae3645bb796fd52c357c5492f0611f3d4`. See [mem/vm/mmu/mmu.go](https://github.com/sarchlab/akita/blob/v3.0.0/mem/vm/mmu/mmu.go), [mem/vm/pagetable.go](https://github.com/sarchlab/akita/blob/v3.0.0/mem/vm/pagetable.go), [mem/vm/tlb/tlb.go](https://github.com/sarchlab/akita/blob/v3.0.0/mem/vm/tlb/tlb.go), and [mem/vm/addresstranslator/addresstranslator.go](https://github.com/sarchlab/akita/blob/v3.0.0/mem/vm/addresstranslator/addresstranslator.go).
- [MGPUSim version history](https://github.com/sarchlab/mgpusim/releases) and [Akita/MGPUSim documentation](https://akitasim.dev/docs/mgpusim/getting_started/). Note that currently maintained documentation may describe **v4**, which uses different paths and is **not** this guide's target.

---

**Deliverable meaning:** This document is a detailed **implementation plan and source map** for GRIT on pinned MGPUSim v3.0.3/Akita v3.0.0, including proposed interfaces, necessary edits, state machines, algorithms, tests, and reproduction conditions. It is **not** an already-implemented or compiled simulator branch. The first executable coding task is implementing the `grit` page directory and the Akita access/fault plumbing while keeping GRIT-off behavior unchanged.




