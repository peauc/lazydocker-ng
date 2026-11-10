package utils

import (
	"sync"
	"sync/atomic"
	"testing"
	"time"

	"github.com/stretchr/testify/assert"
)

func TestThrottleRunsLeadingEdgeImmediately(t *testing.T) {
	ran := make(chan struct{}, 1)
	throttle := NewThrottle(50*time.Millisecond, func() { ran <- struct{}{} })
	defer throttle.Stop()

	throttle.Trigger()

	select {
	case <-ran:
	case <-time.After(25 * time.Millisecond):
		t.Fatal("f did not run on the leading edge")
	}
}

// A burst that arrives before the leading run starts is absorbed into it; one
// that lands after it started gives one trailing run. Either way a burst never
// runs f once per trigger.
func TestThrottleCoalescesBurst(t *testing.T) {
	var calls atomic.Int32
	throttle := NewThrottle(50*time.Millisecond, func() { calls.Add(1) })
	defer throttle.Stop()

	for range 20 {
		throttle.Trigger()
	}
	time.Sleep(300 * time.Millisecond)

	assert.GreaterOrEqual(t, calls.Load(), int32(1))
	assert.LessOrEqual(t, calls.Load(), int32(2))
}

func TestThrottleRunsTrailingEdge(t *testing.T) {
	var calls atomic.Int32
	started := make(chan struct{}, 10)
	throttle := NewThrottle(50*time.Millisecond, func() {
		calls.Add(1)
		started <- struct{}{}
	})
	defer throttle.Stop()

	throttle.Trigger()
	<-started
	for range 5 {
		throttle.Trigger()
	}

	time.Sleep(20 * time.Millisecond)
	assert.Equal(t, int32(1), calls.Load(), "the trailing run waits for the period")
	time.Sleep(130 * time.Millisecond)
	assert.Equal(t, int32(2), calls.Load(), "exactly one trailing run")
}

func TestThrottleNeverOverlaps(t *testing.T) {
	var running, maxRunning, calls atomic.Int32
	throttle := NewThrottle(5*time.Millisecond, func() {
		now := running.Add(1)
		for {
			seen := maxRunning.Load()
			if now <= seen || maxRunning.CompareAndSwap(seen, now) {
				break
			}
		}
		time.Sleep(30 * time.Millisecond)
		running.Add(-1)
		calls.Add(1)
	})
	defer throttle.Stop()

	var wg sync.WaitGroup
	for range 10 {
		wg.Go(func() {
			for range 10 {
				throttle.Trigger()
				time.Sleep(2 * time.Millisecond)
			}
		})
	}
	wg.Wait()
	time.Sleep(100 * time.Millisecond)

	assert.Equal(t, int32(1), maxRunning.Load(), "f must never run concurrently")
	assert.Positive(t, calls.Load())
}

func TestThrottleStop(t *testing.T) {
	var calls atomic.Int32
	throttle := NewThrottle(10*time.Millisecond, func() { calls.Add(1) })

	throttle.Stop()
	throttle.Trigger()
	time.Sleep(100 * time.Millisecond)

	assert.Equal(t, int32(0), calls.Load())
}
