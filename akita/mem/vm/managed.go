package vm

import "github.com/sarchlab/akita/v3/sim"

// ManagedMemory coordinates UVM accesses before translation and data-cache lookup.
// Before is idempotent for a request ID; accepted requests retain a lease until
// After, including translation retries. A false return applies backpressure.
// Lookup returns managed=true for managed pages even while their PTE is invalid.
// Implementations must schedule placement work and wait for actual transfer ACKs.
type ManagedMemory interface {
	Before(now sim.VTimeInSec, id string, pid PID, vaddr, size, gpu uint64, write bool) bool
	After(now sim.VTimeInSec, id string)
	Lookup(pid PID, vaddr, gpu uint64) (Page, bool)
}
