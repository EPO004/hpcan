package irregular

import (
	"bytes"
	"debug/elf"
	"testing"
	"unsafe"
)

func TestABI(t *testing.T) {
	var g graphArgs
	var r rankArgs
	var a gatherArgs
	if unsafe.Offsetof(g.Start) != 40 || unsafe.Offsetof(g.Mode) != 48 || unsafe.Offsetof(r.Damping) != 52 || unsafe.Offsetof(a.Start) != 24 {
		t.Fatal("kernel ABI changed")
	}
	for _, name := range []string{"graph_step", "sssp_step", "cc_step", "rank_step", "gather_step", "produce_step"} {
		b := new(Benchmark)
		k := b.load(name)
		if k.KernelCodeEntryByteOffset != 256 || k.WIPrivateSegmentByteSize != 0 {
			t.Fatalf("%s incompatible header", name)
		}
	}
	f, e := elf.NewFile(bytes.NewReader(code))
	if e != nil {
		t.Fatal(e)
	}
	defer f.Close()
	if f.Section(".text").Addr != f.Section(".text").Offset {
		t.Fatal("v3 loader requires matching text VA and offset")
	}
}
func TestPartitions(t *testing.T) {
	for _, n := range []int{1, 3, 8, 65, 1023, 1024, 1025} {
		for _, k := range []int{1, 2, 4} {
			s := Shards(n, k)
			end := 0
			for _, p := range s {
				if p[0] != end || p[1] < p[0] {
					t.Fatal(s)
				}
				end = p[1]
			}
			if end != n {
				t.Fatal(s)
			}
		}
	}
}
