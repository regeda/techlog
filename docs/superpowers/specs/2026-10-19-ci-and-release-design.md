# CI and release pipeline design

Problem: `evt_MVFOOFLFAR21` (system `techlog`, component `CI workflows`).

## Intent

Techlog has no GitHub pipeline and no release process. Maintainers and
contributors run tests and lint by hand, and nobody can download a binary.
Every change should be tested and linted automatically on GitHub, and a tagged
version should publish Go binaries that Linux and macOS engineers can
download.

The user said who needs this (maintainers and contributors), the outcome
(automatic test and lint on every change, downloadable binaries for tagged
versions) and the audience for binaries (Linux and macOS engineers). They then
asked me to pick the remaining defaults, so every choice below that the user
did not state is **my call**, marked as such.

## Goals

- Every pull request and every push to `main` runs `go test ./...` and
  golangci-lint on GitHub and reports pass or fail.
- Pushing a tag `vX.Y.Z` builds `cmd/techlog` for Linux and macOS, on amd64 and
  arm64, and publishes them on a GitHub Release with checksums.
- An `install.sh` at the repo root downloads the right release binary for the
  machine and installs it (added at the user's request).
- Tool versions are pinned, so a run today and a run in a year behave the same.
- CI runs the same checks as `make check`, with one deliberate difference
  (see "Lint in CI").

## Non-goals

- Windows binaries, Docker images, Homebrew tap, deb/rpm packages.
- Signing, SBOMs, provenance attestation.
- Deploying the website (`make deploy-website` stays manual).
- Adding a `--version` flag or stamping a version into the binary. `cmd/techlog`
  has none today; that is a separate problem.
- Branch protection settings. Making the CI check required is a repository
  setting, not a file in the repo.

## Anti-goals

- A pipeline that disagrees with `make check`, so a change passes locally and
  fails on GitHub, or the other way round.
- A release that can be published from a commit whose tests fail.
- Floating tool versions that make CI red with no change in the repo.

## Constraints from the repo

- The module is `github.com/regeda/techlog`, `go 1.27`, one binary at
  `./cmd/techlog`.
- The SQLite driver is `modernc.org/sqlite`, which is pure Go, so
  `CGO_ENABLED=0` cross-compilation works and no C toolchain is needed in CI.
- `.golangci.yml` is a v2 config; the Makefile pins golangci-lint `v2.15.0`.
- `website/` is excluded from lint and is not part of the Go build.
- The remote is GitHub (`regeda/techlog`).
- Commits follow Conventional Commits.

## Design

Two workflows and one GoReleaser config.

### `.github/workflows/ci.yml`

- Triggers: `pull_request`, and `push` to `main`.
- One job on `ubuntu-latest` with `permissions: contents: read`.
- Steps: checkout; `actions/setup-go` with `go-version-file: go.mod`;
  `go test ./...`; golangci-lint through `golangci/golangci-lint-action`,
  pinned to `v2.15.0`.
- A `concurrency` group per ref that cancels superseded runs on pull requests.
- Tests run on Linux only (my call: the binary targets both, but the code is
  pure Go with no platform branches that I found; add a macOS leg later if a
  platform bug appears).

### Lint in CI

`make lint` runs `golangci-lint run --fix`, which rewrites files. CI must not
fix anything: it runs the linter read-only and fails on any finding. This is
the one intended difference from `make check`.

The version `v2.15.0` lives in the Makefile (`GOLANGCI_LINT_VERSION`) and would
be repeated in the workflow. My call: accept the duplication and add a comment
in both places saying to change both. The builder may instead read the version
from the Makefile in the workflow if that is simple and robust.

### `.github/workflows/release.yml`

- Trigger: push of a tag matching `v*`.
- Job `test` (the same test and lint steps as CI, so a tag cannot publish from a
  red commit), then job `release` that `needs: test`.
- `release` has `permissions: contents: write`, checks out with
  `fetch-depth: 0` (GoReleaser needs tag history for the changelog), and runs
  `goreleaser/goreleaser-action` with `args: release --clean`, authenticated
  with the built-in `GITHUB_TOKEN`. No extra secrets.

### `.goreleaser.yaml`

- GoReleaser pinned to a v2 release.
- One build: `main: ./cmd/techlog`, binary name `techlog`, `CGO_ENABLED=0`,
  `goos: [linux, darwin]`, `goarch: [amd64, arm64]`, `-trimpath`, and
  `-s -w` ldflags.
- Archives: `tar.gz`, named `techlog_<version>_<os>_<arch>`, containing the
  binary, `LICENSE` and `README.md`.
- `checksums.txt` for the archives.
- Changelog generated from commits (Conventional Commits make this readable),
  with `docs:` and `chore:` entries filtered out.
- No version ldflag, as in Non-goals.

### Pinning

Tools are pinned: golangci-lint `v2.15.0`, GoReleaser to a v2 release, Go from
`go.mod`. GitHub Actions are pinned to a major tag (for example
`actions/checkout@v4`), not to a commit SHA. My call: major tags keep the files
readable; SHA pinning is a hardening step for later.

### Releasing

Maintainers release with `git tag vX.Y.Z && git push origin vX.Y.Z`. A short
"Releasing" section in `README.md` says so. A tag without the `v` prefix does
nothing.

### `install.sh`

A POSIX `sh` script at the repo root that installs a released binary. The user
asked for it; the behaviour below is my call unless noted.

- Detects the OS (`uname -s`: Linux or Darwin) and the architecture
  (`uname -m`: `x86_64` or `amd64` becomes amd64, `arm64` or `aarch64` becomes
  arm64). Any other value stops with a clear message.
- Picks the version: `VERSION=vX.Y.Z` if set, otherwise the latest release.
- Downloads the matching `techlog_<version>_<os>_<arch>.tar.gz` and
  `checksums.txt` from the GitHub Release of `regeda/techlog`, using `curl`, or
  `wget` if there is no `curl`.
- Verifies the archive's SHA-256 against `checksums.txt` (`sha256sum`, or
  `shasum -a 256` on macOS) and stops on a mismatch, before installing anything.
