package grit

import (
	"fmt"
	"sync"
)

type Mode string

const (
	ModeOff  Mode = "off"
	ModeOT   Mode = "ot"
	ModeAC   Mode = "ac"
	ModeDUP  Mode = "dup"
	ModeGRIT Mode = "grit"
)

type Config struct {
	FootprintBytes                uint64
	CapacityRatio                 float64
	ResidentPages                 uint64
	PeerBandwidth, HostBandwidth  uint64
	Mode                          Mode
	FaultThreshold, ACThreshold   uint32
	PACache, Prediction           bool
	PACacheCycles, PAMemoryCycles uint64
}

func DefaultConfig() Config {
	return Config{Mode: ModeOff, CapacityRatio: .70, PeerBandwidth: 300000000000, HostBandwidth: 32000000000, FaultThreshold: 4, ACThreshold: 256, PACache: true, Prediction: true, PACacheCycles: 1, PAMemoryCycles: 100}
}
func (c Config) Validate() error {
	if c.Mode != ModeOff && !(c.CapacityRatio > 0 && c.CapacityRatio <= 1) {
		return fmt.Errorf("capacity ratio must be in (0,1]")
	}
	switch c.Mode {
	case ModeOff, ModeOT, ModeAC, ModeDUP, ModeGRIT:
	default:
		return fmt.Errorf("unknown GRIT mode %q", c.Mode)
	}
	if c.Mode != ModeOff && (c.PeerBandwidth == 0 || c.HostBandwidth == 0) {
		return fmt.Errorf("GRIT link bandwidths must be positive")
	}
	if c.FaultThreshold == 0 || c.ACThreshold == 0 {
		return fmt.Errorf("thresholds must be positive")
	}
	return nil
}

type faultKey struct {
	Key             PageKey
	GPU, Generation uint64
	Kind            FaultKind
}
type Fault struct {
	ID              string
	Key             PageKey
	GPU, Generation uint64
	Access          AccessKind
	Kind            FaultKind
}
type FaultResolution struct {
	PreviousScheme Scheme
	Fault          Fault
	Scheme         Scheme
	PACycles       uint64
	Triggered      bool
	Waiters        []string
}

// Controller admits and coalesces real faults. It does not perform data movement:
// Resolve and Complete bracket the driver's acknowledged placement operation.
// Conflicting faults to one page wait; the caller retries admission after Complete.
type Controller struct {
	mu        sync.Mutex
	Config    Config
	Directory *Directory
	PA        *PATable
	Predictor *Predictor
	Counters  *AccessCounters
	pending   map[PageKey]*FaultResolution
	keys      map[PageKey]faultKey
	metrics   Metrics
}

func NewController(c Config) (*Controller, error) {
	if err := c.Validate(); err != nil {
		return nil, err
	}
	return &Controller{Config: c, Directory: NewDirectory(), PA: NewPATable(c.PACache, c.FaultThreshold, c.PACacheCycles, c.PAMemoryCycles), Predictor: NewPredictor(), Counters: NewAccessCounters(c.ACThreshold), pending: map[PageKey]*FaultResolution{}, keys: map[PageKey]faultKey{}}, nil
}
func (c *Controller) Allocate(k PageKey, gpu, addr uint64) error {
	c.mu.Lock()
	defer c.mu.Unlock()
	if err := c.Directory.OnAllocate(k, gpu, addr); err != nil {
		return err
	}
	c.Predictor.Allocate(k)
	return nil
}

