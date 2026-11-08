// Package irregular connects deterministic sparse workloads to the real driver.
package irregular

import (
	"crypto/sha256"
	_ "embed"
	"encoding/binary"
	"encoding/json"
	"fmt"
	"github.com/sarchlab/mgpusim/v3/benchmarks/irregular/graphio"
	"github.com/sarchlab/mgpusim/v3/benchmarks/irregular/reference"
	"github.com/sarchlab/mgpusim/v3/benchmarks/shoc/spmv"
	"github.com/sarchlab/mgpusim/v3/driver"
	"github.com/sarchlab/mgpusim/v3/insts"
	"github.com/sarchlab/mgpusim/v3/kernels"
	"math"
	"math/rand"
	"os"
	"sort"
)

//go:embed kernels.hsaco
var code []byte

type Options struct {
	Workload, Profile, Output, Schedule string
	Graph                               graphio.CSR
	Source, Iterations                  int
	Damping                             float32
	Seed                                int64
	Physical                            []int
}
type Phase struct {
	Name        string
	Epoch       int
	IndexSHA256 string
	DataSHA256  string
	InputPages  map[int][]uint64
}
type Benchmark struct {
	Phases                 []Phase
	d                      *driver.Driver
	ctx                    *driver.Context
	devices                []int
	unified                bool
	Options                Options
	ActualInt, ExpectedInt []uint32
	Actual, Expected       []float32
	Launches               map[int]int
	Iterations             int
	Footprint              uint64
	MaxAbs, MaxRel         float64
}

func NewBenchmark(d *driver.Driver, o Options) *Benchmark {
	return &Benchmark{d: d, Options: o, Launches: map[int]int{}}
}
func (b *Benchmark) SelectGPU(ids []int) { b.devices = ids }
func (b *Benchmark) SetUnifiedMemory()   { b.unified = true }
func (b *Benchmark) alloc(bytes uint64) driver.Ptr {
	if bytes < 4 {
		bytes = 4
	}
	b.Footprint += ((bytes + 4095) / 4096) * 4096
	if b.unified {
		return b.d.AllocateUnifiedMemory(b.ctx, bytes)
	}
	return b.d.AllocateMemory(b.ctx, bytes)
}
func (b *Benchmark) put(v interface{}, length int) driver.Ptr {
	p := b.alloc(uint64(length) * 4)
	if length > 0 {
		b.d.MemCopyH2D(b.ctx, p, v)
	}
	return p
}
func (b *Benchmark) load(name string) *insts.HsaCo {
	k := kernels.LoadProgramFromMemory(code, name)
	if k == nil {
		panic("missing kernel " + name)
	}
	return k
}

// Shards are consecutive, nonoverlapping destination intervals, including tails.
func Shards(n, k int) [][2]int {
	r := make([][2]int, k)
	for i := range r {
		r[i] = [2]int{n * i / k, n * (i + 1) / k}
	}
	return r
}
func (b *Benchmark) launch(k *insts.HsaCo, n int, args func(int, int) interface{}) {
	ids := b.Options.Physical
	if b.Options.Schedule == "unified" {
		ids = b.devices
	}
	shards := Shards(n, len(ids))
	qs := []*driver.CommandQueue{}
	for i, id := range ids {
		lo, hi := shards[i][0], shards[i][1]
		if hi == lo {
			continue
		}
		b.d.SelectGPU(b.ctx, id)
		q := b.d.CreateCommandQueue(b.ctx)
		b.d.EnqueueLaunchKernel(q, k, [3]uint32{uint32((hi - lo + 63) / 64 * 64), 1, 1}, [3]uint16{64, 1, 1}, args(lo, hi))
		qs = append(qs, q)
		b.Launches[id]++
	}
	for _, q := range qs {
		b.d.DrainCommandQueue(q)
	}
}

