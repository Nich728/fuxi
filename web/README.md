# Fuxi web client

The installable PWA. The Go API serves the production build from the same origin (`FUXI_WEB_DIR`, SYSTEM_DESIGN §10), so one Cloudflare Access application signs people in to both.

```bash
task up         # API and database (from the repository root)
task web:dev    # this app on http://localhost:5173, proxying /v1 to the API
task web:check  # lint, tests, build, and the 150 KB gzipped first-load budget
```

- `src/api`: the API client (`client.ts`), its types, and UUIDv7 ids. An expired Access session is detected (`redirect: 'manual'`) and the browser goes through `/sign-in`, which the service worker never answers from its cache.
- `src/screens`: one file per screen, with its stylesheet. `src/components/ui.tsx` holds the shared pieces (tab bar, sheet, seal week, pinyin).
- `src/index.css`: the design tokens from `.interface-design/system.md`. Change them there first.
- Tests sit next to the code; `src/test/fakeApi.tsx` fakes the API for screen tests.

The app is online-only until offline sync (M5).
