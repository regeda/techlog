import test from 'node:test';
import assert from 'node:assert/strict';
import { readFileSync } from 'node:fs';
import { validate } from './lib/validate-event.mjs';

const load = (f) => JSON.parse(readFileSync(`src/examples/${f}`, 'utf8'));
const schema = load('json-schema-core.json');
const timeline = load('timeline.json');
const story = load('story.json');
const idOf = (n) => `evt_01J9K2A0P${String(n).padStart(3, '0')}`;
const byId = Object.fromEntries(timeline.map((e) => [e.id, e]));

test('standalone example events validate against the schema', () => {
  for (const f of ['event-minimal.json', 'event-full.json', 'event-incident.json']) {
    assert.deepEqual(validate(schema, load(f)), [], f);
  }
});

test('validator rejects a missing required field and an unknown top-level key', () => {
  const { status, ...noStatus } = load('event-minimal.json');
  assert.ok(validate(schema, noStatus).some((e) => /status: required/.test(e)));
  assert.ok(validate(schema, { ...load('event-minimal.json'), surprise: 1 }).some((e) => /surprise: not allowed/.test(e)));
  assert.ok(validate(schema, { ...load('event-minimal.json'), extensions: { BadKey: 1 } }).some((e) => /invalid key/.test(e)));
});

test('validator enforces the idempotency key: required, non-empty, at most 128 characters', () => {
  const base = load('event-minimal.json');
  const { idempotency_key, ...without } = base;
  assert.ok(validate(schema, without).some((e) => /idempotency_key: required/.test(e)));
  assert.ok(validate(schema, { ...base, idempotency_key: '' }).some((e) => /shorter than 1/.test(e)));
  assert.ok(validate(schema, { ...base, idempotency_key: 'k'.repeat(129) }).some((e) => /longer than 128/.test(e)));
  assert.deepEqual(validate(schema, { ...base, idempotency_key: 'k'.repeat(128) }), []);
});

test('the timeline keys are unique per actor', () => {
  const seen = new Set();
  for (const e of timeline) {
    const k = `${e.actor.id}\u0000${e.idempotency_key}`;
    assert.ok(!seen.has(k), `${e.id} repeats a key for ${e.actor.id}`);
    seen.add(k);
  }
});

test('timeline: 25 sequential, chronological, schema-valid events linking only to earlier events', () => {
  assert.equal(timeline.length, 25);
  const seen = new Set();
  let prev = '';
  timeline.forEach((e, i) => {
    assert.deepEqual(validate(schema, e), [], e.id);
    assert.equal(e.id, idOf(i + 1));
    assert.ok(e.time >= prev, `${e.id} out of order`);
    prev = e.time;
    for (const l of e.links ?? []) assert.ok(seen.has(l.target), `${e.id} links to unknown/later ${l.target}`);
    seen.add(e.id);
  });
});

test('story: four tags; every event has one or two valid tags and several have two', () => {
  assert.deepEqual(story.tags.map((t) => t.id), ['engineering', 'production', 'marketing', 'support']);
  assert.deepEqual(Object.keys(story.tags_of).sort(), timeline.map((e) => e.id).sort());
  const valid = new Set(story.tags.map((t) => t.id));
  let multi = 0;
  for (const [id, tags] of Object.entries(story.tags_of)) {
    assert.ok(tags.length >= 1 && tags.length <= 2, `${id} has 1-2 tags`);
    assert.equal(new Set(tags).size, tags.length, `${id} has no repeated tag`);
    for (const t of tags) assert.ok(valid.has(t), `${id} -> ${t}`);
    if (tags.length === 2) multi++;
  }
  assert.ok(multi >= 6, `expected several two-tag tickets, got ${multi}`);
  for (const t of story.tags) assert.ok(Object.values(story.tags_of).some((x) => x.includes(t.id)), `${t.id} is used`);
});

test('story: one golden path, chronological, resolvable, touching all four tags', () => {
  const g = story.golden;
  assert.ok(g.title && g.blurb);
  assert.ok(g.steps.length >= 10 && g.steps.length < timeline.length, 'a curated subset of the events');
  assert.equal(new Set(g.steps).size, g.steps.length);
  for (const id of g.steps) assert.ok(byId[id], `${id} exists`);
  assert.deepEqual(g.steps, [...g.steps].sort(), 'steps are in chronological (id) order');
  const tags = new Set(g.steps.flatMap((id) => story.tags_of[id]));
  assert.deepEqual([...tags].sort(), ['engineering', 'marketing', 'production', 'support']);
  assert.equal(story.paths, undefined, 'the three-path model is gone');
  assert.equal(story.tab_of, undefined, 'tabs are replaced by tags');
});

