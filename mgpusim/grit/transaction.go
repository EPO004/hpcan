package grit

import (
	"fmt"
	"sort"
)

type OperationPhase uint8

const (
	PhaseQuiesce OperationPhase = iota
	PhaseCopy
	PhaseInvalidate
	PhaseCommit
	PhaseRetire
	PhaseRestart
	PhaseComplete
)

// Operation is an ACK-driven placement transaction. The caller owns transfer
// timing, backpressure and frame reservations. Each phase is issued once, with
// distinct ACK IDs; unrelated or duplicate ACKs cannot retire an operation.
// Commit publishes only after quiesce, copy and invalidation complete. Old frames
// are reclaimed afterward, before restarted requesters may access the new image.
type Operation struct {
	ID      string
	Plan    PlacementPlan
	Phase   OperationPhase
	issued  bool
	waiting map[string]bool
	used    map[string]bool
}

func NewOperation(id string, p PlacementPlan) *Operation {
	return &Operation{ID: id, Plan: p, waiting: map[string]bool{}, used: map[string]bool{}}
}
func (o *Operation) Issue(acks []string) error {
	if o.issued || o.Phase >= PhaseComplete {
		return fmt.Errorf("phase already issued or completed")
	}
	if o.Phase == PhaseCommit {
		return fmt.Errorf("use Commit for directory publication")
	}
	seen := map[string]bool{}
	for _, id := range acks {
		if id == "" || seen[id] || o.used[id] {
			return fmt.Errorf("ACK IDs must be nonempty and unique")
		}
		seen[id] = true
	}
	for id := range seen {
		o.used[id] = true
	}
	o.waiting = seen
	o.issued = true
	return nil
}
func (o *Operation) Ack(operationID, ackID string) bool {
	if operationID != o.ID || !o.issued || !o.waiting[ackID] {
		return false
	}
	delete(o.waiting, ackID)
	return true
}
func (o *Operation) Advance() bool {
	if !o.issued || len(o.waiting) != 0 {
		return false
	}
	o.issued = false
	o.Phase++
	if o.Phase == PhaseCopy && !o.Plan.Copy {
		o.Phase = PhaseInvalidate
	}
	return true
}
func (o *Operation) Commit(d *Directory) error {
	if o.Phase != PhaseCommit || o.issued {
		return fmt.Errorf("commit before ACK barrier")
	}
	if err := d.CommitOperation(o.Plan.Key, o.Plan.After); err != nil {
		return err
	}
	o.Phase = PhaseRetire
	return nil
}
func (o *Operation) PendingACKs() []string {
	ids := make([]string, 0, len(o.waiting))
	for id := range o.waiting {
		ids = append(ids, id)
	}
	sort.Strings(ids)
	return ids
}
