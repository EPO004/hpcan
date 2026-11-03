package grit

import "sync"

type PAEntry struct {
	Faults   uint32
	SawWrite bool
}
type PAStats struct{ Hits, Misses, Writebacks, BackingReads, BackingWrites, StallCycles uint64 }
type paLine struct {
	key          PageKey
	entry        PAEntry
	valid, dirty bool
	stamp        uint64
}

// PATable models a 16-set, four-way write-back/write-allocate attribute cache.
// Access delays are returned to the event caller; it must wait before resolving
// a fault. Backing-only mode still incurs host-memory latency on each access.
type PATable struct {
	mu                               sync.Mutex
	backing                          map[PageKey]PAEntry
	sets                             [16][4]paLine
	cache                            bool
	threshold                        uint32
	cacheCycles, memoryCycles, clock uint64
	stats                            PAStats
}

func NewPATable(cache bool, threshold uint32, cacheCycles, memoryCycles uint64) *PATable {
	if threshold == 0 {
		panic("fault threshold must be positive")
	}
	return &PATable{backing: map[PageKey]PAEntry{}, cache: cache, threshold: threshold, cacheCycles: cacheCycles, memoryCycles: memoryCycles}
}
func DecideOnFourthFault(sawWrite bool) Scheme {
	if sawWrite {
		return SchemeAccessCounter
	}
	return SchemeDuplication
}

// RecordFault admits one already-coalesced real local/protection fault. A trigger
// deletes both cached and backing history, preventing stale dirty resurrection.
func (p *PATable) RecordFault(k PageKey, a AccessKind, kind FaultKind) (Scheme, bool, uint64) {
	p.mu.Lock()
	defer p.mu.Unlock()
	p.clock++
	var e PAEntry
	var line *paLine
	var cycles uint64
	if p.cache {
		cycles = p.cacheCycles
		set := &p.sets[k.VPN&15]
		for i := range set {
			if set[i].valid && set[i].key == k {
				line = &set[i]
				break
			}
		}
		if line != nil {
			p.stats.Hits++
			e = line.entry
		} else {
			p.stats.Misses++
			p.stats.BackingReads++
			cycles += p.memoryCycles
			e = p.backing[k]
			victim := 0
			for i := range set {
				if !set[i].valid {
					victim = i
					break
				}
				if set[i].stamp < set[victim].stamp {
					victim = i
				}
			}
			line = &set[victim]
			if line.valid && line.dirty {
				p.backing[line.key] = line.entry
				p.stats.Writebacks++
				p.stats.BackingWrites++
				cycles += p.memoryCycles
			}
			*line = paLine{key: k, valid: true}
		}
	} else {
		p.stats.BackingReads++
		cycles = p.memoryCycles
		e = p.backing[k]
	}
	e.Faults++
	e.SawWrite = e.SawWrite || a == AccessWrite || kind == FaultWriteProtection
	trigger := e.Faults >= p.threshold
	scheme := SchemeInvalid
	if trigger {
		scheme = DecideOnFourthFault(e.SawWrite)
		delete(p.backing, k)
		// Deletion is a backing update even when the last state only lived in cache.
		p.stats.BackingWrites++
		cycles += p.memoryCycles
		if line != nil {
			*line = paLine{}
		}
	} else if line != nil {
		line.entry = e
		line.dirty = true
		line.stamp = p.clock
	} else {
		p.backing[k] = e
		p.stats.BackingWrites++
		cycles += p.memoryCycles
	}
	p.stats.StallCycles += cycles
	return scheme, trigger, cycles
}
func (p *PATable) Remove(k PageKey) {
	p.mu.Lock()
	defer p.mu.Unlock()
	delete(p.backing, k)
	for i := range p.sets[k.VPN&15] {
		l := &p.sets[k.VPN&15][i]
		if l.valid && l.key == k {
			*l = paLine{}
		}
	}
}
func (p *PATable) Snapshot(k PageKey) (PAEntry, bool) {
	p.mu.Lock()
	defer p.mu.Unlock()
	for _, l := range p.sets[k.VPN&15] {
		if l.valid && l.key == k {
			return l.entry, true
		}
	}
	e, ok := p.backing[k]
	return e, ok
}
func (p *PATable) Stats() PAStats { p.mu.Lock(); defer p.mu.Unlock(); return p.stats }
func (p *PATable) Flush() uint64 {
	p.mu.Lock()
	defer p.mu.Unlock()
	var n uint64
	for i := range p.sets {
		for j := range p.sets[i] {
			l := &p.sets[i][j]
			if l.valid && l.dirty {
				p.backing[l.key] = l.entry
				l.dirty = false
				n++
				p.stats.Writebacks++
				p.stats.BackingWrites++
			}
		}
	}
	return n * p.memoryCycles
}
