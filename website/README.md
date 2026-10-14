# Techlog website

## What this is
A static website for Techlog: a three-block landing page (headline with a scattered-to-lifecycle animation, how it works, connectors), a "coming soon" Welcome page, eleven documentation pages (including Getting Started in five languages). The site itself has no backend. Its pages describe two things and mark which is which: the Techlog server in this repository (implemented, basic, local, with no authentication or signatures) and the proposed standard and platform around it.

## Quick start
Run these from the `website/` folder of the repository.
```sh
npm run build   # src/ -> site/
npm run serve   # http://localhost:8090
```
Or open `site/index.html` directly from disk. Node 22 or newer is required; Python 3 is needed only for `serve`.

## Commands
| Command | What it does |
|---|---|
| `npm run build` | Assembles `src/` into `site/` and writes the search index. |
| `npm test` | Runs the build, example, landing, search, token-contrast, and tool tests. |
| `npm run check` | Checks every link and runs the content audit on `site/`. |
| `npm run serve` | Serves `site/` locally. |

## Network policy
Every Node script runs under `node --permission` with only the `--allow-fs-*` flags it needs and **no `--allow-net`**. `tests/network-policy.test.mjs` proves that a network call is denied. The project has no dependencies, so there are no package installs. Mermaid diagrams are loaded by the *browser* from `cdn.jsdelivr.net`, on diagram pages only; if that fails, the diagram source stays visible as text.

## Architecture
`src/` is the input, `build.mjs` is the engine, and `site/` is the output.

| Syntax | Meaning |
|---|---|
| Front matter (`title`, `description`, `layout`, `section`, `order`, `nav`) | Page metadata and the docs sidebar entry. |
| `{{> partial}}` | Include a file from `src/partials/`. |
| `{{var}}` | Insert a variable (`root`, `title`, and others). |
| `{{code lang file}}` | Insert a file from `src/examples/` as a highlighted, copyable code block. |
| `{{json-script id file}}` | Embed a JSON file as a `<script type="application/json">`. |

Canonical examples live in `src/examples/`. The docs and the tests read the same files, so they cannot drift apart. `timeline.json` and `story.json` hold the 25-event Spring Spin story.

## Authoring
To add a docs page, create `src/pages/docs/<name>.html` with front matter (`layout: docs`, a `section`, an `order`, and a `nav` label), run `npm run build`, then `npm run check`. Start the page with the header block used by the other docs pages and end it with a `.pager`.

## Status and limitations
| Part | Status |
|---|---|
| Website and documentation | Exists |
| Event model and schema, event API, MCP server (five tools), storage layer with a SQLite backend, read-only web UI, Claude skill | Implemented in the Go server at the repository root: basic, local, no authentication or signatures |
| Rollback-plan and evidence MCP tools, MCP resources and prompts, collectors, policy evaluation, signatures, authentication | Proposed (draft 0.1) |
| Home-page walkthrough | Simulated example |
| Hosted platform | Not available |

- There is no hosted service. The Getting Started code targets an illustrative API at a reserved, unresolvable name and is never run against anything.
- Only the Python and bash samples were syntax-checked and the Go sample compiled; the Java and TypeScript samples were not compiled (no toolchain here).
- The design-partner contact is not yet published.
- Mermaid diagrams need network access in the browser.
- There are no committed browser or end-to-end tests.

## Content rules
No invented customers, testimonials, or certifications, and no claim that the standard is established. A deliberate negation that the audit would flag goes on its own line and ends with `<!--audit-ok-->`.