test('story: one connected graph with many links across different tags', () => {
  const adj = Object.fromEntries(timeline.map((e) => [e.id, new Set()]));
  let crossTag = 0;
  for (const e of timeline) {
    for (const l of e.links ?? []) {
      adj[e.id].add(l.target);
      adj[l.target].add(e.id);
      const a = story.tags_of[e.id];
      const b = story.tags_of[l.target];
      if (!a.some((t) => b.includes(t))) crossTag++;
    }
  }
  const seen = new Set();
  const stack = [timeline[0].id];
  while (stack.length) {
    const id = stack.pop();
    if (seen.has(id)) continue;
    seen.add(id);
    stack.push(...adj[id]);
  }
  assert.equal(seen.size, timeline.length, 'every event is reachable from the first');
  assert.ok(crossTag >= 8, `expected many cross-tag links, got ${crossTag}`);
});

test('hypotheses are only causal and are each later confirmed by a different event', () => {
  const hyps = [];
  timeline.forEach((e, i) => (e.links ?? []).forEach((l) => l.basis === 'hypothesis' && hyps.push({ from: e, i, l })));
  assert.ok(hyps.length >= 2, 'at least two hypotheses in the story');
  for (const { from, i, l } of hyps) {
    assert.equal(l.rel, 'caused_by');
    const confirmedLater = timeline.slice(i + 1).some((e) =>
      e.id !== from.id && (e.links ?? []).some((x) => x.rel === 'caused_by' && x.target === l.target && x.basis === 'confirmed'));
    assert.ok(confirmedLater, `${from.id} -> ${l.target} is never confirmed`);
  }
  assert.ok(timeline.some((e) => (e.links ?? []).some((l) => l.basis === 'confirmed')));
});

test('full example is its timeline event plus extensions; incident example is its timeline event', () => {
  const { extensions, ...full } = load('event-full.json');
  assert.deepEqual(full, byId[idOf(6)]);
  assert.ok(extensions['org.techlog.code']);
  assert.deepEqual(load('event-incident.json'), byId[idOf(11)]);
  assert.ok(byId[idOf(11)].extensions['org.techlog.hypothesis'], 'the incident carries its hypothesis details');
});

test('policy example separates evaluation from enforcement', () => {
  const p = load('policy.json');
  assert.ok(p.requires.evidence.length && p.requires.approvals.length);
  assert.ok(['block', 'require_approval', 'warn'].includes(p.on_unmet));
  assert.ok(['collect_only', 'evaluate_only', 'enforce_via_integration'].includes(p.enforcement.mode));
  assert.equal(p.trigger.system, 'promo-api');
});

test('mcp tools: the five implemented tools with object input schemas', () => {
  const tools = load('mcp-tools.json');
  assert.deepEqual(tools.map((t) => t.name), [
    'techlog_search_events', 'techlog_get_system_history', 'techlog_get_event',
    'techlog_record_event', 'techlog_link_events',
  ]);
  for (const t of tools) {
    assert.equal(t.inputSchema.type, 'object');
    assert.ok(t.description.length > 20);
  }
  for (const f of ['mcp-record-event.json', 'mcp-record-verification.json']) {
    const ex = load(f);
    assert.equal(ex.request.method, 'tools/call');
    assert.equal(ex.request.params.name, 'techlog_record_event');
    assert.equal(ex.response.id, ex.request.id);
    assert.equal(ex.response.result.isError, false);
    const { event } = ex.request.params.arguments;
    assert.equal(event.id, undefined, 'the caller never supplies an id');
    const stored = ex.response.result.structuredContent.event;
    assert.equal(ex.response.result.structuredContent.id, stored.id);
    assert.deepEqual(validate(schema, stored), [], stored.id);
  }
  const rec = load('mcp-record-event.json');
  assert.equal(rec.response.result.structuredContent.id, idOf(4));
  assert.equal(byId[idOf(4)].type, rec.request.params.arguments.event.type);
  const ver = load('mcp-record-verification.json');
  assert.equal(ver.response.result.structuredContent.id, idOf(7));
  assert.equal(byId[idOf(7)].type, ver.request.params.arguments.event.type);
  assert.ok(ver.request.params.arguments.event.evidence.length, 'the verification carries its evidence');
});

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
