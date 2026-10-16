package service_test

import (
	"context"
	"errors"
	"regexp"
	"slices"
	"strings"
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

func TestImportSkipsDuplicateKey(t *testing.T) {
	svc, _ := newService(t)
	a := importDoc("evt_00000000000A", "2026-03-10T09:00:00Z")
	b := importDoc("evt_00000000000B", "2026-03-10T10:00:00Z")
	a["idempotency_key"], b["idempotency_key"] = "same", "same"
	inserted, skipped, err := svc.Import(ctx, []map[string]any{a, b})
	if err != nil || inserted != 1 || skipped != 1 {
		t.Fatalf("import = %d, %d, %v; want 1, 1, nil", inserted, skipped, err)
	}
}

func TestRecordRejectsDuplicateKey(t *testing.T) {
	svc, fs := newService(t)
	d := testutil.Doc("alpha", "problem.reported", "first")
	d["idempotency_key"] = "deploy-4411"
	first := record(t, svc, d)
	before := fs.inserts

	_, err := svc.Record(ctx, d, service.MethodHuman)
	var dup *service.DuplicateError
	if !errors.As(err, &dup) {
		t.Fatalf("got %v, want a DuplicateError", err)
	}
	if dup.ExistingID != first.ID || dup.Key != "deploy-4411" || dup.ActorID != "user:test" {
		t.Errorf("DuplicateError = %+v", dup)
	}
	if want := `duplicate: idempotency_key "deploy-4411" for actor "user:test" was already recorded as ` + first.ID; dup.Error() != want {
		t.Errorf("message = %q, want %q", dup.Error(), want)
	}
	if fs.inserts != before+1 {
		t.Errorf("a duplicate must be tried once, not retried: %d inserts", fs.inserts-before)
	}
	if len(fs.events) != 1 {
		t.Errorf("%d events stored, want 1", len(fs.events))
	}
}

func TestRecordSameKeyDifferentActor(t *testing.T) {
	svc, _ := newService(t)
	a := testutil.Doc("alpha", "problem.reported", "a")
	a["idempotency_key"] = "shared"
	b := testutil.Doc("alpha", "problem.reported", "b")
	b["idempotency_key"] = "shared"
	b["actor"] = map[string]any{"kind": "human", "id": "user:other"}
	record(t, svc, a)
	record(t, svc, b)
}

func TestRecordRequiresKey(t *testing.T) {
	svc, fs := newService(t)
	for name, tc := range map[string]struct {
		mutate func(d map[string]any)
		path   string
	}{
		"missing": {func(d map[string]any) { delete(d, "idempotency_key") }, "/"},
		"empty":   {func(d map[string]any) { d["idempotency_key"] = "" }, "/idempotency_key"},
		"long":    {func(d map[string]any) { d["idempotency_key"] = strings.Repeat("k", 129) }, "/idempotency_key"},
	} {
		t.Run(name, func(t *testing.T) {
			d := testutil.Doc("alpha", "problem.reported", "x")
			tc.mutate(d)
			_, err := svc.Record(ctx, d, service.MethodHuman)
			if !slices.Contains(problemPaths(err), tc.path) {
				t.Fatalf("got %v (paths %v), want a problem at %q", err, problemPaths(err), tc.path)
			}
		})
	}
	if fs.inserts != 0 {
		t.Error("an event without a valid key must not reach the store")
	}
}

func TestRecordValidationBeatsDuplicate(t *testing.T) {
	svc, _ := newService(t)
	d := testutil.Doc("alpha", "problem.reported", "first")
	d["idempotency_key"] = "k"
	record(t, svc, d)
	d["actor"].(map[string]any)["kind"] = "robot" // now invalid AND a duplicate
	_, err := svc.Record(ctx, d, service.MethodHuman)
	if !slices.Contains(problemPaths(err), "/actor/kind") {
		t.Fatalf("got %v, want a validation error, not a duplicate", err)
	}
}

func TestRecordRejectsFutureTime(t *testing.T) {
	cases := []struct {
		name, time string
		ok         bool
	}{
		{"at the skew boundary", "2026-03-12T10:05:00Z", true},
		{"offset text reads later but is past", "2026-03-12T11:30:00+02:00", true},
		{"one second over the skew", "2026-03-12T10:05:01Z", false},
		{"offset instant is in the future", "2026-03-12T12:30:00+02:00", false},
	}
	for _, c := range cases {
		t.Run(c.name, func(t *testing.T) {
			svc, fs := newService(t)
			d := testutil.Doc("alpha", "problem.reported", "p")
			d["time"] = c.time
			_, err := svc.Record(ctx, d, service.MethodHuman)
			if c.ok {
				if err != nil {
					t.Fatalf("want accepted, got %v", err)
				}
				return
			}
			if !slices.Contains(problemPaths(err), "/time") {
				t.Fatalf("want an InvalidError with /time, got %v", err)
			}
			if fs.inserts != 0 {
				t.Error("a rejected event must not reach the store")
			}
		})
	}
}

func TestRecordStoresTimeAsUTC(t *testing.T) {
	cases := []struct{ in, want string }{
		{"2026-03-11T12:00:00+02:00", "2026-03-11T10:00:00Z"},
		{"2026-03-11T10:00:00.250+00:00", "2026-03-11T10:00:00.25Z"},
	}
	for _, c := range cases {
		svc, _ := newService(t)
		d := testutil.Doc("alpha", "problem.reported", "p")
		d["time"] = c.in
		ev := record(t, svc, d)
		if ev.Time != c.want {
			t.Errorf("time %q stored as %q, want %q", c.in, ev.Time, c.want)
		}
	}
}

func TestClockSkewOption(t *testing.T) {
	clock := service.WithClock(func() time.Time { return fixedNow })
	try := func(t *testing.T, skew time.Duration, at time.Time) error {
		t.Helper()
		svc, err := service.New(newFakeStore(), clock, service.WithClockSkew(skew))
		if err != nil {
			t.Fatal(err)
		}
		d := testutil.Doc("alpha", "problem.reported", "p")
		d["time"] = at.Format(time.RFC3339)
		_, err = svc.Record(ctx, d, service.MethodHuman)
		return err
	}
	if err := try(t, 0, fixedNow); err != nil {
		t.Errorf("zero skew, time == now: %v", err)
	}
	if err := try(t, 0, fixedNow.Add(time.Second)); !slices.Contains(problemPaths(err), "/time") {
		t.Errorf("zero skew, now+1s: want /time, got %v", err)
	}
	if err := try(t, time.Hour, fixedNow.Add(59*time.Minute)); err != nil {
		t.Errorf("1h skew, now+59m: %v", err)
	}
	if _, err := service.New(newFakeStore(), service.WithClockSkew(-time.Second)); err == nil {
		t.Error("negative skew must make New fail")
	}
}

func TestRecordRejectsTimeBeforeLinkTarget(t *testing.T) {
	svc, _ := newService(t)
	target := record(t, svc, testutil.Doc("alpha", "problem.reported", "target")) // 2026-03-10T09:00:00Z
	later := record(t, svc, func() map[string]any {
		d := testutil.Doc("alpha", "problem.reported", "later")
		d["time"] = "2026-03-10T12:00:00Z"
		return d
	}())
	at := func(ts string, links ...map[string]any) error {
		d := testutil.Doc("alpha", "proposal.recorded", "p", links...)
		d["time"] = ts
		_, err := svc.Record(ctx, d, service.MethodHuman)
		return err
	}
	t.Run("one second before", func(t *testing.T) {
		if err := at("2026-03-10T08:59:59Z", testutil.Link("motivated_by", target.ID)); !slices.Contains(problemPaths(err), "/links/0") {
			t.Fatalf("got %v", err)
		}
	})
	t.Run("equal time passes", func(t *testing.T) {
		if err := at("2026-03-10T09:00:00Z", testutil.Link("motivated_by", target.ID)); err != nil {
			t.Fatal(err)
		}
	})
	t.Run("offset instant before", func(t *testing.T) {
		if err := at("2026-03-10T10:59:00+02:00", testutil.Link("motivated_by", target.ID)); !slices.Contains(problemPaths(err), "/links/0") {
			t.Fatalf("got %v", err)
		}
	})
	t.Run("only the offending link is reported", func(t *testing.T) {
		err := at("2026-03-10T10:00:00Z", testutil.Link("motivated_by", target.ID), testutil.Link("relates_to", later.ID))
		paths := problemPaths(err)
		if !slices.Contains(paths, "/links/1") || slices.Contains(paths, "/links/0") {
			t.Fatalf("paths = %v", paths)
		}
	})
	t.Run("dangling target reports only the target", func(t *testing.T) {
		err := at("2026-03-01T00:00:00Z", testutil.Link("motivated_by", "evt_000000000000"))
		if paths := problemPaths(err); !slices.Equal(paths, []string{"/links/0/target"}) {
			t.Fatalf("paths = %v", paths)
		}
	})
}

func TestRecordReportsAllTimeProblems(t *testing.T) {
	svc, _ := newService(t)
	d := testutil.Doc("alpha", "proposal.recorded", "p", testutil.Link("motivated_by", "evt_000000000000"))
	d["time"] = "2026-03-12T10:05:01Z"
	_, err := svc.Record(ctx, d, service.MethodHuman)
	paths := problemPaths(err)
	if !slices.Contains(paths, "/time") || !slices.Contains(paths, "/links/0/target") {
		t.Fatalf("paths = %v", paths)
	}
}

func TestRecordSkipsTimeChecksOnBadTime(t *testing.T) {
	svc, _ := newService(t)
	target := record(t, svc, testutil.Doc("alpha", "problem.reported", "target"))
	d := testutil.Doc("alpha", "proposal.recorded", "p", testutil.Link("motivated_by", target.ID))
	d["time"] = "2026-03-11T10:12:00" // no zone
	_, err := svc.Record(ctx, d, service.MethodHuman)
	if !slices.Equal(problemPaths(err), []string{"/time"}) {
		t.Fatalf("paths = %v", problemPaths(err))
	}
}

func TestRecordStampedTimeRaisedToLinkTarget(t *testing.T) {
	svc, _ := newService(t)
	d := testutil.Doc("alpha", "problem.reported", "target")
	d["time"] = "2026-03-12T10:04:00Z"
	target := record(t, svc, d)

	linked := testutil.Doc("alpha", "proposal.recorded", "p", testutil.Link("motivated_by", target.ID))
	delete(linked, "time")
	if ev := record(t, svc, linked); ev.Time != "2026-03-12T10:04:00Z" {
		t.Errorf("linked stamp = %q, want raised to the target's time", ev.Time)
	}
	plain := testutil.Doc("alpha", "problem.reported", "plain")
	delete(plain, "time")
	if ev := record(t, svc, plain); ev.Time != "2026-03-12T10:00:00Z" {
		t.Errorf("plain stamp = %q, want the clock", ev.Time)
	}
}

func TestRecordComparesOffsetTargetAsInstant(t *testing.T) {
	svc, fs := newService(t)
	if err := fs.Insert(ctx, event.Event{ID: "evt_OFFSETTARGET", Time: "2026-03-10T12:00:00+02:00"}); err != nil {
		t.Fatal(err)
	}
	at := func(ts string) error {
		d := testutil.Doc("alpha", "proposal.recorded", "p", testutil.Link("motivated_by", "evt_OFFSETTARGET"))
		d["time"] = ts
		_, err := svc.Record(ctx, d, service.MethodHuman)
		return err
	}
	if err := at("2026-03-10T09:59:00Z"); !slices.Contains(problemPaths(err), "/links/0") {
		t.Errorf("09:59Z is before 10:00Z, got %v", err)
	}
	if err := at("2026-03-10T10:00:00Z"); err != nil {
		t.Errorf("10:00Z equals the target, got %v", err)
	}
}

func TestRecordSkipsLinkCheckForUnparsableTargetTime(t *testing.T) {
	svc, fs := newService(t)
	if err := fs.Insert(ctx, event.Event{ID: "evt_BADTIMETARGE", Time: "not a time"}); err != nil {
		t.Fatal(err)
	}
	d := testutil.Doc("alpha", "proposal.recorded", "p", testutil.Link("motivated_by", "evt_BADTIMETARGE"))
	record(t, svc, d)
}

func TestLinkIgnoresTimeOrder(t *testing.T) {
	svc, _ := newService(t)
	early := testutil.Doc("alpha", "problem.reported", "early")
	early["time"] = "2026-03-01T00:00:00Z"
	late := testutil.Doc("alpha", "problem.reported", "late")
	late["time"] = "2026-03-02T00:00:00Z"
	a, b := record(t, svc, early), record(t, svc, late)
	if _, err := svc.Link(ctx, service.LinkInput{Source: a.ID, Target: b.ID, Rel: "relates_to"}); err != nil {
		t.Fatal(err)
	}
}

func TestRecordStampNotRaisedBeyondSkew(t *testing.T) {
	svc, fs := newService(t)
	if err := fs.Insert(ctx, event.Event{ID: "evt_FARFUTURETGT", Time: "2027-01-01T00:00:00Z"}); err != nil {
		t.Fatal(err)
	}
	d := testutil.Doc("alpha", "proposal.recorded", "p", testutil.Link("motivated_by", "evt_FARFUTURETGT"))
	delete(d, "time")
	_, err := svc.Record(ctx, d, service.MethodHuman)
	if !slices.Contains(problemPaths(err), "/links/0") {
		t.Fatalf("want /links/0, got %v", err)
	}
}

func TestImportAppliesTimeRules(t *testing.T) {
	svc, _ := newService(t)
	withID := func(id, ts string, links ...map[string]any) map[string]any {
		d := testutil.Doc("alpha", "problem.reported", id, links...)
		d["id"], d["time"] = id, ts
		d["schema_version"] = event.SchemaVersion
		d["provenance"] = map[string]any{"recorded_by": "user:test", "method": service.MethodHuman}
		return d
	}
	if _, _, err := svc.Import(ctx, []map[string]any{withID("evt_IMPORTFUTURE", "2026-03-12T10:05:01Z")}); !slices.Contains(problemPaths(err), "/time") {
		t.Errorf("future time: got %v", err)
	}
	first := withID("evt_IMPORTFIRST0", "2026-03-10T12:00:00Z")
	back := withID("evt_IMPORTBACK00", "2026-03-10T11:00:00Z", testutil.Link("motivated_by", "evt_IMPORTFIRST0"))
	if _, _, err := svc.Import(ctx, []map[string]any{first, back}); !slices.Contains(problemPaths(err), "/links/0") {
		t.Errorf("backward link: got %v", err)
	}
	off := withID("evt_IMPORTOFFSET", "2026-03-11T12:00:00+02:00")
	if _, _, err := svc.Import(ctx, []map[string]any{off}); err != nil {
		t.Fatal(err)
	}
	d, err := svc.Get(ctx, "evt_IMPORTOFFSET")
	if err != nil || d.Event.Time != "2026-03-11T10:00:00Z" {
		t.Errorf("offset time stored as %q, %v", d.Event.Time, err)
	}
}
