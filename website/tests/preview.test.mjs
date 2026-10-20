import test from 'node:test';
import assert from 'node:assert/strict';
import { readFileSync } from 'node:fs';

// The site preview must not share a port with the Techlog server (8080).
const PORT = '8090';
const pkg = JSON.parse(readFileSync('package.json', 'utf8'));

test('preview: npm run serve listens on its own port', () => {
  assert.match(pkg.scripts.serve, new RegExp(`http\\.server ${PORT}\\b`));
});

test('preview: the README names the preview URL', () => {
  for (const f of ['README.md']) {
    const text = readFileSync(f, 'utf8');
    const line = text.split('\n').find((l) => l.includes('npm run serve'));
    assert.ok(line, `${f} mentions npm run serve`);
    assert.match(line, new RegExp(`localhost:${PORT}`), `${f}: ${line}`);
    assert.doesNotMatch(line, /8080/, `${f} must not point the preview at the server's port`);
  }
});
