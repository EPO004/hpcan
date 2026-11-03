package grit

import "testing"

func TestDirectoryTransactions(t *testing.T) {
	d := NewDirectory()
	k := PageKey{1, 5}
	if err := d.OnAllocate(k, 1, 4096); err != nil {
		t.Fatal(err)
	}
	if _, ok := d.LookupMapping(PageKey{2, 5}, 1, AccessRead); ok {
		t.Fatal("PID alias")
	}
	s, ok := d.BeginOperation(k)
	if !ok {
		t.Fatal("begin")
	}
	if _, ok = d.BeginOperation(k); ok {
		t.Fatal("double operation")
	}
	if d.CheckAccess(k, 1, AccessWrite, 1) != WaitForPageTransition {
		t.Fatal("pending access")
	}
	s.Scheme = SchemeDuplication
	s.Version++
	s.Replicas = map[uint64]Replica{1: {GPUID: 1, PAddr: 4096, Valid: true, Generation: 2}, 2: {GPUID: 2, PAddr: 8192, Valid: true, Generation: 2}}
	if err := d.CommitOperation(k, s); err != nil {
		t.Fatal(err)
	}
	if d.CheckAccess(k, 2, AccessWrite, 2) != NeedWriteProtectionFault {
		t.Fatal("unprotected write")
	}
	if d.CheckAccess(k, 2, AccessRead, 1) != NeedLocalFault {
		t.Fatal("stale translation")
	}
	s, _ = d.Lookup(k)
	delete(s.Replicas, 1)
	actual, _ := d.Lookup(k)
	if len(actual.Replicas) != 2 {
		t.Fatal("snapshot aliases state")
	}
	s, _ = d.BeginOperation(k)
	s.Replicas[1] = Replica{GPUID: 1, PAddr: 4096, Valid: true, Writable: true, Generation: 2}
	if d.CommitOperation(k, s) == nil {
		t.Fatal("accepted shared writer")
	}
	d.AbortOperation(k)
	actual, _ = d.Lookup(k)
	if actual.Pending || actual.Replicas[1].Writable {
		t.Fatal("abort failed")
	}
}
