package lb

import (
	"crypto/sha256"
	"encoding/binary"
	"fmt"
	"log/slog"
	"sort"
	"sync"
)

const defaultVnodes = 150 // virtual nodes per origin for even distribution

type hashRing struct {
	mu      sync.RWMutex
	ring    map[uint32]string // hash -> origin URL
	sorted  []uint32          // sorted hash keys
	vnodes  int
}

func newHashRing(vnodes int) *hashRing {
	return &hashRing{
		ring:   make(map[uint32]string),
		vnodes: vnodes,
	}
}

func (h *hashRing) hash(key string) uint32 {
	sum := sha256.Sum256([]byte(key))
	return binary.BigEndian.Uint32(sum[:4])
}

func (h *hashRing) add(origin string) {
	h.mu.Lock()
	defer h.mu.Unlock()
	for i := 0; i < h.vnodes; i++ {
		key := fmt.Sprintf("%s#vnode%d", origin, i)
		hash := h.hash(key)
		h.ring[hash] = origin
		h.sorted = append(h.sorted, hash)
	}
	sort.Slice(h.sorted, func(i, j int) bool {
		return h.sorted[i] < h.sorted[j]
	})
	slog.Info("added origin to hash ring", "origin", origin, "vnodes", h.vnodes)
}

func (h *hashRing) remove(origin string) {
	h.mu.Lock()
	defer h.mu.Unlock()
	for i := 0; i < h.vnodes; i++ {
		key := fmt.Sprintf("%s#vnode%d", origin, i)
		hash := h.hash(key)
		delete(h.ring, hash)
	}
	// Rebuild sorted slice
	h.sorted = h.sorted[:0]
	for k := range h.ring {
		h.sorted = append(h.sorted, k)
	}
	sort.Slice(h.sorted, func(i, j int) bool {
		return h.sorted[i] < h.sorted[j]
	})
	slog.Info("removed origin from hash ring", "origin", origin)
}

func (h *hashRing) get(key string) string {
	h.mu.RLock()
	defer h.mu.RUnlock()
	if len(h.sorted) == 0 {
		return ""
	}
	hash := h.hash(key)
	// Binary search for the first node >= hash
	idx := sort.Search(len(h.sorted), func(i int) bool {
		return h.sorted[i] >= hash
	})
	// Wrap around ring
	if idx == len(h.sorted) {
		idx = 0
	}
	return h.ring[h.sorted[idx]]
}

func (h *hashRing) size() int {
	h.mu.RLock()
	defer h.mu.RUnlock()
	return len(h.ring) / h.vnodes
}
