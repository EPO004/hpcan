package grit

import "testing"

func TestPublicationWaitsForCopyAndInvalidation(t *testing.T) {
	d := NewDirectory()
	k := PageKey{1, 2}
	d.OnAllocate(k, 1, 4096)
	s, _ := d.BeginOperation(k)
	p, err := PlanPlacement(s, 2, AccessRead, SchemeOnTouch, 8192)
	if err != nil {
		t.Fatal(err)
	}
	o := NewOperation("op1", p)
	if o.Commit(d) == nil {
		t.Fatal("early publication")
	}
	o.Issue([]string{"drain1", "drain2"})
	if o.Ack("other", "drain1") {
		t.Fatal("wrong operation ACK")
	}
	o.Ack("op1", "drain1")
	if o.Advance() {
		t.Fatal("did not wait for all drains")
	}
	if o.Ack("op1", "drain1") {
		t.Fatal("duplicate ACK")
	}
	o.Ack("op1", "drain2")
	o.Advance()
	if o.Phase != PhaseCopy {
		t.Fatal("skipped copy")
	}
	if err := o.Issue([]string{"drain1"}); err == nil {
		t.Fatal("reused ACK identity across phases")
	}
	o.Issue([]string{"copy"})
	if o.Advance() {
		t.Fatal("unacknowledged copy")
	}
	current, _ := d.Lookup(k)
	if current.OwnerPAddr != 4096 || !current.Pending {
		t.Fatal("destination published early")
	}
	o.Ack("op1", "copy")
	o.Advance()
	o.Issue([]string{"invalidate1"})
	o.Ack("op1", "invalidate1")
	o.Advance()
	if err := o.Commit(d); err != nil {
		t.Fatal(err)
	}
	current, _ = d.Lookup(k)
	if current.OwnerPAddr != 8192 || current.Pending {
		t.Fatal("commit missing")
	}
	for o.Phase != PhaseComplete {
		if err := o.Issue(nil); err != nil {
			t.Fatal(err)
		}
		if !o.Advance() {
			t.Fatal("cannot finish")
		}
	}
}
