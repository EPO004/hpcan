package runner

import (
	"fmt"
	"log"
	"os"

	memtraces "github.com/sarchlab/akita/v3/mem/trace"

	"github.com/sarchlab/akita/v3/analysis"
	"github.com/sarchlab/akita/v3/mem/mem"
	"github.com/sarchlab/akita/v3/mem/vm"
	"github.com/sarchlab/akita/v3/mem/vm/mmu"
	"github.com/sarchlab/akita/v3/monitoring"
	"github.com/sarchlab/akita/v3/noc/networking/nvlink"
	"github.com/sarchlab/akita/v3/noc/networking/pcie"
	"github.com/sarchlab/akita/v3/sim"
	"github.com/sarchlab/akita/v3/tracing"
	"github.com/sarchlab/mgpusim/v3/driver"
	"github.com/sarchlab/mgpusim/v3/grit"
	gritruntime "github.com/sarchlab/mgpusim/v3/grit/runtime"
)

// R9NanoPlatformBuilder can build a platform that equips R9Nano GPU.
type R9NanoPlatformBuilder struct {
	gritConfig                         grit.Config
	gritRuntime                        *gritruntime.Runtime
	useParallelEngine                  bool
	debugISA                           bool
	traceVis                           bool
	traceVisStartTime, traceVisEndTime sim.VTimeInSec
	traceMem                           bool
	dramSize                           uint64
	numGPU                             int
	numSAPerGPU                        int
	numCUPerSA                         int
	useMagicMemoryCopy                 bool
	log2PageSize                       uint64

	engine               sim.Engine
	monitor              *monitoring.Monitor
	perfAnalysisFileName string
	perfAnalyzingPeriod  float64
	perfAnalyzer         *analysis.PerfAnalyzer
	visTracer            tracing.Tracer

	globalStorage *mem.Storage

	gpus []*GPU
}

// MakeR9NanoBuilder creates a EmuBuilder with default parameters.
func MakeR9NanoBuilder() R9NanoPlatformBuilder {
	b := R9NanoPlatformBuilder{
		gritConfig:        grit.DefaultConfig(),
		dramSize:          32 * mem.MB,
		numGPU:            4,
		numSAPerGPU:       16,
		numCUPerSA:        4,
		log2PageSize:      12,
		traceVisStartTime: -1,
		traceVisEndTime:   -1,
	}
	return b
}

// WithParallelEngine lets the EmuBuilder to use parallel engine.
func (b R9NanoPlatformBuilder) WithParallelEngine() R9NanoPlatformBuilder {
	b.useParallelEngine = true
	return b
}

// WithISADebugging enables ISA debugging in the simulation.
func (b R9NanoPlatformBuilder) WithISADebugging() R9NanoPlatformBuilder {
	b.debugISA = true
	return b
}

// WithVisTracing lets the platform to record traces for visualization purposes.
func (b R9NanoPlatformBuilder) WithVisTracing() R9NanoPlatformBuilder {
	b.traceVis = true

	return b
}

// WithPartialVisTracing lets the platform to record traces for visualization
// purposes. The trace will only be collected from the start time to the end
// time.
func (b R9NanoPlatformBuilder) WithPartialVisTracing(
	start, end sim.VTimeInSec,
) R9NanoPlatformBuilder {
	b.traceVis = true
	b.traceVisStartTime = start
	b.traceVisEndTime = end

	return b
}

// WithMemTracing lets the platform to trace memory operations.
func (b R9NanoPlatformBuilder) WithMemTracing() R9NanoPlatformBuilder {
	b.traceMem = true
	return b
}

// WithNumGPU sets the number of GPUs to build.
func (b R9NanoPlatformBuilder) WithNumGPU(n int) R9NanoPlatformBuilder {
	b.numGPU = n
	return b
}

// WithLog2PageSize sets the page size as a power of 2.
func (b R9NanoPlatformBuilder) WithLog2PageSize(
	n uint64,
) R9NanoPlatformBuilder {
	b.log2PageSize = n
	return b
}

// WithMonitor sets the monitor that is used to monitor the simulation
func (b R9NanoPlatformBuilder) WithMonitor(
	m *monitoring.Monitor,
) R9NanoPlatformBuilder {
	b.monitor = m
	return b
}

// WithPerfAnalyzer sets the trace that dumps the WithPerfAnalyzer levers.
func (b R9NanoPlatformBuilder) WithPerfAnalyzer(
	traceDirName string,
	tracePeriod float64,
) R9NanoPlatformBuilder {
	b.perfAnalysisFileName = traceDirName
	b.perfAnalyzingPeriod = tracePeriod
	return b
}

// WithMagicMemoryCopy uses global storage as memory components
func (b R9NanoPlatformBuilder) WithMagicMemoryCopy() R9NanoPlatformBuilder {
	b.useMagicMemoryCopy = true
	return b
}

