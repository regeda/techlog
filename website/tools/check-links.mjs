import { readFileSync, readdirSync, statSync, existsSync } from 'node:fs';
import { join, dirname, relative, sep } from 'node:path';
import { fileURLToPath } from 'node:url';

const ALLOWED_EXTERNAL_HOSTS = new Set(['cdn.jsdelivr.net', 'claude.com']);
const REPO_URL = 'https://github.com/regeda/techlog';

function walk(dir) {
  const out = [];
  for (const name of readdirSync(dir)) {
    const p = join(dir, name);
    if (statSync(p).isDirectory()) out.push(...walk(p));
    else out.push(p);
  }
  return out;
}

const idsOf = (html) => new Set([...html.matchAll(/\sid="([^"]+)"/g)].map((m) => m[1]));

export function checkLinks(siteDir) {
  const problems = [];
  const files = walk(siteDir).filter((f) => f.endsWith('.html'));
  const htmlCache = new Map(files.map((f) => [f, readFileSync(f, 'utf8')]));

  for (const file of files) {
    const html = htmlCache.get(file);
    const name = relative(siteDir, file).split(sep).join('/');
    if (/<script[^>]*type="module"/.test(html)) {
      problems.push(`${name}: type="module" scripts do not work from file://`);
    }
    for (const m of html.matchAll(/\s(?:href|src)="([^"]*)"/g)) {
      const value = m[1];
      if (value.startsWith('data:') || value.startsWith('mailto:')) continue;
      if (value === '' || value === '#') {
        problems.push(`${name}: empty or "#" link`);
        continue;
      }
      if (/^https?:\/\//.test(value)) {
        const host = new URL(value).host;
        const isRepo = value === REPO_URL || value.startsWith(`${REPO_URL}/`);
        if (!ALLOWED_EXTERNAL_HOSTS.has(host) && !isRepo) problems.push(`${name}: external link not allowed: ${value}`);
        continue;
      }
      if (value.startsWith('/')) {
        problems.push(`${name}: root-absolute link breaks file:// and subpaths: ${value}`);
        continue;
      }
      const [path, fragment] = value.split('#');
      let target = path === '' ? file : join(dirname(file), path);
      if (path.endsWith('/')) target = join(target, 'index.html');
      if (!existsSync(target)) {
        problems.push(`${name}: broken link: ${value}`);
        continue;
      }
      if (fragment && target.endsWith('.html')) {
        const targetHtml = htmlCache.get(target) ?? readFileSync(target, 'utf8');
        if (!idsOf(targetHtml).has(fragment)) problems.push(`${name}: missing fragment: ${value}`);
      }
    }
  }
  return problems;
}

if (process.argv[1] === fileURLToPath(import.meta.url)) {
  const dir = process.argv[2] ?? 'site';
  const problems = checkLinks(dir);
  for (const p of problems) console.error(p);
  console.log(problems.length ? `${problems.length} link problem(s)` : `links OK (${dir})`);
  process.exit(problems.length ? 1 : 0);
}
