# How it works demo: split view Implementation Plan

> **For agentic workers:** REQUIRED SUB-SKILL: Use superpowers:subagent-driven-development (recommended) or superpowers:executing-plans to implement this plan task-by-task. Steps use checkbox (`- [ ]`) syntax for tracking.

**Goal:** Replace the single-terminal "How it works" demo on the landing page with a split view: three short agent sessions on the left, and on the right a growing chain of linked Techlog event cards, each with its real JSON.

**Architecture:** The three events live in one example file, `src/examples/demo-chain.json`. The site build renders the right-hand cards from that file through a new `{{demo-cards <file>}}` directive, so the cards and the JSON behind "Show JSON" cannot disagree. The left-hand transcript is static markup in `index.html`. `shell.js` keeps its time-driven playback (a pure function of time) and now shows one session at a time on the left while the cards only accumulate.

**Tech Stack:** Node (zero dependencies, `node:test`), static HTML, plain CSS with the existing design tokens, vanilla JS.

**Spec:** `docs/superpowers/specs/2026-10-16-how-it-works-demo-design.md`

All commands run from `/Users/regeda/code/techlog/website`. Run a single test file with
`node --permission --allow-fs-read=. --allow-fs-write=./.test-out tests/<file>.test.mjs`; run everything with `npm test`.

## Global Constraints

- Takeaway: agents leave linked records; the record outlives the session that made it.
- The chain has exactly 3 events: `decision.made`, `change.implemented`, `incident.opened`, in 3 sessions, with 2 "new session" dividers.
- The incident has `observed_after` (confirmed) and `caused_by` (hypothesis) links; the change has `implements` (confirmed).
- Event format is schema `0.1-draft`; every demo event validates against `src/examples/json-schema-core.json`.
- No autoplay and no `IntersectionObserver`; the demo starts idle with a "▶ Run simulation" button.
- The "Simulated example" badge stays on the figure.
- Target run length about 45 seconds.
- Without JavaScript all three cards and all three sessions are visible.
- `prefers-reduced-motion` is respected.
- The site has zero dependencies and builds offline; do not add any.
- Commits follow Conventional Commits: `<type>(<scope>): <description>`. No attribution trailers.
- Spec refinement agreed in this plan: the JSON toggle is a native `<details>`/`<summary>` (works without JS, built-in `aria-expanded` semantics) instead of a custom button. Task 5 updates the spec wording.

## Review Focus

Failure modes the spec implies that no one asked about, most likely first. Each has a test in the task named.

1. Seeking backwards or into the middle: only the current session shows on the left, and a card appears only once its record line has been reached (Task 3 structure test, Task 4 manual check).
2. JavaScript off or failed: all cards and sessions are visible and the JSON toggles still work (Task 3 test: no `hidden` on cards or steps).
3. A 320px-wide phone: panes stack, no page-level horizontal scroll, long JSON scrolls inside its card (Task 3 CSS test, Task 5 manual check).
4. Reduced motion: no typing animation; the typed command appears whole (Task 4 test on `shell.js`).
5. Event text with HTML-special characters, or an event with no `sources`: the cards escape it and still render (Task 2 tests).

---

### Task 1: The demo chain data

**Files:**
- Create: `src/examples/demo-chain.json`
- Test: `tests/examples.test.mjs`

**Interfaces:**
- Consumes: `validate(schema, value)` from `tests/lib/validate-event.mjs` (already imported in the test file as `validate`), `load` and `schema` helpers already defined in the test file.
- Produces: `src/examples/demo-chain.json`, an array of three events with ids `evt_01J9K2D00001`, `evt_01J9K2D00002`, `evt_01J9K2D00003`, each with `sources[0]` set. Later tasks rely on those ids and on the order.

- [ ] **Step 1: Write the failing test**

Append to `tests/examples.test.mjs`:

```js
test('the demo chain validates and links only to earlier demo events', () => {
  const chain = load('demo-chain.json');
  assert.deepEqual(chain.map((e) => e.type), ['decision.made', 'change.implemented', 'incident.opened']);
  const seen = new Set();
  for (const e of chain) {
    assert.deepEqual(validate(schema, e), [], e.id);
    assert.ok(e.sources && e.sources[0], `${e.id} needs a source for its card`);
    for (const l of e.links ?? []) assert.ok(seen.has(l.target), `${e.id} links to ${l.target}, which is not an earlier demo event`);
    seen.add(e.id);
  }
  assert.deepEqual(chain[1].links.map((l) => `${l.rel}:${l.basis}`), ['implements:confirmed']);
  assert.deepEqual(chain[2].links.map((l) => `${l.rel}:${l.basis}`), ['observed_after:confirmed', 'caused_by:hypothesis']);
});
```

- [ ] **Step 2: Run it to verify it fails**

