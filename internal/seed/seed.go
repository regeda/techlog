// Package seed embeds the example story shown in the docs, so a fresh server
// has something to explore.
package seed

import (
	"bytes"
	_ "embed"
	"encoding/json"
	"fmt"
)

//go:embed spring_spin.json
var springSpin []byte

// SpringSpin returns the simulated "Spring Spin" story: 25 events, oldest
// first, each link pointing at an earlier event. Pass it to service.Import.
func SpringSpin() ([]map[string]any, error) {
	dec := json.NewDecoder(bytes.NewReader(springSpin))
	dec.UseNumber()
	var docs []map[string]any
	if err := dec.Decode(&docs); err != nil {
		return nil, fmt.Errorf("decode spring spin seed: %w", err)
	}
	return docs, nil
}
