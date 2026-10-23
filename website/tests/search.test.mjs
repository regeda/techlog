import test from 'node:test';
import assert from 'node:assert/strict';
import { readFileSync } from 'node:fs';
import vm from 'node:vm';

const sandbox = { window: {}, document: { addEventListener() {}, getElementById: () => null } };
vm.runInNewContext(readFileSync('src/assets/js/search.js', 'utf8'), sandbox);
// Arrays created inside the vm context have a different prototype; copy them into
// this realm so strict deep-equality compares values, not realms.
const search = (...args) => Array.from(sandbox.window.TechlogSearch.search(...args));

const index = [
  { url: 'docs/event-model.html', title: 'Event Model', headings: ['Required fields'], text: 'actor subject provenance' },
  { url: 'docs/mcp.html', title: 'MCP Integration', headings: ['Tools'], text: 'record an event with techlog_record_event' },
  { url: 'docs/security.html', title: 'Security', headings: ['Trust'], text: 'identity and permissions' },
];

test('ranks title matches above text matches', () => {
  const r = search(index, 'event', 8);
  assert.deepEqual(r.map((d) => d.url), ['docs/event-model.html', 'docs/mcp.html']);
});

test('requires every term to match', () => {
  assert.deepEqual(search(index, 'event identity').map((d) => d.url), []);
  assert.deepEqual(search(index, 'actor provenance').map((d) => d.url), ['docs/event-model.html']);
});

test('special, empty, whitespace and very long input never throw and return nothing', () => {
  for (const q of ['(', '[', '\\', '.*', '', '   ', '\u0000', 'x'.repeat(100000)]) {
    assert.deepEqual(search(index, q), [], `query ${JSON.stringify(q.slice(0, 10))}`);
  }
  assert.deepEqual(search([], 'event'), []);
  assert.deepEqual(search(undefined, 'event'), []);
});

test('respects the limit', () => {
  const many = Array.from({ length: 20 }, (_, i) => ({ url: `p${i}.html`, title: 'Event', headings: [], text: '' }));
  assert.equal(search(many, 'event', 5).length, 5);
});
