# How it works demo: split view of sessions and records

Date: 2026-10-16
System: techlog-website, component how-it-works
Problem: `evt_MVA3LZ4TUV0T` (problem.reported)

## Problem

The "How it works" section of the landing page is a simulated agent CLI session
(`#shell` in `website/src/pages/index.html`). It is messy and hard to follow:
first-time visitors cannot see what Techlog generates technically, and the idea
behind it is lost.

Causes found in the current demo:

- One terminal stream mixes three kinds of line: agent tool calls (`gh`,
  `kubectl`, `pd`, `jira`), `/techlog:record` output and agent commentary.
- It tells a long story: about 45 lines, 4 steps, 3 "new session" scenes, 6
  events, 90 seconds.
- The event Techlog actually stores is never shown, only a text rendering of
  what the skill did.

## Intent (agreed with the user)

- Takeaway for a first-time visitor: **agents leave linked records.** Each
  session emits small typed events that link back to earlier ones, so the story
  survives across sessions.
- Scope: a short chain of 3 events.
- Form: open. The demo may stop being a single terminal stream.

Success: a visitor who watches the demo can say what an event is, that it links
to an earlier one, and that the record outlives the session that made it.

## Chosen design: split view with a JSON toggle

### Story

Fictional `promo-api` story, reusing the ids and facts of the existing
examples (`src/examples/story.json`, `timeline.json`). Three events, each from a
different agent session:

1. `decision.made`: spin-to-win approved, 20% discount cap.
2. `change.implemented`: PR #482 merged. Link `implements` to the decision,
   basis `confirmed`.
3. `incident.opened`: 6.2% 5xx on `/spin`. Link `observed_after` to the change,
   basis `confirmed`; link `caused_by` to the change, basis `hypothesis`.

Dropped from this demo: rollback, postmortem, action, the "weeks later" history
query, and the release event. The lifecycle docs already cover them.

### Idea carried by the layout

The agent pane is cleared at every "new session" divider, because the agent has
no memory between sessions. The Techlog pane is never cleared and its chain keeps
growing. The visitor sees that sessions forget and Techlog remembers. The
foot caption states the same idea in one sentence.

### Layout

One `figure` with the existing "Simulated example" badge and two panes.

- **Agent session** (left): per session one user prompt, one line of what the
  agent did, and the `/techlog:record` call. No tool-call noise.
- **Techlog records** (right): one card per recorded event, appearing when its
  `/techlog:record` call completes. A card shows type, one-sentence summary, time
  and source (for example "from the PR merge"), and a link line to the card it
  follows. A confirmed link is drawn solid; a hypothesis link is dashed and
  labelled "hypothesis".
- Each card has a **Show JSON** toggle that reveals the event as Techlog stores
  it (schema `0.1-draft`), with the `links` field highlighted.

Mobile: the panes stack, the transcript above the chain.

### Behaviour

- Same playback model as today: the run is a pure function of time, with a play
  button, a seek bar and a clock. Nothing starts by itself (no autoplay, no
  `IntersectionObserver`). It starts idle with a call to run.
- Target length about 45 seconds, down from 90.
- Without JavaScript all three cards are visible statically, with their links.
- Typing is the point of the demo and is never skipped: the command line types each command, and every output line and each new-session divider is typed too (the divider more slowly, so a new session is noticed). Under `prefers-reduced-motion` only the glow and the caret blink stop.
- Accessibility: the transcript keeps `role="log"`; cards are real elements with
  accessible names; the JSON toggle is a native `<details>`/`<summary>`, which works without JavaScript and exposes its expanded state to assistive tech.

### Data and drift

The JSON shown in the cards comes from one new example file,
`website/src/examples/demo-chain.json`. The page build reads it, so the cards and
the JSON cannot disagree. A test validates each event in it against the core JSON
schema (`src/examples/json-schema-core.json`) so the demo cannot drift from the
real event format.

### Files touched (all under `website/`)

- `src/pages/index.html`: the `#shell` figure and its copy.
- `src/assets/js/shell.js`: timeline for two panes and the JSON toggle.
- `src/assets/css/landing.css`: `.sh-*` rules replaced or extended for panes,
  cards and link styles.
- `src/examples/demo-chain.json`: new, the three events.
- `tests/landing.test.mjs`: the demo test is rewritten for the new structure.

Everything else on the landing page, including the section heading and lead
text, is unchanged unless the new demo makes a sentence untrue.

### Testing

- Structure: figure `#shell` with the "Simulated example" badge, two panes,
  exactly 3 cards, exactly 3 `/techlog:record` calls, 2 session dividers.
- Links: 2 confirmed links and 1 hypothesis link rendered, the hypothesis one
  marked as such.
- Data: `demo-chain.json` events validate against the core schema; the card
  text and the JSON come from the same file.
- Behaviour: starts idle, no autoplay, seek reaches the end state with all
  cards visible, each card has a `<details>` JSON toggle (checked by hand in a browser, since the suite has no DOM).
- Static fallback: with JS off all cards are visible.

### Out of scope

- Other sections of the landing page and the docs pages.
- A real backend or live MCP call: the demo stays a simulation.
- The rollback, postmortem and history-query scenes.

## Rejected options

### B. Stepper with real JSON

Three clicked or timed steps, each showing a one-line prompt, then the actual
event JSON with the `links` field highlighted and a mini link graph.

- For: the most technically honest; shows exactly what is stored.
- Against: JSON as the main content is heavy for non-technical visitors, and the
  agent side of the story (sessions that forget) is lost.
- Kept from it: the real JSON, as the per-card toggle.

### C. Trim the terminal

Keep one terminal stream, cut it to 3 events, drop the tool noise and add a
caption per step.

- For: smallest change; most of the current test still applies.
- Against: keeps the mixed stream of tool calls, record output and commentary,
  which is the cause of the confusion. Expected to stay hard to follow.

### D. Keep the full lifecycle story

Keep merge, release, incident, rollback, postmortem and action (6 or more
events), only presented more clearly.

- For: shows the whole lifecycle and the "confirmed vs hypothesis" payoff after
  the rollback.
- Against: too much story for a first visit; it hides the single idea of linked
  records. The lifecycle docs already tell the longer story.

### Other takeaways considered

The user chose "agents leave linked records". Not chosen: "ask later, get the
story" (a payoff query), "honest causality" (confirmed vs hypothesis as the
headline) and "connects existing tools". The hypothesis link is still shown, as
a secondary detail on the third event.

## Open points for the plan

None that change the design. Details left to the plan: exact card copy, timings
inside the 45 second target, and whether the element ids (`sh-play`, `sh-seek`)
are kept for continuity.
