---
name: record
description: Use when work in this conversation reaches something worth remembering about a system (a problem found, a proposal, a decision, a merged change, a release, an incident, a rollback, a postmortem, a follow-up action) to record it as an event in Techlog, and when asked what happened to a system earlier.
---

# Recording events in Techlog

Techlog is an append-only log of engineering events. Each event says what happened to a system and links back to the events that led to it, so a person can later walk the story from problem to follow-up. Record an event when something worth remembering has happened, not chatter, undecided plans or small edits.

## Connect

Use only the Techlog MCP tools: `techlog_search_events`, `techlog_get_system_history`, `techlog_get_event`, `techlog_record_event`, `techlog_link_events`. Assume a Techlog server is running and reachable through them. Do not reach it any other way, and never record events anywhere else.

If the tools are not available, or a call fails with a connection error, the MCP connection is not configured. Stop, point the user to `/techlog:welcome how to connect techlog mcp`, say plainly that you cannot record until it works, and wait. Do not try to start, stop or repair the server yourself.

## Before you record

1. Search for what already exists: `techlog_search_events` (free text, `system`, `types`, `since`) or `techlog_get_system_history` (latest events of one system).
2. Pick the earlier events this one follows from. Their ids (`evt_...`) go in `links`. A link points backward in time, from the new event to the earlier one.
3. Reuse the system name that history already uses (`promo-api`, not `Promo API`).

## Choose the event type

| What happened | `type` | usual link `rel` to an earlier event |
|---|---|---|
| A need, defect or request was raised | `problem.reported` | `relates_to` a similar report |
| Someone examined a problem or failure and has findings | `investigation.completed` | `investigates`, `caused_by` |
| A solution was proposed | `proposal.recorded` | `motivated_by` |
| A decision was taken | `decision.made` | `decides` |
| A change was built and merged | `change.implemented` | `implements` |
| Tests or checks ran | `verification.completed` | `verifies` |
| A version reached an environment | `release.deployed` | `releases` |
| Something broke in production | `incident.opened` | `observed_after`, `caused_by` |
| A temporary measure reduced impact | `mitigation.applied` | `mitigates` |
| A release was reverted | `rollback.executed` | `reverts`, `mitigates` |
| Health was confirmed after a fix | `recovery.verified` | `verifies` |
| An incident review was written | `postmortem.published` | `follows_up` |
| A follow-up action was agreed or finished | `action.tracked` | `follows_up` |

## Fill in the event

Call `techlog_record_event` with `event` set to an object with these fields. Do not send an `id`.

- `schema_version`: `"0.1-draft"`
- `idempotency_key`: required. A stable string you choose, unique per actor, up to 128 characters. See "Retries and idempotency keys" below.
- `type`: from the table above
- `time`: when it happened, RFC 3339 with a zone, for example `2026-03-12T13:38:00Z`. Take it from the source of the change when it has one, and otherwise leave it out. See "Which time to use" below.
- `actor`: `{"kind": "ai_agent", "id": "agent:claude-code", "on_behalf_of": "user:<who asked>"}`. Leave `on_behalf_of` out if you do not know who it is.
- `subject`: `{"system": "...", "component": "..."}`; `component` and `environment` are optional
- `summary`: one plain sentence, at most 280 characters, that makes sense without context
- `links`: `[{"rel": "...", "target": "evt_...", "basis": "confirmed"}]`
- `sources` (where it came from) and `evidence` (supporting artifacts), each `{"kind": "...", "ref": "..."}`, when you have them. Evidence is a reference: the artifact stays in the tool that owns it.
- `provenance`: `{"recorded_by": "agent:claude-code", "method": "agent_submitted"}`

The result of a check or the state of an action goes in the `summary`, for example "Checks failed: 3 of 40 tests". The result carries the new `id`; use it to link later events.

To record several related events, record the earliest first: a link needs its target to exist. The order follows the table above (problem, proposal, decision, change, verification, release).

## Which time to use

`time` says when the thing happened, not when you recorded it. Pick it in this order:

