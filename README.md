# Techlog

An append-only log of engineering events: what happened to a system, and what led to it. Techlog gives AI agents an MCP interface and everyone else an HTTP API and a web UI, on one port, from one binary.

The event model and the ideas behind it are documented in [`website/`](website/). This directory also holds the Go server.

## Run it

```bash
go run ./cmd/techlog seed     # optional: load the Spring Spin example story
go run ./cmd/techlog serve    # http://localhost:8080
```

- UI: <http://localhost:8080/>. Browse and filter events, open one to see its links, follow a system's timeline, or "walk through" the story around any event.
- API: `/api/v1`. For example `curl 'localhost:8080/api/v1/events?q=canary'`.
- MCP: `/mcp` (streamable HTTP).

Data lives in `share/techlog.db` (git-ignored). Flags: `--addr` (default `localhost:8080`; use `:8080` to listen on all interfaces), `--db`, `--store` (only `sqlite` so far). There is no authentication and nothing is signed, so keep it on localhost or a trusted network.

## Use it from Claude

```bash
claude mcp add --transport http techlog http://localhost:8080/mcp
ln -s "$PWD/skills/techlog-record" ~/.claude/skills/techlog-record
```

The first line connects the tools: `techlog_search_events`, `techlog_get_system_history`, `techlog_get_event`, `techlog_record_event`, `techlog_link_events`. The second installs the `techlog-record` skill, which teaches Claude when to record an event, which type to use, and how to link it to earlier events.

## HTTP API

| Route | Purpose |
|---|---|
| `POST /api/v1/events` | Record an event (the server assigns the id) |
| `GET /api/v1/events/{id}` | One event with its incoming and outgoing links |
| `GET /api/v1/events?q&system&type&since&until&limit` | Search, newest first |
| `GET /api/v1/systems` | Known systems |
| `GET /api/v1/systems/{system}/history` | Latest events of one system, oldest first |
| `POST /api/v1/links` | Link two existing events |

Events are validated against the draft 0.1 JSON Schema in `internal/event/schema/event.json`. A bad event gets a `422` whose `error.details` lists the JSON Pointer path of each problem.

## Layout

```
cmd/techlog/          the binary: serve, seed
internal/event/       event types, ids, JSON Schema validation
internal/store/       the Store interface and a contract test suite
internal/store/sqlite/  the SQLite backend (pure Go, no cgo)
internal/service/     use cases; the API, MCP and UI call only this
internal/api/ mcpserver/ ui/   the three front doors
skills/techlog-record/  the Claude skill
```

## Add a storage backend

1. Create `internal/store/<name>/` implementing `store.Store`.
2. Call `storetest.Run` from its tests. If the contract suite passes, the backend is correct.
3. Add a case for it in `storeFlags.open` in `cmd/techlog/main.go`.

## Develop

```bash
make            # list the tasks
make run        # build and serve; pass flags with ARGS="--addr :9090"
make tools      # once: install golangci-lint (pinned in the Makefile)
make check      # lint and test; run before committing
```

Also `make build`, `make seed`, `make test`, `make lint`, `make fmt`. Lint rules are in `.golangci.yml`.

The smoke test in `cmd/techlog` starts a real server on a local port.
