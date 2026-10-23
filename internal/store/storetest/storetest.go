// Package storetest is the contract suite every store backend must pass.
//
//	func TestContract(t *testing.T) {
//		storetest.Run(t, func(t *testing.T) store.Store { ... fresh, empty store ... })
//	}
package storetest

import (
	"context"
	"errors"
	"fmt"
	"reflect"
	"slices"
	"sync"
	"testing"
	"time"

	"github.com/regeda/techlog/internal/event"
	"github.com/regeda/techlog/internal/store"
)

var ctx = context.Background()

// Run executes the suite. newStore must return a fresh, empty store each call.
func Run(t *testing.T, newStore func(t *testing.T) store.Store) {
	t.Helper()
	cases := map[string]func(*testing.T, store.Store){
		"InsertGetRoundTrip":     insertGetRoundTrip,
		"GetMissing":             getMissing,
		"InsertDuplicate":        insertDuplicate,
		"InsertDuplicateKey":     insertDuplicateKey,
		"KeyScopeIsPerActor":     keyScopeIsPerActor,
		"KeysAreOpaque":          keysAreOpaque,
		"DuplicateLeavesNoTrace": duplicateLeavesNoTrace,
		"ConcurrentSameKey":      concurrentSameKey,
		"InsertWithLinks":        insertWithLinks,
		"InsertDanglingLink":     insertDanglingLink,
		"AddLink":                addLink,
		"SearchFilters":          searchFilters,
		"SearchOffset":           searchOffset,
		"SearchTextPunctuation":  searchTextPunctuation,
		"Systems":                systems,
		"TimeOffsets":            timeOffsets,
		"ConcurrentInserts":      concurrentInserts,
	}
	for name, fn := range cases {
		t.Run(name, func(t *testing.T) { fn(t, newStore(t)) })
	}
}

func mk(id, typ, system, ts, summary string, links ...event.LinkRef) event.Event {
	return event.Event{
		SchemaVersion:  event.SchemaVersion,
		ID:             id,
		Type:           typ,
		Time:           ts,
		Actor:          event.Actor{Kind: "human", ID: "user:a"},
		IdempotencyKey: "key-" + id,
		Subject:        event.Subject{System: system},
		Summary:        summary,
		Links:          links,
		Provenance:     event.Provenance{RecordedBy: "user:a", Method: "human_entered"},
	}
}

func ref(rel, target, basis string) event.LinkRef {
	return event.LinkRef{Rel: rel, Target: target, Basis: basis}
}

func insert(t *testing.T, s store.Store, evs ...event.Event) {
	t.Helper()
	for _, e := range evs {
		if err := s.Insert(ctx, e); err != nil {
			t.Fatalf("insert %s: %v", e.ID, err)
		}
	}
}

func ids(evs []event.Event) []string {
	out := make([]string, len(evs))
	for i, e := range evs {
		out[i] = e.ID
	}
	return out
}

func linkKeys(links []event.Link) []string {
	out := make([]string, len(links))
	for i, l := range links {
		out[i] = l.Source + "|" + l.Target + "|" + l.Rel + "|" + l.Basis
	}
	slices.Sort(out)
	return out
}

func search(t *testing.T, s store.Store, f store.Filter) []string {
	t.Helper()
	got, err := s.Search(ctx, f)
	if err != nil {
		t.Fatalf("search %+v: %v", f, err)
	}
	return ids(got)
}

func expectIDs(t *testing.T, name string, got []string, want ...string) {
	t.Helper()
	if len(got) == 0 && len(want) == 0 {
		return
	}
	if !slices.Equal(got, want) {
		t.Errorf("%s: got %v, want %v", name, got, want)
	}
}

func insertGetRoundTrip(t *testing.T, s store.Store) {
	e := mk("evt_00000000000A", "problem.reported", "alpha", "2026-03-10T09:00:00Z", "first")
	e.Sources = []event.Source{{Kind: "ticket", Ref: "SUP-1"}}
	e.Extensions = map[string]any{"org.techlog.x": map[string]any{"k": "v"}}
	insert(t, s, e)
	got, err := s.Get(ctx, e.ID)
	if err != nil {
		t.Fatal(err)
	}
	if !reflect.DeepEqual(got, e) {
		t.Fatalf("round trip mismatch:\n got %+v\nwant %+v", got, e)
	}
}

