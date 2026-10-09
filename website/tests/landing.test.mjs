import test from 'node:test';
import assert from 'node:assert/strict';
import { readFileSync, mkdirSync } from 'node:fs';
import { build } from '../build.mjs';

mkdirSync('.test-out', { recursive: true });
build({ srcDir: 'src', outDir: '.test-out/landing' });
const html = readFileSync('.test-out/landing/index.html', 'utf8');
const how = html.slice(html.indexOf('id="how"'), html.indexOf('id="connectors"'));
const css = readFileSync('src/assets/css/landing.css', 'utf8');

test('landing: the MCP example is two panes, request and response', () => {
  const panes = how.match(/<figure class="pane">[\s\S]*?<\/figure>/g) ?? [];
  assert.equal(panes.length, 2, 'two panes');
  assert.match(panes[0], /<figcaption>Request<\/figcaption>/);
  assert.match(panes[1], /<figcaption>Response<\/figcaption>/);
  assert.match(panes[0], /tools\/call/);
  assert.doesNotMatch(panes[0], /structuredContent/);
  assert.match(panes[1], /structuredContent/);
  assert.doesNotMatch(panes[1], /tools\/call/);
  assert.equal((how.match(/class="codeblock"/g) ?? []).length, 2, 'no third, stacked JSON block');
});

test('landing: the panes sit side by side on wide screens and are height-capped', () => {
  assert.match(css, /@media \(min-width: 900px\)[^}]*\.panes[^}]*\{[^}]*grid-template-columns:\s*minmax\(0,\s*1fr\)\s+minmax\(0,\s*1fr\)/);
  const pre = css.match(/\.pane\s+pre\s*\{[^}]*\}/)?.[0] ?? '';
  assert.match(pre, /max-height:\s*[\d.]+(rem|vh|px)/, 'a height cap keeps the page short');
  assert.match(pre, /overflow:\s*auto/, 'the rest scrolls inside the pane');
});

test('landing: the example text does not call the shipped tool a proposal', () => {
  assert.doesNotMatch(how, /tools shown are a proposal/i);
});
