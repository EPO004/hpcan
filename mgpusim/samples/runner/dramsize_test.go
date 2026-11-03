package runner

import (
	"github.com/sarchlab/akita/v3/mem/mem"
	"testing"
)

func TestDRAMSizeMB(t *testing.T) {
	for _, mb := range []uint64{1, 32, 64, 4096} {
		if got := dramSizeFromMB(mb); got != mb*mem.MB {
			t.Fatalf("%d MB converted to %d bytes", mb, got)
		}
	}
	for _, mb := range []uint64{0, 3, 48, 1 << 63} {
		t.Run("invalid", func(t *testing.T) {
			defer func() {
				if recover() == nil {
					t.Errorf("accepted invalid size %d", mb)
				}
			}()
			dramSizeFromMB(mb)
		})
	}
	if MakeEmuBuilder().dramSize != 32*mem.MB || MakeR9NanoBuilder().dramSize != 32*mem.MB || MakeR9NanoGPUBuilder().dramSize != 32*mem.MB {
		t.Fatal("platform default must be 32 MB")
	}
}

func TestInvalidPlatformMemoryConfiguration(t *testing.T) {
	for _, c := range []struct {
		size uint64
		gpus int
		page uint64
	}{
		{0, 1, 12}, {3 * mem.MB, 1, 12}, {mem.MB, 0, 12},
		{mem.MB, 1, 64}, {mem.MB, 1, 21}, {1 << 63, 2, 12},
	} {
		func() {
			defer func() {
				if recover() == nil {
					t.Errorf("accepted invalid configuration %+v", c)
				}
			}()
			validateDRAMSize(c.size, c.gpus, c.page)
		}()
	}
}
