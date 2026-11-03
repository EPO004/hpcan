package grit

import (
	"fmt"
	"sort"
)

// PlacementPlan describes work that must finish before directory commit. Physical
// frames are reserved by the caller, without updating the host PTE. Quiesce must
// drain and flush holders before copy/invalidation; Retire runs only after commit.
type PlacementPlan struct {
	Key                              PageKey
	Before, After                    PageState
	Copy                             bool
	Source, Destination              uint64
	Invalidate                       []uint64
	Retire                           []Replica
	Migration, Replication, Collapse bool
}

// PlanPlacement operates on an already locked directory snapshot. It never
// mutates canonical state or releases physical frames. The driver must abort on
// failure, or commit After only after all required copy/invalidation ACKs.
func PlanPlacement(s PageState, gpu uint64, access AccessKind, scheme Scheme, target uint64) (PlacementPlan, error) {
	if !s.Pending {
		return PlacementPlan{}, fmt.Errorf("placement requires page lock")
	}
	if !scheme.Valid() || gpu == 0 {
		return PlacementPlan{}, fmt.Errorf("invalid placement target")
	}
	if err := Validate(s); err != nil {
		return PlacementPlan{}, err
	}
	p := PlacementPlan{Key: s.Key, Before: clone(s), After: clone(s)}
	next := &p.After
	next.Scheme = scheme
	next.Version++
	next.Pending = false
	for id, r := range s.Replicas {
		if r.Valid {
			p.Invalidate = append(p.Invalidate, id)
		}
	}
	// DUP-to-other and all writes collapse physical copies to the authoritative
	// image before choosing new mappings. AC remote aliases share this image.
	if scheme == SchemeDuplication && access == AccessRead {
		next.Replicas = map[uint64]Replica{}
		for id, r := range s.Replicas {
			if r.Valid {
				r.Writable = false
				r.Dirty = false
				r.Generation = next.Version
				next.Replicas[id] = r
			}
		}
		if _, ok := next.Replicas[gpu]; !ok || s.Scheme != SchemeDuplication && gpu != s.HomeGPU {
			if target == s.OwnerPAddr {
				return PlacementPlan{}, fmt.Errorf("replica frame aliases owner")
			}
			next.Replicas[gpu] = Replica{GPUID: gpu, PAddr: target, Valid: true, Generation: next.Version}
			p.Copy = true
			p.Source = s.OwnerPAddr
			p.Destination = target
			p.Replication = true
		}
		// AC maps remote aliases, not independent replicas. Discard aliases while
		// keeping the owner and the newly materialized requesting replica.
		if s.Scheme == SchemeAccessCounter {
			for id, r := range next.Replicas {
				if id != s.HomeGPU && id != gpu && r.PAddr == s.OwnerPAddr {
					delete(next.Replicas, id)
				}
			}
		}
	} else {
		owner, addr := s.HomeGPU, s.OwnerPAddr
		if scheme != SchemeAccessCounter && gpu != owner {
			owner = gpu
			addr = target
			p.Copy = true
			p.Source = s.OwnerPAddr
			p.Destination = target
			p.Migration = true
		}
		if s.Scheme == SchemeDuplication && access == AccessWrite {
			images := map[uint64]bool{}
			for _, replica := range s.Replicas {
				if replica.Valid {
					images[replica.PAddr] = true
				}
			}
			p.Collapse = len(images) > 1
		}
		next.HomeGPU = owner
		next.OwnerPAddr = addr
		next.Replicas = map[uint64]Replica{owner: {GPUID: owner, PAddr: addr, Valid: true, Writable: true, Generation: next.Version}}
		if scheme == SchemeAccessCounter && gpu != owner {
			// Remote aliases address one authoritative image. Coherence is enforced by
			// the access coordinator, without counting AC writes as DUP protection faults.
			next.Replicas[gpu] = Replica{GPUID: gpu, PAddr: addr, Valid: true, Writable: true, Generation: next.Version}
		}
	}
	// Retire distinct physical images, never remote aliases or surviving frames.
	live := map[uint64]bool{}
	for _, r := range next.Replicas {
		live[r.PAddr] = true
	}
	seen := map[uint64]bool{}
	for _, r := range s.Replicas {
		if !live[r.PAddr] && !seen[r.PAddr] {
			p.Retire = append(p.Retire, r)
			seen[r.PAddr] = true
		}
	}
	sort.Slice(p.Invalidate, func(i, j int) bool { return p.Invalidate[i] < p.Invalidate[j] })
	sort.Slice(p.Retire, func(i, j int) bool { return p.Retire[i].PAddr < p.Retire[j].PAddr })
	if err := Validate(*next); err != nil {
		return PlacementPlan{}, err
	}
	return p, nil
}

// PlanCounterMigration moves the hot 4-KB page after a 64-KB AC counter crosses
// its threshold. Hot-page selection is an explicit engineering assumption. It
// retains AC and bypasses PA fault admission, epoch counting and prediction.
func PlanCounterMigration(s PageState, gpu, target uint64) (PlacementPlan, error) {
	if s.Scheme != SchemeAccessCounter {
		return PlacementPlan{}, fmt.Errorf("counter migration requires AC")
	}
	p, err := PlanPlacement(s, gpu, AccessWrite, SchemeOnTouch, target)
	if err != nil {
		return PlacementPlan{}, err
	}
	p.After.Scheme = SchemeAccessCounter
	return p, Validate(p.After)
}
