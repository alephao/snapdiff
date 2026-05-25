# ADR-0005: State Management

- **Status:** Accepted
- **Date:** 2026-05-25

## Context

A review tool naturally accumulates state: which runs are pending, which
verdicts have been cast, which images belong to which run, what comments
were left. The straightforward implementation is a database (SQLite is the
obvious single-binary choice) plus disk storage for images.

But with ADR-0004 (git-driven discovery), every piece of that state has a
canonical source already:

- "Pending runs" = uncommitted snapshot diffs in git.
- "Baseline images" = blobs at `git show HEAD:<path>`.
- "Current images" = files in the working tree.
- "Verdicts" = ephemeral; once the agent receives them, they're acted on.

Persisting any of this just creates an opportunity for drift between
snapdiff's view and git's view.

## Decision

snapdiff has **no persistence layer**. Git is the source of truth. Session
state (the list of diffs, per-file verdicts, the "done" signal channel)
lives in process memory for the lifetime of a `snapdiff await` invocation.

## Consequences

- Crash recovery is trivial: restart, re-read git, you're current.
- Zero schema migrations, zero database file management.
- A daemon crash mid-review loses in-progress verdicts (not yet finalized).
  The working tree is unchanged, so the reviewer just starts over.
- History (which files you approved last Tuesday) lives only in `git log`.
  That's the right place for it.

## Alternatives Considered

- **SQLite + files on disk**: rejected — duplicates git, adds maintenance,
  no clear win for a personal HITL tool.
- **In-memory + JSON snapshot to disk**: rejected — saves little, costs
  some, encourages drift.
