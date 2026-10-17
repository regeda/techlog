import test from 'node:test';
import assert from 'node:assert/strict';
import { readFileSync, mkdirSync, existsSync } from 'node:fs';
import { build, highlightJson, highlightCode, renderDemoEvents } from '../build.mjs';

mkdirSync('.test-out', { recursive: true });
build({ srcDir: 'src', outDir: '.test-out/landing' });
const html = readFileSync('.test-out/landing/index.html', 'utf8');
const how = html.slice(html.indexOf('id="how"'), html.indexOf('id="open"'));
const css = readFileSync('src/assets/css/landing.css', 'utf8');

test('landing: no JSON example and no connectors section', () => {
  assert.doesNotMatch(html, /plain JSON|id="connectors"|Available connectors|class="pane"|class="codeblock"/);
  assert.doesNotMatch(html, /#connectors/);
});

test('highlightJson escapes markup and round-trips the text', () => {
  const src = '{ "a<b": ["x&y", 1.5e3, true, null] }';
  const out = highlightJson(src);
  assert.doesNotMatch(out, /<b|x&y/);
  assert.equal(out.replace(/<[^>]+>/g, '').replace(/&quot;/g, '"').replace(/&lt;/g, '<').replace(/&amp;/g, '&'), src);
});

const welcome = readFileSync('.test-out/landing/welcome.html', 'utf8');

test('welcome: explains how to run the server and connect Claude', () => {
  assert.match(welcome, /Run the server on your machine/);
  assert.match(welcome, /make seed/);
  assert.match(welcome, /make run/);
  assert.match(plain(welcome), /claude mcp add --transport http techlog http:\/\/localhost:8080\/mcp/);
  assert.match(welcome, /localhost:8080/);
  assert.match(welcome, /href="docs\/server\.html"/, 'links to the Run the server page');
  assert.match(welcome, /no authentication/i, 'says plainly what the server does not do');
});

test('welcome: the commands come from the same file as Getting Started', () => {
  const gs = readFileSync('src/examples/getting-started/run-server.sh', 'utf8').trim();
  assert.ok(plain(welcome).includes(gs), 'run-server.sh is embedded verbatim');
});

// plain turns highlighted markup back into the text a reader would copy.
function plain(html) {
  return html.replace(/<[^>]+>/g, '').replace(/&quot;/g, '"').replace(/&lt;/g, '<').replace(/&gt;/g, '>').replace(/&amp;/g, '&');
}

test('getting started: every sample language is highlighted and keeps its source text', () => {
  const gs = readFileSync('.test-out/landing/docs/getting-started.html', 'utf8');
  const samples = { bash: 'quickstart.sh', python: 'quickstart.py', typescript: 'quickstart.ts', go: 'quickstart.go', java: 'Quickstart.java' };
  for (const [lang, file] of Object.entries(samples)) {
    const src = readFileSync(`src/examples/getting-started/${file}`, 'utf8').trim();
    const blocks = [...gs.matchAll(new RegExp(`<code class="language-${lang}">([\\s\\S]*?)</code>`, 'g'))].map((m) => m[1]);
    const block = blocks.find((b) => plain(b) === src) ?? '';
    assert.match(block, /class="tk-com"/, `${lang}: comments`);
    assert.match(block, /class="tk-str"/, `${lang}: strings`);
    assert.match(block, /class="tk-kw"/, `${lang}: keywords`);
    assert.equal(plain(block), src, `${lang}: text is unchanged`);
  }
});

test('highlightCode leaves unknown languages as escaped plain text', () => {
  assert.equal(highlightCode('cobol', 'a < b'), 'a &lt; b');
});

test('welcome: the opening does not claim there is nothing to run', () => {
  assert.match(welcome, /run a basic server yourself|run the basic server yourself/i);
});

test('landing: the logo lives in the header bar, and the motto is big, in two lines', () => {
  assert.doesNotMatch(html, /hero-logo|hero-head/);
  assert.match(html, /<header class="site-header">[\s\S]*?<img src="assets\/img\/logo\.svg"[\s\S]*?<span class="brand-name">Techlog<\/span>/);
  assert.doesNotMatch(html, /Forget Pull Requests/i);
  assert.match(html, /<title>AI Engineering Context — Techlog/);
  assert.match(html, /<h1><span class="shout">Connect Engineering Work<\/span><\/h1>/, 'one line: the motto');
  assert.match(css, /\.hero h1 \{[^}]*white-space:\s*nowrap[^}]*font-size:\s*clamp\(1\.5rem, 8cqw, 8\.2rem\)/, 'the motto stays on one line and scales with its container');
  assert.match(css, /\.hero \.container \{[^}]*container-type:\s*inline-size/);
  const js = readFileSync('src/assets/js/site.js', 'utf8');
  assert.doesNotMatch(js, /has-hero-logo|brand-on/);
});



