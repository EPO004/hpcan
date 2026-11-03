package driver

import "github.com/sarchlab/akita/v3/sim"

// ReserveGRITFrame reserves an unpublished physical frame for an acknowledged
// migration/replication. It does not replace the host canonical mapping.
func (d *Driver) ReserveGRITFrame(gpu uint64) (uint64, bool) {
	return d.memAllocator.(interface{ ReserveGRITFrame(uint64) (uint64, bool) }).ReserveGRITFrame(gpu)
}
func (d *Driver) ReleaseGRITFrame(addr uint64) {
	d.memAllocator.(interface{ ReleaseGRITFrame(uint64) }).ReleaseGRITFrame(addr)
}

// flushGRITBeforeFree prevents a dirty cache line from outliving its physical
// frame and overwriting a subsequent allocation that reuses that frame.
func (d *Driver) flushGRITBeforeFree(ctx *Context) {
	queue := d.CreateCommandQueue(ctx)
	d.Enqueue(queue, &FlushCommand{ID: sim.GetIDGenerator().Generate()})
	d.DrainCommandQueue(queue)
}

// lockManagedAllocation uses the same ordering as the event engine, preventing
// CPU allocation callbacks from holding the allocator while GRIT reserves frames.
func (d *Driver) lockManagedAllocation() func() {
	if _, managed := d.pageTable.(interface{ ManagedIdle() bool }); managed {
		d.engineMutex.Lock()
		return d.engineMutex.Unlock
	}
	return func() {}
}
