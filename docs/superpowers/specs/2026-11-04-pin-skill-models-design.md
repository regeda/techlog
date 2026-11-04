# Pin a model for the `problem` and `welcome` skills — design

Date: 2026-11-04
Status: draft, pending approval
Problem: Techlog event `evt_MW23LKA8AE4M` (`problem.reported`, system `techlog-plugin`, component `skills`)

## Problem

The `problem` and `welcome` skills do simple work but run on whatever model the session uses. When that is
an expensive model, every run spends tokens the job does not need.

Neither skill sets a model: their frontmatter in `plugin/skills/problem/SKILL.md` and
`plugin/skills/welcome/SKILL.md` holds only `name` and `description`.

How the skills are used, as the user described it:

- `welcome` is used rarely, and almost always to check the current setup: read the two config files,
  call `techlog_search_events` once, list the results. It needs no judgment.
- `problem` is mostly a questionnaire, sometimes shown as a richer terminal UI. The model walks a fixed
  decision tree one question at a time. It also writes the summary (at most 280 characters) that readers see
  later in the Techlog timeline, and decides whether a search hit is a duplicate.

## Intent

Each skill runs on a model that fits its job, whatever model the session started on.

## Decisions (the user's)

- `problem` is pinned to Sonnet. This covers the questioning and the summary: the user did not want a
  cheaper model writing the summary.
- `welcome` is pinned to Haiku.

## Goals

1. `/techlog:problem` runs on Sonnet.
2. `/techlog:welcome` runs on Haiku.
3. The pin is set in one place per skill, in its frontmatter, with no new config or code.

## Non-goals

- `proposal` and `record` keep the session model. The problem names only `problem` and `welcome`
  (my call, see below).
- No change to the decision tree, the mappings, the Techlog server or its schema.
- No setting that lets each user choose the model.

## Anti-goals

- A pin that makes `problem` ask worse questions or write a worse summary than today.
- A pin that silently does nothing, so tokens are still wasted and nobody notices.
- A pin that breaks the one-question-at-a-time conversation in `problem`.

## What Claude Code does (from the docs, checked 2026-11-04)

Sources: https://code.claude.com/docs/en/skills.md, https://code.claude.com/docs/en/model-config.md,
https://code.claude.com/docs/en/sub-agents.md

- `model` is an optional frontmatter field of `SKILL.md`. It takes the same values as `/model`
  (`sonnet`, `haiku`, `opus`, full model IDs, and so on).
- Plugin skills support every frontmatter field, including `model`.
- The override applies for the rest of the current turn and is not saved to settings. The session model
  returns on the user's next prompt.
- A value excluded by the organisation's `availableModels` allowlist is ignored, and the skill runs on the
  session model.
- Not documented: whether a skill that another skill invokes through the Skill tool keeps the pin. `problem`
  invokes `superpowers:brainstorming` (through the mapping), so this decides whether the questioning really
  runs on Sonnet. Treat it as unverified.
- The docs disagree about `inherit`: the skills page accepts it, the model-config page does not list it.
  This design does not use it.

## Chosen approach

Add one frontmatter line to each skill:

- `plugin/skills/problem/SKILL.md`: `model: sonnet`
- `plugin/skills/welcome/SKILL.md`: `model: haiku`

The aliases follow whatever model each alias currently names, so the pin does not go stale when a new Sonnet
or Haiku ships (my call: aliases, not full model IDs).

## Rejected options

1. **`context: fork` with a model.** The pin is firmer, because `model` then sets the forked subagent's
   model. Rejected: the subagent gets no conversation history and runs in the background by default, which
   is wrong for a skill that asks the user questions one at a time.
2. **A model setting in `techlog.json`, read by the skills.** Lets each user choose. Rejected: it adds
   config and instructions nobody asked for, and a skill cannot switch its own model by reading a file.
3. **Leave it and document the cost.** Rejected: it does not remove the wasted tokens.
4. **Pin `proposal` and `record` too.** Rejected for this change: the problem does not cover them. `proposal`
   runs a design process that benefits from a strong model. `record` is called by the other skills, and its
   pin would apply on top of theirs. Either can come as a separate problem.

## Constraints

- Only two frontmatter lines change: no Go code, no schema, no `mappings.yaml`.
- The frontmatter must stay valid YAML. Malformed YAML makes Claude Code load the skill with empty
  metadata, so the pin would be lost silently. Run `claude plugin validate plugin` (v2.1.233 or later)
  after the change.
- The pin must not reach skills that are uploaded to claude.ai or the Skills API, which reject `model`.
  These two skills ship only in the Claude Code plugin.

## Open points for the builder

- Check by hand: run `/techlog:welcome` and `/techlog:problem` from a session on a different model and
  confirm each runs on its pinned model (the transcript or `--debug` output shows it).
- Check by hand: during `/techlog:problem`, confirm that the questioning done by the nested
  `superpowers:brainstorming` run is on Sonnet, not only the outer skill. If it is not, report that
  as a new problem rather than working around it in this change.
- My call: aliases (`sonnet`, `haiku`) rather than full model IDs.
- My call: `proposal` and `record` are left as they are.
- The user's own session on a model cheaper than Sonnet would be raised to Sonnet for `problem`. That is the
  accepted outcome of the pin.