Run: `node --permission --allow-fs-read=. --allow-fs-write=./.test-out tests/examples.test.mjs`
Expected: FAIL, `ENOENT` for `src/examples/demo-chain.json`.

- [ ] **Step 3: Create the data**

Create `src/examples/demo-chain.json`:

```json
[
  {
    "schema_version": "0.1-draft",
    "id": "evt_01J9K2D00001",
    "idempotency_key": "demo-spin-decision",
    "type": "decision.made",
    "time": "2026-03-10T14:05:00Z",
    "actor": { "kind": "ai_agent", "id": "agent:claude-code", "on_behalf_of": "user:a.novak" },
    "subject": { "system": "promo-api", "component": "spin-service" },
    "status": "recorded",
    "summary": "Approved spin-to-win: one spin per new account, discount capped at 20%.",
    "sources": [{ "kind": "spec", "ref": "docs/specs/spin-to-win.md" }],
    "provenance": { "recorded_by": "agent:claude-code", "method": "agent_submitted" }
  },
  {
    "schema_version": "0.1-draft",
    "id": "evt_01J9K2D00002",
    "idempotency_key": "demo-spin-pr-482-merged",
    "type": "change.implemented",
    "time": "2026-03-12T09:12:44Z",
    "actor": { "kind": "ai_agent", "id": "agent:claude-code", "on_behalf_of": "user:a.novak" },
    "subject": { "system": "promo-api", "component": "spin-service" },
    "status": "recorded",
    "summary": "PR #482 merged: spin-to-win with the 20% cap.",
    "sources": [{ "kind": "git.pr", "ref": "promo-api#482" }],
    "links": [{ "rel": "implements", "target": "evt_01J9K2D00001", "basis": "confirmed" }],
    "provenance": { "recorded_by": "agent:claude-code", "method": "agent_submitted" }
  },
  {
    "schema_version": "0.1-draft",
    "id": "evt_01J9K2D00003",
    "idempotency_key": "demo-spin-5xx-incident",
    "type": "incident.opened",
    "time": "2026-03-15T13:38:00Z",
    "actor": { "kind": "ai_agent", "id": "agent:claude-code", "on_behalf_of": "user:a.novak" },
    "subject": { "system": "promo-api", "component": "spin-service", "environment": "production" },
    "status": "open",
    "summary": "5xx ratio on /spin reached 6.2%, above the 1.0% rollback threshold.",
    "sources": [{ "kind": "alert", "ref": "alert://promo-api/high-error-rate/a-5521" }],
    "links": [
      { "rel": "observed_after", "target": "evt_01J9K2D00002", "basis": "confirmed" },
      { "rel": "caused_by", "target": "evt_01J9K2D00002", "basis": "hypothesis" }
    ],
    "provenance": { "recorded_by": "agent:claude-code", "method": "agent_submitted" }
  }
]
```

- [ ] **Step 4: Run it to verify it passes**

Run: `node --permission --allow-fs-read=. --allow-fs-write=./.test-out tests/examples.test.mjs`
Expected: PASS. If a schema error names a field (for example in `sources`), fix that field in the JSON, not the test.

- [ ] **Step 5: Commit**

```bash
git add src/examples/demo-chain.json tests/examples.test.mjs
git commit -m "feat(website): add the three-event demo chain for How it works"
```

---

### Task 2: Render the cards at build time

**Files:**
- Modify: `build.mjs` (add `renderDemoCards` after `highlightCode`, add the directive in `render`)
- Test: `tests/landing.test.mjs`

**Interfaces:**
- Consumes: `highlightJson(text: string): string` and `esc(s): string` already in `build.mjs`; `readExample(ctx, file, from)` already in `build.mjs`.
- Produces: `export function renderDemoCards(events: object[]): string`, returning one `<li class="rec-card" id="rec-N" data-rec="N">` per event. The directive `{{demo-cards <file>}}` in a page expands to that string. Card markup classes that Task 3's CSS and Task 4's JS rely on: `rec-card`, `rec-head`, `rec-type`, `rec-id`, `rec-sum`, `rec-meta`, `rec-links`, `rec-link`, `rec-confirmed`, `rec-hypothesis`, `rec-rel`, `rec-basis`, `rec-json`, `rec-hl`.

- [ ] **Step 1: Write the failing tests**

In `tests/landing.test.mjs`, change the build import line to:

```js
import { build, highlightJson, highlightCode, renderDemoCards } from '../build.mjs';
```

and append:

