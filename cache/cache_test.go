package cache

import (
	"testing"
	"time"
)

func TestCacheSetAndGet(t *testing.T) {
	c := New(10)
	entry := &Entry{
		StatusCode: 200,
		Body:       []byte("hello"),
		ExpiresAt:  time.Now().Add(60 * time.Second),
		StaleAt:    time.Now().Add(70 * time.Second),
	}
	c.Set("GET:/hello", entry)

	got, ok := c.Get("GET:/hello")
	if !ok {
		t.Fatal("expected cache hit, got miss")
	}
	if string(got.Body) != "hello" {
		t.Fatalf("expected body 'hello', got '%s'", got.Body)
	}
}

func TestCacheMiss(t *testing.T) {
	c := New(10)
	_, ok := c.Get("GET:/notfound")
	if ok {
		t.Fatal("expected cache miss, got hit")
	}
}

func TestCacheTTLExpiry(t *testing.T) {
	c := New(10)
	entry := &Entry{
		StatusCode: 200,
		Body:       []byte("expired"),
		ExpiresAt:  time.Now().Add(-1 * time.Second), // already expired
		StaleAt:    time.Now().Add(-1 * time.Second),
	}
	c.Set("GET:/expired", entry)

	got, ok := c.Get("GET:/expired")
	if !ok {
		t.Fatal("expected to find entry")
	}
	if !got.IsExpired() {
		t.Fatal("expected entry to be expired")
	}
}

func TestCacheDelete(t *testing.T) {
	c := New(10)
	entry := &Entry{
		StatusCode: 200,
		Body:       []byte("hello"),
		ExpiresAt:  time.Now().Add(60 * time.Second),
		StaleAt:    time.Now().Add(70 * time.Second),
	}
	c.Set("GET:/hello", entry)
	c.Delete("GET:/hello")

	_, ok := c.Get("GET:/hello")
	if ok {
		t.Fatal("expected cache miss after delete")
	}
}

func TestCacheLRUEviction(t *testing.T) {
	c := New(3) // max 3 items
	for i := 0; i < 4; i++ {
		key := "GET:/page" + string(rune('0'+i))
		c.Set(key, &Entry{
			StatusCode: 200,
			Body:       []byte("body"),
			ExpiresAt:  time.Now().Add(60 * time.Second),
			StaleAt:    time.Now().Add(70 * time.Second),
		})
	}
	// Cache should only have 3 items
	stats := c.Stats()
	if stats["items"].(int) != 3 {
		t.Fatalf("expected 3 items after LRU eviction, got %d", stats["items"])
	}
}

func TestCacheInvalidatePrefix(t *testing.T) {
	c := New(10)
	for _, path := range []string{"/api/users", "/api/posts", "/health"} {
		c.Set("GET:"+path, &Entry{
			StatusCode: 200,
			Body:       []byte("body"),
			ExpiresAt:  time.Now().Add(60 * time.Second),
			StaleAt:    time.Now().Add(70 * time.Second),
		})
	}

	count := c.InvalidatePrefix("GET:/api")
	if count != 2 {
		t.Fatalf("expected 2 invalidated, got %d", count)
	}

	_, ok := c.Get("GET:/health")
	if !ok {
		t.Fatal("expected /health to still be cached")
	}
}

func TestStaleWhileRevalidate(t *testing.T) {
	c := New(10)
	entry := &Entry{
		StatusCode: 200,
		Body:       []byte("stale"),
		ExpiresAt:  time.Now().Add(-1 * time.Second), // expired
		StaleAt:    time.Now().Add(10 * time.Second),  // still in stale window
	}
	c.Set("GET:/stale", entry)

	got, ok := c.Get("GET:/stale")
	if !ok {
		t.Fatal("expected to find stale entry")
	}
	if !got.IsExpired() {
		t.Fatal("expected entry to be expired")
	}
	if got.IsStale() {
		t.Fatal("expected entry to still be in stale window")
	}
}
