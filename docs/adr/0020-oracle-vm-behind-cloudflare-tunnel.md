# ADR-0020: Host on one Oracle Cloud Arm VM behind a Cloudflare Tunnel, instead of Fly.io

| | |
|---|---|
| **Status** | Accepted |
| **Date** | 2026-11-28 |
| **Related** | SYSTEM_DESIGN §3, §13, §14 (M3); ADR-0001, ADR-0002, ADR-0018 |

## Context

SYSTEM_DESIGN planned Fly.io for production: the API and worker as two processes, with managed Postgres, and AWS (ECS Fargate and RDS) as an optional later move. Two things changed before the first deploy.

- **Sign-in moved to Cloudflare Access (ADR-0018).** The app's domain must be on Cloudflare, and the origin should be reachable only through Cloudflare. A Cloudflare Tunnel does that without opening any inbound port, from any machine that can make outbound connections.
- **Cost.** Fuxi has one main user and a few friends, and no budget beyond the AI. Fly.io's managed Postgres and always-on machines cost money each month. Oracle Cloud's Always Free tier includes Arm (Ampere A1) compute of up to 4 OCPUs and 24 GB of memory with 200 GB of block storage, which is far more than Fuxi needs (about 125,000 dictionary entries, 90,000 sentences and one user's reviews: a database of tens of megabytes). The owner already runs a Discord bot on Oracle, so the console and its quirks are familiar.

The app was built for this shape from the start: one binary in three modes (ADR-0001), Postgres as the only stateful dependency (ADR-0002), and a Compose file that runs everything.

## Decision

Production is **one VM running Docker Compose** (`deploy/production/compose.yml`): Postgres, a one-shot migration, the API (which also serves the PWA), the worker, a nightly `pg_dump` service, and `cloudflared`. The tunnel connects out to Cloudflare, and its public hostname (`fuxi.<domain>`) forwards to `http://api:8080` inside the Compose network. Nothing listens on a public port; the API's port is bound to loopback for checks on the machine itself. The API still verifies every Access token itself (ADR-0018), so a request that somehow skipped Cloudflare is refused.

The image is **built on the machine** from the repository, so the same Compose file works on an x86 PC and on the Arm VM; every base image is published for both. Settings live in `deploy/production/.env`, which is never committed.

**The first deploy runs on the owner's PC**, to prove the domain, Access and the tunnel with nothing else new. Fuxi then moves to an **Oracle Always Free Arm VM** on a Pay As You Go account: the free shapes stay free, and paid accounts aren't subject to Oracle reclaiming idle Always Free instances. Moving machines needs no change at Cloudflare: the tunnel reconnects from the new machine.

## Alternatives considered

- **Fly.io, as planned.** Simple deploys and managed Postgres, but a monthly cost for an always-on database and machines, and its edge would sit between users and Cloudflare for no benefit once Access is in front.
- **Vercel or another serverless platform.** Built for stateless functions: no long-running worker for CSV imports, no Postgres of its own, and request time limits that don't suit streamed tutor answers (M4).
- **A paid VPS (for example Hostinger).** The same shape as this decision, at a monthly price, with no advantage for this load.
- **A managed free Postgres (Neon, Supabase) with the app elsewhere.** Free tiers pause or cap databases, and splitting the app from its database adds latency and a second account, for a database that fits easily on the VM.
- **Open ports with a reverse proxy (Caddy) and a public IP.** Works, but exposes the origin to the internet, needs firewall rules and certificates, and lets traffic reach the server without passing Access. The tunnel removes all three.
- **Only the owner's PC.** Free and already working, but Fuxi would be down whenever the PC is off or asleep.

## Consequences

- **Hosting costs nothing** while usage stays within the Always Free limits, and the domain is the only bill.
- **Operations are ours:** OS and Docker updates, disk space, and watching the containers. `restart: unless-stopped` and Docker's restart on boot cover crashes and reboots; M7 adds dashboards and alerts.
- **One machine is a single point of failure.** Acceptable for v1. Daily dumps make the data recoverable, but a dump on the VM's own disk doesn't survive losing the VM, so backups must also be copied off the machine (to the owner's PC now; Cloudflare R2 is a natural later step).
- **Releases are `git pull` and `task prod:up`:** Compose rebuilds the image, runs migrations first and restarts the API and worker, with a few seconds of downtime. That's fine for one user; zero-downtime deploys can wait until there's a reason.
- **AWS stays possible** as a portfolio exercise: the image and the Compose services map directly onto ECS tasks and RDS.