```js
const chain = JSON.parse(readFileSync('src/examples/demo-chain.json', 'utf8'));

test('renderDemoCards: one card per event, links marked by basis, JSON from the same event', () => {
  const out = renderDemoCards(chain);
  assert.equal((out.match(/class="rec-card"/g) || []).length, 3);
  assert.equal((out.match(/class="rec-link rec-confirmed"/g) || []).length, 2);
  assert.equal((out.match(/class="rec-link rec-hypothesis"/g) || []).length, 1);
  assert.deepEqual([...out.matchAll(/data-rec="(\d)"/g)].map((m) => m[1]), ['1', '2', '3']);
  for (const e of chain) {
    assert.ok(out.includes(e.id), `${e.id} is shown`);
    assert.ok(out.includes(e.summary), `${e.id} summary is shown`);
  }
  assert.equal((out.match(/<details class="rec-json">/g) || []).length, 3);
  assert.match(out, /<mark class="rec-hl">[\s\S]*links[\s\S]*<\/mark>/, 'the links field is highlighted in the JSON');
});

test('renderDemoCards: escapes event text and copes with an event that has no sources', () => {
  const evil = { ...chain[0], summary: '<b>x</b> & "y"' };
  delete evil.sources;
  const out = renderDemoCards([evil]);
  assert.doesNotMatch(out, /<b>x/);
  assert.match(out, /&lt;b&gt;x&lt;\/b&gt; &amp; &quot;y&quot;/);
  assert.match(out, /class="rec-card"/);
});

test('the {{demo-cards}} directive is expanded in the built landing page', () => {
  assert.doesNotMatch(html, /\{\{demo-cards/);
});
```

(The third test passes trivially until Task 3 adds the directive to the page; it is the guard that a directive never leaks into output.)

- [ ] **Step 2: Run them to verify they fail**

Run: `node --permission --allow-fs-read=. --allow-fs-write=./.test-out tests/landing.test.mjs`
Expected: FAIL, `renderDemoCards` is not exported.

- [ ] **Step 3: Implement `renderDemoCards`**

In `build.mjs`, add directly after the `highlightCode` function:

```js
// renderDemoCards turns the demo chain into the "Techlog records" cards of the
// landing page: one <li> per event. The summary, the links and the JSON behind
// "Show JSON" all come from the same event, so they cannot disagree. The links
// field is marked in the JSON so a visitor sees what the arrows mean.
export function renderDemoCards(events) {
  return events.map((e, i) => {
    const n = i + 1;
    const src = e.sources && e.sources[0];
    const meta = `${esc(e.time)}${src ? ` · ${esc(src.kind)}: ${esc(src.ref)}` : ''}`;
    const links = (e.links || []).map((l) =>
      `<li class="rec-link rec-${esc(l.basis)}"><span class="rec-rel">${esc(l.rel)}</span> <span aria-hidden="true">→</span> <code>${esc(l.target)}</code> <span class="rec-basis">${esc(l.basis)}</span></li>`).join('');
    const lines = JSON.stringify(e, null, 2).split('\n');
    const from = lines.indexOf('  "links": [');
    const to = from < 0 ? -1 : lines.indexOf('  ],', from);
    const json = lines.map(highlightJson);
    if (from >= 0 && to >= 0) { json[from] = `<mark class="rec-hl">${json[from]}`; json[to] += '</mark>'; }
    return `<li class="rec-card" id="rec-${n}" data-rec="${n}">` +
      `<div class="rec-head"><span class="rec-type">${esc(e.type)}</span><span class="rec-id">${esc(e.id)}</span></div>` +
      `<p class="rec-sum">${esc(e.summary)}</p>` +
      `<p class="rec-meta">${meta}</p>` +
      (links ? `<ul class="rec-links" aria-label="Links to earlier events">${links}</ul>` : '') +
      `<details class="rec-json"><summary>Show JSON</summary><pre><code class="language-json">${json.join('\n')}</code></pre></details>` +
      `</li>`;
  }).join('\n');
}
```

In `render()`, add this replacement right after the `json-script` replacement and before `return out;`:

```js
  out = out.replace(/\{\{demo-cards\s+([^\s}]+)\s*\}\}/g, (_, file) =>
    renderDemoCards(JSON.parse(readExample(ctx, file, vars.__file))));
```

- [ ] **Step 4: Run the tests to verify they pass**

Run: `node --permission --allow-fs-read=. --allow-fs-write=./.test-out tests/landing.test.mjs`
Expected: the three new tests PASS. The existing demo test still passes (the page is not changed yet).

- [ ] **Step 5: Commit**

```bash
git add build.mjs tests/landing.test.mjs
git commit -m "feat(website): render demo event cards from the demo chain at build time"
```

---

### Task 3: The split-view markup and styles

**Files:**
- Modify: `src/pages/index.html` (the `#shell` figure, lines 48 to 58)
- Modify: `src/assets/css/landing.css` (lines 139 to 179 region)
- Modify: `tests/landing.test.mjs` (the test starting at line 84)

