# Techlog Server Implementation Plan

> **For agentic workers:** REQUIRED SUB-SKILL: Use superpowers:subagent-driven-development (recommended) or superpowers:executing-plans to implement this plan task-by-task. Steps use checkbox (`- [ ]`) syntax for tracking.

**Goal:** Build a single-binary Go server that records and serves Techlog engineering events over MCP and an HTTP API, with pluggable storage (SQLite first), a read-only web UI for exploring events and walking through stories, and a Claude skill for recording events.

**Architecture:** Dependencies point inward. `api`, `mcpserver` and `ui` depend only on `service`; `service` depends on the `store.Store` interface and the `event` package; `store/sqlite` is the first backend. `service.Record` validates every event against the embedded draft 0.1 JSON Schema, so HTTP and MCP reject the same inputs. One process serves `/` (UI), `/api/v1/*` and `/mcp` on one port.

**Tech Stack:** Go 1.27, `github.com/modelcontextprotocol/go-sdk` v1.8.0, `modernc.org/sqlite` v1.60.1 (pure Go, FTS5), `github.com/santhosh-tekuri/jsonschema/v6` v6.0.3, `log/slog`, `html/template`, `embed`.

**Spec:** `docs/superpowers/specs/2026-10-09-techlog-server-design.md` (approved). Task 1 appends an "Implementation amendments" section to it; where the amendments and the original text differ, the amendments win.

**Verified before handoff:** every file this plan tells you to create was extracted into a scratch module outside the repository and run: `gofmt -l` and `go vet ./...` are clean, all 102 tests pass, and `go test -race` is clean for the store, service and cmd packages. Deliberately breaking the FTS quoting and the UTC time normalisation made the matching tests fail, so they are not vacuous. If a step's test fails when you run it, suspect your transcription before the plan.

## Global Constraints

Every task's requirements implicitly include these.

