package security

import (
	"testing"
	"time"
)

func TestCircuitBreakerClosed(t *testing.T) {
	cb := NewCircuitBreaker(3, 30*time.Second)

	if !cb.Allow() {
		t.Fatal("expected circuit breaker to allow request when closed")
	}
	if cb.State() != "closed" {
		t.Fatalf("expected state 'closed', got '%s'", cb.State())
	}
}

func TestCircuitBreakerOpensAfterFailures(t *testing.T) {
	cb := NewCircuitBreaker(3, 30*time.Second)

	// Record 3 failures
	cb.RecordFailure()
	cb.RecordFailure()
	cb.RecordFailure()

	if cb.State() != "open" {
		t.Fatalf("expected state 'open', got '%s'", cb.State())
	}
	if cb.Allow() {
		t.Fatal("expected circuit breaker to block requests when open")
	}
}

func TestCircuitBreakerHalfOpen(t *testing.T) {
	cb := NewCircuitBreaker(3, 1*time.Millisecond) // very short timeout

	cb.RecordFailure()
	cb.RecordFailure()
	cb.RecordFailure()

	// Wait for timeout
	time.Sleep(5 * time.Millisecond)

	// Should be half-open now
	if !cb.Allow() {
		t.Fatal("expected circuit breaker to allow request in half-open state")
	}
	if cb.State() != "half-open" {
		t.Fatalf("expected state 'half-open', got '%s'", cb.State())
	}
}

func TestCircuitBreakerRecovery(t *testing.T) {
	cb := NewCircuitBreaker(3, 1*time.Millisecond)

	// Open it
	cb.RecordFailure()
	cb.RecordFailure()
	cb.RecordFailure()

	// Wait for timeout
	time.Sleep(5 * time.Millisecond)

	// Half-open — record 2 successes to close
	cb.Allow()
	cb.RecordSuccess()
	cb.RecordSuccess()

	if cb.State() != "closed" {
		t.Fatalf("expected state 'closed' after recovery, got '%s'", cb.State())
	}
}
