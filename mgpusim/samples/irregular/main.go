// irregular runs real GCN3 kernels with independent CPU verification.
package main

import (
	"flag"
	"fmt"
	"github.com/sarchlab/mgpusim/v3/benchmarks/irregular"
	"github.com/sarchlab/mgpusim/v3/benchmarks/irregular/graphio"
	"github.com/sarchlab/mgpusim/v3/benchmarks/irregular/reference"
	"github.com/sarchlab/mgpusim/v3/samples/runner"
	"math"
	"strconv"
	"strings"
)

func main() {
	work := flag.String("workload", "bfs", "bfs, spmv, pagerank, gather, sssp, cc")
	gen := flag.String("generator", "path", "path, cycle, star, grid, disconnected, empty, uniform, skewed, powerlaw, high-locality, low-locality")
	input := flag.String("input", "", "Local .el/.wel/.mtx file; -n preserves isolates in edge lists")
	n := flag.Int("n", 64, "Vertices/rows or gather accesses")
	seed := flag.Int64("seed", 17, "Deterministic seed")
	source := flag.Int("source", 0, "BFS/SSSP source")
	iters := flag.Int("iterations", 0, "PR defaults to 10; graph algorithms default to n passes")
	damp := flag.Float64("damping", .85, "PageRank damping in (0,1)")
	profile := flag.String("profile", "all-shared-read", "Gather: private, all-shared-read, producer-consumer, alternating-readwrite, phase-shift, uniform, skewed, hotset, high-locality, low-locality")
	output := flag.String("result-json", "", "Write verified outputs and input provenance")
	schedule := flag.String("schedule", "physical", "physical destination shards or unified virtual scheduler")
	flag.Parse()
	if flag.NArg() != 0 {
		panic("unexpected positional arguments")
	}
	switch *work {
	case "bfs", "spmv", "pagerank", "gather", "sssp", "cc":
	default:
		panic("unknown workload")
	}
	if *n < 1 || *n > 1<<24 || *iters < 0 || *damp <= 0 || *damp >= 1 || math.IsNaN(*damp) {
		panic("invalid dimensions/iterations/damping")
	}
	if *schedule != "physical" && *schedule != "unified" {
		panic("unknown schedule")
	}
	if *work == "gather" && *schedule != "physical" {
		panic("controlled gather profiles require physical destination shards")
	}
	switch *profile {
	case "private", "all-shared-read", "producer-consumer", "alternating-readwrite", "phase-shift", "uniform", "skewed", "hotset", "high-locality", "low-locality":
	default:
		panic("unknown gather profile")
	}
	var g graphio.CSR
	var e error
	if *input != "" {
		g, e = graphio.Load(*input, *n)
	} else {
		g, e = graphio.Generate(*gen, *n, *seed)
	}
	if e != nil {
		panic(e)
	}
	if *work != "spmv" && g.N != g.Cols {
		panic("graph workloads require square input")
	}
	if (*work == "bfs" || *work == "sssp") && (*source < 0 || *source >= g.N) {
		panic("invalid source")
	}
	if *work == "sssp" {
		for _, v := range g.Val {
			if v <= 0 || v != float32(math.Trunc(float64(v))) || float64(v)*float64(g.N) >= float64(reference.INF) {
				panic("SSSP requires positive integer weights with bounded path sums")
			}
		}
	}
	if *work == "pagerank" && *iters == 0 {
		*iters = 10
	}
	idsText := flag.Lookup("unified-gpus").Value.String()
	if idsText == "" {
		idsText = flag.Lookup("gpus").Value.String()
	}
	if idsText == "" {
		idsText = "1"
	}
	ids := []int{}
	last := 0
	for _, s := range strings.Split(idsText, ",") {
		id, e := strconv.Atoi(s)
		if e != nil || id <= last {
			panic("GPU IDs must be positive and strictly increasing")
		}
		ids = append(ids, id)
		last = id
	}
	if *schedule == "unified" && flag.Lookup("unified-gpus").Value.String() == "" {
		panic("unified schedule requires -unified-gpus")
	}
	if len(ids) > 1 && *schedule == "physical" && flag.Lookup("grit-mode").Value.String() == "off" && flag.Lookup("use-unified-memory").Value.String() == "true" {
		panic("legacy UVM does not preserve shared physical-GPU results in mode off; omit -use-unified-memory for the static-memory baseline")
	}
	fmt.Printf("workload=%s input_sha256=%s physical_gpus=%v schedule=%s\n", *work, g.Hash(), ids, *schedule)
	r := new(runner.Runner).Init()
	b := irregular.NewBenchmark(r.Driver(), irregular.Options{Workload: *work, Graph: g, Profile: *profile, Output: *output, Schedule: *schedule, Source: *source, Iterations: *iters, Damping: float32(*damp), Seed: *seed, Physical: ids})
	r.AddBenchmark(b)
	r.Run()
}
