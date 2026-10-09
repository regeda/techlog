# Idempotency Key Implementation Plan

> **For agentic workers:** REQUIRED SUB-SKILL: Use superpowers:subagent-driven-development (recommended) or superpowers:executing-plans to implement this plan task-by-task. Steps use checkbox (`- [ ]`) syntax for tracking.

**Goal:** Every event carries a required, client-supplied `idempotency_key`, and Techlog rejects a second event from the same actor with the same key.

**Architecture:** The key is a top-level schema field. A `UNIQUE (actor_id, idempotency_key)` constraint in SQLite is the single enforcement point, so concurrent retries cannot both succeed. The store reports a duplicate as `store.DuplicateError` (distinct from the id-collision `ErrConflict`); the service wraps it in `service.DuplicateError`; HTTP maps it to 409 and MCP to an error result that names the existing event.

**Tech Stack:** Go, SQLite (`modernc.org/sqlite`), JSON Schema (`santhosh-tekuri/jsonschema/v6`), the MCP Go SDK, Node (website tests only).

**Spec:** `docs/superpowers/specs/2026-10-09-idempotency-key-design.md`

## Global Constraints

- Field name: `idempotency_key`, top level, snake_case, JSON type string, `minLength` 1, `maxLength` 128, listed in the schema's `required`.
- Uniqueness scope: `(actor.id, idempotency_key)`. The key is an opaque string: no trimming, no case folding.
- The server never generates or defaults the key.
- A duplicate is rejected: HTTP 409 with code `duplicate_event` and `error.existing_id`; MCP error result naming the existing id. No replay of the original event.
- No data migration: `internal/store/sqlite/migrations/001_init.sql` is edited in place; existing databases are deleted (`make flush`).
- Schema version stays `0.1-draft`.
- Validation errors (422) take priority over duplicates (409).
- Commits follow Conventional Commits (`<type>(<scope>): <description>`) and carry **no** `Co-Authored-By` or other attribution trailer.
- `internal/event/schema/event.json` and `website/src/examples/json-schema-core.json` must stay identical (a Go test enforces it).

## Corrections to the spec found while planning

These are facts about the code that the spec got slightly wrong. The plan follows the code; Task 6 updates the spec to match.

1. A *missing* required top-level field is reported at path `/` with message `missing property 'idempotency_key'`; only an *empty or too-long* key is reported at `/idempotency_key`.
2. `testutil.Doc` has about 40 call sites, so instead of taking the key as an argument it generates a unique key per call from a counter. Tests that need a specific key overwrite `d["idempotency_key"]`.
3. The 409 `message` is the full `DuplicateError` text; `existing_id` carries the id separately.

## Review Focus

- Same key and same actor, retried concurrently: exactly one event is stored, the rest get `ErrDuplicate` (Task 2 `ConcurrentSameKey`).
- Same key from a different actor is accepted (Tasks 2, 3, 4).
- A rejected duplicate leaves no event, FTS row or link behind (Task 2 `DuplicateLeavesNoTrace`).
- An id collision must still retry with a new id, and a key duplicate must never trigger that retry (Task 3).
- A key that is empty, 129 characters long, or not a string is rejected with a 422, not stored (Tasks 1, 4).
- Re-running the seed against a populated database stays idempotent (Task 3 `ImportSkipsDuplicateKey`, existing `TestSeedCommandIsRepeatable`).
- Keys are compared byte for byte: `"K"` and `"k"`, and a key with trailing space, are distinct (Task 2 `KeysAreOpaque`).

## File Structure

| File | Responsibility |
|---|---|
| `internal/event/schema/event.json`, `website/src/examples/json-schema-core.json` | The contract (identical copies) |
| `internal/event/event.go` | `Event.IdempotencyKey` |
| `internal/store/store.go` | `ErrDuplicate`, `DuplicateError`, `Insert` doc |
| `internal/store/sqlite/migrations/001_init.sql`, `sqlite.go` | Column, unique constraint, error mapping |
| `internal/store/storetest/storetest.go` | Backend-independent contract cases |
| `internal/service/errors.go`, `service.go` | `DuplicateError`; `Record` and `Import` handling |
| `internal/api/api.go` | 409 mapping, `existing_id` |
| `internal/mcpserver/mcpserver.go` | Duplicate error text, tool description |
| `internal/ui/templates/event.html` | One row showing the key |
| `internal/testutil/testutil.go` | `Doc` generates a unique key |
| `internal/seed/spring_spin.json` + website examples | Fixtures carry keys |
| `skills/techlog-record/SKILL.md`, website pages, `README.md` | Guidance and docs |

---

### Task 1: Contract and fixtures

After this task the schema requires the key, every fixture carries one, and the Go and website tests pass. The store does not yet enforce uniqueness (Task 2).

**Files:**
- Modify: `internal/event/schema/event.json`
- Modify: `website/src/examples/json-schema-core.json` (copy of the above)
- Modify: `internal/event/event.go`
- Create: `internal/event/event_test.go`
- Modify: `internal/event/schema_test.go`
- Modify: `internal/testutil/testutil.go`
- Modify: `internal/seed/spring_spin.json`, `website/src/examples/timeline.json`, `event-full.json`, `event-incident.json`, `event-minimal.json`, `mcp-record-event.json`, `mcp-record-verification.json`, `signed-submission.json` (all under `website/src/examples/`)
- Modify: `website/tests/lib/validate-event.mjs`, `website/tests/examples.test.mjs`
- Modify: `skills/techlog-record/SKILL.md` (the two `json event` examples and the field list only; the guidance text is Task 5)

