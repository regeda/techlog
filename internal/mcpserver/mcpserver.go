// Package mcpserver exposes Techlog to MCP clients (such as Claude) as five
// tools over the streamable HTTP transport.
package mcpserver

import (
	"context"
	"errors"
	"fmt"
	"log/slog"
	"net/http"
	"strings"
	"time"

	"github.com/modelcontextprotocol/go-sdk/mcp"

	"github.com/regeda/techlog/internal/event"
	"github.com/regeda/techlog/internal/service"
	"github.com/regeda/techlog/internal/store"
)

const serverVersion = "0.1.0"

// New builds the MCP server with Techlog's tools.
func New(svc *service.Service) *mcp.Server {
	s := mcp.NewServer(&mcp.Implementation{Name: "techlog", Version: serverVersion}, nil)
	t := &tools{svc: svc}
	mcp.AddTool(s, &mcp.Tool{
		Name:        "techlog_search_events",
		Description: "Search engineering events by free text, system, type and time range. Newest first. Use it before recording, to find the earlier events a new one should link to.",
	}, t.searchEvents)
	mcp.AddTool(s, &mcp.Tool{
		Name:        "techlog_get_system_history",
		Description: "Read the latest events of one system in chronological order, optionally with their links.",
	}, t.systemHistory)
	mcp.AddTool(s, &mcp.Tool{
		Name:        "techlog_get_event",
		Description: "Fetch one event by id, with the links that leave it and the links that point at it.",
	}, t.getEvent)
	mcp.AddTool(s, &mcp.Tool{
		Name:        "techlog_record_event",
		Description: "Record a structured Techlog event (draft 0.1). Do not supply an id: the server assigns it. idempotency_key is required and unique per actor: use one stable key per logical event and reuse it when retrying; a duplicate is rejected and names the existing event. Events are immutable; record a correction as a new event. On a validation error the result lists the field paths to fix.",
	}, t.recordEvent)
	mcp.AddTool(s, &mcp.Tool{
		Name:        "techlog_link_events",
		Description: "Link two existing events. basis is confirmed or hypothesis; it is required for caused_by and defaults to confirmed for other relations.",
	}, t.linkEvents)
	return s
}

// Handler serves the MCP server over streamable HTTP. Mount it at "/mcp".
func Handler(svc *service.Service) http.Handler {
	srv := New(svc)
	return mcp.NewStreamableHTTPHandler(func(*http.Request) *mcp.Server { return srv }, nil)
}

type tools struct{ svc *service.Service }

type searchIn struct {
	Query  string   `json:"query,omitempty" jsonschema:"free text matched against summary, system, component and type; words are ANDed and punctuation is plain text"`
	System string   `json:"system,omitempty" jsonschema:"only events about this system"`
	Types  []string `json:"types,omitempty" jsonschema:"only these event types, for example incident.opened"`
	Since  string   `json:"since,omitempty" jsonschema:"RFC 3339 timestamp or YYYY-MM-DD; events at or after it"`
	Until  string   `json:"until,omitempty" jsonschema:"RFC 3339 timestamp or YYYY-MM-DD; events at or before it"`
	Limit  int      `json:"limit,omitempty" jsonschema:"1 to 100, default 20"`
}

type historyIn struct {
	System       string `json:"system" jsonschema:"the system whose history to read"`
	Since        string `json:"since,omitempty" jsonschema:"RFC 3339 timestamp or YYYY-MM-DD; events at or after it"`
	Limit        int    `json:"limit,omitempty" jsonschema:"1 to 200, default 50; the latest events are returned"`
	IncludeLinks *bool  `json:"include_links,omitempty" jsonschema:"include each event's links, default true"`
}

type getIn struct {
	ID string `json:"id" jsonschema:"event id, for example evt_01J9K2A0P011"`
}

type recordIn struct {
	Event map[string]any `json:"event" jsonschema:"the event without an id: type, time, actor, subject, provenance, idempotency_key, and optionally summary, sources, evidence, links, extensions"`
}

type linkIn struct {
	Source    string `json:"source" jsonschema:"id of the later event, the one the link leaves"`
	Target    string `json:"target" jsonschema:"id of the earlier event, the one the link points at"`
	Rel       string `json:"rel" jsonschema:"relation, for example caused_by, implements, relates_to"`
	Basis     string `json:"basis,omitempty" jsonschema:"confirmed or hypothesis; required for caused_by"`
	Rationale string `json:"rationale,omitempty" jsonschema:"why the link holds"`
}

type eventsOut struct {
	Events []event.Event `json:"events"`
	Count  int           `json:"count"`
}

type recordOut struct {
	ID    string      `json:"id"`
	Event event.Event `json:"event"`
}

type linkOut struct {
	Link event.Link `json:"link"`
}

func (t *tools) searchEvents(ctx context.Context, _ *mcp.CallToolRequest, in searchIn) (*mcp.CallToolResult, eventsOut, error) {
	since, err := optTime("since", in.Since, false)
	if err != nil {
		return errorResult(err), eventsOut{}, nil
	}
	until, err := optTime("until", in.Until, true)
	if err != nil {
		return errorResult(err), eventsOut{}, nil
	}
	events, err := t.svc.Search(ctx, service.SearchParams{
		Query: in.Query, System: in.System, Types: in.Types, Since: since, Until: until, Limit: in.Limit,
	})
	if err != nil {
		return errorResult(err), eventsOut{}, nil
	}
	return textResult(formatEvents(events)), eventsOut{Events: events, Count: len(events)}, nil
}

