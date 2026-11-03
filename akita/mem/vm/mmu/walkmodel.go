package mmu

import "github.com/sarchlab/akita/v3/mem/vm"

type walkCacheKey struct {
	pid    vm.PID
	level  int
	prefix uint64
}
type walkCache struct {
	clock   uint64
	entries map[walkCacheKey]uint64
}

func (m *MMU) walkLatency(req *vm.TranslationReq) int {
	cache := m.walkCaches[req.DeviceID]
	cycles := m.latency // Leaf PTE must always be read.
	for level := 1; level < m.walkLevels; level++ {
		k := walkCacheKey{req.PID, level, req.VAddr >> (12 + 9*uint(level))}
		if _, hit := cache.entries[k]; hit {
			cycles++
		} else {
			cycles += m.latency
		}
	}
	return cycles
}
func (m *MMU) fillWalkCache(req *vm.TranslationReq) {
	c := m.walkCaches[req.DeviceID]
	for level := m.walkLevels - 1; level > 0; level-- {
		c.clock++
		k := walkCacheKey{req.PID, level, req.VAddr >> (12 + 9*uint(level))}
		if _, hit := c.entries[k]; !hit && len(c.entries) >= m.walkCacheEntries {
			var victim walkCacheKey
			oldest := ^uint64(0)
			for key, stamp := range c.entries {
				if stamp < oldest {
					victim = key
					oldest = stamp
				}
			}
			delete(c.entries, victim)
		}
		c.entries[k] = c.clock
	}
}

// WithPageWalkModel enables per-GPU walker limits, a bounded FIFO and an LRU
// intermediate-level walk cache. The existing fixed-latency model stays default.
func (b Builder) WithPageWalkModel(walkers, levels, cacheEntries, queueSize int) Builder {
	if walkers < 1 || levels < 1 || levels > 5 || cacheEntries < 1 || queueSize < walkers {
		panic("invalid page-walk model")
	}
	b.walkers = walkers
	b.walkLevels = levels
	b.walkCacheEntries = cacheEntries
	b.walkQueueSize = queueSize
	return b
}
