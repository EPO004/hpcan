package grit

// Metrics count admitted faults and completed operations, never enqueue attempts.
// Timing/network builders must provide actual traffic and latency measurements.
type Metrics struct {
	LocalPageFaults, ProtectionFaults, FaultTriggers  uint64
	Migrations, Copies, Collapses                     uint64
	BytesMigrated, BytesDuplicated                    uint64
	RemoteAccesses, RemoteReadBytes, RemoteWriteBytes uint64
	Transitions                                       [4][4]uint64
}

func (m *Metrics) RecordCompleted(p PlacementPlan, pageBytes uint64) {
	if p.Migration {
		m.Migrations++
		m.BytesMigrated += pageBytes
	}
	if p.Replication {
		m.Copies++
		m.BytesDuplicated += pageBytes
	}
	if p.Collapse {
		m.Collapses++
	}
	m.Transitions[p.Before.Scheme][p.After.Scheme]++
}
