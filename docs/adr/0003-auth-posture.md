# ADR-0003: Auth Posture

- **Status:** Accepted
- **Date:** 2026-05-25

## Context

The reviewer needs to access the snapdiff UI from outside their home
network (phone, laptop on the road). The UI can mutate the repo working
tree (via `git checkout` on reject), so unauthenticated public exposure
is unacceptable.

Options:

- Bake auth into snapdiff (session cookies, bearer tokens, OAuth, etc.).
- Delegate auth to a network layer (Tailscale identity, Cloudflare Access).
- Hybrid: ship optional shared-token auth as a fallback.

## Decision

snapdiff ships with **no auth code in MVP**. It binds to a configurable
address (default `0.0.0.0:7777`) and assumes the operator fronts it with:

- **Tailscale Serve** (recommended) — devices on the user's tailnet reach
  the UI by Tailscale identity; no public exposure.
- **Cloudflare Access** (alternative) — public hostname gated by the user's
  IdP at the Cloudflare tunnel edge.

The README documents both. The binary itself stays focused on review.

## Consequences

- snapdiff has zero auth-related code paths, sessions, or storage. Smaller
  attack surface, smaller binary, less to test.
- Operating snapdiff requires the user to set up Tailscale (or another
  zero-trust front). One-time cost per machine.
- Local-LAN access is unauthenticated. Acceptable for a single-user
  personal tool on a trusted machine; documented.

## Alternatives Considered

- **Username/password**: rejected — adds login UI, session management, and
  password reset workflows for a single-user tool.
- **Static bearer token in env**: rejected — leaks easily into bookmarks
  and shell history; weaker than network-layer identity.
- **OAuth / passkeys**: rejected — heavy for a personal tool; the network
  layer already provides equivalent identity.
