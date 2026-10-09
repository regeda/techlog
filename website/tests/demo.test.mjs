import test from 'node:test';
import assert from 'node:assert/strict';
import { readFileSync } from 'node:fs';
import vm from 'node:vm';

const sandbox = { window: {} };
vm.runInNewContext(readFileSync('src/assets/js/demo-core.js', 'utf8'), sandbox);
const data = JSON.parse(readFileSync('src/examples/demo-steps.json', 'utf8'));
const timeline = JSON.parse(readFileSync('src/examples/timeline.json', 'utf8'));
const demo = () => sandbox.window.TechlogDemo.create(data.steps);

test('walkthrough is short enough to click through quickly', () => {
  assert.ok(data.steps.length >= 5 && data.steps.length <= 7, `${data.steps.length} steps`);
});

test('every step has a title, a note, and lines on both sides', () => {
  const tones = new Set(['text', 'dim', 'warn', 'ok', 'code']);
  for (const s of data.steps) {
    assert.ok(s.title && s.note, 'title and note');
    for (const side of [s.left, s.right]) {
      assert.ok(side.lines.length >= 1 && side.lines.length <= 6);
      for (const l of side.lines) assert.ok(tones.has(l.t) && l.x, `line ${JSON.stringify(l)}`);
    }
  }
});

test('event ids shown in the demo exist in the Spring Spin timeline', () => {
  const known = new Set(timeline.map((e) => e.id));
  const shown = JSON.stringify(data).match(/evt_01J9K2A0P\d{3}/g) || [];
  assert.ok(shown.length >= 8, 'the demo shows real events');
  for (const id of shown) assert.ok(known.has(id), `${id} exists`);
});

test('the story contrasts the review queue with Techlog', () => {
  const text = JSON.stringify(data).toLowerCase();
  assert.match(text, /waiting for review/);
  assert.match(text, /discussion is gone/);
  assert.match(text, /mcp|record_event|records/);
});

test('stepping starts at the first step and stops at both ends', () => {
  const d = demo();
  assert.equal(d.index(), 0);
  assert.equal(d.canPrev(), false);
  assert.equal(d.prev(), 0, 'prev at the start stays put');
  assert.equal(d.next(), 1);
  assert.equal(d.canPrev(), true);
  for (let i = 0; i < 20; i++) d.next();
  assert.equal(d.index(), d.count - 1);
  assert.equal(d.canNext(), false);
  assert.equal(d.isLast(), true);
  assert.equal(d.next(), d.count - 1, 'next at the end stays put');
});

test('go clamps to the valid range and step() returns the current step', () => {
  const d = demo();
  assert.equal(d.go(3), 3);
  assert.equal(d.step().title, data.steps[3].title);
  assert.equal(d.go(-4), 0);
  assert.equal(d.go(99), d.count - 1);
});
