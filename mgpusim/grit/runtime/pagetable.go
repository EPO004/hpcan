package runtime

import (
	"github.com/sarchlab/akita/v3/mem/vm"
	"github.com/sarchlab/mgpusim/v3/grit"
)

// ObservedPageTable leaves Akita's PageTable interface intact while tracking
// allocations before first touch. Runtime publication uses the underlying table.
type ObservedPageTable struct {
	vm.PageTable
	Runtime *Runtime
}

func (p *ObservedPageTable) Insert(page vm.Page) {
	p.PageTable.Insert(page)
	if p.Runtime != nil && page.Unified {
		p.Runtime.mu.Lock()
		p.Runtime.register(page)
		p.Runtime.mu.Unlock()
	}
}

// FreeManagedPage returns unique physical GPU frames; the allocator releases
// them under its own lock. Host backing consumes no GPU frame.
func (p *ObservedPageTable) FreeManagedPage(pid vm.PID, addr uint64) []uint64 {
	r := p.Runtime
	if r == nil {
		return nil
	}
	r.mu.Lock()
	defer r.mu.Unlock()
	k := key(pid, addr)
	s, ok := r.Controller.Directory.Lookup(k)
	if !ok {
		return nil
	}
	if r.pending != nil || len(r.active) > 0 {
		panic("cannot free managed memory while GPU accesses are in flight")
	}
	frames := []uint64{}
	seen := map[uint64]bool{}
	for gpu, m := range s.Replicas {
		if gpu != 0 && !seen[m.PAddr] {
			frames = append(frames, m.PAddr)
			seen[m.PAddr] = true
			delete(r.byPhysical, m.PAddr)
		}
	}
	if err := r.Controller.Free(k); err != nil {
		panic(err)
	}
	delete(r.lastWrite, k)
	delete(r.used, k)
	if host, ok := r.backing[k]; ok {
		r.freeHost = append(r.freeHost, host)
		delete(r.backing, k)
	}
	return frames
}

// PrepareManagedHostWrite invalidates independent read replicas after a host
// overwrite. Flushing remains on the driver's ordinary ordered copy path.
func (p *ObservedPageTable) PrepareManagedHostWrite(pid vm.PID, addr uint64) []uint64 {
	r := p.Runtime
	if r == nil {
		return nil
	}
	r.mu.Lock()
	defer r.mu.Unlock()
	k := key(pid, addr)
	s, ok := r.Controller.Directory.Lookup(k)
	if !ok {
		return nil
	}
	if r.pending != nil || len(r.active) > 0 {
		panic("host overwrite while managed GPU accesses are in flight")
	}
	s, ok = r.Controller.Directory.BeginOperation(k)
	if !ok {
		panic("host overwrite lock failed")
	}
	frames := []uint64{}
	seen := map[uint64]bool{}
	for gpu, m := range s.Replicas {
		if gpu != s.HomeGPU && m.PAddr != s.OwnerPAddr && !seen[m.PAddr] {
			frames = append(frames, m.PAddr)
			seen[m.PAddr] = true
			delete(r.byPhysical, m.PAddr)
		}
	}
	s.Version++
	s.Replicas = map[uint64]grit.Replica{s.HomeGPU: {GPUID: s.HomeGPU, PAddr: s.OwnerPAddr, Valid: true, Writable: true, Generation: s.Version}}
	if err := r.Controller.Directory.CommitOperation(k, s); err != nil {
		panic(err)
	}
	r.publish(s, s.HomeGPU == 0)
	r.Controller.Directory.Unmap(k, s.HomeGPU)
	delete(r.lastWrite, k)
	return frames
}

func (p *ObservedPageTable) Update(page vm.Page) {
	p.PageTable.Update(page)
	if p.Runtime != nil && page.Unified {
		p.Runtime.mu.Lock()
		p.Runtime.register(page)
		p.Runtime.mu.Unlock()
	}
}
func (p *ObservedPageTable) ManagedIdle() bool {
	r := p.Runtime
	if r == nil {
		return true
	}
	r.mu.Lock()
	defer r.mu.Unlock()
	return r.pending == nil && len(r.active) == 0 && len(r.counters) == 0
}

func (p *ObservedPageTable) ManagedGPUFull(gpu uint64) bool {
	r := p.Runtime
	if r == nil || r.BudgetPages == 0 {
		return false
	}
	r.mu.Lock()
	defer r.mu.Unlock()
	return r.resident(gpu) >= r.BudgetPages
}
func (p *ObservedPageTable) AllocateManagedBacking(pid vm.PID, addr uint64) vm.Page {
	r := p.Runtime
	r.mu.Lock()
	defer r.mu.Unlock()
	host := (r.Storage.Capacity + 4095) &^ 4095
	if len(r.freeHost) > 0 {
		host = r.freeHost[len(r.freeHost)-1]
		r.freeHost = r.freeHost[:len(r.freeHost)-1]
	} else {
		r.Storage.Capacity = host + 4096
	}
	if err := r.Storage.Write(host, make([]byte, 4096)); err != nil {
		panic(err)
	}
	r.backing[key(pid, addr)] = host
	return vm.Page{PID: pid, VAddr: addr, PAddr: host, PageSize: 4096, Valid: true, Unified: true, HostBacked: true}
}