func (t *tools) systemHistory(ctx context.Context, _ *mcp.CallToolRequest, in historyIn) (*mcp.CallToolResult, eventsOut, error) {
	since, err := optTime("since", in.Since, false)
	if err != nil {
		return errorResult(err), eventsOut{}, nil
	}
	events, err := t.svc.History(ctx, in.System, since, in.Limit)
	if err != nil {
		return errorResult(err), eventsOut{}, nil
	}
	if in.IncludeLinks != nil && !*in.IncludeLinks {
		for i := range events {
			events[i].Links = nil
		}
	}
	return textResult(formatEvents(events)), eventsOut{Events: events, Count: len(events)}, nil
}

func (t *tools) getEvent(ctx context.Context, _ *mcp.CallToolRequest, in getIn) (*mcp.CallToolResult, service.Detail, error) {
	d, err := t.svc.Get(ctx, in.ID)
	if err != nil {
		return errorResult(fmt.Errorf("event %s: %w", in.ID, err)), service.Detail{}, nil
	}
	var b strings.Builder
	b.WriteString(formatLine(d.Event))
	for _, l := range d.Incoming {
		fmt.Fprintf(&b, "\n    <- %s from %s (%s)", l.Rel, l.Source, l.Basis)
	}
	return textResult(b.String()), d, nil
}

func (t *tools) recordEvent(ctx context.Context, _ *mcp.CallToolRequest, in recordIn) (*mcp.CallToolResult, recordOut, error) {
	ev, err := t.svc.Record(ctx, in.Event, service.MethodAgent)
	if err != nil {
		return errorResult(err), recordOut{}, nil
	}
	return textResult(fmt.Sprintf("Recorded %s", ev.ID)), recordOut{ID: ev.ID, Event: ev}, nil
}

func (t *tools) linkEvents(ctx context.Context, _ *mcp.CallToolRequest, in linkIn) (*mcp.CallToolResult, linkOut, error) {
	l, err := t.svc.Link(ctx, service.LinkInput{
		Source: in.Source, Target: in.Target, Rel: in.Rel, Basis: in.Basis, Rationale: in.Rationale,
	})
	if err != nil {
		return errorResult(err), linkOut{}, nil
	}
	return textResult(fmt.Sprintf("Linked %s -[%s, %s]-> %s", l.Source, l.Rel, l.Basis, l.Target)), linkOut{Link: l}, nil
}

func optTime(field, raw string, endOfDay bool) (time.Time, error) {
	if raw == "" {
		return time.Time{}, nil
	}
	t, err := service.ParseTime(raw, endOfDay)
	if err != nil {
		return time.Time{}, &service.BadRequestError{Msg: fmt.Sprintf("invalid %s: %v", field, err)}
	}
	return t, nil
}

func textResult(s string) *mcp.CallToolResult {
	return &mcp.CallToolResult{Content: []mcp.Content{&mcp.TextContent{Text: s}}}
}

func errorResult(err error) *mcp.CallToolResult {
	return &mcp.CallToolResult{IsError: true, Content: []mcp.Content{&mcp.TextContent{Text: describe(err)}}}
}

// describe turns an error into text a model can act on.
func describe(err error) string {
	var invalid *service.InvalidError
	var bad *service.BadRequestError
	var dup *service.DuplicateError
	switch {
	case errors.As(err, &dup):
		return fmt.Sprintf("Already recorded: idempotency_key %q for actor %q is event %s. "+
			"Do not retry; link to %s if later events follow from it.",
			dup.Key, dup.ActorID, dup.ExistingID, dup.ExistingID)
	case errors.As(err, &invalid):
		var b strings.Builder
		b.WriteString("Rejected. Fix these problems and call the tool again:")
		for _, p := range invalid.Problems {
			fmt.Fprintf(&b, "\n- %s: %s", p.Path, p.Message)
		}
		return b.String()
	case errors.As(err, &bad):
		return bad.Msg
	case errors.Is(err, store.ErrNotFound):
		return err.Error()
	default:
		slog.Error("mcp: internal error", "err", err)
		return "internal error"
	}
}

func formatEvents(events []event.Event) string {
	if len(events) == 0 {
		return "No events found."
	}
	lines := make([]string, len(events))
	for i, e := range events {
		lines[i] = formatLine(e)
	}
	return fmt.Sprintf("%d event(s):\n%s", len(events), strings.Join(lines, "\n"))
}

func formatLine(e event.Event) string {
	line := fmt.Sprintf("%s  %s  %s  %s", e.ID, e.Time, e.Type, e.Subject.System)
	if e.Summary != "" {
		line += ": " + e.Summary
	}
	for _, l := range e.Links {
		line += fmt.Sprintf("\n    %s %s (%s)", l.Rel, l.Target, l.Basis)
	}
	return line
}