type graphArgs struct {
	Row, Col, Weight, Old, Out driver.Ptr
	Start, End, Mode, Pad      uint32
}
type rankArgs struct {
	Row, Col, Degree, Old, Out driver.Ptr
	Start, End, N              uint32
	Damping                    float32
}
type gatherArgs struct {
	Index, Data, Out driver.Ptr
	Start, End       uint32
}
type produceArgs struct {
	Data                   driver.Ptr
	Start, End, Epoch, Pad uint32
}

func (b *Benchmark) Run() {
	b.ctx = b.d.Init()
	b.d.SelectGPU(b.ctx, b.Options.Physical[0])
	switch b.Options.Workload {
	case "spmv":
		b.spmv()
	case "pagerank":
		b.rank()
	case "gather":
		b.gather()
	default:
		b.graph()
	}
}
func (b *Benchmark) graph() {
	o := b.Options
	g := o.Graph
	n := g.N
	if o.Workload == "cc" {
		g = g.Undirected()
		b.ExpectedInt = reference.CC(g)
	} else if o.Workload == "bfs" {
		b.ExpectedInt = reference.BFS(g, o.Source)
	} else {
		b.ExpectedInt = reference.SSSP(g, o.Source)
	}
	incoming := g.Transpose()
	weights := make([]uint32, len(incoming.Val))
	for i, v := range incoming.Val {
		weights[i] = uint32(v)
	}
	row := b.put(incoming.Row, len(incoming.Row))
	col := b.put(incoming.Col, len(incoming.Col))
	w := b.put(weights, len(weights))
	state := make([]uint32, n)
	for i := range state {
		state[i] = reference.INF
		if o.Workload == "cc" {
			state[i] = uint32(i)
		}
	}
	if o.Workload != "cc" {
		state[o.Source] = 0
	}
	old := b.put(state, n)
	out := b.alloc(uint64(n) * 4)
	name := "graph_step"
	if o.Workload == "sssp" {
		name = "sssp_step"
	}
	if o.Workload == "cc" {
		name = "cc_step"
	}
	k := b.load(name)
	mode := uint32(0)
	if o.Workload == "sssp" {
		mode = 1
	}
	if o.Workload == "cc" {
		mode = 2
	}
	cap := n
	if o.Iterations > 0 {
		cap = o.Iterations
	}
	b.ActualInt = make([]uint32, n)
	for it := 0; it < cap; it++ {
		b.launch(k, n, func(lo, hi int) interface{} {
			return &graphArgs{row, col, w, old, out, uint32(lo), uint32(hi), mode, 0}
		})
		b.d.MemCopyD2H(b.ctx, b.ActualInt, out)
		if n <= 128 {
			want := reference.RelaxOutgoing(g, state, o.Workload)
			for i, v := range b.ActualInt {
				if v != want[i] {
					panic(fmt.Sprintf("%s iteration %d vertex %d got %d want %d", o.Workload, it+1, i, v, want[i]))
				}
			}
		}

		same := true
		for i, v := range b.ActualInt {
			if v != state[i] {
				same = false
			}
		}
		copy(state, b.ActualInt)
		old, out = out, old
		b.Iterations++
		if same {
			return
		}
	}
	if o.Workload == "cc" {
		panic("CC did not reach an unchanged iteration within cap")
	}
	// A sufficient n-1 relaxation bound is valid even if a final unchanged pass
	// was not requested. A truncated result is never accepted by Verify.
}
func (b *Benchmark) spmv() {
	g := b.Options.Graph
	x := make([]float32, g.Cols)
	for i := range x {
		x[i] = float32(i%13+1) / 8
	}
	b.Expected = reference.SpMV(g, x)
	b.Actual = make([]float32, g.N)
	val := b.put(g.Val, len(g.Val))
	vec := b.put(x, len(x))
	cols := b.put(g.Col, len(g.Col))
	row := b.put(g.Row, len(g.Row))
	out := b.alloc(uint64(g.N) * 4)
	k := spmv.LoadCSRKernel()
	// Legacy kernel uses global IDs. Offset the CSR row pointer and output while
	// retaining absolute CSR edge offsets; each launch starts its own row ID at 0.
	b.launch(k, g.N, func(lo, hi int) interface{} {
		return &spmv.KernelArgs{Val: val, Vec: vec, Cols: cols, RowDelimiters: row + driver.Ptr(lo*4), Dim: int32(hi - lo), Out: out + driver.Ptr(lo*4)}
	})
	b.d.MemCopyD2H(b.ctx, b.Actual, out)
	b.Iterations = 1
}
func (b *Benchmark) rank() {
	o := b.Options
	g := o.Graph
	n := g.N
	in := g.Transpose()
	degree := make([]uint32, n)
	state := make([]float32, n)
	for i := range degree {
		degree[i] = g.Row[i+1] - g.Row[i]
		state[i] = 1 / float32(n)
	}
	row := b.put(in.Row, len(in.Row))
	col := b.put(in.Col, len(in.Col))
	deg := b.put(degree, n)
	old := b.put(state, n)
	out := b.alloc(uint64(n) * 4)
	k := b.load("rank_step")
	b.Actual = make([]float32, n)
	for it := 0; it < o.Iterations; it++ {
		b.launch(k, n, func(lo, hi int) interface{} {
			return &rankArgs{row, col, deg, old, out, uint32(lo), uint32(hi), uint32(n), o.Damping}
		})
		old, out = out, old
		b.Iterations++
		if n <= 128 {
			b.d.MemCopyD2H(b.ctx, b.Actual, old)
			want := reference.PageRank(g, it+1, o.Damping)
			checkFloat(b.Actual, want)
		}
	}
	b.d.MemCopyD2H(b.ctx, b.Actual, old)
	b.Expected = reference.PageRank(g, o.Iterations, o.Damping)
}
func checkFloat(got, want []float32) {
	for i, v := range got {
		err := math.Abs(float64(v - want[i]))
		if math.IsNaN(float64(v)) || math.IsInf(float64(v), 0) || err > 1e-5+1e-4*math.Abs(float64(want[i])) {
			panic(fmt.Sprintf("float mismatch %d: got %g want %g", i, v, want[i]))
		}
	}
}
func (b *Benchmark) gather() {
	o := b.Options
	n := o.Graph.N
	ng := len(o.Physical)
	stride := ((n + 1023) / 1024) * 1024
	data := make([]float32, stride*ng)
	for i := range data {
		data[i] = float32((i * 13) % 251)
	}
	dp := b.put(data, len(data))
	out := b.alloc(uint64(n) * 4)
	k := b.load("gather_step")
	producer := b.load("produce_step")
	rng := rand.New(rand.NewSource(o.Seed))
	phases := []string{o.Profile}
	if o.Profile == "phase-shift" {
		phases = []string{"private", "all-shared-read", "alternating-readwrite"}
	}
	for _, phase := range phases {
		rounds := 1
		if phase == "alternating-readwrite" {
			rounds = 4
		}
		for epoch := 0; epoch < rounds; epoch++ {
			if phase == "producer-consumer" || phase == "alternating-readwrite" {
				id := o.Physical[epoch%ng]
				b.d.SelectGPU(b.ctx, id)
				a := produceArgs{dp, 0, uint32(stride), uint32(epoch + 1), 0}
				b.d.LaunchKernel(b.ctx, producer, [3]uint32{uint32((stride + 63) / 64 * 64), 1, 1}, [3]uint16{64, 1, 1}, &a)
				b.Launches[id]++
				for i := 0; i < stride; i++ {
					data[i] = float32((i*13 + (epoch+1)*17) % 251)
				}
			}
			index := make([]uint32, n)
			shards := Shards(n, ng)
			for g, s := range shards {
				for i := s[0]; i < s[1]; i++ {
					j := rng.Intn(stride)
					switch phase {
					case "private":
						j = g*stride + rng.Intn(stride)
					case "skewed", "hotset":
						if rng.Intn(10) < 9 {
							j = rng.Intn(1 + stride/100)
						}
					case "high-locality":
						j = i % stride
					case "low-locality", "uniform", "all-shared-read", "producer-consumer", "alternating-readwrite":
					default:
						panic("unknown gather profile")
					}
					index[i] = uint32(j)
				}
			}
			ih := sha256.New()
			binary.Write(ih, binary.LittleEndian, index)
			dh := sha256.New()
			binary.Write(dh, binary.LittleEndian, data)
			pages := map[int][]uint64{}
			for gi, shard := range shards {
				seen := map[uint64]bool{}
				for i := shard[0]; i < shard[1]; i++ {
					seen[(uint64(dp)+uint64(index[i])*4)/4096] = true
				}
				for p := range seen {
					pages[o.Physical[gi]] = append(pages[o.Physical[gi]], p)
				}
				sort.Slice(pages[o.Physical[gi]], func(i, j int) bool { return pages[o.Physical[gi]][i] < pages[o.Physical[gi]][j] })
			}
			b.Phases = append(b.Phases, Phase{phase, epoch, fmt.Sprintf("%x", ih.Sum(nil)), fmt.Sprintf("%x", dh.Sum(nil)), pages})
			ip := b.put(index, len(index))
			b.launch(k, n, func(lo, hi int) interface{} { return &gatherArgs{ip, dp, out, uint32(lo), uint32(hi)} })
			b.Actual = make([]float32, n)
			b.Expected = make([]float32, n)
			for i, j := range index {
				b.Expected[i] = data[j]*.5 + float32(i%7)
			}
			b.d.MemCopyD2H(b.ctx, b.Actual, out)
			checkFloat(b.Actual, b.Expected)
			b.Iterations++
		}
	}
}
func (b *Benchmark) Verify() {
	if b.ExpectedInt != nil {
		for i, v := range b.ActualInt {
			if v != b.ExpectedInt[i] {
				panic(fmt.Sprintf("%s mismatch at %d got %d want %d (iteration cap %d)", b.Options.Workload, i, v, b.ExpectedInt[i], b.Iterations))
			}
		}
	} else {
		checkFloat(b.Actual, b.Expected)
		b.MaxAbs, b.MaxRel = reference.Errors(b.Actual, b.Expected)
	}
	if b.Options.Workload == "pagerank" {
		sum := float64(0)
		for _, v := range b.Actual {
			if v < 0 {
				panic("negative rank")
			}
			sum += float64(v)
		}
		if math.Abs(sum-1) > 1e-4 {
			panic("PageRank mass mismatch")
		}
	}
	inputHash := b.Options.Graph.Hash()
	if b.Options.Workload == "gather" {
		h := sha256.New()
		for _, p := range b.Phases {
			fmt.Fprint(h, p.Name, p.Epoch, p.IndexSHA256, p.DataSHA256)
		}
		inputHash = fmt.Sprintf("%x", h.Sum(nil))
	}
	report := map[string]interface{}{"phases": b.Phases, "workload": b.Options.Workload, "profile": b.Options.Profile, "sha256": inputHash, "seed": b.Options.Seed, "n": b.Options.Graph.N, "m": len(b.Options.Graph.Col), "physical_gpus": b.Options.Physical, "schedule": b.Options.Schedule, "launches": b.Launches, "iterations": b.Iterations, "footprint_bytes": b.Footprint, "verify_pass": true, "max_abs_error": b.MaxAbs, "max_rel_error": b.MaxRel, "output_uint": b.ActualInt, "output_float": b.Actual}
	p, e := json.MarshalIndent(report, "", "  ")
	if e != nil {
		panic(e)
	}
	if b.Options.Output != "" {
		if e = os.WriteFile(b.Options.Output, p, 0644); e != nil {
			panic(e)
		}
	}
	fmt.Printf("%s verification: Passed! launches=%v max_abs=%g max_rel=%g\n", b.Options.Workload, b.Launches, b.MaxAbs, b.MaxRel)
}