// Build builds a platform with R9Nano GPUs.
func (b R9NanoPlatformBuilder) Build() *Platform {
	validateDRAMSize(b.dramSize, b.numGPU, b.log2PageSize)
	b.engine = b.createEngine()
	if b.monitor != nil {
		b.monitor.RegisterEngine(b.engine)
	}

	b.setupPerformanceAnalyzer()
	b.setupVisTracing()

	b.globalStorage = mem.NewStorage(uint64(1+b.numGPU) * b.dramSize)

	mmuComponent, pageTable := b.createMMU(b.engine)

	observed := &gritruntime.ObservedPageTable{PageTable: pageTable}
	driverTable := pageTable
	if b.gritConfig.Mode != grit.ModeOff {
		driverTable = observed
	}
	gpuDriver := b.buildGPUDriver(driverTable)
	if b.gritConfig.Mode != grit.ModeOff {
		if b.log2PageSize != 12 || b.numGPU < 2 || b.useParallelEngine || b.useMagicMemoryCopy {
			panic("GRIT requires 4-KB pages, at least two GPUs, serial engine and timed copies")
		}
		b.gritRuntime = gritruntime.New(b.engine, b.gritConfig, pageTable, gpuDriver)
		b.gritRuntime.Wake = append(b.gritRuntime.Wake, gpuDriver.TickLater)
		if *gritTrace != "" {
			out, err := os.Create(*gritTrace)
			if err != nil {
				panic(err)
			}
			b.gritRuntime.SetTrace(out)
		}
		b.gritRuntime.Storage = b.globalStorage
		observed.Runtime = b.gritRuntime
		mmuComponent.ManagedMemory = b.gritRuntime
	}

	gpuBuilder := b.createGPUBuilder(b.engine, gpuDriver, mmuComponent)
	pcieConnector, rootComplexID :=
		b.createConnection(b.engine, gpuDriver, mmuComponent)

	mmuComponent.MigrationServiceProvider = gpuDriver.GetPortByName("MMU")

	rdmaAddressTable := b.createRDMAAddrTable()
	pmcAddressTable := b.createPMCPageTable()

	b.createGPUs(
		rootComplexID, pcieConnector,
		gpuBuilder, gpuDriver,
		rdmaAddressTable, pmcAddressTable)

	pcieConnector.EstablishRoute()

	return &Platform{
		GRIT:   b.gritRuntime,
		Engine: b.engine,
		Driver: gpuDriver,
		GPUs:   b.gpus,
	}
}

func (b R9NanoPlatformBuilder) buildGPUDriver(
	pageTable vm.PageTable,
) *driver.Driver {
	gpuDriverBuilder := driver.MakeBuilder()
	if b.useMagicMemoryCopy {
		gpuDriverBuilder = gpuDriverBuilder.WithMagicMemoryCopyMiddleware()
	}
	gpuDriver := gpuDriverBuilder.
		WithEngine(b.engine).
		WithPageTable(pageTable).
		WithLog2PageSize(b.log2PageSize).
		WithCPUMemorySize(b.dramSize).
		WithGlobalStorage(b.globalStorage).
		WithD2HCycles(8500).
		WithH2DCycles(14500).
		Build("Driver")
	if b.visTracer != nil {
		tracing.CollectTrace(gpuDriver, b.visTracer)
	}

	if b.monitor != nil {
		b.monitor.RegisterComponent(gpuDriver)
	}

	if b.perfAnalyzer != nil {
		b.perfAnalyzer.RegisterComponent(gpuDriver)
	}

	// file, err := os.Create("driver_comm.csv")
	// if err != nil {
	// 	panic(err)
	// }
	// gpuDriver.GetPortByName("GPU").AcceptHook(
	// 	sim.NewPortMsgLogger(log.New(file, "", 0)))

	return gpuDriver
}

func (b *R9NanoPlatformBuilder) setupVisTracing() {
	if !b.traceVis {
		return
	}

	var backend tracing.TracerBackend
	switch *visTracerDB {
	case "sqlite":
		be := tracing.NewSQLiteTraceWriter(*visTracerDBFileName)
		be.Init()
		backend = be
	case "csv":
		be := tracing.NewCSVTraceWriter(*visTracerDBFileName)
		be.Init()
		backend = be
	case "mysql":
		be := tracing.NewMySQLTraceWriter()
		be.Init()
		backend = be
	default:
		panic(fmt.Sprintf(
			"Tracer database type must be [sqlite|csv|mysql]. "+
				"Provided value %s is not supported.",
			*visTracerDB))
	}

	visTracer := tracing.NewDBTracer(b.engine, backend)
	visTracer.SetTimeRange(b.traceVisStartTime, b.traceVisEndTime)

	b.visTracer = visTracer
}