- Extracts only the `techlog` binary and installs it to `INSTALL_DIR`, default
  `$HOME/.local/bin`, creating it if needed. It never uses `sudo`. If the
  directory is not on `PATH`, it prints a line saying so.
- Works from a pipe: `curl -sSfL https://raw.githubusercontent.com/regeda/techlog/main/install.sh | sh`.
  The README documents this and the `VERSION` and `INSTALL_DIR` variables.
- Uses a temporary directory that is removed on exit, `set -eu`, and no
  dependency beyond `tar`, `uname`, a downloader and a checksum tool.
- Re-running it replaces the installed binary with the requested version.
- The script is checked by CI with `shellcheck` (my call: one extra step in
  `ci.yml`, pre-installed on `ubuntu-latest`).

## Rejected options

- **Hand-written build matrix plus `gh release create`.** No new tool, but it
  means writing archive naming, checksums and the changelog by hand and
  keeping them right. GoReleaser does all of it from one config file.
- **A release script under `scripts/` called from the Makefile.** It would work
  the same locally and in CI, but the repo's goal is that GitHub does it; a
  local release path is what we are moving away from.
- **Release on every push to `main`.** Produces a stream of untagged versions
  and needs a versioning rule. A tag is an explicit decision.
- **Run CI on a Linux and macOS matrix now.** Doubles the cost and time for no
  known platform-specific code. Deferred.
- **SHA-pinning every action.** Safer, but noisier to read and update. Deferred.
- **Installing to `/usr/local/bin` with `sudo`.** Works on more `PATH`s, but a
  script that asks for root is a bigger risk to pipe from the network.
- **Building from source in `install.sh` (`go install`).** Needs a Go
  toolchain, which the audience may not have; the point is a prebuilt binary.
- **Skipping checksum verification.** Simpler, but the script would install
  whatever the download returns.
- **Reuse `make check` in CI.** `make lint` runs `--fix`, and it installs the
  linter with a `curl | sh` script. The official action is cached and does not
  modify the tree.

## Left to the builder

- Exact action major versions, in particular the `golangci-lint-action`
  version that supports golangci-lint v2.
- Exact GoReleaser v2 version pin.
- Whether to read the lint version from the Makefile.
- Changelog filter wording.
- Wording of the README "Releasing" and "Install" sections.
- Whether `install.sh` also checks that the binary runs after installing.

## Verification

- A pull request run shows test and lint jobs; a deliberately failing test or
  lint finding turns it red.
- Pushing a tag on a fork or a test tag produces a Release with four archives
  and `checksums.txt`, and the Linux amd64 archive's binary starts with
  `techlog serve`.
- `install.sh` passes `shellcheck`; run against a real release on Linux and
  macOS it installs a working `techlog`, refuses a tampered archive, honours
  `VERSION` and `INSTALL_DIR`, and fails with a clear message on an unsupported
  OS or architecture.
- GoReleaser can be checked locally with `goreleaser check` and
  `goreleaser release --snapshot --clean` before tagging.