func getMissing(t *testing.T, s store.Store) {
	if _, err := s.Get(ctx, "evt_MISSING00000"); !errors.Is(err, store.ErrNotFound) {
		t.Fatalf("got %v, want ErrNotFound", err)
	}
}

func insertDuplicate(t *testing.T, s store.Store) {
	e := mk("evt_00000000000A", "problem.reported", "alpha", "2026-03-10T09:00:00Z", "first")
	insert(t, s, e)
	dup := e
	dup.IdempotencyKey = "another-key" // same id, different key: only the id collides
	if err := s.Insert(ctx, dup); !errors.Is(err, store.ErrConflict) {
		t.Fatalf("got %v, want ErrConflict", err)
	}
}

func insertWithLinks(t *testing.T, s store.Store) {
	a := mk("evt_00000000000A", "problem.reported", "alpha", "2026-03-10T09:00:00Z", "a")
	b := mk("evt_00000000000B", "proposal.recorded", "alpha", "2026-03-10T10:00:00Z", "b",
		ref("motivated_by", a.ID, "confirmed"))
	c := mk("evt_00000000000C", "incident.opened", "alpha", "2026-03-10T11:00:00Z", "c",
		ref("caused_by", b.ID, "hypothesis"))
	insert(t, s, a, b, c)

	links, err := s.Links(ctx, b.ID)
	if err != nil {
		t.Fatal(err)
	}
	want := []string{
		"evt_00000000000B|evt_00000000000A|motivated_by|confirmed",
		"evt_00000000000C|evt_00000000000B|caused_by|hypothesis",
	}
	if !slices.Equal(linkKeys(links), want) {
		t.Fatalf("Links(b) = %v, want %v", linkKeys(links), want)
	}

	got, err := s.Get(ctx, b.ID)
	if err != nil {
		t.Fatal(err)
	}
	if !reflect.DeepEqual(got.Links, []event.LinkRef{ref("motivated_by", a.ID, "confirmed")}) {
		t.Fatalf("Get(b).Links = %+v", got.Links)
	}
}

func insertDanglingLink(t *testing.T, s store.Store) {
	e := mk("evt_00000000000B", "proposal.recorded", "alpha", "2026-03-10T10:00:00Z", "b",
		ref("motivated_by", "evt_DOESNOTEXIST", "confirmed"))
	if err := s.Insert(ctx, e); !errors.Is(err, store.ErrNotFound) {
		t.Fatalf("got %v, want ErrNotFound", err)
	}
	if _, err := s.Get(ctx, e.ID); !errors.Is(err, store.ErrNotFound) {
		t.Fatalf("a rejected insert must store nothing; Get returned %v", err)
	}
}

func addLink(t *testing.T, s store.Store) {
	a := mk("evt_00000000000A", "incident.opened", "alpha", "2026-03-10T09:00:00Z", "a")
	b := mk("evt_00000000000B", "investigation.completed", "alpha", "2026-03-10T10:00:00Z", "b")
	insert(t, s, a, b)

	hyp := event.Link{Source: b.ID, Target: a.ID, Rel: "caused_by", Basis: "hypothesis", Rationale: "maybe"}
	if err := s.AddLink(ctx, hyp); err != nil {
		t.Fatal(err)
	}
	if err := s.AddLink(ctx, hyp); err != nil { // identical: no-op
		t.Fatalf("duplicate AddLink: %v", err)
	}
	conf := event.Link{Source: b.ID, Target: a.ID, Rel: "caused_by", Basis: "confirmed"}
	if err := s.AddLink(ctx, conf); err != nil { // same triple, other basis: second row
		t.Fatal(err)
	}

	expectBothLinks(t, s, a.ID)

	got, err := s.Get(ctx, b.ID)
	if err != nil {
		t.Fatal(err)
	}
	if len(got.Links) != 2 {
		t.Fatalf("Get(b).Links = %+v, want both added links merged in", got.Links)
	}

	missing := event.Link{Source: b.ID, Target: "evt_DOESNOTEXIST", Rel: "relates_to", Basis: "confirmed"}
	if err := s.AddLink(ctx, missing); !errors.Is(err, store.ErrNotFound) {
		t.Fatalf("got %v, want ErrNotFound", err)
	}
}

