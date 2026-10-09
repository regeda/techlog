package api_test

import (
	"bytes"
	"context"
	"encoding/json"
	"io"
	"net/http"
	"net/http/httptest"
	"net/url"
	"regexp"
	"slices"
	"strings"
	"testing"

	"github.com/regeda/techlog/internal/api"
	"github.com/regeda/techlog/internal/service"
	"github.com/regeda/techlog/internal/testutil"
)

func setup(t *testing.T) (http.Handler, *service.Service) {
	t.Helper()
	svc := testutil.NewService(t)
	return api.Handler(svc), svc
}

// do sends a request; body may be nil, a string (sent verbatim) or any JSON value.
func do(h http.Handler, method, target string, body any) *httptest.ResponseRecorder {
	var rdr io.Reader
	switch b := body.(type) {
	case nil:
	case string:
		rdr = strings.NewReader(b)
	default:
		raw, _ := json.Marshal(b)
		rdr = bytes.NewReader(raw)
	}
	rec := httptest.NewRecorder()
	h.ServeHTTP(rec, httptest.NewRequest(method, target, rdr))
	return rec
}

func decode(t *testing.T, rec *httptest.ResponseRecorder) map[string]any {
	t.Helper()
	var out map[string]any
	if err := json.Unmarshal(rec.Body.Bytes(), &out); err != nil {
		t.Fatalf("response is not a JSON object: %v\n%s", err, rec.Body)
	}
	return out
}

func errorOf(t *testing.T, rec *httptest.ResponseRecorder) (code string, paths []string) {
	t.Helper()
	e, ok := decode(t, rec)["error"].(map[string]any)
	if !ok {
		t.Fatalf("no error object in %s", rec.Body)
	}
	details, _ := e["details"].([]any)
	for _, d := range details {
		paths = append(paths, d.(map[string]any)["path"].(string))
	}
	return e["code"].(string), paths
}

func record(t *testing.T, svc *service.Service, doc map[string]any) string {
	t.Helper()
	ev, err := svc.Record(context.Background(), doc, service.MethodHuman)
	if err != nil {
		t.Fatal(err)
	}
	return ev.ID
}

func summaries(t *testing.T, rec *httptest.ResponseRecorder) []string {
	t.Helper()
	if rec.Code != http.StatusOK {
		t.Fatalf("status %d: %s", rec.Code, rec.Body)
	}
	events, ok := decode(t, rec)["events"].([]any)
	if !ok {
		t.Fatalf("events is not an array in %s", rec.Body)
	}
	out := make([]string, len(events))
	for i, e := range events {
		out[i], _ = e.(map[string]any)["summary"].(string)
	}
	return out
}

func TestRecordEvent(t *testing.T) {
	h, _ := setup(t)
	rec := do(h, "POST", "/api/v1/events", testutil.Doc("alpha", "problem.reported", "hello"))
	if rec.Code != http.StatusCreated {
		t.Fatalf("status %d: %s", rec.Code, rec.Body)
	}
	ev := decode(t, rec)["event"].(map[string]any)
	id := ev["id"].(string)
	if !regexp.MustCompile(`^evt_[0-9A-Z]{12}$`).MatchString(id) {
		t.Errorf("id = %q", id)
	}
	if m := ev["provenance"].(map[string]any)["method"]; m != "human_entered" {
		t.Errorf("provenance.method = %v, want human_entered", m)
	}
	if loc := rec.Header().Get("Location"); loc != "/api/v1/events/"+id {
		t.Errorf("Location = %q", loc)
	}
}

func TestRecordIgnoresClientID(t *testing.T) {
	h, _ := setup(t)
	d := testutil.Doc("alpha", "problem.reported", "hello")
	d["id"] = "evt_AAAAAAAAAAAA"
	rec := do(h, "POST", "/api/v1/events", d)
	if rec.Code != http.StatusCreated {
		t.Fatalf("status %d: %s", rec.Code, rec.Body)
	}
	if id := decode(t, rec)["event"].(map[string]any)["id"]; id == "evt_AAAAAAAAAAAA" {
		t.Error("the client-supplied id must be ignored")
	}
}

