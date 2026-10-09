import { readFileSync, readdirSync, statSync } from 'node:fs';
import { join, relative, sep } from 'node:path';
import { fileURLToPath } from 'node:url';

const RULES = [
  [/tamper[- ]proof/i, 'tamper-proof claim'],
  [/\bcertified\b|\bcertification\b/i, 'certification claim'],
  [/guarantee[sd]?\b/i, 'guarantee claim'],
  [/\bSOC ?2\b|ISO ?27001|HIPAA|FedRAMP|PCI[- ]DSS/i, 'compliance framework name'],
  [/revolutioniz/i, 'forbidden marketing phrase'],
  [/future of AI/i, 'forbidden marketing phrase'],
  [/one platform to rule/i, 'forbidden marketing phrase'],
  [/\btrusted by\b|\bour customers\b|\bcustomers (love|use|rely)\b|\btestimonial/i, 'customer-traction claim'],
];

function walk(dir) {
  const out = [];
  for (const name of readdirSync(dir)) {
    const p = join(dir, name);
    if (statSync(p).isDirectory()) out.push(...walk(p));
    else out.push(p);
  }
  return out;
}

export function auditContent(siteDir) {
  const problems = [];
  for (const file of walk(siteDir).filter((f) => f.endsWith('.html'))) {
    const name = relative(siteDir, file).split(sep).join('/');
    readFileSync(file, 'utf8').split('\n').forEach((line, i) => {
      if (line.includes('<!--audit-ok-->')) return;
      for (const [re, label] of RULES) {
        if (re.test(line)) {
          problems.push(`${name}:${i + 1}: ${label}: ${line.trim().slice(0, 90)}`);
          break; // one finding per line
        }
      }
    });
  }
  return problems;
}

if (process.argv[1] === fileURLToPath(import.meta.url)) {
  const dir = process.argv[2] ?? 'site';
  const problems = auditContent(dir);
  for (const p of problems) console.error(p);
  console.log(problems.length ? `${problems.length} audit finding(s)` : `content audit OK (${dir})`);
  process.exit(problems.length ? 1 : 0);
}
