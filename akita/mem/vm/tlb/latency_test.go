package tlb

import (
	"github.com/sarchlab/akita/v3/mem/vm"
	"testing"
)

func TestLookupPipelineHonorsReadyTimeAndOrder(t *testing.T) {
	a := &vm.TranslationReq{}
	b := &vm.TranslationReq{}
	t1 := &TLB{lookupLatency: 10, delayed: []delayedRequest{{msg: a, ready: 10}, {msg: b, ready: 11}}}
	if t1.peekTop(9) != nil {
		t.Fatal("lookup completed early")
	}
	if t1.peekTop(10) != a || t1.retrieveTop(10) != a {
		t.Fatal("lost first request")
	}
	if t1.peekTop(10) != nil || t1.peekTop(11) != b {
		t.Fatal("pipeline violated timing/order")
	}
}
