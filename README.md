<div align="right"><a href="README.zh-CN.md">中文文档</a></div>

# JNexus — Lightweight Ops Platform

A self-built, lightweight operations platform: host management (multi OS accounts per host), batch command execution with live output, file distribution, script center, release pipeline with rollback, RBAC with configurable roles, audit logging, dangerous-command blocking, LDAP authentication, and per-user **MFA (TOTP two-step verification)**. Data is stored in an external PostgreSQL.

Tech stack: Go (Gin + GORM) + PostgreSQL + Vue3 (Element Plus + xterm.js).

**Docs:** [API reference](docs/API.md) — integrate external systems via API keys (`/api/ext/*`).

**Online docs:** once deployed, open `http://<server>:8080/docs/api` (bilingual, no login required).

## Features

| Module | Capabilities |
|--------|--------------|
| Dashboard | Post-login home: managed/online hosts, groups, tasks, apps, releases, users, active rules + current user info and quick access |
| Hosts | **Multi-level group tree**, import (`NAME,IP,port,user,group`, unified password on the page, auto SSH-key pairing), concurrent connectivity probe, per-host **multiple OS accounts** (labels, default flag), Web Shell |
| Batch Exec | Tree host selection / multi-IP input, **OS account selection**, concurrent execution, WebSocket live per-host output, success/fail counters & progress |
| Task Console | Aggregate task view: host table (account, status, exit code, duration, failed-first), output panel, failed-only filter, cross-host output keyword search, **export summary `.log` / CSV** |
| Scheduled Tasks | Cron-based execution of commands or scripts on selected hosts; enable/disable, run-now, run history, email notification |
| Reports | Preset collection templates — **server accounts**, **crontab listing**, **health check**, **system info**, **ports & certificates** (listening ports, HTTP/HTTPS protocol detection, HTTPS & local certificate expiry) — run across hosts (empty target = all hosts), view per-host results, export `.log` / CSV; module access controlled per role |
| Host Accounts | Standalone **host accounts** page (all accounts across hosts): filters by host/keyword/rotation status, add/edit/rotate/reveal (admin), **password auto-rotation** with configurable length/complexity/period (LDAP accounts auto-skipped), shows last password-change time |
| File Distribution | Upload → concurrent SFTP to many hosts (tree/IP selection), live progress |
| Scripts | CRUD + one-click batch execution |
| Release Center | App → host bindings (with release OS account); pipeline: stop → timestamped backup → upload → start → health check; one-click rollback to latest backup |
| Users | Two tabs: Users + Group Management. Roles: admin / ops / publisher / viewer / **auditor**; user groups link members, hosts, host groups, **OS accounts** and **account rules** (`host scope × username`, auto-covers future hosts). User rows carry audit fields: creator, last modified by/at, disabled at/by. Per-user **MFA status** with admin reset |
| Web Shell | Full-screen terminal workspace: host asset tree expanded by usable OS account, multiple concurrent sessions, per-account connections |
| **Monitoring** | **Host resources** (CPU / memory / disk, SSH-collected every 60s with progress bars and trend sparklines) + **application monitors** (Uptime Kuma style): HTTP(s) (method, accepted status ranges, keyword contain/absent), TCP port, Ping — configurable interval/timeout, Up/Down status, heartbeat bars, 24h uptime %, check-now, pause. Page organized in six tabs: **CMD monitoring** (groupable/filterable), **application monitors**, **Alert configuration** — notification channels (Email via system SMTP, Webhook, WeCom / DingTalk / Feishu bots, Telegram) with test-send, bound per monitor; notifications fire on status changes. **Alert rules** tab: one GLOBAL rule set (failure threshold seconds - 0 alerts immediately - plus recovery-notice toggle) applied to every monitor. **Maintenance windows** tab (calendar date-range + start/end time, cross-midnight supported): in-window downtime excluded from uptime, gray heartbeats, no alerts; every change is recorded in a change log with window content and active status. **Template settings** tab: the global default templates for all five notification sources (monitor alert/recovery, reboot, CMD alert/recovery) are visible and editable with placeholders. Channels support **visual template editing** — Email channels edit title and body templates separately, other channels a message template; placeholders render at send time and test-send previews the result. **CMD severity levels P1-P4**: per-metric CPU/memory/disk thresholds with per-level duration, notification channels and custom message templates (placeholders {level} {host} {ip} {metric} {value} {threshold} {time}); escalation and recovery are announced automatically. **Host system reboots are detected automatically** (boot_id change during resource collection, Linux hosts) and pushed immediately to all enabled channels. Sample retention 30 days; trend drawer offers 6h/24h/7d/30d ranges (hourly aggregation beyond 48h); a **Profile page** (user menu) shows basic info, self-service email/password and an MFA on/off |
| **MFA** | **TOTP two-step verification** (RFC 6238, works with Google/Microsoft Authenticator): self-service enable via QR code in the user menu, 6-digit code required at login after the password step, self-disable with code, admin reset for lost devices; secrets AES-256-GCM encrypted |
| **Kubernetes** | **Multi-cluster onboarding** via pasted kubeconfig (auto-parses API server / CA / client cert, base64 or PEM) or a CA + client-certificate trio; credentials AES-256-GCM encrypted, never echoed back. **Certificate expiry tracking** with red/yellow highlight and automatic reminders to alert channels 30/7 days before expiry; periodic probing shows cluster online status and version. **Per-cluster members** (admin / user / viewer) plus global K8S view/manage role permissions. **Dedicated full-screen management page** (Manage button) with left sidebar: **Overview** (15 resource count cards + recent events), Cluster (Nodes with live CPU/memory usage, Namespaces), **Workloads** (Pods with logs / in-page Shell drawer with multi-container & split screen / delete; Deployments with **scale** and rolling restart; DaemonSets, StatefulSets, Jobs; CronJobs with create/suspend/resume/delete), **Service Discovery** (Services, Ingresses), **Configuration** (ConfigMaps, Secrets), **Storage** (PVCs, PVs, StorageClasses), **Access Control** (ServiceAccounts), **Helm Releases** (chart, version, revision, status — decoded from release secrets). **Read-only YAML view** for every resource; a global namespace selector applies to all tabs; resource usage comes from metrics-server (columns show `-` when absent). Warning-event aggregation alerts (20h cooldown) and NotReady node detection push to alert channels |
| Audit Log | All write operations recorded (who / action / resource / source IP / status), full output retention in tasks; visible to admin & auditor only |
| Dangerous Commands | Regex rule library (rm -rf, mkfs, dd, shutdown, drop database… 11 built-in), blocks at exec/script/release entry points and writes audit; editable & testable by admin |
| Email (SMTP) | SMTP settings (SSL / STARTTLS, auth, masked password), test send; task-completion notification emails with success/fail counts and per-host result table (failed tasks include output snippets) |
| System Settings | Tabs: General (system name), LDAP auth (server, group-membership check, connection test), Email SMTP, **Password Rotation** (global switch, password length / complexity / default period), **API Keys**, Paired Keys, Role Settings (editable description / menu visibility / host permissions / OS-account perms / report access / K8S perms per role) |
| **API Integration** | **API keys** (System Settings → API Keys) let external systems call `/api/ext/*`: hosts with live status/resources, monitors with status & 24h uptime, task results, and async batch exec (`wait` option). Keys are bound to a user and inherit its role & data permissions; SHA-256 hashed (shown once), optional expiry + IP allowlist, enable/disable, per-key rate limit (120/min), every call audited |
| i18n | Chinese / English switcher (top right) |

