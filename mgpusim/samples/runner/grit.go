package runner

import (
	"flag"
	"github.com/sarchlab/akita/v3/mem/mem"
	"github.com/sarchlab/akita/v3/mem/vm/addresstranslator"
	"github.com/sarchlab/akita/v3/sim"
	"github.com/sarchlab/mgpusim/v3/grit"
	gritruntime "github.com/sarchlab/mgpusim/v3/grit/runtime"
	"math"
	"os"
)

var gritFootprint = flag.Uint64("grit-footprint-bytes", 0, "Application footprint in bytes; sets per-GPU resident budget using -grit-capacity-ratio")
var gritRatio = flag.Float64("grit-capacity-ratio", .70, "Resident capacity / application footprint when -grit-footprint-bytes is provided")
var gritPeerBandwidth = flag.Float64("grit-peer-bandwidth-gbps", 300, "Aggregate nominal peer-link bandwidth per GPU, decimal GB/s")
var gritHostBandwidth = flag.Float64("grit-host-bandwidth-gbps", 32, "Nominal host-link bandwidth, decimal GB/s")
var gritPAMemory = flag.Uint64("grit-pa-memory-cycles", 100, "Fixed backing PA-table access latency in 1-GHz cycles")
var gritPACycles = flag.Uint64("grit-pa-cache-cycles", 1, "PA-cache lookup latency in 1-GHz cycles")
var gritTrace = flag.String("grit-trace", "", "Write GRIT transition events as CSV")
var gritResidentPages = flag.Uint64("grit-resident-pages", 0, "Maximum resident 4-KB managed frames per GPU (0 uses physical capacity)")
var gritMode = flag.String("grit-mode", "off", "Page placement: off, ot, ac, dup, grit")
var gritFaultThreshold = flag.Uint("grit-fault-threshold", 4, "Real local/protection faults per decision epoch")
var gritACThreshold = flag.Uint("grit-ac-threshold", 256, "Remote cache-line accesses per 64-KB counting region")
var gritPACache = flag.Bool("grit-pa-cache", true, "Enable the 16-set, four-way PA cache")
var gritPrediction = flag.Bool("grit-prediction", true, "Enable neighboring-aware policy prediction")

func gritConfiguration() grit.Config {
	c := grit.DefaultConfig()
	c.Mode = grit.Mode(*gritMode)
	c.ResidentPages = *gritResidentPages
	c.FootprintBytes = *gritFootprint
	c.CapacityRatio = *gritRatio
	if math.IsNaN(c.CapacityRatio) || math.IsInf(c.CapacityRatio, 0) {
		panic("capacity ratio must be finite")
	}
	if c.FootprintBytes > 0 {
		if c.ResidentPages > 0 {
			panic("choose either -grit-resident-pages or -grit-footprint-bytes")
		}
		c.ResidentPages = uint64(float64(c.FootprintBytes) * c.CapacityRatio / 4096)
		if c.ResidentPages == 0 {
			panic("GRIT resident budget must hold at least one 4-KB page")
		}
	}
	for _, bw := range []float64{*gritPeerBandwidth, *gritHostBandwidth} {
		if bw <= 0 || math.IsNaN(bw) || math.IsInf(bw, 0) || bw > float64(^uint64(0))/1e9 {
			panic("GRIT link bandwidth must be finite and positive")
		}
	}
	c.PeerBandwidth = uint64(*gritPeerBandwidth * 1e9)
	c.HostBandwidth = uint64(*gritHostBandwidth * 1e9)
	c.PACacheCycles = *gritPACycles
	c.PAMemoryCycles = *gritPAMemory
	if *gritFaultThreshold > uint(^uint32(0)) || *gritACThreshold > uint(^uint32(0)) {
		panic("GRIT threshold exceeds uint32")
	}
	c.FaultThreshold = uint32(*gritFaultThreshold)
	c.ACThreshold = uint32(*gritACThreshold)
	c.PACache = *gritPACache
	c.Prediction = *gritPrediction
	if err := c.Validate(); err != nil {
		panic(err)
	}
	if c.Mode != grit.ModeOff {
		if !*timingFlag || !*useUnifiedMemoryFlag || *unifiedGPUFlag == "" {
			panic("GRIT requires -timing -use-unified-memory -unified-gpus=1,2 (or more GPUs)")
		}
		if *parallelFlag {
			panic("GRIT transition coordinator currently requires the serial event engine")
		}
		if *magicMemoryCopy {
			panic("GRIT requires modeled memory copies; disable -magic-memory-copy")
		}
	}
	return c
}
func (b R9NanoPlatformBuilder) WithGRITConfig(c grit.Config) R9NanoPlatformBuilder {
	b.gritConfig = c
	return b
}
func (b *R9NanoGPUBuilder) connectGRIT() {
	r := b.gritRuntime
	if r == nil {
		return
	}
	g := gritruntime.GPU{MemPort: sim.NewLimitNumMsgPort(r, 512, b.gpuName+".GRITMemory"), Port: sim.NewLimitNumMsgPort(r, 512, b.gpuName+".GRIT"), L1: append(append([]sim.Port{}, b.cp.L1VCaches...), b.cp.L1SCaches...), L2: b.cp.L2Caches, PMCControl: b.cp.PMC, PMCRemote: b.pageMigrationController.GetPortByName("Remote")}
	r.AddPort(b.gpuName, g.Port)
	b.internalConn.PlugIn(g.Port, 512)
	r.GPUs = append(r.GPUs, g)
	ats := append(append(append([]*addresstranslator.AddressTranslator{}, b.l1vAddrTrans...), b.l1sAddrTrans...), b.l1iAddrTrans...)
	for _, at := range ats {
		at.ManagedMemory = r
		r.Wake = append(r.Wake, at.TickLater)
	}
	for _, t := range b.l1vTLBs {
		t.ManagedMemory = r
	}
	for _, t := range b.l1sTLBs {
		t.ManagedMemory = r
	}
	for _, t := range b.l1iTLBs {
		t.ManagedMemory = r
	}
	for _, t := range b.l2TLBs {
		t.ReportManagedMiss = true
		t.ManagedMemory = r
	}
	b.rdmaEngine.OnRemoteAccess = func(now sim.VTimeInSec, req mem.AccessReq) { r.Remote(now, b.gpuID, req) }
}

func (r *Runner) reportGRIT() {
	if r.platform.GRIT == nil {
		return
	}
	path := *filenameFlag + ".grit.json"
	out, err := os.Create(path)
	if err != nil {
		panic(err)
	}
	err = r.platform.GRIT.WriteReport(out, float64(r.kernelTimeCounter.BusyTime()))
	closeErr := out.Close()
	if err != nil {
		panic(err)
	}
	if closeErr != nil {
		panic(closeErr)
	}
	report := r.platform.GRIT.Report(float64(r.kernelTimeCounter.BusyTime()))
	for name, value := range map[string]float64{"local_page_faults": float64(report.Faults.LocalPageFaults), "protection_faults": float64(report.Faults.ProtectionFaults), "grit_fault_triggers": float64(report.Faults.FaultTriggers), "page_migrations": float64(report.Placement.Migrations), "page_copies": float64(report.Placement.Copies), "write_collapses": float64(report.Placement.Collapses), "ac_remote_accesses": float64(report.Placement.RemoteAccesses), "bytes_migrated": float64(report.Placement.BytesMigrated), "bytes_duplicated": float64(report.Placement.BytesDuplicated), "evictions": float64(report.Evictions), "swap_bytes": float64(report.SwapBytes)} {
		r.metricsCollector.Collect("GRIT", name, value)
	}
}
