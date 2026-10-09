import test from 'node:test';
import assert from 'node:assert/strict';
import { mkdirSync, writeFileSync, readFileSync, mkdtempSync, existsSync } from 'node:fs';
import { join } from 'node:path';
import { build, parseFrontMatter, formatJson } from '../build.mjs';

mkdirSync('.test-out', { recursive: true });

function fixture(files) {
  const root = mkdtempSync('.test-out/fx-');
  const src = join(root, 'src');
  const out = join(root, 'site');
  const base = {
    'layouts/page.html': '<!doctype html><html><head>{{> head}}</head><body>{{> header}}<main>{{content}}</main>{{> footer}}</body></html>',
    'layouts/docs.html': '<!doctype html><html><head>{{> head}}</head><body>{{> header}}{{sidebar}}<main>{{content}}</main>{{toc}}</body></html>',
    'partials/head.html': '<title>{{title}} — Techlog</title><meta name="description" content="{{description}}">',
    'partials/header.html': '<header><a href="{{root}}index.html">Home</a><a href="{{root}}docs/index.html">Docs</a></header>',
    'partials/footer.html': '<footer>f</footer>',
    'assets/css/x.css': 'body{}',
  };
  for (const [p, c] of Object.entries({ ...base, ...files })) {
    const f = join(src, p);
    mkdirSync(join(f, '..'), { recursive: true });
    writeFileSync(f, c);
  }
  return { src, out };
}
const read = (out, ...p) => readFileSync(join(out, ...p), 'utf8');

test('parseFrontMatter splits meta and body', () => {
  const { meta, body } = parseFrontMatter('---\ntitle: Hello\nlayout: docs\n---\n<p>x</p>');
  assert.deepEqual(meta, { title: 'Hello', layout: 'docs' });
  assert.equal(body, '<p>x</p>');
});

test('parseFrontMatter without front matter returns empty meta', () => {
  const { meta, body } = parseFrontMatter('<p>x</p>');
  assert.deepEqual(meta, {});
  assert.equal(body, '<p>x</p>');
});

test('renders pages into layouts, with root prefix and current-page marking', () => {
  const { src, out } = fixture({
    'pages/index.html': '---\ntitle: Home\n---\n<h1>Hi</h1>',
    'pages/docs/index.html': '---\ntitle: D\nlayout: docs\nsection: Start\norder: 1\n---\n<h2>One</h2>',
  });
  const r = build({ srcDir: src, outDir: out });
  assert.deepEqual(r.pages.sort(), ['docs/index.html', 'index.html']);
  const home = read(out, 'index.html');
  assert.match(home, /<title>Home — Techlog<\/title>/);
  assert.match(home, /<a href="index.html" aria-current="page">Home<\/a>/);
  const docs = read(out, 'docs', 'index.html');
  assert.match(docs, /href="\.\.\/index\.html"/);
  assert.match(docs, /<a href="\.\.\/docs\/index\.html" aria-current="page">Docs<\/a>/);
  assert.ok(existsSync(join(out, 'assets', 'css', 'x.css')));
});

test('unknown variable, missing partial and missing title fail loudly', () => {
  let f = fixture({ 'pages/index.html': '---\ntitle: T\n---\n{{nope}}' });
  assert.throws(() => build({ srcDir: f.src, outDir: f.out }), /unknown variable \{\{nope\}\}/);
  f = fixture({ 'layouts/page.html': '{{> ghost}}', 'pages/index.html': '---\ntitle: T\n---\nx' });
  assert.throws(() => build({ srcDir: f.src, outDir: f.out }), /missing partial "ghost"/);
  f = fixture({ 'pages/index.html': '<p>no front matter</p>' });
  assert.throws(() => build({ srcDir: f.src, outDir: f.out }), /"title" is required/);
});

test('{{code}} embeds an escaped, copyable block', () => {
  const { src, out } = fixture({
    'examples/a.json': '{"a":"<b>"}\n',
    'pages/index.html': '---\ntitle: T\n---\n{{code json a.json}}',
  });
  build({ srcDir: src, outDir: out });
  const html = read(out, 'index.html');
  assert.match(html, /class="language-json"/);
  assert.match(html, /&lt;b&gt;/);
  assert.match(html, /<button class="copy"/);
});

test('{{code lang file#path}} embeds only that part of a JSON example', () => {
  const doc = { request: { method: 'tools/call', params: { n: 1 } }, response: { ok: true } };
  const { src, out } = fixture({
    'examples/a.json': JSON.stringify(doc),
    'pages/index.html': '---\ntitle: T\n---\n[{{code json a.json#request}}][{{code json a.json#response}}][{{code json a.json#request.params}}]',
  });
  build({ srcDir: src, outDir: out });
  const [req, res, params] = read(out, 'index.html').match(/\[(.*?)\]/gs);
  assert.match(req, /&quot;method&quot;|"method"/);
  assert.doesNotMatch(req, /"ok"|&quot;ok&quot;/);
  assert.match(res, /ok/);
  assert.doesNotMatch(res, /method/);
  assert.match(params, /n/);
  assert.doesNotMatch(params, /method/);
});

