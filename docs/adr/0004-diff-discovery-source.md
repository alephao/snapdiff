# ADR-0004: Diff Discovery Source

- **Status:** Accepted
- **Date:** 2026-05-25

## Context

snapdiff needs to know which snapshot files have changed and what the
"before" image is for each one. Options:

- Agent submits a manifest pointing at baseline + new + diff PNGs.
- snapdiff parses framework-specific test output to discover diffs.
- snapdiff reads the git working tree, treating committed files as baseline.

The first two require the agent (or a per-framework adapter) to produce
structured input. The third trades framework knowledge for git knowledge —
and the user's workflow already uses git.

## Decision

Diff discovery is **git-driven**. snapdiff runs `git diff --name-status
<base_ref> -- <globs>` to enumerate changed snapshot files, reads the
baseline from `git show <base_ref>:<path>`, reads the current bytes from
the working tree. There is no submit step and no input manifest.

The base ref defaults to `HEAD`; it is configurable in `snapdiff.toml`.

## Consequences

- snapdiff is framework-agnostic: any test runner that writes PNG snapshots
  into the repo works without special integration.
- Workflow is dead simple: agent regenerates snapshots, runs
  `snapdiff await`, snapdiff sees what changed.
- snapdiff requires `git` on PATH and the working dir inside a git repo.
- Snapshots not tracked in git aren't comparable. If a project keeps
  baselines elsewhere, snapdiff doesn't help. Acceptable for MVP.

## Alternatives Considered

- **Manifest-driven submit**: rejected for MVP — every framework needs an
  emitter and snapdiff grows a state/persistence layer to track runs.
  Reopen later only if a framework's data shape can't be derived from git.
- **Framework adapters**: rejected for MVP — premature plugin surface.
