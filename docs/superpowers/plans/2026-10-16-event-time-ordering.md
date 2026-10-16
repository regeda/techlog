# Event time ordering Implementation Plan

> **For agentic workers:** REQUIRED SUB-SKILL: Use superpowers:subagent-driven-development (recommended) or superpowers:executing-plans to implement this plan task-by-task. Steps use checkbox (`- [ ]`) syntax for tracking.

**Goal:** `service.Record` rejects event times beyond `now + skew` and times earlier than any linked event, and stores every event time as UTC.

**Architecture:** Both rules live in `Service.prepare()` in `internal/service/service.go`, next to the existing link-target lookup, and add to the same `problems` list, so HTTP and MCP behave the same. The clock skew is a `Service` option with a 5-minute default, exposed as a `serve` flag.

**Tech Stack:** Go, standard `testing`, in-memory `fakeStore` for service tests.

**Spec:** `docs/superpowers/specs/2026-10-16-event-time-ordering-design.md`

## Global Constraints

- Default skew is 5 minutes; skew `0` means no tolerance; a negative skew makes `service.New` fail.
- A time exactly at `now + skew` passes; a time exactly equal to a link target's time passes.
- Comparisons are on instants (parsed times), never on text.
- Stored time is the UTC instant written with `Z` (`time.RFC3339Nano` layout, so sub-seconds are kept).
- `Service.Link` / `techlog_link_events` gets no ordering rule.
- Events already stored are never re-checked or migrated.
- Failures use the existing 422 `invalid_event` shape: `InvalidError` with `event.Problem{Path, Message}`; paths are `/time` and `/links/<i>`.
- Commit messages follow Conventional Commits, with no attribution trailers.

## Review Focus

- A time exactly at `now + skew` and one second over (boundary).
- An offset time whose text looks later than now but is earlier as an instant, and the reverse.
- A client time with fractional seconds keeps its fraction after UTC normalisation.
- A link target stored with a non-UTC offset compares correctly as an instant.
- An event with no `time` and a link target dated slightly in the future is not rejected.

---

### Task 1: Clock skew option, future check, UTC normalisation

**Files:**
- Modify: `internal/service/service.go` (`Service`, `New`, `Record`, `Import`, `prepare`)
- Test: `internal/service/service_test.go`

**Interfaces:**
- Produces: `const DefaultClockSkew = 5 * time.Minute`; `func WithClockSkew(d time.Duration) Option`; `New` returns an error when the skew is negative; `prepare(ctx context.Context, doc map[string]any, now time.Time, stamped bool) (event.Event, error)`. `Record` takes one `s.now()` reading and uses it for the stamp, the id and `prepare`; `stamped` is true when the client sent no `time`; `Import` passes `s.now()` and `false`.

- [ ] **Step 1: Write the failing tests** in `service_test.go` (`fixedNow` is 2026-03-12T10:00:00Z):
  - `TestRecordRejectsFutureTime`: table of `time` values on a doc without links, using `newService`. Passes: `2026-03-12T10:05:00Z` (boundary), `2026-03-12T11:30:00+02:00` (09:30Z although its text reads later than now). Fails with `/time` in `problemPaths(err)` and no insert: `2026-03-12T10:05:01Z`, `2026-03-12T12:30:00+02:00` (10:30Z).
  - `TestRecordStoresTimeAsUTC`: `2026-03-11T12:00:00+02:00` is stored as `2026-03-11T10:00:00Z`; `2026-03-11T10:00:00.250+00:00` is stored as `2026-03-11T10:00:00.25Z`; the returned event's `Time` equals the stored one.
  - `TestClockSkewOption`: build `service.New(newFakeStore(), service.WithClock(...), service.WithClockSkew(0))`: `fixedNow` passes, `fixedNow+1s` fails at `/time`. With `WithClockSkew(time.Hour)`, `fixedNow+59m` passes. With `WithClockSkew(-time.Second)`, `service.New` returns an error.
- [ ] **Step 2: Run** `go test ./internal/service/ -run 'FutureTime|StoresTimeAsUTC|ClockSkew' -v`. Expected: FAIL (`WithClockSkew` undefined).
- [ ] **Step 3: Implement** the option, the `DefaultClockSkew` default set in `New` before options run, and the negative check after them. In `prepare`, after the existing `ParsedTime` parse succeeds, add a problem at `/time` when the instant is after `now.Add(skew)` (skip when `stamped`), then set `ev.Time = t.UTC().Format(time.RFC3339Nano)`. The message states the allowed skew.
- [ ] **Step 4: Run** `go test ./internal/service/ -v`. Expected: PASS. Run `go test ./...` and fix fixtures that use times later than the real clock plus 5 minutes, or that assert the stored time keeps its offset (`internal/api/api_test.go:235` sorts on offset text). Change only fixtures and expectations, not the rule.
- [ ] **Step 5: Commit**

