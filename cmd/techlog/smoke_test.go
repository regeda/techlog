package main

import (
	"context"
	"encoding/json"
	"io"
	"log/slog"
	"net/http"
	"net/http/httptest"
	"strings"
	"testing"

	"github.com/modelcontextprotocol/go-sdk/mcp"
	"github.com/regeda/techlog/internal/seed"
	"github.com/regeda/techlog/internal/testutil"
)

func httpGet(t *testing.T, url string) string {
	t.Helper()
	resp, err := http.Get(url) //nolint:gosec // G107: the url is the test server
	if err != nil {
		t.Fatal(err)
	}
	defer func() {
		if err := resp.Body.Close(); err != nil {
			t.Error(err)
		}
	}()
	body, err := io.ReadAll(resp.Body)
	if err != nil {
		t.Fatal(err)
	}
	if resp.StatusCode != http.StatusOK {
		t.Fatalf("GET %s: status %d\n%s", url, resp.StatusCode, body)
	}
	return string(body)
}

func mustHave(t *testing.T, what, body string, wants ...string) {
	t.Helper()
	for _, w := range wants {
		if !strings.Contains(body, w) {
			t.Errorf("%s: missing %q\n%s", what, w, body)
		}
	}
}

func connectMCP(ctx context.Context, t *testing.T, endpoint string) *mcp.ClientSession {
	t.Helper()
	cs, err := mcp.NewClient(&mcp.Implementation{Name: "smoke", Version: "0"}, nil).Connect(ctx,
		&mcp.StreamableClientTransport{Endpoint: endpoint, DisableStandaloneSSE: true}, nil)
	if err != nil {
		t.Fatalf("connect to /mcp: %v", err)
	}
	testutil.CloseOnCleanup(t, cs)
	return cs
}

// recordOverMCP records doc with the techlog_record_event tool and returns the new event's id.
func recordOverMCP(ctx context.Context, t *testing.T, cs *mcp.ClientSession, doc map[string]any) string {
	t.Helper()
	res, err := cs.CallTool(ctx, &mcp.CallToolParams{Name: "techlog_record_event", Arguments: map[string]any{"event": doc}})
	if err != nil || res.IsError {
		t.Fatalf("record over MCP: err=%v result=%+v", err, res)
	}
	raw, err := json.Marshal(res.StructuredContent)
	if err != nil {
		t.Fatal(err)
	}
	var rec struct{ ID string }
	if err := json.Unmarshal(raw, &rec); err != nil || rec.ID == "" {
		t.Fatalf("structuredContent %s: %v", raw, err)
	}
	return rec.ID
}

// TestSmoke records an event over MCP, then reads it back over the HTTP API
// and in the UI, all against one server on one port.
func TestSmoke(t *testing.T) {
	ctx := context.Background()
	svc := testutil.NewService(t)
	docs, err := seed.SpringSpin()
	if err != nil {
		t.Fatal(err)
	}
	if inserted, _, err := svc.Import(ctx, docs); err != nil || inserted != 25 {
		t.Fatalf("seed import = %d, %v", inserted, err)
	}
	ts := httptest.NewServer(newHandler(svc, slog.New(slog.NewTextHandler(io.Discard, nil))))
	t.Cleanup(ts.Close) // registered before the client, so the client closes first

	cs := connectMCP(ctx, t, ts.URL+"/mcp")
	doc := testutil.Doc("promo-api", "decision.made", "Smoke test decision",
		testutil.Link("relates_to", "evt_01J9K2A0P011"))
	doc["time"] = "2026-03-20T09:00:00Z" // after the linked seed event
	id := recordOverMCP(ctx, t, cs, doc)

	again, err := cs.CallTool(ctx, &mcp.CallToolParams{Name: "techlog_record_event", Arguments: map[string]any{"event": doc}})
	if err != nil || !again.IsError {
		t.Fatalf("recording the same event twice must be rejected: err=%v result=%+v", err, again)
	}
	mustHave(t, "duplicate result", testutil.As[*mcp.TextContent](t, again.Content[0]).Text, id)

	mustHave(t, "API event", httpGet(t, ts.URL+"/api/v1/events/"+id), `"outgoing"`, "evt_01J9K2A0P011", "agent_submitted")
	mustHave(t, "API search", httpGet(t, ts.URL+"/api/v1/events?q=smoke"), id)
	mustHave(t, "UI event page", httpGet(t, ts.URL+"/events/"+id), "Smoke test decision", "evt_01J9K2A0P011")
	mustHave(t, "UI list", httpGet(t, ts.URL+"/?q=smoke"), "Smoke test decision")
	mustHave(t, "UI seeded list", httpGet(t, ts.URL+"/?q=5xx"), "5xx ratio")
	mustHave(t, "UI system page", httpGet(t, ts.URL+"/systems/promo-api"), "Smoke test decision")
	mustHave(t, "UI story", httpGet(t, ts.URL+"/story/evt_01J9K2A0P011"), "Smoke test decision", "5xx ratio")
}
