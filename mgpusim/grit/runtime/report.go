package runtime

import (
	"encoding/csv"
	"encoding/json"
	"fmt"
	"github.com/sarchlab/akita/v3/sim"
	"github.com/sarchlab/mgpusim/v3/grit"
	"io"
)

type Report struct {
	GPUs                  int                  `json:"gpus"`
	FramesUsedPeak        uint64               `json:"frames_used_peak"`
	ACThresholdInterrupts uint64               `json:"ac_threshold_interrupts"`
	Config                grit.Config          `json:"config"`
	KernelTimeNS          float64              `json:"kernel_time_ns"`
	TotalCycles           float64              `json:"total_cycles"`
	Faults                grit.Metrics         `json:"faults"`
	Placement             grit.Metrics         `json:"placement"`
	PA                    grit.PAStats         `json:"pa"`
	Predictor             grit.PredictionStats `json:"predictor"`
	CacheFlushes          uint64               `json:"cache_flushes"`
	Evictions             uint64               `json:"evictions"`
	SwapBytes             uint64               `json:"swap_bytes"`
	L2TLBMisses           [4]uint64            `json:"l2_tlb_missing_accesses_by_scheme"`
	ActiveRequests        int                  `json:"active_requests"`
	PendingOperations     int                  `json:"pending_operations"`
	Model                 string               `json:"model"`
}

func (r *Runtime) Report(kernelSeconds float64) Report {
	r.mu.Lock()
	defer r.mu.Unlock()
	pending := len(r.counters)
	if r.pending != nil {
		pending++
	}
	return Report{GPUs: len(r.GPUs), FramesUsedPeak: r.FramesPeak, ACThresholdInterrupts: r.CounterInterrupts, Config: r.Controller.Config, KernelTimeNS: kernelSeconds * 1e9, TotalCycles: kernelSeconds * 1e9, Faults: r.Controller.Metrics(), Placement: r.Total, PA: r.Controller.PA.Stats(), Predictor: r.Controller.Predictor.Stats(), CacheFlushes: r.Flushes, Evictions: r.Evictions, SwapBytes: r.SwapBytes, L2TLBMisses: r.L2Misses, ActiveRequests: len(r.active), PendingOperations: pending, Model: "globally serialized placement; conservative whole-cache invalidation; shared remote-write coherence; 8 walkers/GPU, 4 levels, 100 cycles/level, 128-entry intermediate walk cache; ideal host backing after timed PCIe transfer"}
}
func (r *Runtime) WriteReport(out io.Writer, kernelSeconds float64) error {
	e := json.NewEncoder(out)
	e.SetIndent("", "  ")
	return e.Encode(r.Report(kernelSeconds))
}
func (r *Runtime) SetTrace(out io.Writer) {
	r.trace = csv.NewWriter(out)
	r.trace.Write([]string{"sim_time_s", "pid", "vpn", "gpu", "event", "old_scheme", "new_scheme", "owner", "generation"})
	r.trace.Flush()
}
func (r *Runtime) emit(now sim.VTimeInSec, w *work, event string) {
	if r.trace == nil {
		return
	}
	s, _ := r.Controller.Directory.Lookup(w.key)
	old, next := s.Scheme, s.Scheme
	if w.plan.After.Scheme.Valid() {
		old = w.plan.Before.Scheme
		next = w.plan.After.Scheme
	}
	err := r.trace.Write([]string{fmt.Sprintf("%.12g", now), fmt.Sprint(w.key.PID), fmt.Sprint(w.key.VPN), fmt.Sprint(w.gpu), event, fmt.Sprint(old), fmt.Sprint(next), fmt.Sprint(s.HomeGPU), fmt.Sprint(s.Version)})
	r.trace.Flush()
	if err != nil {
		panic(err)
	}
	if err = r.trace.Error(); err != nil {
		panic(err)
	}
}
func (r *Runtime) TranslationMiss(pid uint32, addr, gpu uint64) {
	r.mu.Lock()
	defer r.mu.Unlock()
	s, ok := r.Controller.Directory.Lookup(grit.PageKey{PID: pid, VPN: addr >> 12})
	if ok {
		r.L2Misses[s.Scheme]++
	}
}
