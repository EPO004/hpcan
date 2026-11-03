package runner

import (
	"github.com/sarchlab/mgpusim/v3/driver"
	"github.com/sarchlab/mgpusim/v3/grit"
	"testing"
)

// This drives compiled GPU load/store instructions through AT -> TLB -> caches
// -> RDMA/DRAM and the PMC, then checks host-visible bytes after cache flush.
func TestGRITLivePolicies(t *testing.T) {
	for _, mode := range []grit.Mode{grit.ModeOT, grit.ModeAC, grit.ModeDUP, grit.ModeGRIT} {
		t.Run(string(mode), func(t *testing.T) {
			cfg := grit.DefaultConfig()
			cfg.Mode = mode
			cfg.ACThreshold = 2
			b := MakeR9NanoBuilder().WithNumGPU(3).WithGRITConfig(cfg)
			b.numSAPerGPU = 1
			b.numCUPerSA = 1
			p := b.Build()
			d := p.Driver
			d.Run()
			defer d.Terminate()
			ctx := d.Init()
			src := d.AllocateUnifiedMemory(ctx, 4096)
			dst := d.AllocateUnifiedMemory(ctx, 4096)
			input := make([]uint32, 64)
			for i := range input {
				input[i] = uint32(7 + i*13)
			}
			d.MemCopyH2D(ctx, src, input)
			d.MemCopyH2D(ctx, dst, make([]uint32, 64))
			for i := 0; i < 8; i++ {
				d.SelectGPU(ctx, 1+i%3)
				d.MemCopyD2D(ctx, dst, src, 256)
			}
			// Write into the previously read-shared source, then read it on a peer.
			d.SelectGPU(ctx, 2)
			d.MemCopyD2D(ctx, src, dst, 256)
			d.SelectGPU(ctx, 3)
			d.MemCopyD2D(ctx, dst, src, 256)
			output := make([]uint32, 64)
			d.MemCopyD2H(ctx, output, dst)
			for i, v := range output {
				if v != input[i] {
					t.Fatalf("data[%d]=%d expected %d", i, v, input[i])
				}
			}
			m := p.GRIT.Controller.Metrics()
			if m.LocalPageFaults == 0 {
				t.Fatal("GRIT did not run")
			}
			total := p.GRIT.Total
			switch mode {
			case grit.ModeOT:
				if total.Migrations == 0 {
					t.Fatal("no OT migration")
				}
			case grit.ModeAC:
				if total.RemoteAccesses == 0 {
					t.Fatal("AC bypassed RDMA")
				}
			case grit.ModeDUP:
				if total.Copies == 0 || total.Collapses == 0 {
					t.Fatal("DUP did not replicate/collapse", total)
				}
			case grit.ModeGRIT:
				if m.FaultTriggers == 0 {
					t.Fatal("no dynamic scheme decisions")
				}
			}
		})
	}
}

func TestGRITOversubscriptionBacking(t *testing.T) {
	cfg := grit.DefaultConfig()
	cfg.Mode = grit.ModeDUP
	b := MakeR9NanoBuilder().WithNumGPU(2).WithGRITConfig(cfg)
	b.numSAPerGPU = 1
	b.numCUPerSA = 1
	p := b.Build()
	p.GRIT.BudgetPages = 1
	d := p.Driver
	d.Run()
	defer d.Terminate()
	ctx := d.Init()
	src := d.AllocateUnifiedMemory(ctx, 4096)
	dst := d.AllocateUnifiedMemory(ctx, 4096)
	input := make([]uint32, 64)
	for i := range input {
		input[i] = uint32(i + 99)
	}
	d.MemCopyH2D(ctx, src, input)
	d.MemCopyH2D(ctx, dst, make([]uint32, 64))
	for i := 0; i < 4; i++ {
		d.SelectGPU(ctx, 1+i%2)
		d.MemCopyD2D(ctx, dst, src, 256)
	}
	for _, ptr := range []driver.Ptr{src, dst} {
		out := make([]uint32, 64)
		d.MemCopyD2H(ctx, out, ptr)
		for i, v := range out {
			if v != input[i] {
				t.Fatalf("backing data[%d]=%d expected %d", i, v, input[i])
			}
		}
	}
	if p.GRIT.Evictions == 0 || p.GRIT.SwapBytes == 0 {
		t.Fatal("oversubscription did not exercise backing")
	}
}

func TestGRITFreeIsPIDScopedAndReclaimsWholeBuffer(t *testing.T) {
	cfg := grit.DefaultConfig()
	cfg.Mode = grit.ModeGRIT
	b := MakeR9NanoBuilder().WithNumGPU(2).WithGRITConfig(cfg)
	b.numSAPerGPU = 1
	b.numCUPerSA = 1
	p := b.Build()
	d := p.Driver
	d.Run()
	defer d.Terminate()
	a := d.Init()
	other := d.Init()
	first := d.AllocateUnifiedMemory(a, 8192)
	second := d.AllocateUnifiedMemory(other, 8192)
	if first != second {
		t.Fatal("test must reuse a VPN across PIDs")
	}
	expected := []uint32{7, 8, 9}
	d.MemCopyH2D(other, second, expected)
	if err := d.FreeMemory(a, first); err != nil {
		t.Fatal(err)
	}
	if d.FreeMemory(a, first) == nil {
		t.Fatal("double free accepted")
	}
	out := make([]uint32, 3)
	d.MemCopyD2H(other, out, second)
	for i := range out {
		if out[i] != expected[i] {
			t.Fatal("freed another PID's frame")
		}
	}
	if err := d.FreeMemory(other, second); err != nil {
		t.Fatal(err)
	}
}

func TestGRITHostOverwriteRevokesReplicas(t *testing.T) {
	cfg := grit.DefaultConfig()
	cfg.Mode = grit.ModeDUP
	b := MakeR9NanoBuilder().WithNumGPU(2).WithGRITConfig(cfg)
	b.numSAPerGPU = 1
	b.numCUPerSA = 1
	p := b.Build()
	d := p.Driver
	d.Run()
	defer d.Terminate()
	ctx := d.Init()
	src := d.AllocateUnifiedMemory(ctx, 4096)
	dst := d.AllocateUnifiedMemory(ctx, 4096)
	input := make([]uint32, 64)
	for i := range input {
		input[i] = 1
	}
	d.MemCopyH2D(ctx, src, input)
	d.SelectGPU(ctx, 2)
	d.MemCopyD2D(ctx, dst, src, 256)
	for i := range input {
		input[i] = uint32(123 + i)
	}
	d.MemCopyH2D(ctx, src, input)
	d.MemCopyD2D(ctx, dst, src, 256)
	out := make([]uint32, 64)
	d.MemCopyD2H(ctx, out, dst)
	for i := range out {
		if out[i] != input[i] {
			t.Fatal("host overwrite left a stale replica", i, out[i], input[i])
		}
	}
}
