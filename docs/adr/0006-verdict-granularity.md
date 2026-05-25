# ADR-0006: Verdict Granularity

- **Status:** Accepted
- **Date:** 2026-05-25

## Context

A typical agent loop regenerates many snapshot files at once: one screen
rendered across multiple devices, themes, languages, and states. The
reviewer needs efficient triage.

Options:

- Verdict per snapshot file.
- Verdict per snapshot, plus bulk actions across an axis filter.
- Verdict per region within a snapshot (e.g., "approve header, reject footer").

Region-level verdicts are powerful but ill-fit to the git-driven model:
rejection happens at the file level (git checkout is whole-file), so
region rejection collapses back to file rejection for the agent's purposes.

## Decision

The primitive is **per snapshot file**: each file gets one verdict
(`approved` or `rejected{comment?}`). The UI offers **bulk actions across
an axis filter** (e.g., "approve all `theme=dark` for `MyView`"), but
those still record one verdict per affected file under the hood.

## Consequences

- Verdict storage and JSON output stay flat and simple.
- Reviewers can blow through 50-file diffs caused by one design change in
  seconds via bulk actions.
- No region-selection UI to build, store, or send back to the agent.

## Alternatives Considered

- **File-only, no bulk**: rejected — would make routine reviews tedious.
- **Region-level**: rejected — doesn't compose with file-atomic git
  operations and adds nontrivial UI complexity.
