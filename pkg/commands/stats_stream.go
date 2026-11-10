package commands

import (
	"encoding/json"
	"errors"
	"fmt"
	"io"
)

// decodeStatsStream decodes the stream of JSON objects that the Docker stats
// API sends and calls handle for each one. It returns nil when the stream
// ends, and an error for anything else, such as malformed JSON or a read
// error (including the context being cancelled). A field with an unexpected
// type only spoils that field: the sample is still handled and decoding goes
// on. Unlike a line scanner it has no size limit per object.
func decodeStatsStream(r io.Reader, handle func(ContainerStats)) error {
	decoder := json.NewDecoder(r)
	for {
		var stats ContainerStats
		if err := decoder.Decode(&stats); err != nil {
			var typeErr *json.UnmarshalTypeError
			switch {
			case errors.Is(err, io.EOF):
				return nil
			case errors.As(err, &typeErr):
				// the decoder has consumed the whole object and is still in sync
			default:
				return fmt.Errorf("decoding stats stream: %w", err)
			}
		}
		handle(stats)
	}
}
