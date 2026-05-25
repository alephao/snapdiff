# ADR-0001: Agent Integration Model

- **Status:** Accepted
- **Date:** 2026-05-25

## Context

snapdiff sits between an AI coding agent (Claude Code, Cursor, etc.) and a
human reviewer. The agent needs a way to submit a batch of regenerated
snapshot diffs and receive a verdict per file, while the human reviews them
in a web UI. Several integration shapes are possible:

- Blocking CLI commands the agent runs in a shell loop.
- A local HTTP/JSON API the agent calls.
- An MCP server the agent connects to.
- A filesystem drop directory the daemon watches.

Agents driving snapdiff already operate in a shell context and orchestrate
their work through `git`, test runners, and other CLIs. Any model beyond a
plain CLI adds transport complexity (auth, ports, polling, retry) that the
agent has to manage.

## Decision

The agent integration surface is a single blocking CLI command:
`snapdiff await`. It exits when the reviewer hits Finalize, writing a JSON
verdict per file to stdout. No HTTP, no MCP, no file-drop directory.

## Consequences

- The agent integration is trivially scriptable from any shell, regardless
  of which AI tool runs it.
- snapdiff has no agent-facing network surface to secure or version.
- Remote agents (running on a different host from the reviewer) are not
  supported in MVP. Reopening this requires a new ADR.

## Alternatives Considered

- **HTTP/JSON API**: rejected — agents would need an HTTP client, the daemon
  would need auth even for local use, and the surface duplicates the human UI.
- **MCP server**: rejected — couples to MCP-capable agents and bakes an MCP
  transport into a tool that otherwise has none.
- **Filesystem drop**: rejected — partial writes, stale state, no
  backpressure, no exit signal for the agent.
