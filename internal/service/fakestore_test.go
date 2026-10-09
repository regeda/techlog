package service_test

import (
	"context"
	"slices"
	"sort"

	"github.com/regeda/techlog/internal/event"
	"github.com/regeda/techlog/internal/store"
)

// fakeStore is a minimal in-memory store.Store for service tests. It also
// records what the service asked for.
type fakeStore struct {
	events          map[string]event.Event
	links           []event.Link
	insertConflicts int // fail this many Inserts with ErrConflict
	inserts         int
	lastFilter      store.Filter
}

func newFakeStore() *fakeStore { return &fakeStore{events: map[string]event.Event{}} }

func (f *fakeStore) merged(e event.Event) event.Event {
	e.Links = nil
	for _, l := range f.links {
		if l.Source == e.ID {
			e.Links = append(e.Links, event.LinkRef{Rel: l.Rel, Target: l.Target, Basis: l.Basis})
		}
	}
	return e
}

func (f *fakeStore) Insert(_ context.Context, e event.Event) error {
	f.inserts++
	if f.insertConflicts > 0 {
		f.insertConflicts--
		return store.ErrConflict
	}
	if _, ok := f.events[e.ID]; ok {
		return store.ErrConflict
	}
	for _, l := range e.Links {
		if _, ok := f.events[l.Target]; !ok {
			return store.ErrNotFound
		}
	}
	f.events[e.ID] = e
	for _, l := range e.Links {
		f.links = append(f.links, event.Link{Source: e.ID, Target: l.Target, Rel: l.Rel, Basis: l.Basis})
	}
	return nil
}

func (f *fakeStore) AddLink(_ context.Context, l event.Link) error {
	_, okSource := f.events[l.Source]
	_, okTarget := f.events[l.Target]
	if !okSource || !okTarget {
		return store.ErrNotFound
	}
	f.links = append(f.links, l)
	return nil
}

func (f *fakeStore) Get(_ context.Context, id string) (event.Event, error) {
	e, ok := f.events[id]
	if !ok {
		return event.Event{}, store.ErrNotFound
	}
	return f.merged(e), nil
}

func (f *fakeStore) Links(_ context.Context, id string) ([]event.Link, error) {
	var out []event.Link
	for _, l := range f.links {
		if l.Source == id || l.Target == id {
			out = append(out, l)
		}
	}
	return out, nil
}

func (f *fakeStore) Search(_ context.Context, flt store.Filter) ([]event.Event, error) {
	f.lastFilter = flt
	var out []event.Event
	for _, e := range f.events {
		if flt.System != "" && e.Subject.System != flt.System {
			continue
		}
		if len(flt.Types) > 0 && !slices.Contains(flt.Types, e.Type) {
			continue
		}
		t, _ := e.ParsedTime()
		if !flt.Since.IsZero() && t.Before(flt.Since) {
			continue
		}
		if !flt.Until.IsZero() && t.After(flt.Until) {
			continue
		}
		out = append(out, f.merged(e))
	}
	sort.Slice(out, func(i, j int) bool {
		ti, _ := out[i].ParsedTime()
		tj, _ := out[j].ParsedTime()
		if !ti.Equal(tj) {
			return ti.Before(tj) == flt.Ascending
		}
		return (out[i].ID < out[j].ID) == flt.Ascending
	})
	if flt.Limit > 0 && len(out) > flt.Limit {
		out = out[:flt.Limit]
	}
	return out, nil
}

func (f *fakeStore) Systems(_ context.Context) ([]string, error) {
	set := map[string]bool{}
	for _, e := range f.events {
		set[e.Subject.System] = true
	}
	out := make([]string, 0, len(set))
	for s := range set {
		out = append(out, s)
	}
	sort.Strings(out)
	return out, nil
}

func (f *fakeStore) Close() error { return nil }
