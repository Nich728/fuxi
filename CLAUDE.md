# Fuxi: notes for Claude

Fuxi (复习, "to review") is a Mandarin study app: FSRS flashcard reviews, an AI tutor grounded in a real dictionary, and a one-minute vocabulary check. It is also a backend portfolio project, so correctness, tests and documented decisions matter as much as features.

## Read first

- `docs/PRD.md`: what and why. Requirement ids (FR-AI-4, V5, …) are referenced in code and ADRs.
- `docs/SYSTEM_DESIGN.md`: how. **Section numbers are stable**; cite them in comments (`// SYSTEM_DESIGN §6.2`) and keep them stable when editing.
- `docs/adr/`: decisions. New decisions get a new ADR from `0000-template.md`; numbers 0002–0016 are reserved for the decisions already listed in SYSTEM_DESIGN §15.
- `.interface-design/system.md`: the visual design system for the web client (tokens, type, signature elements). Change it there first, then in code.

## Current milestone

M0 Foundations, M1 Content & search and M2 Study core are done. Sign-in is Cloudflare Access (ADR-0018): `internal/identity` verifies the Access JWT and creates users on first sight; locally `FUXI_AUTH=dev` signs requests in as `FUXI_DEV_EMAIL` or the `X-Fuxi-Dev-Email` header. `internal/study` has decks, notes, cards, reviews, the due queue and CSV import; `internal/scheduling` wraps go-fsrs; `internal/jobs` is the Postgres queue. **M3 Daily-usable client** (SYSTEM_DESIGN §14) is in progress, in three PRs: backend additions (`internal/learner` for "I already know this", rating previews, `/study/summary`, the API serving the PWA from `FUXI_WEB_DIR`), the web client in `web/` (both merged or in review), then the first deploy: `deploy/production/compose.yml` with `cloudflared`, on the owner's PC first and then an Oracle Cloud Arm VM (ADR-0020).

## Commands

The owner develops on **Windows**. Use Task, not make (ADR-0017).

| Command | What it does |
|---|---|
| `task up` / `task down` | Start / stop Postgres, migrations, API and worker in Docker |
| `task test` | All tests, including integration tests against real Postgres (needs Docker) |
| `task test:short` | Unit tests only |
| `task run:api` / `task run:worker` | Run the API or worker on the host (dev sign-in) against the Compose Postgres |
| `task lint` | `go vet` + golangci-lint |
| `task migrate -- status` | Run `fuxi migrate` on the host |
| `task migrate:new -- name` | New goose migration in `db/migrations/` |
| `task sqlc` | Regenerate `internal/*/…db` packages from `db/queries/*.sql` (runs sqlc in Docker; it can't build natively here) |
| `task web:dev` | The web client with hot reload on :5173 (proxies `/v1` to the API on :8080) |
| `task web:check` | Web lint (oxlint), tests (Vitest), production build and the 150 KB gzipped first-load budget |
| `task prod:init` / `task prod:up` / `task prod:import` | Production stack in `deploy/production` (ADR-0020, `docs/DEPLOY.md`); settings in the never-committed `deploy/production/.env`. Also `prod:logs`, `prod:health`, `prod:backup`, `prod:restore`, `prod:down` |
| `task data:download` / `task data:import` | Fetch the datasets into `FUXI_DATA_DIR` (set to `D:/fuxi-data` on the owner's PC) and import them |

The local Postgres from Compose is on host port **5433**; the owner has a separate PostgreSQL 17 service on 5432.

## Rules from the design

- **Time is injected.** No business logic calls `time.Now()`; take a `clock.Clock` (`internal/platform/clock`).
- **Package boundaries.** Packages under `internal/` talk through small interfaces; only the owning package queries its tables.
- **Every query is scoped to a user.** Repository functions take a `userID`; no unscoped queries. Every endpoint gets a cross-user authorization case in `cmd/fuxi/api_test.go` (SYSTEM_DESIGN §12.3); the test fails for a route without one.
- **Writes are idempotent.** Client-generated UUIDv7 ids; retries are safe.
- **The review log is append-only.** Never UPDATE or DELETE `review_logs`; card state is derived.
- **AI output never becomes card data** without a dictionary lookup (ADR-0009). AI tools are read-only except Inbox suggestions (ADR-0008).
- **Migrations are never edited after merge.** Add a new one. Each must have a working `-- +goose Down`; `TestUpDownUp` checks it.
- **Postgres first.** No Redis or Kafka without a measurement that says so.

## Layout

`cmd/fuxi` (the binary, router wiring, API tests) · `internal/platform` (config, clock, db, hlc, httpx, logging, migrate, principal, testdb) · `internal/dictionary` (+ `pinyin`, generated `dictdb`) · `internal/identity` · `internal/study` · `internal/scheduling` · `internal/jobs` · `internal/learner` (each with a generated `…db` package) · `db/migrations` (embedded goose SQL) · `db/queries` (sqlc) · `deploy/` (Dockerfile, Compose) · `web/` (PWA, from M3) · `docs/`.

Integration tests call `testdb.Start(t, true)` for a migrated throwaway Postgres. Bulk imports use pgx `CopyFrom` into temp staging tables; read queries go through sqlc.

## Style

- Go: standard library first, `chi` for routing, `pgx` + `sqlc` for SQL, `slog` for logs. Table-driven tests; `testcontainers-go` for anything that touches Postgres.
- Errors: wrap with context (`fmt.Errorf("load deck: %w", err)`); API errors are RFC 9457 problem+json.
- Web (`web/`): TypeScript, plain CSS on the design tokens (`src/index.css` mirrors `.interface-design/system.md`), Radix only for hard primitives (dialogs). Client-generated UUIDv7 ids for every write. Tests next to the code (`*.test.ts[x]`, Vitest + Testing Library, `src/test/fakeApi.tsx` fakes the API).
- Docs and UI copy: plain, sentence case.