## Quick Start (Local Development)

### 1. Prepare PostgreSQL

Any external PostgreSQL works, e.g. via Docker:

```bash
docker run -d --name jnexus-pg -e POSTGRES_USER=jnexus -e POSTGRES_PASSWORD=jnexus123 \
  -e POSTGRES_DB=jnexus -p 5432:5432 postgres:16-alpine
```

### 2. Configure the backend

```bash
cd backend
cp config.example.yaml config.yaml
# Generate the AES master key (encrypts SSH keys at rest)
./jnexus-server -genkey   # or: openssl rand -base64 32
# Edit config.yaml: dsn / jwt_secret / aes_key
```

### 3. Run

```bash
# Build & start (auto-creates all tables + seed data)
cd backend && go build -o jnexus-server ./cmd/server && ./jnexus-server -config config.yaml

# Or dev mode with hot frontend
cd frontend && npm install && npm run dev   # http://localhost:5173 proxies to :8080
```

Open http://localhost:8080 — the built frontend (`frontend/dist`) is served by the backend directly.

**Default account: `admin` / `admin123`** (change it immediately).

### 4. Production (Docker Compose)

Bundled database (JWT/AES keys are auto-generated on first start and persisted in the `data` volume; set `JNEXUS_JWT_SECRET` / `JNEXUS_AES_KEY` env vars to override):

```bash
cd deploy
# version is baked at build time (the container has no .git)
export JNEXUS_VERSION="1.$(git rev-list --count HEAD)"
docker compose up -d --build
```

External PostgreSQL — tables are created automatically on startup (only the database itself must exist):

```bash
export JNEXUS_DB_HOST=10.3.0.100
export JNEXUS_DB_PORT=5432
export JNEXUS_DB_USER=jnexus
export JNEXUS_DB_PASSWORD=yourpass
export JNEXUS_DB_NAME=jnexus        # CREATE DATABASE first
export JNEXUS_VERSION="1.$(git rev-list --count HEAD)"   # bake version into the image
docker compose -f docker-compose.external.yml up -d --build
# JWT/AES keys are auto-generated and persisted in the `data` volume;
# set JNEXUS_JWT_SECRET / JNEXUS_AES_KEY to override.
```

Env precedence: `JNEXUS_DSN` > `JNEXUS_DB_HOST/PORT/USER/PASSWORD/NAME` > `config.yaml` (same for the plain binary).

