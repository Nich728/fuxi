# ADR-0001: Build Fuxi as a modular monolith

| | |
|---|---|
| **Status** | Accepted |
| **Date** | 2026-11-12 |
| **Related** | SYSTEM_DESIGN §1, §2 |

## Context

Fuxi is built by one person at about 8–10 hours a week, for a handful of users at first. It still has several distinct areas (identity, dictionary, study, scheduling, sync, tutor, learner level, jobs, stats) that should stay separable, and it is a portfolio piece, so the boundaries should be visible in the code.

## Decision

Ship one Go binary, `fuxi`, with three modes: `api`, `worker` and `migrate`. Each area is a package under `internal/`. Packages talk to each other through small Go interfaces and never query each other's tables; only the owning package touches its tables. PostgreSQL is the only stateful dependency.

## Alternatives considered

- **Microservices.** Separate deploys, network calls and distributed transactions for no benefit at this scale; much slower to build alone.
- **One package with no boundaries.** Fastest to start, but the areas would leak into each other and the design would be hard to show or change later.

## Consequences

- One build, one deploy, one set of logs; transactions can span modules where the design needs it (for example a review and its `changes` row).
- Boundaries are enforced by convention and review rather than by the network. A package that needs another's data must go through its interface.
- If one area ever needs to scale independently, its package boundary is the seam to split along.
