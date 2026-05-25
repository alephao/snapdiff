# ADR-0015: Pixel Diff Library

- **Status:** Accepted
- **Date:** 2026-05-25

## Context

Two of the five diff modes (pixel-diff overlay and onion) require the
server to produce a derived PNG from the baseline + current pair. Pixel
diff is the more demanding of the two: walk each pixel, mark the changed
ones in a highlight color.

Options:

- Stdlib `image` + `image/png`, hand-rolled pixel walk.
- Third-party library (e.g., a Go port of pixelmatch).
- Defer to client-side JavaScript pixel diffing.

## Decision

Use **stdlib `image` and `image/png`** with a hand-rolled pixel-diff
implementation. Cache the resulting PNG in an in-process map keyed by
`(sha256(baseline), sha256(current))` so the first request pays the cost
and subsequent ones are O(1).

## Consequences

- Zero new dependencies, keeps ADR-0009's pure-Go-only stance.
- Pixel-diff is small, easy to test, and easy to swap if a smarter
  algorithm (anti-aliasing tolerance, perceptual diff) becomes desirable.
- Cache memory grows with unique diff pairs in a session. Acceptable for
  a single-user tool; bounded by session lifetime.
- Cache is in-process only (per ADR-0005); fresh daemon = fresh cache.

## Alternatives Considered

- **Third-party pixelmatch port**: rejected — adds a dependency for code
  we can write in ~50 lines.
- **Client-side diffing**: rejected — bigger JS surface, slower on phones,
  no caching, can't be cached server-side.
