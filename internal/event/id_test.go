package event_test

import (
	"bytes"
	"regexp"
	"testing"
	"time"

	"github.com/regeda/techlog/internal/event"
)

func TestNewID(t *testing.T) {
	re := regexp.MustCompile(`^evt_[0-9A-Z]{12}$`)
	t1 := time.Date(2026, 3, 10, 9, 0, 0, 0, time.UTC)
	t2 := t1.Add(time.Millisecond)

	// The earlier id gets the largest possible random tail; it must still sort first.
	a, err := event.NewID(t1, bytes.NewReader([]byte{255, 255, 255, 255}))
	if err != nil {
		t.Fatal(err)
	}
	b, err := event.NewID(t2, bytes.NewReader([]byte{0, 0, 0, 0}))
	if err != nil {
		t.Fatal(err)
	}
	for _, id := range []string{a, b} {
		if !re.MatchString(id) {
			t.Errorf("id %q does not match %s", id, re)
		}
	}
	if a >= b {
		t.Errorf("ids must sort by creation time: %s should be < %s", a, b)
	}
}

func TestNewIDShortEntropy(t *testing.T) {
	if _, err := event.NewID(time.Now(), bytes.NewReader(nil)); err == nil {
		t.Fatal("expected an error when the entropy source is empty")
	}
}
