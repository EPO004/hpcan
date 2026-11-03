package grit

import "sync"

type CounterKey struct {
	PID            uint32
	RegionVPN, GPU uint64
}
type CounterState struct {
	RemoteAccesses uint32
	MigrationArmed bool
}

// AccessCounters count originating remote cache-line transactions, not TLB misses.
// Count must be invoked once after successful RDMA admission, never on retries.
type AccessCounters struct {
	mu        sync.Mutex
	threshold uint32
	entries   map[CounterKey]CounterState
}

func NewAccessCounters(threshold uint32) *AccessCounters {
	if threshold == 0 {
		panic("AC threshold must be positive")
	}
	return &AccessCounters{threshold: threshold, entries: map[CounterKey]CounterState{}}
}
func RegionKey(k PageKey, gpu uint64) CounterKey { return CounterKey{k.PID, k.VPN &^ 15, gpu} }
func (c *AccessCounters) Count(k PageKey, gpu uint64) bool {
	c.mu.Lock()
	defer c.mu.Unlock()
	key := RegionKey(k, gpu)
	s := c.entries[key]
	if s.MigrationArmed {
		return false
	}
	s.RemoteAccesses++
	trigger := s.RemoteAccesses >= c.threshold
	s.MigrationArmed = trigger
	c.entries[key] = s
	return trigger
}
func (c *AccessCounters) Complete(k PageKey, gpu uint64) {
	c.mu.Lock()
	defer c.mu.Unlock()
	delete(c.entries, RegionKey(k, gpu))
}
func (c *AccessCounters) Remove(k PageKey) {
	c.mu.Lock()
	defer c.mu.Unlock()
	for key := range c.entries {
		if key.PID == k.PID && key.RegionVPN == k.VPN&^15 {
			delete(c.entries, key)
		}
	}
}
