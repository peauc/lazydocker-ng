package commands

import (
	"sync"
	"testing"
	"time"

	dcontainer "github.com/docker/docker/api/types/container"
)

func TestContainerFieldsConcurrency(t *testing.T) {
	c := &Container{ID: "test"}
	summary := dcontainer.Summary{State: "running"}
	var wg sync.WaitGroup
	for i := 0; i < 50; i++ {
		wg.Add(4)
		go func() { defer wg.Done(); c.SetSummary(summary) }()
		go func() { defer wg.Done(); _ = c.GetSummary() }()
		go func() { defer wg.Done(); c.SetMonitoringStats(true) }()
		go func() { defer wg.Done(); _ = c.IsMonitoringStats() }()
	}
	wg.Wait()
}

func TestGetLastStatsConcurrency(t *testing.T) {
	c := &Container{}
	c.StatHistory = []*RecordedStats{{RecordedAt: time.Now()}}
	var wg sync.WaitGroup
	for i := 0; i < 100; i++ {
		wg.Add(2)
		go func() {
			defer wg.Done()
			c.appendStats(&RecordedStats{RecordedAt: time.Now()}, 0)
		}()
		go func() {
			defer wg.Done()
			_, _ = c.GetLastStats()
		}()
	}
	wg.Wait()
}
