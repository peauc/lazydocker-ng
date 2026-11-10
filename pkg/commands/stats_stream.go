package commands

import (
	"encoding/json"
	"errors"
	"fmt"
	"io"
)

// decodeStatsStream decodes the stream of JSON objects that the Docker stats
// API sends and calls handle for each one. It returns nil when the stream
// ends, and an error for anything else, such as a malformed object or a read
// error (including the context being cancelled). Unlike a line scanner it has
// no size limit per object.
func decodeStatsStream(r io.Reader, handle func(ContainerStats)) error {
	decoder := json.NewDecoder(r)
	for {
		var stats ContainerStats
		if err := decoder.Decode(&stats); err != nil {
			if errors.Is(err, io.EOF) {
				return nil
			}
			return fmt.Errorf("decoding stats stream: %w", err)
		}
		handle(stats)
	}
}