## Usage Guide

1. **SSH access**: import a private key (Hosts → SSH Keys) and put the platform public key on targets, or use password auth. **Batch import** hosts with `NAME,IP,port,user,group`; enter the login password in the page-level "Common password" field and enable **auto key pairing** — the platform pushes its public key to each host and switches to key auth automatically.
2. **OS accounts**: each host can hold multiple accounts (Hosts → OS Accounts). Batch-add an account (e.g. `appuser`) to hundreds of existing hosts via the toolbar **Batch Add Accounts** (password + auto-pair).
3. **Team isolation**: create a user group, link it to OS accounts directly, or add an **account rule** (`host scope × username`, e.g. `All hosts × appuser`). Group members can then only use those accounts — ops team gets `root` via its own rule; app teams never see it.
4. **Batch execution**: pick hosts (tree) or type IPs, optionally pick an OS account, run. Live per-host output; the task console aggregates status and lets you download a summary `.log` or CSV.
5. **Release**: configure an app (hosts + deploy dir + jar name + optional commands/health URL/release account) in Applications, then upload the jar in Release Center. Failed releases roll back to the latest backup with one click.
6. **Scheduled tasks**: create a cron schedule (presets available) to run a command/script on selected hosts; runs are recorded as tasks with history.
7. **Reports**: pick a preset template (accounts / crontab / health / system info / ports & certificates) and target hosts (leave empty to collect all hosts); per-host results are archived and exportable as `.log` / CSV. The ports & certificates report lists listening ports, classifies each as http / https / other (live TLS handshake + HTTP HEAD probe), and flags expired certificates — both those served on HTTPS ports and local certificate files under `/etc/ssl`, `/etc/pki`, etc.
8. **Password rotation**: enable per account (Host Accounts page) with a rotation period; new random passwords are stored encrypted and never displayed. LDAP/domain accounts are detected and skipped automatically.
9. **Roles**: System Settings → Role Settings controls each role's description, visible menus, host permissions (view/create/edit/delete), OS-account management, and report access. Admin is always full.
10. **LDAP**: enable in System Settings, optionally require group membership (`Group Base DN` + filter + allowed groups). LDAP users are auto-created on first login with the configured default role.
11. **MFA (TOTP two-step verification)**: enable per user via the user menu (top right) → **MFA Security**. Scan the QR code with any authenticator app (Google / Microsoft Authenticator etc.), enter a 6-digit code to confirm — afterwards sign-in requires password + dynamic code. Users can disable it themselves (code required); admins can reset a user's MFA from the Users page (e.g. lost device). TOTP secrets are stored AES-256-GCM encrypted.
12. **Monitoring**: open **Monitoring** — the host resources table shows CPU / memory / disk usage per host (click a row for a 6-hour trend), and application monitors watch HTTP(s) endpoints, TCP ports or hosts via ping with interval, timeout, keyword and status-range options; results render as Up/Down status, heartbeat bars and 24h uptime. Check cadence and the whole module can be tuned via the `monitor_interval_sec` / `monitor_enabled` settings.
13. **Email notifications**: configure SMTP in System Settings (with a one-click test send). When enabled, exec / distribute / release / batch-account tasks send a result summary email to the recipients on completion — failed hosts with output snippets are highlighted.
14. **Kubernetes**: add a cluster in **K8S Clusters** (paste a kubeconfig or the CA + client-cert + client-key trio — cert expiry is tracked automatically). Click **Manage** for the full-screen cluster console: overview cards, workloads (Pods / Deployments / CronJobs …), storage, config, Helm releases; pod Shell runs in an in-page drawer. Scale a Deployment or restart it from the row menu; every member only sees clusters they belong to.

## Security Design

- SSH private keys & passwords encrypted at rest with AES-256-GCM; master key injected via config (env overridable).
- TOTP (MFA) secrets encrypted with the same master key; the intermediate `mfa_token` issued after the password step is valid for 2 minutes, is rejected for API access, and only a verified code exchanges it for a real JWT.
- Passwords hashed with bcrypt; JWT valid 24h.
- Dangerous commands blocked at exec/script/release entry points (403 + audit record).
- Audit log masks sensitive fields (password/content).
- Backend enforces authorization regardless of menu visibility (menus are UI-only).

## Project Layout

```
backend/     Go backend (cmd/server entry, internal/ layered)
frontend/    Vue3 SPA (src/views per module, src/i18n zh-CN/en-US)
deploy/      docker-compose.yml (bundled DB) + docker-compose.external.yml + Dockerfile
.smoke/      Local test stubs: fake SSH/SFTP server, fake LDAP server, fake SMTP server, K8S mock apiserver
```

## Known Limitations (MVP)

- SSH host-key verification is skipped (designed for trusted intranets).
- File distribution is single-file granularity.
- Single-instance execution; multi-instance deployment needs a task queue.


## License

[MIT](LICENSE) © 2026 hmilyzhang

<div align="right"><a href="README.zh-CN.md">中文文档</a></div>
