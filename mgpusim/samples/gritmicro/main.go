// gritmicro alternates compiled GPU read/write kernels across physical GPUs.
// It exercises shared pages rather than relying on a private-data workload.
package main

import (
	"flag"
	"fmt"
	"github.com/sarchlab/mgpusim/v3/driver"
	"github.com/sarchlab/mgpusim/v3/samples/runner"
)

var rounds = flag.Int("rounds", 8, "Number of alternating shared-page kernel copies (at least 4)")

type benchmark struct {
	d                *driver.Driver
	ctx              *driver.Context
	expected, actual []uint32
}

func (*benchmark) SelectGPU([]int)   {}
func (*benchmark) SetUnifiedMemory() {}
func (b *benchmark) Run() {
	if *rounds < 4 {
		panic("-rounds must be at least four")
	}
	b.ctx = b.d.Init()
	src := b.d.AllocateUnifiedMemory(b.ctx, 4096)
	dst := b.d.AllocateUnifiedMemory(b.ctx, 4096)
	b.expected = make([]uint32, 64)
	b.actual = make([]uint32, 64)
	for i := range b.expected {
		b.expected[i] = uint32(7 + i*13)
	}
	b.d.MemCopyH2D(b.ctx, src, b.expected)
	b.d.MemCopyH2D(b.ctx, dst, make([]uint32, 64))
	for i := 0; i < *rounds; i++ {
		b.d.SelectGPU(b.ctx, 1+i%b.d.GetNumGPUs())
		b.d.MemCopyD2D(b.ctx, dst, src, 256)
	}
	// Collapse a page that was previously read-shared and observe it on a peer.
	b.d.SelectGPU(b.ctx, 2)
	b.d.MemCopyD2D(b.ctx, src, dst, 256)
	b.d.SelectGPU(b.ctx, 1)
	b.d.MemCopyD2D(b.ctx, dst, src, 256)
	b.d.MemCopyD2H(b.ctx, b.actual, dst)
}
func (b *benchmark) Verify() {
	for i, v := range b.actual {
		if v != b.expected[i] {
			panic(fmt.Sprintf("GRIT data mismatch at %d: got %d expected %d", i, v, b.expected[i]))
		}
	}
	fmt.Println("GRIT microbenchmark verification: Passed!")
}
func main() {
	flag.Parse()
	r := new(runner.Runner).Init()
	if r.Driver().GetNumGPUs() < 2 {
		panic("gritmicro requires at least two GPUs")
	}
	r.AddBenchmark(&benchmark{d: r.Driver()})
	r.Run()
}
