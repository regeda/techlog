# Website and README cross-links, install-first Getting Started

Date: 2026-10-19. System: techlog. Components: `website/`, `README.md`.
Motivated by problem `evt_MVFQ2LE3PQ65`.

## Problem

The website (`website/`) and the root `README.md` do not know each other.

- Nothing under `website/src` links to `https://github.com/regeda/techlog`.
- The root README links to `website/` in one sentence and does not name the
  hosted site, `https://techlog-e7k.pages.dev/`.
- The website never says how to get a server. Getting Started and Welcome say
  "from the root of this repository", need Go 1.27 and `make`, and so assume a
  checkout, although a released binary and `install.sh` now exist.
- The Claude plugin page and the README install the plugin from a local path
  (`/plugin marketplace add /path/to/techlog`), which a reader of the website
  does not have.
- `website/README.md` still says the samples target "an illustrative API at a
  reserved, unresolvable name" and were never run. That is stale: the samples
  default to `http://localhost:8080`.

## Goals

1. A website reader can reach the GitHub repo and its README from every page.
2. A GitHub README reader can reach the hosted website and its docs.
3. The website tells a reader how to install Techlog the way the root README
   does, with the ready-made binary and no Go toolchain.
4. The Claude plugin installs from the GitHub repo, not from a local copy.

## Non-goals

- Verifying every language sample automatically (Java and TypeScript are still
  not compiled in CI). Not requested.
- Moving the make targets, the storage notes or other contributor material to
  the website. They stay in the root README.
- Changing the Techlog server, the binary, `install.sh` or the release process.
- Reworking the Welcome page beyond the changes below.

## Design

### Website

**Install.** `getting-started/run-server.sh` is replaced by an example file
that holds the install flow, the same commands as the root README:

```
curl -sSfL https://raw.githubusercontent.com/regeda/techlog/main/install.sh | sh
techlog seed    # load the Spring Spin example story
techlog serve   # http://localhost:8080
```

Getting Started starts from it. The Go 1.27 prerequisite and the "from the root
of this repository" wording go, and nothing in the install steps mentions Go:
the installer fetches a ready binary. (The Go tab among the language samples
stays, since it is a client sample.) The page also says in one line that the
installer puts `techlog` in `$HOME/.local/bin` and verifies the download's
checksum, as the README does.

**Web UI.** The description of the four UI views (Events, Event, System, Story)
moves from `server.html` into Getting Started, after the server is running.

**"Run the server" is removed.** `src/pages/docs/server.html` is deleted. Its
make targets and storage notes are not carried over (the root README covers
them). Every link to `server.html` (Getting Started, Welcome, `welcome` tests,
search index, sidebar, pagers) is repointed or removed.

**HTTP API page.** A new page under the Integrate section, order 3 (after MCP
integration and Claude plugin), holds the route table and the error shape that
`server.html` had. Pagers of the neighbouring pages are updated.

**Claude plugin page.** Install becomes
`/plugin marketplace add regeda/techlog` then
`/plugin install techlog@techlog`. The `make plugin` and `make plugin-remove`
lines and the "from a checkout" wording go. `examples/claude-plugin-install.sh`
is updated, and the Welcome/README mentions follow it.

**Welcome page.** The "Try it now / Run the server on your machine" section is
removed, with its `make seed`, `make run` and `mcp-connect` snippets and its
link to `server.html`. In its place the page says plainly that the platform is
coming soon and that, to test locally today, a reader should use the GitHub
repo (link), with Getting Started as the fallback. The lead sentence that says
the steps are "below" is rewritten to match. The status table, "How we work"
and "While you wait" sections stay.

**GitHub links with an icon.** The site header and the site footer each get a
link to `https://github.com/regeda/techlog` shown with a GitHub icon (inline
SVG, with an accessible name such as "Techlog on GitHub"). Getting Started also
links to the repo and the root README in text.

**`website/README.md`.** The "illustrative API, never run" and
syntax-check lines are corrected to describe the samples as running against
`http://localhost:8080`, and the file points to the root README and the repo.

### Root README

- A short "Website and docs" section names `https://techlog-e7k.pages.dev/`
  and links to its Getting Started, HTTP API and Claude plugin pages.
- The plugin install line `/plugin marketplace add /path/to/this/repo` becomes
  `/plugin marketplace add regeda/techlog`. The local-path form stays, marked as
  for contributors working from a checkout.
- The Install section and the make targets stay as they are.

### Tests

- `website/tests/landing.test.mjs`: the Welcome tests that assert the
  "Run the server on your machine" heading, `make seed`, `make run`, the link
  to `server.html` and the embedded `run-server.sh` are rewritten for the new
  Welcome page (coming-soon statement, repo link, Getting Started fallback).
- A new test checks that the install and run commands in the website's install
  example match those in the root `README.md`, so the two cannot drift.
- A test checks that the header and footer link to the GitHub repo on every
  page, and that no page links to the deleted `server.html`.
- `npm test` and `npm run check` must pass.

## Options considered and rejected

- **Keep `make run` from a checkout in Getting Started.** Keeps a reader out of
  the install flow the README already documents, and needs Go.
- **Copy the commands into the site by hand, with no drift test.** The README
  and the site would drift again.
- **Keep "Run the server" and only link to it.** The user wants the page gone.
- **Verify every language sample in CI.** Out of scope.
- **Move make targets and storage notes into the website.** They are
  contributor material; the README keeps them.

## Facts checked and calls made (reviewer's questions)

Checked on 2026-10-19:
- `regeda/techlog` is public. `.claude-plugin/marketplace.json` is at the repo
  root, the marketplace is named `techlog` and holds one plugin, `techlog`
  (source `./plugin`). So `/plugin marketplace add regeda/techlog` then
  `/plugin install techlog@techlog` is correct.
- `install.sh` is served from `main` at raw.githubusercontent.com.
- The hosted site serves clean URLs: `/docs/getting-started` answers 200 and
  `/docs/getting-started.html` redirects (308) to it. The README links to the
  clean form, for example `https://techlog-e7k.pages.dev/docs/getting-started`.

Calls made (the builder may change them without asking):
- The new page is `src/pages/docs/http-api.html`, `section: Integrate`,
  `order: 3`, nav label "HTTP API". Ordering comes from front matter.
- Drift test: compare the three commands (the `curl … | sh` line, `techlog seed`
  and `techlog serve`) literally, ignoring comments and whitespace. The README
  is edited only if its wording differs.
- The contributor path in the README keeps `make plugin` and the local
  `/plugin marketplace add /path/to/this/repo` form. Tests that embed
  `claude-plugin-install.sh` are updated with it.

## Left to the builder

- The exact wording of the new Welcome section, the HTTP API page's intro and
  the README "Website and docs" section.
- The GitHub icon's size and placement in the header and footer, within the
  existing styles and the token-contrast test.
- Whether the HTTP API page needs the "Basic, local, unauthenticated" callout
  that `server.html` had (the user's decision: it keeps the route table and
  error shape; the callout may stay).
