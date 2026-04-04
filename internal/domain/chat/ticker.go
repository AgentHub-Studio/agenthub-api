package chat

import "time"

// millisecondsUnit is used to construct ticker durations.
const millisecondsUnit = time.Millisecond

// ticker abstracts time.Ticker for testability.
type ticker interface {
	C() <-chan time.Time
	Stop()
}

// realTicker wraps time.Ticker.
type realTicker struct {
	t *time.Ticker
}

func (rt *realTicker) C() <-chan time.Time { return rt.t.C }
func (rt *realTicker) Stop()               { rt.t.Stop() }

// newTicker creates a real ticker. It can be overridden in tests.
var newTicker = func(d time.Duration) ticker {
	return &realTicker{t: time.NewTicker(d)}
}
