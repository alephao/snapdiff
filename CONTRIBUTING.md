# Contributing to snapdiff

Thanks for your interest in improving `snapdiff`. This guide covers how to set
up a dev environment, build and test the project, and how design decisions are
recorded.

Before diving in, skim [`docs/spec.md`](docs/spec.md) — it's the canonical
system design (V-Model, scope, architecture) and the fastest way to understand
how the pieces fit together.

## Prerequisites

- **Go 1.26.3+** (the version is pinned in [`go.mod`](go.mod)).
- **[`templ`](https://templ.guide)** — the web views are generated from `.templ`
  files, so `make build` runs `make generate` first. Install it once:

  ```sh
  GOBIN=$HOME/go/bin go install github.com/a-h/templ/cmd/templ@latest
  ```

  The Makefile looks for `templ` at `$HOME/go/bin/templ`; override with
  `make TEMPL=/path/to/templ ...` if yours lives elsewhere.
- **Node 22+ and pnpm 8.14.1** — *optional*, only needed to run the Playwright
  screenshot suite (see [Screenshot dogfood](#screenshot-dogfood)).

## Repo layout

```
cmd/snapdiff/        CLI entry point (await / serve / gallery / version)
internal/
  config/            snapdiff.toml parsing & validation
  gitscan/           git diff/show wrappers + axis extraction
  imdiff/            PNG pixel-diff
  review/            in-memory session & verdict state
  web/               chi router + templ views
  apply/             applies verdicts (git checkout / remove)
  lifecycle/         daemon startup & graceful shutdown
  gallery/           read-only baseline browser
test/
  acceptance/        end-to-end tests (-tags acceptance)
  integration/       package-level integration tests
  fixtures/          sample git repos & images
tests-screenshots/   Playwright visual-regression suite (opt-in)
docs/
  spec.md            system design (V-Model)
  v-model.md         verification framework & traceability
  adr/               architecture decision records
```

## Build & run locally

```sh
make build      # generates templ output, then builds ./snapdiff
./snapdiff serve

make install    # build, then copy to /usr/local/bin/snapdiff (uses sudo)
```

The version string is derived from `git describe --tags --always --dirty` and
baked in at build time, so local builds report something like `v0.0.1-3-gabc123`.

## Make targets

| Target                 | What it does                                                       |
| ---------------------- | ------------------------------------------------------------------ |
| `all`                  | `lint test build`                                                  |
| `generate`             | Regenerate Go from `.templ` views (requires `templ`).              |
| `build`                | Generate, then build `./snapdiff`.                                 |
| `install`              | Build, then copy the binary to `/usr/local/bin` (sudo).            |
| `test`                 | `go test ./...` (unit + integration).                              |
| `acceptance`           | End-to-end tests: `go test -tags acceptance ./test/acceptance/...`.|
| `vet`                  | `go vet ./...`.                                                    |
| `fmt`                  | `gofmt -w .`.                                                      |
| `lint`                 | gofmt check (excludes `*_templ.go`) + `go vet`.                    |
| `clean`                | Remove `./snapdiff` and `dist/`.                                   |
| `release`              | Cross-compile the 5-platform matrix into `dist/`.                  |
| `screenshots-install`  | `pnpm install` + install the chromium browser.                     |
| `screenshots`          | Run the Playwright suite.                                          |
| `screenshots-update`   | Rewrite the screenshot baselines.                                  |

## Testing

`snapdiff` follows a V-Model: tests exist at several levels, and the acceptance
test is the north star. The everyday checks are:

```sh
make lint         # gofmt + go vet
make test         # unit + integration
make acceptance   # end-to-end against a fixture repo
```

CI runs all three on every push to `main` and every pull request, so run them
locally before opening a PR. See [`docs/v-model.md`](docs/v-model.md) for the
requirement-to-test traceability matrix.

## Screenshot dogfood

The web UI is covered by a Playwright suite under `tests-screenshots/`. Per
[ADR-0017](docs/adr/), the baselines are reviewed **with snapdiff itself** —
there's a `snapdiff.toml` at the repo root pointing at
`tests-screenshots/baselines/`.

First-time setup:

```sh
make screenshots-install      # pnpm install + chromium
```

When you change CSS or `templ` views:

```sh
make screenshots              # green if no regression
# if it fails and the change is intended:
make screenshots-update       # rewrite baselines
snapdiff serve                # review your own diff in snapdiff
# approve / reject in the browser; rejected files are reverted via
# `git checkout HEAD --`, approved ones stay dirty for `git commit`.
```

## Design decisions (ADRs)

Architectural decisions are recorded as ADRs in [`docs/adr/`](docs/adr/). If
you're proposing something that changes the architecture or a documented
trade-off (git-as-state, no-auth, config format, etc.), add an ADR using
[`docs/adr/template.md`](docs/adr/template.md) as part of your PR. Read the
existing ADRs first — many "why isn't this configurable / persisted / pluggable"
questions are already answered there.

## Commit conventions

Use [Conventional Commits](https://www.conventionalcommits.org/) (`feat:`,
`fix:`, `docs:`, `test:`, `chore:`, …). The release changelog is generated by
GoReleaser, which excludes `docs:`, `test:`, `chore:`, and merge commits — so
keep user-facing changes under `feat:`/`fix:`.

## Releasing

Releases are automated. A maintainer pushes a `v*` tag; GitHub Actions runs the
full lint + test suite and then GoReleaser cross-compiles the matrix (darwin,
linux, windows × amd64/arm64, minus windows/arm64) and publishes a GitHub
Release with archives and `checksums.txt`.