**Interfaces:**
- Produces: `event.Event.IdempotencyKey string` (json `idempotency_key`, no `omitempty`); schema property `idempotency_key`; `testutil.Doc(...)` returns a doc whose `idempotency_key` is unique per call (`test-key-<n>`).

- [ ] **Step 1: Write the failing schema tests**

In `internal/event/schema_test.go`, add the key to `validDoc()` (after the `"id"` line):

```go
		"idempotency_key": "support-desk-ticket-790",
```

and add these cases to the `TestValidate` table, after `"unknown top-level field"`:

```go
		{"missing idempotency_key", func(d map[string]any) { delete(d, "idempotency_key") }, "/"},
		{"empty idempotency_key", func(d map[string]any) { d["idempotency_key"] = "" }, "/idempotency_key"},
		{"idempotency_key too long", func(d map[string]any) { d["idempotency_key"] = strings.Repeat("k", 129) }, "/idempotency_key"},
		{"idempotency_key at the limit", func(d map[string]any) { d["idempotency_key"] = strings.Repeat("k", 128) }, ""},
		{"idempotency_key is not a string", func(d map[string]any) { d["idempotency_key"] = 7 }, "/idempotency_key"},
```

Create `internal/event/event_test.go`:

```go
package event_test

import (
	"encoding/json"
	"strings"
	"testing"

	"github.com/regeda/techlog/internal/event"
)

func TestEventJSONCarriesIdempotencyKey(t *testing.T) {
	raw, err := json.Marshal(event.Event{IdempotencyKey: "deploy-4411"})
	if err != nil {
		t.Fatal(err)
	}
	if !strings.Contains(string(raw), `"idempotency_key":"deploy-4411"`) {
		t.Errorf("event JSON lacks the key: %s", raw)
	}
	var back event.Event
	if err := json.Unmarshal(raw, &back); err != nil || back.IdempotencyKey != "deploy-4411" {
		t.Errorf("round trip = %+v, %v", back, err)
	}
}
```

- [ ] **Step 2: Run the tests to verify they fail**

Run: `go test ./internal/event/ -run 'TestValidate$|TestEventJSON' -v`
Expected: FAIL. `TestValidate` fails everywhere (the schema rejects the unknown `idempotency_key` field); `TestEventJSONCarriesIdempotencyKey` does not compile (`unknown field IdempotencyKey`).

- [ ] **Step 3: Add the field to the schema and the model**

In `internal/event/schema/event.json`, add `"idempotency_key",` to `required` right after `"id",`, and add this property right after the `"id"` property:

```json
    "idempotency_key": {
      "type": "string",
      "minLength": 1,
      "maxLength": 128,
      "description": "Chosen by the client. Unique per actor.id: the server rejects a second event from the same actor with the same key. Compared byte for byte."
    },
```

Copy it to the website: `cp internal/event/schema/event.json website/src/examples/json-schema-core.json`.

In `internal/event/event.go`, add to `Event`, right after `ID`:

```go
	IdempotencyKey string         `json:"idempotency_key"`
```

(then run `gofmt -w internal/event/event.go` to realign the struct).

- [ ] **Step 4: Make `testutil.Doc` generate a unique key**

In `internal/testutil/testutil.go`, add `"fmt"` and `"sync/atomic"` to the imports, then:

```go
var keySeq atomic.Int64

// Doc returns a valid event document without an id or provenance (Record
// supplies both). It carries a fresh idempotency_key, so two docs from the
// same actor never collide. Callers may overwrite any key, for example "time"
// or "idempotency_key".
func Doc(system, typ, summary string, links ...map[string]any) map[string]any {
	d := map[string]any{
		"type":            typ,
		"time":            "2026-03-10T09:00:00Z",
		"idempotency_key": fmt.Sprintf("test-key-%d", keySeq.Add(1)),
		"actor":           map[string]any{"kind": "human", "id": "user:test"},
		"subject":         map[string]any{"system": system},
		"status":          "recorded",
		"summary":         summary,
	}
```

(keep the rest of the function unchanged).

- [ ] **Step 5: Add keys to the seed, the website timeline and the standalone examples**

Run from the repo root. The three files below have one `"id": "evt_01J9K2A0P0NN",` line per event; the key is derived from `NN`:

```bash
for f in internal/seed/spring_spin.json website/src/examples/timeline.json \
         website/src/examples/event-full.json website/src/examples/event-incident.json; do
  perl -0pi -e 's/^(\s*)"id": "evt_01J9K2A0P(\d{3})",\n/$&$1"idempotency_key": "seed-spring-spin-$2",\n/mg' "$f"
done
```

`event-minimal.json` is a different event (`evt_01J9K2A0Q001`); add its key by hand after the `"id"` line:

```json
  "idempotency_key": "promo-api-report-2026-03-12-m.okafor",
```

`signed-submission.json` has an event without an id; add after its `"schema_version"` line:

```json
    "idempotency_key": "seed-spring-spin-006",
```

