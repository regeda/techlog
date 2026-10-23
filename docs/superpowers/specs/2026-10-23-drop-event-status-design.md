# Drop the event `status` field — design

Date: 2026-10-23
Status: draft, pending approval
Problem: Techlog event `evt_MVL6AERMSKAA` (`problem.reported`, system `techlog`, component `event schema`)
Evidence of the defect: `evt_MVL59QOXQ7AS` (`investigation.completed` with status `recorded`)

## Problem

An event has a `type` and a `status`. The type already says what happened and its outcome
(`investigation.completed`), so the status either repeats it (`recorded`) or can contradict it
(`investigation.completed` with `open`). `record_event` accepts such a pair, so a reader of the history
cannot trust an event's state.

## Intent

An event says one thing, once: its type. There is no second field to keep consistent, so no event can
contradict itself.

## Decision taken with the user

- Techlog does not need a status. The user's reason: it duplicates the type.
- The one thing status carried that the type does not, such as the result of a verification, goes in the
  `summary` ("Checks failed: 3 of 40 tests"). The user decided this for `verification.completed`. The
  same rule applies to `action.tracked` (open, in progress, resolved): the state goes in the summary, and
  when the action moves on, a new event says so. This extension is an assumption of the design, put to the
  user and not contradicted.
- `status` is removed everywhere: schema, database, API, UI, plugin skills, website docs and examples,
  seed data. Later decision of the user: no migration, "start from scratch". The status column is edited out
  of `001_init.sql`, existing databases are discarded and recreated, and the stored values of recorded
  events are not preserved. This replaces the earlier choice to remove the stored values by migration.
- A client that still sends `status` (an older plugin copy, any other client) is rejected like any other
  unknown field. Validation has no exception: the server already fails on unknown fields (the schema sets
  `additionalProperties: false`), and that stays. The user first chose to ignore `status`, then asked for
  strict validation and chose rejection.

## Goals

- No `status` in the event model, the stored data, any response or any page.
- A client that still sends `status` gets a clear validation error, the same as for any unknown field, so
  it can be fixed.
- Existing events remain readable and keep their ids, links, times and summaries.

## Non-goals

- No replacement field, no per-type allowed-state rules, no lifecycle model.
- No change to event types, links or the idempotency rules.
- No schema version bump: the model is still `0.1-draft`.

## Anti-goals

- A status that survives in the stored event body, in an API response or on a page counts as a failure,
  even if the schema no longer lists it.
- A special case that lets `status` through while other unknown fields fail is a failure.

## What exists today (verified in the repo)

- `internal/event/schema/event.json` lists `status` as required and as an enum of nine values, with
  `additionalProperties: false` at the top level and on its sub-objects. `service.Record` validates every
  incoming event against it, so unknown fields already fail today.
- `internal/event/event.go:75` has `Status string` on the event type.
- `internal/store/sqlite/migrations/001_init.sql` has a `status TEXT NOT NULL` column, and the `body`
  column holds the full event JSON, so a stored event also carries `status` inside `body`.
- `internal/store/sqlite/sqlite.go:165` writes the column.
- `internal/mcpserver/mcpserver.go` returns `Status` from `record_event` (lines 97 and 158) and prints it
  in search lines (line 229).
- `internal/ui/templates/` (`events.html`, `event.html`, `system.html`, `story.html`), `ui.go`, and the
  `.status` rule in `internal/ui/static/style.css` show it.
- `plugin/skills/record/SKILL.md` has the "usual status" column and the list of nine statuses.
  `plugin/skills/problem/SKILL.md`, `plugin/skills/proposal/SKILL.md` and the decision rule in
  `plugin/config/mappings.yaml` set `recorded` or `proposed`.
- Website: `website/src/pages/docs/event-model.html`, the lifecycle page, `mcp.html`, `docs/index.html`,
  and `website/src/examples/*.json`.
- `internal/seed/spring_spin.json` and the test helpers (`storetest`, `sqlite_test.go`,
  `mcpserver_test.go`) set or check status.

## Design

1. **Schema.** Remove `status` from `required` and from `properties` in `event.json`. The schema keeps forbidding extra
   properties, so an incoming event with `status` fails validation like any unknown field. Nothing
   strips or special-cases it. The error should name the field so a client can see what to remove.
2. **Go type.** Remove `Status` from the event type and from every use in the store, the service, the MCP
   server and the API.
3. **Database.** Remove the `status` column from `events` in `001_init.sql`. There is no migration: an
   existing database file does not work with the new binary (its `status NOT NULL` column rejects every
   insert) and is deleted and recreated. Links, the full-text index and the unique
   `(actor_id, idempotency_key)` rule stay as they are.
4. **MCP and API responses.** `record_event` returns the id and the event without a status. Search and
   history lines drop the `[status]` part.
5. **UI.** Remove the Status column and field from every template and the unused CSS rule.
6. **Plugin.** Remove the status column, the status list and every instruction to set a status from the
   skills and from `mappings.yaml`. Add one sentence to `record/SKILL.md`: the result of a check or the
   state of an action goes in the summary. Keep "events are immutable; record a new event when the
   situation changes".
7. **Website.** Remove status from the event model, lifecycle and MCP pages and from every example
   JSON.
8. **Seed and tests.** Remove status from `spring_spin.json` and the test helpers. Add tests: an event
   with `status` is rejected with an error that names the field, and so is an event with any other unknown
   field; a new database has no `status` column and an
   event read back from it has no `status` key.

## Constraints

- Events are immutable: no code edits a stored event.
- The Techlog server instance the user runs holds real events (such as `evt_MVL59QOXQ7AS`). They are
  discarded when the database is recreated; the user accepted this ("do not care about migration").

## Options turned down

- **Keep status and validate it against the type** (for example `investigation.completed` only with
  `recorded`). Turned down: the user's finding is that status duplicates the type, so validation would
  protect a field with no value of its own.
- **Keep status only where it adds something** (`verification.completed`, `action.tracked`,
  `release.deployed`). Turned down: the user chose to put the result in the summary, and two kinds of event
  would then follow different rules.
- **Ignore `status` when a client sends it.** First chosen, then turned down by the user in favour of
  strict validation: it would make `status` the one unknown field that passes, and a client could keep
  sending a field that does nothing. The cost is that older plugin copies fail until they are updated.
- **Keep the stored values and only hide them** (drop from responses and pages, keep the column and the
  `body` key). Turned down by the user: "remove status everywhere".
- **Bump to `0.2-draft`.** Turned down: the model is still a draft, and a clear validation error is enough
  to tell a client the field is gone.

## Left to the builder

- Whether the validation error already names the unknown field; if not, make it do so.
- Whether the plugin version needs a bump so users update their copy; the plugin and the server ship together
  and an old plugin fails on every record.
- Whether the lifecycle page on the website shrinks or is removed, once it has no statuses to describe.
- The wording of the summary-based guidance in the plugin skills.
