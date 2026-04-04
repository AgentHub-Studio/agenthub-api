package agentic

import (
	"sync"
	"time"
)

// MemoizeWithTTL wraps a function with a TTL cache that serves stale results
// while refreshing in the background. Thread-safe.
//
// Inspired by Claude Code's memoizeWithTTL in utils/memoize.ts.
type MemoizeWithTTL[K comparable, V any] struct {
	mu       sync.RWMutex
	fn       func(K) V
	cache    map[K]*ttlEntry[V]
	lifetime time.Duration
}

type ttlEntry[V any] struct {
	value      V
	timestamp  time.Time
	refreshing bool
}

// NewMemoizeWithTTL creates a synchronous TTL-based memoizer.
// Default lifetime is 5 minutes if ttl <= 0.
func NewMemoizeWithTTL[K comparable, V any](fn func(K) V, ttl time.Duration) *MemoizeWithTTL[K, V] {
	if ttl <= 0 {
		ttl = 5 * time.Minute
	}
	return &MemoizeWithTTL[K, V]{
		fn:       fn,
		cache:    make(map[K]*ttlEntry[V]),
		lifetime: ttl,
	}
}

// Get returns the cached value or computes it. Serves stale values while
// refreshing in the background.
func (m *MemoizeWithTTL[K, V]) Get(key K) V {
	now := time.Now()

	m.mu.RLock()
	entry, ok := m.cache[key]
	m.mu.RUnlock()

	if ok {
		if now.Sub(entry.timestamp) <= m.lifetime {
			return entry.value // fresh
		}
		// Stale — refresh in background if not already doing so.
		m.mu.Lock()
		if !entry.refreshing {
			entry.refreshing = true
			go func() {
				val := m.fn(key)
				m.mu.Lock()
				m.cache[key] = &ttlEntry[V]{value: val, timestamp: time.Now()}
				m.mu.Unlock()
			}()
		}
		stale := entry.value
		m.mu.Unlock()
		return stale
	}

	// Cold miss — compute synchronously.
	val := m.fn(key)
	m.mu.Lock()
	m.cache[key] = &ttlEntry[V]{value: val, timestamp: now}
	m.mu.Unlock()
	return val
}

// Clear removes all cached entries.
func (m *MemoizeWithTTL[K, V]) Clear() {
	m.mu.Lock()
	m.cache = make(map[K]*ttlEntry[V])
	m.mu.Unlock()
}

// Size returns the number of cached entries.
func (m *MemoizeWithTTL[K, V]) Size() int {
	m.mu.RLock()
	defer m.mu.RUnlock()
	return len(m.cache)
}

// --- Async TTL Memoizer with in-flight dedup ---

// MemoizeAsyncWithTTL wraps an async function with TTL caching and
// in-flight deduplication. Concurrent cold-miss callers share one result.
//
// Inspired by Claude Code's memoizeWithTTLAsync.
type MemoizeAsyncWithTTL[K comparable, V any] struct {
	mu       sync.Mutex
	fn       func(K) (V, error)
	cache    map[K]*ttlEntry[V]
	inFlight map[K]*inFlightEntry[V]
	lifetime time.Duration
}

type inFlightEntry[V any] struct {
	done chan struct{}
	val  V
	err  error
}

// NewMemoizeAsyncWithTTL creates an async TTL memoizer with in-flight dedup.
func NewMemoizeAsyncWithTTL[K comparable, V any](fn func(K) (V, error), ttl time.Duration) *MemoizeAsyncWithTTL[K, V] {
	if ttl <= 0 {
		ttl = 5 * time.Minute
	}
	return &MemoizeAsyncWithTTL[K, V]{
		fn:       fn,
		cache:    make(map[K]*ttlEntry[V]),
		inFlight: make(map[K]*inFlightEntry[V]),
		lifetime: ttl,
	}
}

// Get returns the cached value or computes it. Concurrent callers for the same
// key share one in-flight computation.
func (m *MemoizeAsyncWithTTL[K, V]) Get(key K) (V, error) {
	now := time.Now()

	m.mu.Lock()
	entry, ok := m.cache[key]
	if ok && now.Sub(entry.timestamp) <= m.lifetime {
		val := entry.value
		m.mu.Unlock()
		return val, nil
	}

	// Check in-flight.
	if pending, ok := m.inFlight[key]; ok {
		m.mu.Unlock()
		<-pending.done
		return pending.val, pending.err
	}

	// Start computation.
	flight := &inFlightEntry[V]{done: make(chan struct{})}
	m.inFlight[key] = flight
	m.mu.Unlock()

	val, err := m.fn(key)
	flight.val = val
	flight.err = err
	close(flight.done)

	m.mu.Lock()
	// Identity guard: only update if still our flight.
	if m.inFlight[key] == flight {
		delete(m.inFlight, key)
		if err == nil {
			m.cache[key] = &ttlEntry[V]{value: val, timestamp: time.Now()}
		}
	}
	m.mu.Unlock()

	return val, err
}

