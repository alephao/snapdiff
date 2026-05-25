# ADR-0002: Verdict Application

- **Status:** Accepted
- **Date:** 2026-05-25

## Context

When the reviewer approves or rejects a diffed snapshot file, something has
to move the bits: rejected files must be discarded so the agent can iterate;
approved files must remain so the agent (or human) can commit them.

Options:

- snapdiff owns the files and applies verdicts directly.
- The agent applies verdicts based on snapdiff's JSON output.
- A hybrid: snapdiff applies, governed by per-project configuration.

## Decision

snapdiff applies verdicts itself, governed by `<repo>/snapdiff.toml`. With
the git-driven model (see ADR-0004), "apply" reduces to: leave approved
files alone, run `git checkout HEAD -- <path>` on rejected files. The
per-project config carries the glob(s) and axis regex; no framework-specific
adapter code lives in snapdiff.

## Consequences

- The agent doesn't need any glue script — `snapdiff await` does the work
  and exits with the verdict JSON for situational awareness only.
- snapdiff requires `git` to be available and the working directory to be
  inside a git repo. Already true for the target workflow.
- If a project's snapshots live outside git (rare for snapshot-test
  workflows), snapdiff doesn't help. Acceptable for MVP.

## Alternatives Considered

- **Agent applies**: rejected — pushes work onto every project's test runner
  glue. The whole point of snapdiff is to remove that ceremony from the
  agent loop.
- **snapdiff with hard-coded framework adapters**: rejected for MVP —
  premature. Adapters can be added later if a framework needs more than the
  config-driven model offers.
