# Deploying Fuxi

Fuxi runs as one Docker Compose stack on one machine, reachable only through Cloudflare: Cloudflare Access signs people in, and a Cloudflare Tunnel carries their requests to the machine, so nothing listens on a public port (ADR-0018, ADR-0020).

```
browser ── https://fuxi.<your domain> ──▶ Cloudflare (DNS, Access sign-in)
                                             │ tunnel (outbound from your machine)
                                             ▼
                      your machine: cloudflared ─▶ api :8080 ─▶ postgres
                                                   worker, backup
```

The first deploy runs on a PC to prove the Cloudflare setup; the long-term home is an Oracle Cloud Always Free Arm VM. The steps are the same on both, apart from setting up the VM.

## 1. Cloudflare (once)

1. **A domain on Cloudflare.** Buy one from Cloudflare Registrar, or buy it elsewhere and switch its nameservers to Cloudflare (**Add a domain**, Free plan). Wait until it shows **Active**.
2. **Login methods.** In the Zero Trust dashboard ([one.dash.cloudflare.com](https://one.dash.cloudflare.com)), under **Settings → Authentication → Login methods** (or **Integrations → Identity providers**), add Google and One-time PIN.
3. **The Access application.** **Access → Applications → Add an application → Self-hosted → Public DNS**:
   - name `Fuxi`, public hostname `fuxi` on your domain, no path;
   - a policy "Allowed people": **Allow**, **Include → Emails** with the addresses allowed in;
   - session duration 1 month (on the application or the policy), so the installed app doesn't ask you to sign in daily.

   Copy its **Application Audience (AUD) Tag** (64 hex characters).
4. **The tunnel.** **Networks → Tunnels → Create a tunnel → Cloudflared**, named `fuxi`. Copy the **token** from the install command it shows (the long string after `--token`); it's a secret. Under **Public hostname** (in newer layouts, **Published application routes**), add `fuxi.<your domain>` with service **HTTP** and URL **`api:8080`**. You don't need to install anything it suggests: the stack runs `cloudflared` itself.

## 2. Run it

On the machine (Docker, Git and [Task](https://taskfile.dev) installed):

```sh
git clone https://github.com/Nich728/fuxi.git && cd fuxi
task data:download        # CC-CEDICT, HSK 3.0, Tatoeba into .data (or FUXI_DATA_DIR)
task prod:init            # creates deploy/production/.env with a random database password
```

Fill in `deploy/production/.env`:

| Setting | Value |
|---|---|
| `FUXI_ACCESS_TEAM_DOMAIN` | `https://<team>.cloudflareaccess.com` (with `https://`) |
| `FUXI_ACCESS_AUD` | the AUD tag |
| `FUXI_OWNER_EMAIL` | your email (optional: its account gets the owner role) |
| `TUNNEL_TOKEN` | the tunnel token (secret) |
| `FUXI_DATA_DIR` | only if the datasets aren't in `.data` |

Then:

```sh
task prod:up              # builds the image, migrates, starts everything (a few minutes the first time)
task prod:import          # loads the dictionary and sentences (about a minute; safe to repeat)
task prod:health          # {"status":"ok"} twice
```

Open `https://fuxi.<your domain>` on your phone: Cloudflare's sign-in page, then Fuxi. Install it to the home screen (Chrome: **Add to Home screen**; Safari: **Share → Add to Home Screen**).

`task prod:up -- api worker backup` starts everything except the tunnel, for checking a machine before it goes live.

## 3. Oracle Cloud Arm VM

1. **Account.** Upgrade to **Pay As You Go**: Always Free resources stay free, and Oracle doesn't reclaim idle instances on paid accounts. Set a budget alert of a few dollars as a safety net.
2. **Instance.** **Compute → Instances → Create**: image **Ubuntu 24.04**, shape **VM.Standard.A1.Flex** (Ampere, Arm) with 2 OCPUs and 12 GB, boot volume 50 GB, and your SSH public key. If Oracle reports no capacity, try another availability domain or try again later. No inbound rules are needed beyond SSH (port 22), which is open by default.
3. **Software**, over SSH:
   ```sh
   sudo apt-get update && sudo apt-get -y upgrade
   curl -fsSL https://get.docker.com | sudo sh
   sudo usermod -aG docker $USER    # then log out and back in
   sudo sh -c "$(curl -fsSL https://taskfile.dev/install.sh)" -- -d -b /usr/local/bin
   sudo apt-get install -y git unattended-upgrades
   ```
4. **Fuxi.** Clone the repository and download the datasets as in step 2. Copy `deploy/production/.env` from your PC (`scp deploy/production/.env ubuntu@<vm-ip>:fuxi/deploy/production/`), then:
   ```sh
   task prod:up -- postgres backup
   ```
5. **Data.** To bring your reviews across, take a backup on the PC (`task prod:backup`), copy the newest file from `deploy/production/backups/` to the same folder on the VM, and run `task prod:restore -- <file>`. To start fresh instead, run `task prod:import`.
6. **Switch over.** Stop the PC's stack (`task prod:down` there), then run `task prod:up` on the VM. The tunnel reconnects from the VM; nothing changes at Cloudflare. Only one machine should run the tunnel at a time, or requests are split between them.

## Day to day

| Task | Command |
|---|---|
| Deploy a new version | `git pull && task prod:up` (rebuilds, migrates, restarts; a few seconds of downtime) |
| Logs | `task prod:logs` or `task prod:logs -- api cloudflared` |
| Containers | `task prod:ps` |
| Back up now | `task prod:backup` (one runs at start-up and then daily; 14 days are kept) |
| Restore | `task prod:restore -- fuxi-YYYYMMDD-HHMMSS.dump` |
| Run a fuxi command | `task prod:fuxi -- migrate status` |
| Stop | `task prod:down` (keeps the database and backups) |

**Copy backups off the machine.** Dumps are in `deploy/production/backups/` on the same disk as the database, so they don't protect against losing the machine. From the PC, now and then: `scp ubuntu@<vm-ip>:fuxi/deploy/production/backups/*.dump D:/fuxi-backups/`.

**Let friends in** by adding their emails to the Access policy. Nothing changes on the server.

**If the tunnel token leaks,** open the tunnel in the Zero Trust dashboard, refresh its token, put the new one in `.env` and run `task prod:up`.
