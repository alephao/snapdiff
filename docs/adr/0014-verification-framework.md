# ADR-0014: Verification Framework

- **Status:** Accepted
- **Date:** 2026-05-25

## Context

The project is being engineered V-Model-style with ADRs for every decision.
The right side of the V (verification) needs an explicit shape: what tests
exist at what level, and what evidence convinces us the system works.

## Decision

Five verification levels, one per left-side artifact:

| Left                       | Right                                |
|----------------------------|--------------------------------------|
| Stakeholder requirements   | Acceptance test (fixture-repo E2E)   |
| System requirements        | System test (per-requirement scenarios) |
| Architecture               | Integration test (wired components)  |
| Module design              | Unit test (per-package)              |
| Implementation             | `go vet` + `staticcheck` + `golangci-lint` |

The **acceptance test is the north star**: a single chromedp-driven script
that exercises the canonical agent loop against a fixture git repo.
Passing acceptance = system satisfies its stakeholder requirements.

See `docs/v-model.md` for the requirement-to-test traceability matrix.

## Consequences

- Every requirement has a documented test owner; nothing falls through.
- The acceptance test doubles as living documentation of how the agent
  loop is meant to look.
- New requirements come with a new test at the corresponding level (and,
  if architecturally novel, a new ADR).

## Alternatives Considered

- **Unit tests only**: rejected — wouldn't catch the integration bugs that
  cost a HITL tool its credibility (e.g., a verdict applied to the wrong
  file).
- **Only an acceptance test**: rejected — too slow a feedback loop for
  module-level work.
