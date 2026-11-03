package grit

import "testing"

func TestFrameReservationsAndVictims(t *testing.T) {
	p := NewFramePool()
	if err := p.AddGPU(1, []uint64{4096, 8192}); err != nil {
		t.Fatal(err)
	}
	a, ok := p.Reserve(1, PageKey{1, 1})
	if !ok {
		t.Fatal("reserve")
	}
	b, ok := p.Reserve(1, PageKey{1, 2})
	if !ok {
		t.Fatal("reserve second")
	}
	if _, ok = p.Reserve(1, PageKey{1, 3}); ok {
		t.Fatal("capacity exceeded")
	}
	if _, ok = p.Victim(1, nil); ok {
		t.Fatal("evicted inflight copy")
	}
	p.Commit(a.PAddr, true)
	p.Commit(b.PAddr, false)
	p.Touch(a.PAddr, true)
	v, ok := p.Victim(1, nil)
	if !ok || v.PAddr != b.PAddr {
		t.Fatal("not LRU")
	}
	if _, ok = p.Reserve(1, PageKey{1, 3}); ok {
		t.Fatal("victim selected before release")
	}
	p.Release(v.PAddr)
	if p.Release(v.PAddr) == nil {
		t.Fatal("double release")
	}
	if _, ok = p.Reserve(1, PageKey{1, 3}); !ok {
		t.Fatal("cannot reuse")
	}
	used, reserved, peak := p.Occupancy()
	if used != 2 || reserved != 1 || peak != 2 {
		t.Fatal("occupancy", used, reserved, peak)
	}
}
