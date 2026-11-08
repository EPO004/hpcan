// Package reference contains independent CPU correctness oracles.
package reference

import (
	"github.com/sarchlab/mgpusim/v3/benchmarks/irregular/graphio"
	"math"
)

const INF uint32 = 0x3fffffff

func BFS(g graphio.CSR, source int) []uint32 {
	d := make([]uint32, g.N)
	for i := range d {
		d[i] = INF
	}
	d[source] = 0
	q := []int{source}
	for len(q) > 0 {
		u := q[0]
		q = q[1:]
		for j := g.Row[u]; j < g.Row[u+1]; j++ {
			v := g.Col[j]
			if d[v] == INF {
				d[v] = d[u] + 1
				q = append(q, int(v))
			}
		}
	}
	return d
}

// Dijkstra uses a deliberately simple O(n^2) independent minimum selection.
func SSSP(g graphio.CSR, source int) []uint32 {
	d := make([]uint32, g.N)
	used := make([]bool, g.N)
	for i := range d {
		d[i] = INF
	}
	d[source] = 0
	for k := 0; k < g.N; k++ {
		u := -1
		for v := range d {
			if !used[v] && (u < 0 || d[v] < d[u]) {
				u = v
			}
		}
		if u < 0 || d[u] == INF {
			break
		}
		used[u] = true
		for j := g.Row[u]; j < g.Row[u+1]; j++ {
			v := g.Col[j]
			w := uint32(g.Val[j])
			if w <= INF-d[u] && d[v] > d[u]+w {
				d[v] = d[u] + w
			}
		}
	}
	return d
}
func CC(g graphio.CSR) []uint32 {
	p := make([]uint32, g.N)
	for i := range p {
		p[i] = uint32(i)
	}
	var root func(uint32) uint32
	root = func(v uint32) uint32 {
		if p[v] != v {
			p[v] = root(p[v])
		}
		return p[v]
	}
	for u := 0; u < g.N; u++ {
		for j := g.Row[u]; j < g.Row[u+1]; j++ {
			a, b := root(uint32(u)), root(g.Col[j])
			if a < b {
				p[b] = a
			} else {
				p[a] = b
			}
		}
	}
	for i := range p {
		p[i] = root(uint32(i))
	}
	return p
}
func SpMV(g graphio.CSR, x []float32) []float32 {
	y := make([]float32, g.N)
	for u := range y {
		sum := float64(0)
		for j := g.Row[u]; j < g.Row[u+1]; j++ {
			sum += float64(g.Val[j]) * float64(x[g.Col[j]])
		}
		y[u] = float32(sum)
	}
	return y
}

// PageRank is outgoing-edge push in float64, independent of GPU pull layout.
func PageRank(g graphio.CSR, iters int, d float32) []float32 {
	x := make([]float64, g.N)
	for i := range x {
		x[i] = 1 / float64(g.N)
	}
	for k := 0; k < iters; k++ {
		sink := float64(0)
		for u := range x {
			if g.Row[u] == g.Row[u+1] {
				sink += x[u]
			}
		}
		y := make([]float64, g.N)
		for v := range y {
			y[v] = (1-float64(d))/float64(g.N) + float64(d)*sink/float64(g.N)
		}
		for u := range x {
			degree := g.Row[u+1] - g.Row[u]
			if degree > 0 {
				for j := g.Row[u]; j < g.Row[u+1]; j++ {
					y[g.Col[j]] += float64(d) * x[u] / float64(degree)
				}
			}
		}
		x = y
	}
	y := make([]float32, g.N)
	for i := range y {
		y[i] = float32(x[i])
	}
	return y
}
func Errors(got, want []float32) (abs, rel float64) {
	for i, v := range got {
		a := math.Abs(float64(v) - float64(want[i]))
		r := a / math.Max(1e-20, math.Abs(float64(want[i])))
		abs = math.Max(abs, a)
		rel = math.Max(rel, r)
	}
	return
}

// RelaxOutgoing checks one synchronous iteration using outgoing-edge pushes,
// independently of the GPU's transposed incoming-neighbor gather layout.
// Final results are still checked by BFS, Dijkstra, and DSU respectively.
func RelaxOutgoing(g graphio.CSR, old []uint32, workload string) []uint32 {
	next := append([]uint32(nil), old...)
	for u := 0; u < g.N; u++ {
		for j := g.Row[u]; j < g.Row[u+1]; j++ {
			v := g.Col[j]
			candidate := old[u]
			if workload != "cc" {
				weight := uint32(1)
				if workload == "sssp" {
					weight = uint32(g.Val[j])
				}
				if candidate == INF || weight > INF-candidate {
					continue
				}
				candidate += weight
			}
			if candidate < next[v] {
				next[v] = candidate
			}
		}
	}
	return next
}