func TestRecordValidation(t *testing.T) {
	h, _ := setup(t)
	d := testutil.Doc("alpha", "problem.reported", "hello")
	d["actor"] = map[string]any{"kind": "robot", "id": "x"}
	rec := do(h, "POST", "/api/v1/events", d)
	if rec.Code != http.StatusUnprocessableEntity {
		t.Fatalf("status %d: %s", rec.Code, rec.Body)
	}
	code, paths := errorOf(t, rec)
	if code != "invalid_event" || !slices.Contains(paths, "/actor/kind") {
		t.Errorf("code=%q paths=%v", code, paths)
	}
}

func TestRecordDanglingLink(t *testing.T) {
	h, _ := setup(t)
	d := testutil.Doc("alpha", "proposal.recorded", "p", testutil.Link("motivated_by", "evt_000000000000"))
	rec := do(h, "POST", "/api/v1/events", d)
	code, paths := errorOf(t, rec)
	if rec.Code != http.StatusUnprocessableEntity || code != "invalid_event" || !slices.Contains(paths, "/links/0/target") {
		t.Fatalf("status %d code=%q paths=%v", rec.Code, code, paths)
	}
}

func TestRecordMalformedBodies(t *testing.T) {
	h, _ := setup(t)
	bodies := map[string]string{
		"not json":      "this is not json",
		"empty":         "",
		"array":         "[1, 2]",
		"null":          "null",
		"string":        `"hello"`,
		"trailing data": `{"type": "x"} {"type": "y"}`,
		"oversized":     `{"summary": "` + strings.Repeat("a", 2<<20) + `"}`,
	}
	for name, body := range bodies {
		t.Run(name, func(t *testing.T) {
			rec := do(h, "POST", "/api/v1/events", body)
			code, _ := errorOf(t, rec)
			if rec.Code != http.StatusBadRequest || code != "bad_request" {
				t.Fatalf("status %d code %q, want 400 bad_request", rec.Code, code)
			}
		})
	}
}

func TestGetEvent(t *testing.T) {
	h, svc := setup(t)
	a := record(t, svc, testutil.Doc("alpha", "problem.reported", "a"))
	b := record(t, svc, testutil.Doc("alpha", "proposal.recorded", "b", testutil.Link("motivated_by", a)))

	rb := decode(t, do(h, "GET", "/api/v1/events/"+b, nil))
	if len(rb["outgoing"].([]any)) != 1 || len(rb["incoming"].([]any)) != 0 {
		t.Errorf("b = %v", rb)
	}
	ra := decode(t, do(h, "GET", "/api/v1/events/"+a, nil))
	if len(ra["incoming"].([]any)) != 1 || len(ra["outgoing"].([]any)) != 0 {
		t.Errorf("a = %v", ra)
	}

	rec := do(h, "GET", "/api/v1/events/evt_000000000000", nil)
	if code, _ := errorOf(t, rec); rec.Code != http.StatusNotFound || code != "not_found" {
		t.Fatalf("status %d code %q", rec.Code, code)
	}
}

func TestSearch(t *testing.T) {
	h, svc := setup(t)
	record(t, svc, testutil.Doc("alpha", "problem.reported", "flat signups"))
	record(t, svc, testutil.Doc("alpha", "proposal.recorded", "spin proposal"))
	record(t, svc, testutil.Doc("beta", "incident.opened", "canary errors"))

	if got := summaries(t, do(h, "GET", "/api/v1/events", nil)); len(got) != 3 {
		t.Errorf("all: %v", got)
	}
	if got := summaries(t, do(h, "GET", "/api/v1/events?system=alpha", nil)); len(got) != 2 {
		t.Errorf("system: %v", got)
	}
	if got := summaries(t, do(h, "GET", "/api/v1/events?type=incident.opened,problem.reported", nil)); len(got) != 2 {
		t.Errorf("types: %v", got)
	}
	if got := summaries(t, do(h, "GET", "/api/v1/events?q=spin", nil)); !slices.Equal(got, []string{"spin proposal"}) {
		t.Errorf("q: %v", got)
	}
	if got := summaries(t, do(h, "GET", "/api/v1/events?limit=1", nil)); len(got) != 1 {
		t.Errorf("limit: %v", got)
	}
	for _, bad := range []string{"limit=abc", "since=notadate", "until=2026-13-40"} {
		rec := do(h, "GET", "/api/v1/events?"+bad, nil)
		if code, _ := errorOf(t, rec); rec.Code != http.StatusBadRequest || code != "bad_request" {
			t.Errorf("%s: status %d code %q", bad, rec.Code, code)
		}
	}
}

