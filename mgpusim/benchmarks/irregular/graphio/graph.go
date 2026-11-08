// Package graphio provides deterministic, sorted CSR inputs. Duplicate matrix
// entries are retained and summed by SpMV; graph algorithms tolerate duplicates.
package graphio

import (
	"bufio"
	"crypto/sha256"
	"encoding/binary"
	"fmt"
	"io"
	"math"
	"math/rand"
	"os"
	"path/filepath"
	"sort"
	"strconv"
	"strings"
)

type Edge struct {
	From, To int
	Value    float32
}
type CSR struct {
	N, Cols  int
	Row, Col []uint32
	Val      []float32
}

func New(n, cols int, edges []Edge) (CSR, error) {
	g := CSR{N: n, Cols: cols}
	if n < 1 || cols < 1 || n > math.MaxInt32-1 || cols > math.MaxInt32-1 || len(edges) > math.MaxInt32 {
		return g, fmt.Errorf("invalid or oversized dimensions")
	}
	edges = append([]Edge(nil), edges...)
	for _, e := range edges {
		if e.From < 0 || e.From >= n || e.To < 0 || e.To >= cols || math.IsNaN(float64(e.Value)) || math.IsInf(float64(e.Value), 0) {
			return g, fmt.Errorf("invalid edge %+v", e)
		}
	}
	sort.SliceStable(edges, func(i, j int) bool {
		if edges[i].From != edges[j].From {
			return edges[i].From < edges[j].From
		}
		return edges[i].To < edges[j].To
	})
	g.Row = make([]uint32, n+1)
	for _, e := range edges {
		g.Row[e.From+1]++
		g.Col = append(g.Col, uint32(e.To))
		g.Val = append(g.Val, e.Value)
	}
	for i := 1; i <= n; i++ {
		g.Row[i] += g.Row[i-1]
	}
	return g, nil
}
func (g CSR) Edges() []Edge {
	es := make([]Edge, 0, len(g.Col))
	for u := 0; u < g.N; u++ {
		for j := g.Row[u]; j < g.Row[u+1]; j++ {
			es = append(es, Edge{u, int(g.Col[j]), g.Val[j]})
		}
	}
	return es
}
func (g CSR) Transpose() CSR {
	es := g.Edges()
	for i := range es {
		es[i].From, es[i].To = es[i].To, es[i].From
	}
	r, err := New(g.Cols, g.N, es)
	if err != nil {
		panic(err)
	}
	return r
}
func (g CSR) Undirected() CSR {
	es := g.Edges()
	for _, e := range g.Edges() {
		if e.From != e.To {
			es = append(es, Edge{e.To, e.From, e.Value})
		}
	}
	r, err := New(g.N, g.N, es)
	if err != nil {
		panic(err)
	}
	return r
}
func (g CSR) Hash() string {
	h := sha256.New()
	binary.Write(h, binary.LittleEndian, uint64(g.N))
	binary.Write(h, binary.LittleEndian, uint64(g.Cols))
	binary.Write(h, binary.LittleEndian, g.Row)
	binary.Write(h, binary.LittleEndian, g.Col)
	binary.Write(h, binary.LittleEndian, g.Val)
	return fmt.Sprintf("%x", h.Sum(nil))
}
func Generate(name string, n int, seed int64) (CSR, error) {
	if n < 1 || n > 1<<24 {
		return CSR{}, fmt.Errorf("n must be in [1,16777216]")
	}
	r := rand.New(rand.NewSource(seed))
	es := []Edge{}
	add := func(u, v int) { es = append(es, Edge{u, v, float32(1 + (u+v)%7)}) }
	switch name {
	case "path", "high-locality":
		for i := 0; i < n-1; i++ {
			add(i, i+1)
		}
	case "cycle":
		for i := 0; i < n; i++ {
			add(i, (i+1)%n)
		}
	case "star":
		for i := 1; i < n; i++ {
			add(0, i)
		}
	case "disconnected":
		for i := 0; i < n-1; i++ {
			if i != n/2-1 {
				add(i, i+1)
			}
		}
	case "empty":
	case "grid":
		w := int(math.Sqrt(float64(n)))
		for i := 0; i < n; i++ {
			if i%w+1 < w && i+1 < n {
				add(i, i+1)
			}
			if i+w < n {
				add(i, i+w)
			}
		}
	case "uniform", "skewed", "powerlaw":
		for i := 0; i < n; i++ {
			for k := 0; k < 4; k++ {
				v := r.Intn(n)
				if name != "uniform" && k < 3 {
					v = r.Intn(1 + n/16)
				}
				add(i, v)
			}
		}
	case "low-locality":
		p := r.Perm(n)
		for i := 0; i < n-1; i++ {
			add(p[i], p[i+1])
		}
	default:
		return CSR{}, fmt.Errorf("unknown generator %q", name)
	}
	return New(n, n, es)
}
func Load(path string, n int) (CSR, error) {
	f, e := os.Open(path)
	if e != nil {
		return CSR{}, e
	}
	defer f.Close()
	return Parse(f, filepath.Ext(path), n)
}
func Parse(r io.Reader, ext string, n int) (CSR, error) {
	expected, seen := 0, 0
	s := bufio.NewScanner(r)
	s.Buffer(make([]byte, 4096), 1024*1024)
	es := []Edge{}
	cols := n
	line := 0
	mtx := ext == ".mtx"
	dims := false
	sym := false
	field := ""
	if ext != ".el" && ext != ".wel" && !mtx {
		return CSR{}, fmt.Errorf("unsupported format %q", ext)
	}
	for s.Scan() {
		line++
		raw := strings.TrimSpace(s.Text())
		if raw == "" {
			continue
		}
		f := strings.Fields(raw)
		if mtx && field == "" {
			if len(f) != 5 || strings.ToLower(f[0]) != "%%matrixmarket" || f[1] != "matrix" || f[2] != "coordinate" || (f[3] != "real" && f[3] != "integer" && f[3] != "pattern") || (f[4] != "general" && f[4] != "symmetric") {
				return CSR{}, fmt.Errorf("invalid Matrix Market header")
			}
			field = f[3]
			sym = f[4] == "symmetric"
			continue
		}
		if strings.HasPrefix(raw, "#") || strings.HasPrefix(raw, "%") {
			continue
		}
		if mtx && !dims {
			if len(f) != 3 {
				return CSR{}, fmt.Errorf("invalid matrix dimensions")
			}
			var err error
			n, err = strconv.Atoi(f[0])
			if err != nil {
				return CSR{}, err
			}
			cols, err = strconv.Atoi(f[1])
			if err != nil {
				return CSR{}, err
			}
			count, err := strconv.Atoi(f[2])
			if err != nil || count < 0 || count > math.MaxInt32 {
				return CSR{}, fmt.Errorf("invalid entry count")
			}
			if sym && n != cols {
				return CSR{}, fmt.Errorf("symmetric matrix must be square")
			}
			dims = true
			// Keep expected entry count separately from expanded edges.
			expected = count
			seen = 0
			continue
		}
		want := 2
		if ext == ".wel" || (mtx && field != "pattern") {
			want = 3
		}
		if len(f) != want {
			return CSR{}, fmt.Errorf("line %d: expected %d fields", line, want)
		}
		u, e := strconv.Atoi(f[0])
		if e != nil {
			return CSR{}, e
		}
		v, e := strconv.Atoi(f[1])
		if e != nil {
			return CSR{}, e
		}
		val := float32(1)
		if want == 3 {
			x, e := strconv.ParseFloat(f[2], 32)
			if e != nil {
				return CSR{}, e
			}
			val = float32(x)
		}
		if mtx {
			u--
			v--
			seen++
		}
		es = append(es, Edge{u, v, val})
		if sym && u != v {
			es = append(es, Edge{v, u, val})
		}
	}
	if e := s.Err(); e != nil {
		return CSR{}, e
	}
	if mtx && (!dims || seen != expected) {
		return CSR{}, fmt.Errorf("matrix entry count mismatch")
	}
	return New(n, cols, es)
}
