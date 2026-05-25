# ADR-0008: Web UI Tech

- **Status:** Accepted
- **Date:** 2026-05-25

## Context

The review UI needs to render:

- An index of diffs grouped/filterable by free-form variation axes.
- A per-diff page with five visualization modes (side-by-side, swipe,
  toggle, pixel-diff overlay, onion).
- Form posts for verdicts (single and bulk) and finalize.

Frontend options:

- Go-rendered HTML + HTMX, plus small amount of vanilla JS for diff modes.
- Embedded SPA (React/Svelte) built with Vite, shipped via `go:embed`.
- Native desktop app (Wails/Tauri-Go) — but the requirement is remote
  access via Tailscale, which needs a server.

## Decision

Go binary serves type-safe templates via **`a-h/templ`** + **HTMX** for
interactivity + vanilla CSS/JS for the diff-mode mechanics (slider,
toggle, opacity bar). The pixel-diff overlay PNG is generated server-side
and served as a regular image (see ADR-0015). No JS build pipeline.

`templ` is preferred over `html/template` because template errors are
caught at compile time (one `templ generate` step is acceptable; pure-Go).

## Consequences

- `go build` (after `templ generate`) is the entire frontend pipeline.
  Single binary stays single binary.
- No Node, no npm/pnpm, no bundler.
- Rich interaction beyond what HTMX + small JS can do becomes awkward.
  If the UI grows ambitious, revisit with a new ADR for a SPA.

## Alternatives Considered

- **`html/template`**: rejected — runtime template errors are an
  unnecessary papercut when `templ` exists.
- **React/Svelte SPA**: rejected — disproportionate toolchain for the
  amount of UI logic involved.
- **Wails/Tauri**: rejected — defeats the remote-access requirement.
