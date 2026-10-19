import test from 'node:test';
import assert from 'node:assert/strict';
import { mkdirSync, writeFileSync, mkdtempSync } from 'node:fs';
import { join } from 'node:path';
import { checkLinks } from '../tools/check-links.mjs';
import { auditContent } from '../tools/audit-content.mjs';

mkdirSync('.test-out', { recursive: true });

function site(files) {
  const dir = mkdtempSync('.test-out/site-');
  for (const [p, c] of Object.entries(files)) {
    const f = join(dir, p);
    mkdirSync(join(f, '..'), { recursive: true });
    writeFileSync(f, c);
  }
  return dir;
}

test('checkLinks accepts good links and reports each kind of bad link', () => {
  const dir = site({
    'index.html':
      '<a href="docs/a.html#x">ok</a><a href="docs/a.html#missing">b1</a><a href="nope.html">b2</a>' +
      '<a href="#">b3</a><a href="https://example.com">b4</a><a href="/abs.html">b5</a>' +
      '<script src="https://cdn.jsdelivr.net/npm/mermaid@11/dist/mermaid.min.js"></script>' +
      '<a href="#top" id="top">self</a><img src="assets/i.svg" alt="">' +
      '<script type="module" src="assets/m.js"></script>',
    'docs/a.html': '<h2 id="x">x</h2><a href="../index.html">home</a>',
    'assets/i.svg': '<svg/>',
    'assets/m.js': '',
  });
  const problems = checkLinks(dir);
  const has = (re) => problems.some((p) => re.test(p));
  assert.ok(has(/missing fragment.*#missing/), 'missing fragment');
  assert.ok(has(/broken link.*nope\.html/), 'broken link');
  assert.ok(has(/empty or "#" link/), 'hash link');
  assert.ok(has(/external link not allowed.*example\.com/), 'external');
  assert.ok(has(/root-absolute.*\/abs\.html/), 'root absolute');
  assert.ok(has(/type="module"/), 'module script');
  assert.ok(!has(/docs\/a\.html#x/), 'good fragment link is fine');
  assert.ok(!has(/cdn\.jsdelivr\.net/), 'cdn allowed');
  assert.ok(!has(/assets\/i\.svg/), 'good asset');
  assert.equal(problems.length, 6, problems.join('\n'));
});

test('auditContent flags forbidden claims unless marked audit-ok', () => {
  const dir = site({
    'a.html':
      '<p>Records are tamper-proof.</p>\n' +
      '<p>We are SOC 2 certified.</p>\n' +
      '<p>Trusted by many customers.</p>\n' +
      '<p>Revolutionizing development.</p>\n' +
      '<p>Techlog does not claim records are tamper-proof.</p><!--audit-ok-->\n' +
      '<p>Plain safe sentence about events.</p>\n',
  });
  const problems = auditContent(dir);
  assert.equal(problems.length, 4, problems.join('\n'));
  assert.ok(problems.every((p) => p.includes('a.html')));
});

test('checkLinks allows the project repo on GitHub and nothing else on that host', () => {
  const dir = site({
    'index.html':
      '<a href="https://github.com/regeda/techlog">repo</a>' +
      '<a href="https://github.com/regeda/techlog/blob/main/README.md">readme</a>' +
      '<a href="https://github.com/regeda/techlog-evil">other</a>' +
      '<a href="https://github.com/someone/else">else</a>',
  });
  const problems = checkLinks(dir);
  assert.equal(problems.length, 2, problems.join('\n'));
  assert.ok(problems.some((p) => /techlog-evil/.test(p)) && problems.some((p) => /someone\/else/.test(p)));
});
