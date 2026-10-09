---
name: techlog-record
description: Use when work in this conversation reaches something worth remembering about a system (a problem found, a proposal, a decision, a merged change, a release, an incident, a rollback, a postmortem, a follow-up action) to record it as an event in Techlog, and when asked what happened to a system earlier. Records through the Techlog MCP tools and links each new event to the earlier events it follows from.
---

# Recording events in Techlog

Techlog is an append-only log of engineering events. Each event says what happened to a system and links back to the events that led to it, so a person can later walk the story from problem to follow-up. Record an event when something worth remembering has happened. Do not record chatter, plans you have not decided on, or every small edit.

## Connect

Use only the Techlog MCP tools: `techlog_search_events`, `techlog_get_system_history`, `techlog_get_event`, `techlog_record_event`, `techlog_link_events`. Assume a Techlog server is running and reachable through them. Do not reach it any other way, and never record events anywhere else.

If the tools are not available, or a call fails with a connection error, the MCP connection is not configured. Stop and ask the user to configure it, for example:

```bash
claude mcp add --transport http techlog <techlog-server-url>/mcp
```

then to reconnect with `/mcp` or restart Claude Code. Say plainly that you cannot record until it works, and wait. Do not try to start, stop or repair the server yourself.

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
- `idempotency_key`: required. A stable string you choose, unique per actor, up to 128 characters. See "Retries and idempotency keys" below.
- `type`, `status`: from the table above
- `time`: when it happened, RFC 3339 with a zone, for example `2026-03-12T13:38:00Z`. Leave it out for something that has just happened, and the server stamps the time of recording. For past work, use a time you can read from a source, such as a git commit time (`git log --format=%aI`). Never invent precision: if a time is approximate, say so when you report to the user.
- `actor`: `{"kind": "ai_agent", "id": "agent:claude-code", "on_behalf_of": "user:<who asked>"}`. Leave `on_behalf_of` out if you do not know who it is.
- `subject`: `{"system": "...", "component": "..."}`; `component` and `environment` are optional
- `summary`: one plain sentence, at most 280 characters, that makes sense without context
- `links`: `[{"rel": "...", "target": "evt_...", "basis": "confirmed"}]`
- `sources` (where it came from) and `evidence` (supporting artifacts), each `{"kind": "...", "ref": "..."}`, when you have them. Evidence is a reference: the artifact stays in the tool that owns it.
- `provenance`: `{"recorded_by": "agent:claude-code", "method": "agent_submitted"}`

The result carries the new `id` in its structured result. Use it to link later events.

To record several related events, such as the story of a piece of work, record the earliest first. A link needs its target to exist, so each event can link only to events that already have an id. Check the order against the table above (problem, proposal, decision, change, verification, release).

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
  "idempotency_key": "promo-api-spin-proposal-approved",
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
  "idempotency_key": "promo-api-3.8.0-canary-5xx-incident",
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

## Retries and idempotency keys

Every event needs an `idempotency_key`. The server stores one event per actor and key, and rejects a second one. This is what makes a retry safe.

- Build the key from the work item, so the same fact always gets the same key: `<repo>#<pr>-merged`, `incident-<ticket>-opened`, `<system>-<version>-deployed`. It is a string of 1 to 128 characters, compared exactly.
- Make the key stable. Never use a random value, a timestamp of the attempt or a counter that restarts, or a retry will not be recognised.
- Make the key specific to one fact. Two different events from the same actor must have different keys, or the second is rejected.
- If the result says `Already recorded: idempotency_key ... is event evt_...`, the first attempt succeeded. Do not retry. Use that id for your links.

## If the server rejects the event

The result has `isError: true` and lists the problems as `- <JSON pointer path>: <message>`. Fix exactly those fields and call the tool again. Common ones:

- `/actor/kind`: must be `human`, `ai_agent` or `system`.
- `/time`: needs a time zone, `Z` or an offset.
- `/summary`: over 280 characters; shorten it.
- `/links/0/target`: that event id does not exist; search again for the right one.
- `/links/0`: `missing property 'basis'`; `caused_by` links must say `confirmed` or `hypothesis`.
- `/`: `missing property 'idempotency_key'`; add the key. `/idempotency_key`: empty or longer than 128 characters.

Do not retry a rejected event unchanged, unless the error was a lost connection: then retry with the same key.

## Rules

- One event per fact. A change that shipped and a test that passed are two events.
- Events are immutable. To correct one, record a new event; optionally add a link to it with `techlog_link_events`. A wrong event cannot be removed, so check every fact before you record it.
- Read before you write: an event that duplicates one already in the history is noise.

## With superpowers

The superpowers skills run the engineering workflow; this skill records what comes out of it. Record at the end of a stage, once, after the user has approved or the result is confirmed:

| Superpowers stage | Record | Evidence or source |
|---|---|---|
| `brainstorming`: design or spec written | `proposal.recorded` | `{"kind": "spec", "ref": "docs/superpowers/specs/<file>.md"}` |
| `brainstorming`: user approves the design | `decision.made`, linked `decides` to the proposal | the same spec |
| `systematic-debugging`: root cause found | `investigation.completed` | log, trace or test that shows it |
| `verification-before-completion`: checks ran | `verification.completed` (`verified` or `failed`) | the command run |
| `finishing-a-development-branch`: merged | `change.implemented`, linked `implements` to the decision | `{"kind": "commit", "ref": "<sha>"}` |

Plans and in-progress edits are not events. Search first, so the new event links to the proposal or decision already recorded.
