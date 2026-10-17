# How it works demo: the connected workflow, from problem to mitigation

Date: 2026-10-17
System: techlog-website, component how-it-works
Problem: `evt_MVBQKHX8GIZX` (problem.reported)
Related: `evt_MVA3V69V9IT2` (the proposal for today's split-view demo, `2026-10-16-how-it-works-demo-design.md`)

## Problem

The landing page's "How it works" demo shows only `/techlog:record`. Techlog is
more than a recorder: an AI orchestrator, `/techlog:welcome` then
`/techlog:problem` then `/techlog:proposal` then `/techlog:record`, sits on top of
the configured development plugin (superpowers) and connects the engineering
work from problem to change. Prospective users and engineers who see only the
record demo cannot tell. Today teams hold that connection together by hand
(a Jira ticket with a PR link, a Confluence page with Jira links), and tracing
from any point back to the original problem takes a powerful model.

## Intent (agreed with the user)

- Takeaway: **a ready, shared engineering workflow where all dependencies are
  connected, in one context.**
- Audience: prospective users evaluating Techlog, and engineers who have only
  seen the record demo.
- The new demo **replaces** the current record-only demo; it does not sit next to it.
- The benefit to prove: when something breaks, the reason behind the suspect
  change is one walk away, and the corrective action is recorded back into the
  same chain.

Success: a visitor can say that each command is a stage of one workflow, that
every stage leaves a linked record, and that an incident can be traced back to
its change, decision, proposal and problem to choose a corrective action quickly.

## Chosen design: the split view, extended to the whole workflow

Keep the shipped split view (`#how` section of `website/src/pages/index.html`,
`website/src/assets/js/shell.js`, data in `website/src/examples/demo-chain.json`):
an agent pane on the left that is cleared at every "new session" divider, and a
Techlog pane on the right that is never cleared and keeps growing. Cards keep
their colour, their type badge and their **Show JSON** toggle, the "Simulated
example" badge stays, and the static no-JS fallback stays.

### Story

Fictional `promo-api` story (the demo is fictional; it is not Techlog's own
system). Five agent sessions, six events.

| # | Session | Command shown | Event added | Link to earlier event |
|---|---------|---------------|-------------|-----------------------|
| 1 | setup | `/techlog:welcome` (devPlugin: superpowers) | none (setup, not an event) | none |
| 2 | day 1 | `/techlog:problem` (a few tree questions, draft, "yes") | `problem.reported` | none |
| 3 | day 2 | `/techlog:proposal` (context note, design, spec) | `proposal.recorded` | `motivated_by` the problem, confirmed |
| 4 | day 3 | decision rule, then `/techlog:record` of the merged change | `decision.made`, `change.implemented` | `decides` the proposal; `implements` the decision, confirmed |
| 5 | day 6 | alert, `/techlog:record`, quick context from Techlog, corrective action | `incident.opened`, `mitigation.applied` | see below |

Session 4 shows the standing decision rule: the decision is recorded before
planning, and the change after the merge. Session 4 produces two cards; the
others one each, so the chain ends at six cards.

### The incident finale (session 5)

The agent has no memory of sessions 1 to 4. A 5xx alert arrives. The agent
records `incident.opened` with `observed_after` the change (confirmed) and
`caused_by` the change (**hypothesis**, as in today's demo: an unproven cause
stays a hypothesis). It then asks Techlog and shows the **quick context**:
the trace back from the incident through the change, the decision, the proposal
(including a rejected option) to the problem. The cards on that path are
highlighted in the Techlog pane. From that context the agent chooses the
**corrective action** (roll back the change under suspicion, keep the decision,
reopen the cap question) and records it as `mitigation.applied`, linked
`mitigates` the incident (confirmed) and `relates_to` the change. The corrective
action is therefore visible in both panes: chosen in the agent pane, kept as a
card in the Techlog pane.

### Details settled after the builder check

- **The decision rule is real.** It is the `decision` stage in
  `plugin/config/mappings.yaml`: before `superpowers:writing-plans` runs, the
  plugin drafts a `decision.made` from the approved spec and the proposal id, and
  records it once the user says yes (the user can say skip). Session 4 shows it
  as a short rule line, the user approving, and the resulting card: `decides` the
  proposal with basis `confirmed`, evidence the spec path. The change that
  follows is recorded with `/techlog:record`, `implements` the decision. The
  demo does not claim `/techlog:record` is mandatory.
- **The incident card stays `open`** after the mitigation. Events are
  append-only and the demo must not show a status flip. The `mitigation.applied`
  card (status `mitigated`) reads as "rollback of PR #482 applied" and links
  `mitigates` the incident. Neither the last agent line nor the caption says the
  incident is resolved. (My call.)
- **Chain data may be renumbered and re-dated.** The three existing ids in
  `demo-chain.json` need not stay stable; the landing-page tests
  (`landing.test.mjs`: chain length, types, "recorded evt_" lines, session and
  divider counts) are rewritten for the new story. Check nothing outside
  `website/` depends on those ids. Reuse the `timeline.json` shape
  (`extensions["org.techlog.proposal"].alternatives_considered`) for the rejected
  option and the 20% cap. (My call.)

### Foot caption

One sentence carrying the idea, for example: each command is one stage of the
workflow and leaves a linked record; an incident walks back to the problem in
one trace and the fix lands in the same chain. (Wording is the builder's call.)

## Alternatives rejected

- **Four-step stepper, one command per tab.** Explains each command in depth,
  but loses the single connected chain across sessions, which is the point, and
  needs the most new copy.
- **Split view with a separate "trace back" finale and no incident.** Shows the
  reverse trace, but not the benefit during an incident, and leaves the
  corrective action out of the record.
- **Keep the record-only demo and add the workflow beside it.** Rejected by the
  user: the new demo replaces the old one.
- **Recording the corrective action only in the agent pane.** Rejected by the
  user: it must be reflected in Techlog too.

## Goals, non-goals, anti-goals

Goals: show the four commands as one workflow; show linked records growing across
sessions; show real JSON for each card; show incident, quick context, corrective
action as one traceable chain.

Non-goals: no live Techlog server calls (the demo stays simulated and static);
no new event types or link relations; no rework of the rest of the landing page
or of the docs pages; no change to the plugin itself.

Anti-goals (a failure even if it technically works):
- the demo becomes a long, messy terminal stream again (the problem the previous
  proposal fixed): keep each session to a few lines;
- it implies `/techlog:record` cannot be used on its own, or that the
  orchestrator is mandatory;
- the incident cause is shown as proven;
- the system name `techlog` is used for website work (use `techlog-website`;
  the user does not allow a Techlog split).

## Constraints

- Event JSON in the demo must be valid against the draft 0.1 schema, and times
  must not run backward along links (a linked event is never earlier than its
  target).
- Keep the static no-JS fallback and the existing colour and badge system; give
  the types that are new to the demo (`problem.reported`, `proposal.recorded`, `mitigation.applied`)
  a colour consistent with the existing ones.
- Works at phone width as the current demo does.

## Left to the builder

- Exact wording of the sessions, questions and caption; the fictional ids and
  times (follow the style of `demo-chain.json`).
- How the "quick context" is rendered in the agent pane (compact trace text) and
  how the highlight on the Techlog pane is animated.
- Whether other pages use `demo-chain.json` (check before changing it) and
  whether the docs need a one-line update.
- Where `/techlog:welcome` appears, as a short first step or a caption line,
  as long as it is not shown as an event.
