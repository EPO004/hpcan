package grit

import "testing"

func TestStrictMajorityAndHole(t *testing.T) {
	p := NewPredictor()
	for i := uint64(0); i < 8; i++ {
		p.Allocate(PageKey{1, i})
	}
	for i := uint64(0); i < 4; i++ {
		p.Change(PageKey{1, i}, SchemeOnTouch, SchemeDuplication)
		p.Promote(PageKey{1, i}, SchemeDuplication)
	}
	m, _ := p.Lookup(PageKey{1, 0})
	if m.GroupCode != 0 {
		t.Fatal("four is not a majority")
	}
	p.Change(PageKey{1, 4}, SchemeOnTouch, SchemeDuplication)
	p.Promote(PageKey{1, 4}, SchemeDuplication)
	m, _ = p.Lookup(PageKey{1, 0})
	if m.GroupCode != 1 {
		t.Fatal("five must promote")
	}
	p.Remove(PageKey{1, 7})
	p.Change(PageKey{1, 0}, SchemeDuplication, SchemeAccessCounter)
	p.Promote(PageKey{1, 0}, SchemeAccessCounter)
	m, _ = p.Lookup(PageKey{1, 0})
	if m.GroupCode != 0 {
		t.Fatal("promotion across hole")
	}
}
func TestRecursivePromotionAndSplit(t *testing.T) {
	p := NewPredictor()
	for i := uint64(0); i < 512; i++ {
		p.Allocate(PageKey{1, i})
	}
	// Build all children, including the minority groups required by the
	// conservative all-eight-child-groups promotion rule.
	for g := uint64(0); g < 8; g++ {
		for h := uint64(0); h < 8; h++ {
			scheme := SchemeDuplication
			if g < 5 && h < 5 {
				scheme = SchemeAccessCounter
			}
			for i := uint64(0); i < 8; i++ {
				p.pages[PageKey{1, g*64 + h*8 + i}] = Prediction{Scheme: scheme}
			}
			m := p.pages[PageKey{1, g*64 + h*8}]
			m.GroupCode = 1
			p.pages[PageKey{1, g*64 + h*8}] = m
		}
	}
	for g := uint64(5); g < 8; g++ {
		p.Promote(PageKey{1, g * 64}, SchemeDuplication)
	}
	for g := uint64(0); g < 5; g++ {
		p.Promote(PageKey{1, g * 64}, SchemeAccessCounter)
	}
	m, _ := p.Lookup(PageKey{1, 0})
	if m.GroupCode != 3 {
		t.Fatal("512 promotion failed", m)
	}
	p.Change(PageKey{1, 123}, SchemeAccessCounter, SchemeDuplication)
	for _, c := range []struct {
		vpn  uint64
		code uint8
	}{{0, 2}, {64, 1}, {112, 1}, {128, 2}, {120, 0}, {124, 0}} {
		m, _ = p.Lookup(PageKey{1, c.vpn})
		if m.GroupCode != c.code {
			t.Fatalf("vpn %d code %d expected %d", c.vpn, m.GroupCode, c.code)
		}
	}
	if p.Change(PageKey{1, 128}, SchemeAccessCounter, SchemeAccessCounter) {
		t.Fatal("unchanged decision checks neighbors")
	}
	if p.Stats().SuppressedAC != 1 {
		t.Fatal("suppression metric")
	}
}

func TestFiveGroupedChildrenPromoteWithoutMinorityGrouping(t *testing.T) {
	p := NewPredictor()
	for i := uint64(0); i < 64; i++ {
		p.Allocate(PageKey{1, i})
	}
	for group := uint64(0); group < 5; group++ {
		for i := uint64(0); i < 5; i++ {
			k := PageKey{1, group*8 + i}
			p.Change(k, SchemeOnTouch, SchemeAccessCounter)
			p.Promote(k, SchemeAccessCounter)
		}
	}
	m, _ := p.Lookup(PageKey{1, 0})
	if m.GroupCode != 2 {
		t.Fatal("five matching groups must promote", m)
	}
}
func TestUnchangedACDecisionUpdatesHostPrediction(t *testing.T) {
	p := NewPredictor()
	k := PageKey{1, 9}
	p.Allocate(k)
	p.Change(k, SchemeOnTouch, SchemeDuplication)
	if p.Change(k, SchemeAccessCounter, SchemeAccessCounter) {
		t.Fatal("unchanged AC must suppress checks")
	}
	m, _ := p.Lookup(k)
	if m.Scheme != SchemeAccessCounter {
		t.Fatal("host prediction overrode actual decision")
	}
}
