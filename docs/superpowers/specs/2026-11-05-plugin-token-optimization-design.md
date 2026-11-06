# Cut token cost in the techlog plugin — design

Date: 2026-11-05
Status: approved (decision `evt_MW3FDUPBG7B5`) and implemented on `perf/plugin-token-optimization`.
Shipped unverified, by the user's instruction to skip the manual checks: the pointer-args gate (change 2), the
rejection-rate gate (change 3), the trigger check and the token measurement. The `record` body is 8.5 KB, down
from 10.8 KB (about 21%), with a 8704-byte test limit: the 5-6 KB goal could not be met without cutting parts
the Changes section keeps in the body. Also changed: `internal/event/skill_test.go` pinned `record/SKILL.md`
too, and was updated to read examples from `references/examples.md`.
Problem: `evt_MW3F3OEXBFGE` (system `techlog-plugin`, component `skills`)
Approach: A, the four changes below, each kept only if its check passes (chosen over B and C, see Rejected options)

## Problem

A review of `plugin/` (11 files, 37 KB, about 9k tokens) found three places where tokens are spent on what the
model already has or does not need, and one contradiction that can write a wrong, immutable event.

- **Re-emitted args.** `problem` step 2 and `proposal` step 5 pass the framing, the decision rule and (in
  `problem`) the whole 3.7 KB tree as `Skill` args. The model has already read them, so it writes about 2k
  tokens (`problem`) or 0.5k (`proposal`) again as output, the most expensive kind.
- **A large `record` skill.** `plugin/skills/record/SKILL.md` is 11 KB (about 2.8k tokens). It loads on every
  record call and every time its description triggers it. About half is two examples, an error table, an
  idempotency section and an MCP connect snippet that `docs/connect-mcp.md` already holds.
- **Long descriptions.** The four descriptions are always in context (about 300 tokens). `problem`,
  `proposal` and `record` describe their workflow, which the skill-writing guidance warns against: the model
  may follow the description instead of reading the body.
- **A contradiction.** `record/SKILL.md` ("With superpowers", lines 151-163) has two `brainstorming` rows that
  clash with other rules. One says to record `proposal.recorded` when the spec is written; `proposal` step 7
  already records exactly one proposal, so this risks a duplicate. The other says to record `decision.made`
  when the user approves the spec; `config/mappings.yaml` (stage `decision`) says to record it just before
  `writing-plans`, and lets the user skip. `record` triggers on its own description, so it may write either
  event on its own. Not yet reproduced; see Check.

## Intent

A run loads and emits only what its stage needs, and one rule at a time decides when a decision is recorded.

## Goals

1. `problem` and `proposal` hand the mapped skill a short pointer, not the config text.
2. The `record` body is about 5 KB or less; the rest is read on demand, and one example stays inline.
3. The descriptions of `problem`, `proposal`, `record` and `welcome` state when to use the skill and not what
   it does.
4. `record` no longer says when to record `proposal.recorded` or `decision.made`.
5. What the user sees does not change: same questions, same drafts, same confirmations.

## Non-goals

- No hook, and no change to how the decision rule is applied. A hook, a per-plugin config split, a shared
  setup step, deduplicating rules between skills, merging Techlog searches and the tree notation are left for a
  separate correctness-focused spec. Review found that the hook is unverified, that moving the discipline
  rules out of `problem` and `proposal` breaks the order of the flow, and that merging the searches changes
  what the user sees.
- No change to the decision tree, the mappings, the model pins, the Techlog server or its schema.
- No change to the questions batching (turn count): it changes what the user sees.

## Anti-goals

- A pointer that silently fails, so the mapped skill walks no tree and nobody notices.
- A split that makes the model write invalid events more often, because the examples are gone or the error
  table is not read when it is needed.
- A shorter description that stops the skill from triggering, or from triggering on a Jira request.

## Changes

### 1. Remove the `brainstorming` rows from `record`

Delete the two `brainstorming` rows (`proposal.recorded` at spec written, `decision.made` at spec approval)
from the "With superpowers" table in `record/SKILL.md`. Keep the other three rows, since they do not clash:
`systematic-debugging` root cause is `investigation.completed`, `verification-before-completion` is
`verification.completed`, and a merge from `finishing-a-development-branch` is `change.implemented`. Add one
line saying that proposals and decisions are owned by `proposal` and the decision rule.

### 2. Pass pointers to the mapped skill

- `problem` step 2: the args are the stage `framing`, then one line saying the decision rule from step 1b
  applies, then one line saying to walk the tree read above, then what the user already said. The tree is not
  repeated.
- `proposal` step 5: the same, with "the confirmed context note above" in place of the tree.
- The skill bodies keep the strings that name the decision rule (`stages.decision`, `Skills held back until the
  decision is recorded`, `no decision rule is configured`), so the rule still reaches the design skill
  through the conversation. The `framing` stays in the args because it overrides the default brainstorming
  flow.

This relies on `superpowers:brainstorming` running inline, in the same context, so it can see the tree, the
rule and the note. It is verified by hand (see Check) before the change is kept.

### 3. Split `record/SKILL.md`

Body, about 5 KB: connect, before you record, type table, fields, which time to use, links and honesty, rules,
and one example (the incident with a hypothesis link). Moved to `plugin/skills/record/references/`:

- `examples.md`: the second example (the decision), read when the user asks for another event type's shape.
- `errors.md`: the error rows, and the retries and idempotency details. The body says: if the result has
  `isError: true` or `Already recorded`, read `references/errors.md`.

The `claude mcp add` snippet is replaced with a pointer to `/techlog:welcome how to connect techlog mcp`.
Paths are written as `${CLAUDE_PLUGIN_ROOT}/skills/record/references/...`, in the way the existing skills
name plugin files.

