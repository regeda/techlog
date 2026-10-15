import {
  readFileSync, writeFileSync, mkdirSync, readdirSync, statSync, cpSync, rmSync, existsSync,
} from 'node:fs';
import { join, relative, dirname, posix, sep, basename, extname } from 'node:path';
import { fileURLToPath } from 'node:url';

const GROUP_ORDER = ['Start', 'Concepts', 'Integrate', 'Operate', 'Reference'];

const esc = (s) =>
  String(s).replace(/&/g, '&amp;').replace(/</g, '&lt;').replace(/>/g, '&gt;').replace(/"/g, '&quot;');

export function parseFrontMatter(text) {
  const m = text.match(/^---\n([\s\S]*?)\n---\n?/);
  if (!m) return { meta: {}, body: text };
  const meta = {};
  for (const line of m[1].split('\n')) {
    const i = line.indexOf(':');
    if (i < 0) continue;
    meta[line.slice(0, i).trim()] = line.slice(i + 1).trim();
  }
  return { meta, body: text.slice(m[0].length) };
}

function walk(dir) {
  const out = [];
  for (const name of readdirSync(dir)) {
    const p = join(dir, name);
    if (statSync(p).isDirectory()) out.push(...walk(p));
    else out.push(p);
  }
  return out;
}

// Empties the directory's contents instead of removing the directory itself,
// so a write permission scoped to ./site is enough.
function emptyDir(dir) {
  mkdirSync(dir, { recursive: true });
  for (const n of readdirSync(dir)) rmSync(join(dir, n), { recursive: true, force: true });
}

function loadDir(dir) {
  const m = {};
  if (!existsSync(dir)) return m;
  for (const f of readdirSync(dir)) m[basename(f, extname(f))] = readFileSync(join(dir, f), 'utf8');
  return m;
}

function stripTags(html) {
  return html
    .replace(/<script[\s\S]*?<\/script>/g, ' ')
    .replace(/<style[\s\S]*?<\/style>/g, ' ')
    .replace(/<pre class="mermaid">[\s\S]*?<\/pre>/g, ' ')
    .replace(/<[^>]+>/g, ' ')
    .replace(/&lt;/g, '<').replace(/&gt;/g, '>').replace(/&quot;/g, '"').replace(/&amp;/g, '&')
    .replace(/\s+/g, ' ')
    .trim();
}

const slug = (s) =>
  stripTags(s).toLowerCase().replace(/[^a-z0-9]+/g, '-').replace(/^-+|-+$/g, '') || 'section';

function addHeadingIds(html) {
  const headings = [];
  const used = new Set();
  const out = html.replace(/<h([23])(?:\s+id="([^"]*)")?>([\s\S]*?)<\/h\1>/g, (_, lvl, id, inner) => {
    const base = id || slug(inner);
    let unique = base;
    let n = 2;
    while (used.has(unique)) unique = `${base}-${n++}`;
    used.add(unique);
    headings.push({ level: Number(lvl), id: unique, text: stripTags(inner) });
    return `<h${lvl} id="${unique}">${inner}</h${lvl}>`;
  });
  return { html: out, headings };
}

function readExample(ctx, file, from) {
  const p = join(ctx.srcDir, 'examples', file);
  if (!existsSync(p)) throw new Error(`${from}: missing example file "${file}"`);
  return readFileSync(p, 'utf8').replace(/\n$/, '');
}

// formatJson prints a JSON value with short objects and arrays on one line and
// longer ones expanded, so examples stay compact without losing structure.
export function formatJson(value, indent = 0) {
  const flat = inlineJson(value);
  if (flat.length + indent <= 80 || value === null || typeof value !== 'object') return flat;
  const pad = ' '.repeat(indent + 2);
  const close = ' '.repeat(indent);
  if (Array.isArray(value)) {
    return `[\n${value.map((v) => pad + formatJson(v, indent + 2)).join(',\n')}\n${close}]`;
  }
  const entries = Object.entries(value).map(([k, v]) => `${pad}${JSON.stringify(k)}: ${formatJson(v, indent + 2)}`);
  return `{\n${entries.join(',\n')}\n${close}}`;
}

