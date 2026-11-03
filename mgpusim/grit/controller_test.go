package grit

import "testing"

func TestFaultCoalescingAndFalseTLBMiss(t *testing.T) {
	cfg := DefaultConfig()
	cfg.Mode = ModeGRIT
	c, err := NewController(cfg)
	if err != nil {
		t.Fatal(err)
	}
	k := PageKey{1, 8}
	c.Allocate(k, 1, 4096)
	if _, ok, err := c.Admit(Fault{"local", k, 1, 1, AccessRead, FaultLocalPage}); err != nil || ok {
		t.Fatal("counted local TLB miss", err)
	}
	if _, ok, err := c.Admit(Fault{"stale", k, 1, 99, AccessRead, FaultLocalPage}); err != nil || ok {
		t.Fatal("counted stale TLB generation as fault", err)
	}
	f := Fault{"r1", k, 2, 1, AccessRead, FaultLocalPage}
	r, ok, err := c.Admit(f)
	if err != nil || !ok {
		t.Fatal(err)
	}
	f.ID = "r2"
	r, ok, err = c.Admit(f)
	if err != nil || !ok || len(r.Waiters) != 2 {
		t.Fatal("coalescing", r, err)
	}
	e, _ := c.PA.Snapshot(k)
	if e.Faults != 1 {
		t.Fatal("double count")
	}
	f.ID = "w1"
	f.Access = AccessWrite
	if _, ok, _ := c.Admit(f); ok {
		t.Fatal("write coalesced behind read")
	}
	if c.Free(k) == nil {
		t.Fatal("freed inflight fault")
	}
	s, _ := c.Directory.Lookup(k)
	p, err := PlanPlacement(s, 2, AccessRead, r.Scheme, 8192)
	if err != nil {
		t.Fatal(err)
	}
	if err = c.Directory.CommitOperation(k, p.After); err != nil {
		t.Fatal(err)
	}
	r, _, err = c.Complete(k)
	if err != nil || len(r.Waiters) != 2 {
		t.Fatal("lost waiter", err)
	}
	if c.Metrics().LocalPageFaults != 1 {
		t.Fatal("fault metric")
	}
	if err = c.Free(k); err != nil {
		t.Fatal(err)
	}
	c.Allocate(k, 1, 12288)
	if _, ok := c.PA.Snapshot(k); ok {
		t.Fatal("reallocation inherited PA history")
	}
}