func TestSearchPunctuation(t *testing.T) {
	h, svc := setup(t)
	d := testutil.Doc("promo-api", "incident.opened", "5xx ratio on spin-service")
	record(t, svc, d)
	for _, q := range []string{"promo-api", "spin-service", `"quoted`, "AND", "(", "a OR b", "col:val", "*"} {
		rec := do(h, "GET", "/api/v1/events?q="+url.QueryEscape(q), nil)
		if rec.Code != http.StatusOK {
			t.Errorf("q=%q: status %d: %s", q, rec.Code, rec.Body)
		}
	}
	if got := summaries(t, do(h, "GET", "/api/v1/events?q=promo-api", nil)); len(got) != 1 {
		t.Errorf("q=promo-api: %v", got)
	}
}

func TestSearchOffsets(t *testing.T) {
	h, svc := setup(t)
	early := testutil.Doc("alpha", "problem.reported", "early")
	early["time"] = "2026-03-11T12:00:00+02:00" // 10:00Z, although the text sorts last
	late := testutil.Doc("alpha", "problem.reported", "late")
	late["time"] = "2026-03-11T10:30:00Z"
	record(t, svc, early)
	record(t, svc, late)

	if got := summaries(t, do(h, "GET", "/api/v1/events?since=2026-03-11T10:15:00Z", nil)); !slices.Equal(got, []string{"late"}) {
		t.Errorf("since: %v", got)
	}
	if got := summaries(t, do(h, "GET", "/api/v1/events?until=2026-03-11T10:00:00Z", nil)); !slices.Equal(got, []string{"early"}) {
		t.Errorf("until: %v", got)
	}
	if got := summaries(t, do(h, "GET", "/api/v1/systems/alpha/history", nil)); !slices.Equal(got, []string{"early", "late"}) {
		t.Errorf("history order: %v", got)
	}
}

func TestHistory(t *testing.T) {
	h, svc := setup(t)
	for _, s := range []struct{ summary, ts string }{{"third", "2026-03-10T11:00:00Z"}, {"first", "2026-03-10T09:00:00Z"}, {"second", "2026-03-10T10:00:00Z"}} {
		d := testutil.Doc("alpha", "problem.reported", s.summary)
		d["time"] = s.ts
		record(t, svc, d)
	}
	if got := summaries(t, do(h, "GET", "/api/v1/systems/alpha/history", nil)); !slices.Equal(got, []string{"first", "second", "third"}) {
		t.Errorf("history: %v", got)
	}
	// An unknown system has an empty history, not an error.
	rec := do(h, "GET", "/api/v1/systems/nope/history", nil)
	if got := summaries(t, rec); len(got) != 0 {
		t.Errorf("unknown system: %v", got)
	}
}

func TestSystems(t *testing.T) {
	h, svc := setup(t)
	record(t, svc, testutil.Doc("beta", "problem.reported", "x"))
	record(t, svc, testutil.Doc("alpha", "problem.reported", "y"))
	got := decode(t, do(h, "GET", "/api/v1/systems", nil))["systems"].([]any)
	if len(got) != 2 || got[0] != "alpha" || got[1] != "beta" {
		t.Errorf("systems = %v", got)
	}
}