**Interfaces:**
- Consumes: `{{demo-cards demo-chain.json}}` and the card classes from Task 2; ids `evt_01J9K2D00001..3` from Task 1.
- Produces: markup contract for Task 4's `shell.js`: `figure#shell`, `.sh-body` (agent pane log), `#sh-input`, `#sh-text`, `#sh-ctl`, `#sh-play`, `#sh-seek`, `#sh-time`; three `.sh-step` elements; each step has exactly one `.sh-res[data-rec="N"]` line (the "✓ recorded" line) where N is 1, 2, 3 in order; the cards `.rec-card` inside `ol.rec-list`.

- [ ] **Step 1: Write the failing test**

In `tests/landing.test.mjs`, replace the existing test that starts `test('landing: how it works is one simulated agent session; the layers follow in their own section', () => {` from its first line down to and including the line
`assert.doesNotMatch(js, /IntersectionObserver|autoplay/, 'does not start by itself');`
(the `const open = ...` assertions after it stay) with:

```js
test('landing: how it works is a split view of three sessions and the records they leave', () => {
  assert.match(how, /<h2>One history\. Every agent\.<\/h2>/);
  assert.doesNotMatch(how, /class="pipe"|class="events"|ev-chain/, 'the three cards and the event flow are gone');
  assert.match(how, /id="shell"[\s\S]*Simulated example/);
  assert.match(how, /Agent session[\s\S]*Techlog records/, 'two labelled panes');
  const typed = [...how.matchAll(/class="sh-typed">([^<]*)</g)].map((m) => m[1]);
  assert.deepEqual(typed.filter((c) => c !== '/techlog:record'), [
    'Approve the spin-to-win design with a 20% cap',
    'Merge PR #482',
    'Investigate PagerDuty alert P1-5521',
  ]);
  assert.equal(typed.filter((c) => c === '/techlog:record').length, 3, 'one /techlog:record per session');
  assert.deepEqual([...how.matchAll(/class="sh-l sh-rule">── ([^<]*) ──</g)].map((m) => m[1]), ['a new session, 2 days later', 'a new session, 3 days later']);
  assert.equal((how.match(/class="sh-step"/g) || []).length, 3);
  assert.deepEqual([...how.matchAll(/class="sh-l sh-res" data-rec="(\d)"/g)].map((m) => m[1]), ['1', '2', '3'], 'one record line per session, in order');
  assert.deepEqual([...how.matchAll(/✓ recorded (evt_\w+)/g)].map((m) => m[1]), chain.map((e) => e.id), 'the transcript records the same events as the cards');
  assert.equal((how.match(/class="rec-card"/g) || []).length, 3);
  assert.match(how, /rec-hypothesis/, 'the unproven cause is shown as a hypothesis');
  assert.doesNotMatch(how, /techlog_record_event|kubectl|gh pr|pd incident|jira issue/, 'no tool-call noise, and the skill rather than the raw tool is shown');
  assert.doesNotMatch(how, /class="(rec-card|sh-step)"[^>]*\bhidden\b/, 'without JavaScript everything is visible');
  assert.match(how, /id="sh-play"[^>]*>▶ Run simulation/, 'starts idle with a call to run');
  const js = readFileSync('src/assets/js/shell.js', 'utf8');
  assert.doesNotMatch(js, /IntersectionObserver|autoplay/, 'does not start by itself');
  assert.match(css, /@media \(max-width: 700px\)[^\n]*\.sh-panes/, 'the panes stack on a phone');
});

test('landing: the layers follow how it works in their own section', () => {
```

Do not delete the `const open = ...` lines or the closing `});` that follow: they now belong to the second test.

- [ ] **Step 2: Run it to verify it fails**

Run: `node --permission --allow-fs-read=. --allow-fs-write=./.test-out tests/landing.test.mjs`
Expected: FAIL on the new structure test (`Agent session` not found).

- [ ] **Step 3: Replace the figure in `index.html`**

Replace everything from `<figure class="shell cs" id="shell" aria-labelledby="sh-cap">` through its closing `</figure>` (keep the `<script src="{{root}}assets/js/shell.js"></script>` line after it) with:

```html
    <figure class="shell cs" id="shell" aria-labelledby="sh-cap">
      <figcaption><span id="sh-cap">Sessions forget. Techlog remembers.</span><span class="badge badge-simulated">Simulated example</span></figcaption>
      <div class="sh-panes">
        <div class="sh-pane sh-pane-agent">
          <p class="sh-pane-title">Agent session <span class="sh-pane-note">starts empty every time</span></p>
          <div class="sh-body" tabindex="0" role="log" aria-label="Simulated agent transcript">
            <div class="sh-idle-hint"><p>Press <strong>Run simulation</strong> to watch three sessions leave linked records.</p></div>
<div class="sh-step"><div class="sh-l sh-user"><span class="sh-gt">&gt;</span> <span class="sh-typed">Approve the spin-to-win design with a 20% cap</span></div><div class="sh-l sh-say"><span class="sh-dot">●</span> Design approved: one spin per new account, discount capped at 20%.</div><div class="sh-l sh-user"><span class="sh-gt">&gt;</span> <span class="sh-typed">/techlog:record</span></div><div class="sh-l sh-tool"><span class="sh-dot">●</span> /techlog:record<span class="sh-args">(<span class="sh-k">decision.made</span>)</span></div><div class="sh-l sh-res" data-rec="1"><span class="sh-elbow">⎿</span>  <span class="sh-ok">✓ recorded evt_01J9K2D00001</span></div></div>
<div class="sh-step"><div class="sh-l sh-rule">── a new session, 2 days later ──</div><div class="sh-l sh-user"><span class="sh-gt">&gt;</span> <span class="sh-typed">Merge PR #482</span></div><div class="sh-l sh-say"><span class="sh-dot">●</span> Merged PR #482: spin-to-win with the 20% cap.</div><div class="sh-l sh-user"><span class="sh-gt">&gt;</span> <span class="sh-typed">/techlog:record</span></div><div class="sh-l sh-tool"><span class="sh-dot">●</span> /techlog:record<span class="sh-args">(<span class="sh-k">change.implemented</span>)</span></div><div class="sh-l sh-res"><span class="sh-elbow">⎿</span>  found  evt_01J9K2D00001  the approved design</div><div class="sh-l sh-res"><span class="sh-elbow">⎿</span>  link implements → evt_01J9K2D00001 <span class="sh-ok">━━ confirmed</span></div><div class="sh-l sh-res" data-rec="2"><span class="sh-elbow">⎿</span>  <span class="sh-ok">✓ recorded evt_01J9K2D00002</span></div></div>
<div class="sh-step"><div class="sh-l sh-rule">── a new session, 3 days later ──</div><div class="sh-l sh-user"><span class="sh-gt">&gt;</span> <span class="sh-typed">Investigate PagerDuty alert P1-5521</span></div><div class="sh-l sh-say"><span class="sh-dot">●</span> 5xx on /spin is at 6.2%, three days after PR #482. That does not prove the PR caused it.</div><div class="sh-l sh-user"><span class="sh-gt">&gt;</span> <span class="sh-typed">/techlog:record</span></div><div class="sh-l sh-tool"><span class="sh-dot">●</span> /techlog:record<span class="sh-args">(<span class="sh-k">incident.opened</span>)</span></div><div class="sh-l sh-res"><span class="sh-elbow">⎿</span>  link observed_after → evt_01J9K2D00002 <span class="sh-ok">━━ confirmed</span></div><div class="sh-l sh-res"><span class="sh-elbow">⎿</span>  link caused_by → evt_01J9K2D00002 <span class="sh-hyp">┅┅ hypothesis</span></div><div class="sh-l sh-res" data-rec="3"><span class="sh-elbow">⎿</span>  <span class="sh-ok">✓ recorded evt_01J9K2D00003</span></div></div>
          </div>
          <div class="sh-input" id="sh-input" aria-hidden="true" hidden><span class="sh-gt">&gt;</span> <span class="sh-line"><span id="sh-text"></span><span class="sh-caret"></span></span></div>
        </div>
        <div class="sh-pane sh-pane-rec">
          <p class="sh-pane-title">Techlog records <span class="sh-pane-note">kept across sessions</span></p>
          <ol class="rec-list" aria-label="Events recorded in Techlog">
{{demo-cards demo-chain.json}}
          </ol>
        </div>
      </div>
      <div class="sh-ctl" id="sh-ctl" hidden>
        <button type="button" class="sh-play" id="sh-play" aria-label="Run simulation">▶ Run simulation</button>
        <input type="range" class="sh-seek" id="sh-seek" min="0" max="1000" step="1" value="0" aria-label="Playback position">
        <span class="sh-time" id="sh-time" aria-hidden="true">0:00 / 0:00</span>
      </div>
      <p class="sh-foot">Each <code>/techlog:record</code> call writes one event and links it to the earlier event it follows. Open <strong>Show JSON</strong> on a card to see exactly what is stored. An unproven cause stays a hypothesis.</p>
    </figure>
```

- [ ] **Step 4: Update the styles**

In `src/assets/css/landing.css`:

a. Change the `.sh-body` rule (line 142) height from `27rem` to `20rem`.

b. Replace the line `.sh-js .sh-step:not(.sh-on) { display: none; }` with:

```css
.sh-js .sh-step:not(.sh-cur) { display: none; }
.sh-js .rec-card:not(.sh-on) { display: none; }
```

c. Replace the final `@media (max-width: 700px) { ... }` line (line 178) with the same rule plus the new parts:

```css
@media (max-width: 700px) { .sh-body, .shell figcaption, .sh-foot { padding-inline: var(--s-3); } .sh-input { margin-inline: var(--s-3); } .sh-ctl { padding-inline: var(--s-3); } .sh-body { height: 16rem; } .sh-panes { grid-template-columns: 1fr; } .sh-pane + .sh-pane { border-left: 0; border-top: 1px solid var(--line-strong); } .rec-list { height: auto; max-height: 24rem; padding-inline: var(--s-3); } .sh-pane-title { padding-inline: var(--s-3); } }
```