function inlineJson(v) {
  if (Array.isArray(v)) return v.length ? `[${v.map(inlineJson).join(', ')}]` : '[]';
  if (v && typeof v === 'object') {
    const entries = Object.entries(v);
    return entries.length ? `{ ${entries.map(([k, x]) => `${JSON.stringify(k)}: ${inlineJson(x)}`).join(', ')} }` : '{}';
  }
  return JSON.stringify(v);
}

// readExamplePart returns one part of a JSON example: "request.params" selects
// doc.request.params. A missing key fails the build.
function readExamplePart(ctx, file, path, from) {
  let part = JSON.parse(readExample(ctx, file, from));
  for (const key of path.split('.')) {
    if (part === null || typeof part !== 'object' || !(key in part)) {
      throw new Error(`${from}: example "${file}" has no "${key}" (path "${path}")`);
    }
    part = part[key];
  }
  return formatJson(part);
}

// highlightJson wraps JSON tokens in spans that css colours from the design
// tokens. It works on the raw text and escapes each piece, so the markup is safe.
const JSON_TOKEN = /("(?:[^"\\]|\\.)*")(\s*:)?|\b(?:true|false|null)\b|-?\d+(?:\.\d+)?(?:[eE][+-]?\d+)?|[{}[\],:]/g;
export function highlightJson(text) {
  let out = '';
  let last = 0;
  for (const m of text.matchAll(JSON_TOKEN)) {
    out += esc(text.slice(last, m.index));
    last = m.index + m[0].length;
    if (m[1] !== undefined) {
      out += `<span class="${m[2] ? 'tk-key' : 'tk-str'}">${esc(m[1])}</span>${m[2] ? `<span class="tk-pun">${esc(m[2])}</span>` : ''}`;
    } else if (/^[{}[\],:]$/.test(m[0])) out += `<span class="tk-pun">${m[0]}</span>`;
    else out += `<span class="${/^[tfn]/.test(m[0]) ? 'tk-lit' : 'tk-num'}">${m[0]}</span>`;
  }
  return out + esc(text.slice(last));
}

// highlightCode colours the sample languages with a small ordered tokenizer:
// comments, strings, numbers, keywords, and shell variables. Anything it does
// not recognise stays plain text. Like highlightJson it escapes every piece.
const SLASH_COMMENT = String.raw`\/\/[^\n]*`;
const LANGS = {
  bash: { comment: String.raw`(?<![\w$])#[^\n]*`, string: String.raw`"(?:[^"\\]|\\.)*"|'[^']*'`, variable: String.raw`\$\{[^}\n]*\}|\$\w+`,
    kw: 'set export if then fi for in do done while echo' },
  python: { comment: String.raw`#[^\n]*`, string: String.raw`"""[\s\S]*?"""|"(?:[^"\\\n]|\\.)*"|'(?:[^'\\\n]|\\.)*'`,
    kw: 'import from def return with as for in if else None True False' },
  typescript: { comment: SLASH_COMMENT, string: String.raw`"(?:[^"\\\n]|\\.)*"|'(?:[^'\\\n]|\\.)*'|` + '`(?:[^`\\\\]|\\\\.)*`',
    kw: 'const let async function await return throw new if as for of true false null undefined string number unknown Promise' },
  go: { comment: SLASH_COMMENT, string: String.raw`"(?:[^"\\\n]|\\.)*"|` + '`[^`]*`',
    kw: 'package import func var return if else for range defer struct map any string true false nil' },
  java: { comment: SLASH_COMMENT, string: String.raw`"""[\s\S]*?"""|"(?:[^"\\\n]|\\.)*"`,
    kw: 'import class public static final void throws new return if true false null' },
};
for (const l of Object.values(LANGS)) {
  l.re = new RegExp(
    `(${l.comment})|(${l.string})|(${l.variable ?? '(?!)'})|(\\b(?:${l.kw.split(' ').join('|')})\\b)|(\\b\\d+(?:\\.\\d+)?\\b)`, 'g');
}
const CODE_CLASS = ['tk-com', 'tk-str', 'tk-var', 'tk-kw', 'tk-num'];
export function highlightCode(lang, text) {
  if (lang === 'json') return highlightJson(text);
  const l = LANGS[lang];
  if (!l) return esc(text);
  let out = '';
  let last = 0;
  for (const m of text.matchAll(l.re)) {
    out += esc(text.slice(last, m.index));
    last = m.index + m[0].length;
    out += `<span class="${CODE_CLASS[m.findIndex((g, i) => i > 0 && g !== undefined) - 1]}">${esc(m[0])}</span>`;
  }
  return out + esc(text.slice(last));
}

