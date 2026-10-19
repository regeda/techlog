import test from 'node:test';
import assert from 'node:assert/strict';
import { readFileSync, mkdirSync, readdirSync, existsSync } from 'node:fs';
import { build } from '../build.mjs';

mkdirSync('.test-out', { recursive: true });
build({ srcDir: 'src', outDir: '.test-out/crosslinks' });
const out = (p) => readFileSync(`.test-out/crosslinks/${p}`, 'utf8');

test('http api: a page under Integrate with the routes and the error shape', () => {
  const page = out('docs/http-api.html');
  for (const s of ['POST /api/v1/events', 'GET /api/v1/systems/{system}/history', 'POST /api/v1/links', 'invalid_event']) {
    assert.ok(page.includes(s), `http-api.html mentions ${s}`);
  }
  const side = out('docs/index.html');
  assert.ok(side.indexOf('>Claude plugin<') > 0 && side.indexOf('>HTTP API<') > side.indexOf('>Claude plugin<'), 'HTTP API follows Claude plugin in the sidebar');
  assert.match(out('docs/claude-plugin.html'), /<nav class="pager"[^>]*>.*href="http-api\.html"/s);
  assert.match(out('docs/policy.html'), /<nav class="pager"[^>]*><a href="http-api\.html">/);
});

// plain turns highlighted markup back into the text a reader would copy.
function plain(html) {
  return html.replace(/<[^>]+>/g, '').replace(/&quot;/g, '"').replace(/&lt;/g, '<').replace(/&gt;/g, '>').replace(/&amp;/g, '&');
}
const builtPages = () => readdirSync('.test-out/crosslinks', { recursive: true }).filter((f) => f.endsWith('.html'));

const INSTALL = ['curl -sSfL https://raw.githubusercontent.com/regeda/techlog/main/install.sh | sh'];
const DEMO = ['techlog seed', 'techlog serve'];
const commandsOf = (f) => readFileSync(`src/examples/getting-started/${f}`, 'utf8')
  .split('\n').map((l) => l.replace(/\s+#.*$/, '').trim()).filter(Boolean);
const installLines = () => [...commandsOf('install-binary.sh'), ...commandsOf('demo-server.sh')];

test('install and demo examples: the commands of the README, no Go or make', () => {
  assert.deepEqual(commandsOf('install-binary.sh'), INSTALL);
  assert.deepEqual(commandsOf('demo-server.sh'), DEMO);
});

test('getting started: an Install section, then a Demo section with the UI, and no checkout steps', () => {
  const gs = out('docs/getting-started.html');
  for (const f of ['install-binary.sh', 'demo-server.sh']) {
    const src = readFileSync(`src/examples/getting-started/${f}`, 'utf8').trim();
    assert.ok(plain(gs).includes(src), `${f} is embedded verbatim`);
  }
  const install = gs.indexOf('<h2 id="install">Install</h2>');
  const demo = gs.indexOf('<h2 id="demo">Demo</h2>');
  assert.ok(install > 0 && demo > install, 'Install comes before Demo');
  assert.ok(gs.indexOf('install-binary') < 0 && plain(gs).indexOf('techlog seed') > plain(gs).indexOf('install.sh | sh'));
  assert.ok(!gs.includes('id="start"') && !gs.includes('Start the server'), 'the old section is gone');
  assert.doesNotMatch(plain(gs), /Go 1\.27|from the root of this repository|make run|make seed/);
  const ui = gs.slice(demo);
  for (const v of ['Events', 'Event', 'System', 'Story']) assert.ok(ui.includes(`<strong>${v}</strong>`), `UI view ${v} is in the Demo section`);
  assert.ok(ui.includes('/story/{id}'));
});

test('run the server: the page and every link to it are gone', () => {
  assert.equal(existsSync('src/pages/docs/server.html'), false);
  assert.equal(existsSync('src/examples/getting-started/run-server.sh'), false);
  for (const f of builtPages()) {
    assert.doesNotMatch(out(f), /server\.html/, `${f} links to server.html`);
  }
});

test('claude plugin page installs from the GitHub repo', () => {
  const text = plain(out('docs/claude-plugin.html'));
  assert.ok(text.includes('/plugin marketplace add regeda/techlog'));
  assert.ok(text.includes('/plugin install techlog@techlog'));
  assert.doesNotMatch(text, /\/path\/to|make plugin|from a checkout/);
});

test('every page links to the GitHub repo from the header and the footer', () => {
  const pages = builtPages();
  assert.ok(pages.length >= 13);
  for (const f of pages) {
    const html = out(f);
    for (const part of ['header', 'footer']) {
      const block = html.match(new RegExp(`<${part}[\\s\\S]*?</${part}>`))?.[0] ?? '';
      assert.match(block, /<a class="gh-link" href="https:\/\/github\.com\/regeda\/techlog" aria-label="Techlog on GitHub"><svg[^>]*aria-hidden="true"/, `${f}: ${part} GitHub link`);
    }
  }
});

const readme = () => readFileSync('../README.md', 'utf8');

test('the website install example matches the README', () => {
  const lines = new Set(readme().split('\n')
    .map((l) => l.replace(/\s+#.*$/, '').trim().replace(/^go run \.\/cmd\/techlog/, 'techlog')));
  for (const cmd of installLines()) assert.ok(lines.has(cmd), `README has "${cmd}"`);
});

test('the root README names the hosted site and installs the plugin from GitHub', () => {
  const r = readme();
  for (const s of ['https://techlog-e7k.pages.dev/', '/plugin marketplace add regeda/techlog']) {
    assert.ok(r.includes(s), `README mentions ${s}`);
  }
  assert.doesNotMatch(r, /techlog-e7k\.pages\.dev\/[^\s)]*\.html/, 'hosted links use clean URLs');
});

test('the website README no longer calls the samples illustrative', () => {
  const r = readFileSync('README.md', 'utf8');
  assert.doesNotMatch(r, /illustrative API|reserved, unresolvable/);
  assert.ok(r.includes('https://github.com/regeda/techlog'));
});

test('no page tells a website reader to use make or a local path', () => {
  for (const f of builtPages()) {
    assert.doesNotMatch(plain(out(f)), /make plugin|make run|make seed|\/path\/to/, `${f} mentions make or a local path`);
  }
});

test('search dropdown floats over the sidebar with its own background', () => {
  const css = readFileSync('src/assets/css/docs.css', 'utf8');
  const rule = (sel) => css.match(new RegExp(`(?:^|\\n)${sel.replace(/[.#]/g, '\\$&')} \\{([^}]*)\\}`))?.[1] ?? '';
  assert.match(rule('.doc-search'), /position:\s*relative/, 'the search box anchors the dropdown');
  const list = rule('#doc-search-results');
  assert.match(list, /position:\s*absolute/, 'the dropdown does not push the sidebar down');
  assert.match(list, /background:\s*var\(--bg-1\)/, 'the dropdown has a background');
  assert.match(list, /border:/);
  assert.match(list, /z-index:/);
  assert.match(css, /#doc-search-results:empty\s*\{[^}]*display:\s*none/, 'an empty dropdown draws nothing');
});

test('getting started has no "These samples run" callout', () => {
  assert.doesNotMatch(out('docs/getting-started.html'), /These samples run/);
});