1. **The time the source of the change records.** A commit: `git log -1 --format=%aI <sha>` (author time) or `%cI` (commit time, for a merge or rebase). A pull request: its merge time. A release: the deploy time. An alert or incident: the time on the alert or ticket. If a source has a timestamp, use it, even for something that just happened.
2. **No time, if the source has no timestamp** or there is no source (a decision taken in this conversation, a finding you just made). Leave `time` out and the server stamps the moment of recording in UTC. Never send the date from the conversation as a time, and never send a time of day you did not read from a source.

A time must not be in the future or earlier than any event the new one links to: the server rejects it. Fix the time or the link. Never invent a time and never guess one; if the source gives only a date or an approximate time, use it and say so in the summary.

## Links and honesty

- `basis: "confirmed"` means the relationship is established, such as a change that implements a proposal.
- `basis: "hypothesis"` means you suspect it. Use it for any cause you have not proven. `caused_by` must always state its basis. Prefer a hypothesis you can later confirm over a confident claim you cannot support.
- To confirm a hypothesis later, record a new `investigation.completed` event with a `caused_by` link whose basis is `confirmed`. You can also add a link between two existing events with `techlog_link_events`.
- Never record something you did not observe or were not told. If you are unsure, say so in the summary.

## Example

An incident, with a suspected cause recorded as a hypothesis:

```json event
{
  "schema_version": "0.1-draft",
  "idempotency_key": "promo-api-3.8.0-canary-5xx-incident",
  "type": "incident.opened",
  "time": "2026-03-12T13:38:00Z",
  "actor": {"kind": "ai_agent", "id": "agent:claude-code", "on_behalf_of": "user:a.novak"},
  "subject": {"system": "promo-api", "component": "spin-service", "environment": "production"},
  "summary": "5xx ratio on the spin endpoint is 6.2% on the 3.8.0 canary, above the 1.0% rollback threshold",
  "evidence": [{"kind": "alert", "ref": "alert://promo-api/high-error-rate/a-5521"}],
  "links": [
    {"rel": "observed_after", "target": "evt_01J9K2A0P010", "basis": "confirmed"},
    {"rel": "caused_by", "target": "evt_01J9K2A0P006", "basis": "hypothesis"}
  ],
  "provenance": {"recorded_by": "agent:claude-code", "method": "agent_submitted", "confidence": "asserted"}
}
```

## Retries and idempotency keys

Every event needs an `idempotency_key`: a stable string built from the work item (`<repo>#<pr>-merged`, `<system>-<version>-deployed`), 1 to 128 characters, unique per fact. The server rejects a second event with the same actor and key, which makes a retry safe.

If the result has `isError: true` or `Already recorded`, read `${CLAUDE_PLUGIN_ROOT}/skills/record/references/errors.md` before you retry. Do not retry a rejected event unchanged. For the shape of another event type, read `${CLAUDE_PLUGIN_ROOT}/skills/record/references/examples.md`.

## Rules

- One event per fact. A change that shipped and a test that passed are two events.
- Events are immutable. To correct one, record a new event; optionally add a link to it with `techlog_link_events`. A wrong event cannot be removed, so check every fact before you record it.
- Read before you write: an event that duplicates one already in the history is noise.

## With superpowers

The superpowers skills run the engineering workflow; this skill records what comes out of it. Record at the end of a stage, once, after the user has approved or the result is confirmed:

| Superpowers stage | Record | Evidence or source |
|---|---|---|
| `systematic-debugging`: root cause found | `investigation.completed` | log, trace or test that shows it |
| `verification-before-completion`: checks ran | `verification.completed` (`verified` or `failed`) | the command run |
| `finishing-a-development-branch`: merged | `change.implemented`, linked `implements` to the decision | `{"kind": "commit", "ref": "<sha>"}` |

Proposals and decisions are owned by `/techlog:proposal` and the decision rule in `mappings.yaml`, so this skill does not record them on its own at a `brainstorming` stage. Plans and in-progress edits are not events. Search first, so the new event links to the proposal or decision already recorded.
