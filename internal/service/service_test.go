package service_test

import (
	"context"
	"errors"
	"regexp"
	"slices"
	"testing"
	"time"

	"github.com/regeda/techlog/internal/event"
	"github.com/regeda/techlog/internal/service"
	"github.com/regeda/techlog/internal/store"
	"github.com/regeda/techlog/internal/testutil"
)

var (
	ctx      = context.Background()
	fixedNow = time.Date(2026, 3, 12, 10, 0, 0, 0, time.UTC)
	idRe     = regexp.MustCompile(`^evt_[0-9A-Z]{12}$`)
)

func newService(t *testing.T) (*service.Service, *fakeStore) {
	t.Helper()
	fs := newFakeStore()
	svc, err := service.New(fs, service.WithClock(func() time.Time { return fixedNow }))
	if err != nil {
		t.Fatal(err)
	}
	return svc, fs
}

func record(t *testing.T, svc *service.Service, doc map[string]any) event.Event {
	t.Helper()
	ev, err := svc.Record(ctx, doc, service.MethodHuman)
	if err != nil {
		t.Fatalf("record: %v", err)
	}
	return ev
}

func problemPaths(err error) []string {
	var inv *service.InvalidError
	if !errors.As(err, &inv) {
		return nil
	}
	paths := make([]string, len(inv.Problems))
	for i, p := range inv.Problems {
		paths[i] = p.Path
	}
	return paths
}

func TestRecordAssignsIDAndDefaults(t *testing.T) {
	svc, _ := newService(t)
	d := testutil.Doc("alpha", "problem.reported", "hello")
	d["id"] = "evt_AAAAAAAAAAAA" // must be ignored
	delete(d, "time")

	ev, err := svc.Record(ctx, d, service.MethodAgent)
	if err != nil {
		t.Fatal(err)
	}
	if ev.ID == "evt_AAAAAAAAAAAA" || !idRe.MatchString(ev.ID) {
		t.Errorf("id = %q, want a freshly assigned evt_ id", ev.ID)
	}
	if ev.SchemaVersion != event.SchemaVersion {
		t.Errorf("schema_version = %q", ev.SchemaVersion)
	}
	if ev.Time != "2026-03-12T10:00:00Z" {
		t.Errorf("time = %q, want the clock's time", ev.Time)
	}
	if ev.Provenance.Method != "agent_submitted" || ev.Provenance.RecordedBy != "user:test" {
		t.Errorf("provenance = %+v", ev.Provenance)
	}
	if d["id"] != "evt_AAAAAAAAAAAA" || d["provenance"] != nil {
		t.Error("Record must not modify the caller's document")
	}
}

func TestRecordKeepsExplicitProvenance(t *testing.T) {
	svc, _ := newService(t)
	d := testutil.Doc("alpha", "release.deployed", "v1")
	d["provenance"] = map[string]any{"recorded_by": "collector:git", "method": "collected"}
	ev := record(t, svc, d)
	if ev.Provenance.RecordedBy != "collector:git" || ev.Provenance.Method != "collected" {
		t.Errorf("provenance = %+v", ev.Provenance)
	}
}

func TestRecordRejectsInvalid(t *testing.T) {
	svc, fs := newService(t)
	d := testutil.Doc("alpha", "problem.reported", "hello")
	d["actor"] = map[string]any{"kind": "robot", "id": "x"}
	_, err := svc.Record(ctx, d, service.MethodHuman)
	if !slices.Contains(problemPaths(err), "/actor/kind") {
		t.Fatalf("want an InvalidError with /actor/kind, got %v", err)
	}
	if fs.inserts != 0 {
		t.Error("an invalid event must not reach the store")
	}
}

func TestRecordLinkRules(t *testing.T) {
	svc, fs := newService(t)
	target := record(t, svc, testutil.Doc("alpha", "problem.reported", "target"))
	before := fs.inserts

	t.Run("dangling target", func(t *testing.T) {
		d := testutil.Doc("alpha", "proposal.recorded", "p", testutil.Link("motivated_by", "evt_000000000000"))
		_, err := svc.Record(ctx, d, service.MethodHuman)
		if !slices.Contains(problemPaths(err), "/links/0/target") {
			t.Fatalf("got %v", err)
		}
	})
	t.Run("basis defaults to confirmed", func(t *testing.T) {
		d := testutil.Doc("alpha", "proposal.recorded", "p", map[string]any{"rel": "motivated_by", "target": target.ID})
		ev := record(t, svc, d)
		if len(ev.Links) != 1 || ev.Links[0].Basis != "confirmed" {
			t.Fatalf("links = %+v", ev.Links)
		}
	})
	t.Run("caused_by needs an explicit basis", func(t *testing.T) {
		d := testutil.Doc("alpha", "incident.opened", "i", map[string]any{"rel": "caused_by", "target": target.ID})
		_, err := svc.Record(ctx, d, service.MethodHuman)
		if !slices.Contains(problemPaths(err), "/links/0") {
			t.Fatalf("got %v (paths %v)", err, problemPaths(err))
		}
	})
	t.Run("explicit hypothesis is kept", func(t *testing.T) {
		d := testutil.Doc("alpha", "incident.opened", "i", testutil.Link("caused_by", target.ID, "hypothesis"))
		ev := record(t, svc, d)
		if ev.Links[0].Basis != "hypothesis" {
			t.Fatalf("links = %+v", ev.Links)
		}
	})
	if fs.inserts != before+2 {
		t.Errorf("only the two valid events should have been inserted, got %d inserts", fs.inserts-before)
	}
}

