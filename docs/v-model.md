# V-Model

snapdiff is engineered with the V-Model: every left-side specification or
design artifact is paired with a right-side verification artifact at the same
abstraction level. As the project grows downward into implementation, the
right side grows upward into verification, meeting in the middle at module
code with its unit tests.

```
   Stakeholder requirements ─────────────── Acceptance test
              │                                    ▲
   System requirements ───────────────────── System test
              │                                    ▲
   Architecture design ────────────────── Integration test
              │                                    ▲
   Module design ──────────────────────── Unit test
              │                                    ▲
              └────────── Implementation ──────────┘
```

## Left side (this section is owned by `docs/`)

| Level                  | Artifact                                   |
|------------------------|--------------------------------------------|
| Stakeholder requirements | `docs/spec.md` § Stakeholder Requirements |
| System requirements    | `docs/spec.md` § System Requirements       |
| Architecture design    | `docs/spec.md` § Architecture + HTTP contract; ADR-008, ADR-013, ADR-016 |
| Module design          | Per-package Go interfaces + package doc comments |
| Implementation         | Go source under `cmd/` and `internal/`     |

## Right side (this section is owned by `test/` and `internal/.../*_test.go`)

| Level                  | Artifact                                   | Location                        |
|------------------------|--------------------------------------------|---------------------------------|
| Acceptance test        | One scripted end-to-end run (chromedp + fixture repo) | `test/acceptance/`     |
| System test            | Per-requirement fixture scenarios          | `test/integration/system_*`     |
| Integration test       | Wire packages together against temp git repos, exercise HTTP | `test/integration/`  |
| Unit test              | Per-package `*_test.go` files              | `internal/<pkg>/*_test.go`      |
| Implementation check   | `go vet`, `staticcheck`, `golangci-lint`, `go build` cross-compile matrix | `Makefile` targets |

## The acceptance test (north star)

A single shell script orchestrates the canonical agent loop:

1. Clone the fixture repo at `test/fixtures/sample-repo/` (a tiny git repo
   with committed PNG snapshots).
2. Mutate a known subset of PNGs in the working tree to simulate the agent
   regenerating snapshots.
3. Start `snapdiff await` against that repo in the background.
4. Drive the web UI via headless chromedp:
   - approve all `theme=dark` diffs via bulk action,
   - approve one specific file individually,
   - reject one specific file with a comment,
   - click Finalize.
5. Wait for `snapdiff await` to exit.
6. Assert:
   - exit code 0,
   - stdout JSON contains exactly the expected verdicts,
   - `git status --porcelain` shows only the approved files as modified
     (rejected file has been reverted to baseline by `git checkout`),
   - daemon process is gone after the linger interval.

If this passes, the system satisfies its stakeholder requirements.

## Requirement-to-test traceability

Each requirement in `docs/spec.md` maps to at least one test:

| Req | Test                                                                   |
|-----|------------------------------------------------------------------------|
| R1  | Acceptance test § Finalize verdicts                                    |
| R2  | Acceptance test § Bulk + per-file actions; `internal/review` unit tests |
| R3  | `internal/gitscan` unit tests § axis regex; `internal/web` index tests |
| R4  | `internal/web` per-mode rendering tests (one per mode); plus the Playwright screenshot suite at `tests-screenshots/` exercising all 5 modes × desktop+mobile (per ADR-0017) |
| R5  | `Makefile` `release` target produces matrix binaries                   |
| R6  | Implicit — no auth code present (verified by code review + grep check) |
| R7  | Acceptance test § `await` stdout JSON contract                         |

| Sys | Test                                                                   |
|-----|------------------------------------------------------------------------|
| S1  | `internal/gitscan` unit tests                                          |
| S2  | Daemon-restart integration test asserts session lost, git untouched    |
| S3  | `internal/apply` unit tests; acceptance test verifies git state        |
| S4  | `internal/config` unit tests for schema, defaults, regex compile       |
| S5  | `internal/gitscan` unit test for non-PNG-in-glob warning + skip        |
| S6  | `internal/lifecycle` unit test for linger; acceptance test for exit    |
