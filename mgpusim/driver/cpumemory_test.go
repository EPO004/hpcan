package driver

import (
	"github.com/sarchlab/akita/v3/mem/mem"
	"github.com/sarchlab/akita/v3/mem/vm"
	"github.com/sarchlab/akita/v3/sim"
	"github.com/sarchlab/mgpusim/v3/driver/internal"
	"testing"
)

func TestCPUMemoryReservationAlignsGPURanges(t *testing.T) {
	for _, mb := range []uint64{1, 32, 64} {
		size := mb * mem.MB
		d := MakeBuilder().WithEngine(sim.NewSerialEngine()).
			WithLog2PageSize(12).WithPageTable(vm.NewPageTable(12)).
			WithCPUMemorySize(size).Build("Driver")
		for id := 1; id <= 2; id++ {
			gpu := &internal.Device{ID: id, Type: internal.DeviceTypeGPU,
				MemState: internal.NewDeviceMemoryState(12)}
			gpu.SetTotalMemSize(size)
			d.memAllocator.RegisterDevice(gpu)
			for _, addr := range []uint64{uint64(id) * size, uint64(id+1)*size - 1} {
				if got := d.memAllocator.GetDeviceIDByPAddr(addr); got != id {
					t.Fatalf("%d MB: address %#x belongs to GPU %d, got %d", mb, addr, id, got)
				}
			}
		}
		if got := d.memAllocator.GetDeviceIDByPAddr(size - 1); got != 0 {
			t.Fatalf("CPU upper bound belongs to device %d", got)
		}
	}
}
