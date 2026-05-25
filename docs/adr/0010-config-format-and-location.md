# ADR-0010: Config Format and Location

- **Status:** Accepted
- **Date:** 2026-05-25

## Context

snapdiff needs per-project configuration: snapshot file globs, the
axis-extraction regex, an optional base git ref override, and the server
bind address. Format and location options:

- TOML / YAML / JSON / HCL / Go-style.
- At repo root vs in `.snapdiff/` vs in `~/.config/snapdiff/`.

## Decision

**TOML** at `<repo>/snapdiff.toml`. Parser: `github.com/BurntSushi/toml`.

## Consequences

- Config lives where every other repo-level tool (Cargo, pyproject, etc.)
  expects: at the root, in TOML.
- Per-project config travels with the project in git, so a fresh clone
  works without out-of-band setup.
- TOML's strict types catch common errors (a glob written as a bare
  string vs a list) at parse time.
- No machine-level user config in MVP. If we add it later (e.g., default
  bind address per user), it's a strict overlay.

## Alternatives Considered

- **YAML**: rejected — whitespace-significance and Norway problems.
- **JSON**: rejected — no comments, weak ergonomics.
- **HCL**: rejected — overkill, niche parser.
- **`.snapdiff/config.toml`**: rejected — extra directory for one file.