func (b *R9NanoPlatformBuilder) setupPerformanceAnalyzer() {
	if b.perfAnalysisFileName != "" {
		b.perfAnalyzer = analysis.MakePerfAnalyzerBuilder().
			WithPeriod(sim.VTimeInSec(b.perfAnalyzingPeriod)).
			WithDBFilename(b.perfAnalysisFileName).
			WithEngine(b.engine).
			Build()
	}
}

func (b *R9NanoPlatformBuilder) createGPUs(
	rootComplexID int,
	pcieConnector platformConnector,
	gpuBuilder R9NanoGPUBuilder,
	gpuDriver *driver.Driver,
	rdmaAddressTable *mem.BankedLowModuleFinder,
	pmcAddressTable *mem.BankedLowModuleFinder,
) {
	lastSwitchID := rootComplexID
	for i := 1; i < b.numGPU+1; i++ {
		if i%2 == 1 {
			lastSwitchID = pcieConnector.AddSwitch(rootComplexID)
		}

		b.createGPU(i, gpuBuilder, gpuDriver,
			rdmaAddressTable, pmcAddressTable,
			pcieConnector, lastSwitchID)
	}
}

func (b R9NanoPlatformBuilder) createPMCPageTable() *mem.BankedLowModuleFinder {
	pmcAddressTable := new(mem.BankedLowModuleFinder)
	pmcAddressTable.BankSize = b.dramSize
	pmcAddressTable.LowModules = append(pmcAddressTable.LowModules, nil)
	return pmcAddressTable
}

func (b R9NanoPlatformBuilder) createRDMAAddrTable() *mem.BankedLowModuleFinder {
	rdmaAddressTable := new(mem.BankedLowModuleFinder)
	rdmaAddressTable.BankSize = b.dramSize
	rdmaAddressTable.LowModules = append(rdmaAddressTable.LowModules, nil)
	return rdmaAddressTable
}

func (b R9NanoPlatformBuilder) createConnection(
	engine sim.Engine,
	gpuDriver *driver.Driver,
	mmuComponent *mmu.MMU,
) (platformConnector, int) {
	if b.gritConfig.Mode != grit.ModeOff {
		c := nvlink.NewConnector().WithEngine(engine).WithPCIeBandwidth(b.gritConfig.HostBandwidth).WithNVLinkBandwidth(b.gritConfig.PeerBandwidth / uint64(b.numGPU-1))
		c.CreateNetwork("GRITFabric")
		root := c.AddRootComplex([]sim.Port{gpuDriver.GetPortByName("GPU"), gpuDriver.GetPortByName("MMU"), mmuComponent.GetPortByName("Migration"), mmuComponent.GetPortByName("Top")})
		return &gritFabric{connector: c}, root
	}
	//connection := sim.NewDirectConnection(engine)
	// connection := noc.NewFixedBandwidthConnection(32, engine, 1*sim.GHz)
	// connection.SrcBufferCapacity = 40960000
	pcieConnector := pcie.NewConnector().
		WithEngine(engine).
		WithVersion(4, 16).
		WithSwitchLatency(140)

	if b.visTracer != nil {
		pcieConnector = pcieConnector.WithVisTracer(b.visTracer)
	}

	pcieConnector.CreateNetwork("PCIe")
	rootComplexID := pcieConnector.AddRootComplex(
		[]sim.Port{
			gpuDriver.GetPortByName("GPU"),
			gpuDriver.GetPortByName("MMU"),
			mmuComponent.GetPortByName("Migration"),
			mmuComponent.GetPortByName("Top"),
		})
	return pcieConnector, rootComplexID
}

func (b R9NanoPlatformBuilder) createEngine() sim.Engine {
	var engine sim.Engine

	if b.useParallelEngine {
		engine = sim.NewParallelEngine()
	} else {
		engine = sim.NewSerialEngine()
	}
	// engine.AcceptHook(sim.NewEventLogger(log.New(os.Stdout, "", 0)))

	return engine
}

func (b R9NanoPlatformBuilder) createMMU(
	engine sim.Engine,
) (*mmu.MMU, vm.PageTable) {
	pageTable := vm.NewPageTable(b.log2PageSize)
	mmuBuilder := mmu.MakeBuilder().
		WithEngine(engine).
		WithFreq(1 * sim.GHz).
		WithPageWalkingLatency(100).
		WithLog2PageSize(b.log2PageSize).
		WithPageTable(pageTable)

	if b.gritConfig.Mode != grit.ModeOff {
		mmuBuilder = mmuBuilder.WithPageWalkModel(8, 4, 128, 64).WithMaxNumReqInFlight(64 * b.numGPU)
	}
	mmuComponent := mmuBuilder.Build("MMU")

	if b.monitor != nil {
		b.monitor.RegisterComponent(mmuComponent)
	}

	return mmuComponent, pageTable
}

