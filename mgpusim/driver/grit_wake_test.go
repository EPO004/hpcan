package driver

import (
	"github.com/golang/mock/gomock"
	"github.com/sarchlab/akita/v3/sim"
	"testing"
)

func TestEngineConsumesWakeAtRunBoundary(t *testing.T) {
	ctrl := gomock.NewController(t)
	defer ctrl.Finish()
	engine := NewMockEngine(ctrl)
	d := &Driver{engineRunning: true}
	d.TickingComponent = sim.NewTickingComponent("WakeRegression", engine, sim.GHz, d)
	calls := 0
	engine.EXPECT().Run().DoAndReturn(func() error {
		calls++
		if calls == 1 {
			d.engineRunningMutex.Lock()
			d.engineWakePending = true
			d.engineRunningMutex.Unlock()
		}
		return nil
	}).Times(2)
	d.runEngine()
	if calls != 2 || d.engineRunning {
		t.Fatal("wake stranded at engine completion")
	}
}