// expectBothLinks checks that the two links added in addLink are stored on
// the target event, with their rationale.
func expectBothLinks(t *testing.T, s store.Store, id string) {
	t.Helper()
	links, err := s.Links(ctx, id)
	if err != nil {
		t.Fatal(err)
	}
	want := []string{
		"evt_00000000000B|evt_00000000000A|caused_by|confirmed",
		"evt_00000000000B|evt_00000000000A|caused_by|hypothesis",
	}
	if !slices.Equal(linkKeys(links), want) {
		t.Fatalf("Links(a) = %v, want %v", linkKeys(links), want)
	}
	for _, l := range links {
		if l.Basis == "hypothesis" && l.Rationale != "maybe" {
			t.Errorf("rationale lost: %+v", l)
		}
	}
}

func searchFilters(t *testing.T, s store.Store) {
	e1 := mk("evt_00000000000A", "problem.reported", "alpha", "2026-03-10T09:00:00Z", "flat signups")
	e2 := mk("evt_00000000000B", "proposal.recorded", "alpha", "2026-03-10T11:30:00Z", "spin proposal")
	e3 := mk("evt_00000000000C", "incident.opened", "beta", "2026-03-12T13:38:00Z", "canary errors")
	insert(t, s, e1, e2, e3)

	expectIDs(t, "default newest first", search(t, s, store.Filter{}), e3.ID, e2.ID, e1.ID)
	expectIDs(t, "ascending", search(t, s, store.Filter{Ascending: true}), e1.ID, e2.ID, e3.ID)
	expectIDs(t, "system", search(t, s, store.Filter{System: "alpha"}), e2.ID, e1.ID)
	expectIDs(t, "types", search(t, s, store.Filter{Types: []string{"incident.opened", "problem.reported"}}), e3.ID, e1.ID)
	since := time.Date(2026, 3, 10, 10, 0, 0, 0, time.UTC)
	expectIDs(t, "since", search(t, s, store.Filter{Since: since}), e3.ID, e2.ID)
	until := time.Date(2026, 3, 10, 11, 30, 0, 0, time.UTC)
	expectIDs(t, "until is inclusive", search(t, s, store.Filter{Until: until}), e2.ID, e1.ID)
	expectIDs(t, "limit", search(t, s, store.Filter{Limit: 2}), e3.ID, e2.ID)
	expectIDs(t, "text", search(t, s, store.Filter{Text: "spin"}), e2.ID)
	expectIDs(t, "text is case-insensitive and ANDed", search(t, s, store.Filter{Text: "SPIN proposal"}), e2.ID)
	expectIDs(t, "text prefix", search(t, s, store.Filter{Text: "sp"}), e2.ID)
	expectIDs(t, "text with system filter", search(t, s, store.Filter{Text: "canary", System: "beta"}), e3.ID)
	expectIDs(t, "text matches the type", search(t, s, store.Filter{Text: "incident"}), e3.ID)
	expectIDs(t, "no match", search(t, s, store.Filter{Text: "nomatch"}))
}

func searchOffset(t *testing.T, s store.Store) {
	e1 := mk("evt_00000000000A", "problem.reported", "alpha", "2026-03-10T09:00:00Z", "one")
	e2 := mk("evt_00000000000B", "problem.reported", "alpha", "2026-03-10T10:00:00Z", "two")
	e3 := mk("evt_00000000000C", "problem.reported", "alpha", "2026-03-10T11:00:00Z", "three")
	insert(t, s, e1, e2, e3)

	expectIDs(t, "skip the newest", search(t, s, store.Filter{Offset: 1}), e2.ID, e1.ID)
	expectIDs(t, "offset with limit", search(t, s, store.Filter{Offset: 1, Limit: 1}), e2.ID)
	expectIDs(t, "offset ascending", search(t, s, store.Filter{Offset: 2, Ascending: true}), e3.ID)
	expectIDs(t, "offset past the end", search(t, s, store.Filter{Offset: 5}))
	expectIDs(t, "negative offset is zero", search(t, s, store.Filter{Offset: -3, Limit: 1}), e3.ID)
}

