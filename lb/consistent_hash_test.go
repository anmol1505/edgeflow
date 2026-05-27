package lb

import (
	"testing"
)

func TestHashRingAddAndGet(t *testing.T) {
	ring := newHashRing(10)
	ring.add("http://localhost:9000")
	ring.add("http://localhost:9001")

	if ring.size() != 2 {
		t.Fatalf("expected ring size 2, got %d", ring.size())
	}

	origin := ring.get("test-key")
	if origin == "" {
		t.Fatal("expected to get an origin, got empty string")
	}
}

func TestHashRingConsistency(t *testing.T) {
	ring := newHashRing(150)
	ring.add("http://localhost:9000")
	ring.add("http://localhost:9001")

	// Same key should always return same origin
	first := ring.get("user-123")
	for i := 0; i < 100; i++ {
		got := ring.get("user-123")
		if got != first {
			t.Fatalf("expected consistent routing, got different origins: %s vs %s", first, got)
		}
	}
}

func TestHashRingRemove(t *testing.T) {
	ring := newHashRing(10)
	ring.add("http://localhost:9000")
	ring.add("http://localhost:9001")
	ring.remove("http://localhost:9000")

	if ring.size() != 1 {
		t.Fatalf("expected ring size 1 after remove, got %d", ring.size())
	}

	// All requests should go to remaining origin
	for i := 0; i < 10; i++ {
		got := ring.get("any-key")
		if got != "http://localhost:9001" {
			t.Fatalf("expected all traffic to go to remaining origin, got %s", got)
		}
	}
}

func TestHashRingDistribution(t *testing.T) {
	ring := newHashRing(150)
	ring.add("http://localhost:9000")
	ring.add("http://localhost:9001")

	counts := map[string]int{}
	for i := 0; i < 1000; i++ {
		key := "key-" + string(rune(i))
		origin := ring.get(key)
		counts[origin]++
	}

	// Each origin should get roughly 50% of traffic (allow 20% variance)
	for origin, count := range counts {
		if count < 300 || count > 700 {
			t.Fatalf("uneven distribution for %s: %d/1000 requests", origin, count)
		}
	}
}
