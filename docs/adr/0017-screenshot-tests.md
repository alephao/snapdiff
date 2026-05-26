# ADR-0017: Screenshot Tests for the Web UI

- **Status:** Accepted
- **Date:** 2026-05-25

## Context

snapdiff's web UI is dense, dark, and full of small layout details (the
five diff modes, the bulk-action bar, the verdict dock, the topbar
counter strip). A CSS or templ change can silently break any of these
without any existing test catching it — none of the Go tests touch
rendered pixels.

We want a visual-regression gate so future UI changes either match
known-good baselines or get explicit review. And because *snapdiff is
itself a screenshot-diff review tool*, the maintenance flow should
dogfood it: when a baseline legitimately changes, the developer reviews
the diff through snapdiff's own UI.

## Decision

Screenshot tests are implemented with **Playwright** (Node + Chromium)
as an opt-in suite under `tests-screenshots/`. The suite boots the
snapdiff binary against a deterministic temp git repo built by a small
Go helper, drives the UI via headless Chromium, and compares against
committed baseline PNGs at `tests-screenshots/baselines/`.

A `snapdiff.toml` at the repo root points its globs at
`tests-screenshots/baselines/*.png` with an axis regex matching the
`<page>-<scenario>-<viewport>.png` naming convention. When the
developer runs `make screenshots-update`, the changed baselines surface
in `git diff`, and `snapdiff serve` from the repo root puts them in a
review session — the same UX every other consumer of snapdiff gets.

## Consequences

- **Visual regressions caught automatically** at the 26-scenario level
  (index + all 5 diff modes + edge cases × desktop/mobile).
- **Dogfood loop is real.** Every UI change forces the maintainer to
  use snapdiff, surfacing real-world friction.
- **Two prereqs added to make screenshots deterministic** — see
  spec.md: JetBrains Mono is now embedded in the binary (no Google
  Fonts) and a `?noanim=1` query param disables the blinking-cursor
  animation. Both are independently useful.
- **Node + Chromium become test-time dependencies.** Documented in the
  README; `make screenshots-install` bootstraps. Not required for
  `make build` / `make test` / `make acceptance`.

## Alternatives Considered

- **chromedp / rod (pure-Go)**: rejected — would have been more
  consistent with the no-Node ethos, but Playwright's snapshot
  comparison + retry / report ergonomics are noticeably better for a
  comprehensive (~26-baseline) suite, and the test infra is decoupled
  from the shipped binary.
- **Smoke set only (~6 baselines)**: rejected — wouldn't catch
  per-mode regressions (each diff mode has its own JS-driven layout).
- **No dogfood (baselines hidden from snapdiff)**: rejected — defeats
  the obvious win of using our own tool to review our own UI changes.
