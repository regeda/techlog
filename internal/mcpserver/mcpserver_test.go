package mcpserver_test

import (
	"context"
	"encoding/json"
	"slices"
	"strings"
	"testing"

	"github.com/modelcontextprotocol/go-sdk/mcp"

	"github.com/regeda/techlog/internal/event"
	"github.com/regeda/techlog/internal/mcpserver"
	"github.com/regeda/techlog/internal/service"
	"github.com/regeda/techlog/internal/testutil"
)

var ctx = context.Background()

func connect(t *testing.T) *mcp.ClientSession {
	t.Helper()
	srv := mcpserver.New(testutil.NewService(t))
	clientT, serverT := mcp.NewInMemoryTransports()
	ss, err := srv.Connect(ctx, serverT, nil)
	if err != nil {
		t.Fatal(err)
	}
	t.Cleanup(func() { ss.Close() })
	cs, err := mcp.NewClient(&mcp.Implementation{Name: "test", Version: "0"}, nil).Connect(ctx, clientT, nil)
	if err != nil {
		t.Fatal(err)
	}
	t.Cleanup(func() { cs.Close() })
	return cs
}

func call(t *testing.T, cs *mcp.ClientSession, name string, args map[string]any) *mcp.CallToolResult {
	t.Helper()
	res, err := cs.CallTool(ctx, &mcp.CallToolParams{Name: name, Arguments: args})
	if err != nil {
		t.Fatalf("%s: protocol error: %v", name, err)
	}
	return res
}

func text(res *mcp.CallToolResult) string {
	var parts []string
	for _, c := range res.Content {
		if tc, ok := c.(*mcp.TextContent); ok {
			parts = append(parts, tc.Text)
		}
	}
	return strings.Join(parts, "\n")
}

func structured(t *testing.T, res *mcp.CallToolResult, v any) {
	t.Helper()
	raw, err := json.Marshal(res.StructuredContent)
	if err != nil {
		t.Fatal(err)
	}
	if err := json.Unmarshal(raw, v); err != nil {
		t.Fatalf("structuredContent %s: %v", raw, err)
	}
}

func recordOK(t *testing.T, cs *mcp.ClientSession, doc map[string]any) string {
	t.Helper()
	res := call(t, cs, "techlog_record_event", map[string]any{"event": doc})
	if res.IsError {
		t.Fatalf("record failed: %s", text(res))
	}
	var out struct{ ID string }
	structured(t, res, &out)
	return out.ID
}

func TestToolList(t *testing.T) {
	cs := connect(t)
	res, err := cs.ListTools(ctx, nil)
	if err != nil {
		t.Fatal(err)
	}
	var names []string
	for _, tool := range res.Tools {
		names = append(names, tool.Name)
	}
	slices.Sort(names)
	want := []string{"techlog_get_event", "techlog_get_system_history", "techlog_link_events", "techlog_record_event", "techlog_search_events"}
	if !slices.Equal(names, want) {
		t.Fatalf("tools = %v, want %v", names, want)
	}
}

func TestRecordAndGet(t *testing.T) {
	cs := connect(t)
	res := call(t, cs, "techlog_record_event", map[string]any{"event": testutil.Doc("alpha", "decision.made", "approved")})
	if res.IsError {
		t.Fatalf("record failed: %s", text(res))
	}
	var rec struct{ ID, Status string }
	structured(t, res, &rec)
	if !strings.HasPrefix(rec.ID, "evt_") || rec.Status != "recorded" {
		t.Errorf("record result = %+v", rec)
	}
	if want := "Recorded " + rec.ID + " (recorded)"; text(res) != want {
		t.Errorf("text = %q, want %q", text(res), want)
	}

	got := call(t, cs, "techlog_get_event", map[string]any{"id": rec.ID})
	if got.IsError {
		t.Fatalf("get failed: %s", text(got))
	}
	var d service.Detail
	structured(t, got, &d)
	if d.Event.ID != rec.ID || d.Event.Provenance.Method != "agent_submitted" {
		t.Errorf("detail = %+v", d.Event)
	}

	missing := call(t, cs, "techlog_get_event", map[string]any{"id": "evt_000000000000"})
	if !missing.IsError || !strings.Contains(text(missing), "not found") {
		t.Errorf("missing event: isError=%v text=%q", missing.IsError, text(missing))
	}
}

func TestRecordInvalidIsActionable(t *testing.T) {
	cs := connect(t)
	d := testutil.Doc("alpha", "problem.reported", "x")
	d["actor"] = map[string]any{"kind": "robot", "id": "x"}
	res := call(t, cs, "techlog_record_event", map[string]any{"event": d})
	if !res.IsError {
		t.Fatal("expected isError")
	}
	if !strings.Contains(text(res), "/actor/kind") {
		t.Errorf("text should name the failing field path, got %q", text(res))
	}
}

