package ui_test

import (
	"context"
	"net/http"
	"net/http/httptest"
	"strings"
	"testing"

	"github.com/regeda/techlog/internal/service"
	"github.com/regeda/techlog/internal/testutil"
	"github.com/regeda/techlog/internal/ui"
)

func setup(t *testing.T) (http.Handler, *service.Service) {
	t.Helper()
	svc := testutil.NewService(t)
	return ui.Handler(svc), svc
}

func get(h http.Handler, target string) *httptest.ResponseRecorder {
	rec := httptest.NewRecorder()
	h.ServeHTTP(rec, httptest.NewRequest("GET", target, nil))
	return rec
}

func record(t *testing.T, svc *service.Service, doc map[string]any) string {
	t.Helper()
	ev, err := svc.Record(context.Background(), doc, service.MethodHuman)
	if err != nil {
		t.Fatal(err)
	}
	return ev.ID
}

func at(doc map[string]any, ts string) map[string]any {
	doc["time"] = ts
	return doc
}

func mustContain(t *testing.T, rec *httptest.ResponseRecorder, wants ...string) {
	t.Helper()
	body := rec.Body.String()
	for _, w := range wants {
		if !strings.Contains(body, w) {
			t.Errorf("body does not contain %q\n%s", w, body)
		}
	}
}

func mustNotContain(t *testing.T, rec *httptest.ResponseRecorder, nots ...string) {
	t.Helper()
	body := rec.Body.String()
	for _, n := range nots {
		if strings.Contains(body, n) {
			t.Errorf("body unexpectedly contains %q", n)
		}
	}
}

func TestEventsList(t *testing.T) {
	h, svc := setup(t)
	record(t, svc, testutil.Doc("alpha", "problem.reported", "flat signups"))
	record(t, svc, testutil.Doc("beta", "incident.opened", "canary errors"))

	rec := get(h, "/")
	if rec.Code != http.StatusOK || !strings.HasPrefix(rec.Header().Get("Content-Type"), "text/html") {
		t.Fatalf("status %d, content-type %q", rec.Code, rec.Header().Get("Content-Type"))
	}
	mustContain(t, rec, "flat signups", "canary errors", `<option value="alpha"`, `<option value="incident.opened"`)

	rec = get(h, "/?system=alpha")
	mustContain(t, rec, "flat signups")
	mustNotContain(t, rec, "canary errors")

	rec = get(h, "/?q=canary")
	mustContain(t, rec, "canary errors")
	mustNotContain(t, rec, "flat signups")

	rec = get(h, "/?type=incident.opened")
	mustContain(t, rec, "canary errors")
	mustNotContain(t, rec, "flat signups")

	mustContain(t, get(h, "/?q=nothing-matches"), "No events match")
}

func TestEventsBadDate(t *testing.T) {
	h, _ := setup(t)
	rec := get(h, "/?since=nope")
	if rec.Code != http.StatusBadRequest {
		t.Fatalf("status %d", rec.Code)
	}
	mustContain(t, rec, "invalid since")
}

func TestEscaping(t *testing.T) {
	h, svc := setup(t)
	id := record(t, svc, testutil.Doc("alpha", "problem.reported", `<script>alert(1)</script>`))
	for _, target := range []string{"/", "/events/" + id, "/systems/alpha", "/story/" + id} {
		rec := get(h, target)
		if rec.Code != http.StatusOK {
			t.Errorf("%s: status %d", target, rec.Code)
		}
		mustContain(t, rec, "&lt;script&gt;")
		mustNotContain(t, rec, "<script>alert")
	}
}

func TestEventPage(t *testing.T) {
	h, svc := setup(t)
	a := record(t, svc, testutil.Doc("alpha", "incident.opened", "the incident"))
	b := record(t, svc, testutil.Doc("alpha", "investigation.completed", "the cause", testutil.Link("caused_by", a, "hypothesis")))

	rec := get(h, "/events/"+b)
	mustContain(t, rec, "the cause", "caused_by", "hypothesis", `href="/events/`+a+`"`, "the incident", "Walk through", "Raw JSON", `href="/story/`+b+`"`)

	rec = get(h, "/events/"+a)
	mustContain(t, rec, "Links to this event", `href="/events/`+b+`"`, "the cause")

	if rec := get(h, "/events/evt_000000000000"); rec.Code != http.StatusNotFound {
		t.Errorf("unknown event: status %d", rec.Code)
	}
}

func TestSystemPage(t *testing.T) {
	h, svc := setup(t)
	record(t, svc, at(testutil.Doc("alpha", "proposal.recorded", "second event"), "2026-03-10T10:00:00Z"))
	record(t, svc, at(testutil.Doc("alpha", "problem.reported", "first event"), "2026-03-10T09:00:00Z"))

	rec := get(h, "/systems/alpha")
	mustContain(t, rec, "first event", "second event")
	body := rec.Body.String()
	if strings.Index(body, "first event") > strings.Index(body, "second event") {
		t.Error("the timeline must list the oldest event first")
	}

	rec = get(h, "/systems/no-such-system")
	if rec.Code != http.StatusOK {
		t.Fatalf("unknown system: status %d", rec.Code)
	}
	mustContain(t, rec, "No events recorded")
}

func TestStoryPage(t *testing.T) {
	h, svc := setup(t)
	a := record(t, svc, at(testutil.Doc("alpha", "problem.reported", "story start"), "2026-03-10T09:00:00Z"))
	b := record(t, svc, at(testutil.Doc("alpha", "proposal.recorded", "story middle", testutil.Link("motivated_by", a)), "2026-03-10T10:00:00Z"))
	c := record(t, svc, at(testutil.Doc("alpha", "incident.opened", "story end", testutil.Link("caused_by", b, "hypothesis")), "2026-03-10T11:00:00Z"))
	record(t, svc, at(testutil.Doc("alpha", "problem.reported", "unrelated"), "2026-03-10T12:00:00Z"))
	// Close a cycle: a relates_to c.
	if _, err := svc.Link(context.Background(), service.LinkInput{Source: a, Target: c, Rel: "relates_to"}); err != nil {
		t.Fatal(err)
	}

	rec := get(h, "/story/"+c)
	if rec.Code != http.StatusOK {
		t.Fatalf("status %d", rec.Code)
	}
	mustContain(t, rec, "story start", "story middle", "story end", `id="`+a+`"`, `href="#`+b+`"`, "t-incident root", "hypothesis")
	mustNotContain(t, rec, "unrelated")

	if rec := get(h, "/story/evt_000000000000"); rec.Code != http.StatusNotFound {
		t.Errorf("unknown story: status %d", rec.Code)
	}
}

func TestStaticAndUnknownPaths(t *testing.T) {
	h, _ := setup(t)
	rec := get(h, "/static/style.css")
	if rec.Code != http.StatusOK || !strings.Contains(rec.Header().Get("Content-Type"), "text/css") {
		t.Errorf("style.css: status %d, content-type %q", rec.Code, rec.Header().Get("Content-Type"))
	}
	if rec := get(h, "/nope"); rec.Code != http.StatusNotFound {
		t.Errorf("/nope: status %d", rec.Code)
	}
}