- Module path `github.com/regeda/techlog`; `go 1.27` in `go.mod` (the installed toolchain is go1.27.1).
- Dependencies are limited to the three libraries above (plus their transitive dependencies) and the standard library. Pin them with `go get <module>@<version>` and `go mod tidy`.
- One process, one port: UI at `/`, API at `/api/v1/*`, MCP at `/mcp` (streamable HTTP).
- Defaults: `--addr localhost:8080`, `--db share/techlog.db`, `--store sqlite`. `share/` is already in `.gitignore`; the server creates it if missing.
- Events follow draft 0.1: 13 types, 9 statuses, typed links with basis `confirmed` or `hypothesis`; `summary` at most 280 characters; id matches `^evt_[0-9A-Z]{12}$` and is assigned by the server (a client-supplied id is ignored on record).
- Events are immutable and append-only: no update or delete anywhere (store, service, API, MCP, UI).
- The UI is read-only. No authentication, no signatures, no authorization.
- Error codes, identical on every HTTP endpoint, shape `{"error": {"code", "message", "details"?}}`: `invalid_event` 422, `not_found` 404, `bad_request` 400, `internal` 500.
- MCP tool names use underscores: `techlog_search_events`, `techlog_get_system_history`, `techlog_get_event`, `techlog_record_event`, `techlog_link_events`. Validation failures return `isError: true` with text the model can act on; success returns a text block plus `structuredContent`.
- Limits (from `website/src/examples/mcp-tools.json`): search default 20, max 100; history default 50, max 200.
- Commits follow Conventional Commits, `<type>(<scope>): <description>`. **Never add a `Co-Authored-By` or any other attribution trailer** (the user's saved preference overrides the harness default).
- Environment notes for this machine: the sandbox blocks writes to `.git`, the Go module proxy (TLS), and binding local ports. Run `git add`/`git commit`, `go get`, `go mod tidy`, and the `cmd/techlog` smoke test with the sandbox disabled. `GOFLAGS=-mod=readonly` is set, so run `go mod tidy` after adding a new import and before running tests.

## Review Focus

Failure modes the spec implies but a straight reading of the tasks would not exercise. Each has a named test in the task that owns the code.

1. **Free-text search with punctuation or operators** (`promo-api`, `spin-service`, `"quoted`, `AND`, `(`, `a OR b`) must return matches or nothing, never a 500. Store test `SearchTextPunctuation` (Task 2) and API test `TestSearchPunctuation` (Task 4).
2. **Timestamps with non-UTC offsets** (`2026-03-11T12:00:00+02:00`) must order and filter by the instant, not by the text. Store test `TimeOffsets` (Task 2) and API test `TestSearchOffsets` (Task 4).
3. **Parallel writers** (several agents recording at once) must all succeed, with no "database is locked". Store test `ConcurrentInserts` (Task 2).
4. **Link edge cases:** a self-link, a dangling target, an identical duplicate link, and a cycle in the story walk must give a clear 422, a no-op, and a terminating walk respectively. Service tests `TestLinkRules`, `TestRecordLinkRules`, `TestStoryTerminatesOnCycles` (Task 3).
5. **Hostile or malformed input:** a non-object, trailing-data or oversized request body gives 400 (not 500), and markup in a summary is escaped in the UI. API test `TestRecordMalformedBodies` (Task 4) and UI test `TestEscaping` (Task 6).

## File Structure

```
go.mod, go.sum
cmd/techlog/main.go                  command dispatch: serve, seed
cmd/techlog/serve.go                 flags, store factory, handler wiring, request logging, graceful shutdown
cmd/techlog/seed.go                  `techlog seed`
cmd/techlog/main_test.go             command dispatch tests
cmd/techlog/smoke_test.go            end-to-end: MCP record -> HTTP read -> UI render
internal/event/event.go              Go types, Types list, ParsedTime
internal/event/id.go                 NewID
internal/event/schema.go             embedded schema, Validator, Problem/ValidationError, Normalize
internal/event/schema/event.json     canonical draft 0.1 schema (copy of the website file)
internal/event/*_test.go             id, schema, drift, skill-example tests
internal/store/store.go              Store interface, Filter, ErrNotFound, ErrConflict
internal/store/storetest/storetest.go  contract suite every backend runs
internal/store/sqlite/sqlite.go      Open, OpenMemory, migrations, all Store methods
internal/store/sqlite/fts.go         user text -> safe FTS5 query
internal/store/sqlite/migrations/001_init.sql
internal/store/sqlite/*_test.go
internal/service/service.go          New, Record, Link, Get, Import
internal/service/errors.go           InvalidError, BadRequestError
internal/service/search.go           SearchParams, Search, History, Systems, ParseTime
internal/service/story.go            Story
internal/service/*_test.go           (package service_test, with a fake store)
internal/testutil/testutil.go        NewService, Doc, Link
internal/api/api.go                  HTTP handlers + error mapping
internal/api/api_test.go
internal/mcpserver/mcpserver.go      MCP server, five tools, streamable HTTP handler
internal/mcpserver/mcpserver_test.go
internal/ui/ui.go                    handlers, template funcs
internal/ui/templates/*.html         layout, events, event, system, story
internal/ui/static/style.css
internal/ui/ui_test.go
internal/seed/seed.go                embedded Spring Spin story
internal/seed/spring_spin.json       copy of website/src/examples/timeline.json
internal/seed/seed_test.go
skills/techlog-record/SKILL.md       the Claude skill
README.md                            run, connect, install the skill, add a backend
```

---

### Task 1: Module scaffold and the `event` package

**Files:**
- Modify: `docs/superpowers/specs/2026-10-09-techlog-server-design.md`
- Create: `go.mod`, `internal/event/schema/event.json`, `internal/event/event.go`, `internal/event/id.go`, `internal/event/schema.go`
- Test: `internal/event/id_test.go`, `internal/event/schema_test.go`

**Interfaces:**
- Produces (used by every later task):
  - types `event.Event`, `Actor`, `Subject`, `Source`, `Evidence`, `LinkRef{Rel, Target, Basis string}`, `Provenance`, `Link{Source, Target, Rel, Basis, Rationale string}`
  - `event.SchemaVersion`, `event.Types []string`, `(Event).ParsedTime() (time.Time, error)`
  - `event.NewID(t time.Time, r io.Reader) (string, error)`
  - `event.SchemaJSON() []byte`
  - `event.Normalize(doc map[string]any) (map[string]any, error)`
  - `event.NewValidator() (*Validator, error)`, `(*Validator).Validate(doc map[string]any) error`, `(*Validator).ValidateLink(doc map[string]any) error`
  - `event.Problem{Path, Message string}`, `*event.ValidationError{Problems []Problem}`

- [ ] **Step 1: Amend the spec**

In `docs/superpowers/specs/2026-10-09-techlog-server-design.md` make two exact replacements, then append the new section at the end of the file.

Replace `techlog serve [--addr :8080] [--db share/techlog.db] [--store sqlite]` with `techlog serve [--addr localhost:8080] [--db share/techlog.db] [--store sqlite]`.

Replace ``- Defaults: `:8080` bound to `localhost`, `./share/techlog.db`.`` with ``- Defaults: `localhost:8080`, `./share/techlog.db`.``

Append:

```markdown

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
```

- [ ] **Step 2: Initialise the module and copy the schema**

```bash
cd /Users/regeda/code/techlog
go mod init github.com/regeda/techlog
go mod edit -go=1.27
mkdir -p internal/event/schema
cp website/src/examples/json-schema-core.json internal/event/schema/event.json
```

- [ ] **Step 3: Write the failing tests**

Create `internal/event/id_test.go`:

```go
package event_test

import (
	"bytes"
	"regexp"
	"testing"
	"time"

	"github.com/regeda/techlog/internal/event"
)

func TestNewID(t *testing.T) {
	re := regexp.MustCompile(`^evt_[0-9A-Z]{12}$`)
	t1 := time.Date(2026, 3, 10, 9, 0, 0, 0, time.UTC)
	t2 := t1.Add(time.Millisecond)

	// The earlier id gets the largest possible random tail; it must still sort first.
	a, err := event.NewID(t1, bytes.NewReader([]byte{255, 255, 255, 255}))
	if err != nil {
		t.Fatal(err)
	}
	b, err := event.NewID(t2, bytes.NewReader([]byte{0, 0, 0, 0}))
	if err != nil {
		t.Fatal(err)
	}
	for _, id := range []string{a, b} {
		if !re.MatchString(id) {
			t.Errorf("id %q does not match %s", id, re)
		}
	}
	if a >= b {
		t.Errorf("ids must sort by creation time: %s should be < %s", a, b)
	}
}

func TestNewIDShortEntropy(t *testing.T) {
	if _, err := event.NewID(time.Now(), bytes.NewReader(nil)); err == nil {
		t.Fatal("expected an error when the entropy source is empty")
	}
}
```

Create `internal/event/schema_test.go`:

```go
package event_test

import (
	"encoding/json"
	"errors"
	"os"
	"reflect"
	"slices"
	"strings"
	"testing"

	"github.com/regeda/techlog/internal/event"
)

func validDoc() map[string]any {
	return map[string]any{
		"schema_version": "0.1-draft",
		"id":             "evt_01J9K2A0P001",
		"type":           "problem.reported",
		"time":           "2026-03-09T14:00:00Z",
		"actor":          map[string]any{"kind": "human", "id": "user:t.nguyen"},
		"subject":        map[string]any{"system": "support-desk"},
		"status":         "recorded",
		"provenance":     map[string]any{"recorded_by": "user:t.nguyen", "method": "human_entered"},
	}
}

func problemPaths(err error) []string {
	var ve *event.ValidationError
	if !errors.As(err, &ve) {
		return nil
	}
	paths := make([]string, len(ve.Problems))
	for i, p := range ve.Problems {
		paths[i] = p.Path
	}
	return paths
}

func newValidator(t *testing.T) *event.Validator {
	t.Helper()
	v, err := event.NewValidator()
	if err != nil {
		t.Fatal(err)
	}
	return v
}

func TestValidate(t *testing.T) {
	v := newValidator(t)
	cases := []struct {
		name     string
		mutate   func(d map[string]any)
		wantPath string // "" means the document must be valid
	}{
		{"valid", func(d map[string]any) {}, ""},
		{"offset time is valid", func(d map[string]any) { d["time"] = "2026-03-11T12:00:00+02:00" }, ""},
		{"integer extension values are valid", func(d map[string]any) {
			d["extensions"] = map[string]any{"org.techlog.code": map[string]any{"files_changed": 5}}
		}, ""},
		{"bad actor kind", func(d map[string]any) { d["actor"].(map[string]any)["kind"] = "robot" }, "/actor/kind"},
		{"missing subject", func(d map[string]any) { delete(d, "subject") }, "/"},
		{"time without zone", func(d map[string]any) { d["time"] = "2026-03-11T10:12:00" }, "/time"},
		{"summary too long", func(d map[string]any) { d["summary"] = strings.Repeat("x", 281) }, "/summary"},
		{"unknown top-level field", func(d map[string]any) { d["foo"] = "bar" }, "/"},
		{"bad link rel", func(d map[string]any) {
			d["links"] = []any{map[string]any{"rel": "nope", "target": "evt_01J9K2A0P002", "basis": "confirmed"}}
		}, "/links/0/rel"},
		{"bad evidence digest", func(d map[string]any) {
			d["evidence"] = []any{map[string]any{"kind": "report", "ref": "ci://1", "digest": "sha256:abc"}}
		}, "/evidence/0/digest"},
	}
	for _, tc := range cases {
		t.Run(tc.name, func(t *testing.T) {
			d := validDoc()
			tc.mutate(d)
			err := v.Validate(d)
			if tc.wantPath == "" {
				if err != nil {
					t.Fatalf("expected valid, got %v", err)
				}
				return
			}
			if !slices.Contains(problemPaths(err), tc.wantPath) {
				t.Fatalf("expected a problem at %q, got paths %v (err: %v)", tc.wantPath, problemPaths(err), err)
			}
		})
	}
}

func TestValidateRejectsBadExtensionKey(t *testing.T) {
	d := validDoc()
	d["extensions"] = map[string]any{"Bad Key": map[string]any{}}
	if err := newValidator(t).Validate(d); err == nil {
		t.Fatal("expected an error for a non-namespaced extension key")
	}
}

func TestValidateDoesNotMutateInput(t *testing.T) {
	d := validDoc()
	d["extensions"] = map[string]any{"org.techlog.x": map[string]any{"n": 1}}
	if err := newValidator(t).Validate(d); err != nil {
		t.Fatal(err)
	}
	if _, ok := d["extensions"].(map[string]any)["org.techlog.x"].(map[string]any)["n"].(int); !ok {
		t.Fatal("Validate must not rewrite the caller's document")
	}
}

func TestValidateLink(t *testing.T) {
	v := newValidator(t)
	ok := map[string]any{"rel": "relates_to", "target": "evt_01J9K2A0P002", "basis": "confirmed"}
	if err := v.ValidateLink(ok); err != nil {
		t.Fatalf("expected valid link, got %v", err)
	}
	if err := v.ValidateLink(map[string]any{"rel": "relates_to", "target": "evt_01J9K2A0P002"}); err == nil {
		t.Error("a link without basis must be rejected")
	}
	err := v.ValidateLink(map[string]any{"rel": "relates_to", "target": "nope", "basis": "confirmed"})
	if !slices.Contains(problemPaths(err), "/target") {
		t.Errorf("expected a problem at /target, got %v", problemPaths(err))
	}
}

func TestNormalizeNilDocument(t *testing.T) {
	out, err := event.Normalize(nil)
	if err != nil || out == nil {
		t.Fatalf("Normalize(nil) = %v, %v; want an empty non-nil map", out, err)
	}
}

func TestTypesMatchSchema(t *testing.T) {
	var s struct {
		Properties struct {
			Type struct {
				Enum []string `json:"enum"`
			} `json:"type"`
		} `json:"properties"`
	}
	if err := json.Unmarshal(event.SchemaJSON(), &s); err != nil {
		t.Fatal(err)
	}
	if !slices.Equal(s.Properties.Type.Enum, event.Types) {
		t.Fatalf("event.Types is out of sync with the schema enum:\n got %v\nwant %v", event.Types, s.Properties.Type.Enum)
	}
}

func TestSchemaMatchesWebsiteCopy(t *testing.T) {
	site, err := os.ReadFile("../../website/src/examples/json-schema-core.json")
	if err != nil {
		t.Fatal(err)
	}
	var a, b any
	if err := json.Unmarshal(event.SchemaJSON(), &a); err != nil {
		t.Fatal(err)
	}
	if err := json.Unmarshal(site, &b); err != nil {
		t.Fatal(err)
	}
	if !reflect.DeepEqual(a, b) {
		t.Fatal("internal/event/schema/event.json and website/src/examples/json-schema-core.json differ; keep them identical")
	}
}
```

- [ ] **Step 4: Run the tests to verify they fail**

Run: `go test ./internal/event/ 2>&1 | head`
Expected: FAIL to compile with `undefined: event.NewID` (and the other new names).

- [ ] **Step 5: Write the implementation**

Create `internal/event/event.go`:

```go
// Package event holds the Techlog event model (draft 0.1): Go types, id
// generation and JSON Schema validation. It depends on nothing else in the
// module.
package event

import "time"

// SchemaVersion is the only event schema version this server accepts.
const SchemaVersion = "0.1-draft"

// Types lists the 13 event types, in lifecycle order. A test keeps it equal to
// the enum in the embedded schema.
var Types = []string{
	"problem.reported",
	"investigation.completed",
	"proposal.recorded",
	"decision.made",
	"change.implemented",
	"verification.completed",
	"release.deployed",
	"incident.opened",
	"mitigation.applied",
	"rollback.executed",
	"recovery.verified",
	"postmortem.published",
	"action.tracked",
}

type Actor struct {
	Kind       string `json:"kind"`
	ID         string `json:"id"`
	OnBehalfOf string `json:"on_behalf_of,omitempty"`
}

type Subject struct {
	System      string `json:"system"`
	Component   string `json:"component,omitempty"`
	Environment string `json:"environment,omitempty"`
}

type Source struct {
	Kind string `json:"kind"`
	Ref  string `json:"ref"`
}

type Evidence struct {
	Kind   string `json:"kind"`
	Ref    string `json:"ref"`
	Digest string `json:"digest,omitempty"`
}

// LinkRef is a link as it appears inside an event: it points at an earlier event.
type LinkRef struct {
	Rel    string `json:"rel"`
	Target string `json:"target"`
	Basis  string `json:"basis"`
}

type Provenance struct {
	RecordedBy string `json:"recorded_by"`
	Method     string `json:"method"`
	Confidence string `json:"confidence,omitempty"`
}

// Event is one immutable engineering event. Time is kept as the submitted
// RFC 3339 string; use ParsedTime for comparisons.
type Event struct {
	SchemaVersion string         `json:"schema_version"`
	ID            string         `json:"id"`
	Type          string         `json:"type"`
	Time          string         `json:"time"`
	Actor         Actor          `json:"actor"`
	Subject       Subject        `json:"subject"`
	Status        string         `json:"status"`
	Summary       string         `json:"summary,omitempty"`
	Sources       []Source       `json:"sources,omitempty"`
	Evidence      []Evidence     `json:"evidence,omitempty"`
	Links         []LinkRef      `json:"links,omitempty"`
	Provenance    Provenance     `json:"provenance"`
	Extensions    map[string]any `json:"extensions,omitempty"`
}

// ParsedTime parses Time as RFC 3339.
func (e Event) ParsedTime() (time.Time, error) {
	return time.Parse(time.RFC3339, e.Time)
}

// Link is a stored link between two events. Rationale is only kept for links
// added after the fact; it is not part of the event schema.
type Link struct {
	Source    string `json:"source"`
	Target    string `json:"target"`
	Rel       string `json:"rel"`
	Basis     string `json:"basis"`
	Rationale string `json:"rationale,omitempty"`
}
```

Create `internal/event/id.go`:

```go
package event

import (
	"fmt"
	"io"
	"time"
)

const idAlphabet = "0123456789ABCDEFGHIJKLMNOPQRSTUVWXYZ"

// NewID returns "evt_" followed by 12 characters from [0-9A-Z]: eight base-36
// characters of t in Unix milliseconds (so ids sort by creation time) and four
// random base-36 characters read from r.
func NewID(t time.Time, r io.Reader) (string, error) {
	var b [12]byte
	ms := uint64(t.UnixMilli())
	for i := 7; i >= 0; i-- {
		b[i] = idAlphabet[ms%36]
		ms /= 36
	}
	var rnd [4]byte
	if _, err := io.ReadFull(r, rnd[:]); err != nil {
		return "", fmt.Errorf("read id entropy: %w", err)
	}
	for i, v := range rnd {
		b[8+i] = idAlphabet[int(v)%36]
	}
	return "evt_" + string(b[:]), nil
}
```

Create `internal/event/schema.go`:

```go
package event

import (
	"bytes"
	_ "embed"
	"encoding/json"
	"errors"
	"fmt"
	"strings"

	"github.com/santhosh-tekuri/jsonschema/v6"
)

//go:embed schema/event.json
var schemaJSON []byte

// schemaURL is the $id declared inside schema/event.json.
const schemaURL = "https://techlog.example.invalid/schema/0.1-draft/event.json"

// SchemaJSON returns a copy of the embedded draft 0.1 event schema.
func SchemaJSON() []byte { return bytes.Clone(schemaJSON) }

// Problem is one validation failure. Path is a JSON Pointer into the document
// ("/" for the document itself).
type Problem struct {
	Path    string `json:"path"`
	Message string `json:"message"`
}

// ValidationError lists every problem found in a document.
type ValidationError struct {
	Problems []Problem
}

func (e *ValidationError) Error() string {
	parts := make([]string, len(e.Problems))
	for i, p := range e.Problems {
		parts[i] = p.Path + ": " + p.Message
	}
	return "invalid: " + strings.Join(parts, "; ")
}

// Validator checks documents against the embedded schema. It is safe for
// concurrent use.
type Validator struct {
	event *jsonschema.Schema
	link  *jsonschema.Schema
}

// NewValidator compiles the embedded schema once.
func NewValidator() (*Validator, error) {
	doc, err := jsonschema.UnmarshalJSON(bytes.NewReader(schemaJSON))
	if err != nil {
		return nil, fmt.Errorf("parse embedded schema: %w", err)
	}
	c := jsonschema.NewCompiler()
	c.DefaultDraft(jsonschema.Draft2020)
	c.AssertFormat() // enforce "date-time"; formats are annotations by default
	if err := c.AddResource(schemaURL, doc); err != nil {
		return nil, fmt.Errorf("add schema: %w", err)
	}
	ev, err := c.Compile(schemaURL)
	if err != nil {
		return nil, fmt.Errorf("compile event schema: %w", err)
	}
	link, err := c.Compile(schemaURL + "#/$defs/link")
	if err != nil {
		return nil, fmt.Errorf("compile link schema: %w", err)
	}
	return &Validator{event: ev, link: link}, nil
}

// Validate checks an event document. The error is a *ValidationError.
func (v *Validator) Validate(doc map[string]any) error {
	return validate(v.event, doc)
}

// ValidateLink checks one link ({rel, target, basis}).
func (v *Validator) ValidateLink(doc map[string]any) error {
	return validate(v.link, doc)
}

func validate(s *jsonschema.Schema, doc map[string]any) error {
	inst, err := Normalize(doc)
	if err != nil {
		return err
	}
	err = s.Validate(inst)
	if err == nil {
		return nil
	}
	var ve *jsonschema.ValidationError
	if !errors.As(err, &ve) {
		return err
	}
	var problems []Problem
	seen := map[Problem]bool{}
	leafProblems(ve.DetailedOutput(), func(p Problem) {
		if !seen[p] {
			seen[p] = true
			problems = append(problems, p)
		}
	})
	if len(problems) == 0 {
		problems = []Problem{{Path: "/", Message: err.Error()}}
	}
	return &ValidationError{Problems: problems}
}

// leafProblems reports the innermost failures of an output tree. The outer
// units of a failure reached through "$ref" only say "validation failed"; the
// leaves carry the real message, such as "value must be one of 'human', ...".
func leafProblems(u *jsonschema.OutputUnit, add func(Problem)) {
	if len(u.Errors) == 0 {
		if u.Error != nil {
			path := u.InstanceLocation
			if path == "" {
				path = "/"
			}
			add(Problem{Path: path, Message: fmt.Sprint(u.Error)})
		}
		return
	}
	for i := range u.Errors {
		leafProblems(&u.Errors[i], add)
	}
}

// Normalize returns a deep copy of doc made only of JSON types (maps, slices,
// strings, json.Number, bools, nil), so callers may pass Go ints and the
// validator never sees an unsupported type. A nil doc becomes an empty map.
func Normalize(doc map[string]any) (map[string]any, error) {
	raw, err := json.Marshal(doc)
	if err != nil {
		return nil, err
	}
	dec := json.NewDecoder(bytes.NewReader(raw))
	dec.UseNumber()
	var out map[string]any
	if err := dec.Decode(&out); err != nil {
		return nil, err
	}
	if out == nil {
		out = map[string]any{}
	}
	return out, nil
}
```

- [ ] **Step 6: Tidy and run the tests**

Run (sandbox disabled, for the module proxy):
```bash
go get github.com/santhosh-tekuri/jsonschema/v6@v6.0.3 && go mod tidy
go test ./internal/event/ -v 2>&1 | tail -40
```
Expected: PASS for all tests. These paths and messages were checked against the real schema while writing this plan: `/actor/kind` ("value must be one of 'human', 'ai_agent', 'system'"), `/links/0` ("missing property 'basis'"), `/time`, `/summary`, `/evidence/0/digest`, and `/` for a missing or unknown top-level property. The enum and pattern errors sit behind `$ref`, which is why `validate` walks to the leaves of `DetailedOutput()` instead of using `BasicOutput()` (that one reports a bare "validation failed").

- [ ] **Step 7: Commit**

```bash
git add go.mod go.sum internal/event docs/superpowers/specs/2026-10-09-techlog-server-design.md
git commit -m "feat(event): add event model, id generation and schema validation"
```

---
### Task 2: Store interface, contract suite and SQLite backend

**Files:**
- Create: `internal/store/store.go`, `internal/store/storetest/storetest.go`
- Create: `internal/store/sqlite/sqlite.go`, `internal/store/sqlite/fts.go`, `internal/store/sqlite/migrations/001_init.sql`
- Test: `internal/store/sqlite/sqlite_test.go`, `internal/store/sqlite/fts_test.go`

**Interfaces:**
- Consumes: `event.Event`, `event.Link`, `event.LinkRef`, `(Event).ParsedTime()` from Task 1.
- Produces:
  - `store.ErrNotFound`, `store.ErrConflict`
  - `store.Filter{Text, System string; Types []string; Since, Until time.Time; Limit int; Ascending bool}` (zero `Since`/`Until` = unbounded; `Limit <= 0` means 100)
  - `store.Store` with `Insert(ctx, event.Event) error`, `AddLink(ctx, event.Link) error`, `Get(ctx, id string) (event.Event, error)`, `Search(ctx, Filter) ([]event.Event, error)`, `Links(ctx, id string) ([]event.Link, error)` (both directions), `Systems(ctx) ([]string, error)`, `Close() error`
  - Behaviour every backend must keep: `Insert` of an existing id returns `ErrConflict`; a link whose source or target does not exist returns `ErrNotFound` and stores nothing; reads return an event with `Links` merged from the links table (nil when none); events are never modified.
  - `storetest.Run(t *testing.T, newStore func(t *testing.T) store.Store)`
  - `sqlite.Open(path string) (*sqlite.Store, error)` (creates the parent directory), `sqlite.OpenMemory() (*sqlite.Store, error)` (private, unique in-memory database)

- [ ] **Step 1: Write the store interface**

Create `internal/store/store.go`:

```go
// Package store defines the storage contract for Techlog events. Backends live
// in subpackages and are verified by the suite in package storetest.
package store

import (
	"context"
	"errors"
	"time"

	"github.com/regeda/techlog/internal/event"
)

var (
	// ErrNotFound means an event (or a link endpoint) does not exist.
	ErrNotFound = errors.New("not found")
	// ErrConflict means an event with that id already exists.
	ErrConflict = errors.New("conflict")
)

// Filter selects events. Zero values mean "no constraint", except Limit, where
// a value <= 0 means 100. Results are newest first unless Ascending is set.
type Filter struct {
	Text      string
	System    string
	Types     []string
	Since     time.Time // inclusive
	Until     time.Time // inclusive
	Limit     int
	Ascending bool
}

// Store persists immutable events and the links between them. There is no
// update or delete.
type Store interface {
	// Insert stores an event and the links in e.Links. It returns ErrConflict
	// if the id exists and ErrNotFound if a link target does not exist; in
	// both cases nothing is stored.
	Insert(ctx context.Context, e event.Event) error
	// AddLink stores a link between two existing events. Adding an identical
	// link again is a no-op. It returns ErrNotFound if either end is missing.
	AddLink(ctx context.Context, l event.Link) error
	// Get returns one event with its outgoing links merged into Links.
	Get(ctx context.Context, id string) (event.Event, error)
	Search(ctx context.Context, f Filter) ([]event.Event, error)
	// Links returns every link where id is the source or the target.
	Links(ctx context.Context, id string) ([]event.Link, error)
	// Systems returns the distinct subject systems, sorted.
	Systems(ctx context.Context) ([]string, error)
	Close() error
}
```

- [ ] **Step 2: Write the contract suite**

Create `internal/store/storetest/storetest.go`:

```go
// Package storetest is the contract suite every store backend must pass.
//
//	func TestContract(t *testing.T) {
//		storetest.Run(t, func(t *testing.T) store.Store { ... fresh, empty store ... })
//	}
package storetest

import (
	"context"
	"errors"
	"fmt"
	"reflect"
	"slices"
	"sync"
	"testing"
	"time"

	"github.com/regeda/techlog/internal/event"
	"github.com/regeda/techlog/internal/store"
)

var ctx = context.Background()

// Run executes the suite. newStore must return a fresh, empty store each call.
func Run(t *testing.T, newStore func(t *testing.T) store.Store) {
	t.Helper()
	cases := map[string]func(*testing.T, store.Store){
		"InsertGetRoundTrip":    insertGetRoundTrip,
		"GetMissing":            getMissing,
		"InsertDuplicate":       insertDuplicate,
		"InsertWithLinks":       insertWithLinks,
		"InsertDanglingLink":    insertDanglingLink,
		"AddLink":               addLink,
		"SearchFilters":         searchFilters,
		"SearchTextPunctuation": searchTextPunctuation,
		"Systems":               systems,
		"TimeOffsets":           timeOffsets,
		"ConcurrentInserts":     concurrentInserts,
	}
	for name, fn := range cases {
		t.Run(name, func(t *testing.T) { fn(t, newStore(t)) })
	}
}

func mk(id, typ, system, ts, summary string, links ...event.LinkRef) event.Event {
	return event.Event{
		SchemaVersion: event.SchemaVersion,
		ID:            id,
		Type:          typ,
		Time:          ts,
		Actor:         event.Actor{Kind: "human", ID: "user:a"},
		Subject:       event.Subject{System: system},
		Status:        "recorded",
		Summary:       summary,
		Links:         links,
		Provenance:    event.Provenance{RecordedBy: "user:a", Method: "human_entered"},
	}
}

func ref(rel, target, basis string) event.LinkRef {
	return event.LinkRef{Rel: rel, Target: target, Basis: basis}
}

func insert(t *testing.T, s store.Store, evs ...event.Event) {
	t.Helper()
	for _, e := range evs {
		if err := s.Insert(ctx, e); err != nil {
			t.Fatalf("insert %s: %v", e.ID, err)
		}
	}
}

func ids(evs []event.Event) []string {
	out := make([]string, len(evs))
	for i, e := range evs {
		out[i] = e.ID
	}
	return out
}

func linkKeys(links []event.Link) []string {
	out := make([]string, len(links))
	for i, l := range links {
		out[i] = l.Source + "|" + l.Target + "|" + l.Rel + "|" + l.Basis
	}
	slices.Sort(out)
	return out
}

func search(t *testing.T, s store.Store, f store.Filter) []string {
	t.Helper()
	got, err := s.Search(ctx, f)
	if err != nil {
		t.Fatalf("search %+v: %v", f, err)
	}
	return ids(got)
}

func expectIDs(t *testing.T, name string, got []string, want ...string) {
	t.Helper()
	if len(got) == 0 && len(want) == 0 {
		return
	}
	if !slices.Equal(got, want) {
		t.Errorf("%s: got %v, want %v", name, got, want)
	}
}

func insertGetRoundTrip(t *testing.T, s store.Store) {
	e := mk("evt_00000000000A", "problem.reported", "alpha", "2026-03-10T09:00:00Z", "first")
	e.Sources = []event.Source{{Kind: "ticket", Ref: "SUP-1"}}
	e.Extensions = map[string]any{"org.techlog.x": map[string]any{"k": "v"}}
	insert(t, s, e)
	got, err := s.Get(ctx, e.ID)
	if err != nil {
		t.Fatal(err)
	}
	if !reflect.DeepEqual(got, e) {
		t.Fatalf("round trip mismatch:\n got %+v\nwant %+v", got, e)
	}
}

func getMissing(t *testing.T, s store.Store) {
	if _, err := s.Get(ctx, "evt_MISSING00000"); !errors.Is(err, store.ErrNotFound) {
		t.Fatalf("got %v, want ErrNotFound", err)
	}
}

func insertDuplicate(t *testing.T, s store.Store) {
	e := mk("evt_00000000000A", "problem.reported", "alpha", "2026-03-10T09:00:00Z", "first")
	insert(t, s, e)
	if err := s.Insert(ctx, e); !errors.Is(err, store.ErrConflict) {
		t.Fatalf("got %v, want ErrConflict", err)
	}
}

func insertWithLinks(t *testing.T, s store.Store) {
	a := mk("evt_00000000000A", "problem.reported", "alpha", "2026-03-10T09:00:00Z", "a")
	b := mk("evt_00000000000B", "proposal.recorded", "alpha", "2026-03-10T10:00:00Z", "b",
		ref("motivated_by", a.ID, "confirmed"))
	c := mk("evt_00000000000C", "incident.opened", "alpha", "2026-03-10T11:00:00Z", "c",
		ref("caused_by", b.ID, "hypothesis"))
	insert(t, s, a, b, c)

	links, err := s.Links(ctx, b.ID)
	if err != nil {
		t.Fatal(err)
	}
	want := []string{
		"evt_00000000000B|evt_00000000000A|motivated_by|confirmed",
		"evt_00000000000C|evt_00000000000B|caused_by|hypothesis",
	}
	if !slices.Equal(linkKeys(links), want) {
		t.Fatalf("Links(b) = %v, want %v", linkKeys(links), want)
	}

	got, err := s.Get(ctx, b.ID)
	if err != nil {
		t.Fatal(err)
	}
	if !reflect.DeepEqual(got.Links, []event.LinkRef{ref("motivated_by", a.ID, "confirmed")}) {
		t.Fatalf("Get(b).Links = %+v", got.Links)
	}
}

func insertDanglingLink(t *testing.T, s store.Store) {
	e := mk("evt_00000000000B", "proposal.recorded", "alpha", "2026-03-10T10:00:00Z", "b",
		ref("motivated_by", "evt_DOESNOTEXIST", "confirmed"))
	if err := s.Insert(ctx, e); !errors.Is(err, store.ErrNotFound) {
		t.Fatalf("got %v, want ErrNotFound", err)
	}
	if _, err := s.Get(ctx, e.ID); !errors.Is(err, store.ErrNotFound) {
		t.Fatalf("a rejected insert must store nothing; Get returned %v", err)
	}
}

func addLink(t *testing.T, s store.Store) {
	a := mk("evt_00000000000A", "incident.opened", "alpha", "2026-03-10T09:00:00Z", "a")
	b := mk("evt_00000000000B", "investigation.completed", "alpha", "2026-03-10T10:00:00Z", "b")
	insert(t, s, a, b)

	hyp := event.Link{Source: b.ID, Target: a.ID, Rel: "caused_by", Basis: "hypothesis", Rationale: "maybe"}
	if err := s.AddLink(ctx, hyp); err != nil {
		t.Fatal(err)
	}
	if err := s.AddLink(ctx, hyp); err != nil { // identical: no-op
		t.Fatalf("duplicate AddLink: %v", err)
	}
	conf := event.Link{Source: b.ID, Target: a.ID, Rel: "caused_by", Basis: "confirmed"}
	if err := s.AddLink(ctx, conf); err != nil { // same triple, other basis: second row
		t.Fatal(err)
	}

	links, err := s.Links(ctx, a.ID)
	if err != nil {
		t.Fatal(err)
	}
	want := []string{
		"evt_00000000000B|evt_00000000000A|caused_by|confirmed",
		"evt_00000000000B|evt_00000000000A|caused_by|hypothesis",
	}
	if !slices.Equal(linkKeys(links), want) {
		t.Fatalf("Links(a) = %v, want %v", linkKeys(links), want)
	}
	for _, l := range links {
		if l.Basis == "hypothesis" && l.Rationale != "maybe" {
			t.Errorf("rationale lost: %+v", l)
		}
	}

	got, err := s.Get(ctx, b.ID)
	if err != nil {
		t.Fatal(err)
	}
	if len(got.Links) != 2 {
		t.Fatalf("Get(b).Links = %+v, want both added links merged in", got.Links)
	}

	missing := event.Link{Source: b.ID, Target: "evt_DOESNOTEXIST", Rel: "relates_to", Basis: "confirmed"}
	if err := s.AddLink(ctx, missing); !errors.Is(err, store.ErrNotFound) {
		t.Fatalf("got %v, want ErrNotFound", err)
	}
}

func searchFilters(t *testing.T, s store.Store) {
	e1 := mk("evt_00000000000A", "problem.reported", "alpha", "2026-03-10T09:00:00Z", "flat signups")
	e2 := mk("evt_00000000000B", "proposal.recorded", "alpha", "2026-03-10T11:30:00Z", "spin proposal")
	e3 := mk("evt_00000000000C", "incident.opened", "beta", "2026-03-12T13:38:00Z", "canary errors")
	insert(t, s, e1, e2, e3)

	expectIDs(t, "default newest first", search(t, s, store.Filter{}), e3.ID, e2.ID, e1.ID)
	expectIDs(t, "ascending", search(t, s, store.Filter{Ascending: true}), e1.ID, e2.ID, e3.ID)
	expectIDs(t, "system", search(t, s, store.Filter{System: "alpha"}), e2.ID, e1.ID)
	expectIDs(t, "types", search(t, s, store.Filter{Types: []string{"incident.opened", "problem.reported"}}), e3.ID, e1.ID)
	since := time.Date(2026, 3, 10, 10, 0, 0, 0, time.UTC)
	expectIDs(t, "since", search(t, s, store.Filter{Since: since}), e3.ID, e2.ID)
	until := time.Date(2026, 3, 10, 11, 30, 0, 0, time.UTC)
	expectIDs(t, "until is inclusive", search(t, s, store.Filter{Until: until}), e2.ID, e1.ID)
	expectIDs(t, "limit", search(t, s, store.Filter{Limit: 2}), e3.ID, e2.ID)
	expectIDs(t, "text", search(t, s, store.Filter{Text: "spin"}), e2.ID)
	expectIDs(t, "text is case-insensitive and ANDed", search(t, s, store.Filter{Text: "SPIN proposal"}), e2.ID)
	expectIDs(t, "text prefix", search(t, s, store.Filter{Text: "sp"}), e2.ID)
	expectIDs(t, "text with system filter", search(t, s, store.Filter{Text: "canary", System: "beta"}), e3.ID)
	expectIDs(t, "text matches the type", search(t, s, store.Filter{Text: "incident"}), e3.ID)
	expectIDs(t, "no match", search(t, s, store.Filter{Text: "nomatch"}))
}

func searchTextPunctuation(t *testing.T, s store.Store) {
	e := mk("evt_00000000000A", "incident.opened", "promo-api", "2026-03-12T13:38:00Z", `5xx ratio "high"`)
	e.Subject.Component = "spin-service"
	insert(t, s, e)

	expectIDs(t, "hyphenated system", search(t, s, store.Filter{Text: "promo-api"}), e.ID)
	expectIDs(t, "hyphenated component", search(t, s, store.Filter{Text: "spin-service"}), e.ID)
	expectIDs(t, "leading digit token", search(t, s, store.Filter{Text: "5xx"}), e.ID)

	// None of these may be an error; what they match is not specified.
	for _, q := range []string{`"quoted`, `AND`, `OR`, `NOT`, `spin*`, `(`, `)`, `-`, `:`, `'`,
		`NEAR(a b)`, `col:val`, `a OR b`, `"`, `""`, `^x`, `x"y`, `{`, `}`} {
		if _, err := s.Search(ctx, store.Filter{Text: q}); err != nil {
			t.Errorf("Search(%q) returned an error: %v", q, err)
		}
	}
}

