# Let the server stamp the time when the plugin has no source time — design

Date: 2026-10-19
Status: draft, pending approval
Problem: Techlog event `evt_MVF4N8K1O8SP` (`problem.reported`, system `techlog-plugin`)
Evidence of the defect: `evt_MVBQKHX8GIZX` (`2026-10-17T00:00:00Z`) and `evt_MVF4GUX2CZGX` (`2026-10-19T00:00:00Z`)

## Problem

When a user reports a problem without a time, the plugin records it at midnight UTC of that day, so the
timeline misleads readers: the event looks as if it was observed hours before it was reported.

Nothing under `plugin/` sets midnight. The skills say to use "the current time" when no source time exists
(`plugin/skills/problem/SKILL.md` step 5, `plugin/skills/proposal/SKILL.md` step 6,
`plugin/skills/record/SKILL.md` "Which time to use", and the decision rule in
`plugin/config/mappings.yaml`), but the model has no clock. The conversation gives it only today's date,
and a date with no time of day becomes `T00:00:00Z`. This cause fits both events but is not proven.

The user confirmed that they gave no time at all in the report that produced the second event.

## Intent

An event with no source time carries the real moment it was recorded. Nobody, and no model, has to know
the clock for that: the server already does.

## How it works today (verified in the repo)

`internal/service/service.go` stamps a blank `time` with the server clock, in UTC, as RFC 3339
(`applyDefaults`, line 113). A stamped event skips the future-time check (`stamped`, line 73), so
the server cannot reject it for being ahead of the clock. `record/SKILL.md` already allows leaving `time`
out. No server change is needed.

## Goals

1. When there is no source time, the plugin sends no `time`, and the server stamps it.
2. The same rule covers every skill that records an event: `problem`, `proposal`, and `decision.made`
   through the decision rule.
3. The draft shown to the user says the time will be set when recorded, instead of showing a made-up time.
4. The plugin never turns the conversation's date, or a date it infers, into a time of day.

## Non-goals

- No change to the Techlog server, its schema, or its time checks.
- No change to events already recorded: they are immutable. A correction would be a new event.
- No change to the rule that a source with a timestamp (a commit, a merged PR, a deploy, an alert) supplies
  the time. The proposal's time stays the spec's commit time when the spec is committed.
- A source that gives only a date (for example "reported yesterday") is not changed: the existing rule says
  to use what it gives. That still yields midnight for such a source; it is left as it is (my call, see below).

## Anti-goals

- The model running `date` or guessing a clock to fill the field.
- A draft that shows a time the user did not give and the server did not set.
- A fix that works only for `problem` and leaves `proposal` or the decision rule on "the current time".

## Chosen approach

State the rule once, in `record/SKILL.md`, and make the other skills point to it.

- `record/SKILL.md`, field list: `time` is optional. Omit it when the source has no timestamp.
- `record/SKILL.md`, "Which time to use", item 2: when the source has no timestamp, or there is no source,
  leave `time` out. The server stamps the moment of recording. Never send the date from the conversation
  as a time, and never send a time of day you did not read from a source. Remove "or send the current time
  yourself with a zone".
- `problem/SKILL.md`, step 5: use the time the user gives; otherwise leave `time` out. In step 3, the draft
  shows `time: set by the server when recorded` unless the user gave a time.
- `proposal/SKILL.md`, step 6: the spec's commit time if committed; otherwise leave `time` out. The draft
  shows the same line.
- `config/mappings.yaml`, decision rule step 2: replace "Its time is the current time" with: leave `time`
  out, the user's approval is the decision and the server stamps it.

## Rejected options

1. **Have the skill read the clock with `date -u` and send an explicit time.** The draft would show the
   exact time and not depend on the server. Rejected: it adds a shell call and permission prompt to every
   record, the time drifts between the draft and the record, and the server already provides a clock.
2. **Only forbid date-only and midnight times in the skill text.** One line. Rejected: it does not remove
   the cause. The model would still have no clock to read, so it would guess another time.
3. **Reject midnight times in the server.** Rejected: the server cannot tell this defect from a real
   midnight event, and the fault is in the plugin layer.

## Constraints

- Only plugin text and `mappings.yaml` change: no Go code, no schema.
- Wording stays in the style of the existing skills: short, imperative, one rule per sentence.

## Open points for the builder

- My call: leave the date-only-source rule alone. It is a separate, smaller case, and a fix would need a
  decision about what time of day "yesterday" means.
- My call: the draft line reads `time: set by the server when recorded`. The exact words are the builder's.
- Check by hand after the change: record a problem with no time and confirm the stored time is the moment
  of recording in UTC, not midnight.
