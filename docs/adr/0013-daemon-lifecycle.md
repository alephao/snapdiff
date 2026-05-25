# ADR-0013: Daemon Lifecycle

- **Status:** Accepted
- **Date:** 2026-05-25

## Context

snapdiff needs an HTTP server to serve the review UI, but the user
shouldn't have to install a launchd/systemd unit just to use it.

Options:

- Always-on daemon (system service).
- Spawned by `snapdiff await`, dies when `await` exits.
- Hybrid: spawned by `await`, lingers briefly after finalize so the browser
  can show a success state.

## Decision

- **`snapdiff await`** starts the HTTP server **in-process** (no subprocess
  fork). On finalize, the server stays up for `linger_seconds` (default
  60), then exits, taking the `await` process with it.
- **`snapdiff serve`** is an explicit alternative for ad-hoc review without
  an agent loop. It runs until interrupted (Ctrl-C).

The bind address comes from `snapdiff.toml` (`[server].bind`); the linger
window is configurable too.

## Consequences

- Zero install/management overhead beyond dropping the binary on PATH.
- The reviewer's browser tab survives finalize by `linger_seconds`,
  enough to display "done — verdicts sent to agent" before the server
  evaporates.
- An agent that loops fast (e.g., fix, regenerate, `await` again) gets a
  fresh daemon each iteration. Acceptable; startup is cheap.

## Alternatives Considered

- **Always-on daemon**: rejected for MVP — adds install ceremony.
  Reopenable behind a new ADR if reviewers want a single Tailscale URL
  for ad-hoc browsing.
- **No linger**: rejected — browser would see a network error on finalize
  before it could render the success state.
