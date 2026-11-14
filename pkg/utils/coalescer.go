package utils

import "sync"

// Coalescer runs f for callers that only need f to run once more after they
// ask, such as refreshes: f never runs twice at once, and any number of
// requests made during a run are answered by a single run after it.
type Coalescer struct {
	f func()

	mu      sync.Mutex
	running bool
	pending bool
}

func NewCoalescer(f func()) *Coalescer {
	return &Coalescer{f: f}
}

// Run runs f and returns when it is done. If f is already running, Run
// returns at once and the goroutine running f runs it one more time.
func (c *Coalescer) Run() {
	c.mu.Lock()
	if c.running {
		c.pending = true
		c.mu.Unlock()
		return
	}
	c.running = true
	c.mu.Unlock()

	for {
		c.f()

		c.mu.Lock()
		if !c.pending {
			c.running = false
			c.mu.Unlock()
			return
		}
		c.pending = false
		c.mu.Unlock()
	}
}