// renderDemoCards turns the demo chain into the "Techlog records" cards of the
// landing page: one <li> per event. The summary, the links and the JSON behind
// "Show JSON" all come from the same event, so they cannot disagree. The links
// field is marked in the JSON so a visitor sees what the arrows mean.
export function renderDemoCards(events) {
  return events.map((e, i) => {
    const n = i + 1;
    const src = e.sources && e.sources[0];
    const meta = `${esc(e.time)}${src ? ` · ${esc(src.kind)}: ${esc(src.ref)}` : ''}`;
    const links = (e.links || []).map((l) =>
      `<li class="rec-link rec-${esc(l.basis)}"><span class="rec-rel">${esc(l.rel)}</span> <span aria-hidden="true">→</span> <code>${esc(l.target)}</code> <span class="rec-basis">${esc(l.basis)}</span></li>`).join('');
    const lines = JSON.stringify(e, null, 2).split('\n');
    const from = lines.indexOf('  "links": [');
    const to = from < 0 ? -1 : lines.indexOf('  ],', from);
    const json = lines.map(highlightJson);
    if (from >= 0 && to >= 0) { json[from] = `<mark class="rec-hl">${json[from]}`; json[to] += '</mark>'; }
    return `<li class="rec-card" id="rec-${n}" data-rec="${n}">` +
      `<div class="rec-head"><span class="rec-type">${esc(e.type)}</span><span class="rec-id">${esc(e.id)}</span></div>` +
      `<p class="rec-sum">${esc(e.summary)}</p>` +
      `<p class="rec-meta">${meta}</p>` +
      (links ? `<ul class="rec-links" aria-label="Links to earlier events">${links}</ul>` : '') +
      `<details class="rec-json"><summary>Show JSON</summary><pre><code class="language-json">${json.join('\n')}</code></pre></details>` +
      `</li>`;
  }).join('\n');
}

function render(template, vars, ctx) {
  let out = template.replace(/\{\{>\s*([\w-]+)\s*\}\}/g, (_, name) => {
    if (!(name in ctx.partials)) throw new Error(`${vars.__file}: missing partial "${name}"`);
    return render(ctx.partials[name], vars, ctx);
  });
  out = out.replace(/\{\{(\w+)\}\}/g, (_, key) => {
    if (!(key in vars)) throw new Error(`${vars.__file}: unknown variable {{${key}}}`);
    return vars[key];
  });
  out = out.replace(/\{\{code\s+(\w+)\s+([^\s}#]+)(?:#([\w.]+))?\s*\}\}/g, (_, lang, file, path) =>
    `<div class="codeblock" data-lang="${lang}"><button class="copy" type="button" aria-label="Copy code">Copy</button>` +
    `<pre><code class="language-${lang}">${((t) => highlightCode(lang, t))(path ? readExamplePart(ctx, file, path, vars.__file) : readExample(ctx, file, vars.__file))}</code></pre></div>`);
  out = out.replace(/\{\{json-script\s+([\w-]+)\s+([^\s}]+)\s*\}\}/g, (_, id, file) => {
    const data = JSON.stringify(JSON.parse(readExample(ctx, file, vars.__file))).replace(/</g, '\\u003c');
    return `<script type="application/json" id="${id}">${data}</script>`;
  });
  out = out.replace(/\{\{demo-cards\s+([^\s}]+)\s*\}\}/g, (_, file) =>
    renderDemoCards(JSON.parse(readExample(ctx, file, vars.__file))));
  return out;
}

function buildSidebar(docs, root) {
  const groups = GROUP_ORDER.map((g) => {
    const items = docs
      .filter((d) => d.meta.section === g)
      .sort((a, b) => Number(a.meta.order || 99) - Number(b.meta.order || 99));
    if (!items.length) return '';
    const lis = items
      .map((d) => `<li><a href="${root}${d.rel}">${esc(d.meta.nav || d.meta.title)}</a></li>`)
      .join('');
    return `<div class="docs-nav-group"><p class="docs-nav-title">${g}</p><ul>${lis}</ul></div>`;
  }).join('');
  return (
    `<nav class="docs-nav" aria-label="Documentation">` +
    `<button class="docs-nav-toggle" type="button" aria-expanded="false" aria-controls="docs-nav-list">Documentation menu</button>` +
    `<div id="docs-nav-list">${groups}</div></nav>`
  );
}

function buildToc(headings) {
  const h2 = headings.filter((h) => h.level === 2);
  if (!h2.length) return '';
  const lis = h2.map((h) => `<li><a href="#${h.id}">${esc(h.text)}</a></li>`).join('');
  return `<nav class="toc" aria-label="On this page"><p class="toc-title">On this page</p><ul>${lis}</ul></nav>`;
}

function markCurrent(html, pagePath) {
  return html.replace(/<a href="([^"#:]+)"(?![^>]*aria-current)/g, (m, href) => {
    let target = posix.normalize(posix.join(posix.dirname(pagePath), href));
    if (target.endsWith('/')) target += 'index.html';
    return target === pagePath ? `<a href="${href}" aria-current="page"` : m;
  });
}