d. Append before the `@media (prefers-reduced-motion` line:

```css
.sh-panes { display: grid; grid-template-columns: 1fr 1fr; }
.sh-pane { min-width: 0; display: flex; flex-direction: column; }
.sh-pane + .sh-pane { border-left: 1px solid var(--line-strong); }
.sh-pane-title { margin: 0; padding: var(--s-3) var(--s-5) 0; font: 700 var(--fs-xs) var(--font-mono); letter-spacing: .08em; text-transform: uppercase; color: var(--text-1); }
.sh-pane-note { margin-left: var(--s-2); font-weight: 400; letter-spacing: 0; text-transform: none; color: var(--text-2); }
.rec-list { list-style: none; margin: 0; padding: var(--s-4) var(--s-5); display: grid; align-content: start; gap: var(--s-3); height: 24rem; overflow: auto; }
.rec-card { padding: var(--s-3) var(--s-4); border: 1px solid var(--line-strong); border-radius: var(--r-2); background: var(--bg-1); font: var(--fs-sm)/1.5 var(--font-sans); color: var(--text-1); }
.rec-head { display: flex; justify-content: space-between; gap: var(--s-3); flex-wrap: wrap; font: 700 var(--fs-xs) var(--font-mono); }
.rec-type { color: var(--info); }
.rec-id { color: var(--text-2); }
.rec-sum { margin: var(--s-2) 0; color: var(--text-0); }
.rec-meta { margin: 0; overflow-wrap: anywhere; font: var(--fs-xs) var(--font-mono); color: var(--text-2); }
.rec-links { list-style: none; margin: var(--s-2) 0 0; padding: 0; display: grid; gap: var(--s-1); }
.rec-link { padding-left: var(--s-3); border-left: 3px solid var(--ok); font: var(--fs-xs) var(--font-mono); overflow-wrap: anywhere; }
.rec-link.rec-hypothesis { border-left-style: dashed; border-left-color: var(--hyp); color: var(--hyp); }
.rec-confirmed .rec-basis { color: var(--ok); }
.rec-json summary { margin-top: var(--s-2); cursor: pointer; font: 600 var(--fs-xs) var(--font-mono); color: var(--accent); }
.rec-json pre { margin: var(--s-2) 0 0; max-width: 100%; overflow: auto; font: var(--fs-xs)/1.5 var(--font-mono); }
.rec-hl { background: color-mix(in srgb, var(--accent) 18%, transparent); }
```

- [ ] **Step 5: Run the tests to verify they pass**

Run: `npm test`
Expected: PASS for the whole suite, including the new structure test and the unchanged `open` test. If `tokens.test.mjs` or another test reacts to the new CSS (for example a contrast or token-use check), fix the CSS to use existing tokens rather than loosening the test.

- [ ] **Step 6: Commit**

```bash
git add src/pages/index.html src/assets/css/landing.css tests/landing.test.mjs
git commit -m "feat(website): show How it works as agent sessions beside the records they leave"
```

---

### Task 4: Playback for two panes

**Files:**
- Modify: `src/assets/js/shell.js` (full replacement)
- Test: `tests/landing.test.mjs`

**Interfaces:**
- Consumes: the markup contract from Task 3 (`.sh-step`, `.sh-l`, `.sh-res[data-rec]`, `.rec-card`, `#sh-*` ids).
- Produces: classes toggled at runtime: `sh-on` on lines, steps and cards that have been reached; `sh-cur` on the one step currently shown; `sh-idle` and `sh-js` on the figure (unchanged).

- [ ] **Step 1: Write the failing test**

Append to `tests/landing.test.mjs`:

```js
test('shell.js: one session at a time, cards accumulate, 45 seconds, reduced motion, no autoplay', () => {
  const js = readFileSync('src/assets/js/shell.js', 'utf8');
  assert.match(js, /TARGET_MS = 45000/);
  assert.match(js, /sh-cur/, 'only the current session is shown on the left');
  assert.match(js, /data-rec/, 'cards are tied to the record line of their session');
  assert.match(js, /prefers-reduced-motion/, 'typing is skipped for reduced motion');
  assert.doesNotMatch(js, /IntersectionObserver|autoplay/);
});
```

- [ ] **Step 2: Run it to verify it fails**

Run: `node --permission --allow-fs-read=. --allow-fs-write=./.test-out tests/landing.test.mjs`
Expected: FAIL on `TARGET_MS = 45000`.

- [ ] **Step 3: Replace `src/assets/js/shell.js`**

