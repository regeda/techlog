# Default clock skew independent of tests, then 5 seconds — design

Date: 2026-10-16
Status: approved in conversation, pending written-spec review
Problem: Techlog event `evt_MVAVW2ZC6Y0F` (`problem.reported`, system `techlog`, component `record_event`)
Related change: `evt_MVAVM2LXWA1G` (commit `fe2447c`), which added `DefaultClockSkew`

## Problem

`service.DefaultClockSkew` (5 minutes) is the starting skew of `service.New`. The service unit
tests build the service with only `WithClock`, so they run on the default, and several of them
hard-code its value: a time of `2026-03-12T10:05:00Z` must pass and `10:05:01Z` must fail, and
the stamp tests use a target at `10:04:00Z`. Changing the constant makes them fail. There is no
runtime impact, but the default cannot be changed safely.

## Intent

The default can be changed without touching any test. The author wants it changed too: to 5 seconds.
The 5s edit that found the coupling was a probe; the later wish to change the default is a separate,
stated intent.

## Goals

1. No test depends on the *value* of `DefaultClockSkew`.
2. The default becomes `5 * time.Second`.
3. Tests and docs agree with the new default.

## Non-goals

- No change to the skew rules themselves (future check, link-order check, UTC storage).
- No change to the `-clock-skew` flag, `WithClockSkew`, or the rule that a negative skew fails `New`.
- No change to events already recorded in Techlog: they are immutable and still say "5-minute".

## Anti-goals

- A test that still only passes because the default happens to be 5 minutes.
- A test that checks the default's number (a test that would fail for the very change it should allow).

## Constraints

- Step 1 lands before step 2, so that the suite is green at each step and the coupling is gone
  before the value moves.
- Conventional Commits, no attribution trailers.

## Decisions

| Question | Decision |
|---|---|
| How tests become independent | The test helper pins an explicit skew (approach A) |
| Default after the change | `5 * time.Second` |
| Ordering | Step 1 (tests), then step 2 (default and docs) |
| Where the 5s change lives | This proposal, as the second step, not a separate problem |
| Earlier 5-minute decision `evt_MVAUE8B8W1K5` | Superseded by this proposal's decision; say so in its summary |

### Step 1: tests stop depending on the default

All in `internal/service/service_test.go`:

- Add a test-local constant `testSkew` (any positive value; 5 minutes keeps today's tests
  readable) and make `newService(t)` pass `service.WithClockSkew(testSkew)` next to `WithClock`.
- Express every skew-dependent time through it: the boundary tests use `fixedNow.Add(testSkew)` and
  that plus one second; the stamp and link tests with a target inside the skew use
  `fixedNow.Add(testSkew - time.Minute)`. No literal such as `10:05:00Z` remains for the boundary.
- Add one test, `TestDefaultClockSkewApplies`, that builds `service.New(newFakeStore(), WithClock(...))`
  with no skew option and asserts `fixedNow + service.DefaultClockSkew` is accepted and one second
  more is rejected at `/time`. It names the constant, never its value, so it holds for any positive default.
- `TestClockSkewOption` already sets its skews explicitly and stays as it is.
- Tests in other packages use `testutil.NewService` with the real clock and past dates; they are not
  affected and need no change.

### Step 2: the default becomes 5 seconds

- `internal/service/service.go`: `const DefaultClockSkew = 5 * time.Second`.
- The `-clock-skew` flag takes its default from the constant, so its default follows; its help text
  needs no change.
- Docs: `docs/superpowers/specs/2026-10-16-event-time-ordering-design.md` (decisions table, goals,
  configuration, tests) and step 5 of `service.Record` in
  `docs/superpowers/specs/2026-10-09-techlog-server-design.md` say "5 minutes"; they say "5 seconds"
  after this step. Search the repo for other mentions (`grep -rn "5-minute\|5 minutes\|5m"` over
  `docs`, `plugin`, `website`, `README.md`).

### Rejected approaches

- **Compute every boundary from `service.DefaultClockSkew` in each test.** The tests pass for any
  value, but each is coupled to the constant, and a very small default makes fixtures such as "inside
  the skew" fragile.
- **Remove the default from `service.New` and require every caller to pass a skew.** It changes the
  API and every call site, which is too much for a test-isolation problem.
- **Only update the hard-coded 5-minute values to 5 seconds.** The tests stay coupled, and the next
  change breaks them again.

## Risk (accepted by the author)

A 5-second skew is much tighter than 5 minutes. A client whose clock runs more than 5 seconds ahead
of the server has its events rejected as future-dated. Machines that are not time-synchronised, such as
laptops and agents on other hosts, are the likely victims. Operators can raise it with `-clock-skew`.
This is the author's call; the spec only records it.

## Verification

- After step 1, with the constant temporarily set to `1s`, `5s` and `1h` in turn, `go test ./...` is
  green each time. Then restore the constant. This is a manual check, not a permanent test.
- After step 2, `go test ./...` and `go vet ./...` are green.
- `grep` finds no stale "5-minute" claim about the default in the repo's own docs (events recorded in
  Techlog are not touched).

## Facts checked in the repo

- The `-clock-skew` flag takes its default from the constant (`cmd/techlog/serve.go:26`). The only
  `cmd` test that touches the flag passes `-1s` and does not depend on the default. Nothing under
  `cmd`, `internal/plugin`, `internal/mcpserver`, `internal/api` or `internal/ui` hard-codes `5m`,
  and none records events stamped "now" from a client, so 5s does not make them flaky.
- The only uncommitted change in the working tree (besides this spec) is the one-line probe in
  `internal/service/service.go` (`5 * time.Second`). It can be discarded, `git checkout` on that
  file loses nothing else, and step 2 then makes the same edit properly.
- Mentions of the 5-minute default outside tests are in the two specs named above and in the
  historical plan `docs/superpowers/plans/2026-10-16-event-time-ordering.md`; leave the plan as it
  is, it records what was built then.
- `TestDefaultClockSkewApplies` goes through `Record`, so the rejection is reported at `/time`.
- Recording Techlog events (decision, change) is the workflow's job, not a build step: the
  supersession of `evt_MVAUE8B8W1K5` is stated in the new decision's summary, and the old event is
  never edited.

## Left to the builder

- The exact value of `testSkew`, and how the stamp/link tests derive their in-range times from it.
- The wording of the doc changes.
- The commit split (one commit per step is expected).
- This spec is committed as a `docs` commit before step 1.