func TestRecordRetriesOnIDConflict(t *testing.T) {
	svc, fs := newService(t)
	fs.insertConflicts = 2
	record(t, svc, testutil.Doc("alpha", "problem.reported", "hello"))
	if fs.inserts != 3 {
		t.Errorf("inserts = %d, want 3 (two conflicts then success)", fs.inserts)
	}

	fs.inserts = 0
	fs.insertConflicts = 100
	_, err := svc.Record(ctx, testutil.Doc("alpha", "problem.reported", "again"), service.MethodHuman)
	if !errors.Is(err, store.ErrConflict) {
		t.Fatalf("got %v, want ErrConflict after giving up", err)
	}
	if fs.inserts != 5 {
		t.Errorf("inserts = %d, want 5 attempts", fs.inserts)
	}
}

func TestLinkRules(t *testing.T) {
	svc, _ := newService(t)
	a := record(t, svc, testutil.Doc("alpha", "incident.opened", "a"))
	b := record(t, svc, testutil.Doc("alpha", "investigation.completed", "b"))

	cases := []struct {
		name     string
		in       service.LinkInput
		wantPath string // "" means success
		wantBase string
	}{
		{"basis defaults to confirmed", service.LinkInput{Source: b.ID, Target: a.ID, Rel: "relates_to"}, "", "confirmed"},
		{"hypothesis with rationale", service.LinkInput{Source: b.ID, Target: a.ID, Rel: "caused_by", Basis: "hypothesis", Rationale: "maybe"}, "", "hypothesis"},
		{"identical link again is fine", service.LinkInput{Source: b.ID, Target: a.ID, Rel: "relates_to"}, "", "confirmed"},
		{"caused_by without basis", service.LinkInput{Source: b.ID, Target: a.ID, Rel: "caused_by"}, "/", ""},
		{"self link", service.LinkInput{Source: a.ID, Target: a.ID, Rel: "relates_to"}, "/target", ""},
		{"bad rel", service.LinkInput{Source: b.ID, Target: a.ID, Rel: "nope"}, "/rel", ""},
		{"missing target", service.LinkInput{Source: b.ID, Target: "evt_000000000000", Rel: "relates_to"}, "/target", ""},
		{"missing source", service.LinkInput{Source: "evt_000000000000", Target: a.ID, Rel: "relates_to"}, "/source", ""},
		{"empty source", service.LinkInput{Target: a.ID, Rel: "relates_to"}, "/source", ""},
	}
	for _, tc := range cases {
		t.Run(tc.name, func(t *testing.T) {
			l, err := svc.Link(ctx, tc.in)
			if tc.wantPath == "" {
				if err != nil {
					t.Fatal(err)
				}
				if l.Basis != tc.wantBase {
					t.Errorf("basis = %q, want %q", l.Basis, tc.wantBase)
				}
				return
			}
			if !slices.Contains(problemPaths(err), tc.wantPath) {
				t.Fatalf("want a problem at %q, got %v (%v)", tc.wantPath, problemPaths(err), err)
			}
		})
	}
}

func TestGetDetail(t *testing.T) {
	svc, _ := newService(t)
	a := record(t, svc, testutil.Doc("alpha", "problem.reported", "a"))
	b := record(t, svc, testutil.Doc("alpha", "proposal.recorded", "b", testutil.Link("motivated_by", a.ID)))

	da, err := svc.Get(ctx, a.ID)
	if err != nil {
		t.Fatal(err)
	}
	if len(da.Incoming) != 1 || da.Incoming[0].Source != b.ID || len(da.Outgoing) != 0 {
		t.Errorf("a: incoming=%+v outgoing=%+v", da.Incoming, da.Outgoing)
	}
	db, err := svc.Get(ctx, b.ID)
	if err != nil {
		t.Fatal(err)
	}
	if len(db.Outgoing) != 1 || db.Outgoing[0].Target != a.ID || db.Incoming == nil {
		t.Errorf("b: incoming=%+v outgoing=%+v", db.Incoming, db.Outgoing)
	}
	if _, err := svc.Get(ctx, "evt_000000000000"); !errors.Is(err, store.ErrNotFound) {
		t.Errorf("got %v, want ErrNotFound", err)
	}
}

func importDoc(id, ts string, links ...map[string]any) map[string]any {
	d := testutil.Doc("alpha", "problem.reported", "imported "+id, links...)
	d["id"] = id
	d["time"] = ts
	d["schema_version"] = event.SchemaVersion
	d["provenance"] = map[string]any{"recorded_by": "seed", "method": "collected"}
	return d
}

func TestImportKeepsIDsAndSkipsExisting(t *testing.T) {
	svc, _ := newService(t)
	docs := []map[string]any{
		importDoc("evt_00000000000A", "2026-03-10T09:00:00Z"),
		importDoc("evt_00000000000B", "2026-03-10T10:00:00Z", testutil.Link("relates_to", "evt_00000000000A")),
	}
	inserted, skipped, err := svc.Import(ctx, docs)
	if err != nil || inserted != 2 || skipped != 0 {
		t.Fatalf("first import = %d, %d, %v", inserted, skipped, err)
	}
	inserted, skipped, err = svc.Import(ctx, docs)
	if err != nil || inserted != 0 || skipped != 2 {
		t.Fatalf("second import = %d, %d, %v", inserted, skipped, err)
	}
	d, err := svc.Get(ctx, "evt_00000000000B")
	if err != nil || d.Event.ID != "evt_00000000000B" {
		t.Fatalf("Get = %+v, %v", d, err)
	}

	bad := []map[string]any{importDoc("evt_00000000000C", "2026-03-10T11:00:00Z", testutil.Link("relates_to", "evt_0000000000ZZ"))}
	if _, _, err := svc.Import(ctx, bad); !slices.Contains(problemPaths(err), "/links/0/target") {
		t.Fatalf("got %v", err)
	}
}
