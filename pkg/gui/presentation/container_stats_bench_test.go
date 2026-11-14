package presentation

import (
	"testing"
	"time"

	"github.com/peauc/lazydocker-ng/pkg/commands"
	"github.com/peauc/lazydocker-ng/pkg/config"
)

// containerWithHistory has three minutes of one-second samples, which is what
// the default stats.maxDuration keeps.
func containerWithHistory() *commands.Container {
	container := &commands.Container{}
	start := time.Now().Add(-3 * time.Minute)
	for i := range 180 {
		stats := commands.ContainerStats{}
		stats.PidsStats.Current = 12
		stats.CPUStats.CPUUsage.PercpuUsage = make([]int64, 8)
		stats.MemoryStats.Usage = 100_000_000 + i
		stats.MemoryStats.Limit = 8_000_000_000
		container.StatHistory = append(container.StatHistory, &commands.RecordedStats{
			ClientStats: stats,
			DerivedStats: commands.DerivedStats{
				CPUPercentage:    float64(i % 100),
				MemoryPercentage: float64(i%50) / 2,
			},
			RecordedAt: start.Add(time.Duration(i) * time.Second),
		})
	}
	return container
}

func BenchmarkRenderStats(b *testing.B) {
	userConfig := config.GetDefaultConfig()
	container := containerWithHistory()

	b.ReportAllocs()
	for b.Loop() {
		if _, err := RenderStats(&userConfig, container, 120); err != nil {
			b.Fatal(err)
		}
	}
}
