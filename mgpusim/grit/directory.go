package grit

import (
	"fmt"
	"sync"
)

// Directory is the canonical resident-frame directory. Returned states are copies.
// BeginOperation/CommitOperation serialize acknowledged placement transactions.
type Directory struct {
	mu       sync.Mutex
	pages    map[PageKey]PageState
	before   map[PageKey]PageState
	unmapped map[PageKey]map[uint64]bool
}

func NewDirectory() *Directory {
	return &Directory{pages: map[PageKey]PageState{}, before: map[PageKey]PageState{}, unmapped: map[PageKey]map[uint64]bool{}}
}
func (d *Directory) OnAllocate(k PageKey, gpu, addr uint64) error {
	d.mu.Lock()
	defer d.mu.Unlock()
	if gpu == 0 {
		return fmt.Errorf("GPU IDs must be physical, nonzero IDs")
	}
	if _, ok := d.pages[k]; ok {
		return fmt.Errorf("page already allocated: %v", k)
	}
	d.pages[k] = PageState{Key: k, Scheme: SchemeOnTouch, HomeGPU: gpu, OwnerPAddr: addr, Version: 1, Replicas: map[uint64]Replica{gpu: {GPUID: gpu, PAddr: addr, Valid: true, Writable: true, Generation: 1}}}
	return nil
}
func (d *Directory) Lookup(k PageKey) (PageState, bool) {
	d.mu.Lock()
	defer d.mu.Unlock()
	s, ok := d.pages[k]
	return clone(s), ok
}
func (d *Directory) LookupMapping(k PageKey, gpu uint64, a AccessKind) (Replica, bool) {
	d.mu.Lock()
	defer d.mu.Unlock()
	s, ok := d.pages[k]
	if !ok || s.Pending || d.unmapped[k][gpu] {
		return Replica{}, false
	}
	r, ok := s.Replicas[gpu]
	return r, ok && r.Valid && r.Generation == s.Version && (a == AccessRead || r.Writable)
}
func (d *Directory) BeginOperation(k PageKey) (PageState, bool) {
	d.mu.Lock()
	defer d.mu.Unlock()
	s, ok := d.pages[k]
	if !ok || s.Pending {
		return PageState{}, false
	}
	d.before[k] = clone(s)
	s.Pending = true
	d.pages[k] = s
	return clone(s), true
}
func Validate(s PageState) error {
	if !s.Scheme.Valid() {
		return fmt.Errorf("invalid scheme")
	}
	writers := 0
	owner := false
	for gpu, r := range s.Replicas {
		if gpu != r.GPUID {
			return fmt.Errorf("GPU mapping identity mismatch")
		}
		if !r.Valid {
			continue
		}
		if r.Generation != s.Version {
			return fmt.Errorf("stale valid mapping")
		}
		if r.Writable {
			writers++
		}
		if gpu == s.HomeGPU && r.PAddr == s.OwnerPAddr {
			owner = true
		}
		if s.Scheme == SchemeOnTouch && (gpu != s.HomeGPU || r.PAddr != s.OwnerPAddr) {
			return fmt.Errorf("OT has non-owner mapping")
		}
		if s.Scheme == SchemeAccessCounter && r.PAddr != s.OwnerPAddr {
			return fmt.Errorf("AC must address authoritative frame")
		}
	}
	if !owner {
		return fmt.Errorf("authoritative owner missing")
	}
	if writers > 1 && s.Scheme != SchemeAccessCounter {
		return fmt.Errorf("multiple writable mappings")
	}
	if s.Scheme == SchemeDuplication && len(s.Replicas) > 1 && writers != 0 {
		return fmt.Errorf("shared DUP mappings must be read-only")
	}
	return nil
}
func (d *Directory) CommitOperation(k PageKey, next PageState) error {
	d.mu.Lock()
	defer d.mu.Unlock()
	s, ok := d.pages[k]
	if !ok || !s.Pending {
		return fmt.Errorf("no operation in progress")
	}
	if next.Key != k || next.Version < s.Version || next.Epoch < s.Epoch {
		return fmt.Errorf("invalid transaction identity/version")
	}
	next.Pending = false
	if err := Validate(next); err != nil {
		return err
	}
	d.pages[k] = clone(next)
	delete(d.unmapped, k)
	delete(d.before, k)
	return nil
}
func (d *Directory) AbortOperation(k PageKey) {
	d.mu.Lock()
	defer d.mu.Unlock()
	if s, ok := d.before[k]; ok {
		d.pages[k] = s
		delete(d.before, k)
	}
}
func (d *Directory) OnFree(k PageKey) error {
	d.mu.Lock()
	defer d.mu.Unlock()
	if d.pages[k].Pending {
		return fmt.Errorf("page operation pending")
	}
	delete(d.pages, k)
	delete(d.unmapped, k)
	return nil
}

// CheckAccess must run before cache lookup, including write hits. Generation zero
// means the caller has no cached translation; nonzero generations must match.
type AccessDecision uint8

const (
	AccessAllowed AccessDecision = iota
	NeedLocalFault
	NeedWriteProtectionFault
	WaitForPageTransition
)

func (d *Directory) CheckAccess(k PageKey, gpu uint64, a AccessKind, generation uint64) AccessDecision {
	d.mu.Lock()
	defer d.mu.Unlock()
	s, ok := d.pages[k]
	if !ok {
		return NeedLocalFault
	}
	if s.Pending {
		return WaitForPageTransition
	}
	r, ok := s.Replicas[gpu]
	if !ok || d.unmapped[k][gpu] || !r.Valid || r.Generation != s.Version || (generation != 0 && generation != r.Generation) {
		return NeedLocalFault
	}
	if a == AccessWrite && !r.Writable {
		return NeedWriteProtectionFault
	}
	return AccessAllowed
}

// Unmap separates GPU-local PTE validity from physical frame residency.
func (d *Directory) Unmap(k PageKey, gpu uint64) {
	d.mu.Lock()
	defer d.mu.Unlock()
	if d.unmapped[k] == nil {
		d.unmapped[k] = map[uint64]bool{}
	}
	d.unmapped[k][gpu] = true
}

// OnAllocateBacking registers an allocated page that has no resident GPU frame.
// CPU backing is authoritative until the first GPU fault restores it.
func (d *Directory) OnAllocateBacking(k PageKey, addr uint64) error {
	d.mu.Lock()
	defer d.mu.Unlock()
	if _, ok := d.pages[k]; ok {
		return fmt.Errorf("page already allocated")
	}
	d.pages[k] = PageState{Key: k, Scheme: SchemeOnTouch, OwnerPAddr: addr, Version: 1, Replicas: map[uint64]Replica{0: {PAddr: addr, Valid: true, Writable: true, Generation: 1}}}
	return nil
}