test('landing: how it works is a split view of four sessions and the records they leave', () => {
  assert.match(how, /<h2>One history\. Every agent\.<\/h2>/);
  assert.doesNotMatch(how, /class="pipe"|class="events"|ev-chain/, 'the three cards and the event flow are gone');
  assert.match(how, /id="shell"[\s\S]*Simulated example/);
  assert.match(how, /Agent session[\s\S]*Techlog records/, 'two labelled panes');
  const typed = [...how.matchAll(/class="sh-typed">([^<]*)</g)].map((m) => m[1]);
  assert.deepEqual(typed, ['/techlog:problem', 'yes', '/techlog:proposal', 'Approved. Plan it.', 'Merge PR #482', '/techlog:record', 'Investigate PagerDuty alert P1-5521', '/techlog:record']);
  assert.deepEqual([...how.matchAll(/class="sh-l sh-rule">── ([^<]*) ──</g)].map((m) => m[1]), ['a new session, today later', 'a new session, 1 day later', 'a new session, 3 days later']);
  assert.equal((how.match(/class="sh-step"/g) || []).length, 4);
  assert.deepEqual([...how.matchAll(/class="sh-l sh-res" data-rec="(\d)"/g)].map((m) => m[1]), ['1', '2', '3', '4', '5', '6'], 'one record line per event, in order');
  assert.doesNotMatch(how, /techlog:welcome/, 'welcome is not part of the demo');
  assert.doesNotMatch(how, /data-trace/, 'the incident trace is text only, with no highlight animation');
  assert.match(how, /quick context: incident/, 'the quick context line stays');
  assert.doesNotMatch(how, /data-tab|sh-tabs/, 'no tabs on the left: the sessions simply stay');
  assert.doesNotMatch(how, /Show JSON|rec-json/, 'the cards show no JSON');
  assert.doesNotMatch(how, /resolved/i, 'the incident is never claimed resolved');
  assert.match(how, /four sessions/);
  assert.deepEqual([...how.matchAll(/✓ recorded (evt_\w+)/g)].map((m) => m[1]), chain.map((e) => e.id), 'the transcript records the same events as the cards');
  assert.equal((how.match(/class="rec-card"/g) || []).length, 6);
  assert.match(how, /rec-hypothesis/, 'the unproven cause is shown as a hypothesis');
  assert.doesNotMatch(how, /techlog_record_event|kubectl|gh pr|pd incident|jira issue/, 'no tool-call noise, and the skill rather than the raw tool is shown');
  assert.doesNotMatch(how, /class="(rec-card|sh-step)"[^>]*\bhidden\b/, 'without JavaScript everything is visible');
  assert.match(how, /id="sh-play"[^>]*>▶ Run simulation/, 'starts idle with a call to run');
  const js = readFileSync('src/assets/js/shell.js', 'utf8');
  assert.doesNotMatch(js, /IntersectionObserver|autoplay/, 'does not start by itself');
  assert.match(css, /@media \(max-width: 700px\)[^\n]*\.sh-panes/, 'the panes stack on a phone');
});

