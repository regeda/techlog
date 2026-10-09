import test from 'node:test';
import assert from 'node:assert/strict';
import { readFileSync, mkdirSync } from 'node:fs';
import { build, highlightJson, highlightCode } from '../build.mjs';

mkdirSync('.test-out', { recursive: true });
build({ srcDir: 'src', outDir: '.test-out/landing' });
const html = readFileSync('.test-out/landing/index.html', 'utf8');
const how = html.slice(html.indexOf('id="how"'), html.indexOf('id="connectors"'));
const css = readFileSync('src/assets/css/landing.css', 'utf8');

test('landing: the MCP example is a single abridged request, no response', () => {
  const panes = how.match(/<figure class="pane">[\s\S]*?<\/figure>/g) ?? [];
  assert.equal(panes.length, 1, 'one pane');
  assert.match(panes[0], /<figcaption>Request, abridged<\/figcaption>/);
  assert.match(panes[0], /tools\/call/);
  assert.doesNotMatch(how, /structuredContent|<figcaption>Response/);
  assert.equal((how.match(/class="codeblock"/g) ?? []).length, 1);
});

test('landing: the abridged request is a subset of the full example and stays short', () => {
  const full = JSON.parse(readFileSync('src/examples/mcp-record-event.json', 'utf8')).request;
  const brief = JSON.parse(readFileSync('src/examples/mcp-request-brief.json', 'utf8'));
  const subset = (a, b) =>
    Array.isArray(a) ? a.every((x, i) => subset(x, b[i]))
      : a && typeof a === 'object' ? Object.entries(a).every(([k, v]) => b && k in b && subset(v, b[k]))
        : typeof a === 'string' ? b.startsWith(a) : a === b; // a string may be cut short
  assert.ok(subset(brief, full), 'every field of the brief matches the full request');
  const pre = how.match(/<pre><code[^>]*>([\s\S]*?)<\/code><\/pre>/)[1].replace(/<[^>]+>/g, '').replace(/&quot;/g, '"');
  const lines = pre.split('\n');
  assert.ok(lines.length <= 25, `at most 25 lines, got ${lines.length}`);
  assert.ok(Math.max(...lines.map((l) => l.length)) <= 90, 'fits the page width without scrolling');
});

test('landing: the JSON is highlighted with design-token colours', () => {
  for (const c of ['tk-key', 'tk-str', 'tk-num']) assert.match(how, new RegExp(`class="${c}"`));
  const components = readFileSync('src/assets/css/components.css', 'utf8');
  assert.match(components, /\.tk-key\s*\{\s*color:\s*var\(--info\)/);
  assert.match(css, /\.pane pre\s*\{[^}]*max-height:\s*[\d.]+rem[^}]*overflow:\s*auto/);
});

test('highlightJson escapes markup and round-trips the text', () => {
  const src = '{ "a<b": ["x&y", 1.5e3, true, null] }';
  const out = highlightJson(src);
  assert.doesNotMatch(out, /<b|x&y/);
  assert.equal(out.replace(/<[^>]+>/g, '').replace(/&quot;/g, '"').replace(/&lt;/g, '<').replace(/&amp;/g, '&'), src);
});

test('landing: the example text does not call the shipped tool a proposal', () => {
  assert.doesNotMatch(how, /tools shown are a proposal/i);
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
