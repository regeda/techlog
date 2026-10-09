package service_test

import (
	"errors"
	"slices"
	"testing"

	"github.com/regeda/techlog/internal/event"
	"github.com/regeda/techlog/internal/service"
	"github.com/regeda/techlog/internal/store"
	"github.com/regeda/techlog/internal/testutil"
)

func recordAt(t *testing.T, svc *service.Service, ts, summary string, links ...map[string]any) event.Event {
	t.Helper()
	d := testutil.Doc("alpha", "problem.reported", summary, links...)
	d["time"] = ts
	return record(t, svc, d)
}

func summaries(evs []event.Event) []string {
	out := make([]string, len(evs))
	for i, e := range evs {
		out[i] = e.Summary
	}
	return out
}

func TestStory(t *testing.T) {
	svc, _ := newService(t)
	a := recordAt(t, svc, "2026-03-10T09:00:00Z", "a")
	b := recordAt(t, svc, "2026-03-10T10:00:00Z", "b", testutil.Link("motivated_by", a.ID))
	c := recordAt(t, svc, "2026-03-10T11:00:00Z", "c", testutil.Link("implements", b.ID))
	recordAt(t, svc, "2026-03-10T11:30:00Z", "unrelated")
	recordAt(t, svc, "2026-03-10T12:00:00Z", "e", testutil.Link("relates_to", a.ID))

	st, err := svc.Story(ctx, c.ID, 10)
	if err != nil {
		t.Fatal(err)
	}
	if st.Root != c.ID {
		t.Errorf("root = %q", st.Root)
	}
	if got := summaries(st.Events); !slices.Equal(got, []string{"a", "b", "c", "e"}) {
		t.Errorf("events = %v, want the connected set, oldest first", got)
	}
	if len(st.Edges) != 3 {
		t.Errorf("edges = %+v, want 3 (c->b, b->a, e->a)", st.Edges)
	}

	near, err := svc.Story(ctx, c.ID, 1)
	if err != nil {
		t.Fatal(err)
	}
	if got := summaries(near.Events); !slices.Equal(got, []string{"b", "c"}) {
		t.Errorf("depth 1 events = %v", got)
	}

	if _, err := svc.Story(ctx, "evt_000000000000", 5); !errors.Is(err, store.ErrNotFound) {
		t.Errorf("got %v, want ErrNotFound", err)
	}
}

func TestStoryTerminatesOnCycles(t *testing.T) {
	svc, _ := newService(t)
	a := recordAt(t, svc, "2026-03-10T09:00:00Z", "a")
	b := recordAt(t, svc, "2026-03-10T10:00:00Z", "b", testutil.Link("relates_to", a.ID))
	// a -> b closes a cycle: a relates_to b, b relates_to a.
	if _, err := svc.Link(ctx, service.LinkInput{Source: a.ID, Target: b.ID, Rel: "relates_to"}); err != nil {
		t.Fatal(err)
	}
	st, err := svc.Story(ctx, a.ID, 50)
	if err != nil {
		t.Fatal(err)
	}
	if len(st.Events) != 2 || len(st.Edges) != 2 {
		t.Fatalf("events=%d edges=%d, want 2 and 2", len(st.Events), len(st.Edges))
	}
}
