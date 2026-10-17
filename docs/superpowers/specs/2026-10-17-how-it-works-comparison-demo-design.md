# How it works demo: the same question, without and with Techlog

Date: 2026-10-17
System: techlog-website, component how-it-works
Problem: `evt_MVBQKHX8GIZX` (problem.reported): the landing demo does not show how Techlog connects engineering work.
Supersedes: `2026-10-17-how-it-works-connected-workflow-demo-design.md` (proposal `evt_MVBR3IIKN6G8`, decision `evt_MVBR7N17HYK8`), whose CLI transcript and event track are replaced by this design.

## Problem

The "How it works" demo has to make one thing obvious to prospective users and
engineers: Techlog is a ready, shared engineering workflow where everything is
connected, so an agent can find the reasons behind a piece of work. A simulated
CLI with sessions and an event track (the previous design) explains the
mechanism but makes the visitor work to see the benefit.

## Intent (agreed with the user)

- Show the benefit directly: **the same question put to an agent without and with
  Techlog.** Without Techlog the agent has no context and sends the user off to
  look. With Techlog the agent finds the connected events and answers from them.
- **No sessions, no tracks, no dividers, no CLI.** The visitor sees: a question
  from a user, then two panes, left "Without Techlog", right "With Techlog".
- The Techlog events in the right pane are **clickable and expand to minimum
  details.**
- The right pane shows **three events: problem, decision, implementation.**

Success: a visitor can say what the agent could not do without Techlog and what
it found with it (why the 20% cap exists, who decided, which change implements
it), and can open an event to see what Techlog holds.

## Chosen design

One `figure` on the landing page `#how` section, keeping the "Simulated example"
badge, replacing the split view of the previous design. The reference mockup is
the user's image (`/Users/regeda/Desktop/Screenshot 2026-10-17 at 22.42.52.png`):
two chat panes, the right with a green border.

### The question

A single user bubble above both panes (the same question goes to both):
"Why do we enforce a 20% discount cap? I need to change the pricing logic."

### Left pane: Without Techlog

Header: a red dot, "Without Techlog", note "no context". The agent answers that
it does not have enough information about the original reasoning; it can read
the code and tests but not the design discussion or business context; and it
suggests checking the original issue or requirement, past design discussions,
related pull requests, and business rules or policies. (Wording from the
mockup; the builder may tighten it.)

### Right pane: With Techlog

Header: a green dot, "With Techlog", note "connected events". The agent says it
is using Techlog and lists the chain behind the cap as **three events, oldest
first, joined by a vertical line**:

| # | Event type | Date | Collapsed row says | Expanded details (minimum) |
|---|-----------|------|--------------------|----------------------------|
| 1 | `problem.reported` | Mar 9 | the need: new visitors want a first-order offer, retention is flat | summary, source (ticket) |
| 2 | `decision.made` | Mar 10 | approved spin-to-win, discount capped at 20% | summary, link to the problem, source (spec) |
| 3 | `change.implemented` | Mar 10 | PR #482 implements the 20% cap | summary, link `implements` the decision, source (PR) |

Then a closing answer: the cap is a business rule from an approved decision;
before changing the pricing logic, check the original problem and the tests that
came with PR #482.

A row is a native `<details>`/`<summary>` (so it opens without JavaScript and by
keyboard). Expanded details show the event's summary, its link to the earlier
event in words and arrow form (`implements → decision.made`), and its source.
**No JSON.** Several rows may be open at once.

### Data

`website/src/examples/demo-chain.json` shrinks to these three events, with
fictional `promo-api` / `spin-service` data as today. Links, my call: the
decision is `relates_to` the problem (confirmed); the change `implements` the
decision (confirmed). The proposal, incident and mitigation events of the
previous design are dropped from the demo (the lifecycle docs cover them).

### Behaviour