// Clear removes all cached entries and in-flight trackers.
func (m *MemoizeAsyncWithTTL[K, V]) Clear() {
	m.mu.Lock()
	m.cache = make(map[K]*ttlEntry[V])
	m.inFlight = make(map[K]*inFlightEntry[V])
	m.mu.Unlock()
}

// Size returns the number of cached entries.
func (m *MemoizeAsyncWithTTL[K, V]) Size() int {
	m.mu.Lock()
	defer m.mu.Unlock()
	return len(m.cache)
}

// --- LRU Cache ---

// LRUCache is a thread-safe least-recently-used cache with fixed capacity.
//
// Inspired by Claude Code's memoizeWithLRU.
type LRUCache[K comparable, V any] struct {
	mu       sync.Mutex
	cap      int
	items    map[K]*lruNode[K, V]
	head     *lruNode[K, V] // most recent
	tail     *lruNode[K, V] // least recent
}

type lruNode[K comparable, V any] struct {
	key  K
	val  V
	prev *lruNode[K, V]
	next *lruNode[K, V]
}

// NewLRUCache creates an LRU cache with the given capacity.
func NewLRUCache[K comparable, V any](capacity int) *LRUCache[K, V] {
	if capacity <= 0 {
		capacity = 100
	}
	return &LRUCache[K, V]{
		cap:   capacity,
		items: make(map[K]*lruNode[K, V]),
	}
}

// Get retrieves a value and marks it as recently used. Returns (value, true)
// or (zero, false) if not found.
func (c *LRUCache[K, V]) Get(key K) (V, bool) {
	c.mu.Lock()
	defer c.mu.Unlock()

	node, ok := c.items[key]
	if !ok {
		var zero V
		return zero, false
	}
	c.moveToFront(node)
	return node.val, true
}

// Peek retrieves a value without updating recency.
func (c *LRUCache[K, V]) Peek(key K) (V, bool) {
	c.mu.Lock()
	defer c.mu.Unlock()

	node, ok := c.items[key]
	if !ok {
		var zero V
		return zero, false
	}
	return node.val, true
}

// Set stores a key-value pair, evicting the least-recently-used entry if at capacity.
func (c *LRUCache[K, V]) Set(key K, val V) {
	c.mu.Lock()
	defer c.mu.Unlock()

	if node, ok := c.items[key]; ok {
		node.val = val
		c.moveToFront(node)
		return
	}

	node := &lruNode[K, V]{key: key, val: val}
	c.items[key] = node
	c.pushFront(node)

	if len(c.items) > c.cap {
		c.evict()
	}
}

// Delete removes a key.
func (c *LRUCache[K, V]) Delete(key K) bool {
	c.mu.Lock()
	defer c.mu.Unlock()

	node, ok := c.items[key]
	if !ok {
		return false
	}
	c.remove(node)
	delete(c.items, key)
	return true
}

// Has checks if a key exists.
func (c *LRUCache[K, V]) Has(key K) bool {
	c.mu.Lock()
	defer c.mu.Unlock()
	_, ok := c.items[key]
	return ok
}

// Size returns the current number of entries.
func (c *LRUCache[K, V]) Size() int {
	c.mu.Lock()
	defer c.mu.Unlock()
	return len(c.items)
}

// Cap returns the max capacity.
func (c *LRUCache[K, V]) Cap() int {
	return c.cap
}

// Clear removes all entries.
func (c *LRUCache[K, V]) Clear() {
	c.mu.Lock()
	defer c.mu.Unlock()
	c.items = make(map[K]*lruNode[K, V])
	c.head = nil
	c.tail = nil
}

// linked list operations (caller must hold mu)

func (c *LRUCache[K, V]) pushFront(node *lruNode[K, V]) {
	node.prev = nil
	node.next = c.head
	if c.head != nil {
		c.head.prev = node
	}
	c.head = node
	if c.tail == nil {
		c.tail = node
	}
}

func (c *LRUCache[K, V]) remove(node *lruNode[K, V]) {
	if node.prev != nil {
		node.prev.next = node.next
	} else {
		c.head = node.next
	}
	if node.next != nil {
		node.next.prev = node.prev
	} else {
		c.tail = node.prev
	}
}

func (c *LRUCache[K, V]) moveToFront(node *lruNode[K, V]) {
	if c.head == node {
		return
	}
	c.remove(node)
	c.pushFront(node)
}

func (c *LRUCache[K, V]) evict() {
	if c.tail == nil {
		return
	}
	delete(c.items, c.tail.key)
	c.remove(c.tail)
}
