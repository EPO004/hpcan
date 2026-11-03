package runner

import "github.com/sarchlab/akita/v3/mem/mem"

func dramSizeFromMB(size uint64) uint64 {
	if size == 0 || size&(size-1) != 0 || size > ^uint64(0)/mem.MB {
		panic("gpu-dram-size-mb must be a positive power of two without byte-size overflow")
	}
	return size * mem.MB
}

func validateDRAMSize(size uint64, numGPU int, log2PageSize uint64) {
	if size < mem.MB || size&(size-1) != 0 {
		panic("GPU DRAM size must be a power of two and at least 1 MB")
	}
	if log2PageSize >= 64 || size < uint64(1)<<log2PageSize {
		panic("GPU DRAM size must be at least one page")
	}
	if numGPU < 1 || uint64(numGPU) >= ^uint64(0)/size {
		panic("invalid GPU count or total memory size overflow")
	}
}
