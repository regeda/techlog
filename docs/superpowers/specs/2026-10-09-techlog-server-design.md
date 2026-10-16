# Techlog Server Design

Date: 2026-10-09
Status: Draft for review

## 1. Goal

Build a basic Techlog server in Go: MCP tools and an HTTP API for recording and
reading engineering events, pluggable storage (SQLite first), a simple web UI to
explore events and walk through stories, and a Claude skill that teaches Claude
to record events.

Success criteria:

- `techlog serve` starts one process that serves the UI, the HTTP API and MCP on
  one port, with defaults that need no configuration.
- Claude can connect with
  `claude mcp add --transport http techlog http://localhost:8080/mcp` and record
  and query events.
- `POST /api/v1/events` and the MCP `record_event` tool reject the same invalid
  inputs, using JSON Schema validation.
- A new storage backend can be added without touching the service, API, MCP or
  UI packages.

### Non-goals

- Security: no authentication, no signatures, no authorization. The server binds
  to `localhost` by default.
- `create_rollback_plan` and `submit_evidence` tools. Evidence is recorded in an
  event's `evidence` array.
- UI writes. The UI is read-only.
- Event update or delete. Events are immutable and append-only.
- A second storage backend (only the interface and contract tests).

### Source of truth for the event model

The website docs in `website/src/pages/docs/` (event model, lifecycle, MCP) and
`website/src/examples/json-schema-core.json` define the draft 0.1 event model:
13 event types, 9 statuses, typed links with `confirmed` or `hypothesis` basis,
and namespaced extensions.

## 2. Architecture

```
cmd/techlog/main.go          flags, wiring, graceful shutdown
internal/event/              Go types, ID generation, embedded JSON Schema
internal/store/              Store interface + shared contract tests
internal/store/sqlite/       first backend, embedded migrations
internal/service/            use cases: Record, Link, Get, Search, History
internal/api/                REST handlers (/api/v1/...)
internal/mcpserver/          MCP tools, thin wrappers over service
internal/ui/                 embedded html/template + CSS, handlers
skills/techlog-record/SKILL.md
```

Dependencies point inward. `api`, `mcpserver` and `ui` depend only on `service`.
`service` depends on the `store.Store` interface and `event`. Adding a backend
means a new `internal/store/<name>` package and a new `--store` flag value.

Libraries:

- MCP: `github.com/modelcontextprotocol/go-sdk` (streamable HTTP handler, typed
  tools).
- SQLite: `modernc.org/sqlite` (pure Go, no cgo).
- JSON Schema: `github.com/santhosh-tekuri/jsonschema/v6` (draft 2020-12).
- Logging: standard library `log/slog`.

### Store interface

```go
type Store interface {
    Insert(ctx context.Context, e event.Event) error
    AddLink(ctx context.Context, l event.Link) error
    Get(ctx context.Context, id string) (event.Event, error)
    Search(ctx context.Context, f Filter) ([]event.Event, error)
    Links(ctx context.Context, id string) ([]event.Link, error) // both directions
    Close() error
}
```

`Filter` carries free text, system, types, since, until and limit. There is no
update or delete. `internal/store` ships a contract test suite that every
backend runs; the SQLite backend runs it on an in-memory database.

### Run

`techlog serve [--addr localhost:8080] [--db share/techlog.db] [--store sqlite]` serves:

- `/` the web UI
- `/api/v1/*` the HTTP API
- `/mcp` MCP over streamable HTTP

`techlog seed` loads the Spring Spin example story so the UI has data.

## 3. Data model

SQLite schema, applied by embedded migrations:

- `events`: `id` (PK), `type`, `time`, `status`, `system`, `component`,
  `actor_kind`, `actor_id`, `summary`, `body` (full event JSON, the source of
  truth for reads). Search columns are indexed.
- `links`: `source`, `target`, `rel`, `basis`, `rationale`, `created_at`. Links
  in a submitted event and links added later via `link_events` both land here. A
  read merges them into the event's `links`. The event row is never rewritten.
- An FTS5 index over `summary`, `system` and `type` supports free-text search.

SQLite runs with WAL and a busy timeout.

## 4. Validation

Events are validated against the draft 0.1 JSON Schema, embedded with `go:embed`
at `internal/event/schema/event.json` and compiled once at startup. The schema
file is canonical; `website/src/examples/json-schema-core.json` must match it,
enforced by a test.

`service.Record` performs, in order:

1. Decode the body.
2. Apply defaults: `schema_version`, `time` if missing, `provenance.method`
   (`agent_submitted` via MCP, `human_entered` via HTTP).
3. Assign `id` (`evt_` plus 12 characters from `[0-9A-Z]`, time-sortable). A
   client-supplied `id` is ignored.
4. Validate against the JSON Schema.
5. Check rules a schema cannot express: every link target exists, a
   `caused_by` link carries an explicit `basis` (other links default to
   `confirmed`), `time` is not later than the server clock plus the allowed
   skew (`serve -clock-skew`, default 5 seconds), and `time` is not earlier
   than the time of any linked event. Times are compared as instants and
   stored as UTC with `Z`; see `2026-10-16-event-time-ordering-design.md`.
6. Insert.

Because the service does this, the HTTP API and MCP behave identically.

A schema failure returns HTTP 422:

```json
{"error": {"code": "invalid_event",
           "details": [{"path": "/actor/kind", "message": "..."}]}}
```