func (b *R9NanoPlatformBuilder) createGPUBuilder(
	engine sim.Engine,
	gpuDriver *driver.Driver,
	mmuComponent *mmu.MMU,
) R9NanoGPUBuilder {
	gpuBuilder := MakeR9NanoGPUBuilder().
		WithEngine(engine).
		WithMMU(mmuComponent).
		WithNumCUPerShaderArray(b.numCUPerSA).
		WithNumShaderArray(b.numSAPerGPU).
		WithDRAMSize(b.dramSize).
		WithNumMemoryBank(16).
		WithLog2MemoryBankInterleavingSize(7).
		WithLog2PageSize(b.log2PageSize).
		WithGlobalStorage(b.globalStorage)

	if b.monitor != nil {
		gpuBuilder = gpuBuilder.WithMonitor(b.monitor)
	}

	if b.perfAnalyzer != nil {
		gpuBuilder = gpuBuilder.WithPerfAnalyzer(b.perfAnalyzer)
	}

	if b.visTracer != nil {
		gpuBuilder = gpuBuilder.WithVisTracer(b.visTracer)
	}

	gpuBuilder.gritRuntime = b.gritRuntime
	if b.gritRuntime != nil {
		gpuBuilder = gpuBuilder.WithL2CacheSize(256 * mem.KB)
	}
	gpuBuilder = b.setMemTracer(gpuBuilder)
	gpuBuilder = b.setISADebugger(gpuBuilder)

	return gpuBuilder
}

func (b *R9NanoPlatformBuilder) setISADebugger(
	gpuBuilder R9NanoGPUBuilder,
) R9NanoGPUBuilder {
	if !b.debugISA {
		return gpuBuilder
	}

	gpuBuilder = gpuBuilder.WithISADebugging()
	return gpuBuilder
}

func (b *R9NanoPlatformBuilder) setMemTracer(
	gpuBuilder R9NanoGPUBuilder,
) R9NanoGPUBuilder {
	if !b.traceMem {
		return gpuBuilder
	}

	file, err := os.Create("mem.trace")
	if err != nil {
		panic(err)
	}
	logger := log.New(file, "", 0)
	memTracer := memtraces.NewTracer(logger, b.engine)
	gpuBuilder = gpuBuilder.WithMemTracer(memTracer)
	return gpuBuilder
}

func (b *R9NanoPlatformBuilder) createGPU(
	index int,
	gpuBuilder R9NanoGPUBuilder,
	gpuDriver *driver.Driver,
	rdmaAddressTable *mem.BankedLowModuleFinder,
	pmcAddressTable *mem.BankedLowModuleFinder,
	pcieConnector platformConnector,
	pcieSwitchID int,
) *GPU {
	name := fmt.Sprintf("GPU[%d]", index)
	memAddrOffset := uint64(index) * b.dramSize
	gpu := gpuBuilder.
		WithMemAddrOffset(memAddrOffset).
		Build(name, uint64(index))
	gpuDriver.RegisterGPU(
		gpu.Domain.GetPortByName("CommandProcessor"),
		driver.DeviceProperties{
			CUCount:  b.numCUPerSA * b.numSAPerGPU,
			DRAMSize: b.dramSize,
		},
	)
	gpu.CommandProcessor.Driver = gpuDriver.GetPortByName("GPU")

	b.configRDMAEngine(gpu, rdmaAddressTable)
	b.configPMC(gpu, gpuDriver, pmcAddressTable)

	pcieConnector.PlugInDevice(pcieSwitchID, gpu.Domain.Ports())

	b.gpus = append(b.gpus, gpu)

	return gpu
}

func (b *R9NanoPlatformBuilder) configRDMAEngine(
	gpu *GPU,
	addrTable *mem.BankedLowModuleFinder,
) {
	gpu.RDMAEngine.RemoteRDMAAddressTable = addrTable
	addrTable.LowModules = append(
		addrTable.LowModules,
		gpu.RDMAEngine.ToOutside)
}

func (b *R9NanoPlatformBuilder) configPMC(
	gpu *GPU,
	gpuDriver *driver.Driver,
	addrTable *mem.BankedLowModuleFinder,
) {
	gpu.PMC.RemotePMCAddressTable = addrTable
	addrTable.LowModules = append(
		addrTable.LowModules,
		gpu.PMC.GetPortByName("Remote"))
	gpuDriver.RemotePMCPorts = append(
		gpuDriver.RemotePMCPorts, gpu.PMC.GetPortByName("Remote"))
}

// WithDRAMSize sets per-GPU capacity in bytes and the matching CPU reservation.
func (b R9NanoPlatformBuilder) WithDRAMSize(size uint64) R9NanoPlatformBuilder {
	b.dramSize = size
	return b
}