// Admit returns admitted=false for conflicting faults or cache/TLB misses with a
// valid GPU mapping. Coalesced faults return the original resolution and add a
// waiter without changing PA history. A write never coalesces behind a read.
func (c *Controller) Admit(f Fault) (FaultResolution, bool, error) {
	c.mu.Lock()
	defer c.mu.Unlock()
	if c.Config.Mode == ModeOff {
		return FaultResolution{}, false, fmt.Errorf("GRIT disabled")
	}
	if f.ID == "" || f.GPU == 0 || f.Access > AccessWrite || f.Kind > FaultWriteProtection {
		return FaultResolution{}, false, fmt.Errorf("invalid fault")
	}
	key := faultKey{f.Key, f.GPU, f.Generation, f.Kind}
	if existing, ok := c.pending[f.Key]; ok {
		if c.keys[f.Key] != key || existing.Fault.Access != f.Access {
			return FaultResolution{}, false, nil
		}
		for _, id := range existing.Waiters {
			if id == f.ID {
				return copyResolution(*existing), true, nil
			}
		}
		existing.Waiters = append(existing.Waiters, f.ID)
		return copyResolution(*existing), true, nil
	}
	state, ok := c.Directory.Lookup(f.Key)
	if !ok {
		return FaultResolution{}, false, fmt.Errorf("page unallocated")
	}
	// A stale TLB generation with a current, permitted GPU PTE needs a refill,
	// not a new local-page fault or PA history update.
	if c.Directory.CheckAccess(f.Key, f.GPU, f.Access, 0) == AccessAllowed {
		return FaultResolution{}, false, nil
	}
	decision := c.Directory.CheckAccess(f.Key, f.GPU, f.Access, f.Generation)
	if decision == AccessAllowed || decision == WaitForPageTransition {
		return FaultResolution{}, false, nil
	}
	if (decision == NeedWriteProtectionFault) != (f.Kind == FaultWriteProtection) {
		return FaultResolution{}, false, fmt.Errorf("fault kind disagrees with mapping")
	}
	if _, ok := c.Directory.BeginOperation(f.Key); !ok {
		return FaultResolution{}, false, nil
	}
	r := FaultResolution{PreviousScheme: state.Scheme, Fault: f, Scheme: state.Scheme, Waiters: []string{f.ID}}
	switch c.Config.Mode {
	case ModeOT:
		r.Scheme = SchemeOnTouch
	case ModeAC:
		r.Scheme = SchemeAccessCounter
	case ModeDUP:
		r.Scheme = SchemeDuplication
	case ModeGRIT:
		if c.Config.Prediction {
			if p, ok := c.Predictor.Lookup(f.Key); ok {
				r.Scheme = p.Scheme
			}
		}
		selected, trigger, cycles := c.PA.RecordFault(f.Key, f.Access, f.Kind)
		r.PACycles = cycles
		r.Triggered = trigger
		if trigger {
			r.Scheme = selected
			c.metrics.FaultTriggers++
		}
	}
	if f.Kind == FaultWriteProtection {
		c.metrics.ProtectionFaults++
	} else {
		c.metrics.LocalPageFaults++
	}
	c.pending[f.Key] = &r
	c.keys[f.Key] = key
	return copyResolution(r), true, nil
}
func copyResolution(r FaultResolution) FaultResolution {
	r.Waiters = append([]string(nil), r.Waiters...)
	return r
}

// Plan constructs the physical operation for an admitted fault, with the epoch
// reset published atomically alongside its effective placement.
func (c *Controller) Plan(k PageKey, target uint64) (PlacementPlan, error) {
	c.mu.Lock()
	defer c.mu.Unlock()
	r, ok := c.pending[k]
	if !ok {
		return PlacementPlan{}, fmt.Errorf("no admitted fault")
	}
	s, ok := c.Directory.Lookup(k)
	if !ok {
		return PlacementPlan{}, fmt.Errorf("page missing")
	}
	p, err := PlanPlacement(s, r.Fault.GPU, r.Fault.Access, r.Scheme, target)
	if err == nil && r.Triggered {
		p.After.Epoch++
	}
	return p, err
}

// Complete requires the driver to have committed exactly the effective decision.
// Prediction returns a background-work flag; Promote must run as a later event.
func (c *Controller) Complete(k PageKey) (FaultResolution, bool, error) {
	c.mu.Lock()
	defer c.mu.Unlock()
	r, ok := c.pending[k]
	if !ok {
		return FaultResolution{}, false, fmt.Errorf("no admitted fault")
	}
	state, ok := c.Directory.Lookup(k)
	if !ok || state.Pending || state.Scheme != r.Scheme {
		return FaultResolution{}, false, fmt.Errorf("placement not committed")
	}
	if _, valid := c.Directory.LookupMapping(k, r.Fault.GPU, r.Fault.Access); !valid {
		return FaultResolution{}, false, fmt.Errorf("requester mapping not resolved")
	}
	background := false
	if c.Config.Mode == ModeGRIT && r.Triggered {
		background = c.Predictor.Change(k, r.PreviousScheme, r.Scheme)
	}
	delete(c.pending, k)
	delete(c.keys, k)
	return copyResolution(*r), background && c.Config.Prediction, nil
}
func (c *Controller) Free(k PageKey) error {
	c.mu.Lock()
	defer c.mu.Unlock()
	if _, ok := c.pending[k]; ok {
		return fmt.Errorf("fault pending")
	}
	if err := c.Directory.OnFree(k); err != nil {
		return err
	}
	c.PA.Remove(k)
	c.Predictor.Remove(k)
	c.Counters.Remove(k)
	return nil
}
func (c *Controller) Metrics() Metrics { c.mu.Lock(); defer c.mu.Unlock(); return c.metrics }

func (c *Controller) AllocateBacking(k PageKey, addr uint64) error {
	c.mu.Lock()
	defer c.mu.Unlock()
	if err := c.Directory.OnAllocateBacking(k, addr); err != nil {
		return err
	}
	c.Predictor.Allocate(k)
	return nil
}
