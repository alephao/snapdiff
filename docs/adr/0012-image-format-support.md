# ADR-0012: Image Format Support

- **Status:** Accepted
- **Date:** 2026-05-25

## Context

Snapshot test outputs are typically PNG. Some frameworks support JPEG or
WebP. snapdiff has to decide which formats to ingest in MVP.

## Decision

**PNG only in MVP.** Files in the configured globs that aren't PNG are
ignored with a warning logged to stderr.

## Consequences

- Decoder/encoder is the stdlib `image/png` — no third-party image deps.
- Covers iOS pointfree SnapshotTesting, Paparazzi, Playwright, Storybook,
  and Jest defaults out of the box.
- Projects using JPEG/WebP snapshot output are not supported until a new
  ADR adds them. Trivially expandable later (stdlib has `image/jpeg`).

## Alternatives Considered

- **PNG + JPEG**: deferred — no concrete demand and JPEG is lossy, which
  makes pixel-diffs noisier.
- **PNG + WebP**: deferred — requires a pure-Go WebP encoder (rare).
