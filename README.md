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
| M2 Study core | ✅ | Sign-in via Cloudflare Access, decks, notes, cards, FSRS reviews, due queue, CSV import |
| M3 Daily-usable client | | Installable PWA |
| M3.5 Practice | | Exercises checked by the app: typed answers, tones, cloze, sentence order ([ADR-0019](docs/adr/0019-suggested-ratings-for-exercises.md)) |
| M4 Vocabulary check + AI tutor | | Includes AI-checked translation and "use it" exercises |
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

### Study through the API

Locally, sign-in is replaced by dev mode (`FUXI_AUTH=dev`): every request is signed in as `dev@fuxi.localhost`, or as the email in an `X-Fuxi-Dev-Email` header. Ids for anything you create are UUIDs you generate, so retries are safe.

```bash
curl -X PUT  localhost:8080/v1/devices/0193a000-0000-7000-8000-000000000001 -d '{"name": "laptop"}'
curl -X POST localhost:8080/v1/decks -d '{"id": "0193a000-0000-7000-8000-00000000d001", "name": "HSK 1"}'
curl -X POST localhost:8080/v1/decks/0193a000-0000-7000-8000-00000000d001/notes \
     -d '{"id": "0193a000-0000-7000-8000-00000000a001", "entry_id": 123}'   # an id from /v1/dict/search
curl localhost:8080/v1/study/queue                                          # due and new cards
curl -X POST localhost:8080/v1/reviews -d '{"id": "<new uuid>", "card_id": "<from the queue>",
     "rating": 3, "reviewed_at": "2026-11-14T09:00:00Z", "device_id": "0193a000-0000-7000-8000-000000000001"}'
curl -X POST "localhost:8080/v1/decks/0193a000-0000-7000-8000-00000000d001/import?import_id=<new uuid>" \
     --data-binary @vocab.csv                                               # hanzi,pinyin,meaning per line → {"job_id": 1}
curl localhost:8080/v1/jobs/1
```

### Sign-in in production

Fuxi doesn't store passwords. [Cloudflare Access](https://developers.cloudflare.com/cloudflare-one/policies/access/) (free for up to 50 users) sits in front of the app's domain and signs people in with Google or an emailed code; the API verifies the token Access attaches to every request ([ADR-0018](docs/adr/0018-cloudflare-access-for-sign-in.md)).

1. Put the domain on Cloudflare. In Zero Trust, add Google and/or One-time PIN as login methods.
2. Create a self-hosted Access application for the domain, with a policy that allows the emails you invite.
3. Run the API with `FUXI_AUTH=cloudflare`, `FUXI_ACCESS_TEAM_DOMAIN=https://<team>.cloudflareaccess.com`, `FUXI_ACCESS_AUD=<the application's AUD tag>`, and optionally `FUXI_OWNER_EMAIL`.
4. Expose the origin only through Cloudflare, for example with a Cloudflare Tunnel.

To invite a friend, add their email to the policy. To sign a device out remotely, revoke the user's sessions in Zero Trust.

### Tests and other commands

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
