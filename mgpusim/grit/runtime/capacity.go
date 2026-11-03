package runtime

import (
	"github.com/sarchlab/akita/v3/mem/mem"
	"github.com/sarchlab/akita/v3/mem/vm"
	"github.com/sarchlab/akita/v3/sim"
	"github.com/sarchlab/mgpusim/v3/grit"
)

type eviction struct {
	key             grit.PageKey
	state           grit.PageState
	addr, gpu, host uint64
	data            []byte
	until           sim.VTimeInSec
	transferred     bool
}

func (r *Runtime) resident(gpu uint64) uint64 {
	var n uint64
	for addr, k := range r.byPhysical {
		s, ok := r.Controller.Directory.Lookup(k)
		if !ok {
			continue
		}
		for id, m := range s.Replicas {
			if id == gpu && m.PAddr == addr {
				n++
				break
			}
		}
	}
	return n
}
func (r *Runtime) reserve(now sim.VTimeInSec, gpu uint64, exclude grit.PageKey) (uint64, bool) {
	if r.BudgetPages == 0 || r.resident(gpu) < r.BudgetPages {
		if addr, ok := r.Allocator.ReserveGRITFrame(gpu); ok {
			if uint64(len(r.byPhysical)+1) > r.FramesPeak {
				r.FramesPeak = uint64(len(r.byPhysical) + 1)
			}
			return addr, true
		}
	}
	if addr, ok := r.evictReplica(gpu, exclude); ok {
		if uint64(len(r.byPhysical)+1) > r.FramesPeak {
			r.FramesPeak = uint64(len(r.byPhysical) + 1)
		}
		return addr, true
	}
	// Only authoritative frames remain: stage a timed DRAM read and host backing.
	var chosen grit.PageKey
	var addr uint64
	found := false
	for physical, k := range r.byPhysical {
		if k == exclude {
			continue
		}
		s, ok := r.Controller.Directory.Lookup(k)
		if !ok || s.Pending || s.HomeGPU != gpu || s.OwnerPAddr != physical {
			continue
		}
		if !found || r.used[k] < r.used[chosen] || r.used[k] == r.used[chosen] && physical < addr {
			chosen = k
			addr = physical
			found = true
		}
	}
	if !found {
		panic("GRIT resident budget cannot hold the locked working page; increase -grit-resident-pages")
	}
	s, _ := r.Controller.Directory.BeginOperation(chosen)
	// If another independent replica exists, promote it without a host transfer.
	survivor := uint64(0)
	for id, m := range s.Replicas {
		if id != gpu && id != 0 && m.Valid && m.PAddr != addr && (survivor == 0 || id < survivor) {
			survivor = id
		}
	}
	if survivor != 0 {
		delete(s.Replicas, gpu)
		s.HomeGPU = survivor
		s.OwnerPAddr = s.Replicas[survivor].PAddr
		s.Version++
		for id, m := range s.Replicas {
			m.Generation = s.Version
			s.Replicas[id] = m
		}
		if err := r.Controller.Directory.CommitOperation(chosen, s); err != nil {
			panic(err)
		}
		r.publish(s, false)
		delete(r.byPhysical, addr)
		r.Evictions++
		if uint64(len(r.byPhysical)+1) > r.FramesPeak {
			r.FramesPeak = uint64(len(r.byPhysical) + 1)
		}
		return addr, true
	}
	host, exists := r.backing[chosen]
	if !exists {
		if len(r.freeHost) > 0 {
			host = r.freeHost[len(r.freeHost)-1]
			r.freeHost = r.freeHost[:len(r.freeHost)-1]
		} else {
			host = (r.Storage.Capacity + 4095) &^ 4095
			r.Storage.Capacity = host + 4096
		}
		r.backing[chosen] = host
	}

	r.evict = &eviction{key: chosen, state: s, addr: addr, gpu: gpu, host: host, data: make([]byte, 4096)}
	g := r.GPUs[gpu-1]
	for off := uint64(0); off < 4096; off += 64 {
		req := mem.ReadReqBuilder{}.WithSendTime(now).WithSrc(g.MemPort).WithDst(g.Memory.Find(addr + off)).WithAddress(addr + off).WithByteSize(64).Build()
		r.out = append(r.out, req)
		r.waiting[req.ID] = req.Dst
		r.readOffsets[req.ID] = off
	}
	return 0, false
}
func (r *Runtime) finishEviction(now sim.VTimeInSec) bool {
	e := r.evict
	if e == nil {
		return true
	}
	if len(r.waiting) > 0 || len(r.out) > 0 {
		return false
	}
	if !e.transferred {
		e.until = now + sim.VTimeInSec(4096/r.HostBandwidth)
		e.transferred = true
		return false
	}
	if now < e.until {
		return false
	}
	if err := r.Storage.Write(e.host, e.data); err != nil {
		panic(err)
	}
	s := e.state
	s.HomeGPU = 0
	s.OwnerPAddr = e.host
	s.Version++
	s.Replicas = map[uint64]grit.Replica{0: {GPUID: 0, PAddr: e.host, Valid: true, Writable: true, Generation: s.Version}}
	if err := r.Controller.Directory.CommitOperation(e.key, s); err != nil {
		panic(err)
	}
	r.publish(s, true)
	delete(r.byPhysical, e.addr)
	delete(r.lastWrite, e.key)
	r.Allocator.ReleaseGRITFrame(e.addr)
	r.Evictions++
	r.SwapBytes += 4096
	r.evict = nil
	return true
}
func (r *Runtime) publish(s grit.PageState, host bool) {
	p, _ := r.Table.Find(vm.PID(s.Key.PID), s.Key.VPN<<12)
	p.PAddr = s.OwnerPAddr
	p.DeviceID = s.HomeGPU
	p.Scheme = uint8(s.Scheme)
	p.Generation = s.Version
	p.IsPinned = false
	p.IsMigrating = false
	p.HostBacked = host
	r.Table.Update(p)
}
func (r *Runtime) queueRestore(now sim.VTimeInSec, w *work) {
	g := r.GPUs[w.gpu-1]
	for off := uint64(0); off < 4096; off += 64 {
		req := mem.WriteReqBuilder{}.WithSendTime(now).WithSrc(g.MemPort).WithDst(g.Memory.Find(w.plan.Destination + off)).WithAddress(w.plan.Destination + off).WithData(w.restore[off : off+64]).Build()
		r.out = append(r.out, req)
		r.waiting[req.ID] = req.Dst
	}
	w.restore = nil
	r.SwapBytes += 4096
}
