# ADR-0016: HTTP Router

- **Status:** Accepted
- **Date:** 2026-05-25

## Context

The web server needs a router with:

- Path parameters (`/diff/{id}/baseline.png`).
- Method-scoped handlers (`GET` / `POST`).
- Composable middleware (logging, request ID).
- Pure-Go, single-binary-friendly.

Options:

- Stdlib `net/http` (Go 1.22+ pattern routing).
- `chi`, `gorilla/mux`, `echo`, `fiber`, etc.

## Decision

Use **`github.com/go-chi/chi/v5`**: minimal, idiomatic, no reflection
magic, pure-Go.

## Consequences

- Familiar API for any Go web dev; trivial to test.
- One small dependency. Pure-Go preserves cross-compile matrix.
- Could be swapped to stdlib `net/http` later if dependency surface
  matters more — handlers are written against `http.HandlerFunc`.

## Alternatives Considered

- **Stdlib `net/http` with 1.22+ patterns**: viable; rejected because
  the chi router groups + middleware story is meaningfully nicer for the
  size of API we have (~8 endpoints + image handlers + healthz).
- **Echo / Fiber / Gin**: rejected — heavier, more opinionated.
- **Gorilla/mux**: rejected — stale-ish; chi is its modern peer.