```js
(function () {
  // simulated demo on a timeline: three agent sessions on the left, the records they leave in Techlog on the right.
  // The agent pane shows one session at a time (a new session starts empty); the record cards only ever accumulate.
  // The whole run is a pure function of time, so the bar can seek.
  var fig = document.getElementById('shell');
  if (!fig) return;
  var body = fig.querySelector('.sh-body');
  var input = document.getElementById('sh-input');
  var text = document.getElementById('sh-text');
  var ctl = document.getElementById('sh-ctl');
  var playBtn = document.getElementById('sh-play');
  var seek = document.getElementById('sh-seek');
  var clock = document.getElementById('sh-time');
  var reduce = !!(window.matchMedia && window.matchMedia('(prefers-reduced-motion: reduce)').matches);
  // typing and output delays scale with one factor; the pause after a new-session divider is absolute.
  // The factor is solved so the whole run lasts TARGET_MS, pauses included.
  var TARGET_MS = 45000, SESSION_PAUSE_MS = 2000;
  var TYPE_MS = 30, LEAD_MS = 350;
  var items = [], steps = [], total = 0;
  var stepEls = fig.querySelectorAll('.sh-step');
  var cardEls = fig.querySelectorAll('.rec-card');

  function layout(slow) {
    TYPE_MS = 30 * slow; LEAD_MS = 350 * slow;
    var gap = 600 * slow, t = 400;
    items = []; steps = [];
    stepEls.forEach(function (st) {
      // a new session's divider shows at once, right after the last line; typing starts SESSION_PAUSE_MS later
      if (st.querySelector('.sh-rule')) t -= gap;
      var step = { el: st, first: -1 };
      st.querySelectorAll('.sh-l').forEach(function (l) {
        var it = { el: l, step: step };
        if (l.classList.contains('sh-user')) {
          it.cmd = true;
          it.full = l.querySelector('.sh-typed').textContent.trim();
          it.start = t + LEAD_MS;
          it.send = it.start + it.full.length * TYPE_MS + 300 * slow;
          it.at = it.send;
          t = it.send + 450 * slow;
        } else {
          it.at = t;
          t += l.classList.contains('sh-rule') ? SESSION_PAUSE_MS - LEAD_MS : slow * (l.classList.contains('sh-tool') ? 550 : l.classList.contains('sh-say') ? 450 : 300);
        }
        if (step.first < 0) step.first = it.at;
        items.push(it);
        // the record card appears just after the line that says the event was recorded
        var rec = l.getAttribute('data-rec');
        if (rec && cardEls[Number(rec) - 1]) items.push({ el: cardEls[Number(rec) - 1], at: it.at + 300 * slow });
      });
      t += gap; steps.push(step);
    });
    return t;
  }
  var t1 = layout(1), t2 = layout(2);       // total time is linear in the factor
  var slope = t2 - t1, fixed = t1 - slope;
  total = layout(Math.max(0.5, (TARGET_MS - fixed) / slope));

  var pos = 0, playing = false, last = 0, raf = 0, wasPlaying = false;
  function fmt(ms) { var s = Math.round(ms / 1000); return Math.floor(s / 60) + ':' + ('0' + (s % 60)).slice(-2); }

  function render() {
    var typing = '';
    items.forEach(function (it) {
      var on = pos >= it.at;
      if (it.el.classList.contains('sh-on') !== on) it.el.classList.toggle('sh-on', on);
      if (it.cmd && pos >= it.start && pos < it.send) typing = reduce ? it.full : it.full.slice(0, Math.floor((pos - it.start) / TYPE_MS));
    });
    var cur = -1;
    steps.forEach(function (s, i) { if (pos >= s.first) cur = i; });
    steps.forEach(function (s, i) {
      if (s.el.classList.contains('sh-on') !== (i <= cur)) s.el.classList.toggle('sh-on', i <= cur);
      if (s.el.classList.contains('sh-cur') !== (i === cur)) s.el.classList.toggle('sh-cur', i === cur);
    });
    text.textContent = typing;
    body.scrollTop = body.scrollHeight;
    seek.value = String(Math.round(pos / total * 1000));
    clock.textContent = fmt(pos) + ' / ' + fmt(total);
    seek.setAttribute('aria-valuetext', fmt(pos) + ' of ' + fmt(total));
  }
  function label() {
    var atEnd = pos >= total;
    var idle = !playing && pos === 0;
    fig.classList.toggle('sh-idle', idle);
    playBtn.textContent = playing ? '❚❚' : idle ? '▶ Run simulation' : atEnd ? '↺' : '▶';
    playBtn.setAttribute('aria-label', playing ? 'Pause' : idle ? 'Run simulation' : atEnd ? 'Replay' : 'Play');
  }
  function tick(now) {
    if (!playing) return;
    pos = Math.min(total, pos + (now - last)); last = now;
    render();
    if (pos >= total) { playing = false; label(); return; }
    raf = requestAnimationFrame(tick);
  }
  function play() {
    if (pos >= total) pos = 0;
    playing = true; label(); last = performance.now(); raf = requestAnimationFrame(tick);
  }
  function pause() { playing = false; cancelAnimationFrame(raf); label(); }

  playBtn.addEventListener('click', function () { if (playing) pause(); else play(); });
  seek.addEventListener('input', function () {
    if (playing) { wasPlaying = true; pause(); }
    pos = Number(seek.value) / 1000 * total; render(); label();
  });
  seek.addEventListener('change', function () { if (wasPlaying) { wasPlaying = false; play(); } });

  // starts idle: nothing is shown until the visitor presses the glowing button
  fig.classList.add('sh-js'); ctl.hidden = false; input.hidden = false;
  render(); label();
})();
```

