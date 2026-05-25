# ADR-0011: Variation Axis Model

- **Status:** Accepted
- **Date:** 2026-05-25

## Context

Snapshot tests are typically taken across multiple variation axes:

- **State** — empty, few items, many items.
- **Size** — iPhone 17, iPhone 17 Pro Max, web desktop, web mobile.
- **Theme** — light, dark.
- **Language** — en-US, pt-BR, es-ES.
- **Path** (web only) — different routes.

The set of axes varies wildly between projects (an iOS app cares about
device + theme + state; a web app cares about viewport + locale + route).
snapdiff needs a model that doesn't hard-code a vocabulary.

## Decision

Variation axes are extracted via a **named-capture regex** declared in
`snapdiff.toml`, applied to the file path relative to repo root. The
regex's named groups become axis names. Whatever names the regex emits,
the UI uses for grouping and filtering.

Example:
```toml
axis_regex = '(?P<test>[^/]+)__(?P<state>[^_]+)__(?P<device>[^_]+)__(?P<theme>[^_]+)__(?P<lang>[^.]+)\.png'
```

If a file doesn't match the regex, it's still included in the diff list
but has no axis values (group label: "unparsed").

## Consequences

- Zero baked-in vocabulary; iOS/Android/web projects each describe their
  own axes.
- Axes are derived from filenames/paths, which most snapshot frameworks
  already encode that way.
- If a project encodes variation in PNG metadata or a sidecar file, the
  regex model doesn't capture it. Acceptable for MVP; revisit if it bites.

## Alternatives Considered

- **Fixed axis vocabulary**: rejected — wrong for cross-platform.
- **Sidecar JSON per snapshot**: rejected — every framework would need an
  emitter; defeats ADR-0004's framework-agnostic stance.
