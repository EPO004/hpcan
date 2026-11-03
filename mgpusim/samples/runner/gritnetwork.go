package runner

import (
	"github.com/sarchlab/akita/v3/noc/networking/nvlink"
	"github.com/sarchlab/akita/v3/sim"
)

// A common interface preserves the legacy PCIe platform with GRIT disabled.
type platformConnector interface {
	AddSwitch(int) int
	PlugInDevice(int, []sim.Port)
	EstablishRoute()
}
type gritFabric struct {
	connector *nvlink.Connector
	devices   []int
}

func (f *gritFabric) AddSwitch(root int) int {
	id := f.connector.AddPCIeSwitch()
	f.connector.ConnectSwitchesWithPCIeLink(root, id)
	return id
}
func (f *gritFabric) PlugInDevice(base int, ports []sim.Port) {
	f.devices = append(f.devices, f.connector.PlugInDevice(base, ports))
}
func (f *gritFabric) EstablishRoute() {
	for i := range f.devices {
		for j := i + 1; j < len(f.devices); j++ {
			f.connector.ConnectDevicesWithNVLink(f.devices[i], f.devices[j], 1)
		}
	}
	f.connector.EstablishRoute()
}
