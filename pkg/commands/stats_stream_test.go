package commands

import (
	"bytes"
	"context"
	"encoding/json"
	"strings"
	"testing"

	"github.com/stretchr/testify/assert"
)

func TestDecodeStatsStreamLongLine(t *testing.T) {
	big := ContainerStats{}
	big.CPUStats.CPUUsage.PercpuUsage = make([]int64, 20000)
	for i := range big.CPUStats.CPUUsage.PercpuUsage {
		big.CPUStats.CPUUsage.PercpuUsage[i] = 123456789
	}
	line, err := json.Marshal(big)
	assert.NoError(t, err)
	assert.Greater(t, len(line), 64*1024, "the first line must exceed bufio.Scanner's default limit")
	stream := string(line) + "\n" + `{"pids_stats":{"current":7}}` + "\n"

	var got []ContainerStats
	err = decodeStatsStream(strings.NewReader(stream), func(s ContainerStats) { got = append(got, s) })

	assert.NoError(t, err)
	if assert.Len(t, got, 2) {
		assert.Len(t, got[0].CPUStats.CPUUsage.PercpuUsage, 20000)
		assert.Equal(t, 7, got[1].PidsStats.Current)
	}
}

func TestDecodeStatsStreamMalformed(t *testing.T) {
	calls := 0

	err := decodeStatsStream(strings.NewReader(`{"read":"2026-01-01T00:00:00Z"}`+"\n{oops"), func(ContainerStats) { calls++ })

	assert.Equal(t, 1, calls)
	assert.ErrorContains(t, err, "decoding stats stream")
}

func TestDecodeStatsStreamEOF(t *testing.T) {
	calls := 0

	err := decodeStatsStream(&bytes.Buffer{}, func(ContainerStats) { calls++ })

	assert.NoError(t, err)
	assert.Equal(t, 0, calls)
}

type cancelledReader struct{}

func (cancelledReader) Read([]byte) (int, error) { return 0, context.Canceled }

func TestDecodeStatsStreamPassesCancel(t *testing.T) {
	err := decodeStatsStream(cancelledReader{}, func(ContainerStats) {})

	assert.ErrorIs(t, err, context.Canceled)
}

// A field with an unexpected type (e.g. from a Docker-compatible daemon) only
// spoils that sample, as with the old json.Unmarshal: the stream goes on.
func TestDecodeStatsStreamSurvivesTypeMismatch(t *testing.T) {
	stream := `{"pids_stats":{"current":"oops"},"num_procs":3}` + "\n" + `{"pids_stats":{"current":7}}` + "\n"

	var got []ContainerStats
	err := decodeStatsStream(strings.NewReader(stream), func(s ContainerStats) { got = append(got, s) })

	assert.NoError(t, err)
	if assert.Len(t, got, 2) {
		assert.Equal(t, 3, got[0].NumProcs, "the rest of the mismatched sample is kept")
		assert.Equal(t, 7, got[1].PidsStats.Current)
	}
}