func TestLinks(t *testing.T) {
	h, svc := setup(t)
	a := record(t, svc, testutil.Doc("alpha", "incident.opened", "a"))
	b := record(t, svc, testutil.Doc("alpha", "investigation.completed", "b"))

	rec := do(h, "POST", "/api/v1/links", map[string]any{"source": b, "target": a, "rel": "caused_by", "basis": "hypothesis", "rationale": "guess"})
	if rec.Code != http.StatusCreated {
		t.Fatalf("status %d: %s", rec.Code, rec.Body)
	}
	if out := decode(t, do(h, "GET", "/api/v1/events/"+a, nil)); len(out["incoming"].([]any)) != 1 {
		t.Errorf("a = %v", out)
	}

	self := do(h, "POST", "/api/v1/links", map[string]any{"source": a, "target": a, "rel": "relates_to"})
	if code, paths := errorOf(t, self); self.Code != http.StatusUnprocessableEntity || code != "invalid_event" || !slices.Contains(paths, "/target") {
		t.Errorf("self link: status %d code %q paths %v", self.Code, code, paths)
	}
	dangling := do(h, "POST", "/api/v1/links", map[string]any{"source": a, "target": "evt_000000000000", "rel": "relates_to"})
	if dangling.Code != http.StatusUnprocessableEntity {
		t.Errorf("dangling link: status %d", dangling.Code)
	}
	unknown := do(h, "POST", "/api/v1/links", `{"source":"x","target":"y","rel":"relates_to","extra":1}`)
	if code, _ := errorOf(t, unknown); unknown.Code != http.StatusBadRequest || code != "bad_request" {
		t.Errorf("unknown field: status %d code %q", unknown.Code, code)
	}
}

func TestUnknownRoute(t *testing.T) {
	h, _ := setup(t)
	rec := do(h, "GET", "/api/v1/nope", nil)
	if code, _ := errorOf(t, rec); rec.Code != http.StatusNotFound || code != "not_found" {
		t.Fatalf("status %d code %q", rec.Code, code)
	}
}

func TestRecordDuplicateKeyIs409(t *testing.T) {
	h, _ := setup(t)
	d := testutil.Doc("alpha", "problem.reported", "hello")
	d["idempotency_key"] = "deploy-4411"
	first := do(h, "POST", "/api/v1/events", d)
	if first.Code != http.StatusCreated {
		t.Fatalf("first: status %d: %s", first.Code, first.Body)
	}
	firstID := decode(t, first)["event"].(map[string]any)["id"].(string)

	rec := do(h, "POST", "/api/v1/events", d)
	if rec.Code != http.StatusConflict {
		t.Fatalf("status %d: %s", rec.Code, rec.Body)
	}
	e := decode(t, rec)["error"].(map[string]any)
	if e["code"] != "duplicate_event" || e["existing_id"] != firstID {
		t.Errorf("error = %v, want code duplicate_event and existing_id %s", e, firstID)
	}
	if msg, _ := e["message"].(string); !strings.Contains(msg, `"deploy-4411"`) || !strings.Contains(msg, firstID) {
		t.Errorf("message = %q", msg)
	}
	if _, has := decode(t, do(h, "POST", "/api/v1/events", testutil.Doc("alpha", "problem.reported", "x")))["error"]; has {
		t.Error("an event with a fresh key must still be accepted")
	}
}

func TestRecordSameKeyDifferentActorIs201(t *testing.T) {
	h, _ := setup(t)
	a := testutil.Doc("alpha", "problem.reported", "a")
	a["idempotency_key"] = "shared"
	b := testutil.Doc("alpha", "problem.reported", "b")
	b["idempotency_key"] = "shared"
	b["actor"] = map[string]any{"kind": "human", "id": "user:other"}
	for _, d := range []map[string]any{a, b} {
		if rec := do(h, "POST", "/api/v1/events", d); rec.Code != http.StatusCreated {
			t.Fatalf("status %d: %s", rec.Code, rec.Body)
		}
	}
}

func TestRecordKeyValidation(t *testing.T) {
	h, _ := setup(t)
	missing := testutil.Doc("alpha", "problem.reported", "hello")
	delete(missing, "idempotency_key")
	empty := testutil.Doc("alpha", "problem.reported", "hello")
	empty["idempotency_key"] = ""
	for name, tc := range map[string]struct {
		doc  map[string]any
		path string
	}{"missing": {missing, "/"}, "empty": {empty, "/idempotency_key"}} {
		t.Run(name, func(t *testing.T) {
			rec := do(h, "POST", "/api/v1/events", tc.doc)
			if rec.Code != http.StatusUnprocessableEntity {
				t.Fatalf("status %d: %s", rec.Code, rec.Body)
			}
			code, paths := errorOf(t, rec)
			if code != "invalid_event" || !slices.Contains(paths, tc.path) {
				t.Errorf("code=%q paths=%v, want a problem at %q", code, paths, tc.path)
			}
		})
	}
}
