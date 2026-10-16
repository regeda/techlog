# Event time ordering — design

Date: 2026-10-16
Status: approved; implemented on feat/event-time-ordering
Problem: Techlog event `evt_MVATV0VCEFGG` (`problem.reported`, system `techlog`, component `record_event`)

## Problem

`Service.Record` stores the `time` a client sends as it is. Nothing checks it against the
clock or against the events the new event links to. A client can send a time in the future,
or a time earlier than the event it follows from (a `decision.made` dated before its
`problem.reported`), and a time with any zone offset is kept in that offset. The timeline of
a story then misleads and an audit of it cannot be trusted.

## Intent

A story read from its last event back along its links must go back in time, or stay at the
same time, at every step. The server enforces this, so no client can break it. An event
cannot claim to have happened in the future.

## Goals

- Reject an event whose time is later than the server's clock plus an allowed skew.
- Reject an event whose time is earlier than the time of any event it links to.
- Store every event time normalised to UTC.
- Behave the same for HTTP and MCP, because both go through `service.Record`.

## Non-goals

- No ordering rule for links added later with `Service.Link` / `techlog_link_events`. Those
  may link `relates_to` in any order. This is a decision, not an omission.
- No re-check, migration or repair of events already stored. Events are immutable.
- No silent correction of a client's time (clamping, restamping). The event is rejected.
- No limit on how far in the past a time may be, beyond the link rule.

## Anti-goals

- A rule that reads as strict but can be bypassed by a zone offset: comparisons are on the
  instant, never on the text.
- A rejection that hides the other problems: the new problems join the existing list.

## Decisions

| Question | Decision |
|---|---|
| Where is it enforced | `service.Record`, in `prepare()`, after schema validation (approach A) |
| Future limit | `now + skew`; skew defaults to 5 seconds |
| Skew setting | A server option, default 5s; `0` means no tolerance |
| Which links | Every link on the new event, whatever its `rel` |
| Equal times | Allowed (an event may have the same time as a link target) |
| Links added by `Link` | No ordering rule |
| Stored time | The UTC instant, written with `Z` |
| Missing `time` | Unchanged: the server stamps its own now, which passes by construction |
| Failure shape | HTTP 422 `invalid_event` with `details` as today, MCP result as today |

### Rejected approaches

- **Express it in the JSON Schema.** A schema cannot know the server clock or another
  event's time, and the schema file must stay identical to the website copy.
- **Enforce it in the store (SQL trigger or constraint).** It splits the rules across two
  layers and gives poorer error details.
- **Silently correct bad times (clamp or restamp).** It contradicts the rule that a client
  cannot state such a time, hides client bugs, and rewrites what the client claimed.

## Behaviour

`prepare()` already parses `ev.ParsedTime()` and looks up each link target. Add, in the same
pass and into the same `problems` list:

1. **Future check.** If `time` is later than `s.now() + skew`, add a problem at `/time`
   saying the time is in the future beyond the allowed clock skew (state the skew).
2. **Link order check.** For each link whose target exists, if the event's time is earlier
   than the target's time, add a problem at `/links/<i>` that names the target and its time.
   A target that does not exist is skipped, since it is already reported at
   `/links/<i>/target`.
3. **Normalisation.** After a time is parsed, the stored time is that instant in UTC with
   `Z`. The comparisons use the instant, so an offset such as `+02:00` cannot change the
   outcome.

Both checks run for every `Record` call, whether it came from HTTP or MCP.

### Edge cases (my calls)

- **Server-stamped time.** When the client sends no `time`, the server stamps `s.now()`.
  That time is exempt from the future check. If a link target is dated later than the stamp
  and not later than now plus the skew, the stamp is raised to the latest target time. A target
  dated beyond that (only legacy events can be) cannot be followed by a stamp, so the event is
  rejected at `/links/<i>` rather than silently given a far-future time.
- **Bad time.** If `time` fails schema validation or parsing, both new checks are skipped, so
  one bad time gives one problem.
- **Target times.** `prepare()` already calls `store.Get` for each target, which returns the
  full event, so its time is read from that result and no store interface change is needed.
  The comparison parses the stored time with `ParsedTime` and compares instants, so legacy
  events stored with an offset compare correctly. A stored time that does not parse skips
  the check for that link.
- **Clock reading.** Take one `s.now()` reading per `Record` call and use it for the stamp,
  the id and the future check.

### Retries

Duplicate detection happens at `store.Insert`, after `prepare()`, and that order does not
change. A retry with the same idempotency key and the same time passes the checks again,
because the future check only gets easier as the clock moves on and target times never
change, and then ends in the existing duplicate result.

## Configuration

- `service.WithClockSkew(d time.Duration)` option, default 5 seconds.
- The server exposes it as a configuration option. Its name and form follow however the
  server is configured today; the builder picks them. A negative value is a startup error
  (my call).

## Tests

- A time inside the skew passes; one just over it fails; the clock is faked with `WithClock`.
- An offset time (`+02:00`) that is really in the future fails; one that is really in the
  past passes and is stored as UTC with `Z`.
- A link target dated later than the event fails at `/links/<i>`; an equal time passes.
- Several problems in one event are all reported in one 422.
- `Link` still accepts any order.
- A skew of `0` and a custom skew behave as set.

## Documentation

- `docs/superpowers/specs/2026-10-09-techlog-server-design.md`: step 5 of `service.Record`
  gains the two rules, and the stored time form is stated.
- The plugin skills that tell agents how to choose `time` get one line saying the time must
  not be earlier than the events linked to.

## Left to the builder

- The configuration option's name and where it is read.
- The exact wording of the error messages.
- Whether sub-second precision is kept when normalising (keeping it is the safer default).
- Whether existing example events in the docs and website need their times adjusted.