My call, to confirm in review: the demo **plays out** on a Run control (as
today): the question types in, the left answer types, then the right pane's
agent line types and the three rows appear one after another, then the closing
answer; total run about 30 seconds, with the existing play/pause and seek bar.
Rows can be opened while it runs and after it ends. Without JavaScript the page
is the finished comparison with all rows closed but openable.

### Details settled after the builder check (my calls unless marked)

- **Data.** Keep the existing times: problem Mar 9 10:00, decision Mar 10 14:05,
  change Mar 10 16:40 (UTC). The decision keeps its source `spec
  docs/specs/spin-to-win.md`. `relates_to` with basis `confirmed` is valid in the
  draft schema. The demo shows a decision linked straight to its problem, because
  the proposal is not on screen; the lifecycle docs still teach
  `decision decides proposal`, and nothing here contradicts them.
- **Order and timing (if it plays out).** The left answer is typed first and
  completes; then the right pane's agent line, the three rows one after another,
  and the closing answer. Typing is by character, as today, and the total is
  solved to the target (about 30 s) like `TARGET_MS` now. Space for the three
  rows and the closing answer is reserved from the start, so nothing grows or
  jumps while it runs; no inner scrolling.
- **Controls (if it plays out).** Keep the existing Run, pause and replay
  control and the seek bar: seeking scrubs the typed text and row appearance (the
  run stays a pure function of time). It starts only when the visitor presses
  Run, never on its own.
- **The mockup image.** The spec text is the contract; the image is a visual
  reference for structure (a user bubble above two panes, header dots and labels,
  a green border on the right pane, event cards on a vertical line). The
  follow-up input in the image is omitted, not shown disabled.
- **Page copy and tests.** The `#how` heading, lead and the "Simulated example"
  badge stay as they are. Tests and `tools/audit-content.mjs` rules that assert
  the old CLI or six-event design are reworked or removed; any audit wording rule
  that the new copy trips is fixed in the copy.
- **Branch.** Continue on `feat/website-connected-workflow-demo`; the net diff
  against `main` replaces the previous design's demo.

## Alternatives rejected

- **The previous design (CLI transcript beside an event track, sessions, tabs):**
  explains the mechanism but not the benefit; replaced at the user's request.
- **Six events in the right pane (adding proposal, incident, mitigation):**
  narrowed to three by the user.
- **A static finished comparison with no playback:** simpler, but loses the
  "watch the agent look it up" moment; kept as the no-JS fallback only.

## Goals, non-goals, anti-goals

Goals: one question, two answers, three clickable connected events; accessible
and readable at phone width (panes stack, the Techlog pane second).

Non-goals: no live Techlog calls; no JSON view; no session or track concept; no
change to the plugin, the docs pages or the rest of the landing page.

Anti-goals: implying Techlog replaces reading the code (the without-pane agent
reads code and tests too); an unrealistic strawman in the left pane (the agent
is honest and useful, just missing context); chat chrome that dominates the
point (no avatars or follow-up input that do nothing: if shown, a follow-up
input is disabled and decorative, or omitted); using the system name `techlog`
for website work.

## Constraints

- Event data validates against the draft 0.1 schema; links point to earlier
  events; times do not run backward along links.
- Colours: the existing per-type colours for the three types; the "without"
  pane is marked red and the "with" pane green with text labels as well (never
  colour alone).
- No inner scrolling of either pane; the panes share the height of the taller.
- Works at phone width, in light and dark themes, and with reduced motion
  (typing continues; only decorative blinking stops).
- The site builds with zero dependencies; tests and the link and content checks
  stay green.

## Left to the builder

- Exact wording of both answers and the closing line; how dates are formatted.
- Whether the typing engine of `shell.js` is adapted or replaced (the old split
  view, tabs and trace code are not needed).
- The expand affordance and the vertical connector styling.
- What happens to the old `renderDemoCards` and CLI-specific tests: remove or
  rework as the new markup needs.
- Whether the work continues on `feat/website-connected-workflow-demo` (the
  unmerged commits of the previous design stay in history and the net change
  replaces them; my default) or restarts from `main`.