The two MCP examples hold a request event (no id) and a stored event (with id). Insert the key after the request's `schema_version` line (line 10) and after the stored event's `id` line (line 66 in `mcp-record-event.json`, line 46 in `mcp-record-verification.json`). Do not touch the `structuredContent.id` line:

```bash
cd website/src/examples
perl -i -pe '$_ .= qq(          "idempotency_key": "seed-spring-spin-004",\n) if $. == 10 || $. == 66' mcp-record-event.json
perl -i -pe '$_ .= qq(          "idempotency_key": "seed-spring-spin-007",\n) if $. == 10 || $. == 46' mcp-record-verification.json
cd ../../..
```

Verify the result: `git diff --stat` shows only added lines (2 per MCP file, 25 each for seed and timeline, 1 each for the others), and `grep -c idempotency_key internal/seed/spring_spin.json` prints `25`.

- [ ] **Step 6: Add keys to the skill examples**

In `skills/techlog-record/SKILL.md`, in the first `json event` example (the decision) add after the `"schema_version"` line:

```json
  "idempotency_key": "promo-api-spin-proposal-approved",
```

and in the second (the incident):

```json
  "idempotency_key": "promo-api-3.8.0-canary-5xx-incident",
```

and add this bullet to the field list under "Fill in the event", after the `schema_version` bullet:

```markdown
- `idempotency_key`: required. A stable string you choose, unique per actor, up to 128 characters. See "Retries and idempotency keys" below.
```

(the section it points to is written in Task 5; do not leave the link dangling in a release: Tasks 1 and 5 ship together).

- [ ] **Step 7: Teach the website validator and tests about the key**

In `website/tests/lib/validate-event.mjs`, after the `maxLength` line add:

```js
    if (schema.minLength && value.length < schema.minLength) errors.push(`${path}: shorter than ${schema.minLength}`);
```

and update the comment at the top to list `minLength`. In `website/tests/examples.test.mjs`, add after the "validator rejects a missing required field" test:

```js
test('validator enforces the idempotency key: required, non-empty, at most 128 characters', () => {
  const base = load('event-minimal.json');
  const { idempotency_key, ...without } = base;
  assert.ok(validate(schema, without).some((e) => /idempotency_key: required/.test(e)));
  assert.ok(validate(schema, { ...base, idempotency_key: '' }).some((e) => /shorter than 1/.test(e)));
  assert.ok(validate(schema, { ...base, idempotency_key: 'k'.repeat(129) }).some((e) => /longer than 128/.test(e)));
  assert.deepEqual(validate(schema, { ...base, idempotency_key: 'k'.repeat(128) }), []);
});

test('the timeline keys are unique per actor', () => {
  const seen = new Set();
  for (const e of timeline) {
    const k = `${e.actor.id}\u0000${e.idempotency_key}`;
    assert.ok(!seen.has(k), `${e.id} repeats a key for ${e.actor.id}`);
    seen.add(k);
  }
});
```

- [ ] **Step 8: Run both test suites**

Run: `go test ./...`
Expected: PASS. If a test fails with `/` and `missing property 'idempotency_key'`, it builds a document without `testutil.Doc`; add `"idempotency_key"` to it.

Run: `cd website && npm test`
Expected: PASS.

- [ ] **Step 9: Commit**

```bash
git add internal/event internal/testutil internal/seed website/src/examples website/tests skills/techlog-record/SKILL.md
git commit -m "feat(event): require an idempotency_key in the event contract"
```

---

### Task 2: Store enforces uniqueness

**Files:**
- Modify: `internal/store/store.go`
- Modify: `internal/store/sqlite/migrations/001_init.sql`
- Modify: `internal/store/sqlite/sqlite.go` (`Insert`, new helper)
- Modify: `internal/store/storetest/storetest.go`

**Interfaces:**
- Consumes: `event.Event.IdempotencyKey` (Task 1).
- Produces: `store.ErrDuplicate` (sentinel); `*store.DuplicateError{ExistingID string}` whose `Is(ErrDuplicate)` is true; `Store.Insert` returns it when `(actor.id, idempotency_key)` exists. `ErrConflict` keeps meaning "this `id` exists".

- [ ] **Step 1: Write the failing contract cases**

In `internal/store/storetest/storetest.go`:

(a) In `mk`, add the key so existing cases stay unique per event: after `Actor:` add

```go
		IdempotencyKey: "key-" + id,
```

(b) Fix the existing `insertDuplicate` so it collides on `id` only, deterministically:

```go
func insertDuplicate(t *testing.T, s store.Store) {
	e := mk("evt_00000000000A", "problem.reported", "alpha", "2026-03-10T09:00:00Z", "first")
	insert(t, s, e)
	dup := e
	dup.IdempotencyKey = "another-key" // same id, different key: only the id collides
	if err := s.Insert(ctx, dup); !errors.Is(err, store.ErrConflict) {
		t.Fatalf("got %v, want ErrConflict", err)
	}
}
```

(c) Register and add the new cases. In the `cases` map add:

```go
		"InsertDuplicateKey":      insertDuplicateKey,
		"KeyScopeIsPerActor":      keyScopeIsPerActor,
		"KeysAreOpaque":           keysAreOpaque,
		"DuplicateLeavesNoTrace":  duplicateLeavesNoTrace,
		"ConcurrentSameKey":       concurrentSameKey,
```

and append:

```go
func insertDuplicateKey(t *testing.T, s store.Store) {
	a := mk("evt_00000000000A", "problem.reported", "alpha", "2026-03-10T09:00:00Z", "first")
	a.IdempotencyKey = "deploy-4411"
	insert(t, s, a)
	b := mk("evt_00000000000B", "problem.reported", "alpha", "2026-03-10T10:00:00Z", "retry")
	b.IdempotencyKey = "deploy-4411"
	err := s.Insert(ctx, b)
	if !errors.Is(err, store.ErrDuplicate) {
		t.Fatalf("got %v, want ErrDuplicate", err)
	}
	if errors.Is(err, store.ErrConflict) {
		t.Fatal("a key duplicate must not look like an id conflict: the service retries those")
	}
	var dup *store.DuplicateError
	if !errors.As(err, &dup) || dup.ExistingID != a.ID {
		t.Fatalf("got %#v, want DuplicateError{ExistingID: %s}", err, a.ID)
	}
	if _, err := s.Get(ctx, b.ID); !errors.Is(err, store.ErrNotFound) {
		t.Fatalf("the rejected event must not be stored, Get = %v", err)
	}
}

func keyScopeIsPerActor(t *testing.T, s store.Store) {
	a := mk("evt_00000000000A", "problem.reported", "alpha", "2026-03-10T09:00:00Z", "a")
	a.IdempotencyKey = "shared"
	b := mk("evt_00000000000B", "problem.reported", "alpha", "2026-03-10T10:00:00Z", "b")
	b.IdempotencyKey = "shared"
	b.Actor = event.Actor{Kind: "human", ID: "user:b"} // another actor: same key is fine
	c := mk("evt_00000000000C", "problem.reported", "alpha", "2026-03-10T11:00:00Z", "c")
	c.IdempotencyKey = "different" // same actor, another key: fine
	insert(t, s, a, b, c)
}

func keysAreOpaque(t *testing.T, s store.Store) {
	for i, key := range []string{"k", "K", "k ", " k"} {
		e := mk(fmt.Sprintf("evt_O%011d", i), "problem.reported", "alpha", "2026-03-10T09:00:00Z", "o")
		e.IdempotencyKey = key
		insert(t, s, e) // none of these may collide with another
	}
}

func duplicateLeavesNoTrace(t *testing.T, s store.Store) {
	a := mk("evt_00000000000A", "problem.reported", "alpha", "2026-03-10T09:00:00Z", "first")
	a.IdempotencyKey = "k"
	insert(t, s, a)
	b := mk("evt_00000000000B", "proposal.recorded", "alpha", "2026-03-10T10:00:00Z", "zebracorn",
		ref("motivated_by", a.ID, "confirmed"))
	b.IdempotencyKey = "k"
	if err := s.Insert(ctx, b); !errors.Is(err, store.ErrDuplicate) {
		t.Fatalf("got %v, want ErrDuplicate", err)
	}
	expectIDs(t, "search finds nothing for the rejected event", search(t, s, store.Filter{Text: "zebracorn"}))
	links, err := s.Links(ctx, a.ID)
	if err != nil || len(links) != 0 {
		t.Fatalf("links of the first event = %v, %v; want none", links, err)
	}
}

func concurrentSameKey(t *testing.T, s store.Store) {
	const n = 20
	var wg sync.WaitGroup
	errs := make(chan error, n)
	for i := 0; i < n; i++ {
		wg.Add(1)
		go func(i int) {
			defer wg.Done()
			e := mk(fmt.Sprintf("evt_K%011d", i), "problem.reported", "alpha", "2026-03-10T09:00:00Z", "k")
			e.IdempotencyKey = "same-for-all"
			errs <- s.Insert(ctx, e)
		}(i)
	}
	wg.Wait()
	close(errs)
	var stored, dups int
	for err := range errs {
		switch {
		case err == nil:
			stored++
		case errors.Is(err, store.ErrDuplicate):
			dups++
		default:
			t.Errorf("unexpected error: %v", err)
		}
	}
	if stored != 1 || dups != n-1 {
		t.Fatalf("stored %d, duplicates %d; want 1 and %d", stored, dups, n-1)
	}
	if got := search(t, s, store.Filter{Limit: 100}); len(got) != 1 {
		t.Fatalf("%d events in the store, want 1", len(got))
	}
}
```

- [ ] **Step 2: Run to verify it fails**

Run: `go test ./internal/store/... 2>&1 | head -20`
Expected: build failure, `undefined: store.ErrDuplicate` and `store.DuplicateError`.

- [ ] **Step 3: Add the store errors**

In `internal/store/store.go`, extend the `var (...)` block:

```go
	// ErrDuplicate means the same actor already recorded an event with that
	// idempotency key. Insert returns a *DuplicateError that matches it.
	ErrDuplicate = errors.New("duplicate idempotency key")
```

and below the block add:

```go
// DuplicateError is returned by Insert when (actor id, idempotency key) is
// already stored. ExistingID is the event that holds the key.
type DuplicateError struct{ ExistingID string }

func (e *DuplicateError) Error() string {
	return "duplicate idempotency key: already recorded as " + e.ExistingID
}

func (e *DuplicateError) Is(target error) bool { return target == ErrDuplicate }
```

Update the `Insert` comment:

```go
	// Insert stores an event and the links in e.Links. It returns ErrConflict
	// if the id exists, ErrDuplicate (as a *DuplicateError) if the actor
	// already used the idempotency key, and ErrNotFound if a link target does
	// not exist; in every case nothing is stored.
```

- [ ] **Step 4: Add the column and the constraint**

In `internal/store/sqlite/migrations/001_init.sql`, change the `events` table: add the column after `actor_id` and the constraint before the closing parenthesis:

```sql
    actor_id   TEXT NOT NULL,
    idempotency_key TEXT NOT NULL,
```

```sql
    body       TEXT NOT NULL,            -- the full event JSON; the source of truth for reads
    UNIQUE (actor_id, idempotency_key)
);
```

(Mind the comma after `body TEXT NOT NULL`: the original line has none.)

- [ ] **Step 5: Map the constraint failures in `Insert`**

In `internal/store/sqlite/sqlite.go`, replace the `INSERT INTO events` statement and its error handling in `Insert`:

```go
	_, err = tx.ExecContext(ctx,
		`INSERT INTO events (id, type, time, status, system, component, actor_kind, actor_id, idempotency_key, summary, body)
		 VALUES (?, ?, ?, ?, ?, ?, ?, ?, ?, ?, ?)`,
		e.ID, e.Type, t.UTC().Format(timeLayout), e.Status, e.Subject.System, e.Subject.Component,
		e.Actor.Kind, e.Actor.ID, e.IdempotencyKey, e.Summary, string(body))
	if err != nil {
		if strings.Contains(err.Error(), "UNIQUE constraint failed") {
			if strings.Contains(err.Error(), "events.idempotency_key") {
				return duplicateOf(ctx, tx, e)
			}
			return store.ErrConflict
		}
		return err
	}
```

and add the helper below `Insert`:

```go
// duplicateOf looks up the event that already holds e's (actor, key) pair.
func duplicateOf(ctx context.Context, q querier, e event.Event) error {
	var id string
	err := q.QueryRowContext(ctx,
		`SELECT id FROM events WHERE actor_id = ? AND idempotency_key = ?`,
		e.Actor.ID, e.IdempotencyKey).Scan(&id)
	if err != nil {
		return fmt.Errorf("find the event holding idempotency key: %w", err)
	}
	return &store.DuplicateError{ExistingID: id}
}
```

- [ ] **Step 6: Run the store tests**

Run: `go test ./internal/store/... -race -count=1`
Expected: PASS, including `ConcurrentSameKey`.

- [ ] **Step 7: Run everything and commit**

Run: `go test ./...`
Expected: PASS. (Service tests still pass: `testutil.Doc` keys are unique, and the service has not changed yet.)

```bash
git add internal/store
git commit -m "feat(store): reject a second event with the same actor and idempotency key"
```

---

### Task 3: Service

**Files:**
- Modify: `internal/service/errors.go`
- Modify: `internal/service/service.go` (`Record`, `Import`)
- Modify: `internal/service/fakestore_test.go`
- Modify: `internal/service/service_test.go`

**Interfaces:**
- Consumes: `*store.DuplicateError`, `store.ErrDuplicate` (Task 2).
- Produces: `*service.DuplicateError{Key, ActorID, ExistingID string}` returned by `Record`; `Import` counts key duplicates as skipped.

- [ ] **Step 1: Make the fake store enforce the key and write the failing tests**

In `internal/service/fakestore_test.go`, in `Insert`, after the `if _, ok := f.events[e.ID]; ok { return store.ErrConflict }` block add:

```go
	for _, other := range f.events {
		if other.Actor.ID == e.Actor.ID && other.IdempotencyKey == e.IdempotencyKey {
			return &store.DuplicateError{ExistingID: other.ID}
		}
	}
```

In `internal/service/service_test.go` add:

```go
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
	for name, mutate := range map[string]struct {
		fn   func(d map[string]any)
		path string
	}{
		"missing": {func(d map[string]any) { delete(d, "idempotency_key") }, "/"},
		"empty":   {func(d map[string]any) { d["idempotency_key"] = "" }, "/idempotency_key"},
		"long":    {func(d map[string]any) { d["idempotency_key"] = strings.Repeat("k", 129) }, "/idempotency_key"},
	} {
		t.Run(name, func(t *testing.T) {
			d := testutil.Doc("alpha", "problem.reported", "x")
			mutate.fn(d)
			_, err := svc.Record(ctx, d, service.MethodHuman)
			if !slices.Contains(problemPaths(err), mutate.path) {
				t.Fatalf("got %v (paths %v), want a problem at %q", err, problemPaths(err), mutate.path)
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
```

Add `"strings"` to the test file's imports.

Also extend the existing `TestRecordRetriesOnIDConflict` unchanged: it must still pass (an id collision retries, a key duplicate never does).

- [ ] **Step 2: Run to verify it fails**

Run: `go test ./internal/service/ 2>&1 | head`
Expected: build failure, `undefined: service.DuplicateError`.

- [ ] **Step 3: Add `DuplicateError`**

In `internal/service/errors.go` add `"fmt"` to the imports and:

```go
// DuplicateError reports that the actor already recorded an event with this
// idempotency key. ExistingID is that event.
type DuplicateError struct {
	Key        string
	ActorID    string
	ExistingID string
}

func (e *DuplicateError) Error() string {
	return fmt.Sprintf("duplicate: idempotency_key %q for actor %q was already recorded as %s",
		e.Key, e.ActorID, e.ExistingID)
}
```