func systems(t *testing.T, s store.Store) {
	got, err := s.Systems(ctx)
	if err != nil || len(got) != 0 {
		t.Fatalf("empty store: Systems = %v, %v", got, err)
	}
	insert(t, s,
		mk("evt_00000000000A", "problem.reported", "beta", "2026-03-10T09:00:00Z", "x"),
		mk("evt_00000000000B", "problem.reported", "alpha", "2026-03-10T10:00:00Z", "y"),
		mk("evt_00000000000C", "problem.reported", "beta", "2026-03-10T11:00:00Z", "z"),
	)
	got, err = s.Systems(ctx)
	if err != nil {
		t.Fatal(err)
	}
	if !slices.Equal(got, []string{"alpha", "beta"}) {
		t.Fatalf("Systems = %v", got)
	}
}

func timeOffsets(t *testing.T, s store.Store) {
	// Instants: x = 10:00Z, y = 10:30Z, z = 11:00Z. As text, x ("12:00") sorts last.
	x := mk("evt_00000000000X", "problem.reported", "alpha", "2026-03-11T12:00:00+02:00", "x")
	y := mk("evt_00000000000Y", "problem.reported", "alpha", "2026-03-11T10:30:00Z", "y")
	z := mk("evt_00000000000Z", "problem.reported", "alpha", "2026-03-11T09:00:00-02:00", "z")
	insert(t, s, z, x, y)

	expectIDs(t, "ordered by instant", search(t, s, store.Filter{Ascending: true}), x.ID, y.ID, z.ID)
	since := time.Date(2026, 3, 11, 10, 15, 0, 0, time.UTC)
	expectIDs(t, "since by instant", search(t, s, store.Filter{Since: since, Ascending: true}), y.ID, z.ID)
	until := time.Date(2026, 3, 11, 10, 30, 0, 0, time.UTC)
	expectIDs(t, "until by instant", search(t, s, store.Filter{Until: until, Ascending: true}), x.ID, y.ID)
}

func concurrentInserts(t *testing.T, s store.Store) {
	const n = 20
	var wg sync.WaitGroup
	errs := make(chan error, n)
	for i := 0; i < n; i++ {
		wg.Add(1)
		go func(i int) {
			defer wg.Done()
			e := mk(fmt.Sprintf("evt_C%011d", i), "problem.reported", "alpha", "2026-03-10T09:00:00Z", "c")
			errs <- s.Insert(ctx, e)
		}(i)
	}
	wg.Wait()
	close(errs)
	for err := range errs {
		if err != nil {
			t.Errorf("concurrent insert failed: %v", err)
		}
	}
	if got := search(t, s, store.Filter{Limit: 100}); len(got) != n {
		t.Fatalf("stored %d events, want %d", len(got), n)
	}
}
```

- [ ] **Step 3: Write the SQLite tests**

Create `internal/store/sqlite/sqlite_test.go`:

```go
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
```

Create `internal/store/sqlite/fts_test.go`:

```go
package sqlite

import "testing"

func TestFTSQuery(t *testing.T) {
	cases := map[string]string{
		"":                 "",
		"   ":              "",
		"spin":             `"spin"*`,
		"promo-api spin":   `"promo-api"* "spin"*`,
		"(":                "",
		"- ( )":            "",
		`say "hi"`:         `"say"* """hi"""*`,
		"AND":              `"AND"*`,
		"incident (opened": `"incident"* "(opened"*`,
	}
	for in, want := range cases {
		if got := ftsQuery(in); got != want {
			t.Errorf("ftsQuery(%q) = %q, want %q", in, got, want)
		}
	}
}
```

- [ ] **Step 4: Run the tests to verify they fail**

Run: `go vet ./internal/store/... 2>&1 | head`
Expected: FAIL with `no required module provides package .../internal/store/sqlite` or `undefined: sqlite.Open`.

- [ ] **Step 5: Write the migration**

Create `internal/store/sqlite/migrations/001_init.sql`:

```sql
CREATE TABLE events (
    id         TEXT PRIMARY KEY,
    type       TEXT NOT NULL,
    time       TEXT NOT NULL,            -- normalised UTC, fixed width: sorts as text
    status     TEXT NOT NULL,
    system     TEXT NOT NULL,
    component  TEXT NOT NULL DEFAULT '',
    actor_kind TEXT NOT NULL,
    actor_id   TEXT NOT NULL,
    summary    TEXT NOT NULL DEFAULT '',
    body       TEXT NOT NULL             -- the full event JSON; the source of truth for reads
);
CREATE INDEX events_time        ON events (time);
CREATE INDEX events_system_time ON events (system, time);
CREATE INDEX events_type        ON events (type);

CREATE TABLE links (
    source     TEXT NOT NULL,
    target     TEXT NOT NULL,
    rel        TEXT NOT NULL,
    basis      TEXT NOT NULL,
    rationale  TEXT NOT NULL DEFAULT '',
    created_at TEXT NOT NULL,
    PRIMARY KEY (source, target, rel, basis)
);
CREATE INDEX links_target ON links (target);

CREATE VIRTUAL TABLE events_fts USING fts5 (id UNINDEXED, summary, system, component, type);
```

- [ ] **Step 6: Write the FTS query builder**

Create `internal/store/sqlite/fts.go`:

```go
package sqlite

import (
	"strings"
	"unicode"
)

// ftsQuery turns free text typed by a person into a safe FTS5 MATCH expression.
// Each word becomes a quoted prefix phrase and the phrases are ANDed, so
// operators, column filters and punctuation in the input are plain text.
// Words with no letter or digit are dropped; an input with none returns "".
func ftsQuery(text string) string {
	var parts []string
	for _, w := range strings.Fields(text) {
		if !strings.ContainsFunc(w, func(r rune) bool { return unicode.IsLetter(r) || unicode.IsDigit(r) }) {
			continue
		}
		parts = append(parts, `"`+strings.ReplaceAll(w, `"`, `""`)+`"*`)
	}
	return strings.Join(parts, " ")
}
```

- [ ] **Step 7: Write the SQLite store**

Create `internal/store/sqlite/sqlite.go`:

```go
// Package sqlite is the SQLite storage backend, built on the pure-Go
// modernc.org/sqlite driver (no cgo).
package sqlite

import (
	"context"
	"database/sql"
	"embed"
	"encoding/json"
	"errors"
	"fmt"
	"os"
	"path/filepath"
	"strings"
	"sync/atomic"
	"time"

	_ "modernc.org/sqlite"

	"github.com/regeda/techlog/internal/event"
	"github.com/regeda/techlog/internal/store"
)

//go:embed migrations/*.sql
var migrationsFS embed.FS

const (
	pragmas = "_pragma=busy_timeout(5000)&_pragma=journal_mode(WAL)"
	// timeLayout is fixed-width UTC, so the time column sorts and compares as text.
	timeLayout = "2006-01-02T15:04:05.000Z"
)

var memCounter atomic.Int64

// Store implements store.Store on SQLite. It uses a single connection: writes
// are serialised by the pool instead of failing with "database is locked", and
// no method may start a second query while a result set is still open.
type Store struct{ db *sql.DB }

var _ store.Store = (*Store)(nil)

// Open opens (creating it, and its directory, if needed) the database at path.
func Open(path string) (*Store, error) {
	if err := os.MkdirAll(filepath.Dir(path), 0o755); err != nil {
		return nil, fmt.Errorf("create database directory: %w", err)
	}
	return open("file:" + path + "?" + pragmas)
}

// OpenMemory opens a private in-memory database, unique per call. Used by tests.
func OpenMemory() (*Store, error) {
	n := memCounter.Add(1)
	return open(fmt.Sprintf("file:techlogmem%d?mode=memory&cache=shared&%s", n, pragmas))
}

func open(dsn string) (*Store, error) {
	db, err := sql.Open("sqlite", dsn)
	if err != nil {
		return nil, err
	}
	db.SetMaxOpenConns(1)
	if err := migrate(db); err != nil {
		db.Close()
		return nil, fmt.Errorf("migrate: %w", err)
	}
	return &Store{db: db}, nil
}

// migrate applies migrations/NNN_*.sql files, in name order, that are newer
// than PRAGMA user_version.
func migrate(db *sql.DB) error {
	entries, err := migrationsFS.ReadDir("migrations")
	if err != nil {
		return err
	}
	var current int
	if err := db.QueryRow("PRAGMA user_version").Scan(&current); err != nil {
		return err
	}
	for i, e := range entries {
		version := i + 1
		if version <= current {
			continue
		}
		script, err := migrationsFS.ReadFile("migrations/" + e.Name())
		if err != nil {
			return err
		}
		tx, err := db.Begin()
		if err != nil {
			return err
		}
		if _, err := tx.Exec(string(script)); err != nil {
			tx.Rollback()
			return fmt.Errorf("%s: %w", e.Name(), err)
		}
		if _, err := tx.Exec(fmt.Sprintf("PRAGMA user_version = %d", version)); err != nil {
			tx.Rollback()
			return err
		}
		if err := tx.Commit(); err != nil {
			return err
		}
	}
	return nil
}

func (s *Store) Close() error { return s.db.Close() }

// querier is satisfied by *sql.DB and *sql.Tx.
type querier interface {
	QueryRowContext(ctx context.Context, query string, args ...any) *sql.Row
	ExecContext(ctx context.Context, query string, args ...any) (sql.Result, error)
}

func ensureEvents(ctx context.Context, q querier, ids ...string) error {
	for _, id := range ids {
		var one int
		err := q.QueryRowContext(ctx, `SELECT 1 FROM events WHERE id = ?`, id).Scan(&one)
		if errors.Is(err, sql.ErrNoRows) {
			return fmt.Errorf("event %s: %w", id, store.ErrNotFound)
		}
		if err != nil {
			return err
		}
	}
	return nil
}

func insertLink(ctx context.Context, q querier, source, target, rel, basis, rationale string) error {
	_, err := q.ExecContext(ctx,
		`INSERT OR IGNORE INTO links (source, target, rel, basis, rationale, created_at) VALUES (?, ?, ?, ?, ?, ?)`,
		source, target, rel, basis, rationale, time.Now().UTC().Format(timeLayout))
	return err
}

func (s *Store) Insert(ctx context.Context, e event.Event) error {
	t, err := e.ParsedTime()
	if err != nil {
		return fmt.Errorf("event time: %w", err)
	}
	body, err := json.Marshal(e)
	if err != nil {
		return err
	}
	tx, err := s.db.BeginTx(ctx, nil)
	if err != nil {
		return err
	}
	defer tx.Rollback()

	targets := make([]string, len(e.Links))
	for i, l := range e.Links {
		targets[i] = l.Target
	}
	if err := ensureEvents(ctx, tx, targets...); err != nil {
		return err
	}
	_, err = tx.ExecContext(ctx,
		`INSERT INTO events (id, type, time, status, system, component, actor_kind, actor_id, summary, body)
		 VALUES (?, ?, ?, ?, ?, ?, ?, ?, ?, ?)`,
		e.ID, e.Type, t.UTC().Format(timeLayout), e.Status, e.Subject.System, e.Subject.Component,
		e.Actor.Kind, e.Actor.ID, e.Summary, string(body))
	if err != nil {
		if strings.Contains(err.Error(), "UNIQUE constraint failed") {
			return store.ErrConflict
		}
		return err
	}
	if _, err := tx.ExecContext(ctx,
		`INSERT INTO events_fts (id, summary, system, component, type) VALUES (?, ?, ?, ?, ?)`,
		e.ID, e.Summary, e.Subject.System, e.Subject.Component, e.Type); err != nil {
		return err
	}
	for _, l := range e.Links {
		if err := insertLink(ctx, tx, e.ID, l.Target, l.Rel, l.Basis, ""); err != nil {
			return err
		}
	}
	return tx.Commit()
}

func (s *Store) AddLink(ctx context.Context, l event.Link) error {
	tx, err := s.db.BeginTx(ctx, nil)
	if err != nil {
		return err
	}
	defer tx.Rollback()
	if err := ensureEvents(ctx, tx, l.Source, l.Target); err != nil {
		return err
	}
	if err := insertLink(ctx, tx, l.Source, l.Target, l.Rel, l.Basis, l.Rationale); err != nil {
		return err
	}
	return tx.Commit()
}

func (s *Store) Get(ctx context.Context, id string) (event.Event, error) {
	var body string
	err := s.db.QueryRowContext(ctx, `SELECT body FROM events WHERE id = ?`, id).Scan(&body)
	if errors.Is(err, sql.ErrNoRows) {
		return event.Event{}, store.ErrNotFound
	}
	if err != nil {
		return event.Event{}, err
	}
	return s.hydrate(ctx, body)
}

// hydrate decodes a stored body and replaces its links with the current
// outgoing rows from the links table (so links added later are included).
func (s *Store) hydrate(ctx context.Context, body string) (event.Event, error) {
	var e event.Event
	if err := json.Unmarshal([]byte(body), &e); err != nil {
		return event.Event{}, fmt.Errorf("decode stored event: %w", err)
	}
	rows, err := s.db.QueryContext(ctx,
		`SELECT target, rel, basis FROM links WHERE source = ? ORDER BY rowid`, e.ID)
	if err != nil {
		return event.Event{}, err
	}
	defer rows.Close()
	var links []event.LinkRef
	for rows.Next() {
		var l event.LinkRef
		if err := rows.Scan(&l.Target, &l.Rel, &l.Basis); err != nil {
			return event.Event{}, err
		}
		links = append(links, l)
	}
	if err := rows.Err(); err != nil {
		return event.Event{}, err
	}
	e.Links = links
	return e, nil
}

func (s *Store) Search(ctx context.Context, f store.Filter) ([]event.Event, error) {
	var (
		where []string
		args  []any
	)
	if q := ftsQuery(f.Text); q != "" {
		where = append(where, "e.id IN (SELECT id FROM events_fts WHERE events_fts MATCH ?)")
		args = append(args, q)
	}
	if f.System != "" {
		where = append(where, "e.system = ?")
		args = append(args, f.System)
	}
	if len(f.Types) > 0 {
		where = append(where, "e.type IN ("+strings.TrimSuffix(strings.Repeat("?,", len(f.Types)), ",")+")")
		for _, t := range f.Types {
			args = append(args, t)
		}
	}
	if !f.Since.IsZero() {
		where = append(where, "e.time >= ?")
		args = append(args, f.Since.UTC().Format(timeLayout))
	}
	if !f.Until.IsZero() {
		where = append(where, "e.time <= ?")
		args = append(args, f.Until.UTC().Format(timeLayout))
	}
	dir := "DESC"
	if f.Ascending {
		dir = "ASC"
	}
	limit := f.Limit
	if limit <= 0 {
		limit = 100
	}
	query := "SELECT e.body FROM events e"
	if len(where) > 0 {
		query += " WHERE " + strings.Join(where, " AND ")
	}
	query += " ORDER BY e.time " + dir + ", e.id " + dir + " LIMIT ?"
	args = append(args, limit)

	bodies, err := s.queryStrings(ctx, query, args...)
	if err != nil {
		return nil, err
	}
	events := make([]event.Event, 0, len(bodies))
	for _, b := range bodies {
		e, err := s.hydrate(ctx, b)
		if err != nil {
			return nil, err
		}
		events = append(events, e)
	}
	return events, nil
}

func (s *Store) Links(ctx context.Context, id string) ([]event.Link, error) {
	rows, err := s.db.QueryContext(ctx,
		`SELECT source, target, rel, basis, rationale FROM links WHERE source = ? OR target = ? ORDER BY rowid`, id, id)
	if err != nil {
		return nil, err
	}
	defer rows.Close()
	var links []event.Link
	for rows.Next() {
		var l event.Link
		if err := rows.Scan(&l.Source, &l.Target, &l.Rel, &l.Basis, &l.Rationale); err != nil {
			return nil, err
		}
		links = append(links, l)
	}
	return links, rows.Err()
}

func (s *Store) Systems(ctx context.Context) ([]string, error) {
	return s.queryStrings(ctx, `SELECT DISTINCT system FROM events ORDER BY system`)
}

