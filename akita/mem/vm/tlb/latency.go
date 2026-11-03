package tlb

import "github.com/sarchlab/akita/v3/sim"

type delayedRequest struct {
	msg   sim.Msg
	ready sim.VTimeInSec
}

// WithLookupLatency sets a pipelined lookup delay, preserving input throughput.
// Zero retains the original behavior for legacy builds.
func (b Builder) WithLookupLatency(cycles int) Builder {
	if cycles < 0 {
		panic("negative TLB lookup latency")
	}
	b.lookupLatency = cycles
	return b
}
func (t *TLB) advanceInput(now sim.VTimeInSec) bool {
	if t.lookupLatency == 0 {
		return false
	}
	progress := len(t.delayed) > 0
	for i := 0; i < t.numReqPerCycle && len(t.delayed) < 256; i++ {
		msg := t.topPort.Retrieve(now)
		if msg == nil {
			break
		}
		t.delayed = append(t.delayed, delayedRequest{msg: msg, ready: now + sim.VTimeInSec(t.lookupLatency)/sim.VTimeInSec(t.Freq)})
		progress = true
	}
	return progress
}
func (t *TLB) peekTop(now sim.VTimeInSec) sim.Msg {
	if t.lookupLatency == 0 {
		return t.topPort.Peek()
	}
	if len(t.delayed) == 0 || t.delayed[0].ready > now {
		return nil
	}
	return t.delayed[0].msg
}
func (t *TLB) retrieveTop(now sim.VTimeInSec) sim.Msg {
	if t.lookupLatency == 0 {
		return t.topPort.Retrieve(now)
	}
	if len(t.delayed) == 0 {
		return nil
	}
	msg := t.delayed[0].msg
	t.delayed = t.delayed[1:]
	return msg
}
