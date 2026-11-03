package grit

import (
	"fmt"
	"sync"
)

type Frame struct {
	PAddr, GPU      uint64
	Key             PageKey
	Reserved, Dirty bool
	stamp           uint64
}

// FramePool enforces resident capacity including inflight reservations. Addresses
// must be actual allocator-provided frames. Reserve never evicts implicitly: the
// caller must complete a victim's flush/backing/invalidation before Release.
// LRU victim selection is an engineering assumption, not a paper requirement.
type FramePool struct {
	mu          sync.Mutex
	free        map[uint64][]uint64
	frames      map[uint64]Frame
	clock, peak uint64
}

func NewFramePool() *FramePool {
	return &FramePool{free: map[uint64][]uint64{}, frames: map[uint64]Frame{}}
}
func (p *FramePool) AddGPU(gpu uint64, addresses []uint64) error {
	p.mu.Lock()
	defer p.mu.Unlock()
	if gpu == 0 {
		return fmt.Errorf("invalid GPU")
	}
	if _, ok := p.free[gpu]; ok {
		return fmt.Errorf("GPU already registered")
	}
	seen := map[uint64]bool{}
	for _, addr := range addresses {
		if seen[addr] {
			return fmt.Errorf("duplicate frame")
		}
		seen[addr] = true
		for _, list := range p.free {
			for _, other := range list {
				if other == addr {
					return fmt.Errorf("frame on multiple GPUs")
				}
			}
		}
		if _, ok := p.frames[addr]; ok {
			return fmt.Errorf("frame already assigned")
		}
	}
	p.free[gpu] = append([]uint64(nil), addresses...)
	return nil
}
func (p *FramePool) Reserve(gpu uint64, k PageKey) (Frame, bool) {
	p.mu.Lock()
	defer p.mu.Unlock()
	list := p.free[gpu]
	if len(list) == 0 {
		return Frame{}, false
	}
	addr := list[len(list)-1]
	p.free[gpu] = list[:len(list)-1]
	p.clock++
	f := Frame{PAddr: addr, GPU: gpu, Key: k, Reserved: true, stamp: p.clock}
	p.frames[addr] = f
	if uint64(len(p.frames)) > p.peak {
		p.peak = uint64(len(p.frames))
	}
	return f, true
}
func (p *FramePool) Commit(addr uint64, dirty bool) error {
	p.mu.Lock()
	defer p.mu.Unlock()
	f, ok := p.frames[addr]
	if !ok || !f.Reserved {
		return fmt.Errorf("no reservation")
	}
	f.Reserved = false
	f.Dirty = dirty
	p.clock++
	f.stamp = p.clock
	p.frames[addr] = f
	return nil
}
func (p *FramePool) Touch(addr uint64, write bool) error {
	p.mu.Lock()
	defer p.mu.Unlock()
	f, ok := p.frames[addr]
	if !ok || f.Reserved {
		return fmt.Errorf("frame not resident")
	}
	p.clock++
	f.stamp = p.clock
	f.Dirty = f.Dirty || write
	p.frames[addr] = f
	return nil
}

// Victim does not release memory. Exclude all locked pages and the transfer source.
func (p *FramePool) Victim(gpu uint64, exclude map[PageKey]bool) (Frame, bool) {
	p.mu.Lock()
	defer p.mu.Unlock()
	var best Frame
	found := false
	for _, f := range p.frames {
		if f.GPU != gpu || f.Reserved || exclude[f.Key] {
			continue
		}
		if !found || f.stamp < best.stamp || (f.stamp == best.stamp && f.PAddr < best.PAddr) {
			best = f
			found = true
		}
	}
	return best, found
}
func (p *FramePool) Release(addr uint64) error {
	p.mu.Lock()
	defer p.mu.Unlock()
	f, ok := p.frames[addr]
	if !ok {
		return fmt.Errorf("frame already free")
	}
	delete(p.frames, addr)
	p.free[f.GPU] = append(p.free[f.GPU], addr)
	return nil
}
func (p *FramePool) Occupancy() (used, reserved, peak uint64) {
	p.mu.Lock()
	defer p.mu.Unlock()
	for _, f := range p.frames {
		if f.Reserved {
			reserved++
		}
	}
	return uint64(len(p.frames)), reserved, p.peak
}