- [ ] **Step 4: Handle the duplicate in `Record` and `Import`**

In `internal/service/service.go`, in `Record`, replace the block after the retry loop:

```go
	var dup *store.DuplicateError
	if errors.As(err, &dup) {
		return event.Event{}, &DuplicateError{Key: ev.IdempotencyKey, ActorID: ev.Actor.ID, ExistingID: dup.ExistingID}
	}
	if err != nil {
		return event.Event{}, err
	}
	return s.store.Get(ctx, ev.ID)
```

(the loop itself is unchanged: `ErrDuplicate` is not `ErrConflict`, so it breaks out on the first attempt). Update the `Record` doc comment's last sentence: `The event is validated against the JSON Schema, then link targets are checked; a second event with the same actor and idempotency_key is rejected with a *DuplicateError.`

In `Import`, change the conflict case to:

```go
		case errors.Is(err, store.ErrConflict), errors.Is(err, store.ErrDuplicate):
			skipped++
```

and mention "or whose actor and idempotency key" in its doc comment (`skips ids and keys that exist`).

- [ ] **Step 5: Run the service tests**

Run: `go test ./internal/service/ -race -count=1`
Expected: PASS.

- [ ] **Step 6: Run everything and commit**

Run: `go test ./...`
Expected: PASS.

```bash
git add internal/service
git commit -m "feat(service): return DuplicateError for a repeated actor and idempotency key"
```

---

### Task 4: HTTP API, MCP and web UI

**Files:**
- Modify: `internal/api/api.go`, `internal/api/api_test.go`
- Modify: `internal/mcpserver/mcpserver.go`, `internal/mcpserver/mcpserver_test.go`
- Modify: `internal/ui/templates/event.html`, `internal/ui/ui_test.go`
- Modify: `cmd/techlog/smoke_test.go`

**Interfaces:**
- Consumes: `*service.DuplicateError` (Task 3).
- Produces: HTTP 409 `{"error": {"code": "duplicate_event", "message": ..., "existing_id": ...}}`; an MCP error result whose text contains the existing id; an `Idempotency key` row on the event page.

- [ ] **Step 1: Write the failing API tests**

In `internal/api/api_test.go` add:

```go
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
```

- [ ] **Step 2: Run to verify they fail**

Run: `go test ./internal/api/ -run 'DuplicateKey|KeyValidation|SameKey' -v`
Expected: `TestRecordDuplicateKeyIs409` FAILS with status 500 (an unmapped error); the other two PASS already (they pin behaviour that must not regress).

- [ ] **Step 3: Map the error in the API**

In `internal/api/api.go`, extend `errorBody` and `writeError`:

```go
type errorBody struct {
	Error struct {
		Code       string          `json:"code"`
		Message    string          `json:"message"`
		ExistingID string          `json:"existing_id,omitempty"`
		Details    []event.Problem `json:"details,omitempty"`
	} `json:"error"`
}
```

(`writeError` stays as is.) In `fail`, add a case before `errors.Is(err, store.ErrNotFound)`:

```go
	case errors.As(err, &dup):
		var b errorBody
		b.Error.Code, b.Error.Message, b.Error.ExistingID = "duplicate_event", dup.Error(), dup.ExistingID
		writeJSON(w, http.StatusConflict, b)
```

and declare `var dup *service.DuplicateError` next to the other two `var` lines.

- [ ] **Step 4: Run the API tests**

Run: `go test ./internal/api/ -count=1`
Expected: PASS.

- [ ] **Step 5: Write the failing MCP test, then implement**

In `internal/mcpserver/mcpserver_test.go` add:

```go
func TestRecordDuplicateKeyNamesExistingEvent(t *testing.T) {
	cs := connect(t)
	d := testutil.Doc("alpha", "problem.reported", "x")
	d["idempotency_key"] = "deploy-4411"
	id := recordOK(t, cs, d)

	res := call(t, cs, "techlog_record_event", map[string]any{"event": d})
	if !res.IsError {
		t.Fatal("expected isError for a repeated idempotency key")
	}
	got := text(res)
	for _, want := range []string{id, "already recorded", "deploy-4411", "Do not retry"} {
		if !strings.Contains(got, want) {
			t.Errorf("text %q does not contain %q", got, want)
		}
	}
}

func TestRecordToolDescriptionMentionsKey(t *testing.T) {
	cs := connect(t)
	res, err := cs.ListTools(ctx, nil)
	if err != nil {
		t.Fatal(err)
	}
	for _, tool := range res.Tools {
		if tool.Name == "techlog_record_event" && !strings.Contains(tool.Description, "idempotency_key") {
			t.Errorf("description does not mention idempotency_key: %q", tool.Description)
		}
	}
}
```

Run: `go test ./internal/mcpserver/ -run 'DuplicateKey|ToolDescription' -v`
Expected: FAIL (`internal error` text; description lacks the key).

In `internal/mcpserver/mcpserver.go`:

- Replace the `techlog_record_event` description:

```go
		Description: "Record a structured Techlog event (draft 0.1). Do not supply an id: the server assigns it. idempotency_key is required and unique per actor: use one stable key per logical event and reuse it when retrying; a duplicate is rejected and names the existing event. Events are immutable; record a correction as a new event. On a validation error the result lists the field paths to fix.",
```

