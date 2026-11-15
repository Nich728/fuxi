# ADR-0018: Use Cloudflare Access for sign-in instead of passwords and invite codes

| | |
|---|---|
| **Status** | Accepted |
| **Date** | 2026-11-14 |
| **Related** | SYSTEM_DESIGN §4.1, §10, §12.1, §13; PRD U1, U2, U5, U6, U8, U9, U10, FR-U-2, FR-U-4; ADR-0015 |

## Context

The design had Fuxi run its own accounts: argon2id passwords, 15-minute access JWTs, rotating refresh tokens per device, and invite codes so only friends could register (ADR-0015). That is a fair amount of security-sensitive code (hashing, token rotation and reuse detection, reset by email later) for an app with one main user and a handful of friends.

Cloudflare Access (part of Cloudflare Zero Trust, free for up to 50 users) can sit in front of the app's domain. It shows its own login page, signs people in with Google or a one-time code sent by email, and forwards only signed-in requests, each carrying a JWT signed by the team's keys. Who may sign in is a policy listing allowed email addresses.

The client is an installable PWA (PRD U3), so a web login page in front of the same origin works on Android and on iPhones. The one open risk is an iPhone home-screen app losing the Access cookie more often than Safari does; it will be tested on a friend's iPhone early in M3.

## Decision

Sign-in is Cloudflare Access. The API verifies the `Cf-Access-Jwt-Assertion` header (or the `CF_Authorization` cookie) on every request under `/v1` except the shared dictionary: RS256 only, signature against the team's published keys (refetched when an unknown key id appears, at most once a minute), issuer equal to the team domain, audience equal to the application's AUD tag, `exp`/`nbf` with one minute of leeway, and a non-empty email. The origin checks this itself, so reaching it directly without Cloudflare gives 401.

A `users` row is created the first time a valid token arrives for a new email. The **Access policy's email list replaces invite codes**: adding a friend means adding their address in the Zero Trust dashboard. `FUXI_OWNER_EMAIL` gives that address the owner role when its account is created.

Devices stay in Fuxi: the PWA generates a UUIDv7 once, registers it with `PUT /v1/devices/{id}`, and sends it with every review. They are needed for the review log now and for sync in M5.

For local development and tests, `FUXI_AUTH=dev` signs every request in as `FUXI_DEV_EMAIL`, or as the email in the `X-Fuxi-Dev-Email` header. The API logs a warning at start-up in this mode. Compose and the Taskfile default to it; the binary defaults to `cloudflare` and refuses to start without `FUXI_ACCESS_TEAM_DOMAIN` and `FUXI_ACCESS_AUD`.

## Alternatives considered

- **Own accounts, as designed (§12.1).** Works the same on every device and keeps the API independent of Cloudflare, but means owning password storage, token rotation, reset emails and the invite flow. Kept as the fallback if the iPhone test fails; the `Authenticator` interface is where it would plug in.
- **"Sign in with Google" built into Fuxi (OIDC).** Avoids passwords but still needs sessions, refresh, and an allow-list, and excludes friends without Google accounts. Access gives Google and email codes for less code.
- **A hosted auth service (Auth0, Clerk, Supabase Auth).** More features than needed, another account and SDK, and free tiers that change.

## Consequences

- No passwords, refresh tokens or invite codes in Fuxi: `invites`, `refresh_tokens` and `users.password_hash` are gone from the schema, and there are no `/auth/*` endpoints. PRD U1, U2, U6, U8, U9 and FR-U-4 are met by Access (sign-in by Google or emailed code, session length set in the Access application, allow-list instead of codes). U10's "create invite codes" becomes "add an email to the policy".
- Signing out is `/cdn-cgi/access/logout` on the app's domain. Logging out another device remotely (U5, FR-U-2) is done in the Zero Trust dashboard (revoke the user's sessions); `DELETE /v1/devices/{id}` only forgets the device in Fuxi.
- The app needs its own domain on Cloudflare (DNS there), and in production the origin should only be reachable through Cloudflare (for example a Cloudflare Tunnel). The JWT check stays regardless.
- Fuxi depends on Cloudflare for sign-in. If Access is down, the app can't be reached anyway, since it is in front of the origin.
- Offline review (M5) is unaffected: the PWA reviews from IndexedDB and syncs once Access lets it through again.
- Revisit if the iPhone home-screen test in M3 shows frequent re-logins, or if Fuxi grows past Access's free 50 users.
