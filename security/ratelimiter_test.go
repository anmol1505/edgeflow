package security

import (
	"testing"
)

func TestRateLimiterAllow(t *testing.T) {
	rl := NewRateLimiter(10, 10)

	for i := 0; i < 10; i++ {
		if !rl.Allow("127.0.0.1") {
			t.Fatalf("expected request %d to be allowed", i+1)
		}
	}
}

func TestRateLimiterBlock(t *testing.T) {
	// capacity=3 so 3 requests allowed then blocked
	rl := NewRateLimiter(1, 3)

	allowed := 0
	for i := 0; i < 6; i++ {
		if rl.Allow("127.0.0.1") {
			allowed++
		}
	}
	if allowed != 3 {
		t.Fatalf("expected exactly 3 allowed requests, got %d", allowed)
	}
}

func TestRateLimiterPerIP(t *testing.T) {
	rl := NewRateLimiter(1, 1)

	// IP 1 uses its only token
	rl.Allow("1.1.1.1")

	// IP 1 should now be blocked
	if rl.Allow("1.1.1.1") {
		t.Fatal("expected IP 1 to be blocked")
	}

	// IP 2 has fresh bucket — should be allowed
	if !rl.Allow("2.2.2.2") {
		t.Fatal("expected IP 2 to be allowed")
	}
}
