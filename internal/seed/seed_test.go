package seed_test

import (
	"context"
	"testing"

	"github.com/regeda/techlog/internal/seed"
	"github.com/regeda/techlog/internal/testutil"
)

// linksPointBackwards checks that every link points at an event listed
// earlier, so Import can insert in order.
func linksPointBackwards(t *testing.T, docs []map[string]any) {
	t.Helper()
	seen := map[string]bool{}
	for _, d := range docs {
		id := testutil.As[string](t, d["id"])
		if id == "" || seen[id] {
			t.Fatalf("missing or duplicate id %q", id)
		}
		if links, ok := d["links"]; ok {
			for _, l := range testutil.As[[]any](t, links) {
				if target := testutil.As[string](t, testutil.As[map[string]any](t, l)["target"]); !seen[target] {
					t.Errorf("%s links to %s, which is not listed before it", id, target)
				}
			}
		}
		seen[id] = true
	}
}

func TestSpringSpin(t *testing.T) {
	docs, err := seed.SpringSpin()
	if err != nil {
		t.Fatal(err)
	}
	if len(docs) != 25 {
		t.Fatalf("got %d events, want 25", len(docs))
	}
	linksPointBackwards(t, docs)

	// The whole story must be valid against the schema and importable.
	svc := testutil.NewService(t)
	inserted, skipped, err := svc.Import(context.Background(), docs)
	if err != nil || inserted != 25 || skipped != 0 {
		t.Fatalf("import = %d, %d, %v", inserted, skipped, err)
	}
	inserted, skipped, err = svc.Import(context.Background(), docs)
	if err != nil || inserted != 0 || skipped != 25 {
		t.Fatalf("second import = %d, %d, %v", inserted, skipped, err)
	}
}
