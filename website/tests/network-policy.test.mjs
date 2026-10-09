import test from 'node:test';
import assert from 'node:assert/strict';

// Fetches a closed LOCAL port: without the permission model this fails with
// ECONNREFUSED; with --permission and no --allow-net it fails with
// ERR_ACCESS_DENIED. No external traffic is ever attempted.
test('node runs without network permission', async () => {
  let code;
  try {
    await fetch('http://127.0.0.1:54321/'); // not port 9: fetch rejects it as a "bad port"
  } catch (e) {
    code = e.code ?? e.cause?.code;
  }
  assert.equal(code, 'ERR_ACCESS_DENIED', `expected the permission model to deny network, got: ${code}`);
});
