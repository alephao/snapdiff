# ADR-0009: Language and Distribution

- **Status:** Accepted
- **Date:** 2026-05-25

## Context

The stakeholder requirement is a single cross-compiled binary with no
external runtime dependencies, so the reviewer can drop it on any of their
machines (laptop, home server, work laptop). The reviewer cares about iOS,
Android, and web frontend workflows.

## Decision

Implement in **Go**, using only pure-Go dependencies. Cross-compile via
`go build` to the matrix: `darwin/amd64`, `darwin/arm64`, `linux/amd64`,
`linux/arm64`, `windows/amd64`. No CGO.

## Consequences

- `make release` produces all five binaries from a single host.
- No need for `xgo`, Docker cross-build images, or platform-specific runners.
- Some otherwise-attractive libraries (anything using CGO for image
  processing, SQLite via the C driver, etc.) are off-limits. The stdlib
  `image`/`image/png` covers our needs; pure-Go alternatives exist for
  everything else we need.

## Alternatives Considered

- **Rust**: comparable single-binary story but slower iteration speed for
  this kind of glue/web tool; team familiarity is with Go.
- **Node + pkg**: rejected — bloats the binary and adds a runtime.
- **Python + PyInstaller**: rejected — same issue, plus slower startup.
