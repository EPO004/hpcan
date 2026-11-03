// Package runtime connects GRIT policy to Akita's real translation, cache, RDMA
// and page-migration paths. Placement transitions serialize globally, while normal
// OT/DUP accesses retain the existing cache hierarchy and memory parallelism.
package runtime

import (
	"encoding/csv"
	"fmt"
	"sync"

	"github.com/sarchlab/akita/v3/mem/cache"
	"github.com/sarchlab/akita/v3/mem/mem"
	"github.com/sarchlab/akita/v3/mem/vm"
	"github.com/sarchlab/akita/v3/sim"
	"github.com/sarchlab/mgpusim/v3/grit"
	"github.com/sarchlab/mgpusim/v3/timing/pagemigrationcontroller"
)

type Allocator interface {
	ReserveGRITFrame(uint64) (uint64, bool)
	ReleaseGRITFrame(uint64)
}
type GPU struct {
	MemPort               sim.Port
	Memory                mem.LowModuleFinder
	Port                  sim.Port
	L1, L2                []sim.Port
	PMCControl, PMCRemote sim.Port
}
type lease struct {
	key            grit.PageKey
	gpu            uint64
	write, managed bool
}
type work struct {
	restore                   []byte
	id                        string
	key                       grit.PageKey
	gpu                       uint64
	write, counter, coherence bool
	phase                     int
	until                     sim.VTimeInSec
	plan                      grit.PlacementPlan
	admitted                  bool
}
type predictionWork struct {
	key    grit.PageKey
	scheme grit.Scheme
}
type Runtime struct {
	FramesPeak, CounterInterrupts uint64
	predictions                   []predictionWork
	backing                       map[grit.PageKey]uint64
	freeHost                      []uint64
	trace                         *csv.Writer
	L2Misses                      [4]uint64
	Storage                       *mem.Storage
	BudgetPages                   uint64
	HostBandwidth                 float64
	Evictions, SwapBytes          uint64
	evict                         *eviction
	readOffsets                   map[string]uint64
	used                          map[grit.PageKey]uint64
	clock                         uint64
	*sim.TickingComponent
	mu         sync.Mutex
	Controller *grit.Controller
	Table      vm.PageTable
	Allocator  Allocator
	GPUs       []GPU
	Wake       []func(sim.VTimeInSec)
	active     map[string]lease
	perPage    map[grit.PageKey]int
	pending    *work
	out        []sim.Msg
	waiting    map[string]sim.Port
	byPhysical map[uint64]grit.PageKey
	lastWrite  map[grit.PageKey]uint64
	tickets    map[string]bool
	counters   []work
	Total      grit.Metrics
	Flushes    uint64
}

