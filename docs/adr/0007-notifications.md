# ADR-0007: Notifications

- **Status:** Accepted
- **Date:** 2026-05-25

## Context

When `snapdiff await` is blocked waiting for the reviewer, the reviewer
needs some way to know a review is pending — especially when away from the
machine running the agent.

Options:

- None; reviewer checks the URL the agent printed.
- Pluggable webhook on review-ready.
- Built-in integration with a specific service (ntfy, Pushover, Slack).
- Web Push (browser notifications).

## Decision

**No notification system in MVP.** When `snapdiff await` spawns the daemon,
it prints the review URL to stdout *and* opens the reviewer's default
browser at that URL on the local machine. The agent's transcript (Claude
Code, shell, CI output) remains the canonical signal for remote reviewers.
The browser pop can be suppressed with `SNAPDIFF_NO_BROWSER=1` (used by
the acceptance test and headless contexts). `snapdiff serve` does not pop
the browser — it is invoked interactively, so the user already has a
terminal open. The reviewer can leave the review tab open to receive
updates via in-page polling/SSE later.

## Consequences

- Smaller MVP, no service dependencies, no provider opinion baked in.
- Reviewer at the agent's machine gets a zero-effort surface — the tab
  appears the moment `await` is ready to accept verdicts.
- Reviewer away from their devices won't know a review is pending until
  they next check their agent transcript. Acceptable tradeoff for MVP.
- Adding notifications later is a pure feature addition behind a new ADR;
  no MVP code paths constrain the shape.

## Alternatives Considered

- **Pluggable webhook**: deferred — useful, but premature without a
  concrete user pull.
- **Built-in ntfy.sh**: deferred — couples to one provider.
- **Web Push**: deferred — non-trivial (service worker + VAPID), and the
  open-tab polling pattern covers most of the value.