- Update the `recordIn.Event` jsonschema tag: `the event without an id: type, time, actor, subject, status, provenance, idempotency_key, and optionally summary, sources, evidence, links, extensions`.
- In `describe`, add `var dup *service.DuplicateError` and a case:

```go
	case errors.As(err, &dup):
		return fmt.Sprintf("Already recorded: idempotency_key %q for actor %q is event %s. "+
			"Do not retry; link to %s if later events follow from it.",
			dup.Key, dup.ActorID, dup.ExistingID, dup.ExistingID)
```

Run: `go test ./internal/mcpserver/ -count=1`. Expected: PASS.

- [ ] **Step 6: Web UI row**

In `internal/ui/ui_test.go`, inside `TestEventPage` after the first `mustContain` add:

```go
	mustContain(t, get(h, "/events/"+a), "Idempotency key")
```

and add a focused test:

```go
func TestEventPageShowsKey(t *testing.T) {
	h, svc := setup(t)
	d := testutil.Doc("alpha", "problem.reported", "keyed")
	d["idempotency_key"] = "deploy-4411"
	id := record(t, svc, d)
	mustContain(t, get(h, "/events/"+id), "Idempotency key", "<code>deploy-4411</code>")
}
```

Run `go test ./internal/ui/ -run EventPage -v` (FAIL), then in `internal/ui/templates/event.html` add after the Provenance row:

```html
  <dt>Idempotency key</dt><dd><code>{{.Event.IdempotencyKey}}</code></dd>
```

Run `go test ./internal/ui/ -count=1`. Expected: PASS. `TestEscaping` already covers HTML escaping through `html/template`.

- [ ] **Step 7: Smoke test through the real stack**

In `cmd/techlog/smoke_test.go`, after the `rec.ID` check and before the `mustHave` lines, add:

```go
	again, err := cs.CallTool(ctx, &mcp.CallToolParams{Name: "techlog_record_event", Arguments: map[string]any{"event": doc}})
	if err != nil || !again.IsError {
		t.Fatalf("recording the same event twice must be rejected: err=%v result=%+v", err, again)
	}
	mustHave(t, "duplicate result", again.Content[0].(*mcp.TextContent).Text, rec.ID)
```

Run: `go test ./cmd/... -count=1`. Expected: PASS.

- [ ] **Step 8: Run everything and commit**

Run: `go test ./... && make lint`
Expected: PASS. (If `golangci-lint` is missing, run `make tools` first.)

```bash
git add internal/api internal/mcpserver internal/ui cmd
git commit -m "feat(api): answer a repeated idempotency key with 409 and an MCP error"
```

---

### Task 5: Skill guidance, website and README

**Files:**
- Modify: `skills/techlog-record/SKILL.md`, `internal/event/skill_test.go`
- Modify: `website/src/examples/getting-started/quickstart.{sh,go,py,ts}`, `Quickstart.java`
- Modify: `website/src/examples/mcp-tools.json`
- Modify: `website/src/pages/docs/event-model.html`, `deep-dive.html`, `getting-started.html`
- Modify: `README.md`

**Interfaces:** consumes the shipped behaviour of Tasks 1-4; produces no code interfaces.

- [ ] **Step 1: Write the failing skill test**

In `internal/event/skill_test.go` add:

```go
// The skill must teach the model to choose a stable key and what to do on a
// duplicate, or retries would create the very duplicates the key prevents.
func TestSkillExplainsIdempotencyKeys(t *testing.T) {
	b, err := os.ReadFile(skillPath)
	if err != nil {
		t.Fatal(err)
	}
	s := string(b)
	for _, want := range []string{"## Retries and idempotency keys", "stable", "Already recorded", "Do not retry"} {
		if !strings.Contains(s, want) {
			t.Errorf("SKILL.md should contain %q", want)
		}
	}
}
```

Run: `go test ./internal/event/ -run TestSkillExplains -v`. Expected: FAIL.

- [ ] **Step 2: Write the skill section**

In `skills/techlog-record/SKILL.md`, add before `## If the server rejects the event`:

