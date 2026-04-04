package agentic

import (
	"sync"
	"time"
)

// Cache-first API result pattern with background refresh.
//
// Inspired by Claude Code's ApiResult + isQualifiedForGrove — returns
// cached data immediately and fetches in the background when stale.
// Designed for optional features (notifications, settings, flags) where
// availability should never block the main flow.

// CacheFirstResult distinguishes between a successful fetch and a failure.
// On success Data is populated; on failure only Success is false.
type CacheFirstResult[T any] struct {
	Success bool
	Data    T
}

// CacheFirstFetchFunc fetches a value from a remote source.
type CacheFirstFetchFunc[T any] func() (T, error)

// CacheFirstConfig configures the cache-first store.
type CacheFirstConfig struct {
	// TTL is how long a cached entry is considered fresh.
	TTL time.Duration
	// OnError is called when a background fetch fails (optional).
	OnError func(error)
}

type cacheEntry[T any] struct {
	value     T
	fetchedAt time.Time
}

// CacheFirstStore provides non-blocking, cache-first access to a value.
type CacheFirstStore[T any] struct {
	mu        sync.RWMutex
	config    CacheFirstConfig
	fetchFunc CacheFirstFetchFunc[T]
	entry     *cacheEntry[T]
	fetching  bool
}

// NewCacheFirstStore creates a cache-first store.
func NewCacheFirstStore[T any](config CacheFirstConfig, fetchFunc CacheFirstFetchFunc[T]) *CacheFirstStore[T] {
	if config.TTL <= 0 {
		config.TTL = 24 * time.Hour
	}
	return &CacheFirstStore[T]{
		config:    config,
		fetchFunc: fetchFunc,
	}
}

// Get returns the cached value if fresh. If stale, returns the cached
// value and triggers a background refresh. If no cache exists, triggers
// a background fetch and returns a failure result (non-blocking).
func (s *CacheFirstStore[T]) Get() CacheFirstResult[T] {
	s.mu.RLock()
	entry := s.entry
	s.mu.RUnlock()

	if entry == nil {
		// No cache — fire background fetch, return failure
		s.triggerBackgroundFetch()
		var zero T
		return CacheFirstResult[T]{Success: false, Data: zero}
	}

	if time.Since(entry.fetchedAt) > s.config.TTL {
		// Stale — return cached value and refresh in background
		s.triggerBackgroundFetch()
		return CacheFirstResult[T]{Success: true, Data: entry.value}
	}

	// Fresh
	return CacheFirstResult[T]{Success: true, Data: entry.value}
}

// GetSync fetches the value synchronously, updating the cache.
// Returns a failure result on error.
func (s *CacheFirstStore[T]) GetSync() CacheFirstResult[T] {
	val, err := s.fetchFunc()
	if err != nil {
		if s.config.OnError != nil {
			s.config.OnError(err)
		}
		// On error, return cached if available
		s.mu.RLock()
		entry := s.entry
		s.mu.RUnlock()
		if entry != nil {
			return CacheFirstResult[T]{Success: true, Data: entry.value}
		}
		var zero T
		return CacheFirstResult[T]{Success: false, Data: zero}
	}

	s.mu.Lock()
	s.entry = &cacheEntry[T]{value: val, fetchedAt: time.Now()}
	s.mu.Unlock()

	return CacheFirstResult[T]{Success: true, Data: val}
}

// Invalidate clears the cache, forcing a fresh fetch on next Get.
func (s *CacheFirstStore[T]) Invalidate() {
	s.mu.Lock()
	s.entry = nil
	s.mu.Unlock()
}

// Set manually populates the cache with a value.
func (s *CacheFirstStore[T]) Set(value T) {
	s.mu.Lock()
	s.entry = &cacheEntry[T]{value: value, fetchedAt: time.Now()}
	s.mu.Unlock()
}

// IsFresh returns true if the cache has a non-expired entry.
func (s *CacheFirstStore[T]) IsFresh() bool {
	s.mu.RLock()
	defer s.mu.RUnlock()
	if s.entry == nil {
		return false
	}
	return time.Since(s.entry.fetchedAt) <= s.config.TTL
}

// IsFetching returns true if a background fetch is in progress.
func (s *CacheFirstStore[T]) IsFetching() bool {
	s.mu.RLock()
	defer s.mu.RUnlock()
	return s.fetching
}

// triggerBackgroundFetch starts a background goroutine to fetch and cache.
func (s *CacheFirstStore[T]) triggerBackgroundFetch() {
	s.mu.Lock()
	if s.fetching {
		s.mu.Unlock()
		return
	}
	s.fetching = true
	s.mu.Unlock()

	go func() {
		defer func() {
			s.mu.Lock()
			s.fetching = false
			s.mu.Unlock()
		}()

		val, err := s.fetchFunc()
		if err != nil {
			if s.config.OnError != nil {
				s.config.OnError(err)
			}
			return
		}

		s.mu.Lock()
		s.entry = &cacheEntry[T]{value: val, fetchedAt: time.Now()}
		s.mu.Unlock()
	}()
}
