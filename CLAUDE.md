# Fuxi: notes for Claude

Fuxi (复习, "to review") is a Mandarin study app: FSRS flashcard reviews, an AI tutor grounded in a real dictionary, and a one-minute vocabulary check. It is also a backend portfolio project, so correctness, tests and documented decisions matter as much as features.

## Read first

- `docs/PRD.md`: what and why. Requirement ids (FR-AI-4, V5, …) are referenced in code and ADRs.
- `docs/SYSTEM_DESIGN.md`: how. **Section numbers are stable**; cite them in comments (`// SYSTEM_DESIGN §6.2`) and keep them stable when editing.
- `docs/adr/`: decisions. New decisions get a new ADR from `0000-template.md`; numbers 0002–0016 are reserved for the decisions already listed in SYSTEM_DESIGN §15.
- `.interface-design/system.md`: the visual design system for the web client (tokens, type, signature elements). Change it there first, then in code.

## Current milestone

M0 Foundations and M1 Content & search are done: `internal/dictionary` imports CC-CEDICT, the HSK 3.0 syllabus and Tatoeba, and serves `/v1/dict/search` and `/v1/dict/entries/{id}`. Next is **M2 Study core** (auth, invites, decks, notes, cards, FSRS, reviews; SYSTEM_DESIGN §4.1, §4.3, §6, §12.1, §14).

## Commands

The owner develops on **Windows**. Use Task, not make (ADR-0017).

| Command | What it does |
|---|---|
| `task up` / `task down` | Start / stop Postgres, migrations, API and worker in Docker |
| `task test` | All tests, including integration tests against real Postgres (needs Docker) |
| `task test:short` | Unit tests only |
| `task lint` | `go vet` + golangci-lint |
| `task migrate -- status` | Run `fuxi migrate` on the host |
| `task migrate:new -- name` | New goose migration in `db/migrations/` |
| `task sqlc` | Regenerate `internal/*/…db` packages from `db/queries/*.sql` (runs sqlc in Docker; it can't build natively here) |
| `task data:download` / `task data:import` | Fetch the datasets into `FUXI_DATA_DIR` (set to `D:/fuxi-data` on the owner's PC) and import them |

The local Postgres from Compose is on host port **5433**; the owner has a separate PostgreSQL 17 service on 5432.

## Rules from the design

- **Time is injected.** No business logic calls `time.Now()`; take a `clock.Clock` (`internal/platform/clock`).
- **Package boundaries.** Packages under `internal/` talk through small interfaces; only the owning package queries its tables.
- **Every query is scoped to a user.** Repository functions take a `userID`; no unscoped queries. Every endpoint gets a cross-user authorization test (SYSTEM_DESIGN §12.3).
- **Writes are idempotent.** Client-generated UUIDv7 ids; retries are safe.
- **The review log is append-only.** Never UPDATE or DELETE `review_logs`; card state is derived.
- **AI output never becomes card data** without a dictionary lookup (ADR-0009). AI tools are read-only except Inbox suggestions (ADR-0008).
- **Migrations are never edited after merge.** Add a new one. Each must have a working `-- +goose Down`; `TestUpDownUp` checks it.
- **Postgres first.** No Redis or Kafka without a measurement that says so.

## Layout

`cmd/fuxi` (the binary) · `internal/platform` (config, clock, db, httpx, logging, migrate, testdb) · `internal/dictionary` (+ `pinyin`, generated `dictdb`) · `db/migrations` (embedded goose SQL) · `db/queries` (sqlc) · `deploy/` (Dockerfile, Compose) · `web/` (PWA, from M3) · `docs/`.

Integration tests call `testdb.Start(t, true)` for a migrated throwaway Postgres. Bulk imports use pgx `CopyFrom` into temp staging tables; read queries go through sqlc.

## Style

- Go: standard library first, `chi` for routing, `pgx` + `sqlc` for SQL, `slog` for logs. Table-driven tests; `testcontainers-go` for anything that touches Postgres.
- Errors: wrap with context (`fmt.Errorf("load deck: %w", err)`); API errors are RFC 9457 problem+json.
- Docs and UI copy: plain, sentence case.