func TestSearchWithoutQuery(t *testing.T) {
	cs := connect(t)
	recordOK(t, cs, testutil.Doc("alpha", "problem.reported", "a"))
	recordOK(t, cs, testutil.Doc("beta", "problem.reported", "b"))

	res := call(t, cs, "techlog_search_events", map[string]any{"system": "alpha"})
	if res.IsError {
		t.Fatalf("search failed: %s", text(res))
	}
	var out struct {
		Events []event.Event
		Count  int
	}
	structured(t, res, &out)
	if out.Count != 1 || len(out.Events) != 1 || out.Events[0].Subject.System != "alpha" {
		t.Errorf("out = %+v", out)
	}

	empty := call(t, cs, "techlog_search_events", map[string]any{"query": "nothing-matches"})
	if empty.IsError || !strings.Contains(text(empty), "No events found") {
		t.Errorf("empty search: isError=%v text=%q", empty.IsError, text(empty))
	}
}

func TestSearchPunctuationAndBadDates(t *testing.T) {
	cs := connect(t)
	recordOK(t, cs, testutil.Doc("promo-api", "incident.opened", "5xx"))
	for _, q := range []string{"promo-api", `"quoted`, "(", "a OR b"} {
		if res := call(t, cs, "techlog_search_events", map[string]any{"query": q}); res.IsError {
			t.Errorf("query %q: %s", q, text(res))
		}
	}
	res := call(t, cs, "techlog_search_events", map[string]any{"since": "yesterday"})
	if !res.IsError || !strings.Contains(text(res), "since") {
		t.Errorf("bad since: isError=%v text=%q", res.IsError, text(res))
	}
}

func TestLinkEvents(t *testing.T) {
	cs := connect(t)
	a := recordOK(t, cs, testutil.Doc("alpha", "incident.opened", "a"))
	b := recordOK(t, cs, testutil.Doc("alpha", "investigation.completed", "b"))

	ok := call(t, cs, "techlog_link_events", map[string]any{"source": b, "target": a, "rel": "relates_to"})
	if ok.IsError {
		t.Fatalf("link failed: %s", text(ok))
	}
	var d service.Detail
	structured(t, call(t, cs, "techlog_get_event", map[string]any{"id": b}), &d)
	if len(d.Event.Links) != 1 || d.Event.Links[0].Basis != "confirmed" {
		t.Errorf("links = %+v", d.Event.Links)
	}

	noBasis := call(t, cs, "techlog_link_events", map[string]any{"source": b, "target": a, "rel": "caused_by"})
	if !noBasis.IsError || !strings.Contains(text(noBasis), "basis") {
		t.Errorf("caused_by without basis: isError=%v text=%q", noBasis.IsError, text(noBasis))
	}
	hyp := call(t, cs, "techlog_link_events", map[string]any{"source": b, "target": a, "rel": "caused_by", "basis": "hypothesis", "rationale": "guess"})
	if hyp.IsError {
		t.Errorf("hypothesis link failed: %s", text(hyp))
	}
	self := call(t, cs, "techlog_link_events", map[string]any{"source": a, "target": a, "rel": "relates_to"})
	if !self.IsError || !strings.Contains(text(self), "itself") {
		t.Errorf("self link: isError=%v text=%q", self.IsError, text(self))
	}
}

func TestSystemHistory(t *testing.T) {
	cs := connect(t)
	first := testutil.Doc("alpha", "problem.reported", "first")
	first["time"] = "2026-03-10T09:00:00Z"
	a := recordOK(t, cs, first)
	second := testutil.Doc("alpha", "proposal.recorded", "second", testutil.Link("motivated_by", a))
	second["time"] = "2026-03-10T10:00:00Z"
	recordOK(t, cs, second)

	type history struct {
		Events []event.Event
		Count  int
	}
	var withLinks history
	structured(t, call(t, cs, "techlog_get_system_history", map[string]any{"system": "alpha"}), &withLinks)
	if withLinks.Count != 2 || withLinks.Events[0].Summary != "first" || len(withLinks.Events[1].Links) != 1 {
		t.Errorf("with links = %+v", withLinks)
	}

	var without history
	structured(t, call(t, cs, "techlog_get_system_history", map[string]any{"system": "alpha", "include_links": false}), &without)
	for _, e := range without.Events {
		if len(e.Links) != 0 {
			t.Errorf("include_links=false but %s has links", e.ID)
		}
	}

	none := call(t, cs, "techlog_get_system_history", map[string]any{"system": "nope"})
	if none.IsError {
		t.Errorf("unknown system must not be an error: %s", text(none))
	}
}
