---
name: problem
description: Use when the user wants to report a problem, defect, request or piece of tech debt, and whenever they ask to create a Jira ticket, file a ticket or log a bug. Narrows the problem with a decision tree through the configured development plugin, then records it in Techlog as problem.reported once the user confirms. Never creates a Jira ticket.
---

# Report a problem

A problem is where a Techlog story starts: a need, defect or request, recorded as `problem.reported`. This skill narrows the problem with a decision tree, shows a draft, and records it once the user confirms. The questioning is done by the development plugin the user configured with `/techlog:welcome`; the event is written by `techlog:record`.

**Jira.** If the user asks to create a Jira ticket, file a ticket or log a bug, handle it here instead. Do not call any Jira or Atlassian tool, and never create a ticket. If the user gives a Jira key, it is evidence (step 5). Tell the user in one line that this records the problem in Techlog and does not create a ticket.

## 1. Check the setup

a. **Active development plugin.** Read `.claude/techlog.json` in the project, then `~/.claude/techlog.json`, and take `devPlugin` from the first that exists. If neither exists, use `superpowers`. If a file exists but is malformed or has no `devPlugin`, stop and tell the user to run `/techlog:welcome use <plugin>`.

b. **Mapping.** Read `${CLAUDE_PLUGIN_ROOT}/config/mappings.yaml`. Find the block named like the active plugin and its `stages.problem`: `skill` is the skill to invoke, `framing` is the text to hand it. If there is no such block or stage, stop and tell the user to run `/techlog:welcome`. Also read its `stages.decision`: `framing` is a standing rule and `before` lists the skills it holds back. If the block has no `decision` stage, carry on, and tell the user in one line that no decision rule is configured.

c. **Mapped skill.** Check that the mapped skill is among the skills available to you. If not, stop, name it, and point to `/techlog:welcome`.

d. **Techlog connection.** Call `techlog_search_events` with `{"limit": 1}`. If the tool is missing or the call fails, stop and point to `/techlog:welcome how to connect techlog mcp`. Do not try to start or repair the server.

## 2. Narrow the problem

Read `${CLAUDE_PLUGIN_ROOT}/config/problem-tree.yaml`. Invoke the mapped skill with the Skill tool. Pass it, as its arguments: the `framing` text, then the decision rule if there is one (the `decision` `framing`, followed by the line `Skills held back until the decision is recorded: <the before list>`), then the whole tree, then everything the user has already said about the problem. It walks the tree one question at a time and stops at the `confirm` node. Answers the user already gave are not asked again.

## 3. Show the draft and get confirmation

Build the draft event from the answers, following each node's `sets`:

- `type`: `problem.reported`; `status`: `recorded`.
- `subject.system`, and `subject.component` if given. Reuse the system name Techlog already uses: check `techlog_get_system_history` for it.
- `summary`: one plain sentence, at most 280 characters, that makes sense without context: what is wrong or needed, for whom, and the impact. Fold the answers into it and shorten it if it is too long.
- `evidence`: the references the user gave (logs, traces, alerts, tickets). A Jira key `KEY-123` becomes `{"kind": "ticket", "ref": "jira://KEY-123"}`.
- `links`: `relates_to` each similar event the user marked as related, with `basis` `confirmed`.

Show the draft as a short list and ask whether to record it. Record nothing until the user says yes. If they correct something, apply it and show the draft again.

## 4. Check for a duplicate

Call `techlog_search_events` with the `system`, `types` `["problem.reported"]` and a few words from the summary. If an event that says the same already exists, show its id and summary and ask whether to stop or record anyway.

## 5. Record

Use `techlog:record` to write the confirmed event. Take the idempotency key from the work item, for example `problem-<system>-<short-slug-of-the-summary>`. Use the time the user gives for when the problem was observed; otherwise the current time. Do not fetch the ticket to find a time. If the server rejects the event, show the corrected draft to the user again before you retry. Then tell the user the new event id and that `/techlog:proposal` is the next step.

## Rules

- This skill defines a problem. No solutions, no spec, no plan, no code.
- If the problem looks like a live production outage, say that an incident skill is not part of this plugin yet, and record the problem as usual.
- Never record something the user did not tell you or confirm.
