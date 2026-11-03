package addresstranslator

import (
	"github.com/golang/mock/gomock"
	"github.com/sarchlab/akita/v3/mem/mem"
	"github.com/sarchlab/akita/v3/mem/vm"
	"github.com/sarchlab/akita/v3/sim"
	"testing"
)

type deniedManaged struct{ called bool }

func (m *deniedManaged) Before(_ sim.VTimeInSec, _ string, _ vm.PID, _, _, _ uint64, _ bool) bool {
	m.called = true
	return false
}
func (*deniedManaged) After(sim.VTimeInSec, string)                  {}
func (*deniedManaged) Lookup(vm.PID, uint64, uint64) (vm.Page, bool) { return vm.Page{}, false }
func TestManagedGuardStallsBeforeTranslationAndCache(t *testing.T) {
	ctrl := gomock.NewController(t)
	defer ctrl.Finish()
	port := NewMockPort(ctrl)
	request := mem.WriteReqBuilder{}.WithPID(1).WithAddress(4096).WithData([]byte{7}).Build()
	port.EXPECT().Peek().Return(request)
	guard := &deniedManaged{}
	at := &AddressTranslator{topPort: port, ManagedMemory: guard}
	// No translation/bottom ports are configured: touching either would panic.
	if at.translate(1) || !guard.called {
		t.Fatal("managed write bypassed admission guard")
	}
}
