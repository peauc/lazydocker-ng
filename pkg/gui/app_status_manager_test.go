package gui

import (
	"sync"
	"testing"
)

func TestStatusManagerConcurrency(t *testing.T) {
	m := &statusManager{}
	var wg sync.WaitGroup
	for i := 0; i < 100; i++ {
		wg.Add(3)
		go func() { defer wg.Done(); m.addWaitingStatus("task-a") }()
		go func() { defer wg.Done(); m.removeStatus("task-a") }()
		go func() { defer wg.Done(); _ = m.getStatusString() }()
	}
	wg.Wait()
}
