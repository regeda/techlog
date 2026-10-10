import test from 'node:test';
import assert from 'node:assert/strict';
import { readFileSync } from 'node:fs';

const css = readFileSync('src/assets/css/tokens.css', 'utf8');

// Parse "selector { --name: value; ... }" blocks into { selector: { name: value } }.
function blocks(src) {
  const out = {};
  for (const m of src.matchAll(/([^{}]+)\{([^}]*)\}/g)) {
    const vars = {};
    for (const d of m[2].matchAll(/--([\w-]+)\s*:\s*([^;]+);/g)) vars[d[1]] = d[2].trim();
    out[m[1].trim().replace(/\s+/g, ' ')] = vars;
  }
  return out;
}
const b = blocks(css);
const light = b[':root'];
const darkKey = Object.keys(b).find((k) => k.includes('.cs') && k.includes('.codeblock'));
const dark = b[darkKey];

function lum(hex) {
  const h = hex.replace('#', '');
  const [r, g, bl] = [0, 2, 4].map((i) => parseInt(h.slice(i, i + 2), 16) / 255)
    .map((c) => (c <= 0.03928 ? c / 12.92 : ((c + 0.055) / 1.055) ** 2.4));
  return 0.2126 * r + 0.7152 * g + 0.0722 * bl;
}
const ratio = (a, c) => {
  const [hi, lo] = [lum(a), lum(c)].sort((x, y) => y - x);
  return (hi + 0.05) / (lo + 0.05);
};
const hex = /^#[0-9a-fA-F]{6}$/;

test('page theme is the light grey of the Techlog UI; the terminal scope is dark', () => {
  assert.ok(light, 'root tokens exist');
  assert.equal(light['bg-0'].toLowerCase(), '#fafaf8', 'page background is the light grey of the Techlog UI');
  assert.ok(dark, 'a dark scope exists for the console and code blocks (.cs, .codeblock, pre.mermaid)');
  assert.ok(lum(dark['bg-0']) < 0.05 && lum(dark['bg-1']) < 0.05, 'dark scope backgrounds are dark');
});

test('light theme: text, accent and status colors are readable on the page and on cards', () => {
  for (const bg of ['bg-0', 'bg-1']) {
    for (const k of ['text-0', 'text-1', 'text-2', 'accent', 'accent-hover', 'info', 'ok', 'warn', 'danger', 'hyp', 'orange']) {
      assert.match(light[k], hex, k);
      assert.ok(ratio(light[k], light[bg]) >= 4.5, `light ${k} on ${bg}: ${ratio(light[k], light[bg]).toFixed(2)}`);
    }
  }
  assert.ok(ratio(light['accent-ink'], light.accent) >= 4.5, 'button text on accent');
  assert.ok(ratio(light['accent-ink'], light['accent-hover']) >= 4.5, 'button text on hovered accent');
});

test('dark scope: text, accent, status colors and gold are readable on dark panels', () => {
  for (const bg of ['bg-0', 'bg-1']) {
    for (const k of ['text-1', 'text-2', 'accent', 'info', 'ok', 'warn', 'danger', 'hyp', 'orange', 'gold']) {
      assert.match(dark[k], hex, k);
      assert.ok(ratio(dark[k], dark[bg]) >= 4.5, `dark ${k} on ${bg}: ${ratio(dark[k], dark[bg]).toFixed(2)}`);
    }
  }
  assert.ok(ratio(dark['accent-ink'], dark.accent) >= 4.5, 'dark button text on accent');
});

test('buttons: primary (green) and grey have readable text in both scopes', () => {
  for (const [name, scope] of [['light', light], ['dark', dark]]) {
    for (const [bg, ink] of [['primary', 'primary-ink'], ['primary-hover', 'primary-ink'], ['grey-btn', 'grey-ink'], ['grey-btn-hover', 'grey-ink']]) {
      assert.match(scope[bg], hex, `${name} ${bg}`);
      assert.ok(ratio(scope[ink], scope[bg]) >= 4.5, `${name} ${ink} on ${bg}: ${ratio(scope[ink], scope[bg]).toFixed(2)}`);
    }
  }
  const g = parseInt(light.primary.slice(3, 5), 16);
  const r = parseInt(light.primary.slice(1, 3), 16);
  const bl = parseInt(light.primary.slice(5, 7), 16);
  assert.ok(g > r && g > bl, 'the primary button is green');
});

