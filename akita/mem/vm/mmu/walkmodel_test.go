package mmu

import (
	"github.com/sarchlab/akita/v3/mem/vm"
	"testing"
)

func TestWalkCacheLatencyAndIsolation(t *testing.T) {
	m := &MMU{latency: 100, walkLevels: 4, walkCacheEntries: 128, walkCaches: map[uint64]*walkCache{1: {entries: map[walkCacheKey]uint64{}}, 2: {entries: map[walkCacheKey]uint64{}}}}
	r := &vm.TranslationReq{PID: 1, DeviceID: 1, VAddr: 4096}
	if m.walkLatency(r) != 400 {
		t.Fatal("cold walk must read all levels")
	}
	m.fillWalkCache(r)
	if m.walkLatency(r) != 103 {
		t.Fatal("walk cache must skip three intermediate reads")
	}
	r.DeviceID = 2
	if m.walkLatency(r) != 400 {
		t.Fatal("GPU walk caches alias")
	}
	r.DeviceID = 1
	r.PID = 2
	if m.walkLatency(r) != 400 {
		t.Fatal("PID walk caches alias")
	}
}