```markdown
## Retries and idempotency keys

Every event needs an `idempotency_key`. The server stores one event per actor and key, and rejects a second one. This is what makes a retry safe.

- Build the key from the work item, so the same fact always gets the same key: `<repo>#<pr>-merged`, `incident-<ticket>-opened`, `<system>-<version>-deployed`. It is a string of 1 to 128 characters, compared exactly.
- Make the key stable. Never use a random value, a timestamp of the attempt or a counter that restarts, or a retry will not be recognised.
- Make the key specific to one fact. Two different events from the same actor must have different keys, or the second is rejected.
- If the result says `Already recorded: idempotency_key ... is event evt_...`, the first attempt succeeded. Do not retry. Use that id for your links.
```

and add to "If the server rejects the event" the bullet:

```markdown
- `/`: `missing property 'idempotency_key'`; add the key. `/idempotency_key`: empty or longer than 128 characters.
```

Change the final line of that section to: `Do not retry a rejected event unchanged, unless the error was a lost connection: then retry with the same key.`

Run `go test ./internal/event/ -count=1`. Expected: PASS (`TestSkillExamplesAreValid` also validates the examples from Task 1).

- [ ] **Step 3: Update the quickstart snippets**

Each of the five files records two events. Add a key to each call, the same value in every language so the docs agree: `"idempotency_key": "promo-api-spin-proposal"` for the proposal and `"idempotency_key": "promo-api-spin-verification-ci-90412"` for the verification, placed after `"schema_version"`. Match each file's own syntax:

- `quickstart.sh` (inside the `-d '{...}'` JSON): `"idempotency_key": "promo-api-spin-proposal",`
- `quickstart.go` (map literal, gofmt-aligned): `"idempotency_key": "promo-api-spin-proposal",`
- `quickstart.py` (dict): `"idempotency_key": "promo-api-spin-proposal",`
- `quickstart.ts` (object): `idempotency_key: "promo-api-spin-proposal",`
- `Quickstart.java` (JSON text block): `"idempotency_key": "promo-api-spin-proposal",`

Also edit the comment above call 1 in each file to read: `The server assigns the id and, because "time" is omitted, the time. The idempotency_key makes a retry safe: reusing it is rejected with 409.` Then run `gofmt -l website/src/examples/getting-started/quickstart.go` (expect no output) and, if the toolchains are installed, `python3 -m py_compile website/src/examples/getting-started/quickstart.py`.

- [ ] **Step 4: Update the website pages and the tool contract**

- `event-model.html`: in the "Required fields" table add a row after `id`:
  `<tr><td><code>idempotency_key</code></td><td>A string of 1 to 128 characters chosen by the client, unique per <code>actor.id</code>. The server rejects a second event from the same actor with the same key. It is compared exactly: no trimming, no case folding.</td></tr>`
  Add a row to the "What the server adds" table: `<tr><td>Retries and duplicates</td><td>A submission whose <code>idempotency_key</code> the actor already used is rejected and nothing is stored. Over HTTP the response is <code>409</code> with code <code>duplicate_event</code> and <code>error.existing_id</code>; over MCP it is an error result naming the existing event. The server does not replay the original event: fetch it by id. Validation errors are reported before duplicates.</td></tr>`
- `deep-dive.html`: change "An event has eight required fields:" to "nine" and insert `<code>idempotency_key</code>` after `<code>id</code>`; replace the "Idempotency." bullet with: `<li><strong>Idempotency.</strong> <code>idempotency_key</code> is required on every event and unique per actor. A retried request is rejected as a duplicate (HTTP 409, or an MCP error result) and names the event that already exists, so a retry records one event.</li>`
- `getting-started.html`: in the "A first event" paragraph, after "Every field here is required." add `The <code>idempotency_key</code> is yours to choose: reuse it when you retry, and the server rejects the repeat.`
- `mcp-tools.json`: replace the `techlog_record_event` `description` with the exact string from Task 4 Step 5, and the `event` property description with `the event without an id: type, time, actor, subject, status, provenance, idempotency_key, and optionally summary, sources, evidence, links, extensions`.

Run `cd website && npm test && npm run build && npm run check`. Expected: all PASS (the link checker and content audit also read the pages).

- [ ] **Step 5: README**

In `README.md`, after the sentence "Data lives in `share/techlog.db` (git-ignored)." add: ` Events require an `idempotency_key`, unique per actor. When upgrading from a version without it, delete the old database first (`make flush`): the schema changed in place and there is no migration.`

- [ ] **Step 6: Commit**

```bash
git add skills internal/event/skill_test.go website README.md
git commit -m "docs(skill): teach stable idempotency keys and document the 409 contract"
```

---

### Task 6: Spec corrections, full verification

**Files:**
- Modify: `docs/superpowers/specs/2026-10-09-idempotency-key-design.md`
- Modify: `docs/superpowers/specs/2026-10-09-techlog-server-design.md` (addendum)

- [ ] **Step 1: Align the spec with what shipped**

In the idempotency spec: section 3 test list and section 4 — a missing key is reported at `/` with `missing property 'idempotency_key'`; an empty or too-long key at `/idempotency_key`. Section 4 — the example 409 message is the full `DuplicateError` text (`duplicate: idempotency_key "deploy-4411" for actor "ci-bot" was already recorded as evt_0K3F9A2B7XQZ`). Section 5, "Test fixtures": `testutil.Doc` generates a unique key per call from a counter; tests that need a specific key overwrite it.

In `2026-10-09-techlog-server-design.md` append:

```markdown
## Addendum: idempotency key

Events now require an `idempotency_key`, unique per actor. See `2026-10-09-idempotency-key-design.md`.
```

- [ ] **Step 2: Verify end to end**

Run, and read the output of each:

```bash
go test ./... -race -count=1
make lint
(cd website && npm test && npm run build && npm run check)
```

Expected: all PASS.

Then try the real server against a fresh database:

```bash
make flush FORCE=1 && make seed
make run &        # then, in another shell:
curl -sS -X POST localhost:8080/api/v1/events -H 'Content-Type: application/json' -d '{"type":"problem.reported","actor":{"kind":"human","id":"user:me"},"subject":{"system":"demo"},"status":"recorded","idempotency_key":"demo-1"}'
# repeat the same command: expect HTTP 409, code duplicate_event, existing_id set
```

Stop the server afterwards (`kill %1`).

- [ ] **Step 3: Commit**

```bash
git add docs
git commit -m "docs(spec): align the idempotency spec with the implementation"
```
