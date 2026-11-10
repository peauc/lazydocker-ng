package utils

import (
	"sync"
	"time"
)

// Throttle runs f on its own goroutine, never two at once and at most once per
// period. Trigger runs f right away when the last run started more than period
// ago; otherwise it schedules one run for the end of the period. Triggers that
// arrive while a run is pending are absorbed into it.
type Throttle struct {
	pending  chan struct{}
	stop     chan struct{}
	stopOnce sync.Once
}

func NewThrottle(period time.Duration, f func()) *Throttle {
	t := &Throttle{
		pending: make(chan struct{}, 1),
		stop:    make(chan struct{}),
	}

	go func() {
		var lastStart time.Time
		for {
			select {
			case <-t.stop:
				return
			case <-t.pending:
			}

			if wait := period - time.Since(lastStart); wait > 0 {
				timer := time.NewTimer(wait)
				select {
				case <-t.stop:
					timer.Stop()
					return
				case <-timer.C:
				}
			}

			// triggers that arrived while waiting are absorbed into this run
			select {
			case <-t.pending:
			default:
			}
			// the select above may have picked a trigger although Stop was called
			select {
			case <-t.stop:
				return
			default:
			}

			lastStart = time.Now()
			f()
		}
	}()

	return t
}

// Trigger asks for a run of f.
func (t *Throttle) Trigger() {
	select {
	case t.pending <- struct{}{}:
	default:
	}
}

// Stop ends the throttle; later triggers are ignored.
func (t *Throttle) Stop() {
	t.stopOnce.Do(func() { close(t.stop) })
}