Paths are JSON Pointers. The MCP tool returns the same details as `isError`
text.

## 5. HTTP API (`/api/v1`)

| Method and path | Purpose |
|---|---|
| `POST /events` | Record an event |
| `GET /events/{id}` | Event with links in both directions |
| `GET /events` | Search: `q`, `system`, `type`, `since`, `until`, `limit` |
| `GET /systems/{system}/history` | Time-ordered history for one system |
| `GET /systems` | List systems (for UI filters) |
| `POST /links` | Add a link between existing events |

Error shape is the same everywhere: `{"error": {"code", "message"}}`. Codes:
`invalid_event` (422), `not_found` (404), `bad_request` (400), `internal` (500).

## 6. MCP tools (`/mcp`)

Names use underscores because some MCP clients restrict names to
`[a-zA-Z0-9_-]`. The website docs use dotted names and should be updated to
match.

- `techlog_search_events` (query, system, types, since, until, limit)
- `techlog_get_system_history` (system, since, limit, include_links)
- `techlog_get_event` (id)
- `techlog_record_event` (event)
- `techlog_link_events` (source, target, rel, basis, rationale)

Each tool returns a text block for the model and `structuredContent` for code.
Validation failures return `isError: true` with details the model can act on.

## 7. Web UI

Server-rendered with `html/template`; templates and CSS are embedded. No JS
build and no framework.

- `/` event list, newest first, with search box and filters for system, type and
  date range. Filters are GET params, so views are linkable. Rows show time, type
  badge, status, system, actor and summary.
- `/events/{id}` full event, links in both directions, sources and evidence.
  Hypothesis links look different from confirmed ones. Collapsible raw JSON.
- `/systems/{system}` vertical timeline of the system's history, with each event
  linked to causes and effects. A "walk through" action opens a story view: the
  events reachable through links, ordered by time, with a depth cap.

The UI never writes.

## 8. Claude skill

`skills/techlog-record/SKILL.md`:

- Triggers when the user finishes or decides something worth remembering:
  problem found, decision, merged change, release, incident, rollback.
- Explains connecting: `claude mcp add --transport http techlog http://localhost:8080/mcp`.
- Search before recording (`search_events`, `get_system_history`) so the new
  event links to the right earlier ones.
- Table mapping situations to event type, status and typical link `rel`.
- Rules: one event per fact; `summary` at most 280 characters; `hypothesis` for
  suspected causes and `confirmed` only for established ones; never edit past
  events, record a correction as a new event.
- Actor: `kind: ai_agent`, `id: agent:claude-code`, `on_behalf_of` the user when
  known.
- Two worked `record_event` examples and how to fix a validation error.
- Fallback when MCP is not connected: a `curl` example for
  `POST /api/v1/events`.

## 9. Errors and operations

- Structured logging with `log/slog`.
- Graceful shutdown on SIGINT and SIGTERM.
- Defaults: `localhost:8080`, `./share/techlog.db`.
- `share/` at the repository root holds all runtime data the server needs: the
  SQLite database and its WAL/SHM files, and any future local state. It is listed
  in `.gitignore`, so nothing in it is committed. The server creates the
  directory on start if it is missing. Relative `--db` paths resolve against the
  working directory, so run the server from the repository root, or pass an
  absolute path.

## 10. Testing

- Store contract suite, run against SQLite in memory.
- Service tests with a fake store: validation, link rules, ID assignment.
- API tests with `httptest`, including the 422 error shape.
- MCP tests using the go-sdk in-memory transport, one per tool.
- UI tests: pages render and include seeded events.
- Schema drift test: embedded schema equals the website copy.
- Smoke test: start the server, record an event over MCP, read it back over HTTP
  and in the UI.

## 11. Implementation amendments

Decisions made while writing the implementation plan. They refine the sections above.

- **Default address.** The default `--addr` is `localhost:8080`. A bare `:8080` listens on all interfaces, which contradicts "binds to localhost by default"; pass `--addr :8080` to expose the server.
- **Store interface.** Adds `Systems(ctx) ([]string, error)` for `GET /systems` and the UI filters. `Filter` has `Ascending bool`; the default order is newest first.
- **Contract suite location.** `internal/store/storetest`, an importable package, so every backend's tests can call it.
- **IDs.** `evt_` + 8 base-36 characters of the creation time in milliseconds + 4 random base-36 characters. On an id collision the service retries with a new id.
- **Links.** The `links` primary key is `(source, target, rel, basis)`. Adding an identical link twice is a no-op. Adding the same link with the other basis stores a second row, so a hypothesis can later be corroborated by a confirmed link without editing anything.
- **Event reads** return the event with `links` merged from the links table. `GET /events/{id}` also returns separate `outgoing` and `incoming` link lists, which carry `rationale`.
- **Link validation errors** from `link_events` and `POST /links` use the same `invalid_event` code and `details` shape.
- **MCP inputs.** `query` on `techlog_search_events` is optional. `basis` on `techlog_link_events` is optional and defaults to `confirmed`, except for `caused_by`, where it is required.
- **History** returns the latest N events in chronological order.
- **Seed data and test helpers.** `internal/seed` embeds the Spring Spin story. `internal/testutil` holds shared test helpers.

## Addendum: idempotency key

Events now require an `idempotency_key`, unique per actor. See `2026-10-09-idempotency-key-design.md`.
