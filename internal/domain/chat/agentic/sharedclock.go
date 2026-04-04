package agentic

import (
	"sync"
	"time"
)

// Shared clock with subscriber-driven lifecycle.
//
// Inspired by Claude Code's ClockContext — a single synchronized clock
// that all animations subscribe to. The clock only ticks when at least
// one keep-alive subscriber exists, minimizing CPU when idle. All
// subscribers in the same tick see the same time value, keeping
// animations perfectly synchronized.

// SharedClockSubscriber is a callback invoked on each tick.
type SharedClockSubscriber func()

type clockSub struct {
	fn        SharedClockSubscriber
	keepAlive bool
}

// SharedClock is a subscriber-driven tick clock.
type SharedClock struct {
	mu            sync.Mutex
	subscribers   map[int]*clockSub
	nextID        int
	ticker        *time.Ticker
	stopCh        chan struct{}
	tickInterval  time.Duration
	startTime     time.Time
	tickTime      int64 // ms since start, snapshot per tick
	running       bool
	closed        bool
}

// NewSharedClock creates a clock with the given tick interval.
func NewSharedClock(tickInterval time.Duration) *SharedClock {
	return &SharedClock{
		subscribers:  make(map[int]*clockSub),
		tickInterval: tickInterval,
	}
}

// Subscribe registers a callback. If keepAlive is true, the clock will
// run as long as this subscriber exists. Returns an unsubscribe function.
func (c *SharedClock) Subscribe(fn SharedClockSubscriber, keepAlive bool) func() {
	c.mu.Lock()
	defer c.mu.Unlock()

	if c.closed {
		return func() {}
	}

	id := c.nextID
	c.nextID++
	c.subscribers[id] = &clockSub{fn: fn, keepAlive: keepAlive}
	c.updateInterval()

	return func() {
		c.mu.Lock()
		defer c.mu.Unlock()
		delete(c.subscribers, id)
		c.updateInterval()
	}
}

// Now returns milliseconds since the clock started. When the clock is
// ticking, all callers within the same tick see the same value.
func (c *SharedClock) Now() int64 {
	c.mu.Lock()
	defer c.mu.Unlock()

	if c.startTime.IsZero() {
		c.startTime = time.Now()
	}

	if c.running && c.tickTime > 0 {
		return c.tickTime
	}
	return time.Since(c.startTime).Milliseconds()
}

// SetTickInterval changes the tick rate. If the clock is running it
// restarts with the new interval.
func (c *SharedClock) SetTickInterval(d time.Duration) {
	c.mu.Lock()
	defer c.mu.Unlock()
	if d == c.tickInterval {
		return
	}
	c.tickInterval = d
	if c.running {
		c.stopTicker()
		c.startTicker()
	}
}

// TickInterval returns the current tick interval.
func (c *SharedClock) TickInterval() time.Duration {
	c.mu.Lock()
	defer c.mu.Unlock()
	return c.tickInterval
}

// SubscriberCount returns the number of active subscribers.
func (c *SharedClock) SubscriberCount() int {
	c.mu.Lock()
	defer c.mu.Unlock()
	return len(c.subscribers)
}

// IsRunning returns true if the clock ticker is active.
func (c *SharedClock) IsRunning() bool {
	c.mu.Lock()
	defer c.mu.Unlock()
	return c.running
}

// Close stops the clock and removes all subscribers.
func (c *SharedClock) Close() {
	c.mu.Lock()
	defer c.mu.Unlock()
	c.closed = true
	c.stopTicker()
	c.subscribers = make(map[int]*clockSub)
}

// updateInterval starts or stops the ticker based on keep-alive subscribers.
// Caller must hold mu.
func (c *SharedClock) updateInterval() {
	hasKeepAlive := false
	for _, s := range c.subscribers {
		if s.keepAlive {
			hasKeepAlive = true
			break
		}
	}

	if hasKeepAlive && !c.running {
		c.startTicker()
	} else if !hasKeepAlive && c.running {
		c.stopTicker()
	}
}

// startTicker begins the tick loop. Caller must hold mu.
func (c *SharedClock) startTicker() {
	if c.startTime.IsZero() {
		c.startTime = time.Now()
	}
	c.running = true
	c.ticker = time.NewTicker(c.tickInterval)
	c.stopCh = make(chan struct{})

	// Capture by value so the goroutine doesn't read nil after stopTicker.
	tickCh := c.ticker.C
	stopCh := c.stopCh

	go func() {
		for {
			select {
			case <-tickCh:
				c.tick()
			case <-stopCh:
				return
			}
		}
	}()
}

// stopTicker halts the tick loop. Caller must hold mu.
func (c *SharedClock) stopTicker() {
	if !c.running {
		return
	}
	c.running = false
	if c.ticker != nil {
		c.ticker.Stop()
		c.ticker = nil
	}
	if c.stopCh != nil {
		close(c.stopCh)
		c.stopCh = nil
	}
}

// tick updates tickTime and notifies all subscribers.
func (c *SharedClock) tick() {
	c.mu.Lock()
	c.tickTime = time.Since(c.startTime).Milliseconds()
	// Copy subscribers to avoid holding lock during callbacks
	subs := make([]SharedClockSubscriber, 0, len(c.subscribers))
	for _, s := range c.subscribers {
		subs = append(subs, s.fn)
	}
	c.mu.Unlock()

	for _, fn := range subs {
		fn()
	}
}
