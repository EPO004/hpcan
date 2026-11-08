package graphio

import (
	"strings"
	"testing"
)

func TestInputs(t *testing.T) {
	for _, name := range []string{"path", "star", "cycle", "grid", "disconnected", "empty", "uniform", "skewed", "low-locality"} {
		g, e := Generate(name, 17, 17)
		if e != nil {
			t.Fatal(e)
		}
		h, _ := Generate(name, 17, 17)
		if g.Hash() != h.Hash() || g.Row[17] != uint32(len(g.Col)) {
			t.Fatal(name)
		}
	}
}
func TestMatrixMarket(t *testing.T) {
	g, e := Parse(strings.NewReader("%%MatrixMarket matrix coordinate real symmetric\n% test\n3 3 3\n1 2 2\n1 2 3\n3 3 4\n"), ".mtx", 0)
	if e != nil || len(g.Col) != 5 || g.Row[1] != 2 {
		t.Fatalf("%+v %v", g, e)
	}
}
func TestReject(t *testing.T) {
	for _, s := range []string{"-1 0", "0 9", "0 x", "0 1 2", "0"} {
		if _, e := Parse(strings.NewReader(s), ".el", 3); e == nil {
			t.Fatal(s)
		}
	}
	for _, s := range []string{"", "%%MatrixMarket matrix array real general\n2 2 0", "%%MatrixMarket matrix coordinate pattern general\n2 2 2\n1 2"} {
		if _, e := Parse(strings.NewReader(s), ".mtx", 0); e == nil {
			t.Fatal(s)
		}
	}
}
func TestFinalLineAndIsolates(t *testing.T) {
	g, e := Parse(strings.NewReader("# comment\n0 1"), ".el", 4)
	if e != nil || g.N != 4 || g.Row[4] != 1 {
		t.Fatal(g, e)
	}
}