func New(engine sim.Engine, cfg grit.Config, table vm.PageTable, allocator Allocator) *Runtime {
	c, err := grit.NewController(cfg)
	if err != nil {
		panic(err)
	}
	r := &Runtime{backing: map[grit.PageKey]uint64{}, HostBandwidth: float64(cfg.HostBandwidth), BudgetPages: cfg.ResidentPages, readOffsets: map[string]uint64{}, used: map[grit.PageKey]uint64{}, Controller: c, Table: table, Allocator: allocator, active: map[string]lease{}, perPage: map[grit.PageKey]int{}, waiting: map[string]sim.Port{}, byPhysical: map[uint64]grit.PageKey{}, lastWrite: map[grit.PageKey]uint64{}, tickets: map[string]bool{}}
	r.TickingComponent = sim.NewTickingComponent("GRIT", engine, sim.GHz, r)
	return r
}
func key(pid vm.PID, addr uint64) grit.PageKey {
	return grit.PageKey{PID: uint32(pid), VPN: addr >> 12}
}
func (r *Runtime) register(p vm.Page) {
	k := key(p.PID, p.VAddr)
	if _, ok := r.Controller.Directory.Lookup(k); ok {
		return
	}
	if p.HostBacked {
		if err := r.Controller.AllocateBacking(k, p.PAddr); err != nil {
			panic(err)
		}
		return
	}
	if err := r.Controller.Allocate(k, p.DeviceID, p.PAddr); err != nil {
		panic(err)
	}
	// A backing frame does not imply an installed GPU-local PTE.
	r.Controller.Directory.Unmap(k, p.DeviceID)
	r.byPhysical[p.PAddr] = k
	if uint64(len(r.byPhysical)) > r.FramesPeak {
		r.FramesPeak = uint64(len(r.byPhysical))
	}
}
func (r *Runtime) Lookup(pid vm.PID, addr, gpu uint64) (vm.Page, bool) {
	r.mu.Lock()
	defer r.mu.Unlock()
	p, ok := r.Table.Find(pid, addr)
	if !ok || !p.Unified {
		return vm.Page{}, false
	}
	k := key(pid, addr)
	s, found := r.Controller.Directory.Lookup(k)
	if !found {
		p.Valid = false
		return p, true
	}
	m, valid := r.Controller.Directory.LookupMapping(k, gpu, grit.AccessRead)
	p.Valid = valid
	if valid {
		p.PAddr = m.PAddr
		p.DeviceID = s.HomeGPU
		p.Generation = m.Generation
		p.Writable = m.Writable
		p.Scheme = uint8(s.Scheme)
	}
	return p, true
}
func (r *Runtime) Before(now sim.VTimeInSec, id string, pid vm.PID, addr, size, gpu uint64, write bool) bool {
	r.mu.Lock()
	defer r.mu.Unlock()
	if _, ok := r.active[id]; ok {
		return true
	}
	if r.pending != nil {
		return false
	}
	p, ok := r.Table.Find(pid, addr)
	if !ok || !p.Unified {
		r.active[id] = lease{}
		return true
	}
	if size == 0 || addr%4096+size > 4096 {
		panic("GRIT access must fit within one 4-KB page")
	}
	r.register(p)
	k := key(pid, addr)
	a := grit.AccessRead
	if write {
		a = grit.AccessWrite
	}
	s, _ := r.Controller.Directory.Lookup(k)
	if s.Scheme == grit.SchemeAccessCounter && r.perPage[k] > 0 {
		return false
	}
	decision := r.Controller.Directory.CheckAccess(k, gpu, a, 0)
	if decision != grit.AccessAllowed {
		r.pending = &work{id: id, key: k, gpu: gpu, write: write}
		r.TickLater(now)
		return false
	}
	// AC aliases share one physical image. Serialize conflicting cached accesses
	// and flush before a writer or a reader on another GPU observes dirty data.
	if s.Scheme == grit.SchemeAccessCounter && !r.tickets[id] && (write || r.lastWrite[k] != 0 && r.lastWrite[k] != gpu) {
		r.pending = &work{id: id, key: k, gpu: gpu, write: write, coherence: true}
		r.TickLater(now)
		return false
	}
	delete(r.tickets, id)
	r.active[id] = lease{key: k, gpu: gpu, write: write, managed: true}
	r.perPage[k]++
	r.clock++
	r.used[k] = r.clock
	return true
}
func (r *Runtime) After(now sim.VTimeInSec, id string) {
	r.mu.Lock()
	defer r.mu.Unlock()
	l, ok := r.active[id]
	if !ok {
		return
	}
	delete(r.active, id)
	if l.managed {
		r.perPage[l.key]--
		if l.write {
			r.lastWrite[l.key] = l.gpu
		}
	}
	r.wake(now)
}
func (r *Runtime) wake(now sim.VTimeInSec) {
	for _, w := range r.Wake {
		w(now)
	}
}
func (r *Runtime) queueFlush(now sim.VTimeInSec, level int) {
	for _, g := range r.GPUs {
		ports := g.L1
		if level == 2 {
			ports = g.L2
		}
		for _, dst := range ports {
			req := cache.FlushReqBuilder{}.WithSendTime(now).WithSrc(g.Port).WithDst(dst).InvalidateAllCacheLines().Build()
			r.out = append(r.out, req)
			r.waiting[req.ID] = dst
			r.Flushes++
		}
	}
}
func (r *Runtime) exchange(now sim.VTimeInSec) bool {
	progress := false
	if len(r.out) > 0 {
		msg := r.out[0]
		msg.Meta().SendTime = now
		if msg.Meta().Src.Send(msg) == nil {
			r.out = r.out[1:]
			progress = true
		}
	}
	for _, g := range r.GPUs {
		for _, port := range []sim.Port{g.Port, g.MemPort} {
			for {
				msg := port.Peek()
				if msg == nil {
					break
				}
				var id string
				switch v := msg.(type) {
				case *mem.DataReadyRsp:
					id = v.RespondTo
					off, ok := r.readOffsets[id]
					if !ok || r.evict == nil {
						panic("unexpected backing read")
					}
					copy(r.evict.data[off:], v.Data)
					delete(r.readOffsets, id)
				case *mem.WriteDoneRsp:
					id = v.RespondTo
				case *cache.FlushRsp:
					id = v.RspTo
				case *pagemigrationcontroller.PageMigrationRspFromPMC:
					id = v.RespondTo
				default:
					panic(fmt.Sprintf("unexpected GRIT response %T", msg))
				}
				expected, ok := r.waiting[id]
				if !ok || expected != msg.Meta().Src {
					panic("unmatched GRIT acknowledgement")
				}
				delete(r.waiting, id)
				port.Retrieve(now)
				progress = true
			}
		}
	}
	return progress
}
func (r *Runtime) Tick(now sim.VTimeInSec) bool {
	r.mu.Lock()
	defer r.mu.Unlock()
	progress := r.exchange(now)
	for r.pending == nil && len(r.counters) > 0 {
		w := r.counters[0]
		r.counters = r.counters[1:]
		state, ok := r.Controller.Directory.Lookup(w.key)
		if !ok || state.Scheme != grit.SchemeAccessCounter || state.HomeGPU == w.gpu {
			r.Controller.Counters.Complete(w.key, w.gpu)
			continue
		}
		r.pending = &w
	}

	if len(r.predictions) > 0 {
		prediction := r.predictions[0]
		r.predictions = r.predictions[1:]
		r.Controller.Predictor.Promote(prediction.key, prediction.scheme)
		progress = true
	}
	if r.pending == nil {
		return progress
	}
	w := r.pending
	switch w.phase {
	case 0:
		if len(r.active) > 0 {
			return true
		}
		if !w.coherence {
			if w.counter {
				if _, ok := r.Controller.Directory.BeginOperation(w.key); !ok {
					panic("counter placement lock failed")
				}
			} else {
				a := grit.AccessRead
				if w.write {
					a = grit.AccessWrite
				}
				kind := grit.FaultLocalPage
				if r.Controller.Directory.CheckAccess(w.key, w.gpu, a, 0) == grit.NeedWriteProtectionFault {
					kind = grit.FaultWriteProtection
				}
				resolution, ok, err := r.Controller.Admit(grit.Fault{ID: w.id, Key: w.key, GPU: w.gpu, Access: a, Kind: kind})
				if err != nil || !ok {
					panic(fmt.Sprintf("GRIT fault admission: %v", err))
				}
				w.admitted = true
				w.until = now + sim.VTimeInSec(resolution.PACycles)/1e9
			}
		}
		r.emit(now, w, "quiesced")
		r.queueFlush(now, 1)
		w.phase = 1
	case 1:
		if len(r.waiting) > 0 || len(r.out) > 0 {
			return true
		}
		r.queueFlush(now, 2)
		w.phase = 2
	case 2:
		if len(r.waiting) > 0 || len(r.out) > 0 || now < w.until {
			return true
		}
		if w.coherence {
			delete(r.lastWrite, w.key)
			r.tickets[w.id] = true
			r.pending = nil
			r.wake(now)
			return true
		}
		if !r.finishEviction(now) {
			return true
		}
		if r.BudgetPages > 0 {
			for i := range r.GPUs {
				gpu := uint64(i + 1)
				if r.resident(gpu) > r.BudgetPages {
					addr, ok := r.reserve(now, gpu, w.key)
					if ok {
						r.Allocator.ReleaseGRITFrame(addr)
					}
					return true
				}
			}
		}
		if !r.prepare(now, w) {
			return true
		}
		w.phase = 3
	case 3:
		if len(w.restore) > 0 {
			if now < w.until {
				return true
			}
			r.queueRestore(now, w)
			return true
		}
		if len(r.waiting) > 0 || len(r.out) > 0 {
			return true
		}
		if err := r.Controller.Directory.CommitOperation(w.key, w.plan.After); err != nil {
			panic(err)
		}
		r.publish(w.plan.After, false)
		for _, old := range w.plan.Retire {
			delete(r.byPhysical, old.PAddr)
			if old.GPUID != 0 {
				r.Allocator.ReleaseGRITFrame(old.PAddr)
			}
		}
		for _, m := range w.plan.After.Replicas {
			r.byPhysical[m.PAddr] = w.key
		}
		r.Total.RecordCompleted(w.plan, 4096)
		r.emit(now, w, "placement_committed")
		delete(r.lastWrite, w.key)
		if w.admitted {
			if w.write && w.plan.After.Scheme == grit.SchemeAccessCounter {
				r.tickets[w.id] = true
			}
			resolution, background, err := r.Controller.Complete(w.key)
			if err != nil {
				panic(err)
			}
			if background {
				r.predictions = append(r.predictions, predictionWork{w.key, resolution.Scheme})
				r.TickLater(now)
			}
		}
		if w.counter {
			r.Controller.Counters.Complete(w.key, w.gpu)
		}
		r.pending = nil
		r.wake(now)
	}
	return true
}
func (r *Runtime) prepare(now sim.VTimeInSec, w *work) bool {
	s, _ := r.Controller.Directory.Lookup(w.key)
	target := s.OwnerPAddr
	scheme := s.Scheme
	epoch := s.Epoch
	if w.admitted { /* Plan includes the controller's effective decision. */
		initial, err := r.Controller.Plan(w.key, ^uint64(0)-4095)
		if err != nil {
			panic(err)
		}
		scheme = initial.After.Scheme
		epoch = initial.After.Epoch
	}
	copyNeeded := w.counter || scheme != grit.SchemeAccessCounter && w.gpu != s.HomeGPU
	if scheme == grit.SchemeDuplication && !w.write {
		if m, ok := s.Replicas[w.gpu]; ok && m.Valid && s.Scheme == grit.SchemeDuplication {
			copyNeeded = false
			target = m.PAddr
		}
	}
	if s.HomeGPU == 0 {
		copyNeeded = true
	}
	if copyNeeded {
		if m, exists := s.Replicas[w.gpu]; exists && m.PAddr != s.OwnerPAddr {
			target = m.PAddr
		} else {
			var ok bool
			target, ok = r.reserve(now, w.gpu, w.key)
			if !ok {
				return false
			}
		}
	}

	var err error
	if s.HomeGPU == 0 {
		a := grit.AccessRead
		if w.write {
			a = grit.AccessWrite
		}
		w.plan, err = grit.PlanPlacement(s, w.gpu, a, grit.SchemeOnTouch, target)
		w.plan.After.Scheme = scheme
		w.plan.After.Epoch = epoch
	} else if w.counter {
		w.plan, err = grit.PlanCounterMigration(s, w.gpu, target)
	} else {
		w.plan, err = r.Controller.Plan(w.key, target)
	}
	if err != nil {
		panic(err)
	}
	if w.plan.Copy && s.HomeGPU == 0 {
		w.restore, err = r.Storage.Read(s.OwnerPAddr, 4096)
		if err != nil {
			panic(err)
		}
		w.until = now + sim.VTimeInSec(4096/r.HostBandwidth)
	} else if w.plan.Copy {
		src := r.GPUs[s.HomeGPU-1]
		dst := r.GPUs[w.gpu-1]
		req := pagemigrationcontroller.PageMigrationReqToPMCBuilder{}.WithSendTime(now).WithSrc(dst.Port).WithDst(dst.PMCControl).WithReadFrom(w.plan.Source).WithWriteTo(w.plan.Destination).WithPageSize(4096).WithPMCPortOfRemoteGPU(src.PMCRemote).Build()
		r.out = append(r.out, req)
		r.waiting[req.ID] = dst.PMCControl
	}
	return true
}