### 4. Trim the descriptions

Each description is rewritten to "Use when ..." triggers only, in the third person, with the workflow moved
to the body, which already holds it. `welcome` also moves "Takes a free-text question, or use <plugin>..."
into the body. The strings the Go tests require stay (see Tests).

## Tests

The plugin files are pinned by tests in `internal/plugin/`. Before the change, run `go test ./internal/plugin/`
and keep it green.

- `TestSkillsCarryTheDecisionRule`, `TestProblemSkill`, `TestProposalSkill`, `TestWelcomeSkill`: they require
  strings in the skill bodies and descriptions, among them `problem.reported`, `create a Jira ticket`,
  `file a ticket`, `log a bug` in the `problem` description, and `proposal`, `proposal.recorded` in the
  `proposal` description. Changes 2 and 4 keep those strings. If a string has to go, say so here and update the
  test in the same change.
- `TestSkillsReferenceExistingPluginFiles`: the new `references/` paths must exist.
- `TestEverySkillIsWellFormed`, `TestSkillModelPins`: no change expected.
- Add a test for the `record` size: the body stays under a limit, and each `references/` file named in the body
  exists.
- `docs_page_test.go` reads `website/src/pages/docs/claude-plugin.html`. No change is expected, since no
  skill name, stage or mapping changes; the test run confirms it.

## Check

Use the skill-creator eval loop, with a snapshot of the current plugin as baseline (`old_skill`).

- **Baseline first, for the contradiction.** Run a session that finishes `brainstorming` with the plugin's
  `record` available, and see whether `decision.made` is written at spec approval. If it is not, change 1 is
  a tidy-up and not a fix; say so.
- **Evals:** `problem` for each of the four kinds; `proposal` from an existing problem; `record` of a change
  after a merge; `record` of an event that the server rejects (a `caused_by` link without a basis).
- **Assertions:** tree walked in order and nothing asked twice; summary at most 280 characters; no record
  before confirmation; the decision rule still offered before `writing-plans`; no Jira call; the recorded event
  is accepted on the first try.
- **Measure** input tokens, output tokens and tool calls per run, with and without the change. The estimates
  (about 2k output tokens per `/techlog:problem`, about 1k input tokens per record call, about 100 tokens per
  session from the descriptions) stand only if the numbers agree; report the real ones.
- **Rejection rate:** compare the share of events rejected on the first try, before and after the split of
  `record`. If it rises, keep the second example and the error rows in the body.
- **By hand:** during `/techlog:problem`, confirm that the nested `brainstorming` run walks the tree without
  it being in the args. If it does not, keep the tree in the args and report that as a new problem.
- **Trigger check:** the new descriptions still trigger on "create a Jira ticket for this bug", "propose a fix
  for the problem we recorded" and "what happened to promo-api last week".
- Run `claude plugin validate plugin` after the change.

## Calls made by the author

The builder check raised these; each was settled without changing what gets built. Change any of them here
before planning if you disagree.

- **Pass or fail.** Each gate is run three times per eval case, with and without the change. Change 2 passes
  if the nested `brainstorming` run walks the tree in order in all three runs. Change 3 passes if the share of
  first-try rejections does not rise by more than one run in the set. A failed gate means that change is
  reverted, and the rest of the work goes on.
- **Order and staging.** Each change is its own commit, in the Order of work below, so a failed gate reverts
  one commit.
- **Change 2 is try-then-revert.** If the nested run cannot see the tree, keep the tree in the args and report
  it as a new problem. The tests stay green either way, since they pin strings in the skill bodies, not the
  args.
- **Baseline for the contradiction.** A manual session, repeated three times. Change 1 ships whatever the
  baseline shows: commit type `fix` if a bad write was seen, `chore` if not.
- **Rejection rate.** Counted from `isError` results in the eval transcripts. No server log is needed. The
  `caused_by` eval case is one of several: add a second, an event with a missing required field.
- **`record` size test.** A hard limit of 6 KB on the body, which leaves margin over the 5 KB target.
- **Eval workspace.** Kept in scratch, not committed. The results go in the PR description. Only the new Go
  test and the plugin edits are committed.
- **Path test.** `TestSkillsReferenceExistingPluginFiles` matches `${CLAUDE_PLUGIN_ROOT}/` followed by
  `[A-Za-z0-9_./-]+`, so nested `references/` paths already work, and no test change is needed for it.
- **Version.** Bump to 0.2.1: the change reduces cost and removes a conflicting rule, with no new feature.

## Rejected options

1. **The full recommendation set** (hook, per-plugin config, shared setup, deduplicated rules, merged
   searches). Deferred: little token gain for the risk, and several items change behaviour users can see.
2. **Only fix the contradiction.** Cheapest, but it leaves the largest saving, the re-emitted args.
3. **Move all of `record`'s examples out.** Saves more input tokens, but may raise the rate of rejected
   events, and each rejection costs a full call.
4. **Correctness first, then size** (fix the contradiction and trim descriptions now; do the pointer args and
   the `record` split only after measuring). Safest, but it defers the largest saving behind a separate
   measuring step that the gates in this design already provide: pointer args and the `record` split are each
   kept only if their check passes.

## Order of work

1. Run `go test ./internal/plugin/` and record the baseline numbers (Check).
2. Change 1.
3. Change 2, then the by-hand check; keep it only if the check passes.
4. Change 3, then the rejection-rate check.
5. Change 4, then the trigger check.
6. Update or add the tests, run `claude plugin validate plugin`, bump the version in
   `plugin/.claude-plugin/plugin.json`.
