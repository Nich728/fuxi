# Fuxi 复习

A Mandarin study app built around two things: an **AI tutor** you can ask about any word or grammar point, and **spaced-repetition reviews** scheduled with FSRS. A one-minute **vocabulary check** estimates your level at sign-up so the app never wastes time on words you already know. Reviews work offline and sync across devices.

Fuxi is also a backend portfolio project: a deterministic scheduling engine, an offline-first sync protocol, and an AI integration that is grounded in a real dictionary, safe and cost-controlled.

- Product requirements: [docs/PRD.md](docs/PRD.md)
- System design: [docs/SYSTEM_DESIGN.md](docs/SYSTEM_DESIGN.md)
- Decisions: [docs/adr/](docs/adr/)

## Status

| Milestone | | |
|---|---|---|
| M0 Foundations | ✅ | Binary modes, Compose, migrations, CI |
| M1 Content & search | ✅ | CC-CEDICT, Tatoeba, HSK 3.0 importers; search API |
| M2 Study core | | Auth, decks, notes, cards, FSRS reviews |
| M3 Daily-usable client | | Installable PWA |
| M4 Vocabulary check + AI tutor | | |
| M5 Offline sync | | |
| M6 Insight · M7 Production polish | | |

## Architecture

A modular monolith ([ADR-0001](docs/adr/0001-modular-monolith.md)): one Go binary, three modes, PostgreSQL 16 as the only stateful dependency.

```
fuxi api        HTTP JSON API (+ SSE for the tutor)
fuxi worker     background jobs on a Postgres queue
fuxi migrate    goose migrations, embedded in the binary
```

## Getting started

Requirements: [Go 1.27+](https://go.dev/dl/), [Docker Desktop](https://www.docker.com/products/docker-desktop/), [Task](https://taskfile.dev) (`winget install Task.Task` or `brew install go-task`).

```bash
task up        # build and start Postgres, migrations, API and worker
```

The API listens on http://localhost:8080 (`/healthz`, `/readyz`).

Load the dictionary (about 30 MB of downloads; set `FUXI_DATA_DIR` to choose where they go):

```bash
task data:download   # CC-CEDICT, the HSK 3.0 syllabus list, Tatoeba
task data:import     # about a minute; running it again does nothing
```

Then try it:

```bash
curl "http://localhost:8080/v1/dict/search?q=ma"        # 吗, 马, 骂 … 妈
curl "http://localhost:8080/v1/dict/search?q=know"      # 知道, 认识 …
curl "http://localhost:8080/v1/dict/entries/{id}?max_hsk=2"   # a word with easy example sentences
```

Search accepts characters (simplified or traditional), pinyin with tone numbers (`ci2dian3`, `lv4`), pinyin without tones (`zhidao`, `zhīdào`) and English. A Chinese phrase that isn't a dictionary word is split into the words it contains: `你今天吃饭了吗` → 你, 今天, 吃饭, 了, 吗.

```bash
task test      # all tests, including integration tests against real Postgres
task lint      # go vet + golangci-lint
task down      # stop the stack
task --list    # every command
```

## Credits

- Dictionary: [CC-CEDICT](https://cc-cedict.org/wiki/), CC BY-SA 4.0. Dictionary data derived from it stays CC BY-SA and is kept separate from the code.
- Example sentences: [Tatoeba](https://tatoeba.org), CC BY 2.0 FR.
- HSK levels: the HSK 3.0 examination syllabus (新版HSK考试大纲, Center for Language Education and Cooperation, 2025), as extracted by [harukicoder/hsk30](https://github.com/harukicoder/hsk30) (MIT).

The app will show these credits too once it has screens (M3).
