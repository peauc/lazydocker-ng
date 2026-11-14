package utils

import (
	"sync"
	"sync/atomic"
	"testing"

	"github.com/stretchr/testify/assert"
)

func TestCoalescerRunsSequentialCallsEachTime(t *testing.T) {
	runs := 0
	c := NewCoalescer(func() { runs++ })

	c.Run()
	c.Run()
	c.Run()

	assert.Equal(t, 3, runs)
}

func TestCoalescerFoldsCallsMadeDuringARun(t *testing.T) {
	var runs, concurrent, maxConcurrent atomic.Int32
	started := make(chan struct{}, 10)
	release := make(chan struct{})

	c := NewCoalescer(func() {
		if now := concurrent.Add(1); now > maxConcurrent.Load() {
			maxConcurrent.Store(now)
		}
		runs.Add(1)
		started <- struct{}{}
		<-release
		concurrent.Add(-1)
	})

	var first sync.WaitGroup
	first.Go(c.Run)
	<-started

	// these arrive while the first run is in progress: they return at once
	for range 5 {
		c.Run()
	}
	assert.EqualValues(t, 1, runs.Load())

	close(release)
	first.Wait()

	assert.EqualValues(t, 2, runs.Load(), "five requests during a run share one run after it")
	assert.EqualValues(t, 1, maxConcurrent.Load(), "f never runs twice at once")

	c.Run()
	assert.EqualValues(t, 3, runs.Load(), "idle again, so the next call runs f")
}
