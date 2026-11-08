package reference

import (
	"github.com/sarchlab/mgpusim/v3/benchmarks/irregular/graphio"
	"math"
	"testing"
)

func TestOracles(t *testing.T) {
	g, _ := graphio.New(5, 5, []graphio.Edge{{From: 0, To: 1, Value: 5}, {From: 0, To: 2, Value: 1}, {From: 2, To: 1, Value: 1}, {From: 1, To: 3, Value: 2}})
	b := BFS(g, 0)
	d := SSSP(g, 0)
	c := CC(g)
	if b[3] != 2 || b[4] != INF || d[3] != 4 || d[4] != INF || c[3] != 0 || c[4] != 4 {
		t.Fatal(b, d, c)
	}
	p := PageRank(g, 10, .85)
	s := float64(0)
	for _, v := range p {
		s += float64(v)
	}
	if math.Abs(s-1) > 1e-6 {
		t.Fatal(s)
	}
}
