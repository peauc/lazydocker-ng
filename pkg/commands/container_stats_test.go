package commands

import (
	"encoding/json"
	"testing"

	"github.com/mcuadros/go-lookup"
	"github.com/stretchr/testify/assert"
)

func TestCalculateContainerCPUPercentage(t *testing.T) {
	container := &ContainerStats{}
	container.CPUStats.CPUUsage.TotalUsage = 10
	container.CPUStats.SystemCPUUsage = 10
	container.PrecpuStats.CPUUsage.TotalUsage = 5
	container.PrecpuStats.SystemCPUUsage = 2

	assert.EqualValues(t, 62.5, container.CalculateContainerCPUPercentage())
}

const twoInterfaces = `{"networks":{"eth0":{"rx_bytes":100,"tx_bytes":10},"eth1":{"rx_bytes":50,"tx_bytes":5}}}`

func TestContainerNetworksSumsAllInterfaces(t *testing.T) {
	var stats ContainerStats
	assert.NoError(t, json.Unmarshal([]byte(twoInterfaces), &stats))

	assert.Equal(t, 150, stats.Networks.Total.RxBytes)
	assert.Equal(t, 15, stats.Networks.Total.TxBytes)
	assert.Equal(t, 100, stats.Networks.Eth0.RxBytes)
	assert.Len(t, stats.Networks.Interfaces, 2)
}

func TestContainerNetworksAbsent(t *testing.T) {
	var stats ContainerStats
	assert.NoError(t, json.Unmarshal([]byte(`{}`), &stats))

	assert.Equal(t, NetworkStats{}, stats.Networks.Total)
	assert.Equal(t, NetworkStats{}, stats.Networks.Eth0)
}

func TestStatPathsStillResolve(t *testing.T) {
	var stats ContainerStats
	assert.NoError(t, json.Unmarshal([]byte(twoInterfaces), &stats))
	recorded := RecordedStats{ClientStats: stats}

	for path, want := range map[string]int{
		"ClientStats.Networks.Eth0.RxBytes":            100,
		"ClientStats.Networks.Total.RxBytes":           150,
		"ClientStats.Networks.Interfaces.eth1.RxBytes": 50,
	} {
		value, err := lookup.LookupString(recorded, path)
		if assert.NoError(t, err, path) {
			assert.EqualValues(t, want, value.Interface(), path)
		}
	}
}

func TestCPUPercentageZeroSystemDelta(t *testing.T) {
	stats := &ContainerStats{}
	stats.CPUStats.CPUUsage.TotalUsage = 200
	stats.PrecpuStats.CPUUsage.TotalUsage = 100
	stats.CPUStats.SystemCPUUsage = 50
	stats.PrecpuStats.SystemCPUUsage = 50

	assert.Equal(t, 0.0, stats.CalculateContainerCPUPercentage())
}

func TestMemoryPercentageZeroLimit(t *testing.T) {
	stats := &ContainerStats{}
	stats.MemoryStats.Usage = 10
	stats.MemoryStats.Limit = 0

	assert.Equal(t, 0.0, stats.CalculateContainerMemoryUsage())
}
