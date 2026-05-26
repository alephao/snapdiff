# snapdiff — System Design Specification

## Context

`snapdiff` is a human-in-the-loop review tool for agent-driven screenshot test
workflows. When an agentic frontend developer (web / iOS / Android) asks an AI
to implement a UI change, the agent runs screenshot tests, regenerates
snapshots on failure, and ends up with a pile of diffed PNGs in the working
tree. snapdiff sits between the agent and the human: the agent blocks on a
single CLI command after regenerating snapshots; the human reviews diffs in a
web UI (reachable from outside the home network via Tailscale Serve or
Cloudflare Access); the CLI exits with a JSON verdict per file the agent can
act on (commit, iterate, etc.).

Engineering approach: **V-Model systems engineering with ADRs for every
decision** (see `docs/adr/`).

## Stakeholder Requirements

- **R1.** A single human reviewer can triage screenshot diffs produced by an
  agent and return verdicts the agent can consume.
- **R2.** Verdicts are per-snapshot-file with optional rejection comments;
  bulk actions across a variation axis are supported.
- **R3.** Variation axes (state, device/size, theme, language, web-path, etc.)
  are first-class — diffs can be grouped and filtered by axis.
- **R4.** Five diff visualization modes per snapshot: side-by-side, swipe,
  toggle, pixel-diff highlight overlay, onion (opacity blend).
- **R5.** Single cross-compiled Go binary
  (darwin/linux/windows × amd64/arm64); no external runtime dependencies.
- **R6.** UI is reachable from outside the home network. Auth at the network
  layer (Tailscale Serve primary; Cloudflare Access alternative). No auth
  code in the binary in MVP.
- **R7.** Agent integration is a single blocking shell command that exits
  with machine-readable verdicts.

## System Requirements (derived)

- **S1.** Diff discovery is driven by git: working tree vs configured base
  ref (default `HEAD`), scoped by configurable globs.
- **S2.** No persistence; git is the source of truth; session is in-memory.
- **S3.** Approved files are left as-is; rejected files reverted via
  `git checkout HEAD -- <path>`.
- **S4.** Per-project config at `<repo>/snapdiff.toml`: snapshot globs,
  axis-extraction regex (named capture groups → axis names), optional base
  ref override, server bind address.
- **S5.** PNG-only in MVP; non-PNG files in globs ignored with warning.
- **S6.** Daemon lazy-spawned inside `snapdiff await`, lingers ~60s after
  finalize, then exits.

## Out of Scope (MVP)

Submit/manifest input, persistent run history, notifications, multi-repo
daemon, comment threading, image annotation, framework-specific adapters,
background lifecycle commands beyond `serve`/`await`, auth code in the binary.

## System Context

```
       ┌─────────────────────────────────────────────────┐
       │                  Reviewer (human)               │
       │     phone / laptop / tablet on tailnet          │
       └──────────────────┬──────────────────────────────┘
                          │ HTTPS via Tailscale Serve
                          ▼
   ┌──────────────────────────────────────────────────────┐
   │  snapdiff daemon (in-process inside `await`)         │
   │  - chi HTTP router  - templ views  - in-mem session  │
   └──────┬───────────────────────────────────┬───────────┘
          │ git subprocess                    │ stdout JSON
          ▼                                   ▼
   ┌──────────────┐                  ┌────────────────────┐
   │  Git repo +  │                  │ Agent (Claude Code)│
   │ working tree │                  │ blocked on `await` │
   └──────────────┘                  └────────────────────┘
```

## Architecture

Single Go binary. Packages under `internal/`:

- **`cmd/snapdiff`** — CLI entrypoint. Subcommands: `await`, `serve`,
  `--version`.
- **`internal/config`** — Loads & validates `snapdiff.toml`.
- **`internal/gitscan`** — Wraps `git diff --name-status` and
  `git show HEAD:<path>`. Returns
  `[]Diff{path, status: added|modified|deleted, baseline []byte, current []byte}`.
- **`internal/imdiff`** — PNG pixel-diff mask via stdlib `image`/`image/png`.
  In-process cache keyed by `(sha256(baseline), sha256(current))`.
- **`internal/review`** — In-memory session: list of `Diff`, per-file verdict
  (`pending|approved|rejected{comment}`), "done" signal channel.
- **`internal/web`** — chi router + templ views. Endpoints:
  - `GET /` — grouped/filterable diff index
  - `GET /diff/{id}` — single diff page with mode switcher
  - `GET /diff/{id}/baseline.png|current.png|pixeldiff.png` — image bytes
  - `POST /diff/{id}/verdict` — `{status, comment?}`
  - `POST /diff/bulk-verdict` — `{filter: {axis: value...}, status, comment?}`
  - `POST /session/finalize` — applies verdicts, signals `await`
  - `GET /healthz`