func searchTextPunctuation(t *testing.T, s store.Store) {
	e := mk("evt_00000000000A", "incident.opened", "promo-api", "2026-03-12T13:38:00Z", `5xx ratio "high"`)
	e.Subject.Component = "spin-service"
	insert(t, s, e)

	expectIDs(t, "hyphenated system", search(t, s, store.Filter{Text: "promo-api"}), e.ID)
	expectIDs(t, "hyphenated component", search(t, s, store.Filter{Text: "spin-service"}), e.ID)
	expectIDs(t, "leading digit token", search(t, s, store.Filter{Text: "5xx"}), e.ID)

	// None of these may be an error; what they match is not specified.
	for _, q := range []string{`"quoted`, `AND`, `OR`, `NOT`, `spin*`, `(`, `)`, `-`, `:`, `'`,
		`NEAR(a b)`, `col:val`, `a OR b`, `"`, `""`, `^x`, `x"y`, `{`, `}`} {
		if _, err := s.Search(ctx, store.Filter{Text: q}); err != nil {
			t.Errorf("Search(%q) returned an error: %v", q, err)
		}
	}
}

func systems(t *testing.T, s store.Store) {
	got, err := s.Systems(ctx)
	if err != nil || len(got) != 0 {
		t.Fatalf("empty store: Systems = %v, %v", got, err)
	}
	insert(t, s,
		mk("evt_00000000000A", "problem.reported", "beta", "2026-03-10T09:00:00Z", "x"),
		mk("evt_00000000000B", "problem.reported", "alpha", "2026-03-10T10:00:00Z", "y"),
		mk("evt_00000000000C", "problem.reported", "beta", "2026-03-10T11:00:00Z", "z"),
	)
	got, err = s.Systems(ctx)
	if err != nil {
		t.Fatal(err)
	}
	if !slices.Equal(got, []string{"alpha", "beta"}) {
		t.Fatalf("Systems = %v", got)
	}
}

func timeOffsets(t *testing.T, s store.Store) {
	// Instants: x = 10:00Z, y = 10:30Z, z = 11:00Z. As text, x ("12:00") sorts last.
	x := mk("evt_00000000000X", "problem.reported", "alpha", "2026-03-11T12:00:00+02:00", "x")
	y := mk("evt_00000000000Y", "problem.reported", "alpha", "2026-03-11T10:30:00Z", "y")
	z := mk("evt_00000000000Z", "problem.reported", "alpha", "2026-03-11T09:00:00-02:00", "z")
	insert(t, s, z, x, y)

	expectIDs(t, "ordered by instant", search(t, s, store.Filter{Ascending: true}), x.ID, y.ID, z.ID)
	since := time.Date(2026, 3, 11, 10, 15, 0, 0, time.UTC)
	expectIDs(t, "since by instant", search(t, s, store.Filter{Since: since, Ascending: true}), y.ID, z.ID)
	until := time.Date(2026, 3, 11, 10, 30, 0, 0, time.UTC)
	expectIDs(t, "until by instant", search(t, s, store.Filter{Until: until, Ascending: true}), x.ID, y.ID)
}

func concurrentInserts(t *testing.T, s store.Store) {
	const n = 20
	var wg sync.WaitGroup
	errs := make(chan error, n)
	for i := 0; i < n; i++ {
		wg.Add(1)
		go func(i int) {
			defer wg.Done()
			e := mk(fmt.Sprintf("evt_C%011d", i), "problem.reported", "alpha", "2026-03-10T09:00:00Z", "c")
			errs <- s.Insert(ctx, e)
		}(i)
	}
	wg.Wait()
	close(errs)
	for err := range errs {
		if err != nil {
			t.Errorf("concurrent insert failed: %v", err)
		}
	}
	if got := search(t, s, store.Filter{Limit: 100}); len(got) != n {
		t.Fatalf("stored %d events, want %d", len(got), n)
	}
}

