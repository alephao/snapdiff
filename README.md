# snapdiff

Human-in-the-loop review tool for agent-driven screenshot test workflows.

## What it does

When an AI agent regenerates failing screenshot tests in your repo, you end up
with a working tree full of diffed PNGs. `snapdiff` gives you a web UI to
triage them — approve, or reject with a comment — and feeds the verdicts back
to the agent so it can iterate.

## How it works

`snapdiff` is git-driven. It compares the working tree against `HEAD` (or a
configured base ref), scoped to PNG files matching globs in your
`snapdiff.toml`. Approved files are left alone; rejected files are reverted
via `git checkout HEAD -- <path>`. There is no submit step, no manifest, no
persistence layer — git is the state.

## Quick start

```sh
# 1. Write a snapdiff.toml in your repo (see snapdiff.toml.example).
# 2. Have your agent run tests, regenerate snapshots.
# 3. Then:
snapdiff await
# Open the printed URL, review diffs, hit Finalize.
# The command exits with JSON verdicts on stdout the agent can consume.
```

For ad-hoc review without an agent:

```sh
snapdiff serve
```

## Remote access

`snapdiff` ships with no auth code. Front it with one of:

- **Tailscale Serve** (recommended) — devices on your tailnet reach the UI by
  identity, no public exposure. See
  [tailscale.com/kb/1242/tailscale-serve](https://tailscale.com/kb/1242/tailscale-serve).
- **Cloudflare Access** — public hostname gated by your IdP at the tunnel edge.

See `docs/spec.md` for design rationale and `docs/adr/` for the decision log.

## Build

```sh
make build      # local binary
make test       # unit + integration
make release    # cross-compile matrix
```

## Status

Pre-MVP. See `docs/spec.md` for the V-Model breakdown of scope and verification.
