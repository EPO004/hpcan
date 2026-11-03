package internal

import "github.com/sarchlab/akita/v3/mem/vm"

// ReserveGRITFrame allocates a physical frame without publishing a virtual PTE.
// It returns backpressure on full memory instead of panicking.
func (a *memoryAllocatorImpl) ReserveGRITFrame(gpu uint64) (uint64, bool) {
	a.Lock()
	defer a.Unlock()
	d, ok := a.devices[int(gpu)]
	if !ok || d.Type == DeviceTypeUnifiedGPU || d.MemState.noAvailablePAddrs() {
		return 0, false
	}
	return d.allocatePage(), true
}
func (a *memoryAllocatorImpl) ReleaseGRITFrame(addr uint64) {
	a.Lock()
	defer a.Unlock()
	gpu := a.deviceIDByPAddr(addr)
	a.devices[gpu].MemState.addSinglePAddr(addr)
}

// FreeGRITPage identifies pages by PID and returns every replica frame exactly
// once. This path also handles canonical host-backed pages outside GPU ranges.
func (a *memoryAllocatorImpl) FreeGRITPage(pid vm.PID, addr uint64) {
	a.Lock()
	defer a.Unlock()
	p, ok := a.pageTable.Find(pid, addr)
	if !ok {
		panic("managed page not found")
	}
	frames := []uint64{}
	if observer, ok := a.pageTable.(interface{ FreeManagedPage(vm.PID, uint64) []uint64 }); ok {
		frames = observer.FreeManagedPage(pid, addr)
	}
	if len(frames) == 0 && !p.HostBacked {
		frames = append(frames, p.PAddr)
	}
	for _, frame := range frames {
		id := a.deviceIDByPAddr(frame)
		a.devices[id].MemState.addSinglePAddr(frame)
	}
	a.pageTable.Remove(pid, p.VAddr)
	if old, ok := a.vAddrToPageMapping[p.VAddr]; ok && old.PID == pid {
		delete(a.vAddrToPageMapping, p.VAddr)
	}
}

// prepareGRITReplacement detaches replicas for an explicit API remap. Allocator
// remaps are performed between kernels, and data initialization remains the
// responsibility of the caller, matching the existing remap API.
func (a *memoryAllocatorImpl) prepareGRITReplacement(page *vm.Page) {
	observer, ok := a.pageTable.(interface{ FreeManagedPage(vm.PID, uint64) []uint64 })
	if !ok {
		return
	}
	old, found := a.pageTable.Find(page.PID, page.VAddr)
	if !found {
		return
	}
	page.Unified = old.Unified
	for _, frame := range observer.FreeManagedPage(page.PID, page.VAddr) {
		id := a.deviceIDByPAddr(frame)
		a.devices[id].MemState.addSinglePAddr(frame)
	}
}