```bash
git add internal/service internal/api
git commit -m "feat(service): reject future event times and store times as UTC"
```

### Task 2: Link order check and stamped-time raise

**Files:**
- Modify: `internal/service/service.go` (`prepare`)
- Test: `internal/service/service_test.go`

**Interfaces:**
- Consumes: `prepare(ctx, doc, now, stamped)` and `DefaultClockSkew` from Task 1.
- Produces: no new names. `prepare` reuses the event returned by the existing `s.store.Get(ctx, l.Target)` call.

- [ ] **Step 1: Write the failing tests** in `service_test.go`, with a target recorded from `testutil.Doc` (time `2026-03-10T09:00:00Z`):
  - `TestRecordRejectsTimeBeforeLinkTarget`: `2026-03-10T08:59:59Z` fails with `/links/0`; `2026-03-10T09:00:00Z` (equal) passes; `2026-03-10T10:59:00+02:00` (08:59Z) fails; with two links where only the second target is later, paths contain `/links/1` and not `/links/0`; a dangling target with an early time reports only `/links/0/target`.
  - `TestRecordReportsAllTimeProblems`: a future time plus a link before its target gives both `/time` and `/links/0` in one `InvalidError`.
  - `TestRecordSkipsTimeChecksOnBadTime`: time `2026-03-11T10:12:00` (no zone) with a link gives only `/time`.
  - `TestRecordStampedTimeRaisedToLinkTarget`: a target recorded with `2026-03-12T10:04:00Z`; an event with no `time` and a link to it gets `Time == "2026-03-12T10:04:00Z"`; the same event without a link gets `2026-03-12T10:00:00Z`.
  - `TestRecordComparesOffsetTargetAsInstant`: insert a target directly through the fake store with time `2026-03-10T12:00:00+02:00`; an event at `2026-03-10T09:59:00Z` fails, at `10:00:00Z` passes.
  - `TestRecordSkipsLinkCheckForUnparsableTargetTime`: insert a target directly with time `not a time`; the event passes.
  - `TestLinkIgnoresTimeOrder`: `svc.Link` between two events with the source earlier than the target succeeds.
- [ ] **Step 2: Run** `go test ./internal/service/ -run 'TimeBefore|AllTimeProblems|BadTime|StampedTime|OffsetTarget|UnparsableTarget|LinkIgnoresTimeOrder' -v`. Expected: FAIL for the new rules, PASS for `TestLinkIgnoresTimeOrder`.
- [ ] **Step 3: Implement** in `prepare`. Keep each target's parsed time while looping over links. For each existing target whose time parses, if the event's parsed time is earlier, add a problem at `/links/<i>` that names the target id and its time. When `stamped`, replace the future check and the link check with raising `ev.Time` to the latest target time if that is later than the stamp (then normalise as in Task 1). Skip both checks when the event's own time failed to parse.
- [ ] **Step 4: Run** `go test ./... && go vet ./...`. Expected: PASS. Fix fixtures that now link to a later target (change times only).
- [ ] **Step 5: Commit**

```bash
git add internal/service
git commit -m "feat(service): reject events dated before the events they link to"
```

### Task 3: Server flag and documentation

**Files:**
- Modify: `cmd/techlog/serve.go`
- Modify: `docs/superpowers/specs/2026-10-09-techlog-server-design.md` (step 5 of `service.Record`, and the stored time form)
- Modify: `plugin/skills/record/SKILL.md` ("Which time to use": one line that the time must not be earlier than the events linked to)
- Test: `cmd/techlog/main_test.go`; run `internal/plugin` tests

**Interfaces:**
- Consumes: `service.WithClockSkew`, `service.DefaultClockSkew`.

- [ ] **Step 1: Write the failing test** `TestServeRejectsNegativeClockSkew` in `cmd/techlog/main_test.go`: `runServe(ctx, []string{"-clock-skew", "-1s", <store flags for t.TempDir()>}, io.Discard)` returns an error before listening.
- [ ] **Step 2: Run** `go test ./cmd/techlog/ -run ClockSkew -v`. Expected: FAIL (flag not defined).
- [ ] **Step 3: Implement** a `-clock-skew` duration flag on `serve`, default `service.DefaultClockSkew`, help text "how far ahead of the server clock an event time may be", passed to `service.New` as `service.WithClockSkew`. Update the two documents.
- [ ] **Step 4: Run** `go test ./... && go vet ./...`. Expected: PASS (the plugin tests may pin skill text; update them with the new line if they do).
- [ ] **Step 5: Commit**

```bash
git add cmd docs plugin
git commit -m "feat(cmd): add a -clock-skew flag and document the event time rules"
```
