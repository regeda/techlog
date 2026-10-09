# Idempotency key for events — design

Date: 2026-10-09
Status: approved in conversation, pending written-spec review

## Problem

`Service.Record` assigns a fresh `evt_` id on every call, so a retried submission
(network timeout, agent retry) creates a duplicate event. Nothing in the contract
lets a client say "this is the same event as before".

## Decisions

| Question | Decision |
|---|---|
| Field | `idempotency_key`, top-level, snake_case |
| Required? | Yes, in the schema (`required`) |
| Who supplies it? | The client. The server never generates or defaults it |
| Uniqueness scope | Per actor: unique on `(actor.id, idempotency_key)` |
| Duplicate behaviour | Reject. No replay of the original response |
| Existing data | No migration. `001_init.sql` is edited in place; operators delete the SQLite file |
| Schema version | Stays `0.1-draft` (breaking change to a draft) |
| Enforcement | A database `UNIQUE` constraint, so concurrent retries cannot both succeed |

## 1. Contract and event model

- `internal/event/schema/event.json`: add property
  `"idempotency_key": {"type": "string", "minLength": 1, "maxLength": 128}` and
  add it to `required`. `additionalProperties: false` is unchanged. The property
  description states the scope rule (unique per `actor.id`), which the schema
  itself cannot enforce.
- `event.Event`: add `IdempotencyKey string` with tag `json:"idempotency_key"`
  (no `omitempty`).
- The key is an opaque string, compared byte for byte: no trimming, no case
  folding. `id` stays server-assigned and unrelated to the key.
- The key lives in the stored body JSON, so `Get` and search results return it.

## 2. Store

- `001_init.sql`: `events` gains `idempotency_key TEXT NOT NULL` and
  `UNIQUE (actor_id, idempotency_key)`.
- `internal/store/store.go`: add `ErrDuplicate` and
  `DuplicateError{ExistingID string}` whose `Is` method matches `ErrDuplicate`.
  `ErrConflict` keeps its meaning (generated id collided). The `Insert` doc
  comment lists `ErrDuplicate`.
- `sqlite.Insert`: today any `UNIQUE constraint failed` becomes `ErrConflict`.
  Branch on the constraint columns in the error text: `events.id` gives
  `ErrConflict`; `events.actor_id, events.idempotency_key` gives `ErrDuplicate`,
  after one `SELECT id … WHERE actor_id=? AND idempotency_key=?` in the same
  transaction to fill `ExistingID`. Nothing is stored in either case.
- No lookup-by-key method on `Store` and no key in `Filter` (YAGNI).
- Contract suite (`storetest`): same actor + same key gives `ErrDuplicate` with
  the right `ExistingID`; different actor + same key succeeds; same actor +
  different key succeeds; a rejected duplicate leaves no FTS row and no links;
  the concurrent-insert test gets a same-key case where exactly one insert wins.

## 3. Service and errors

- `Record`: defaults, schema validation (now requiring the key) and link-target
  checks run first, unchanged. The key is never defaulted or normalised.
- Insert result handling: `ErrConflict` keeps the id retry (up to
  `maxIDAttempts`). `ErrDuplicate` is not retried; `Record` returns
  `service.DuplicateError{Key, ActorID, ExistingID}`.
- `DuplicateError` (in `errors.go`) message:
  `duplicate: idempotency_key "<key>" for actor "<id>" was already recorded as <existing id>`.
- Validation (422) takes priority over duplicates (409): the duplicate check is
  the last step because the database enforces it atomically.
- A rejected duplicate has no side effects. The generated id is discarded and the
  existing event is neither returned nor modified.
- `Import` (the seeder): treat `ErrDuplicate` like `ErrConflict` and count the
  event as skipped, so re-seeding a populated store stays idempotent whichever
  constraint SQLite reports first.
- `Link`, `Get` and `Search` are untouched.
- Tests: the fake store enforces `(actor, key)` like SQLite. Cases: duplicate
  gives `DuplicateError` with `ExistingID` and no id retry; missing key gives
  `InvalidError` at `/idempotency_key`; id collision still retries and succeeds;
  same key under another actor succeeds; `Import` skips duplicates.

## 4. HTTP API and MCP

- `POST /api/v1/events`: `fail` maps `*service.DuplicateError` to **409** with
  code `duplicate_event`. The error body gains an optional `error.existing_id`
  (omitted for every other error):
  ```json
  {"error": {"code": "duplicate_event",
             "message": "idempotency_key \"deploy-4411\" for actor \"ci-bot\" was already recorded",
             "existing_id": "evt_0K3F9A2B7XQZ"}}
  ```
  A missing or empty key stays a 422 with a `details` entry at `/idempotency_key`.
- MCP `techlog_record_event`: `describe` handles `DuplicateError` with an error
  result (`IsError: true`) saying the event was already recorded as `<existing id>`
  and that the caller should not retry. The tool description adds:
  "`idempotency_key` is required and unique per actor. Use one stable key per
  logical event and reuse it when retrying. A duplicate is rejected and names the
  existing event." `recordOut` is unchanged.
- Neither transport replays the original event; both reject.
- Web UI is read-only. The event detail page renders selected fields, so add one
  row, `<dt>Idempotency key</dt><dd>…</dd>`, to `internal/ui/templates/event.html`
  and cover it in `ui_test.go`.
- Tests: `api_test.go` (409 body, 422 for missing key, other actor gives 201);
  `mcpserver_test.go` (error text contains the existing id); `smoke_test.go`
  (the same event recorded twice through the real stack is rejected the second time).

## 5. Seed, skill, docs and examples

- Seed (`internal/seed/spring_spin.json`): every event gets a deterministic key
  (for example `seed-spring-spin-01`). `seed.go`/`Import` already skip existing
  events; see section 3.
- Skill (`skills/techlog-record/SKILL.md`): add the field to the field list and
  example payloads. Guidance: derive a stable key from the work item (for example
  `<repo>#<pr>-merged`, `incident-<ticket>-opened`), never a random value per
  attempt. On a duplicate error, link to the existing id instead of retrying.
  `skill_test.go` already validates the skill's examples against the schema.
- Website: add the key to the 10 event example JSONs, the five quickstart snippets
  (`.go`, `.py`, `.ts`, `.java`, `.sh`) and `json-schema-core.json`. Update
  `event-model.html` (required, unique per actor, opaque string, 1–128
  characters) and add a short "Retries and duplicates" note. Other docs pages
  (`getting-started`, `deep-dive`, `standard`, `security`) change only where they
  list required fields.
- Repo docs: add a short addendum to
  `docs/superpowers/specs/2026-10-09-techlog-server-design.md` pointing here. The
  old plan stays untouched.
- Test fixtures: a shared helper takes the key as an argument so no fixture is
  hand-edited twice.
- README: one line saying to delete the existing SQLite file after upgrading.

## Out of scope

Key rotation or expiry (keys are kept forever; events are immutable), a
lookup-by-key endpoint, replay of the original response, a schema version bump,
and data migration.
