# snapdiff

> Human-in-the-loop review for agent-driven screenshot-test workflows.

[![CI](https://github.com/alephao/snapdiff/actions/workflows/ci.yml/badge.svg)](https://github.com/alephao/snapdiff/actions/workflows/ci.yml)
[![Release](https://img.shields.io/github/v/release/alephao/snapdiff?sort=semver)](https://github.com/alephao/snapdiff/releases)
[![Go](https://img.shields.io/github/go-mod/go-version/alephao/snapdiff)](go.mod)
[![License: MIT](https://img.shields.io/badge/license-MIT-blue.svg)](LICENSE)

When an AI agent implements a UI change, it runs your screenshot tests,
regenerates the snapshots that failed, and leaves you with a working tree full
of diffed PNGs. `snapdiff` sits between the agent and you: the agent blocks on a
single CLI command, you triage the diffs in a web UI — approve, or reject with a
comment — and the command exits with a JSON verdict per file that the agent can
act on (commit, iterate, move on).

What makes it small and predictable:

- **Git-driven.** Diffs are discovered by comparing the working tree against
  `HEAD` (or a configured base ref). No manifest, no framework adapters.
- **Git is the state.** No database, no persistence layer. Approved files are
  left dirty for you to commit; rejected files are reverted with `git checkout`.
- **No auth code.** The binary trusts the network it's on — front it with
  Tailscale or Cloudflare (see [Remote access](#remote-access)).
- **Single static binary.** Pure Go, cross-compiled, zero runtime dependencies.
- **Framework-agnostic.** It only cares about PNG files matching globs you
  configure — works with any test framework that writes PNG snapshots.

## Install

### Pre-built binary (recommended)

Download the archive for your platform from the
[latest release](https://github.com/alephao/snapdiff/releases/latest),
verify it against `checksums.txt`, extract `snapdiff`, and put it on your `PATH`.
Builds are published for macOS, Linux, and Windows (amd64/arm64; Windows amd64
only).

### With `go install`

```sh
go install github.com/alephao/snapdiff/cmd/snapdiff@latest
```

### From source

```sh
make build      # produces ./snapdiff
```

Building from source needs the `templ` code generator — see
[CONTRIBUTING.md](CONTRIBUTING.md) for the full setup.

## Configure

Place a `snapdiff.toml` at your repo root. Minimal example:

```toml
[snapshots]
# Globs for snapshot PNG files, relative to repo root.
globs = ["**/__Snapshots__/*.png"]

# Named capture groups become axis names used for grouping and bulk actions
# in the review UI. Axis names are free-form.
axis_regex = '(?P<test>[^/_]+)__(?P<state>[^/_]+)__(?P<device>[^/.]+)\.png'
```

| Field                    | Required | Default         | Meaning                                                        |
| ------------------------ | -------- | --------------- | -------------------------------------------------------------- |
| `snapshots.globs`        | yes      | —               | Globs (relative to repo root) selecting snapshot PNGs.         |
| `snapshots.axis_regex`   | yes      | —               | Regex whose named groups become filter/bulk-action axes.       |
| `snapshots.base_ref`     | no       | `HEAD`          | Git ref the working tree is diffed against.                    |
| `server.bind`            | no       | `0.0.0.0:7777`  | Address the web UI binds to.                                   |
| `server.linger_seconds`  | no       | `60`            | Seconds `serve` stays up after Finalize so the tab can render. |

See [`snapdiff.toml.example`](snapdiff.toml.example) for the fully annotated
version.

## Usage

### In an agent loop

After your agent has run tests and regenerated snapshots:

```sh
snapdiff await
```

`await` scans for diffs and:

- If there are none, it prints `{"verdicts": []}` and exits immediately.
- Otherwise it starts the review server, prints the review URL to **stderr**,
  and blocks. Open the URL, triage the diffs, and hit **Finalize**.

On Finalize the command applies your verdicts, prints the verdict JSON to
**stdout**, and exits — so an agent can capture stdout while the URL on stderr
goes to the human:

```json
{
  "verdicts": [
    { "path": "tests/__Snapshots__/Login__empty__iphone.png", "status": "approved" },
    { "path": "tests/__Snapshots__/Login__error__iphone.png", "status": "rejected", "comment": "logo is cropped" }
  ]
}
```

### Ad-hoc review (no agent)

```sh
snapdiff serve
```

Same UI, but it doesn't emit verdict JSON and lingers for `linger_seconds`
after Finalize instead of exiting instantly.

### Browse baselines

```sh
snapdiff gallery
```

A read-only browser for every baseline PNG in the working tree — no review
session, no verdicts applied.

### Flags & environment

All three commands accept:

- `--repo <dir>` — repo directory (default: current directory)
- `--config <path>` — path to `snapdiff.toml` (default: `<repo>/snapdiff.toml`)

Set `SNAPDIFF_NO_BROWSER` to stop `await`/`gallery` from auto-opening a browser.

## The review UI

- **Five diff modes** per file: side-by-side, swipe, toggle, pixel-diff overlay,
  and onion.
- **Axis grouping & bulk actions.** The named groups from `axis_regex` (e.g.
  `device`, `theme`, `lang`) become chips you can filter by and act on in bulk —
  every action still records one verdict per file.
- **Per-file verdicts.** Approve, or reject with an optional comment.
- **Finalize** applies everything: approved files are left dirty in the working
  tree for you to `git commit`; rejected files are reverted (newly-added files
  are deleted, modified/deleted files are restored via
  `git checkout <base_ref> -- <path>`).

## Remote access

`snapdiff` ships with no auth code. Front it with one of:

- **Tailscale Serve** (recommended) — devices on your tailnet reach the UI by
  identity, no public exposure. See
  [tailscale.com/kb/1242/tailscale-serve](https://tailscale.com/kb/1242/tailscale-serve).
- **Cloudflare Access** — public hostname gated by your IdP at the tunnel edge.

## How it works

`snapdiff` runs `git diff --name-status <base_ref>` (plus `git ls-files` for
untracked files) to find changed PNGs, and `git show <base_ref>:<path>` to fetch
each baseline. The review session lives entirely in memory for the lifetime of
the command; git is the only durable state. See [`docs/spec.md`](docs/spec.md)
for the design rationale and [`docs/adr/`](docs/adr/) for the decision log.

## Contributing

Contributions are welcome. See [CONTRIBUTING.md](CONTRIBUTING.md) for dev
prerequisites, the repo layout, build/test commands, and how design decisions
are recorded as ADRs.

## Status

Early / pre-1.0. The core loop — git-driven diff discovery, the five-mode review
UI, bulk actions, and JSON verdicts — is implemented and covered by unit,
integration, and acceptance tests. Deliberately **out of scope** for now:
persistent run history, notifications, a multi-repo daemon, in-binary auth, and
non-PNG image formats. See [`docs/spec.md`](docs/spec.md) for the full V-Model
breakdown of scope and verification.

## License

[MIT](LICENSE) © Aleph Retamal