// queryStrings runs a single-column text query and closes the result set
// before returning, so the single connection is free for the next query.
func (s *Store) queryStrings(ctx context.Context, query string, args ...any) ([]string, error) {
	rows, err := s.db.QueryContext(ctx, query, args...)
	if err != nil {
		return nil, err
	}
	defer rows.Close()
	var out []string
	for rows.Next() {
		var v string
		if err := rows.Scan(&v); err != nil {
			return nil, err
		}
		out = append(out, v)
	}
	return out, rows.Err()
}
```

- [ ] **Step 8: Tidy and run the tests**

Run (sandbox disabled):
```bash
go get modernc.org/sqlite@v1.60.1 && go mod tidy
go test ./internal/store/... -v 2>&1 | tail -60
```
Expected: PASS for `TestContract/*`, `TestOpenCreatesDirectoryAndPersists`, `TestFTSQuery`.

If `SearchTextPunctuation` fails on a query: print the SQLite error. FTS5 only fails on syntax, and `ftsQuery` quotes every word, so a failure means a word survived the filter in an unquoted form; fix `ftsQuery`, not the test. If `TestFTSQuery` shows `"AND"*` is not what you want, leave it: quoted operators are plain text and the contract test proves they do not error.

- [ ] **Step 9: Commit**

```bash
git add go.mod go.sum internal/store
git commit -m "feat(store): add store contract and sqlite backend"
```

---
### Task 3: Service layer and shared test helpers

**Files:**
- Create: `internal/service/errors.go`, `internal/service/service.go`, `internal/service/search.go`, `internal/service/story.go`
- Create: `internal/testutil/testutil.go`
- Test: `internal/service/fakestore_test.go`, `internal/service/service_test.go`, `internal/service/search_test.go`, `internal/service/story_test.go` (all `package service_test`)

**Interfaces:**
- Consumes: Task 1 (`event.*`, `event.NewValidator`, `event.Normalize`, `event.NewID`) and Task 2 (`store.Store`, `store.Filter`, `store.ErrNotFound`, `store.ErrConflict`).
- Produces (used by `api`, `mcpserver`, `ui`, `cmd`):
  - `service.New(st store.Store, opts ...Option) (*Service, error)`, `service.WithClock(func() time.Time) Option`
  - `service.MethodHuman = "human_entered"`, `service.MethodAgent = "agent_submitted"`
  - `(*Service).Record(ctx, doc map[string]any, method string) (event.Event, error)`: validates, assigns the id, inserts, returns the stored event
  - `(*Service).Link(ctx, LinkInput) (event.Link, error)`, `service.LinkInput{Source, Target, Rel, Basis, Rationale string}` (JSON tags `source,target,rel,basis,rationale`)
  - `(*Service).Get(ctx, id) (Detail, error)`, `service.Detail{Event event.Event; Outgoing, Incoming []event.Link}` (JSON `event`, `outgoing`, `incoming`; slices never nil)
  - `service.SearchParams{Query, System string; Types []string; Since, Until time.Time; Limit int}`, `service.SearchParamsFromQuery(url.Values) (SearchParams, error)` (query keys `q`, `system`, `type` (repeatable or comma-separated), `since`, `until`, `limit`), `service.ParseTime(raw string, endOfDay bool) (time.Time, error)`
  - `(*Service).Search(ctx, SearchParams) ([]event.Event, error)` (never nil; limit default 20, max 100)
  - `(*Service).History(ctx, system string, since time.Time, limit int) ([]event.Event, error)` (latest N, chronological; limit default 50, max 200; empty system is a `BadRequestError`)
  - `(*Service).Systems(ctx) ([]string, error)`
  - `(*Service).Story(ctx, id string, depth int) (Story, error)`, `service.Story{Root string; Events []event.Event; Edges []event.Link}` (events chronological; default depth 10, max 50, at most 200 events)
  - `(*Service).Import(ctx, docs []map[string]any) (inserted, skipped int, err error)`: keeps the supplied ids, skips ids that already exist
  - `*service.InvalidError{Problems []event.Problem}`, `*service.BadRequestError{Msg string}`; a missing event surfaces as `store.ErrNotFound`
  - `testutil.NewService(t testing.TB) *service.Service` (in-memory SQLite), `testutil.Doc(system, typ, summary string, links ...map[string]any) map[string]any` (a valid event document without id/provenance, time `2026-03-10T09:00:00Z`), `testutil.Link(rel, target string, basis ...string) map[string]any` (basis defaults to `confirmed`)

- [ ] **Step 1: Write the shared test helpers**

Create `internal/testutil/testutil.go`:

```go
// Package testutil holds helpers shared by the tests of several packages.
package testutil

import (
	"testing"

	"github.com/regeda/techlog/internal/service"
	"github.com/regeda/techlog/internal/store/sqlite"
)

// NewService returns a Service backed by a fresh in-memory SQLite database.
func NewService(t testing.TB) *service.Service {
	t.Helper()
	st, err := sqlite.OpenMemory()
	if err != nil {
		t.Fatal(err)
	}
	t.Cleanup(func() { st.Close() })
	svc, err := service.New(st)
	if err != nil {
		t.Fatal(err)
	}
	return svc
}

// Doc returns a valid event document without an id or provenance (Record
// supplies both). Callers may overwrite any key, for example "time".
func Doc(system, typ, summary string, links ...map[string]any) map[string]any {
	d := map[string]any{
		"type":    typ,
		"time":    "2026-03-10T09:00:00Z",
		"actor":   map[string]any{"kind": "human", "id": "user:test"},
		"subject": map[string]any{"system": system},
		"status":  "recorded",
		"summary": summary,
	}
	if len(links) > 0 {
		arr := make([]any, len(links))
		for i, l := range links {
			arr[i] = l
		}
		d["links"] = arr
	}
	return d
}

// Link returns a link object; the basis defaults to "confirmed".
func Link(rel, target string, basis ...string) map[string]any {
	b := "confirmed"
	if len(basis) > 0 {
		b = basis[0]
	}
	return map[string]any{"rel": rel, "target": target, "basis": b}
}
```

- [ ] **Step 2: Write the fake store**

Create `internal/service/fakestore_test.go`:

```go
package service_test

import (
	"context"
	"slices"
	"sort"

	"github.com/regeda/techlog/internal/event"
	"github.com/regeda/techlog/internal/store"
)

// fakeStore is a minimal in-memory store.Store for service tests. It also
// records what the service asked for.
type fakeStore struct {
	events          map[string]event.Event
	links           []event.Link
	insertConflicts int // fail this many Inserts with ErrConflict
	inserts         int
	lastFilter      store.Filter
}

func newFakeStore() *fakeStore { return &fakeStore{events: map[string]event.Event{}} }

func (f *fakeStore) merged(e event.Event) event.Event {
	e.Links = nil
	for _, l := range f.links {
		if l.Source == e.ID {
			e.Links = append(e.Links, event.LinkRef{Rel: l.Rel, Target: l.Target, Basis: l.Basis})
		}
	}
	return e
}

func (f *fakeStore) Insert(_ context.Context, e event.Event) error {
	f.inserts++
	if f.insertConflicts > 0 {
		f.insertConflicts--
		return store.ErrConflict
	}
	if _, ok := f.events[e.ID]; ok {
		return store.ErrConflict
	}
	for _, l := range e.Links {
		if _, ok := f.events[l.Target]; !ok {
			return store.ErrNotFound
		}
	}
	f.events[e.ID] = e
	for _, l := range e.Links {
		f.links = append(f.links, event.Link{Source: e.ID, Target: l.Target, Rel: l.Rel, Basis: l.Basis})
	}
	return nil
}

func (f *fakeStore) AddLink(_ context.Context, l event.Link) error {
	_, okSource := f.events[l.Source]
	_, okTarget := f.events[l.Target]
	if !okSource || !okTarget {
		return store.ErrNotFound
	}
	f.links = append(f.links, l)
	return nil
}

func (f *fakeStore) Get(_ context.Context, id string) (event.Event, error) {
	e, ok := f.events[id]
	if !ok {
		return event.Event{}, store.ErrNotFound
	}
	return f.merged(e), nil
}

func (f *fakeStore) Links(_ context.Context, id string) ([]event.Link, error) {
	var out []event.Link
	for _, l := range f.links {
		if l.Source == id || l.Target == id {
			out = append(out, l)
		}
	}
	return out, nil
}

func (f *fakeStore) Search(_ context.Context, flt store.Filter) ([]event.Event, error) {
	f.lastFilter = flt
	var out []event.Event
	for _, e := range f.events {
		if flt.System != "" && e.Subject.System != flt.System {
			continue
		}
		if len(flt.Types) > 0 && !slices.Contains(flt.Types, e.Type) {
			continue
		}
		t, _ := e.ParsedTime()
		if !flt.Since.IsZero() && t.Before(flt.Since) {
			continue
		}
		if !flt.Until.IsZero() && t.After(flt.Until) {
			continue
		}
		out = append(out, f.merged(e))
	}
	sort.Slice(out, func(i, j int) bool {
		ti, _ := out[i].ParsedTime()
		tj, _ := out[j].ParsedTime()
		if !ti.Equal(tj) {
			return ti.Before(tj) == flt.Ascending
		}
		return (out[i].ID < out[j].ID) == flt.Ascending
	})
	if flt.Limit > 0 && len(out) > flt.Limit {
		out = out[:flt.Limit]
	}
	return out, nil
}

func (f *fakeStore) Systems(_ context.Context) ([]string, error) {
	set := map[string]bool{}
	for _, e := range f.events {
		set[e.Subject.System] = true
	}
	out := make([]string, 0, len(set))
	for s := range set {
		out = append(out, s)
	}
	sort.Strings(out)
	return out, nil
}

func (f *fakeStore) Close() error { return nil }
```

- [ ] **Step 3: Write the service tests**

Create `internal/service/service_test.go`:

```go
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
```

Create `internal/service/search_test.go`:

```go
package service_test

import (
	"errors"
	"net/url"
	"slices"
	"testing"
	"time"

	"github.com/regeda/techlog/internal/service"
	"github.com/regeda/techlog/internal/testutil"
)

func TestSearchParamsFromQuery(t *testing.T) {
	v := url.Values{
		"q": {" spin "}, "system": {"alpha"}, "type": {"a.b, c.d", "e.f"},
		"since": {"2026-03-10"}, "until": {"2026-03-10"}, "limit": {"7"},
	}
	p, err := service.SearchParamsFromQuery(v)
	if err != nil {
		t.Fatal(err)
	}
	if p.Query != "spin" || p.System != "alpha" || p.Limit != 7 {
		t.Errorf("p = %+v", p)
	}
	if !slices.Equal(p.Types, []string{"a.b", "c.d", "e.f"}) {
		t.Errorf("types = %v", p.Types)
	}
	if want := time.Date(2026, 3, 10, 0, 0, 0, 0, time.UTC); !p.Since.Equal(want) {
		t.Errorf("since = %v", p.Since)
	}
	if want := time.Date(2026, 3, 10, 23, 59, 59, int(999*time.Millisecond), time.UTC); !p.Until.Equal(want) {
		t.Errorf("until = %v, want the end of that day", p.Until)
	}

	p, err = service.SearchParamsFromQuery(url.Values{"since": {"2026-03-11T12:00:00+02:00"}})
	if err != nil || !p.Since.Equal(time.Date(2026, 3, 11, 10, 0, 0, 0, time.UTC)) {
		t.Errorf("offset since = %v, %v", p.Since, err)
	}
}

func TestSearchParamsFromQueryErrors(t *testing.T) {
	for _, v := range []url.Values{
		{"since": {"yesterday"}}, {"until": {"2026-13-40"}}, {"limit": {"abc"}},
	} {
		_, err := service.SearchParamsFromQuery(v)
		var bad *service.BadRequestError
		if !errors.As(err, &bad) {
			t.Errorf("%v: got %v, want a BadRequestError", v, err)
		}
	}
}

func TestSearchAndHistoryClampLimits(t *testing.T) {
	svc, fs := newService(t)
	for _, tc := range []struct{ in, want int }{{0, 20}, {-3, 20}, {5, 5}, {500, 100}} {
		if _, err := svc.Search(ctx, service.SearchParams{Limit: tc.in}); err != nil {
			t.Fatal(err)
		}
		if fs.lastFilter.Limit != tc.want {
			t.Errorf("Search limit %d -> %d, want %d", tc.in, fs.lastFilter.Limit, tc.want)
		}
	}
	for _, tc := range []struct{ in, want int }{{0, 50}, {5, 5}, {1000, 200}} {
		if _, err := svc.History(ctx, "alpha", time.Time{}, tc.in); err != nil {
			t.Fatal(err)
		}
		if fs.lastFilter.Limit != tc.want {
			t.Errorf("History limit %d -> %d, want %d", tc.in, fs.lastFilter.Limit, tc.want)
		}
	}
}

func TestSearchNeverReturnsNil(t *testing.T) {
	svc, _ := newService(t)
	got, err := svc.Search(ctx, service.SearchParams{Query: "nothing"})
	if err != nil || got == nil {
		t.Fatalf("Search = %v, %v; want an empty non-nil slice", got, err)
	}
}

func TestHistoryLatestNChronological(t *testing.T) {
	svc, _ := newService(t)
	for i, ts := range []string{"2026-03-10T09:00:00Z", "2026-03-10T10:00:00Z", "2026-03-10T11:00:00Z", "2026-03-10T12:00:00Z"} {
		d := testutil.Doc("alpha", "problem.reported", string(rune('a'+i)))
		d["time"] = ts
		record(t, svc, d)
	}
	record(t, svc, testutil.Doc("beta", "problem.reported", "other system"))

	got, err := svc.History(ctx, "alpha", time.Time{}, 3)
	if err != nil {
		t.Fatal(err)
	}
	var summaries []string
	for _, e := range got {
		summaries = append(summaries, e.Summary)
	}
	if !slices.Equal(summaries, []string{"b", "c", "d"}) {
		t.Fatalf("history = %v, want the latest three, oldest first", summaries)
	}

	empty, err := svc.History(ctx, "no-such-system", time.Time{}, 10)
	if err != nil || empty == nil || len(empty) != 0 {
		t.Fatalf("unknown system: %v, %v; want an empty list and no error", empty, err)
	}

	var bad *service.BadRequestError
	if _, err := svc.History(ctx, "", time.Time{}, 10); !errors.As(err, &bad) {
		t.Fatalf("empty system: got %v, want a BadRequestError", err)
	}
}
```

Create `internal/service/story_test.go`:

```go
package service_test

import (
	"errors"
	"slices"
	"testing"

	"github.com/regeda/techlog/internal/event"
	"github.com/regeda/techlog/internal/service"
	"github.com/regeda/techlog/internal/store"
	"github.com/regeda/techlog/internal/testutil"
)

func recordAt(t *testing.T, svc *service.Service, ts, summary string, links ...map[string]any) event.Event {
	t.Helper()
	d := testutil.Doc("alpha", "problem.reported", summary, links...)
	d["time"] = ts
	return record(t, svc, d)
}

func summaries(evs []event.Event) []string {
	out := make([]string, len(evs))
	for i, e := range evs {
		out[i] = e.Summary
	}
	return out
}

func TestStory(t *testing.T) {
	svc, _ := newService(t)
	a := recordAt(t, svc, "2026-03-10T09:00:00Z", "a")
	b := recordAt(t, svc, "2026-03-10T10:00:00Z", "b", testutil.Link("motivated_by", a.ID))
	c := recordAt(t, svc, "2026-03-10T11:00:00Z", "c", testutil.Link("implements", b.ID))
	recordAt(t, svc, "2026-03-10T11:30:00Z", "unrelated")
	recordAt(t, svc, "2026-03-10T12:00:00Z", "e", testutil.Link("relates_to", a.ID))

	st, err := svc.Story(ctx, c.ID, 10)
	if err != nil {
		t.Fatal(err)
	}
	if st.Root != c.ID {
		t.Errorf("root = %q", st.Root)
	}
	if got := summaries(st.Events); !slices.Equal(got, []string{"a", "b", "c", "e"}) {
		t.Errorf("events = %v, want the connected set, oldest first", got)
	}
	if len(st.Edges) != 3 {
		t.Errorf("edges = %+v, want 3 (c->b, b->a, e->a)", st.Edges)
	}

	near, err := svc.Story(ctx, c.ID, 1)
	if err != nil {
		t.Fatal(err)
	}
	if got := summaries(near.Events); !slices.Equal(got, []string{"b", "c"}) {
		t.Errorf("depth 1 events = %v", got)
	}

	if _, err := svc.Story(ctx, "evt_000000000000", 5); !errors.Is(err, store.ErrNotFound) {
		t.Errorf("got %v, want ErrNotFound", err)
	}
}

func TestStoryTerminatesOnCycles(t *testing.T) {
	svc, _ := newService(t)
	a := recordAt(t, svc, "2026-03-10T09:00:00Z", "a")
	b := recordAt(t, svc, "2026-03-10T10:00:00Z", "b", testutil.Link("relates_to", a.ID))
	// a -> b closes a cycle: a relates_to b, b relates_to a.
	if _, err := svc.Link(ctx, service.LinkInput{Source: a.ID, Target: b.ID, Rel: "relates_to"}); err != nil {
		t.Fatal(err)
	}
	st, err := svc.Story(ctx, a.ID, 50)
	if err != nil {
		t.Fatal(err)
	}
	if len(st.Events) != 2 || len(st.Edges) != 2 {
		t.Fatalf("events=%d edges=%d, want 2 and 2", len(st.Events), len(st.Edges))
	}
}
```

- [ ] **Step 4: Run the tests to verify they fail**

Run: `go vet ./internal/service/ 2>&1 | head`
Expected: FAIL to compile: `undefined: service.New` (the package has no non-test files yet).

- [ ] **Step 5: Write the errors**

Create `internal/service/errors.go`:

```go
package service

import (
	"strings"

	"github.com/regeda/techlog/internal/event"
)

// InvalidError reports that a submitted event or link failed validation. Each
// Problem names the JSON Pointer path of the offending field.
type InvalidError struct {
	Problems []event.Problem
}

func (e *InvalidError) Error() string {
	parts := make([]string, len(e.Problems))
	for i, p := range e.Problems {
		parts[i] = p.Path + ": " + p.Message
	}
	return "invalid: " + strings.Join(parts, "; ")
}

// BadRequestError reports a malformed request (a bad date, limit, or body).
type BadRequestError struct {
	Msg string
}

func (e *BadRequestError) Error() string { return e.Msg }
```

- [ ] **Step 6: Write the service**

Create `internal/service/service.go`:

```go
// Package service holds Techlog's use cases. The HTTP API, the MCP server and
// the web UI call only this package, never a store directly.
package service

import (
	"bytes"
	"context"
	"crypto/rand"
	"encoding/json"
	"errors"
	"fmt"
	"io"
	"time"

	"github.com/regeda/techlog/internal/event"
	"github.com/regeda/techlog/internal/store"
)

// Provenance methods applied when a submission does not state one.
const (
	MethodHuman = "human_entered"   // events submitted through the HTTP API
	MethodAgent = "agent_submitted" // events submitted through MCP
)

const maxIDAttempts = 5

type Service struct {
	store     store.Store
	validator *event.Validator
	now       func() time.Time
	entropy   io.Reader
}

type Option func(*Service)

// WithClock replaces time.Now, for tests.
func WithClock(now func() time.Time) Option { return func(s *Service) { s.now = now } }

func New(st store.Store, opts ...Option) (*Service, error) {
	v, err := event.NewValidator()
	if err != nil {
		return nil, err
	}
	s := &Service{store: st, validator: v, now: time.Now, entropy: rand.Reader}
	for _, o := range opts {
		o(s)
	}
	return s, nil
}

// Record stores a new event. doc is the event without an id (a supplied id is
// ignored). method is the provenance method to use when doc has none. The
// event is validated against the JSON Schema, then link targets are checked.
func (s *Service) Record(ctx context.Context, doc map[string]any, method string) (event.Event, error) {
	doc, err := event.Normalize(doc)
	if err != nil {
		return event.Event{}, &BadRequestError{Msg: "event is not valid JSON: " + err.Error()}
	}
	applyDefaults(doc, method, s.now())
	id, err := event.NewID(s.now(), s.entropy)
	if err != nil {
		return event.Event{}, err
	}
	doc["id"] = id

	ev, err := s.prepare(ctx, doc)
	if err != nil {
		return event.Event{}, err
	}
	for attempt := 0; ; attempt++ {
		err = s.store.Insert(ctx, ev)
		if !errors.Is(err, store.ErrConflict) || attempt == maxIDAttempts-1 {
			break
		}
		newID, idErr := event.NewID(s.now(), s.entropy)
		if idErr != nil {
			return event.Event{}, idErr
		}
		ev.ID = newID
	}
	if err != nil {
		return event.Event{}, err
	}
	return s.store.Get(ctx, ev.ID)
}

// applyDefaults fills what the caller may omit. It never invents a basis for a
// caused_by link: that must be stated.
func applyDefaults(doc map[string]any, method string, now time.Time) {
	delete(doc, "id")
	if isBlank(doc["schema_version"]) {
		doc["schema_version"] = event.SchemaVersion
	}
	if isBlank(doc["time"]) {
		doc["time"] = now.UTC().Format(time.RFC3339)
	}
	if doc["provenance"] == nil {
		doc["provenance"] = map[string]any{}
	}
	if prov, ok := doc["provenance"].(map[string]any); ok {
		if isBlank(prov["method"]) {
			prov["method"] = method
		}
		if isBlank(prov["recorded_by"]) {
			if actor, ok := doc["actor"].(map[string]any); ok {
				if id, ok := actor["id"].(string); ok && id != "" {
					prov["recorded_by"] = id
				}
			}
		}
	}
	if links, ok := doc["links"].([]any); ok {
		for _, raw := range links {
			if l, ok := raw.(map[string]any); ok && isBlank(l["basis"]) && l["rel"] != "caused_by" {
				l["basis"] = "confirmed"
			}
		}
	}
}

func isBlank(v any) bool {
	if v == nil {
		return true
	}
	s, ok := v.(string)
	return ok && s == ""
}

// prepare validates doc (which already has an id) and decodes it. It reports
// schema problems and unresolvable link targets together.
func (s *Service) prepare(ctx context.Context, doc map[string]any) (event.Event, error) {
	if err := s.validator.Validate(doc); err != nil {
		var ve *event.ValidationError
		if errors.As(err, &ve) {
			return event.Event{}, &InvalidError{Problems: ve.Problems}
		}
		return event.Event{}, err
	}
	raw, err := json.Marshal(doc)
	if err != nil {
		return event.Event{}, err
	}
	dec := json.NewDecoder(bytes.NewReader(raw))
	dec.UseNumber()
	var ev event.Event
	if err := dec.Decode(&ev); err != nil {
		return event.Event{}, fmt.Errorf("decode validated event: %w", err)
	}

	var problems []event.Problem
	if _, err := ev.ParsedTime(); err != nil {
		problems = append(problems, event.Problem{Path: "/time", Message: err.Error()})
	}
	for i, l := range ev.Links {
		if _, err := s.store.Get(ctx, l.Target); errors.Is(err, store.ErrNotFound) {
			problems = append(problems, event.Problem{
				Path:    fmt.Sprintf("/links/%d/target", i),
				Message: fmt.Sprintf("event %s does not exist", l.Target),
			})
		} else if err != nil {
			return event.Event{}, err
		}
	}
	if len(problems) > 0 {
		return event.Event{}, &InvalidError{Problems: problems}
	}
	return ev, nil
}

// LinkInput describes a link to add between two existing events.
type LinkInput struct {
	Source    string `json:"source"`
	Target    string `json:"target"`
	Rel       string `json:"rel"`
	Basis     string `json:"basis,omitempty"`
	Rationale string `json:"rationale,omitempty"`
}

// Link adds a link. Basis defaults to "confirmed" except for caused_by, where
// it must be stated. An identical link added twice is a no-op.
func (s *Service) Link(ctx context.Context, in LinkInput) (event.Link, error) {
	if in.Basis == "" && in.Rel != "caused_by" {
		in.Basis = "confirmed"
	}
	doc := map[string]any{"rel": in.Rel, "target": in.Target}
	if in.Basis != "" {
		doc["basis"] = in.Basis
	}
	var problems []event.Problem
	if err := s.validator.ValidateLink(doc); err != nil {
		var ve *event.ValidationError
		if !errors.As(err, &ve) {
			return event.Link{}, err
		}
		problems = append(problems, ve.Problems...)
	}
	if in.Source == "" {
		problems = append(problems, event.Problem{Path: "/source", Message: "source is required"})
	} else if in.Source == in.Target {
		problems = append(problems, event.Problem{Path: "/target", Message: "an event cannot link to itself"})
	}
	if len(problems) == 0 {
		for _, end := range []struct{ path, id string }{{"/source", in.Source}, {"/target", in.Target}} {
			if _, err := s.store.Get(ctx, end.id); errors.Is(err, store.ErrNotFound) {
				problems = append(problems, event.Problem{Path: end.path, Message: fmt.Sprintf("event %s does not exist", end.id)})
			} else if err != nil {
				return event.Link{}, err
			}
		}
	}
	if len(problems) > 0 {
		return event.Link{}, &InvalidError{Problems: problems}
	}
	l := event.Link{Source: in.Source, Target: in.Target, Rel: in.Rel, Basis: in.Basis, Rationale: in.Rationale}
	if err := s.store.AddLink(ctx, l); err != nil {
		return event.Link{}, err
	}
	return l, nil
}

// Detail is an event with the links on either side of it.
type Detail struct {
	Event    event.Event  `json:"event"`
	Outgoing []event.Link `json:"outgoing"`
	Incoming []event.Link `json:"incoming"`
}

// Get returns one event and its links. A missing event is store.ErrNotFound.
func (s *Service) Get(ctx context.Context, id string) (Detail, error) {
	ev, err := s.store.Get(ctx, id)
	if err != nil {
		return Detail{}, err
	}
	links, err := s.store.Links(ctx, id)
	if err != nil {
		return Detail{}, err
	}
	d := Detail{Event: ev, Outgoing: []event.Link{}, Incoming: []event.Link{}}
	for _, l := range links {
		if l.Source == id {
			d.Outgoing = append(d.Outgoing, l)
		}
		if l.Target == id {
			d.Incoming = append(d.Incoming, l)
		}
	}
	return d, nil
}

// Import stores events that already carry ids (used to load the seed story). It
// validates them like Record but keeps their ids, and skips ids that exist.
// Documents must be ordered so every link target comes before its source.
func (s *Service) Import(ctx context.Context, docs []map[string]any) (inserted, skipped int, err error) {
	for _, raw := range docs {
		doc, err := event.Normalize(raw)
		if err != nil {
			return inserted, skipped, &BadRequestError{Msg: "event is not valid JSON: " + err.Error()}
		}
		ev, err := s.prepare(ctx, doc)
		if err != nil {
			return inserted, skipped, err
		}
		switch err := s.store.Insert(ctx, ev); {
		case errors.Is(err, store.ErrConflict):
			skipped++
		case err != nil:
			return inserted, skipped, err
		default:
			inserted++
		}
	}
	return inserted, skipped, nil
}
```

- [ ] **Step 7: Write search and story**

Create `internal/service/search.go`:

```go
package service

import (
	"context"
	"fmt"
	"net/url"
	"slices"
	"strconv"
	"strings"
	"time"

	"github.com/regeda/techlog/internal/event"
	"github.com/regeda/techlog/internal/store"
)

const (
	defaultSearchLimit  = 20
	maxSearchLimit      = 100
	defaultHistoryLimit = 50
	maxHistoryLimit     = 200
)

// SearchParams selects events. Zero values mean "no constraint".
type SearchParams struct {
	Query  string
	System string
	Types  []string
	Since  time.Time
	Until  time.Time
	Limit  int
}

// ParseTime accepts an RFC 3339 timestamp or a YYYY-MM-DD date. A bare date is
// the start of that day in UTC, or its last millisecond when endOfDay is set.
func ParseTime(raw string, endOfDay bool) (time.Time, error) {
	raw = strings.TrimSpace(raw)
	if t, err := time.Parse(time.RFC3339, raw); err == nil {
		return t, nil
	}
	d, err := time.Parse("2006-01-02", raw)
	if err != nil {
		return time.Time{}, fmt.Errorf("%q is not an RFC 3339 timestamp or a YYYY-MM-DD date", raw)
	}
	if endOfDay {
		d = d.Add(24*time.Hour - time.Millisecond)
	}
	return d, nil
}

// SearchParamsFromQuery reads q, system, type (repeatable or comma-separated),
// since, until and limit. Problems are *BadRequestError.
func SearchParamsFromQuery(v url.Values) (SearchParams, error) {
	p := SearchParams{Query: strings.TrimSpace(v.Get("q")), System: v.Get("system")}
	for _, raw := range v["type"] {
		for _, t := range strings.Split(raw, ",") {
			if t = strings.TrimSpace(t); t != "" {
				p.Types = append(p.Types, t)
			}
		}
	}
	var err error
	if raw := v.Get("since"); raw != "" {
		if p.Since, err = ParseTime(raw, false); err != nil {
			return SearchParams{}, &BadRequestError{Msg: "invalid since: " + err.Error()}
		}
	}
	if raw := v.Get("until"); raw != "" {
		if p.Until, err = ParseTime(raw, true); err != nil {
			return SearchParams{}, &BadRequestError{Msg: "invalid until: " + err.Error()}
		}
	}
	if raw := v.Get("limit"); raw != "" {
		if p.Limit, err = strconv.Atoi(raw); err != nil {
			return SearchParams{}, &BadRequestError{Msg: fmt.Sprintf("invalid limit: %q is not an integer", raw)}
		}
	}
	return p, nil
}

func clamp(n, def, max int) int {
	switch {
	case n <= 0:
		return def
	case n > max:
		return max
	}
	return n
}

// Search returns events newest first. The result is never nil.
func (s *Service) Search(ctx context.Context, p SearchParams) ([]event.Event, error) {
	events, err := s.store.Search(ctx, store.Filter{
		Text: p.Query, System: p.System, Types: p.Types, Since: p.Since, Until: p.Until,
		Limit: clamp(p.Limit, defaultSearchLimit, maxSearchLimit),
	})
	if err != nil {
		return nil, err
	}
	if events == nil {
		events = []event.Event{}
	}
	return events, nil
}

// History returns the latest events of one system, oldest first. An unknown
// system has an empty history; it is not an error.
func (s *Service) History(ctx context.Context, system string, since time.Time, limit int) ([]event.Event, error) {
	if system == "" {
		return nil, &BadRequestError{Msg: "system is required"}
	}
	events, err := s.store.Search(ctx, store.Filter{
		System: system, Since: since, Limit: clamp(limit, defaultHistoryLimit, maxHistoryLimit),
	})
	if err != nil {
		return nil, err
	}
	if events == nil {
		events = []event.Event{}
	}
	slices.Reverse(events)
	return events, nil
}

func (s *Service) Systems(ctx context.Context) ([]string, error) {
	systems, err := s.store.Systems(ctx)
	if err != nil {
		return nil, err
	}
	if systems == nil {
		systems = []string{}
	}
	return systems, nil
}
```

Create `internal/service/story.go`:

```go
package service

import (
	"context"
	"slices"
	"strings"

	"github.com/regeda/techlog/internal/event"
)

const (
	defaultStoryDepth = 10
	maxStoryDepth     = 50
	maxStoryNodes     = 200
)

// Story is the set of events connected to a root event through links, in
// either direction, with the links between them.
type Story struct {
	Root   string        `json:"root"`
	Events []event.Event `json:"events"` // oldest first
	Edges  []event.Link  `json:"edges"`
}

// Story walks links breadth-first from id, at most depth hops (default 10, max
// 50) and at most 200 events. Cycles are safe: each event is visited once.
func (s *Service) Story(ctx context.Context, id string, depth int) (Story, error) {
	if depth <= 0 {
		depth = defaultStoryDepth
	}
	depth = min(depth, maxStoryDepth)

	root, err := s.store.Get(ctx, id)
	if err != nil {
		return Story{}, err
	}
	seen := map[string]event.Event{id: root}
	edges := map[event.Link]struct{}{}
	frontier := []string{id}
	for hop := 0; hop < depth && len(frontier) > 0; hop++ {
		var next []string
		for _, cur := range frontier {
			links, err := s.store.Links(ctx, cur)
			if err != nil {
				return Story{}, err
			}
			for _, l := range links {
				edges[l] = struct{}{}
				other := l.Target
				if other == cur {
					other = l.Source
				}
				if _, ok := seen[other]; ok || len(seen) >= maxStoryNodes {
					continue
				}
				ev, err := s.store.Get(ctx, other)
				if err != nil {
					return Story{}, err
				}
				seen[other] = ev
				next = append(next, other)
			}
		}
		frontier = next
	}

	st := Story{Root: id, Events: make([]event.Event, 0, len(seen)), Edges: []event.Link{}}
	for _, ev := range seen {
		st.Events = append(st.Events, ev)
	}
	slices.SortFunc(st.Events, func(a, b event.Event) int {
		ta, _ := a.ParsedTime()
		tb, _ := b.ParsedTime()
		if c := ta.Compare(tb); c != 0 {
			return c
		}
		return strings.Compare(a.ID, b.ID)
	})
	for l := range edges {
		_, hasSource := seen[l.Source]
		_, hasTarget := seen[l.Target]
		if hasSource && hasTarget {
			st.Edges = append(st.Edges, l)
		}
	}
	slices.SortFunc(st.Edges, func(a, b event.Link) int {
		return strings.Compare(a.Source+a.Target+a.Rel+a.Basis, b.Source+b.Target+b.Rel+b.Basis)
	})
	return st, nil
}
```

- [ ] **Step 8: Format, tidy and run the tests**

Run:
```bash
gofmt -l internal/service internal/testutil
go mod tidy
go test ./internal/service/... ./internal/testutil/... -v 2>&1 | tail -60
```
Expected: `gofmt -l` prints nothing; tests PASS.

- [ ] **Step 9: Commit**

```bash
git add go.mod go.sum internal/service internal/testutil
git commit -m "feat(service): add record, link, search, history and story use cases"
```

---
### Task 4: HTTP API

**Files:**
- Create: `internal/api/api.go`
- Test: `internal/api/api_test.go`

**Interfaces:**
- Consumes: `service.*` and `testutil.*` from Task 3.
- Produces: `api.Handler(svc *service.Service) http.Handler`. Register it at `/api/v1/`; it handles every route below itself, including a JSON 404 for unknown paths under `/api/v1/`.

| Route | Success | Body |
|---|---|---|
| `POST /api/v1/events` | 201 + `Location` | `{"event": {...}}` |
| `GET /api/v1/events/{id}` | 200 | `{"event", "outgoing": [...], "incoming": [...]}` |
| `GET /api/v1/events?q&system&type&since&until&limit` | 200 | `{"events": [...]}` |
| `GET /api/v1/systems` | 200 | `{"systems": [...]}` |
| `GET /api/v1/systems/{system}/history?since&limit` | 200 | `{"system", "events": [...]}` |
| `POST /api/v1/links` | 201 | `{"link": {...}}` |

Errors: `{"error": {"code", "message", "details"?}}`; `details` is the list of `{path, message}` problems on `invalid_event`.

- [ ] **Step 1: Write the failing tests**

Create `internal/api/api_test.go`:

```go
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
```

- [ ] **Step 2: Run the tests to verify they fail**

Run: `go vet ./internal/api/ 2>&1 | head -5`
Expected: FAIL: `undefined: api.Handler` (no non-test files yet).

- [ ] **Step 3: Write the handlers**

Create `internal/api/api.go`:

```go
// Package api is the JSON HTTP API under /api/v1.
package api

import (
	"encoding/json"
	"errors"
	"fmt"
	"log/slog"
	"net/http"

	"github.com/regeda/techlog/internal/event"
	"github.com/regeda/techlog/internal/service"
	"github.com/regeda/techlog/internal/store"
)

const maxBodyBytes = 1 << 20

type handler struct{ svc *service.Service }

// Handler serves every /api/v1 route. Mount it at "/api/v1/".
func Handler(svc *service.Service) http.Handler {
	h := &handler{svc: svc}
	mux := http.NewServeMux()
	mux.HandleFunc("POST /api/v1/events", h.recordEvent)
	mux.HandleFunc("GET /api/v1/events", h.searchEvents)
	mux.HandleFunc("GET /api/v1/events/{id}", h.getEvent)
	mux.HandleFunc("GET /api/v1/systems", h.listSystems)
	mux.HandleFunc("GET /api/v1/systems/{system}/history", h.history)
	mux.HandleFunc("POST /api/v1/links", h.addLink)
	mux.HandleFunc("/api/v1/", func(w http.ResponseWriter, r *http.Request) {
		writeError(w, http.StatusNotFound, "not_found", "no such endpoint: "+r.Method+" "+r.URL.Path, nil)
	})
	return mux
}

func (h *handler) recordEvent(w http.ResponseWriter, r *http.Request) {
	doc, err := decodeObject(w, r)
	if err != nil {
		fail(w, r, err)
		return
	}
	ev, err := h.svc.Record(r.Context(), doc, service.MethodHuman)
	if err != nil {
		fail(w, r, err)
		return
	}
	w.Header().Set("Location", "/api/v1/events/"+ev.ID)
	writeJSON(w, http.StatusCreated, map[string]any{"event": ev})
}

func (h *handler) getEvent(w http.ResponseWriter, r *http.Request) {
	id := r.PathValue("id")
	d, err := h.svc.Get(r.Context(), id)
	if err != nil {
		fail(w, r, fmt.Errorf("event %s: %w", id, err))
		return
	}
	writeJSON(w, http.StatusOK, d)
}

func (h *handler) searchEvents(w http.ResponseWriter, r *http.Request) {
	p, err := service.SearchParamsFromQuery(r.URL.Query())
	if err != nil {
		fail(w, r, err)
		return
	}
	events, err := h.svc.Search(r.Context(), p)
	if err != nil {
		fail(w, r, err)
		return
	}
	writeJSON(w, http.StatusOK, map[string]any{"events": events})
}

func (h *handler) listSystems(w http.ResponseWriter, r *http.Request) {
	systems, err := h.svc.Systems(r.Context())
	if err != nil {
		fail(w, r, err)
		return
	}
	writeJSON(w, http.StatusOK, map[string]any{"systems": systems})
}

func (h *handler) history(w http.ResponseWriter, r *http.Request) {
	p, err := service.SearchParamsFromQuery(r.URL.Query())
	if err != nil {
		fail(w, r, err)
		return
	}
	system := r.PathValue("system")
	events, err := h.svc.History(r.Context(), system, p.Since, p.Limit)
	if err != nil {
		fail(w, r, err)
		return
	}
	writeJSON(w, http.StatusOK, map[string]any{"system": system, "events": events})
}

func (h *handler) addLink(w http.ResponseWriter, r *http.Request) {
	dec := json.NewDecoder(http.MaxBytesReader(w, r.Body, maxBodyBytes))
	dec.DisallowUnknownFields()
	var in service.LinkInput
	if err := dec.Decode(&in); err != nil {
		fail(w, r, &service.BadRequestError{Msg: "request body must be a JSON link object: " + err.Error()})
		return
	}
	l, err := h.svc.Link(r.Context(), in)
	if err != nil {
		fail(w, r, err)
		return
	}
	writeJSON(w, http.StatusCreated, map[string]any{"link": l})
}

// decodeObject reads exactly one JSON object from the request body (at most
// 1 MiB). Numbers keep their precision.
func decodeObject(w http.ResponseWriter, r *http.Request) (map[string]any, error) {
	dec := json.NewDecoder(http.MaxBytesReader(w, r.Body, maxBodyBytes))
	dec.UseNumber()
	var doc map[string]any
	if err := dec.Decode(&doc); err != nil {
		return nil, &service.BadRequestError{Msg: "request body must be a JSON object: " + err.Error()}
	}
	if doc == nil {
		return nil, &service.BadRequestError{Msg: "request body must be a JSON object"}
	}
	if dec.More() {
		return nil, &service.BadRequestError{Msg: "unexpected data after the JSON object"}
	}
	return doc, nil
}

type errorBody struct {
	Error struct {
		Code    string          `json:"code"`
		Message string          `json:"message"`
		Details []event.Problem `json:"details,omitempty"`
	} `json:"error"`
}

func writeError(w http.ResponseWriter, status int, code, message string, details []event.Problem) {
	var b errorBody
	b.Error.Code, b.Error.Message, b.Error.Details = code, message, details
	writeJSON(w, status, b)
}

// fail maps a service error to the HTTP error contract.
func fail(w http.ResponseWriter, r *http.Request, err error) {
	var invalid *service.InvalidError
	var bad *service.BadRequestError
	switch {
	case errors.As(err, &invalid):
		writeError(w, http.StatusUnprocessableEntity, "invalid_event", "validation failed", invalid.Problems)
	case errors.As(err, &bad):
		writeError(w, http.StatusBadRequest, "bad_request", bad.Msg, nil)
	case errors.Is(err, store.ErrNotFound):
		writeError(w, http.StatusNotFound, "not_found", err.Error(), nil)
	default:
		slog.Error("api: internal error", "method", r.Method, "path", r.URL.Path, "err", err)
		writeError(w, http.StatusInternalServerError, "internal", "internal error", nil)
	}
}

func writeJSON(w http.ResponseWriter, status int, v any) {
	w.Header().Set("Content-Type", "application/json")
	w.WriteHeader(status)
	if err := json.NewEncoder(w).Encode(v); err != nil {
		slog.Error("api: write response", "err", err)
	}
}
```

- [ ] **Step 4: Run the tests**

Run: `go test ./internal/api/ -v 2>&1 | tail -40`
Expected: PASS. If `TestRecordMalformedBodies/empty` fails with a status other than 400, an empty body makes `Decode` return `io.EOF`, which the handler already wraps; check that `fail` is reached.

- [ ] **Step 5: Commit**

```bash
git add internal/api
git commit -m "feat(api): add JSON HTTP API for events, links and systems"
```

---

### Task 5: MCP server

**Files:**
- Create: `internal/mcpserver/mcpserver.go`
- Test: `internal/mcpserver/mcpserver_test.go`

**Interfaces:**
- Consumes: `service.*`, `testutil.*`, the go-sdk `mcp` package (v1.8.0): `mcp.NewServer`, `mcp.AddTool`, `mcp.NewStreamableHTTPHandler`, `mcp.NewInMemoryTransports`.
- Produces: `mcpserver.New(svc *service.Service) *mcp.Server`, `mcpserver.Handler(svc *service.Service) http.Handler` (streamable HTTP; mount at `/mcp`).

Tools (inputs are optional unless marked required; the SDK infers each input schema from the Go struct, and a field without `omitempty` is required):

| Tool | Input | Output (`structuredContent`) |
|---|---|---|
| `techlog_search_events` | `query`, `system`, `types[]`, `since`, `until`, `limit` | `{events, count}` |
| `techlog_get_system_history` | `system` (required), `since`, `limit`, `include_links` (default true) | `{events, count}` |
| `techlog_get_event` | `id` (required) | `{event, outgoing, incoming}` |
| `techlog_record_event` | `event` (required, no id) | `{id, status, event}` |
| `techlog_link_events` | `source`, `target`, `rel` (required), `basis`, `rationale` | `{link}` |

- [ ] **Step 1: Write the failing tests**

Create `internal/mcpserver/mcpserver_test.go`:

```go
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
```

- [ ] **Step 2: Run the tests to verify they fail**

Run: `go vet ./internal/mcpserver/ 2>&1 | head -5`
Expected: FAIL: `undefined: mcpserver.New`.

- [ ] **Step 3: Write the server**

Create `internal/mcpserver/mcpserver.go`:

```go
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
		Description: "Record a structured Techlog event (draft 0.1). Do not supply an id: the server assigns it. Events are immutable; record a correction as a new event. On a validation error the result lists the field paths to fix.",
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
	Event map[string]any `json:"event" jsonschema:"the event without an id: type, time, actor, subject, status, provenance, and optionally summary, sources, evidence, links, extensions"`
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
	ID     string      `json:"id"`
	Status string      `json:"status"`
	Event  event.Event `json:"event"`
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
	return textResult(fmt.Sprintf("Recorded %s (%s)", ev.ID, ev.Status)), recordOut{ID: ev.ID, Status: ev.Status, Event: ev}, nil
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
	switch {
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
	line := fmt.Sprintf("%s  %s  %s [%s]  %s", e.ID, e.Time, e.Type, e.Status, e.Subject.System)
	if e.Summary != "" {
		line += ": " + e.Summary
	}
	for _, l := range e.Links {
		line += fmt.Sprintf("\n    %s %s (%s)", l.Rel, l.Target, l.Basis)
	}
	return line
}
```

- [ ] **Step 4: Tidy and run the tests**

Run (sandbox disabled, for the module proxy):
```bash
go get github.com/modelcontextprotocol/go-sdk@v1.8.0 && go mod tidy
gofmt -l internal/ ; go test ./internal/mcpserver/ -v 2>&1 | tail -40
```
Expected: `gofmt -l` prints nothing; tests PASS.

If a call fails with a protocol error about invalid arguments, the SDK rejected them against the schema it inferred from the Go struct: check the field's `omitempty` (a field without it is required). If the SDK rejects the `*bool` field `include_links`, stop and report the exact error; do not change the tool's documented default (`include_links` is true unless set to false) without asking.

- [ ] **Step 5: Commit**

```bash
git add go.mod go.sum internal/mcpserver
git commit -m "feat(mcp): add MCP tools over streamable HTTP"
```

---
### Task 6: Web UI

**Files:**
- Create: `internal/ui/ui.go`
- Create: `internal/ui/templates/layout.html`, `events.html`, `event.html`, `system.html`, `story.html`
- Create: `internal/ui/static/style.css`
- Test: `internal/ui/ui_test.go`

**Interfaces:**
- Consumes: `service.*`, `event.Types`, `testutil.*`.
- Produces: `ui.Handler(svc *service.Service) http.Handler`. Mount it at `/` (it answers 404 for paths it does not own, so the API and MCP routes must be registered on the outer mux first). Routes, all `GET`, read-only:
  - `/` event list with filters `q`, `system`, `type`, `since`, `until` (plain query params, so every view is linkable; at most 100 rows)
  - `/events/{id}` one event with links in both directions, sources, evidence, raw JSON, and a "walk through" link
  - `/systems/{system}` timeline of the latest 200 events, oldest first
  - `/story/{id}?depth=N` the connected set of events, oldest first, each with its outgoing relations
  - `/static/style.css`

- [ ] **Step 1: Write the failing tests**

Create `internal/ui/ui_test.go`:

```go
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
```

- [ ] **Step 2: Run the tests to verify they fail**

Run: `go vet ./internal/ui/ 2>&1 | head -5`
Expected: FAIL: `undefined: ui.Handler`.

- [ ] **Step 3: Write the templates**

Create `internal/ui/templates/layout.html`:

```html
{{define "head"}}<!doctype html>
<html lang="en">
<head>
<meta charset="utf-8">
<meta name="viewport" content="width=device-width, initial-scale=1">
<title>{{.Title}} · Techlog</title>
<link rel="stylesheet" href="/static/style.css">
</head>
<body>
<header class="top"><a class="brand" href="/">Techlog</a></header>
<main>
{{end}}
{{define "foot"}}
</main>
</body>
</html>
{{end}}
```

Create `internal/ui/templates/events.html`:

```html
{{template "head" .}}
<h1>Events</h1>
<form class="filters" method="get" action="/">
  <input type="search" name="q" value="{{.Q}}" placeholder="Search events…" aria-label="Search">
  <select name="system" aria-label="System">
    <option value="">All systems</option>
    {{range .Systems}}<option value="{{.}}"{{if eq . $.System}} selected{{end}}>{{.}}</option>{{end}}
  </select>
  <select name="type" aria-label="Type">
    <option value="">All types</option>
    {{range .Types}}<option value="{{.}}"{{if eq . $.Type}} selected{{end}}>{{.}}</option>{{end}}
  </select>
  <label>From <input type="date" name="since" value="{{.Since}}"></label>
  <label>To <input type="date" name="until" value="{{.Until}}"></label>
  <button type="submit">Filter</button>
  <a href="/">Reset</a>
</form>
{{if .Error}}<p class="error">{{.Error}}</p>{{end}}
<p class="muted">{{len .Events}} event(s), newest first{{if eq (len .Events) 100}} (showing the newest 100){{end}}</p>
{{if .Events}}
<table>
  <thead><tr><th>Time (UTC)</th><th>Type</th><th>Status</th><th>System</th><th>Actor</th><th>Summary</th></tr></thead>
  <tbody>
  {{range .Events}}
    <tr>
      <td class="nowrap">{{fmtTime .Time}}</td>
      <td><span class="badge t-{{typeClass .Type}}">{{.Type}}</span></td>
      <td>{{.Status}}</td>
      <td><a href="/systems/{{pathEscape .Subject.System}}">{{.Subject.System}}</a></td>
      <td>{{.Actor.ID}}</td>
      <td><a href="/events/{{.ID}}">{{if .Summary}}{{.Summary}}{{else}}{{.ID}}{{end}}</a></td>
    </tr>
  {{end}}
  </tbody>
</table>
{{else if not .Error}}<p>No events match.</p>{{end}}
{{template "foot" .}}
```

Create `internal/ui/templates/event.html`:

```html
{{template "head" .}}
<p class="crumbs"><a href="/">Events</a> / <a href="/systems/{{pathEscape .Event.Subject.System}}">{{.Event.Subject.System}}</a></p>
<h1><span class="badge t-{{typeClass .Event.Type}}">{{.Event.Type}}</span> <code>{{.Event.ID}}</code></h1>
{{if .Event.Summary}}<p class="lead">{{.Event.Summary}}</p>{{end}}
<p><a class="button" href="/story/{{.Event.ID}}">Walk through this story →</a></p>
<dl class="facts">
  <dt>Time</dt><dd>{{fmtTime .Event.Time}} UTC <span class="muted">({{.Event.Time}})</span></dd>
  <dt>Status</dt><dd>{{.Event.Status}}</dd>
  <dt>System</dt><dd><a href="/systems/{{pathEscape .Event.Subject.System}}">{{.Event.Subject.System}}</a>{{if .Event.Subject.Component}} / {{.Event.Subject.Component}}{{end}}{{if .Event.Subject.Environment}} ({{.Event.Subject.Environment}}){{end}}</dd>
  <dt>Actor</dt><dd>{{.Event.Actor.ID}} <span class="muted">({{.Event.Actor.Kind}}{{if .Event.Actor.OnBehalfOf}}, on behalf of {{.Event.Actor.OnBehalfOf}}{{end}})</span></dd>
  <dt>Provenance</dt><dd>{{.Event.Provenance.Method}} by {{.Event.Provenance.RecordedBy}}{{if .Event.Provenance.Confidence}} ({{.Event.Provenance.Confidence}}){{end}}</dd>
</dl>
<h2>Links from this event</h2>
{{template "linkrows" .Outgoing}}
<h2>Links to this event</h2>
{{template "linkrows" .Incoming}}
{{if .Event.Sources}}<h2>Sources</h2><ul>{{range .Event.Sources}}<li>{{.Kind}}: <code>{{.Ref}}</code></li>{{end}}</ul>{{end}}
{{if .Event.Evidence}}<h2>Evidence</h2><ul>{{range .Event.Evidence}}<li>{{.Kind}}: <code>{{.Ref}}</code>{{if .Digest}} <span class="muted">{{.Digest}}</span>{{end}}</li>{{end}}</ul>{{end}}
<details><summary>Raw JSON</summary><pre>{{toJSON .Event}}</pre></details>
{{template "foot" .}}

{{define "linkrows"}}
{{if .}}
<table class="links">
  {{range .}}
  <tr>
    <td>{{.Rel}}</td>
    <td><span class="chip {{.Basis}}">{{.Basis}}</span></td>
    <td>
      <a href="/events/{{.ID}}">{{.ID}}</a>
      {{if .Type}}<span class="badge t-{{typeClass .Type}}">{{.Type}}</span>{{end}}
      {{.Summary}}
      {{if .Rationale}}<br><span class="muted">{{.Rationale}}</span>{{end}}
    </td>
  </tr>
  {{end}}
</table>
{{else}}<p class="muted">None.</p>{{end}}
{{end}}
```

Create `internal/ui/templates/system.html`:

```html
{{template "head" .}}
<p class="crumbs"><a href="/">Events</a> / System</p>
<h1>{{.System}}</h1>
<p class="muted">{{len .Events}} event(s), oldest first{{if eq (len .Events) 200}} (the latest 200){{end}}</p>
{{if .Events}}
<ol class="timeline">
  {{range .Events}}
  <li class="t-{{typeClass .Type}}">
    <div class="when">{{fmtTime .Time}}</div>
    <div class="what">
      <span class="badge t-{{typeClass .Type}}">{{.Type}}</span>
      <span class="status">{{.Status}}</span>
      <a href="/events/{{.ID}}">{{if .Summary}}{{.Summary}}{{else}}{{.ID}}{{end}}</a>
      {{range .Links}}<span class="chip {{.Basis}}">{{.Rel}} <a href="/events/{{.Target}}">{{.Target}}</a></span>{{end}}
      <a class="walk" href="/story/{{.ID}}">walk through →</a>
    </div>
  </li>
  {{end}}
</ol>
{{else}}<p>No events recorded for this system.</p>{{end}}
{{template "foot" .}}
```

Create `internal/ui/templates/story.html`:

```html
{{template "head" .}}
<p class="crumbs"><a href="/">Events</a> / Story from <a href="/events/{{.Root}}">{{.Root}}</a></p>
<h1>Story</h1>
<p class="muted">{{len .Items}} connected event(s), oldest first. Links are followed in both directions. A dashed link is a hypothesis, not an established fact.</p>
<ol class="story">
  {{range .Items}}
  <li id="{{.Event.ID}}" class="t-{{typeClass .Event.Type}}{{if .Root}} root{{end}}">
    <div class="when">{{fmtTime .Event.Time}}</div>
    <div class="what">
      <span class="badge t-{{typeClass .Event.Type}}">{{.Event.Type}}</span>
      <span class="status">{{.Event.Status}}</span>
      <span class="muted">{{.Event.Subject.System}}</span>
      <p class="lead"><a href="/events/{{.Event.ID}}">{{if .Event.Summary}}{{.Event.Summary}}{{else}}{{.Event.ID}}{{end}}</a></p>
      {{range .Edges}}<div class="rel {{.Basis}}">{{.Rel}} <a href="#{{.Target}}">{{.Target}}</a> <span class="muted">({{.Basis}})</span></div>{{end}}
    </div>
  </li>
  {{end}}
</ol>
{{template "foot" .}}
```

- [ ] **Step 4: Write the stylesheet**

Create `internal/ui/static/style.css`:

```css
:root {
  --bg: #fafaf8; --fg: #1d1d1b; --muted: #6b6b66; --line: #deded8; --card: #ffffff; --accent: #1f5fbf;
  --warn: #b4540a;
}
@media (prefers-color-scheme: dark) {
  :root { --bg: #161615; --fg: #ececea; --muted: #9a9a94; --line: #33332f; --card: #1e1e1c; --accent: #6ea8ff; --warn: #e08a3c; }
}
* { box-sizing: border-box; }
body { margin: 0; background: var(--bg); color: var(--fg); font: 15px/1.5 system-ui, -apple-system, "Segoe UI", sans-serif; }
a { color: var(--accent); }
code, pre { font-family: ui-monospace, SFMono-Regular, Menlo, monospace; font-size: 0.9em; }
pre { overflow-x: auto; background: var(--card); border: 1px solid var(--line); padding: 12px; border-radius: 6px; }
.top { padding: 12px 16px; border-bottom: 1px solid var(--line); background: var(--card); }
.brand { font-weight: 700; text-decoration: none; color: var(--fg); letter-spacing: 0.02em; }
main { max-width: 1100px; margin: 0 auto; padding: 16px; }
h1 { margin: 8px 0 12px; font-size: 1.5rem; }
h2 { margin: 24px 0 8px; font-size: 1.1rem; }
.muted { color: var(--muted); }
.error { color: #c0262d; font-weight: 600; }
.nowrap { white-space: nowrap; }
.crumbs { color: var(--muted); margin: 4px 0; }
.lead { font-size: 1.05rem; margin: 6px 0; }
.filters { display: flex; flex-wrap: wrap; gap: 8px; align-items: center; margin-bottom: 12px; }
.filters input, .filters select, .filters button { font: inherit; padding: 6px 8px; border: 1px solid var(--line); border-radius: 6px; background: var(--card); color: var(--fg); }
.filters input[type=search] { min-width: 220px; }
.button { display: inline-block; padding: 6px 12px; border: 1px solid var(--accent); border-radius: 6px; text-decoration: none; }
table { width: 100%; border-collapse: collapse; background: var(--card); border: 1px solid var(--line); }
th, td { text-align: left; padding: 8px 10px; border-bottom: 1px solid var(--line); vertical-align: top; }
th { font-size: 0.8rem; text-transform: uppercase; letter-spacing: 0.04em; color: var(--muted); }
.facts { display: grid; grid-template-columns: max-content 1fr; gap: 4px 16px; }
.facts dt { color: var(--muted); }
.facts dd { margin: 0; }

.badge { --c: #6b6b66; display: inline-block; padding: 1px 8px; border: 1px solid var(--c); color: var(--c); border-radius: 999px; font-size: 0.8rem; white-space: nowrap; }
.t-problem { --c: #b4540a; } .t-investigation { --c: #6f42c1; } .t-proposal { --c: #1f5fbf; }
.t-decision { --c: #0a7d5a; } .t-change { --c: #0a7d5a; } .t-verification { --c: #0a7d5a; }
.t-release { --c: #1f5fbf; } .t-incident { --c: #c0262d; } .t-mitigation { --c: #b4540a; }
.t-rollback { --c: #c0262d; } .t-recovery { --c: #0a7d5a; } .t-postmortem { --c: #6f42c1; } .t-action { --c: #6b6b66; }
@media (prefers-color-scheme: dark) {
  .t-problem, .t-mitigation { --c: #e08a3c; } .t-investigation, .t-postmortem { --c: #b08cf0; }
  .t-proposal, .t-release { --c: #6ea8ff; } .t-decision, .t-change, .t-verification, .t-recovery { --c: #4cc29a; }
  .t-incident, .t-rollback { --c: #ff7a80; }
}
.chip { display: inline-block; padding: 0 8px; border: 1px solid var(--line); border-radius: 4px; font-size: 0.8rem; }
.chip.hypothesis, .rel.hypothesis { border-style: dashed; }
.chip.hypothesis { color: var(--warn); }

.timeline, .story { list-style: none; margin: 0; padding: 0 0 0 16px; border-left: 2px solid var(--line); }
.timeline li, .story li { display: grid; grid-template-columns: 130px 1fr; gap: 12px; padding: 8px 0 8px 12px; position: relative; }
.timeline li::before, .story li::before { content: ""; position: absolute; left: -22px; top: 14px; width: 10px; height: 10px; border-radius: 50%; background: var(--c, var(--muted)); }
.when { color: var(--muted); font-size: 0.85rem; white-space: nowrap; }
.status { color: var(--muted); font-size: 0.85rem; margin: 0 6px; }
.walk { font-size: 0.85rem; margin-left: 8px; }
.story li.root { background: color-mix(in srgb, var(--accent) 10%, transparent); border-radius: 6px; }
.rel { font-size: 0.85rem; margin: 2px 0 0 8px; padding-left: 8px; border-left: 2px solid var(--line); }
.rel.hypothesis { border-left-style: dashed; }
@media (max-width: 640px) { .timeline li, .story li { grid-template-columns: 1fr; gap: 2px; } }
```

- [ ] **Step 5: Write the handlers**

Create `internal/ui/ui.go`:

```go
// Package ui is Techlog's read-only web UI: server-rendered pages with
// embedded templates and CSS, no JavaScript and no build step.
package ui

import (
	"bytes"
	"context"
	"embed"
	"encoding/json"
	"errors"
	"html/template"
	"log/slog"
	"net/http"
	"net/url"
	"strconv"
	"strings"
	"time"

	"github.com/regeda/techlog/internal/event"
	"github.com/regeda/techlog/internal/service"
	"github.com/regeda/techlog/internal/store"
)

//go:embed templates/*.html
var templateFS embed.FS

//go:embed static
var staticFS embed.FS

const (
	eventsPageLimit  = 100
	historyPageLimit = 200
)

var funcs = template.FuncMap{
	"fmtTime": func(s string) string {
		t, err := time.Parse(time.RFC3339, s)
		if err != nil {
			return s
		}
		return t.UTC().Format("2006-01-02 15:04")
	},
	// typeClass maps "incident.opened" to "incident", the CSS colour family.
	"typeClass": func(t string) string {
		family, _, _ := strings.Cut(t, ".")
		return family
	},
	"pathEscape": url.PathEscape,
	"toJSON": func(v any) string {
		b, err := json.MarshalIndent(v, "", "  ")
		if err != nil {
			return err.Error()
		}
		return string(b)
	},
}

type server struct {
	svc  *service.Service
	tmpl *template.Template
}

// Handler serves the UI. Mount it at "/".
func Handler(svc *service.Service) http.Handler {
	s := &server{
		svc:  svc,
		tmpl: template.Must(template.New("").Funcs(funcs).ParseFS(templateFS, "templates/*.html")),
	}
	mux := http.NewServeMux()
	mux.HandleFunc("GET /{$}", s.events)
	mux.HandleFunc("GET /events/{id}", s.event)
	mux.HandleFunc("GET /systems/{system}", s.system)
	mux.HandleFunc("GET /story/{id}", s.story)
	mux.Handle("GET /static/", http.FileServerFS(staticFS))
	return mux
}

type eventsPage struct {
	Title                         string
	Q, System, Type, Since, Until string
	Systems, Types                []string
	Events                        []event.Event
	Error                         string
}

func (s *server) events(w http.ResponseWriter, r *http.Request) {
	q := r.URL.Query()
	page := eventsPage{
		Title: "Events", Q: q.Get("q"), System: q.Get("system"), Type: q.Get("type"),
		Since: q.Get("since"), Until: q.Get("until"),
		Types: event.Types, Events: []event.Event{},
	}
	systems, err := s.svc.Systems(r.Context())
	if err != nil {
		s.fail(w, r, err)
		return
	}
	page.Systems = systems

	status := http.StatusOK
	p, err := service.SearchParamsFromQuery(q)
	var bad *service.BadRequestError
	switch {
	case errors.As(err, &bad):
		page.Error = bad.Msg
		status = http.StatusBadRequest
	case err != nil:
		s.fail(w, r, err)
		return
	default:
		p.Limit = eventsPageLimit
		if page.Events, err = s.svc.Search(r.Context(), p); err != nil {
			s.fail(w, r, err)
			return
		}
	}
	s.render(w, status, "events.html", page)
}

// linkRow is one link in the event page, with the other event's type and summary.
type linkRow struct {
	ID, Rel, Basis, Rationale, Type, Summary string
}

type eventPage struct {
	Title              string
	Event              event.Event
	Outgoing, Incoming []linkRow
}

func (s *server) event(w http.ResponseWriter, r *http.Request) {
	d, err := s.svc.Get(r.Context(), r.PathValue("id"))
	if err != nil {
		s.fail(w, r, err)
		return
	}
	s.render(w, http.StatusOK, "event.html", eventPage{
		Title:    d.Event.ID,
		Event:    d.Event,
		Outgoing: s.rows(r.Context(), d.Outgoing, false),
		Incoming: s.rows(r.Context(), d.Incoming, true),
	})
}

// rows describes the event at the far end of each link. A lookup failure only
// leaves the type and summary blank.
func (s *server) rows(ctx context.Context, links []event.Link, incoming bool) []linkRow {
	rows := make([]linkRow, 0, len(links))
	for _, l := range links {
		id := l.Target
		if incoming {
			id = l.Source
		}
		row := linkRow{ID: id, Rel: l.Rel, Basis: l.Basis, Rationale: l.Rationale}
		if d, err := s.svc.Get(ctx, id); err == nil {
			row.Type, row.Summary = d.Event.Type, d.Event.Summary
		}
		rows = append(rows, row)
	}
	return rows
}

type systemPage struct {
	Title, System string
	Events        []event.Event
}

func (s *server) system(w http.ResponseWriter, r *http.Request) {
	system := r.PathValue("system")
	events, err := s.svc.History(r.Context(), system, time.Time{}, historyPageLimit)
	if err != nil {
		s.fail(w, r, err)
		return
	}
	s.render(w, http.StatusOK, "system.html", systemPage{Title: system, System: system, Events: events})
}

type storyItem struct {
	Event event.Event
	Edges []event.Link // links that leave this event, within the story
	Root  bool
}

type storyPage struct {
	Title, Root string
	Items       []storyItem
}

func (s *server) story(w http.ResponseWriter, r *http.Request) {
	depth, _ := strconv.Atoi(r.URL.Query().Get("depth")) // invalid or missing: the service default
	st, err := s.svc.Story(r.Context(), r.PathValue("id"), depth)
	if err != nil {
		s.fail(w, r, err)
		return
	}
	out := map[string][]event.Link{}
	for _, l := range st.Edges {
		out[l.Source] = append(out[l.Source], l)
	}
	page := storyPage{Title: "Story " + st.Root, Root: st.Root, Items: make([]storyItem, len(st.Events))}
	for i, ev := range st.Events {
		page.Items[i] = storyItem{Event: ev, Edges: out[ev.ID], Root: ev.ID == st.Root}
	}
	s.render(w, http.StatusOK, "story.html", page)
}

// render executes into a buffer first, so a template error cannot leave a
// half-written page.
func (s *server) render(w http.ResponseWriter, status int, name string, data any) {
	var buf bytes.Buffer
	if err := s.tmpl.ExecuteTemplate(&buf, name, data); err != nil {
		slog.Error("ui: render", "template", name, "err", err)
		http.Error(w, "internal error", http.StatusInternalServerError)
		return
	}
	w.Header().Set("Content-Type", "text/html; charset=utf-8")
	w.WriteHeader(status)
	buf.WriteTo(w)
}

func (s *server) fail(w http.ResponseWriter, r *http.Request, err error) {
	if errors.Is(err, store.ErrNotFound) {
		http.NotFound(w, r)
		return
	}
	slog.Error("ui: internal error", "path", r.URL.Path, "err", err)
	http.Error(w, "internal error", http.StatusInternalServerError)
}
```

- [ ] **Step 6: Format and run the tests**

Run:
```bash
gofmt -l internal/ui
go test ./internal/ui/ -v 2>&1 | tail -40
```
Expected: PASS. Common failures: a template parse error at startup (the test panics in `template.Must`; read the line number it names), or `TestStoryPage` missing `t-incident root` if the class attribute spacing in `story.html` differs from the test string.

- [ ] **Step 7: Commit**

```bash
git add internal/ui
git commit -m "feat(ui): add read-only web UI with event list, timeline and story walkthrough"
```

---
### Task 7: Seed data and the `techlog` command

**Files:**
- Create: `internal/seed/seed.go`, `internal/seed/spring_spin.json` (copied from `website/src/examples/timeline.json`)
- Create: `cmd/techlog/main.go`, `cmd/techlog/serve.go`, `cmd/techlog/seed.go`
- Test: `internal/seed/seed_test.go`, `cmd/techlog/main_test.go`, `cmd/techlog/smoke_test.go`

**Interfaces:**
- Consumes: `service.New`, `(*Service).Import`, `api.Handler`, `mcpserver.Handler`, `ui.Handler`, `sqlite.Open`, `testutil.NewService`.
- Produces:
  - `seed.SpringSpin() ([]map[string]any, error)`: the 25-event Spring Spin story, oldest first, every link pointing at an earlier event
  - the `techlog` binary: `techlog serve [--addr localhost:8080] [--db share/techlog.db] [--store sqlite]` and `techlog seed [--db share/techlog.db] [--store sqlite]`
  - in package `main`: `run(ctx, args, out) error`, `newHandler(svc, log) http.Handler` (mounts `/api/v1/`, `/mcp`, `/`)

- [ ] **Step 1: Copy the seed data and write its test**

```bash
mkdir -p internal/seed
cp website/src/examples/timeline.json internal/seed/spring_spin.json
```

Create `internal/seed/seed_test.go`:

```go
package seed_test

import (
	"context"
	"testing"

	"github.com/regeda/techlog/internal/seed"
	"github.com/regeda/techlog/internal/testutil"
)

func TestSpringSpin(t *testing.T) {
	docs, err := seed.SpringSpin()
	if err != nil {
		t.Fatal(err)
	}
	if len(docs) != 25 {
		t.Fatalf("got %d events, want 25", len(docs))
	}
	// Every link must point at an event listed earlier, so Import can insert in order.
	seen := map[string]bool{}
	for _, d := range docs {
		id, _ := d["id"].(string)
		if id == "" || seen[id] {
			t.Fatalf("missing or duplicate id %q", id)
		}
		links, _ := d["links"].([]any)
		for _, l := range links {
			if target := l.(map[string]any)["target"].(string); !seen[target] {
				t.Errorf("%s links to %s, which is not listed before it", id, target)
			}
		}
		seen[id] = true
	}

	// The whole story must be valid against the schema and importable.
	svc := testutil.NewService(t)
	inserted, skipped, err := svc.Import(context.Background(), docs)
	if err != nil || inserted != 25 || skipped != 0 {
		t.Fatalf("import = %d, %d, %v", inserted, skipped, err)
	}
	inserted, skipped, err = svc.Import(context.Background(), docs)
	if err != nil || inserted != 0 || skipped != 25 {
		t.Fatalf("second import = %d, %d, %v", inserted, skipped, err)
	}
}
```

- [ ] **Step 2: Write the seed loader**

Create `internal/seed/seed.go`:

```go
// Package seed embeds the example story shown in the docs, so a fresh server
// has something to explore.
package seed

import (
	"bytes"
	_ "embed"
	"encoding/json"
	"fmt"
)

//go:embed spring_spin.json
var springSpin []byte

// SpringSpin returns the simulated "Spring Spin" story: 25 events, oldest
// first, each link pointing at an earlier event. Pass it to service.Import.
func SpringSpin() ([]map[string]any, error) {
	dec := json.NewDecoder(bytes.NewReader(springSpin))
	dec.UseNumber()
	var docs []map[string]any
	if err := dec.Decode(&docs); err != nil {
		return nil, fmt.Errorf("decode spring spin seed: %w", err)
	}
	return docs, nil
}
```

- [ ] **Step 3: Write the command tests**

Create `cmd/techlog/main_test.go`:

```go
package main

import (
	"bytes"
	"context"
	"path/filepath"
	"strings"
	"testing"
)

func runCmd(t *testing.T, args ...string) (string, error) {
	t.Helper()
	var out bytes.Buffer
	err := run(context.Background(), args, &out)
	return out.String(), err
}

func TestRunErrors(t *testing.T) {
	if _, err := runCmd(t); err == nil || !strings.Contains(err.Error(), "missing command") {
		t.Errorf("no args: %v", err)
	}
	if _, err := runCmd(t, "bogus"); err == nil || !strings.Contains(err.Error(), `unknown command "bogus"`) {
		t.Errorf("bogus: %v", err)
	}
	dir := t.TempDir()
	if _, err := runCmd(t, "serve", "--store", "nope", "--db", filepath.Join(dir, "x.db")); err == nil || !strings.Contains(err.Error(), `unknown store "nope"`) {
		t.Errorf("unknown store: %v", err)
	}
}

func TestHelp(t *testing.T) {
	out, err := runCmd(t, "help")
	if err != nil || !strings.Contains(out, "techlog serve") || !strings.Contains(out, "techlog seed") {
		t.Fatalf("help = %q, %v", out, err)
	}
}

func TestSeedCommandIsRepeatable(t *testing.T) {
	db := filepath.Join(t.TempDir(), "share", "techlog.db")
	out, err := runCmd(t, "seed", "--db", db)
	if err != nil || !strings.Contains(out, "seeded 25 new events (0 already present)") {
		t.Fatalf("first seed = %q, %v", out, err)
	}
	out, err = runCmd(t, "seed", "--db", db)
	if err != nil || !strings.Contains(out, "seeded 0 new events (25 already present)") {
		t.Fatalf("second seed = %q, %v", out, err)
	}
}
```

Create `cmd/techlog/smoke_test.go`:

```go
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
	resp, err := http.Get(url)
	if err != nil {
		t.Fatal(err)
	}
	defer resp.Body.Close()
	body, _ := io.ReadAll(resp.Body)
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
	defer ts.Close()

	cs, err := mcp.NewClient(&mcp.Implementation{Name: "smoke", Version: "0"}, nil).Connect(ctx,
		&mcp.StreamableClientTransport{Endpoint: ts.URL + "/mcp", DisableStandaloneSSE: true}, nil)
	if err != nil {
		t.Fatalf("connect to /mcp: %v", err)
	}
	defer cs.Close()

	doc := testutil.Doc("promo-api", "decision.made", "Smoke test decision",
		testutil.Link("relates_to", "evt_01J9K2A0P011"))
	res, err := cs.CallTool(ctx, &mcp.CallToolParams{Name: "techlog_record_event", Arguments: map[string]any{"event": doc}})
	if err != nil || res.IsError {
		t.Fatalf("record over MCP: err=%v result=%+v", err, res)
	}
	var rec struct{ ID string }
	raw, _ := json.Marshal(res.StructuredContent)
	if err := json.Unmarshal(raw, &rec); err != nil || rec.ID == "" {
		t.Fatalf("structuredContent %s: %v", raw, err)
	}

	mustHave(t, "API event", httpGet(t, ts.URL+"/api/v1/events/"+rec.ID), `"outgoing"`, "evt_01J9K2A0P011", "agent_submitted")
	mustHave(t, "API search", httpGet(t, ts.URL+"/api/v1/events?q=smoke"), rec.ID)
	mustHave(t, "UI event page", httpGet(t, ts.URL+"/events/"+rec.ID), "Smoke test decision", "evt_01J9K2A0P011")
	mustHave(t, "UI list", httpGet(t, ts.URL+"/?q=smoke"), "Smoke test decision")
	mustHave(t, "UI seeded list", httpGet(t, ts.URL+"/?q=5xx"), "5xx ratio")
	mustHave(t, "UI system page", httpGet(t, ts.URL+"/systems/promo-api"), "Smoke test decision")
	mustHave(t, "UI story", httpGet(t, ts.URL+"/story/evt_01J9K2A0P011"), "Smoke test decision", "5xx ratio")
}
```

- [ ] **Step 4: Run the tests to verify they fail**

Run: `go vet ./internal/seed/ ./cmd/... 2>&1 | head -8`
Expected: FAIL: `undefined: seed.SpringSpin` and `undefined: run`/`newHandler`.

- [ ] **Step 5: Write the command**

Create `cmd/techlog/main.go`:

```go
// Command techlog runs the Techlog server: web UI, HTTP API and MCP on one port.
package main

import (
	"context"
	"flag"
	"fmt"
	"io"
	"os"

	"github.com/regeda/techlog/internal/store"
	"github.com/regeda/techlog/internal/store/sqlite"
)

const usage = `Usage:
  techlog serve [--addr localhost:8080] [--db share/techlog.db] [--store sqlite]
  techlog seed  [--db share/techlog.db] [--store sqlite]

  serve  start the web UI (/), the HTTP API (/api/v1) and MCP (/mcp) on one port
  seed   load the Spring Spin example story (safe to run more than once)
`

func main() {
	if err := run(context.Background(), os.Args[1:], os.Stdout); err != nil {
		fmt.Fprintln(os.Stderr, "techlog:", err)
		os.Exit(1)
	}
}

func run(ctx context.Context, args []string, out io.Writer) error {
	if len(args) == 0 {
		return fmt.Errorf("missing command\n\n%s", usage)
	}
	switch args[0] {
	case "serve":
		return runServe(ctx, args[1:], out)
	case "seed":
		return runSeed(ctx, args[1:], out)
	case "help", "-h", "--help":
		fmt.Fprint(out, usage)
		return nil
	default:
		return fmt.Errorf("unknown command %q\n\n%s", args[0], usage)
	}
}

// storeFlags are the flags shared by every command that opens the store.
type storeFlags struct{ db, kind string }

func addStoreFlags(fs *flag.FlagSet) *storeFlags {
	f := &storeFlags{}
	fs.StringVar(&f.db, "db", "share/techlog.db", "database path (sqlite)")
	fs.StringVar(&f.kind, "store", "sqlite", "storage backend: sqlite")
	return f
}

// open is the one place that knows the available backends. To add a backend,
// add a case here and a package under internal/store.
func (f *storeFlags) open() (store.Store, error) {
	switch f.kind {
	case "sqlite":
		s, err := sqlite.Open(f.db)
		if err != nil {
			return nil, err
		}
		return s, nil
	default:
		return nil, fmt.Errorf("unknown store %q (available: sqlite)", f.kind)
	}
}
```

Create `cmd/techlog/serve.go`:

```go
package main

import (
	"context"
	"errors"
	"flag"
	"fmt"
	"io"
	"log/slog"
	"net"
	"net/http"
	"os"
	"os/signal"
	"syscall"
	"time"

	"github.com/regeda/techlog/internal/api"
	"github.com/regeda/techlog/internal/mcpserver"
	"github.com/regeda/techlog/internal/service"
	"github.com/regeda/techlog/internal/ui"
)

func runServe(ctx context.Context, args []string, out io.Writer) error {
	fs := flag.NewFlagSet("serve", flag.ContinueOnError)
	addr := fs.String("addr", "localhost:8080", "listen address; use :8080 to listen on all interfaces")
	sf := addStoreFlags(fs)
	if err := fs.Parse(args); err != nil {
		return err
	}

	st, err := sf.open()
	if err != nil {
		return err
	}
	defer st.Close()
	svc, err := service.New(st)
	if err != nil {
		return err
	}

	log := slog.New(slog.NewTextHandler(os.Stderr, nil))
	slog.SetDefault(log)

	ln, err := net.Listen("tcp", *addr)
	if err != nil {
		return err
	}
	srv := &http.Server{Handler: newHandler(svc, log), ReadHeaderTimeout: 10 * time.Second}

	ctx, stop := signal.NotifyContext(ctx, os.Interrupt, syscall.SIGTERM)
	defer stop()
	errc := make(chan error, 1)
	go func() { errc <- srv.Serve(ln) }()
	fmt.Fprintf(out, "techlog listening on http://%s  (UI /, API /api/v1, MCP /mcp; db %s)\n", ln.Addr(), sf.db)

	select {
	case err := <-errc:
		return err
	case <-ctx.Done():
	}
	shutdownCtx, cancel := context.WithTimeout(context.Background(), 5*time.Second)
	defer cancel()
	if err := srv.Shutdown(shutdownCtx); err != nil {
		srv.Close() // an open MCP stream kept the server from draining in time
	}
	if err := <-errc; err != nil && !errors.Is(err, http.ErrServerClosed) {
		return err
	}
	return nil
}

// newHandler mounts the three front doors on one mux. Specific routes come
// first; the UI answers everything else (and 404s what it does not own).
func newHandler(svc *service.Service, log *slog.Logger) http.Handler {
	mux := http.NewServeMux()
	mux.Handle("/api/v1/", api.Handler(svc))
	mux.Handle("/mcp", mcpserver.Handler(svc))
	mux.Handle("/", ui.Handler(svc))
	return logRequests(log, mux)
}

// statusWriter records the status code. It forwards Flush, which MCP's
// server-sent-event streams need.
type statusWriter struct {
	http.ResponseWriter
	status int
}

func (w *statusWriter) WriteHeader(code int) {
	w.status = code
	w.ResponseWriter.WriteHeader(code)
}

func (w *statusWriter) Flush() {
	if f, ok := w.ResponseWriter.(http.Flusher); ok {
		f.Flush()
	}
}

func (w *statusWriter) Unwrap() http.ResponseWriter { return w.ResponseWriter }

func logRequests(log *slog.Logger, next http.Handler) http.Handler {
	return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		start := time.Now()
		sw := &statusWriter{ResponseWriter: w, status: http.StatusOK}
		next.ServeHTTP(sw, r)
		log.Info("request", "method", r.Method, "path", r.URL.Path, "status", sw.status, "took", time.Since(start).Round(time.Millisecond))
	})
}
```

Create `cmd/techlog/seed.go`:

```go
package main

import (
	"context"
	"flag"
	"fmt"
	"io"

	"github.com/regeda/techlog/internal/seed"
	"github.com/regeda/techlog/internal/service"
)

func runSeed(ctx context.Context, args []string, out io.Writer) error {
	fs := flag.NewFlagSet("seed", flag.ContinueOnError)
	sf := addStoreFlags(fs)
	if err := fs.Parse(args); err != nil {
		return err
	}
	st, err := sf.open()
	if err != nil {
		return err
	}
	defer st.Close()
	svc, err := service.New(st)
	if err != nil {
		return err
	}
	docs, err := seed.SpringSpin()
	if err != nil {
		return err
	}
	inserted, skipped, err := svc.Import(ctx, docs)
	if err != nil {
		return err
	}
	fmt.Fprintf(out, "seeded %d new events (%d already present) in %s\n", inserted, skipped, sf.db)
	return nil
}
```

- [ ] **Step 6: Tidy and run the tests**

Run (the smoke test listens on a local port, so run with the sandbox disabled):
```bash
gofmt -l . ; go mod tidy
go test ./internal/seed/ ./cmd/... -v 2>&1 | tail -40
```
Expected: `gofmt -l` prints nothing; PASS for `TestSpringSpin`, `TestRunErrors`, `TestHelp`, `TestSeedCommandIsRepeatable`, `TestSmoke`.

If `TestSmoke` fails to connect to `/mcp` with a 403 or a host error, the SDK's streamable handler is rejecting the request origin or Host header; read the response body in the error, and do not weaken any check beyond what the SDK documents for local development.

- [ ] **Step 7: Commit**

```bash
git add go.mod go.sum cmd internal/seed
git commit -m "feat(cmd): add techlog serve and seed commands with end-to-end smoke test"
```

---

### Task 8: Claude skill, README and final verification

**Files:**
- Create: `skills/techlog-record/SKILL.md`, `README.md`
- Test: `internal/event/skill_test.go`

**Interfaces:**
- Consumes: `event.NewValidator`, the five MCP tools from Task 5, `POST /api/v1/events` from Task 4.
- Produces: the skill directory `skills/techlog-record/`, installable by symlinking into `~/.claude/skills/`.

- [ ] **Step 1: Write the failing skill test**

The skill's example events must stay valid against the schema. Create `internal/event/skill_test.go`:

```go
package event_test

import (
	"encoding/json"
	"os"
	"regexp"
	"strings"
	"testing"
)

const skillPath = "../../skills/techlog-record/SKILL.md"

func TestSkillFrontMatter(t *testing.T) {
	b, err := os.ReadFile(skillPath)
	if err != nil {
		t.Fatal(err)
	}
	s := string(b)
	if !strings.HasPrefix(s, "---\nname: techlog-record\ndescription: ") {
		t.Fatalf("SKILL.md must start with front matter naming the skill and describing when to use it:\n%.200s", s)
	}
}

// Every ```json event block in the skill is an event as a model would send it
// (without an id). Each must validate once the server's id is added.
func TestSkillExamplesAreValid(t *testing.T) {
	b, err := os.ReadFile(skillPath)
	if err != nil {
		t.Fatal(err)
	}
	blocks := regexp.MustCompile("(?s)```json event\n(.*?)\n```").FindAllStringSubmatch(string(b), -1)
	if len(blocks) < 2 {
		t.Fatalf("expected at least two ```json event examples in SKILL.md, found %d", len(blocks))
	}
	v := newValidator(t)
	for i, m := range blocks {
		var doc map[string]any
		if err := json.Unmarshal([]byte(m[1]), &doc); err != nil {
			t.Fatalf("example %d is not valid JSON: %v", i+1, err)
		}
		if _, ok := doc["id"]; ok {
			t.Errorf("example %d must not contain an id: the server assigns it", i+1)
		}
		doc["id"] = "evt_000000000000"
		if err := v.Validate(doc); err != nil {
			t.Errorf("example %d is invalid: %v", i+1, err)
		}
	}
}
```

- [ ] **Step 2: Run the test to verify it fails**

Run: `go test ./internal/event/ -run Skill 2>&1 | tail -8`
Expected: FAIL: `open ../../skills/techlog-record/SKILL.md: no such file or directory`.

- [ ] **Step 3: Write the skill**

Create `skills/techlog-record/SKILL.md` (the outer fence below uses four backticks because the file itself contains fenced blocks):

````markdown
---
name: techlog-record
description: Use when work in this conversation reaches something worth remembering about a system (a problem found, a proposal, a decision, a merged change, a release, an incident, a rollback, a postmortem, a follow-up action) to record it as an event in Techlog, and when asked what happened to a system earlier. Records through the Techlog MCP tools and links each new event to the earlier events it follows from.
---

# Recording events in Techlog

Techlog is an append-only log of engineering events. Each event says what happened to a system and links back to the events that led to it, so a person can later walk the story from problem to follow-up. Record an event when something worth remembering has happened. Do not record chatter, plans you have not decided on, or every small edit.

## Connect

The tools come from the Techlog MCP server. If `techlog_record_event` is not available, ask the user to connect it:

```bash
claude mcp add --transport http techlog http://localhost:8080/mcp
```

The server starts with `techlog serve`. If you cannot use MCP, use the HTTP fallback at the end.

## Before you record

1. Search for what already exists: `techlog_search_events` (free text, `system`, `types`, `since`) or `techlog_get_system_history` (latest events of one system).
2. Pick the earlier events this one follows from. Their ids (`evt_...`) go in `links`. A link points backward in time, from the new event to the earlier one.
3. Reuse the system name that history already uses (`promo-api`, not `Promo API`).

## Choose the event type

| What happened | `type` | usual `status` | usual link `rel` to an earlier event |
|---|---|---|---|
| A need, defect or request was raised | `problem.reported` | `recorded` | `relates_to` a similar report |
| Someone examined a problem or failure and has findings | `investigation.completed` | `recorded` | `investigates`, `caused_by` |
| A solution was proposed | `proposal.recorded` | `proposed` | `motivated_by` |
| A decision was taken | `decision.made` | `recorded` | `decides` |
| A change was built and merged | `change.implemented` | `recorded` | `implements` |
| Tests or checks ran | `verification.completed` | `verified` or `failed` | `verifies` |
| A version reached an environment | `release.deployed` | `recorded` or `in_progress` | `releases` |
| Something broke in production | `incident.opened` | `open` | `observed_after`, `caused_by` |
| A temporary measure reduced impact | `mitigation.applied` | `mitigated` | `mitigates` |
| A release was reverted | `rollback.executed` | `recorded` | `reverts`, `mitigates` |
| Health was confirmed after a fix | `recovery.verified` | `verified` | `verifies` |
| An incident review was written | `postmortem.published` | `recorded` | `follows_up` |
| A follow-up action was agreed or finished | `action.tracked` | `open`, `in_progress` or `resolved` | `follows_up` |

Statuses: `proposed`, `recorded`, `in_progress`, `verified`, `failed`, `open`, `mitigated`, `resolved`, `superseded`. A status describes the thing at the moment you record it. When the situation changes, record a new event; never try to change an old one.

## Fill in the event

Call `techlog_record_event` with `event` set to an object with these fields. Do not send an `id`.

- `schema_version`: `"0.1-draft"`
- `type`, `status`: from the table above
- `time`: when it happened, RFC 3339 with a zone, for example `2026-03-12T13:38:00Z`
- `actor`: `{"kind": "ai_agent", "id": "agent:claude-code", "on_behalf_of": "user:<who asked>"}`. Leave `on_behalf_of` out if you do not know who it is.
- `subject`: `{"system": "...", "component": "..."}`; `component` and `environment` are optional
- `summary`: one plain sentence, at most 280 characters, that makes sense without context
- `links`: `[{"rel": "...", "target": "evt_...", "basis": "confirmed"}]`
- `sources` (where it came from) and `evidence` (supporting artifacts), each `{"kind": "...", "ref": "..."}`, when you have them. Evidence is a reference: the artifact stays in the tool that owns it.
- `provenance`: `{"recorded_by": "agent:claude-code", "method": "agent_submitted"}`

The result carries the new `id`. Use it to link later events.

## Links and honesty

- `basis: "confirmed"` means the relationship is established, such as a change that implements a proposal.
- `basis: "hypothesis"` means you suspect it. Use it for any cause you have not proven. `caused_by` must always state its basis. Prefer a hypothesis you can later confirm over a confident claim you cannot support.
- To confirm a hypothesis later, record a new `investigation.completed` event with a `caused_by` link whose basis is `confirmed`. You can also add a link between two existing events with `techlog_link_events`.
- Never record something you did not observe or were not told. If you are unsure, say so in the summary.

## Examples

A decision, linked to the proposal it decides:

```json event
{
  "schema_version": "0.1-draft",
  "type": "decision.made",
  "time": "2026-03-10T14:05:00Z",
  "actor": {"kind": "ai_agent", "id": "agent:claude-code", "on_behalf_of": "user:a.novak"},
  "subject": {"system": "promo-api", "component": "spin-service"},
  "status": "recorded",
  "summary": "Approved the spin-to-win proposal: one spin per new account, discount capped at 20%",
  "links": [{"rel": "decides", "target": "evt_01J9K2A0P004", "basis": "confirmed"}],
  "provenance": {"recorded_by": "agent:claude-code", "method": "agent_submitted"}
}
```

An incident, with a suspected cause recorded as a hypothesis:

```json event
{
  "schema_version": "0.1-draft",
  "type": "incident.opened",
  "time": "2026-03-12T13:38:00Z",
  "actor": {"kind": "ai_agent", "id": "agent:claude-code", "on_behalf_of": "user:a.novak"},
  "subject": {"system": "promo-api", "component": "spin-service", "environment": "production"},
  "status": "open",
  "summary": "5xx ratio on the spin endpoint is 6.2% on the 3.8.0 canary, above the 1.0% rollback threshold",
  "evidence": [{"kind": "alert", "ref": "alert://promo-api/high-error-rate/a-5521"}],
  "links": [
    {"rel": "observed_after", "target": "evt_01J9K2A0P010", "basis": "confirmed"},
    {"rel": "caused_by", "target": "evt_01J9K2A0P006", "basis": "hypothesis"}
  ],
  "provenance": {"recorded_by": "agent:claude-code", "method": "agent_submitted", "confidence": "asserted"}
}
```

## If the server rejects the event

The result has `isError: true` and lists the problems as `- <JSON pointer path>: <message>`. Fix exactly those fields and call the tool again. Common ones:

- `/actor/kind`: must be `human`, `ai_agent` or `system`.
- `/time`: needs a time zone, `Z` or an offset.
- `/summary`: over 280 characters; shorten it.
- `/links/0/target`: that event id does not exist; search again for the right one.
- `/links/0`: `missing property 'basis'`; `caused_by` links must say `confirmed` or `hypothesis`.

Do not retry a rejected event unchanged.

## Rules

- One event per fact. A change that shipped and a test that passed are two events.
- Events are immutable. To correct one, record a new event; optionally add a link to it with `techlog_link_events`.
- Read before you write: an event that duplicates one already in the history is noise.

## When MCP is not connected

```bash
curl -s -X POST http://localhost:8080/api/v1/events \
  -H 'content-type: application/json' \
  -d '{"schema_version":"0.1-draft","type":"decision.made","time":"2026-03-10T14:05:00Z","actor":{"kind":"ai_agent","id":"agent:claude-code"},"subject":{"system":"promo-api"},"status":"recorded","summary":"Approved the proposal","provenance":{"recorded_by":"agent:claude-code","method":"agent_submitted"}}'
```

A 422 response lists the problems in `error.details`, the same as above. To search: `curl 'http://localhost:8080/api/v1/events?q=spin&system=promo-api'`.
````

- [ ] **Step 4: Run the skill test**

Run: `go test ./internal/event/ -run Skill -v 2>&1 | tail -10`
Expected: PASS for `TestSkillFrontMatter` and `TestSkillExamplesAreValid`.

- [ ] **Step 5: Write the README**

Create `README.md` (outer fence is four backticks because the file contains fenced blocks):

````markdown
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
go test ./...
```

The smoke test in `cmd/techlog` starts a real server on a local port.
````

- [ ] **Step 6: Full verification**

Run, with the sandbox disabled (the smoke test binds a port):
```bash
gofmt -l .
go vet ./...
go test ./... 2>&1 | tail -20
```
Expected: `gofmt -l` and `go vet` print nothing; every package reports `ok`.

Then run the real binary once:
```bash
go build -o "$TMPDIR/techlog" ./cmd/techlog
"$TMPDIR/techlog" seed --db "$TMPDIR/techlog-check.db"
"$TMPDIR/techlog" serve --db "$TMPDIR/techlog-check.db" &
PID=$!
sleep 2
curl -s localhost:8080/api/v1/systems
curl -s 'localhost:8080/api/v1/events?q=canary&limit=1' | head -c 400
curl -s -o /dev/null -w '%{http_code}\n' localhost:8080/
curl -s -o /dev/null -w '%{http_code}\n' localhost:8080/story/evt_01J9K2A0P011
kill $PID
```
Expected: the systems list (`promo-api`, `spring-spin`, `support-desk`), one JSON event, then `200` and `200`.

Finally, open <http://localhost:8080/story/evt_01J9K2A0P011> in a browser and check the page by eye: the story is listed oldest first, the dashed hypothesis link is visible, and the page reads well in both light and dark mode.

- [ ] **Step 7: Check the skill with Claude (manual)**

With the server running, in a Claude Code session in this repository:
```bash
claude mcp add --transport http techlog http://localhost:8080/mcp
ln -s "$PWD/skills/techlog-record" ~/.claude/skills/techlog-record
```
Then ask Claude: "Record a decision in Techlog: we will cap spin discounts at 20% (system promo-api), linked to the Spring Spin proposal." Expected: Claude searches first, then calls `techlog_record_event` with an `agent_submitted` event linked to the proposal, and the new event appears in the UI. Ask: "What happened to promo-api this month?" Expected: Claude calls `techlog_get_system_history`.

- [ ] **Step 8: Commit**

```bash
git add README.md skills internal/event/skill_test.go
git commit -m "feat(skill): add techlog-record Claude skill and README"
```

---

## Out of scope (follow-ups, not part of this plan)

- Update the website docs and examples (`website/src/pages/docs/mcp.html`, `website/src/examples/mcp-*.json`) to the underscore tool names and to the implemented status of the MCP server.
- `techlog_create_rollback_plan` and `techlog_submit_evidence` tools, MCP resources and prompts.
- A second storage backend (for example Postgres), which only needs a new package, a `storetest.Run` call and a case in `storeFlags.open`.