- [ ] **Step 4: Run the tests to verify they pass**

Run: `npm test`
Expected: PASS.

- [ ] **Step 5: Check the behaviour in a browser (the test cannot, there is no DOM)**

Run: `npm run build && npm run serve`, open `http://localhost:8090/index.html#how`, and check each of these. Expected results in brackets.
1. Idle: the hint shows on the left, no cards on the right, the play button glows. [yes]
2. Play: session 1 types, "✓ recorded" appears, then card 1 appears. At "a new session, 2 days later" the left pane is empty again and card 1 stays. [yes]
3. Drag the seek bar back to the middle: only the session at that time shows on the left, and only the cards already reached show on the right. Drag to the very start: everything hides. [yes]
4. At the end all 3 cards show; "Show JSON" opens the JSON with the `links` block tinted; the third card shows a dashed hypothesis link. [yes]
5. Total run is about 45 seconds. [0:45 on the clock, give or take a second]
6. With "Emulate CSS prefers-reduced-motion: reduce" in devtools, commands appear whole, not letter by letter. [yes]

If any check fails, fix `shell.js` or the CSS and re-run.

- [ ] **Step 6: Commit**

```bash
git add src/assets/js/shell.js tests/landing.test.mjs
git commit -m "feat(website): play the demo as sessions that clear beside records that stay"
```

---

### Task 5: Spec wording, site checks, final verification

**Files:**
- Modify: `docs/superpowers/specs/2026-10-16-how-it-works-demo-design.md`

**Interfaces:**
- Consumes: the finished work of Tasks 1 to 4.
- Produces: nothing for later tasks.

- [ ] **Step 1: Align the spec with the plan's one refinement**

In the spec, in the Behaviour list, replace
`the JSON toggle is a button with \`aria-expanded\`.`
with
`the JSON toggle is a native \`<details>\`/\`<summary>\`, which works without JavaScript and exposes its expanded state to assistive tech.`
and in Testing replace
`the JSON toggle flips \`aria-expanded\`.`
with
`each card has a \`<details>\` JSON toggle (checked by hand in a browser, since the suite has no DOM).`

- [ ] **Step 2: Run the whole suite and the site checks**

Run: `npm test && npm run build && npm run check`
Expected: all PASS; link checker and content audit report no problems. If the audit flags the new copy (for example a banned word or an unmarked claim), reword the copy in `index.html`, not the audit.

- [ ] **Step 3: Check a 320px viewport and JS off**

In the served page (`npm run serve`), with devtools at 320px width: no horizontal page scroll, panes stacked with the records under the transcript, long JSON scrolls inside its card. Then disable JavaScript and reload: all 3 sessions and 3 cards are visible, "Show JSON" opens, and the play button area is hidden. [all yes]

- [ ] **Step 4: Commit**

```bash
git add docs/superpowers/specs/2026-10-16-how-it-works-demo-design.md
git commit -m "docs(website): note the details toggle in the demo spec"
```

---

## Self-review

**Spec coverage.** Story and 3 events: Task 1. Idea carried by layout (left clears, right grows): Tasks 3 (markup, `sh-cur`) and 4 (JS). Two panes, cards, solid and dashed links, Show JSON with `links` highlighted: Tasks 2 and 3. Playback model, no autoplay, 45 s: Task 4. No-JS fallback: Task 3 test and Task 5 check. Reduced motion: Task 4. Accessibility (`role="log"` kept, `aria-label`s, `details`): Tasks 2 and 3. Data and drift: Tasks 1 and 2 (single file, validated, rendered by the build). Files touched: matches the spec, plus `build.mjs` (needed for the single-source rendering; the spec said "the page build reads it") and `tests/examples.test.mjs`. Out of scope items stay untouched.

**Placeholders.** None: every step has its code or exact command.

**Type consistency.** `renderDemoCards(events)` is the same name in Task 2 code and tests; card classes in Task 2 match the CSS in Task 3 and the JS in Task 4; `data-rec` on the record lines (Task 3) is what Task 4 reads; event ids `evt_01J9K2D00001..3` match across Tasks 1 and 3.

**Review Focus.** All five items have a named test or manual check above.
