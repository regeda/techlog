package event_test

import (
	"encoding/json"
	"strings"
	"testing"

	"github.com/regeda/techlog/internal/event"
)

func TestEventJSONCarriesIdempotencyKey(t *testing.T) {
	raw, err := json.Marshal(event.Event{IdempotencyKey: "deploy-4411"})
	if err != nil {
		t.Fatal(err)
	}
	if !strings.Contains(string(raw), `"idempotency_key":"deploy-4411"`) {
		t.Errorf("event JSON lacks the key: %s", raw)
	}
	var back event.Event
	if err := json.Unmarshal(raw, &back); err != nil || back.IdempotencyKey != "deploy-4411" {
		t.Errorf("round trip = %+v, %v", back, err)
	}
}
