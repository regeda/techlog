---
name: proposal
description: Use when the user wants to propose a solution to a recorded problem, or to design a change and keep the reasons and the rejected options. Collects context from Techlog and the repo for the user to confirm, runs the configured development plugin's design process, then records proposal.recorded in Techlog linked to the problem.
---

# Propose a solution

A proposal answers a problem: what to do about it, and which options were turned down. This skill finds the problem in Techlog, collects context for the user to confirm, runs the design process of the development plugin the user configured with `/techlog:welcome`, and records the result as `proposal.recorded` through `techlog:record`.

## 1. Check the setup

a. **Active development plugin.** Read `.claude/techlog.json` in the project, then `~/.claude/techlog.json`, and take `devPlugin` from the first that exists. If neither exists, use `superpowers`. If a file exists but is malformed or has no `devPlugin`, stop and tell the user to run `/techlog:welcome use <plugin>`.

b. **Mapping.** Read `${CLAUDE_PLUGIN_ROOT}/config/mappings.yaml`. Find the block named like the active plugin and its `stages.proposal`: `skill` is the skill to invoke, `framing` is the text to hand it. If there is no such block or stage, stop and tell the user to run `/techlog:welcome`. Also read its `stages.decision`: `framing` is a standing rule and `before` lists the skills it holds back. If the block has no `decision` stage, carry on, and tell the user in one line that no decision rule is configured.

c. **Mapped skill.** Check that the mapped skill is among the skills available to you. If not, stop, name it, and point to `/techlog:welcome`.

d. **Techlog connection.** Call `techlog_search_events` with `{"limit": 1}`. If the tool is missing or the call fails, stop and point to `/techlog:welcome how to connect techlog mcp`. Do not try to start or repair the server.

## 2. Find the problem

If the user gave an event id, read it with `techlog_get_event` and check that it is a `problem.reported`. Otherwise search with `techlog_search_events`, `types` `["problem.reported"]`, and the `system` if the user named one. Show the candidates (id, system, time, summary) and let the user pick one.

If no problem exists, offer to run `/techlog:problem` first and stop until the user answers.

## 3. Gather context

- The system's recent history: `techlog_get_system_history` with the system and a `limit` of 20.
- Events linked to the problem: the links in the `techlog_get_event` result.
- Repo facts, read-only: the current branch, `git log --oneline -10`, the files and components the problem seems to touch, and existing specs or decision records under `docs/`.

## 4. Confirm the context note

Write a short context note with four parts: the problem (id and summary), the system history that matters (three to five events, with ids), the repo facts, and the event id the proposal will link to with `motivated_by`. Show it and ask the user to confirm or correct it. Do not start the design before they confirm. Apply corrections and show the note again.

## 5. Design

Invoke the mapped skill with the Skill tool. Pass it, as its arguments: the `framing` text, then the decision rule if there is one (the `decision` `framing`, followed by the line `Skills held back until the decision is recorded: <the before list>`), then the confirmed context note. It runs the design process until the design is approved and the spec is written. Do not start `writing-plans` or any implementation skill afterwards.

## 6. Show the draft and get confirmation

Build the event:

- `type` `proposal.recorded`, `status` `proposed`, the same `subject` as the problem.
- `summary`: one plain sentence, at most 280 characters: the approach chosen and the main option turned down.
- `links`: `[{"rel": "motivated_by", "target": "<problem id>", "basis": "confirmed"}]`.
- `evidence`: `[{"kind": "spec", "ref": "<path of the spec>"}]`. The rejected options stay in the spec.
- `time`: the spec's commit time (`git log -1 --format=%aI -- <path>`) if it is committed, otherwise the current time.

Show it as a short list and ask whether to record it. Record nothing until the user says yes. If they correct something, apply it and show the draft again.

## 7. Record

Use `techlog:record` to write the confirmed event. Record exactly one event, the proposal: do not also record a decision, even if the user approved the design. If the server rejects the event, show the corrected draft to the user again before you retry. Then tell the user the new event id. Planning is theirs to start: tell them that when they ask to plan, the decision is recorded first, as the standing rule says, and that they can say skip.

## Rules

- Never record a proposal the user has not seen: the design is approved first.
- One problem per proposal. If the design turns out to answer more than one, ask which to link.
- Never record something the user did not tell you or confirm.