func insertDuplicateKey(t *testing.T, s store.Store) {
	a := mk("evt_00000000000A", "problem.reported", "alpha", "2026-03-10T09:00:00Z", "first")
	a.IdempotencyKey = "deploy-4411"
	insert(t, s, a)
	b := mk("evt_00000000000B", "problem.reported", "alpha", "2026-03-10T10:00:00Z", "retry")
	b.IdempotencyKey = "deploy-4411"
	err := s.Insert(ctx, b)
	if !errors.Is(err, store.ErrDuplicate) {
		t.Fatalf("got %v, want ErrDuplicate", err)
	}
	if errors.Is(err, store.ErrConflict) {
		t.Fatal("a key duplicate must not look like an id conflict: the service retries those")
	}
	var dup *store.DuplicateError
	if !errors.As(err, &dup) || dup.ExistingID != a.ID {
		t.Fatalf("got %#v, want DuplicateError{ExistingID: %s}", err, a.ID)
	}
	if _, err := s.Get(ctx, b.ID); !errors.Is(err, store.ErrNotFound) {
		t.Fatalf("the rejected event must not be stored, Get = %v", err)
	}
}

func keyScopeIsPerActor(t *testing.T, s store.Store) {
	a := mk("evt_00000000000A", "problem.reported", "alpha", "2026-03-10T09:00:00Z", "a")
	a.IdempotencyKey = "shared"
	b := mk("evt_00000000000B", "problem.reported", "alpha", "2026-03-10T10:00:00Z", "b")
	b.IdempotencyKey = "shared"
	b.Actor = event.Actor{Kind: "human", ID: "user:b"} // another actor: same key is fine
	c := mk("evt_00000000000C", "problem.reported", "alpha", "2026-03-10T11:00:00Z", "c")
	c.IdempotencyKey = "different" // same actor, another key: fine
	insert(t, s, a, b, c)
}

func keysAreOpaque(t *testing.T, s store.Store) {
	for i, key := range []string{"k", "K", "k ", " k"} {
		e := mk(fmt.Sprintf("evt_O%011d", i), "problem.reported", "alpha", "2026-03-10T09:00:00Z", "o")
		e.IdempotencyKey = key
		insert(t, s, e) // none of these may collide with another
	}
}

func duplicateLeavesNoTrace(t *testing.T, s store.Store) {
	a := mk("evt_00000000000A", "problem.reported", "alpha", "2026-03-10T09:00:00Z", "first")
	a.IdempotencyKey = "k"
	insert(t, s, a)
	b := mk("evt_00000000000B", "proposal.recorded", "alpha", "2026-03-10T10:00:00Z", "zebracorn",
		ref("motivated_by", a.ID, "confirmed"))
	b.IdempotencyKey = "k"
	if err := s.Insert(ctx, b); !errors.Is(err, store.ErrDuplicate) {
		t.Fatalf("got %v, want ErrDuplicate", err)
	}
	expectIDs(t, "search finds nothing for the rejected event", search(t, s, store.Filter{Text: "zebracorn"}))
	links, err := s.Links(ctx, a.ID)
	if err != nil || len(links) != 0 {
		t.Fatalf("links of the first event = %v, %v; want none", links, err)
	}
}

func concurrentSameKey(t *testing.T, s store.Store) {
	const n = 20
	var wg sync.WaitGroup
	errs := make(chan error, n)
	for i := 0; i < n; i++ {
		wg.Add(1)
		go func(i int) {
			defer wg.Done()
			e := mk(fmt.Sprintf("evt_K%011d", i), "problem.reported", "alpha", "2026-03-10T09:00:00Z", "k")
			e.IdempotencyKey = "same-for-all"
			errs <- s.Insert(ctx, e)
		}(i)
	}
	wg.Wait()
	close(errs)
	var stored, dups int
	for err := range errs {
		switch {
		case err == nil:
			stored++
		case errors.Is(err, store.ErrDuplicate):
			dups++
		default:
			t.Errorf("unexpected error: %v", err)
		}
	}
	if stored != 1 || dups != n-1 {
		t.Fatalf("stored %d, duplicates %d; want 1 and %d", stored, dups, n-1)
	}
	if got := search(t, s, store.Filter{Limit: 100}); len(got) != 1 {
		t.Fatalf("%d events in the store, want 1", len(got))
	}
}
