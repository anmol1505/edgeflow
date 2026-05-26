package cache

import (
	"container/list"
	"log/slog"
	"net/http"
	"sync"
	"time"
)

type Entry struct {
	StatusCode int
	Headers    http.Header
	Body       []byte
	ExpiresAt  time.Time
	StaleAt    time.Time // stale-while-revalidate window
	ETag       string
}

func (e *Entry) IsExpired() bool {
	return time.Now().After(e.ExpiresAt)
}

func (e *Entry) IsStale() bool {
	return time.Now().After(e.StaleAt)
}

type item struct {
	key   string
	entry *Entry
}

type Cache struct {
	mu       sync.RWMutex
	items    map[string]*list.Element
	lru      *list.List
	maxItems int
	inflight sync.Map // for request deduplication
}

func New(maxItems int) *Cache {
	c := &Cache{
		items:    make(map[string]*list.Element),
		lru:      list.New(),
		maxItems: maxItems,
	}
	go c.cleanupLoop()
	return c
}

func (c *Cache) Get(key string) (*Entry, bool) {
	c.mu.RLock()
	defer c.mu.RUnlock()
	el, ok := c.items[key]
	if !ok {
		return nil, false
	}
	c.lru.MoveToFront(el)
	return el.Value.(*item).entry, true
}

func (c *Cache) Set(key string, entry *Entry) {
	c.mu.Lock()
	defer c.mu.Unlock()

	if el, ok := c.items[key]; ok {
		c.lru.MoveToFront(el)
		el.Value.(*item).entry = entry
		return
	}

	if c.lru.Len() >= c.maxItems {
		c.evict()
	}

	el := c.lru.PushFront(&item{key: key, entry: entry})
	c.items[key] = el
}

func (c *Cache) Delete(key string) {
	c.mu.Lock()
	defer c.mu.Unlock()
	if el, ok := c.items[key]; ok {
		c.lru.Remove(el)
		delete(c.items, key)
	}
}

func (c *Cache) InvalidatePrefix(prefix string) int {
	c.mu.Lock()
	defer c.mu.Unlock()
	count := 0
	for key, el := range c.items {
		if len(key) >= len(prefix) && key[:len(prefix)] == prefix {
			c.lru.Remove(el)
			delete(c.items, key)
			count++
		}
	}
	return count
}

func (c *Cache) evict() {
	el := c.lru.Back()
	if el == nil {
		return
	}
	c.lru.Remove(el)
	delete(c.items, el.Value.(*item).key)
}

func (c *Cache) cleanupLoop() {
	for {
		time.Sleep(30 * time.Second)
		c.mu.Lock()
		for key, el := range c.items {
			if el.Value.(*item).entry.IsExpired() {
				c.lru.Remove(el)
				delete(c.items, key)
				slog.Info("cache entry expired", "key", key)
			}
		}
		c.mu.Unlock()
	}
}

func (c *Cache) Stats() map[string]any {
	c.mu.RLock()
	defer c.mu.RUnlock()
	return map[string]any{
		"items":     c.lru.Len(),
		"max_items": c.maxItems,
	}
}