- **`internal/apply`** — `git checkout HEAD -- <path>` for rejected; no-op
  for approved.
- **`internal/lifecycle`** — Graceful shutdown + ~60s linger after finalize.

**Dependencies** (pure-Go for single-binary cross-compile):
`a-h/templ`, `BurntSushi/toml`, `go-chi/chi/v5`, stdlib for the rest.

## Configuration Schema

See `snapdiff.toml.example`. Axis groups should use `[^/_]+` (not `[^/]+`)
so they don't span underscores or slashes in surrounding path components
(e.g. `__Snapshots__/` directories).

## Data Flow (happy path)

1. Agent runs `snapdiff await` from repo root.
2. `config` loads `snapdiff.toml`. `gitscan` runs git diff against `base_ref`
   scoped by globs; reads baseline via `git show`, current from filesystem;
   builds `[]Diff`.
3. Empty diff → exit 0 immediately with `{"verdicts":[]}` on stdout; no
   daemon spawned.
4. Non-empty → in-process HTTP server starts, review URL printed to stdout,
   `await` blocks on session's "done" channel.
5. Reviewer opens UI on tailnet. Index groups diffs by axis (default groupby
   `test`, axis chips for filtering). Each row: thumbnail + verdict state.
6. Reviewer opens individual diffs, picks one of five modes, applies per-file
   or bulk verdicts.
7. Reviewer clicks Finalize → `POST /session/finalize` validates no `pending`
   remains, `apply` runs `git checkout` on rejected files, verdict JSON
   assembled, "done" signaled.
8. `await` unblocks, writes verdict JSON to stdout, lingers ~60s, exits 0.

**Stdout JSON contract** emitted by `await`:

```json
{
  "verdicts": [
    {"path": "ios/__Snapshots__/Foo__empty__iphone17__dark__en-US.png",
     "status": "approved"},
    {"path": "ios/__Snapshots__/Foo__empty__iphone17__dark__pt-BR.png",
     "status": "rejected",
     "comment": "logo is cropped in pt-BR"}
  ]
}
```

**Edge cases**: not-a-git-repo fails fast; Ctrl-C on `await` shuts server with
no verdicts applied; browser close without finalize keeps session in memory;
non-PNG in glob is ignored with warning.

## V-Model

| Left (spec/design)            | Right (verification)                                      |
|-------------------------------|-----------------------------------------------------------|
| Stakeholder requirements      | Acceptance test — scripted fixture-repo end-to-end        |
| System requirements           | System test — per-requirement fixture scenarios           |
| Architecture (HTTP contract)  | Integration test — wire packages against temp git repos   |
| Module design (interfaces)    | Unit test — per-package coverage                          |
| Implementation                | Code review + `go vet` + `staticcheck` + `golangci-lint`  |

The **acceptance test is the north star**. See `docs/v-model.md` for detail.

## ADR Index

| #   | Title                       | Status   |
|-----|-----------------------------|----------|
| 001 | Agent integration model     | Accepted |
| 002 | Verdict application         | Accepted |
| 003 | Auth posture                | Accepted |
| 004 | Diff discovery source       | Accepted |
| 005 | State management            | Accepted |
| 006 | Verdict granularity         | Accepted |
| 007 | Notifications               | Accepted |
| 008 | Web UI tech                 | Accepted |
| 009 | Language & distribution     | Accepted |
| 010 | Config format & location    | Accepted |
| 011 | Variation axis model        | Accepted |
| 012 | Image format support        | Accepted |
| 013 | Daemon lifecycle            | Accepted |
| 014 | Verification framework      | Accepted |
| 015 | Pixel diff library          | Accepted |
| 016 | HTTP router                 | Accepted |
| 017 | Screenshot tests (Playwright + dogfood) | Accepted |

## Build Order

1. `internal/config` + tests
2. `internal/gitscan` + tests (temp git repo via `t.TempDir()`)
3. `internal/imdiff` + tests (fixture PNG pairs)
4. `internal/review` + tests
5. `internal/web` handlers + templ views + per-mode JS/CSS + tests
6. `internal/apply` + tests (mocked exec)
7. `internal/lifecycle`
8. `cmd/snapdiff` wiring
9. `test/acceptance` chromedp end-to-end
10. `Makefile` + cross-compile matrix

## Deferred

Notifications (ntfy/webhook/web push), SQLite history, framework adapters,
multi-repo, annotation overlay, comment threading. Each gets a new ADR when
reopened.
