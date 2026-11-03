package grit

import "testing"

func TestAllPolicyTransitions(t *testing.T) {
	for _, old := range []Scheme{SchemeOnTouch, SchemeAccessCounter, SchemeDuplication} {
		for _, next := range []Scheme{SchemeOnTouch, SchemeAccessCounter, SchemeDuplication} {
			for _, access := range []AccessKind{AccessRead, AccessWrite} {
				d := NewDirectory()
				k := PageKey{1, 1}
				d.OnAllocate(k, 1, 4096)
				s, _ := d.BeginOperation(k)
				s.Scheme = old
				if old == SchemeDuplication {
					s.Replicas[1] = Replica{GPUID: 1, PAddr: 4096, Valid: true, Generation: 1}
					s.Replicas[3] = Replica{GPUID: 3, PAddr: 12288, Valid: true, Generation: 1}
				}
				if err := d.CommitOperation(k, s); err != nil {
					t.Fatal(err)
				}
				s, _ = d.BeginOperation(k)
				p, err := PlanPlacement(s, 2, access, next, 8192)
				if err != nil {
					t.Fatalf("%d -> %d access %d: %v", old, next, access, err)
				}
				if err = d.CommitOperation(k, p.After); err != nil {
					t.Fatal(err)
				}
				if p.Copy && p.Source != 4096 {
					t.Fatal("copy not authoritative")
				}
				if next == SchemeDuplication && access == AccessRead {
					if !p.Replication || len(p.After.Replicas) < 2 {
						t.Fatal("missing replica")
					}
				}
				if next == SchemeAccessCounter && p.After.OwnerPAddr != 4096 {
					t.Fatal("AC moved on fault")
				}
			}
		}
	}
}

// Apply plans to independent physical byte images, verifying data, not counters.
func TestPlacementPreservesBytes(t *testing.T) {
	d := NewDirectory()
	k := PageKey{1, 1}
	d.OnAllocate(k, 1, 4096)
	frames := map[uint64][]byte{4096: {7, 0, 0, 0}}
	nextAddr := uint64(8192)
	apply := func(gpu uint64, a AccessKind, scheme Scheme) {
		t.Helper()
		s, _ := d.BeginOperation(k)
		p, err := PlanPlacement(s, gpu, a, scheme, nextAddr)
		if err != nil {
			t.Fatal(err)
		}
		if p.Copy {
			frames[p.Destination] = append([]byte(nil), frames[p.Source]...)
			nextAddr += 4096
		}
		if err = d.CommitOperation(k, p.After); err != nil {
			t.Fatal(err)
		}
		for _, r := range p.Retire {
			delete(frames, r.PAddr)
		}
	}
	apply(2, AccessRead, SchemeOnTouch)
	r, _ := d.LookupMapping(k, 2, AccessRead)
	if frames[r.PAddr][0] != 7 {
		t.Fatal("OT data loss")
	}
	apply(1, AccessRead, SchemeOnTouch)
	r, _ = d.LookupMapping(k, 1, AccessWrite)
	frames[r.PAddr][0] = 1
	apply(2, AccessRead, SchemeDuplication)
	apply(3, AccessRead, SchemeDuplication)
	if len(frames) != 3 {
		t.Fatal("replicas not independent")
	}
	apply(2, AccessWrite, SchemeDuplication)
	r, ok := d.LookupMapping(k, 2, AccessWrite)
	if !ok || frames[r.PAddr][0] != 1 {
		t.Fatal("collapse lost authoritative data")
	}
	frames[r.PAddr][0] = 2
	if len(frames) != 1 {
		t.Fatal("collapse leaked frames")
	}
	apply(3, AccessRead, SchemeDuplication)
	r, _ = d.LookupMapping(k, 3, AccessRead)
	if frames[r.PAddr][0] != 2 {
		t.Fatal("stale read after collapse")
	}
}

func TestACRemoteMappingAndThresholdMigration(t *testing.T) {
	d := NewDirectory()
	k := PageKey{1, 33}
	d.OnAllocate(k, 1, 4096)
	s, _ := d.BeginOperation(k)
	p, err := PlanPlacement(s, 2, AccessRead, SchemeAccessCounter, 8192)
	if err != nil {
		t.Fatal(err)
	}
	if p.Copy {
		t.Fatal("AC fault must not move data")
	}
	d.CommitOperation(k, p.After)
	r, ok := d.LookupMapping(k, 2, AccessRead)
	if !ok || r.PAddr != 4096 {
		t.Fatal("remote alias missing")
	}
	counters := NewAccessCounters(256)
	for i := 1; i <= 256; i++ {
		if counters.Count(k, 2) != (i == 256) {
			t.Fatal("counter threshold", i)
		}
	}
	s, _ = d.BeginOperation(k)
	p, err = PlanCounterMigration(s, 2, 8192)
	if err != nil {
		t.Fatal(err)
	}
	if !p.Copy || p.After.Scheme != SchemeAccessCounter || p.After.HomeGPU != 2 || p.After.Epoch != s.Epoch {
		t.Fatal("bad AC threshold migration")
	}
	bytes := map[uint64][]byte{4096: {7, 0, 0, 0}}
	bytes[p.Destination] = append([]byte(nil), bytes[p.Source]...)
	if err = d.CommitOperation(k, p.After); err != nil {
		t.Fatal(err)
	}
	r, _ = d.LookupMapping(k, 2, AccessRead)
	if bytes[r.PAddr][0] != 7 {
		t.Fatal("AC migration lost bytes")
	}
}

func TestACRemoteWriteDoesNotGenerateProtectionFault(t *testing.T) {
	d := NewDirectory()
	k := PageKey{1, 1}
	d.OnAllocate(k, 1, 4096)
	s, _ := d.BeginOperation(k)
	p, err := PlanPlacement(s, 2, AccessRead, SchemeAccessCounter, 8192)
	if err != nil {
		t.Fatal(err)
	}
	d.CommitOperation(k, p.After)
	if d.CheckAccess(k, 2, AccessWrite, 0) != AccessAllowed {
		t.Fatal("AC write incorrectly requires DUP protection fault")
	}
}