test('landing: the layers follow how it works in their own section', () => {
  const open = html.slice(html.indexOf('id="open"'), html.indexOf('id="start"'));
  assert.match(open, /<h2>Open by design\. Built for agents\.<\/h2>/);
  assert.ok(html.indexOf('id="open"') > html.indexOf('id="how"'));
  const layers = open.match(/<ul class="layers"[\s\S]*?<\/ul>/)[0];
  assert.deepEqual([...layers.matchAll(/<h3>([^<]*)<\/h3>/g)].map((m) => m[1]), ['Open event standard', 'MCP interface', 'Techlog platform']);
  assert.match(layers, /no hosted service or tool connector yet/);
  assert.deepEqual([...layers.matchAll(/class="badge [^"]*">([^<]*)</g)].map((m) => m[1]), ['Proposed · draft 0.1', 'Implemented · basic', 'Local only']);
});

test('lifecycle: "Confirm hypothesis" comes after Stages, as a four-block diagram with little text', () => {
  const lc = readFileSync('.test-out/landing/docs/lifecycle.html', 'utf8');
  assert.match(lc, /<h2 id="confirm-hypothesis">Confirm hypothesis<\/h2>/);
  assert.doesNotMatch(lc, /From hypothesis to confirmed<\/h2>|id="hypothesis"/);
  assert.ok(lc.indexOf('id="stages"') < lc.indexOf('id="confirm-hypothesis"') && lc.indexOf('id="confirm-hypothesis"') < lc.indexOf('id="rules"'), 'after Stages, before Rules of thumb');
  const sec = lc.slice(lc.indexOf('id="confirm-hypothesis"'), lc.indexOf('id="rules"'));
  assert.equal((sec.match(/<li[ >]/g) ?? []).length, 4, 'four blocks at most');
  assert.deepEqual([...sec.matchAll(/<strong>([^<]*)<\/strong>/g)].map((m) => m[1]), ['Guess', 'Act', 'Confirm', 'Fix']);
  for (const w of ['badge-hypothesis', 'badge-confirmed']) assert.ok(sec.includes(w), w);
  assert.doesNotMatch(sec, /<table|<ul/, 'a diagram, not a table or a list');
  assert.doesNotMatch(sec, /In the example story|Nothing is edited/, 'no example paragraph');
  const words = sec.replace(/<[^>]+>/g, ' ').split(/\s+/).filter(Boolean).length;
  assert.ok(words <= 100, `short: ${words} words`);
  const components = readFileSync('src/assets/css/components.css', 'utf8');
  assert.match(components, /\.confirm \{[^}]*grid-template-columns:\s*repeat\(4,/);
});

test('header: the Techlog name stays next to the logo, also after scrolling', () => {
  const header = readFileSync('src/partials/header.html', 'utf8');
  assert.match(header, /<span class="brand-name">Techlog<\/span>/);
  const components = readFileSync('src/assets/css/components.css', 'utf8');
  assert.doesNotMatch(components, /\.scrolled \.site-header \.brand-name/, 'the name is not collapsed on scroll');
  assert.match(html, /<span class="brand-name">Techlog<\/span>/);
});

test('landing: the hero is centred', () => {
  assert.match(css, /\.hero h1 \{[^}]*text-align:\s*center/);
  assert.match(css, /\.hero-row \{[^}]*justify-content:\s*center[^}]*text-align:\s*center/);
});



test('landing: the demo is gone', () => {
  assert.doesNotMatch(html, /id="demo"|demo-data|demo-events|demo-core|assets\/js\/demo\.js/);
  assert.doesNotMatch(html, /class="cs dm"/);
  assert.ok(!existsSync('src/assets/js/demo.js') && !existsSync('src/assets/css/demo.css') && !existsSync('src/examples/demo-steps.json'));
  assert.doesNotMatch(readFileSync('src/partials/head.html', 'utf8'), /demo\.css/);
});

test('landing: Read the docs sits under the animation, and Try Techlog is only in the header', () => {
  const after = html.slice(html.indexOf('assets/js/scatter.js'));
  const cta = after.match(/<div class="hero-cta">([\s\S]*?)<\/div>/)[1];
  assert.match(cta, /btn btn-xl btn-grey" href="docs\/index\.html"><span aria-hidden="true">📖<\/span> Read the docs</);
  assert.doesNotMatch(html.replace(/<header[\s\S]*?<\/header>/, ''), /Try Techlog|Coming soon|cta-soon/, 'no Try Techlog outside the header');
  assert.match(css, /\.btn-xl\s*\{[^}]*font-size:\s*var\(--fs-xl\)/);
});

test('landing: eight items from four systems start scattered with tangled links and end as one looped lifecycle', () => {
  const sc = html.slice(html.indexOf('id="scatter"'), html.indexOf('assets/js/scatter.js'));
  assert.equal((sc.match(/class="sc-card /g) ?? []).length, 10, 'eight steps and two AI agents');
  for (const sys of ['sc-slack', 'sc-github', 'sc-atlassian', 'sc-pagerduty']) assert.ok(sc.includes(sys), sys);
  for (const w of ['Slack', 'GitHub', 'Jira', 'Confluence', 'PagerDuty']) assert.ok(sc.includes(w), w);
  const stages = [...sc.matchAll(/class="sc-step">([^<]*)</g)].map((m) => m[1]);
  assert.deepEqual(stages, ['1 · Problem', '2 · Decision', '3 · Change', '4 · Release', '5 · Incident', '6 · Rollback', '7 · Postmortem', '8 · Corrective action']);
  assert.doesNotMatch(sc, /sc-nav|<strong>Humans<\/strong>/, "no humans/agents block under the demo");
  for (let n = 1; n <= 10; n++) assert.match(css, new RegExp(`\\.sc-card:nth-child\\(${n}\\) \\{[^}]*--sx:[^}]*--cx:[^}]*--cy:`), `card ${n} has both positions`);
  assert.match(css, /\.sc-chaos \{[^}]*stroke-dasharray/);
  assert.match(css, /\.sc-loop \{[^}]*animation:\s*sc-flow/);
});

test('landing: the Techlog robot is one element in the centre of both states, grey and glowing at first, and Replay appears top right', () => {
  const sc = html.slice(html.indexOf('id="scatter"'), html.indexOf('assets/js/scatter.js'));
  assert.doesNotMatch(sc, /sc-progress|sc-range|sc-end|type="range"/, 'no slide control below');
  assert.match(sc, /id="sc-caption" aria-live="polite">Legacy Changelog<\/p>/);
  assert.equal((sc.match(/logo\.svg/g) ?? []).length, 1, 'a single Techlog robot, used for both states');
  assert.doesNotMatch(sc, /sc-hub/, 'no second robot for the after state');
  const stage = sc.slice(sc.indexOf('class="sc-stage"'));
  assert.match(stage, /^class="sc-stage">\s*<button type="button" class="sc-go" id="sc-go" aria-label="Connect Techlog" title="[^"]*"><span class="sc-go-disc"><img src="assets\/img\/logo\.svg"[^>]*><\/span><span class="sc-go-name">Techlog<\/span><\/button>/);
  assert.match(sc, /id="sc-replay" hidden>↺ Replay<\/button>/, 'Replay is in the top bar and hidden at first');
  assert.match(css, /\.sc-go \{[^}]*position:\s*absolute;\s*left:\s*50%;\s*top:\s*50%;[^}]*transform:\s*translate\(-50%, -50%\)/, 'the centre of the stage');
  assert.match(css, /\.sc-go-disc \{[^}]*width:\s*4\.75rem;[^}]*height:\s*4\.75rem[^}]*border-radius:\s*50%[^}]*animation:\s*sc-ask 1\.8s ease-in-out infinite/, 'round, and the disc itself pulses');
  assert.match(css, /\.sc-go img \{[^}]*filter:\s*grayscale\(1\)/, 'the grey filter is on the picture, so it does not grey the glow');
  assert.match(css, /\.sc-go\.is-on \.sc-go-disc \{[^}]*border-color:\s*var\(--primary\)[^}]*animation:\s*none/, 'green and calm once connected');
  assert.match(css, /@keyframes sc-ask[^\n]*box-shadow: 0 0 0 0 transparent[^\n]*box-shadow: 0 0 30px 9px/, 'glows in and out');
  assert.match(css, /prefers-reduced-motion: reduce\) \{ \.sc-go:not\(\.is-on\) \.sc-go-disc \{ animation: sc-ask[^}]*!important/, 'the glow also plays when base.css turns animations off');
  const js = readFileSync('src/assets/js/scatter.js', 'utf8');
  assert.match(js, /go\.addEventListener\('click', function \(\) \{ animateTo\(1, DURATION\); \}\)/);
  assert.match(js, /replay\.addEventListener\('click', function \(\) \{ animateTo\(0, DURATION\); \}\)/);
  assert.match(js, /replay\.hidden = p < 0\.9/);
  assert.match(js, /go\.classList\.toggle\('is-on', ring\)/, 'the robot only changes colour');
  assert.doesNotMatch(js, /go\.style\.(display|opacity|left|top|transform)/, 'the robot never moves, hides, or fades');
  assert.match(js, /'Legacy Changelog'/);
  assert.match(js, /DURATION = 300/, 'every change takes 300 ms');
  assert.doesNotMatch(js, /IntersectionObserver|WAIT|setTimeout|addEventListener\('input'/, 'no timer and no slider: the button drives the change');
  assert.match(js, /Fade, not fly/);
  assert.match(js, /style\.opacity = fade/, 'cards fade between the two layouts');
  assert.doesNotMatch(js, /prefers-reduced-motion|\breduce\b/, 'the fade plays even with reduced motion');
  assert.match(js, /\(i \+ 1\) % ringPts\.length/, 'the last item links back to the first, closing the loop');
});

test('landing: no card of the legacy pane covers the Techlog robot in the centre', () => {
  const rule = (src, n) => new RegExp(`\\.sc-card:nth-child\\(${n}\\) \\{[^}]*\\}`).exec(src)?.[0] ?? '';
  const [base, media] = css.split('@media (max-width: 700px)');
  for (const phone of [false, true]) {
    const robot = phone ? { halfW: 7, halfH: 6 } : { halfW: 4, halfH: 17 }; // the robot with its caption, in % of the stage
    const width = phone ? { card: 38, agent: 20 } : { card: 19, agent: 15 };
    const halfH = phone ? 12 : 10;
    for (let n = 1; n <= 10; n++) {
      const src = (phone && /--sx/.test(rule(media, n))) ? rule(media, n) : rule(base, n);
      const sx = Number(/--sx:\s*([\d.]+)/.exec(src)[1]);
      const sy = Number(/--sy:\s*([\d.]+)/.exec(src)[1]);
      const halfW = (n > 8 ? width.agent : width.card) / 2;
      assert.ok(!(Math.abs(sx - 50) < halfW + robot.halfW && Math.abs(sy - 50) < halfH + robot.halfH), `${phone ? 'phone' : 'desktop'}: card ${n} covers the robot`);
    }
  }
});

test('landing: layout does not jump between legacy and Techlog', () => {
  assert.match(css, /\.sc-stage \{[^}]*height:\s*18\.7rem/, 'fixed stage height');
  assert.match(css, /@media \(max-width: 700px\) \{[\s\S]*?\.sc-stage \{ height: 25rem; \}/, 'fixed stage height on a phone');
  assert.match(css, /\.sc-head \{[^}]*min-height:\s*2\.75rem/, 'the top bar keeps its height with or without Replay');
  assert.doesNotMatch(css.match(/\.sc-head \{[^}]*\}/)[0], /flex-wrap:\s*wrap/, 'the top bar never wraps');
  assert.match(css, /\.sc-caption \{[^}]*white-space:\s*nowrap/);
});

test('landing: in the after state Techlog is connected to the two agents only, not to the steps', () => {
  const js = readFileSync('src/assets/js/scatter.js', 'utf8');
  assert.doesNotMatch(js, /hubspoke|hubSpoke/i);
  assert.doesNotMatch(css, /hubspoke/i);
  assert.match(js, /agents\.forEach\(function \(ag, i\) \{\s*var a = loopPts\[cards\.length \+ i\];/, 'a link per agent');
  assert.doesNotMatch(js, /centre\[0\]\)[^\n]*\n[^\n]*cards\.forEach|ringPts\.forEach[\s\S]{0,900}centre\[/, 'no link from the centre to a step');
});

test('landing: before, Claude opens a PR and Codex writes the postmortem', () => {
  const sc = html.slice(html.indexOf('id="scatter"'), html.indexOf('assets/js/scatter.js'));
  assert.match(sc, /sc-agent sc-a1" data-before="Opens PR #482"[^>]*>[\s\S]*?<strong>Claude<\/strong><span class="sc-sub">Opens PR #482</);
  assert.match(sc, /sc-agent sc-a2" data-before="Writes the postmortem"[^>]*>[\s\S]*?<strong>Codex<\/strong><span class="sc-sub">Writes the postmortem</);
  assert.match(sc, /data-after="Steps 1, 2, 7, 8"/);
  assert.match(sc, /data-after="Steps 3, 4, 5, 6"/);
  assert.doesNotMatch(html, /Control plane/i, 'the agent blocks do not say Control plane');
  const js = readFileSync('src/assets/js/scatter.js', 'utf8');
  assert.match(js, /\{ agent: 0, step: 2, label: 'opens PR' \}/);
  assert.match(js, /\{ agent: 1, step: 6, label: 'writes postmortem' \}/);
});

// positions of the steps and agents, read from the CSS: the base layout and the phone layout
function layout(phone) {
  const pos = {};
  const grab = (src) => { for (const m of src.matchAll(/\.sc-card:nth-child\((\d+)\) \{([^}]*)\}/g)) { const cx = /--cx:\s*([\d.]+)/.exec(m[2]); const cy = /--cy:\s*([\d.]+)/.exec(m[2]); if (cx && cy) pos[m[1]] = [Number(cx[1]), Number(cy[1])]; } };
  const [base, media] = css.split('@media (max-width: 700px)');
  grab(base);
  if (phone) grab(media);
  return pos;
}
const cross = (a, b, c, d) => {
  const o = (p, q, r) => Math.sign((q[0] - p[0]) * (r[1] - p[1]) - (q[1] - p[1]) * (r[0] - p[0]));
  return o(a, b, c) !== o(a, b, d) && o(c, d, a) !== o(c, d, b);
};

test('landing: after, two agents in the centre each control their own steps, with no shared step and no crossing links', () => {
  const OWNER = [0, 0, 1, 1, 1, 1, 0, 0];
  const js = readFileSync('src/assets/js/scatter.js', 'utf8');
  assert.match(js, /OWNER = \[0, 0, 1, 1, 1, 1, 0, 0\]/);
  for (const phone of [false, true]) {
    const pos = layout(phone);
    const links = OWNER.map((o, i) => ({ agent: o, from: pos[9 + o], to: pos[1 + i] }));
    assert.deepEqual([0, 1].map((a) => links.filter((l) => l.agent === a).length), [4, 4], 'four steps each; no step is shared');
    for (let i = 0; i < links.length; i++) for (let j = i + 1; j < links.length; j++) {
      if (links[i].agent === links[j].agent) continue; // the same agent may fan out from one point
      assert.ok(!cross(links[i].from, links[i].to, links[j].from, links[j].to), `${phone ? 'phone' : 'desktop'}: link ${i + 1} crosses link ${j + 1}`);
    }
    const left = pos[9][0] < pos[10][0] || pos[9][1] < pos[10][1];
    assert.ok(left, 'Claude is left of, or above, Codex');
  }
  assert.match(js, /sc-spoke-' \+ OWNER\[i\]/);
  assert.match(css, /\.sc-spoke-1 \{ stroke: var\(--info\)/);
  assert.match(css, /\.sc-card:nth-child\(9\) \{[^}]*--cx:\s*33;\s*--cy:\s*58;/);
  assert.match(css, /\.sc-card:nth-child\(10\) \{[^}]*--cx:\s*67;\s*--cy:\s*42;/);
});

test('landing: both agents connect to the robot in the centre, and the robot does not move between the states', () => {
  const js = readFileSync('src/assets/js/scatter.js', 'utf8');
  assert.match(js, /sc-hublink sc-hublink-/);
  assert.match(js, /hubLinks\[i\]\.setAttribute\('x2', centre\[0\]\)/);
  assert.match(css, /\.sc-hublink-1 \{ stroke: var\(--info\)/);
  for (const phone of [false, true]) {
    const pos = layout(phone);
    assert.ok(Math.abs(pos[9][0] - 50) + Math.abs(pos[10][0] - 50) + Math.abs(pos[9][1] - 50) + Math.abs(pos[10][1] - 50) > 0, 'the agents are beside the robot, not on it');
  }
  assert.doesNotMatch(css.match(/\.sc-go \{[^}]*\}/)[0], /left:\s*0/, 'not in a corner any more');
});

test('landing: the after pane has a light green background that fades in and out with the change', () => {
  assert.match(css, /\.scatter \{[^}]*--tint:\s*0;[^}]*background:\s*color-mix\(in srgb, var\(--primary\) calc\(var\(--tint\) \* 9%\), var\(--bg-1\)\)/, 'light green: 9% of the primary colour');
  const js = readFileSync('src/assets/js/scatter.js', 'utf8');
  assert.match(js, /root\.style\.setProperty\('--tint', smooth\(0\.5, 1, p\)/, 'driven by the same progress as the fade, so it also plays with reduced motion');
});

test('lifecycle: the chain has no text after the diagram, and every block has a question mark with a tooltip', () => {
  const lc = readFileSync('.test-out/landing/docs/lifecycle.html', 'utf8');
  const chain = lc.slice(lc.indexOf('id="chain"'), lc.indexOf('id="stages"'));
  const afterDiagram = chain.slice(chain.indexOf('</section>\n</div>') + '</section>\n</div>'.length);
  assert.doesNotMatch(afterDiagram, /<p>/, 'no paragraph after the diagram');
  assert.doesNotMatch(chain, /The incident links to the release it was observed after/);
  assert.doesNotMatch(lc, /Where a corrective action re-enters|Corrective action outcome/, 'the outcomes table is gone');
  assert.doesNotMatch(chain, /<table|<caption/, 'nothing but the diagram in the chain');
  const steps = [...chain.matchAll(/<li>([A-Za-z ]+)<button type="button" class="tip" aria-label="About ([A-Za-z ]+)" aria-describedby="(tip-[a-z-]+)">\?<\/button><span class="tip-text" role="tooltip" id="\3"><code>([a-z.]+)<\/code>([^<]+)<\/span><\/li>/g)];
  assert.deepEqual(steps.map((m) => m[1]), ['Problem', 'Proposal', 'Decision', 'Implementation', 'Verification', 'Release', 'Incident', 'Mitigation', 'Rollback', 'Postmortem', 'Corrective action']);
  for (const m of steps) { assert.equal(m[1], m[2]); assert.ok(m[5].length > 20 && m[5].length < 170, `${m[1]}: a short explanation`); }
  const components = readFileSync('src/assets/css/components.css', 'utf8');
  assert.match(components, /\.tip:hover \+ \.tip-text, \.tip:focus-visible \+ \.tip-text \{[^}]*visibility:\s*visible/, 'shown on hover and on keyboard focus');
  assert.match(components, /\.tip-text \{[^}]*visibility:\s*hidden/);
});

test('landing: no status labels between the lead and the demo', () => {
  assert.doesNotMatch(html, /trust-strip/);
  const hero = html.slice(html.indexOf('class="hero-row"'), html.indexOf('id="scatter"'));
  assert.doesNotMatch(hero, /badge/, 'no labels between the lead and the demo');
});

test('welcome: the heading breaks after "Welcome."', () => {
  assert.match(welcome, /<h1[^>]*>Welcome\.<br> We're glad you're here\.<\/h1>/);
});

test('every "Try Techlog" link on every page leads to the same URL, without a fragment', () => {
  const pages = ['index.html', 'welcome.html', 'docs/index.html', 'docs/lifecycle.html', 'docs/getting-started.html', 'docs/mcp.html'];
  const urls = new Set();
  let count = 0;
  for (const page of pages) {
    const src = readFileSync(`.test-out/landing/${page}`, 'utf8');
    for (const m of src.matchAll(/<a [^>]*href="([^"]*)"[^>]*>(?:<span[^>]*>[^<]*<\/span>)?\s*Try Techlog<\/a>/g)) {
      count++;
      const depth = page.split('/').length - 1;
      urls.add(m[1].replace(/^(\.\.\/)+/, ''));
      assert.ok(!m[1].includes('#'), `${page}: ${m[1]} has a fragment`);
      assert.equal(m[1], '../'.repeat(depth) + 'welcome.html', `${page}: ${m[1]}`);
    }
  }
  assert.ok(count >= 6, `found ${count} Try Techlog links (the header on every page)`);
  assert.deepEqual([...urls], ['welcome.html']);
});

const chain = JSON.parse(readFileSync('src/examples/demo-chain.json', 'utf8'));

test('renderDemoEvents: one closed row per event, with the minimum details behind a click', () => {
  const out = renderDemoEvents(chain);
  assert.equal((out.match(/class="cmp-event"/g) || []).length, 3);
  assert.deepEqual([...out.matchAll(/data-type="([^"]+)"/g)].map((m) => m[1]), chain.map((e) => e.type), 'rows follow the data order');
  assert.equal((out.match(/<details/g) || []).length, 3);
  assert.doesNotMatch(out, /<details[^>]*\bopen\b/, 'every row starts closed');
  assert.deepEqual([...out.matchAll(/class="cmp-edate">([^<]*)</g)].map((m) => m[1]), ['Mar 9', 'Mar 10', 'Mar 10']);
  for (const e of chain) {
    assert.ok(out.includes(e.extensions['org.techlog.demo'].headline), `${e.type} headline is in its row`);
    assert.ok(out.includes(e.summary.replace(/&/g, '&amp;')) || out.includes(e.summary), `${e.type} summary is in its details`);
    assert.ok(out.includes(`${e.sources[0].kind}: ${e.sources[0].ref}`), `${e.type} source is shown`);
  }
  const words = out.replace(/<[^>]+>/g, ' ').replace(/\s+/g, ' ');
  assert.match(words, /relates_to → problem\.reported confirmed/, 'the decision names the event it relates to, by type');
  assert.match(words, /implements → decision\.made confirmed/);
  assert.equal((out.match(/class="cmp-elink"/g) || []).length, 2, 'the problem has no link');
  assert.doesNotMatch(out, /<pre|Show JSON|rec-json|evt_/, 'no JSON and no ids');
});

test('renderDemoEvents: escapes event text and copes with an event that has no sources', () => {
  const evil = { ...chain[0], summary: '<b>x</b> & "y"', extensions: { 'org.techlog.demo': { headline: '<i>h</i>' } } };
  delete evil.sources;
  const out = renderDemoEvents([evil]);
  assert.doesNotMatch(out, /<b>x|<i>h/);
  assert.match(out, /&lt;b&gt;x&lt;\/b&gt; &amp; &quot;y&quot;/);
  assert.match(out, /&lt;i&gt;h&lt;\/i&gt;/);
  assert.doesNotMatch(out, /cmp-esrc/);
  assert.match(out, /class="cmp-event"/);
});

test('the {{demo-events}} directive is expanded in the built landing page', () => {
  assert.doesNotMatch(html, /\{\{demo-(events|cards)/);
  assert.match(html, /class="cmp-event"/);
});

test('shell.js: sessions accumulate, cards accumulate, 80 seconds, reduced motion, no autoplay', () => {
  const js = readFileSync('src/assets/js/shell.js', 'utf8');
  assert.match(js, /TARGET_MS = 80000/);
    assert.match(js, /data-rec/, 'cards are tied to the record line of their session');
  assert.doesNotMatch(js, /matchMedia|prefers-reduced-motion/, 'typing is the point of the demo, so it is never skipped');
  assert.doesNotMatch(js, /IntersectionObserver|autoplay/);
});

test('the record cards stay inside their pane: the list column may shrink below the JSON width', () => {
  assert.match(css, /\.rec-list \{[^}]*grid-template-columns: minmax\(0, 1fr\)/);
});

test('review fixes: the record list follows the newest card, a card lands before the wipe, and the dead controls stay hidden without JS', () => {
  const js = readFileSync('src/assets/js/shell.js', 'utf8');
  assert.match(js, /CARD_DWELL_MS/, 'the session stays visible for a beat after its card lands');
  assert.match(css, /\.sh-ctl\[hidden\] \{ display: none; \}/, 'the control bar honours hidden when JavaScript never unhides it');
});

test('the hiding rules of the demo only apply once JavaScript has added .sh-js', () => {
  const hiding = css.split('\n').filter((l) => /display: none|visibility: hidden/.test(l) && /\.(rec-card|sh-step|sh-l)\b/.test(l));
  assert.ok(hiding.length >= 3, 'the three hiding rules exist (two with display: none, the cards with visibility)');
  for (const l of hiding) assert.match(l, /^\.sh-js /, `without .sh-js the demo would hide in a no-JS browser: ${l}`);
});

test('typing: the command line types, and so do the session dividers and every output line', () => {
  const js = readFileSync('src/assets/js/shell.js', 'utf8');
  assert.match(js, /OUT_TYPE_MS/, 'output lines are typed');
  assert.match(js, /DIVIDER_TYPE_MS/, 'a new-session divider is typed more slowly so it is noticed');
  assert.match(js, /sh-typing/, 'the line being typed carries a caret');
  assert.match(css, /\.sh-typing::after \{/, 'the caret is styled');
  assert.match(css, /@media \(prefers-reduced-motion: reduce\)[^\n]*\.sh-typing::after/, 'only the caret blink stops under reduced motion, not the typing');
});

test('each demo event type has its own colour: a coloured border and a badge for the event name', () => {
  const out = renderDemoCards(chain);
  assert.deepEqual([...out.matchAll(/class="rec-card" id="rec-\d" data-rec="\d" data-type="([^"]+)"/g)].map((m) => m[1]), chain.map((e) => e.type));
  const colour = {};
  for (const m of css.matchAll(/\.rec-card\[data-type="([^"]+)"\] \{ --ev: var\((--[a-z]+)\); \}/g)) colour[m[1]] = m[2];
  assert.deepEqual(Object.keys(colour).sort(), chain.map((e) => e.type).sort(), 'every demo event type is coloured');
  assert.equal(new Set(Object.values(colour)).size, 6, 'six different colours');
  assert.deepEqual(colour, { 'problem.reported': '--warn', 'proposal.recorded': '--accent', 'decision.made': '--info', 'change.implemented': '--ok', 'incident.opened': '--danger', 'mitigation.applied': '--orange' });
  assert.equal(colour['change.implemented'], '--ok', 'a change is green');
  assert.ok(!Object.values(colour).includes('--hyp'), 'purple stays reserved for the hypothesis link');
  assert.match(css, /\.rec-card \{[^}]*border: 1px solid var\(--ev, var\(--line-strong\)\)/, 'the card border takes the event colour');
  assert.match(css, /\.rec-type \{[^}]*background: color-mix\(in srgb, var\(--ev[^}]*color: var\(--ev/, 'the event name is a coloured badge');
  assert.match(css, /\.rec-type \{[^}]*var\(--ev, var\(--info\)\) 6%, var\(--bg-1\)/, 'a 6% tint keeps badge text at 4.5:1 or better in both themes');
});

test('no trace animation: the cards are never highlighted as connected', () => {
  const js = readFileSync('src/assets/js/shell.js', 'utf8');
  assert.doesNotMatch(js, /data-trace|rec-trace|focus|getBoundingClientRect/);
  assert.doesNotMatch(css, /rec-trace/);
});

test('the agent pane never erases a session: every reached session stays, in order, and the pane scrolls', () => {
  const js = readFileSync('src/assets/js/shell.js', 'utf8');
  assert.doesNotMatch(js, /sh-cur|sh-view|pick|tablist|role', 'tab'/, 'no tabs and no one-session-at-a-time view');
  assert.match(css, /\.sh-js \.sh-step:not\(\.sh-on\) \{ display: none; \}/, 'a session shows once it is reached and then stays');
  assert.doesNotMatch(css, /sh-tab|sh-view|sh-cur/);
  assert.match(js, /body\.scrollTop = body\.scrollHeight/, 'the pane follows the newest line, older sessions stay above it');
});

test('the records pane has no scroll: all six cards fit, their space is reserved, and nothing in it toggles open', () => {
  const js = readFileSync('src/assets/js/shell.js', 'utf8');
  assert.doesNotMatch(js, /list\.scrollTop|rec-list/, 'the script never scrolls the records');
  const listRule = css.match(/\n\.rec-list \{[^}]*\}/)[0];
  assert.doesNotMatch(listRule, /overflow|height/, 'no fixed height and no scrolling on the records list');
  assert.doesNotMatch(css.match(/@media \(max-width: 700px\)[^\n]*/)[0], /\.rec-list \{[^}]*(max-)?height/, 'nor on a phone');
  assert.match(css, /\.sh-js \.rec-card:not\(\.sh-on\) \{ visibility: hidden; \}/, 'an unreached card still takes its place, so the pane never grows or jumps');
  assert.doesNotMatch(css, /rec-json|rec-hl/);
  assert.match(css, /\.sh-pane-agent \.sh-body \{[^}]*flex: 1 1 0/, 'the left pane takes the height of the right one');
});

test('copy: the caption names the corrective action, not a fix, and the problem is confirmed by the user before it is recorded', () => {
  assert.match(how, /the corrective action lands in the same chain/);
  assert.doesNotMatch(how, /the fix lands/);
  const problemStep = how.split('class="sh-step"')[1];
  assert.ok(problemStep.indexOf('sh-typed">yes<') > 0 && problemStep.indexOf('sh-typed">yes<') < problemStep.indexOf('data-rec="1"'), 'a typed yes comes before the record line');
});
