package grit

import "testing"

func TestFaultEpochs(t *testing.T) {
	for _, cache := range []bool{false, true} {
		for _, write := range []bool{false, true} {
			p := NewPATable(cache, 4, 1, 20)
			k := PageKey{1, 9}
			for i := 0; i < 4; i++ {
				a := AccessRead
				if write && i == 2 {
					a = AccessWrite
				}
				s, trigger, delay := p.RecordFault(k, a, FaultLocalPage)
				if delay == 0 || trigger != (i == 3) {
					t.Fatal("bad trigger/delay", i)
				}
				if trigger && s != DecideOnFourthFault(write) {
					t.Fatal("wrong policy")
				}
			}
			if _, ok := p.Snapshot(k); ok {
				t.Fatal("epoch survived")
			}
			p.RecordFault(k, AccessRead, FaultLocalPage)
			e, _ := p.Snapshot(k)
			if e.Faults != 1 || e.SawWrite {
				t.Fatal("epoch not reset")
			}
		}
	}
}
func TestPACacheLRUAndDeletion(t *testing.T) {
	p := NewPATable(true, 4, 1, 20)
	for i := uint64(0); i < 4; i++ {
		p.RecordFault(PageKey{1, i * 16}, AccessRead, FaultLocalPage)
	}
	p.RecordFault(PageKey{1, 0}, AccessWrite, FaultLocalPage)
	p.RecordFault(PageKey{1, 64}, AccessRead, FaultLocalPage)
	if _, ok := p.backing[PageKey{1, 16}]; !ok {
		t.Fatal("did not evict LRU")
	}
	if p.Stats().Writebacks != 1 {
		t.Fatal("writebacks")
	}
	p.RecordFault(PageKey{1, 16}, AccessRead, FaultLocalPage)
	e, _ := p.Snapshot(PageKey{1, 16})
	if e.Faults != 2 {
		t.Fatal("lost refill state")
	}
	p.Remove(PageKey{1, 16})
	p.Flush()
	if _, ok := p.Snapshot(PageKey{1, 16}); ok {
		t.Fatal("freed entry resurrected")
	}
}
func TestPAFullCapacityAndPID(t *testing.T) {
	p := NewPATable(true, 4, 1, 20)
	for i := uint64(0); i < 64; i++ {
		p.RecordFault(PageKey{1, i}, AccessRead, FaultLocalPage)
	}
	if len(p.backing) != 0 {
		t.Fatal("cache hit updated backing")
	}
	for i := uint64(0); i < 64; i++ {
		p.RecordFault(PageKey{1, i}, AccessRead, FaultLocalPage)
	}
	if p.Stats().Hits != 64 {
		t.Fatal("not 64 entries")
	}
	p.RecordFault(PageKey{2, 1}, AccessWrite, FaultLocalPage)
	e, _ := p.Snapshot(PageKey{1, 1})
	if e.SawWrite {
		t.Fatal("PID alias")
	}
}
func TestRemoteThreshold(t *testing.T) {
	c := NewAccessCounters(256)
	k := PageKey{1, 17}
	for i := 1; i <= 300; i++ {
		if c.Count(k, 2) != (i == 256) {
			t.Fatal("AC crossing", i)
		}
	}
	if c.Count(PageKey{2, 17}, 2) {
		t.Fatal("PID alias")
	}
	c.Complete(k, 2)
	if c.Count(k, 2) {
		t.Fatal("not reset")
	}
}
