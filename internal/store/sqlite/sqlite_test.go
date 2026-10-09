package sqlite_test

import (
	"fmt"
	"path/filepath"
	"reflect"
	"sync"
	"testing"

	"github.com/regeda/techlog/internal/event"
	"github.com/regeda/techlog/internal/store"
	"github.com/regeda/techlog/internal/store/sqlite"
	"github.com/regeda/techlog/internal/store/storetest"
)

func TestContract(t *testing.T) {
	storetest.Run(t, func(t *testing.T) store.Store {
		s, err := sqlite.OpenMemory()
		if err != nil {
			t.Fatal(err)
		}
		t.Cleanup(func() { s.Close() })
		return s
	})
}

func TestOpenCreatesDirectoryAndPersists(t *testing.T) {
	path := filepath.Join(t.TempDir(), "share", "techlog.db")
	s, err := sqlite.Open(path)
	if err != nil {
		t.Fatal(err)
	}
	e := event.Event{
		SchemaVersion: event.SchemaVersion, ID: "evt_00000000000A", Type: "problem.reported",
		Time: "2026-03-10T09:00:00Z", Actor: event.Actor{Kind: "human", ID: "user:a"},
		Subject: event.Subject{System: "alpha"}, Status: "recorded",
		Provenance: event.Provenance{RecordedBy: "user:a", Method: "human_entered"},
	}
	if err := s.Insert(t.Context(), e); err != nil {
		t.Fatal(err)
	}
	if err := s.Close(); err != nil {
		t.Fatal(err)
	}

	// Reopening runs the migrations again; they must be idempotent.
	s2, err := sqlite.Open(path)
	if err != nil {
		t.Fatal(err)
	}
	defer s2.Close()
	got, err := s2.Get(t.Context(), e.ID)
	if err != nil {
		t.Fatal(err)
	}
	if !reflect.DeepEqual(got, e) {
		t.Fatalf("got %+v, want %+v", got, e)
	}
}

// Two handles on one file stand in for `techlog serve` plus `techlog seed`
// (or two servers). A write transaction that reads before it writes must wait
// for the lock instead of failing with SQLITE_BUSY.
func TestTwoHandlesOnOneFileDoNotLockEachOther(t *testing.T) {
	path := filepath.Join(t.TempDir(), "techlog.db")
	a, err := sqlite.Open(path)
	if err != nil {
		t.Fatal(err)
	}
	defer a.Close()
	b, err := sqlite.Open(path)
	if err != nil {
		t.Fatal(err)
	}
	defer b.Close()

	mk := func(id string, links ...event.LinkRef) event.Event {
		return event.Event{
			SchemaVersion: event.SchemaVersion, ID: id, Type: "problem.reported",
			Time: "2026-03-10T09:00:00Z", Actor: event.Actor{Kind: "human", ID: "user:a"},
			Subject: event.Subject{System: "alpha"}, Status: "recorded", Links: links,
			Provenance: event.Provenance{RecordedBy: "user:a", Method: "human_entered"},
		}
	}
	if err := a.Insert(t.Context(), mk("evt_BASE00000000")); err != nil {
		t.Fatal(err)
	}

	const perHandle = 40
	errs := make(chan error, 2*perHandle)
	var wg sync.WaitGroup
	for h, s := range []*sqlite.Store{a, b} {
		wg.Add(1)
		go func() {
			defer wg.Done()
			for i := 0; i < perHandle; i++ {
				e := mk(fmt.Sprintf("evt_H%dN%08d", h, i), event.LinkRef{Rel: "relates_to", Target: "evt_BASE00000000", Basis: "confirmed"})
				errs <- s.Insert(t.Context(), e)
			}
		}()
	}
	wg.Wait()
	close(errs)
	for err := range errs {
		if err != nil {
			t.Errorf("insert failed: %v", err)
		}
	}
}
