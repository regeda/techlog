import { readdirSync } from 'node:fs';

const dir = new URL('.', import.meta.url);
for (const f of readdirSync(dir).filter((n) => n.endsWith('.test.mjs')).sort()) {
  await import(`./${f}`);
}