export function build({ srcDir = 'src', outDir = 'site' } = {}) {
  const ctx = { srcDir, partials: loadDir(join(srcDir, 'partials')) };
  const layouts = loadDir(join(srcDir, 'layouts'));
  const pagesDir = join(srcDir, 'pages');

  emptyDir(outDir);
  if (existsSync(join(srcDir, 'assets'))) {
    cpSync(join(srcDir, 'assets'), join(outDir, 'assets'), { recursive: true });
  }

  const pages = walk(pagesDir)
    .filter((f) => f.endsWith('.html'))
    .map((file) => {
      const rel = relative(pagesDir, file).split(sep).join('/');
      const { meta, body } = parseFrontMatter(readFileSync(file, 'utf8'));
      if (!meta.title) throw new Error(`${rel}: front matter "title" is required`);
      return { rel, meta, body };
    });
  const docs = pages.filter((p) => p.meta.layout === 'docs');
  const searchIndex = [];

  for (const page of pages) {
    const layoutName = page.meta.layout || 'page';
    const tpl = layouts[layoutName];
    if (!tpl) throw new Error(`${page.rel}: unknown layout "${layoutName}"`);
    const root = '../'.repeat(page.rel.split('/').length - 1);
    const vars = {
      title: esc(page.meta.title),
      description: esc(page.meta.description || ''),
      root,
      __file: page.rel,
    };
    let content = render(page.body, vars, ctx);
    let headings = [];
    if (layoutName === 'docs') ({ html: content, headings } = addHeadingIds(content));
    const full = render(
      tpl,
      {
        ...vars,
        content,
        sidebar: layoutName === 'docs' ? buildSidebar(docs, root) : '',
        toc: buildToc(headings),
      },
      ctx,
    );
    const outPath = join(outDir, ...page.rel.split('/'));
    mkdirSync(dirname(outPath), { recursive: true });
    writeFileSync(outPath, markCurrent(full, page.rel));
    if (layoutName === 'docs') {
      searchIndex.push({
        url: page.rel,
        title: page.meta.title,
        description: page.meta.description || '',
        headings: headings.filter((h) => h.level === 2).map((h) => h.text),
        text: stripTags(content).slice(0, 8000),
      });
    }
  }

  mkdirSync(join(outDir, 'assets'), { recursive: true });
  writeFileSync(
    join(outDir, 'assets', 'search-index.js'),
    `window.TECHLOG_SEARCH=${JSON.stringify(searchIndex).replace(/</g, '\\u003c')};\n`,
  );
  return { pages: pages.map((p) => p.rel) };
}

if (process.argv[1] === fileURLToPath(import.meta.url)) {
  const r = build();
  console.log(`built ${r.pages.length} pages into site/`);
}
