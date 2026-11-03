package grit

import "sync"

type Prediction struct {
	Scheme    Scheme
	GroupCode uint8
}
type PredictionStats struct {
	Promotions, Demotions [4]uint64
	SuppressedAC          uint64
}

// Predictor holds host-only future policies, independent of resident mappings.
// Groups require all eight allocated children; holes never vote for a policy.
type Predictor struct {
	mu    sync.Mutex
	pages map[PageKey]Prediction
	stats PredictionStats
}

func NewPredictor() *Predictor              { return &Predictor{pages: map[PageKey]Prediction{}} }
func GroupBaseVPN(vpn, pages uint64) uint64 { return vpn / pages * pages }
func groupSize(code uint8) uint64           { return uint64(1) << (3 * code) }
func (p *Predictor) Allocate(k PageKey) {
	p.mu.Lock()
	defer p.mu.Unlock()
	if _, ok := p.pages[k]; !ok {
		p.pages[k] = Prediction{Scheme: SchemeOnTouch}
	}
}
func (p *Predictor) Lookup(k PageKey) (Prediction, bool) {
	p.mu.Lock()
	defer p.mu.Unlock()
	m, ok := p.pages[k]
	return m, ok
}
func (p *Predictor) Stats() PredictionStats { p.mu.Lock(); defer p.mu.Unlock(); return p.stats }

// split preserves every unaffected sibling group while recursively splitting the
// branch containing the changed page. Only aligned base PTEs carry group codes.
func (p *Predictor) split(k PageKey) {
	for code := uint8(3); code > 0; code-- {
		size := groupSize(code)
		base := PageKey{k.PID, GroupBaseVPN(k.VPN, size)}
		m, ok := p.pages[base]
		if !ok || m.GroupCode != code {
			continue
		}
		p.stats.Demotions[code]++
		child := size / 8
		for i := uint64(0); i < 8; i++ {
			b := PageKey{k.PID, base.VPN + i*child}
			v := p.pages[b]
			v.GroupCode = code - 1
			p.pages[b] = v
		}
	}
}

// Change applies an actual page decision. Promotion is a separate background
// event so a fault can retire before neighboring policy metadata is inspected.
func (p *Predictor) Change(k PageKey, old, next Scheme) bool {
	p.mu.Lock()
	defer p.mu.Unlock()
	if !next.Valid() {
		panic("invalid prediction scheme")
	}
	if _, ok := p.pages[k]; !ok {
		return false
	}
	if old == next {
		// Suppress neighboring votes, but a new decision still supersedes a host
		// prediction. Split only if that prediction belongs to a different group.
		if p.pages[k].Scheme != next {
			p.split(k)
			m := p.pages[k]
			m.Scheme = next
			m.GroupCode = 0
			p.pages[k] = m
		}
		if next == SchemeAccessCounter {
			p.stats.SuppressedAC++
		}
		return false
	}
	p.split(k)
	m := p.pages[k]
	m.Scheme = next
	m.GroupCode = 0
	p.pages[k] = m
	return true
}
func (p *Predictor) Promote(k PageKey, next Scheme) {
	p.mu.Lock()
	defer p.mu.Unlock()
	m, ok := p.pages[k]
	if !ok || m.Scheme != next {
		return
	}
	start := uint8(1)
	for code := uint8(1); code <= 3; code++ {
		b := PageKey{k.PID, GroupBaseVPN(k.VPN, groupSize(code))}
		if v, exists := p.pages[b]; exists && v.GroupCode == code && v.Scheme == next {
			start = code + 1
		}
	}
	for code := start; code <= 3; code++ {
		size := groupSize(code)
		child := size / 8
		base := GroupBaseVPN(k.VPN, size)
		matches := 0
		for vpn := base; vpn < base+size; vpn++ {
			if _, ok := p.pages[PageKey{k.PID, vpn}]; !ok {
				return
			}
		}
		for i := uint64(0); i < 8; i++ {
			v := p.pages[PageKey{k.PID, base + i*child}]
			if v.GroupCode == code-1 && v.Scheme == next {
				matches++
			}
		}
		if matches < 5 {
			return
		}
		for vpn := base; vpn < base+size; vpn++ {
			p.pages[PageKey{k.PID, vpn}] = Prediction{Scheme: next}
		}
		p.pages[PageKey{k.PID, base}] = Prediction{Scheme: next, GroupCode: code}
		p.stats.Promotions[code]++
	}
}
func (p *Predictor) Remove(k PageKey) {
	p.mu.Lock()
	defer p.mu.Unlock()
	p.split(k)
	delete(p.pages, k)
}
