package sqlite_test

import (
	"path/filepath"
	"reflect"
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