// evictReplica reclaims an independent read replica only after global quiescence
// and dirty cache writeback. The authoritative image is always retained.
func (r *Runtime) evictReplica(gpu uint64, exclude grit.PageKey) (uint64, bool) {
	var chosen grit.PageKey
	var addr uint64
	found := false
	for physical, k := range r.byPhysical {
		if k == exclude {
			continue
		}
		s, ok := r.Controller.Directory.Lookup(k)
		if !ok || s.Pending || s.HomeGPU == gpu {
			continue
		}
		m, ok := s.Replicas[gpu]
		if !ok || !m.Valid || m.PAddr != physical || physical == s.OwnerPAddr {
			continue
		}
		if !found || r.used[k] < r.used[chosen] || r.used[k] == r.used[chosen] && physical < addr {
			chosen = k
			addr = physical
			found = true
		}
	}
	if !found {
		return 0, false
	}
	s, _ := r.Controller.Directory.BeginOperation(chosen)
	delete(s.Replicas, gpu)
	s.Version++
	for id, m := range s.Replicas {
		m.Generation = s.Version
		s.Replicas[id] = m
	}
	if err := r.Controller.Directory.CommitOperation(chosen, s); err != nil {
		panic(err)
	}
	delete(r.byPhysical, addr)
	return addr, true
}

// Remote is called once after successful admission to the actual RDMA link.
func (r *Runtime) Remote(now sim.VTimeInSec, gpu uint64, req mem.AccessReq) {
	r.mu.Lock()
	defer r.mu.Unlock()
	k, ok := r.byPhysical[req.GetAddress()&^4095]
	if !ok {
		return
	}
	s, ok := r.Controller.Directory.Lookup(k)
	if !ok || s.Scheme != grit.SchemeAccessCounter || s.HomeGPU == gpu {
		return
	}
	n := (req.GetAddress()%64 + req.GetByteSize() + 63) / 64
	for i := uint64(0); i < n; i++ {
		r.Total.RemoteAccesses++
		if r.Controller.Counters.Count(k, gpu) {
			r.CounterInterrupts++
			r.counters = append(r.counters, work{id: sim.GetIDGenerator().Generate(), key: k, gpu: gpu, counter: true})
			r.TickLater(now)
		}
	}
	if _, write := req.(*mem.WriteReq); write {
		r.Total.RemoteWriteBytes += req.GetByteSize()
	} else {
		r.Total.RemoteReadBytes += req.GetByteSize()
	}
}