test('{{code}} with a path that does not exist fails the build', () => {
  const { src, out } = fixture({
    'examples/a.json': '{"request":{}}',
    'pages/index.html': '---\ntitle: T\n---\n{{code json a.json#nope}}',
  });
  assert.throws(() => build({ srcDir: src, outDir: out }), /no "nope"/);
  const f = fixture({
    'examples/a.txt': 'plain',
    'pages/index.html': '---\ntitle: T\n---\n{{code json a.txt#x}}',
  });
  assert.throws(() => build({ srcDir: f.src, outDir: f.out }), /not valid JSON|Unexpected/);
});

test('formatJson keeps short values on one line and expands long ones', () => {
  assert.equal(formatJson({ a: 1, b: [1, 2] }), '{ "a": 1, "b": [1, 2] }');
  assert.equal(formatJson('x'), '"x"');
  const long = { description: 'x'.repeat(90) };
  assert.equal(formatJson(long), `{\n  "description": "${'x'.repeat(90)}"\n}`);
  assert.equal(formatJson({ a: { b: 1 }, list: [{ k: 1 }, { k: 2 }] }), '{ "a": { "b": 1 }, "list": [{ "k": 1 }, { "k": 2 }] }');
  assert.equal(formatJson({}), '{}');
  assert.equal(formatJson([]), '[]');
});

test('{{json-script}} embeds JSON safely and round-trips', () => {
  const data = [{ x: '</script>' }];
  const { src, out } = fixture({
    'examples/t.json': JSON.stringify(data),
    'pages/index.html': '---\ntitle: T\n---\n{{json-script tl t.json}}',
  });
  build({ srcDir: src, outDir: out });
  const html = read(out, 'index.html');
  const m = html.match(/<script type="application\/json" id="tl">(.*?)<\/script>/s);
  assert.ok(m, 'script tag present');
  assert.ok(!m[1].includes('</script>'));
  assert.deepEqual(JSON.parse(m[1]), data);
});

test('docs sidebar is grouped in fixed order and sorted by order', () => {
  const docs = (t, s, o) => `---\ntitle: ${t}\nlayout: docs\nsection: ${s}\norder: ${o}\n---\n<h2>H</h2>`;
  const { src, out } = fixture({
    'pages/docs/index.html': docs('Overview', 'Start', 1),
    'pages/docs/c.html': docs('Gamma', 'Concepts', 2),
    'pages/docs/b.html': docs('Beta', 'Concepts', 1),
    'pages/docs/s.html': docs('Second', 'Start', 2),
  });
  build({ srcDir: src, outDir: out });
  const html = read(out, 'docs', 'index.html');
  const pos = (s) => html.indexOf(`>${s}</a>`);
  assert.ok(pos('Overview') < pos('Second'));
  assert.ok(pos('Second') < pos('Beta'));
  assert.ok(pos('Beta') < pos('Gamma'));
  assert.match(html, /aria-controls="docs-nav-list"/);
});

test('headings get unique ids; toc lists h2; no h2 means no toc', () => {
  const { src, out } = fixture({
    'pages/docs/index.html':
      '---\ntitle: D\nlayout: docs\nsection: Start\norder: 1\n---\n<h2>Same</h2><h3>Sub</h3><h2>Same</h2>',
    'pages/docs/bare.html': '---\ntitle: B\nlayout: docs\nsection: Start\norder: 2\n---\n<p>no headings</p>',
  });
  build({ srcDir: src, outDir: out });
  const html = read(out, 'docs', 'index.html');
  assert.match(html, /<h2 id="same">Same<\/h2>/);
  assert.match(html, /<h2 id="same-2">Same<\/h2>/);
  assert.match(html, /<nav class="toc"/);
  assert.match(html, /href="#same-2"/);
  assert.doesNotMatch(read(out, 'docs', 'bare.html'), /class="toc"/);
});

test('search index covers docs pages only and is loadable without fetch', () => {
  const { src, out } = fixture({
    'pages/index.html': '---\ntitle: Landing Only\n---\n<p>x</p>',
    'pages/docs/index.html': '---\ntitle: Event Model\nlayout: docs\nsection: Start\norder: 1\n---\n<h2>Fields</h2><p>actor and subject</p>',
  });
  build({ srcDir: src, outDir: out });
  const js = read(out, 'assets', 'search-index.js');
  assert.ok(js.startsWith('window.TECHLOG_SEARCH='));
  assert.match(js, /Event Model/);
  assert.match(js, /actor and subject/);
  assert.doesNotMatch(js, /Landing Only/);
});
