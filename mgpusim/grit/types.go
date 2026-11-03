// Package grit implements the policy and residency machinery for GRIT (HPCA 2024).
// State mutations must be invoked at simulator events; locks protect snapshots,
// and do not substitute for modeled memory operations or acknowledgement ordering.
package grit

type Scheme uint8

const (
	SchemeInvalid Scheme = iota
	SchemeOnTouch
	SchemeAccessCounter
	SchemeDuplication
)

func (s Scheme) Valid() bool { return s >= SchemeOnTouch && s <= SchemeDuplication }

type AccessKind uint8

const (
	AccessRead AccessKind = iota
	AccessWrite
)

type FaultKind uint8

const (
	FaultLocalPage FaultKind = iota
	FaultWriteProtection
)

type PageKey struct {
	PID uint32
	VPN uint64
}
type Replica struct {
	GPUID, PAddr           uint64
	Valid, Writable, Dirty bool
	Generation             uint64
}
type PageState struct {
	Key                 PageKey
	Scheme              Scheme
	HomeGPU, OwnerPAddr uint64
	Replicas            map[uint64]Replica
	Version, Epoch      uint64
	Pending             bool
	GroupCode           uint8
}

func clone(s PageState) PageState {
	r := make(map[uint64]Replica, len(s.Replicas))
	for k, v := range s.Replicas {
		r[k] = v
	}
	s.Replicas = r
	return s
}
