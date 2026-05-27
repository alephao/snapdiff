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
  fork) and opens the reviewer's default browser at the bound URL as soon
  as the listener is up (see ADR-0007; suppressible via
  `SNAPDIFF_NO_BROWSER=1`). On finalize, the server exits **immediately**
  so the agent's verdict JSON returns without delay; no linger window
  applies in `await` mode.
- **`snapdiff serve`** is an explicit alternative for ad-hoc review without
  an agent loop. It runs until interrupted (Ctrl-C). It does **not** open
  the browser — the user invoked it interactively and already has the URL
  in their terminal. The configured `linger_seconds` applies on shutdown
  so the browser can render the success state.

The bind address comes from `snapdiff.toml` (`[server].bind`); the linger
window is configurable too (and only effective in `serve`).

## Consequences

- Zero install/management overhead beyond dropping the binary on PATH.
- `await` returns verdicts to the agent the instant the reviewer hits
  Finalize. The reviewer's browser tab will see a one-shot network error
  on its next request — acceptable because the human work is already
  done by that point.
- In `serve`, the reviewer's browser tab survives finalize by
  `linger_seconds`, enough to display "done — verdicts sent to agent"
  before the server evaporates.
- An agent that loops fast (e.g., fix, regenerate, `await` again) gets a
  fresh daemon each iteration. Acceptable; startup is cheap.

## Alternatives Considered

- **Always-on daemon**: rejected for MVP — adds install ceremony.
  Reopenable behind a new ADR if reviewers want a single Tailscale URL
  for ad-hoc browsing.
- **Linger in `await` too**: rejected — the agent is blocked on stdout,
  so any post-finalize delay is a delay in the agent's next iteration.
  `serve` keeps linger because no agent is waiting on it.
