<div align="right"><a href="README.zh-CN.md">中文文档</a></div>

# JNexus — Lightweight Ops Platform

A self-built, lightweight operations platform: host management (multi OS accounts per host), batch command execution with live output, file distribution, script center, release pipeline with rollback, RBAC with configurable roles, audit logging, dangerous-command blocking, LDAP authentication, and per-user **MFA (TOTP two-step verification)**. Data is stored in an external PostgreSQL.

Tech stack: Go (Gin + GORM) + PostgreSQL + Vue3 (Element Plus + xterm.js).

**Docs:** [Project documentation](docs/README.md) — full guide by module, roadmap, and the [API reference](docs/API.md) for external integration via API keys (`/api/ext/*`).

**Online docs:** once deployed, open `http://<server>:8080/docs/api` (bilingual, no login required).

## Features

| Module | Capabilities |
|--------|--------------|
| Dashboard | Post-login home: managed/online hosts, groups, tasks, apps, releases, users, active rules + current user info and quick access |
| Assets | **Multi-level group tree**, CSV import (`NAME,IP,port,user,group`, unified password on the page, auto SSH-key pairing), concurrent connectivity probe, **batch move to group**, per-host **multiple OS accounts** (labels, default flag); deleting a host removes its OS accounts; **cloud asset sync** from AWS / Azure / Huawei Cloud (AK/SK or service principal, per-account regions, scheduled sync, tag-based grouping, same-IP conflict handling, auto-removal, optional account template so imported hosts come with a ready default OS account); **Windows hosts**: WinRM (PowerShell) command execution, CIM metrics collection, SMB file distribution, password rotation via Set-LocalUser, and **in-browser RDP** via guacamole-lite + guacd gateway — an account picker appears when a host has several usable accounts (DOMAIN\user and UPN logons supported) |
| Batch Exec | Tree host selection / multi-IP input, **OS account selection**, concurrent execution, WebSocket live per-host output, success/fail counters & progress |
| Task Console | Aggregate task view: host table (account, status, exit code, duration, failed-first), output panel, failed-only filter, cross-host output keyword search, **export summary `.log` / CSV** |
| Scheduled Tasks | Cron-based execution of commands or scripts on selected hosts; enable/disable, run-now, run history, email notification |
| Reports | Preset collection templates — **server accounts**, **crontab listing**, **health check**, **system info**, **ports & certificates** (listening ports, HTTP/HTTPS protocol detection, HTTPS & local certificate expiry) — run across hosts (empty target = all hosts; **dual-version presets**: bash over SSH on Linux, PowerShell over WinRM on Windows, dispatched by host OS), view per-host results, **merged HTML export** (all hosts in one printable document) and the cross-host **ports & certificates matrix**, plus `.log` / CSV export; cross-host **account matrix** for the accounts preset; module access controlled per role |
| Account Credentials | All host OS accounts in one place: filters by host/keyword/rotation status, add/edit/rotate/reveal (admin), **password auto-rotation** with configurable length/complexity/period — normal accounts rotate via a root/NOPASSWD-sudo **privilege chain** on the same host; **password history** (admin, audited, last 24 per account); **account templates** store LDAP/AD passwords once for reuse when adding hosts or bulk importing; batch add/delete with reference protection |
| File Distribution | Upload → concurrent SFTP to many hosts (tree/IP selection), live progress |
| Scripts | CRUD + one-click batch execution; **dual-version scripts** (bash + PowerShell tabs) dispatched per host OS — Windows hosts without a PowerShell variant are skipped; dangerous-command interception covers both versions |
| Release Center | App → host bindings (with release OS account); pipeline: stop → timestamped backup → upload → start → health check; one-click rollback to latest backup |
| Users | Two tabs: Users + Group Management. Roles: admin / ops / publisher / viewer / **auditor**; user groups link members, hosts, host groups, **OS accounts** and **account rules** (`host scope × username`, auto-covers future hosts). User rows carry audit fields: creator, last modified by/at, disabled at/by. Per-user **MFA status** with admin reset. **Custom roles**: create roles beyond the six built-ins by copying an existing role and editing its capability matrix (module × action); roles are enforced on every API route. **Team data-level access**: user groups bind members + host groups + OS accounts + **applications** — members only see and release their team's apps, and optional host-visibility restriction limits their host list to the bound groups |
| Web Shell | Full-screen terminal workspace (Linux/SSH): **session tabs atop the terminal**, host asset tree mirrors the hosts page (ungrouped node, greyed Windows hosts launch RDP), every usable account listed under its host, auto-refit on layout changes, multiple concurrent sessions |
| **Monitoring** | **Host resources** (CPU / memory / disk, SSH-collected every 60s with progress bars and trend sparklines) + **application monitors** (Uptime Kuma style): HTTP(s) (method, accepted status ranges, keyword contain/absent), TCP port, Ping — configurable interval/timeout, Up/Down status, heartbeat bars, 24h uptime %, check-now, pause. **HTTPS certificate lifecycle** for https targets: expiry captured on every probe with a remaining-days badge (green/yellow/red) and tiered alerts (default 30-day warn / 7-day critical) to the monitor's bound channels. Page organized in six tabs: **CMD monitoring** (groupable/filterable), **application monitors**, **Alert configuration** — notification channels (Email via system SMTP, Webhook, WeCom / DingTalk / Feishu bots, Telegram) with test-send, bound per monitor; notifications fire on status changes. **Alert rules** tab: one GLOBAL rule set (failure threshold seconds - 0 alerts immediately - plus recovery-notice toggle) applied to every monitor. **Maintenance windows** tab (calendar date-range + start/end time, cross-midnight supported): in-window downtime excluded from uptime, gray heartbeats, no alerts; a live banner shows whether a window is in effect right now; every change is recorded in a change log with current-config/superseded tags (admins can delete or clear records). **AI Diagnostics** tab (admin): on disk/mem/cpu alerts the host state is gathered automatically (SSH for Linux, WinRM for Windows) and analyzed by AI, results pushed to the channels bound to that level; app-monitor downtime triggers server-side network probes (DNS / TCP / HTTP certificate / ping), appending SSH gather when the target matches a managed host, delivered to the monitor's channels; effective levels/metrics, cooldown and diagnosis prompt configurable; disk alerts may run admin-defined controlled cleanup commands (verbatim, fully audited). **Template settings** tab: the global default templates for all five notification sources (monitor alert/recovery, reboot, CMD alert/recovery) are visible and editable with placeholders. Channels support **visual template editing** — Email channels edit title and body templates separately, other channels a message template; placeholders render at send time and test-send previews the result. **CMD severity levels P1-P4**: per-metric CPU/memory/disk thresholds with per-level duration, notification channels and custom message templates (placeholders {level} {host} {ip} {metric} {value} {threshold} {time}); escalation and recovery are announced automatically. **Host system reboots are detected automatically** (boot_id change during resource collection, Linux hosts) and pushed immediately to all enabled channels. Sample retention 30 days; trend drawer offers 6h/24h/7d/30d ranges (hourly aggregation beyond 48h); a **Profile page** (user menu) shows basic info, self-service email/password and an MFA toggle |
| **Observability (OpenObserve)** | Optional integration with [OpenObserve](https://openobserve.ai) (open source, AGPL-3.0): host metrics, task execution output, alert events, **Windows event logs** and the **database audit trail** are **dual-written** for long-term storage and full-text search — the lightweight built-in monitoring keeps working even with OpenObserve down. **Log Search menu**: stream picker (built-ins auto-listed, custom streams auto-discovered), SQL query editor with per-stream field hints, 1h/24h/7d/30d + custom time ranges, dynamic-column result table, CSV export. **Observability admin page** (System Admin): connection health with latency probe, per-stream enable toggles + push statistics (pushed/failed/last push/last error), custom push tester and the external push API guide. **Custom ingestion**: `POST /api/ext/oo/{stream}` (API-key auth, audited, rate-limited) lets external systems push JSON into any stream; accounts stay 100% JNexus — end users never touch the OpenObserve UI; the menu auto-hides while the integration is disabled |
| MFA | **TOTP two-step verification** (RFC 6238, works with Google/Microsoft Authenticator): self-service enable via QR code in the user menu, 6-digit code required at login after the password step, self-disable with code, admin reset for lost devices; secrets AES-256-GCM encrypted |
| **Kubernetes** | **Multi-cluster onboarding** via pasted kubeconfig (auto-parses API server / CA / client cert, base64 or PEM) or a CA + client-certificate trio; credentials AES-256-GCM encrypted, never echoed back. **Certificate expiry tracking** with red/yellow highlight and automatic reminders to alert channels 30/7 days before expiry; periodic probing shows cluster online status and version. **Per-cluster members** (admin / user / viewer) plus global K8S view/manage role permissions. **Dedicated full-screen management page** (Manage button) with left sidebar: **Overview** (15 resource count cards + recent events), Cluster (Nodes with live CPU/memory usage, Namespaces), **Workloads** (Pods with logs / in-page Shell drawer with multi-container & split screen / delete; Deployments and StatefulSets with **scale**, rolling restart; DaemonSets, StatefulSets, Jobs; CronJobs with create/suspend/resume/delete), **Service Discovery** (Services, Ingresses), **Configuration** (ConfigMaps, Secrets), **Storage** (PVCs, PVs, StorageClasses), **Access Control** (ServiceAccounts), **Helm Releases** (chart, version, revision, status — decoded from release secrets). **YAML view with edit (audited, name/namespace guard) and download** for every resource; live-follow pod logs streamed over WebSocket (logs drawer); a global namespace selector applies to all tabs; resource usage comes from metrics-server (columns show `-` when absent). **Capacity planning** tab: 15-min usage sampling retained 400 days, 30d/180d/1y trend charts (usage vs capacity with 80% line) and linear-regression forecast estimating days-to-capacity; Warning-event aggregation alerts (20h cooldown) and NotReady node detection push to alert channels. **Monthly operations report** tab: capacity red/yellow/green risk lights, per-host and per-cluster capacity forecasts, alert statistics with P1/P2 timeline, config-change summary — printable to PDF, alert details exportable as CSV |
| **Web Apps (PAM)** | Vault web consoles (URL + username/password, AES encrypted). Open = a full-screen headless-browser session: the server auto-fills the vaulted credentials, signs in and streams the page live with mouse/wheel/keyboard relayed - the password never reaches the user. Session starts audited, 30-minute lifetime cap, per-role menu authorization |
| **Database Workbench** | Register MySQL / SQL Server / PostgreSQL sources (AES-encrypted credentials, shared with OpenObserve ingestion). Web SQL editor with **schema-aware completion** (tables/columns/keywords, CodeMirror 6, theme-adaptive), result grid with type badges and row numbers, CSV export, query history; per-source read-only mode, dangerous-SQL interception, statement timeout and row cap; multiple accounts per source with per-group usage authorization; every statement audited |
| Audit Log | All write operations recorded (who / action / resource / source IP / status), full output retention in tasks; visible to admin & auditor only |
| Dangerous Commands | Regex rule library (rm -rf, mkfs, dd, shutdown, drop database… 11 built-in), blocks at exec/script/release entry points and writes audit; editable & testable by admin |
| Email (SMTP) | SMTP settings (SSL / STARTTLS, auth, masked password), test send; task-completion notification emails with success/fail counts and per-host result table (failed tasks include output snippets) |
| System Settings | Tabs: General (system name), LDAP auth (server, group-membership check, connection test), Email SMTP, **Password Rotation** (global switch, password length / complexity / default period), **API Keys**, Paired Keys (platform public key + **rotation config**: toggle / interval / rotate-now, background execution with audit; paginated paired-credential list with keyword search), Role Settings (editable description / menu visibility / host permissions / OS-account perms / report access / K8S perms per role), **Observability** (OpenObserve URL / org / encrypted credentials + connection test) |
| **API Integration** | **API keys** (System Settings → API Keys) let external systems call `/api/ext/*`: hosts with live status/resources, monitors with status & 24h uptime, task results, async batch exec (`wait` option), and **custom data push into OpenObserve** (`POST /ext/oo/{stream}`). Keys are bound to a user and inherit its role & data permissions; SHA-256 hashed (shown once), optional expiry + IP allowlist, enable/disable, per-key rate limit (120/min), every call audited |
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
# version: no setup needed - the repo VERSION file (currently 2.x) is baked into the image
# to override: export JNEXUS_VERSION="2.x" before building
docker compose up -d --build
```

Optional observability backend ([OpenObserve](https://openobserve.ai), single container, off by default):

```bash
docker compose --profile observability up -d --build
# then in JNexus: System Settings → Observability → enable, URL http://openobserve:5080,
# org "default", credentials = the ZO_ROOT_USER_EMAIL / ZO_ROOT_USER_PASSWORD you configured.
# The Log Search menu appears automatically once the integration is enabled and reachable.
```

#### Reverse proxy (HTTPS) — WebSocket support required

If JNexus is served behind a reverse proxy with HTTPS, the proxy must also forward
**WebSocket upgrades** — the Web Shell, live task output, log tail and the RDP gateway
proxy (`/rdp-gw`) all use WebSocket. nginx example:

```nginx
location / {
    proxy_pass http://127.0.0.1:8080;
    proxy_http_version 1.1;
    proxy_set_header Upgrade $http_upgrade;      # required: WebSocket upgrade
    proxy_set_header Connection "upgrade";       # required: WebSocket upgrade
    proxy_set_header Host $host;
    proxy_set_header X-Forwarded-Proto https;    # tells JNexus the page is HTTPS
    proxy_read_timeout 3600s;
}
```

Without `Upgrade`/`Connection` forwarding, login and pages work but RDP/Web Shell disconnect immediately.

External PostgreSQL — tables are created automatically on startup (only the database itself must exist):

```bash
export JNEXUS_DB_HOST=10.3.0.100
export JNEXUS_DB_PORT=5432
export JNEXUS_DB_USER=jnexus
export JNEXUS_DB_PASSWORD=yourpass
export JNEXUS_DB_NAME=jnexus        # CREATE DATABASE first
export JNEXUS_VERSION="2.0"   # optional: bake exact version (VERSION file is the fallback)
docker compose -f docker-compose.external.yml up -d --build
# JWT/AES keys are auto-generated and persisted in the `data` volume;
# set JNEXUS_JWT_SECRET / JNEXUS_AES_KEY to override.
```

Env precedence: `JNEXUS_DSN` > `JNEXUS_DB_HOST/PORT/USER/PASSWORD/NAME` > `config.yaml` (same for the plain binary).

## Usage Guide

1. **SSH access**: import a private key (Assets → SSH Keys) and put the platform public key on targets, or use password auth. **Batch import** hosts with `NAME,IP,port,user,group`; enter the login password in the page-level "Common password" field and enable **auto key pairing** — the platform pushes its public key to each host and switches to key auth automatically.
2. **OS accounts**: each host can hold multiple accounts (Assets → OS Accounts). Batch-add an account (e.g. `appuser`) to hundreds of existing hosts via the toolbar **Batch Add Accounts** (password + auto-pair).
3. **Team isolation**: create a user group, link it to OS accounts directly, or add an **account rule** (`host scope × username`, e.g. `All hosts × appuser`). Group members can then only use those accounts — ops team gets `root` via its own rule; app teams never see it.
4. **Batch execution**: pick hosts (tree) or type IPs, optionally pick an OS account, run. Live per-host output; the task console aggregates status and lets you download a summary `.log` or CSV.
5. **Release**: configure an app (hosts + deploy dir + jar name + optional commands/health URL/release account) in Applications, then upload the jar in Release Center. Failed releases roll back to the latest backup with one click.
6. **Scheduled tasks**: create a cron schedule (presets available) to run a command/script on selected hosts; runs are recorded as tasks with history.
7. **Reports**: pick a preset template (accounts / crontab / health / system info / ports & certificates) and target hosts (leave empty to collect all hosts); per-host results are archived and exportable as `.log` / CSV. All hosts' results can be merged into one printable HTML document, and the ports & certificates preset adds a cross-host ports-and-certificates matrix. The ports & certificates report lists listening ports, classifies each as http / https / other (live TLS handshake + HTTP HEAD probe), and flags expired certificates — both those served on HTTPS ports and local certificate files under `/etc/ssl`, `/etc/pki`, etc.
8. **Password rotation**: enable per account (Account Credentials page) with a rotation period; new random passwords are stored encrypted and never displayed. LDAP/domain accounts are detected and skipped automatically.
9. **Roles**: System Settings → Role Settings controls each role's description, visible menus, host permissions (view/create/edit/delete), OS-account management, and report access. Admin is always full.
10. **LDAP**: enable in System Settings, optionally require group membership (`Group Base DN` + filter + allowed groups). LDAP users are auto-created on first login with the configured default role.
11. **MFA (TOTP two-step verification)**: enable per user via the user menu (top right) → **MFA Security**. Scan the QR code with any authenticator app (Google / Microsoft Authenticator etc.), enter a 6-digit code to confirm — afterwards sign-in requires password + dynamic code. Users can disable it themselves (code required); admins can reset a user's MFA from the Users page (e.g. lost device). TOTP secrets are stored AES-256-GCM encrypted.
12. **Monitoring**: open **Monitoring** — the host resources table shows CPU / memory / disk usage per host (click a row for a 6-hour trend), and application monitors watch HTTP(s) endpoints, TCP ports or hosts via ping with interval, timeout, keyword and status-range options; results render as Up/Down status, heartbeat bars and 24h uptime. Check cadence and the whole module can be tuned via the `monitor_interval_sec` / `monitor_enabled` settings.
13. **Email notifications**: configure SMTP in System Settings (with a one-click test send). When enabled, exec / distribute / release / batch-account tasks send a result summary email to the recipients on completion — failed hosts with output snippets are highlighted.
14. **Kubernetes**: add a cluster in **K8S Clusters** (paste a kubeconfig or the CA + client-cert + client-key trio — cert expiry is tracked automatically). Click **Manage** for the full-screen cluster console: overview cards, workloads (Pods / Deployments / CronJobs …), storage, config, Helm releases; pod Shell runs in an in-page drawer. Scale a Deployment or restart it from the row menu; every member only sees clusters they belong to.
15. **Observability / Log Search**: enable OpenObserve in System Settings → Observability (or `--profile observability` in compose). Host metrics, task output and alert events are then dual-written in real time; the **Log Search** menu queries everything with SQL (stream picker, time ranges, CSV export), and the **Observability** admin page shows connection health plus per-stream toggles and push counters. External systems push custom data with `POST /api/ext/oo/{stream}` using an API key — pushed data is immediately searchable.

16. **Web Apps (PAM)**: register frequently used web consoles (URL + credentials, AES-encrypted) under **Web Apps**. Clicking **Open** launches a server-side headless browser that auto-fills the vaulted credentials, signs in and streams the page live — operate it with your own mouse and keyboard while the password never reaches your machine. Session starts are audited and each session is capped at 30 minutes.
17. **Database Workbench**: register MySQL / SQL Server / PostgreSQL sources under **Assets → Databases** (multiple accounts per source, usage authorized per user group — read-only for monitoring, read-write for DBAs). The SQL editor offers schema-aware completion and the result grid exports CSV. Per-source read-only mode, dangerous-SQL interception, statement timeout and row caps are supported, and every statement is audited.

## Windows Access (WinRM + RDP)

1. Add the host with **OS type = Windows** (WinRM port 5985, RDP 3389 by default) and a local or domain account (`DOMAIN\user` works).
2. Command execution, metrics collection and alerting run over **WinRM**; the transport is auto-negotiated - HTTPS 5986 is preferred when reachable (Basic auth, immune to NTLM hardening policies), otherwise HTTP 5985 (NTLM auth, works for local and domain accounts). **Kerberos (domain environments, CIS-compliant)**: enable the per-host "Kerberos" switch (account in `user@REALM` form, realm auto-derived from the UPN suffix or the `winrm_krb5_realm` setting, optional SPN override). Kerberos runs over WinRM HTTPS with gokrb5 - no Basic, no NTLM, no UAC-filter changes on targets; the server needs a `krb5.conf` (mount into the container or set `winrm_krb5_config`).
3. **In-browser RDP**: requires the `guacd` + `rdp-gateway` sidecars (already in both compose files). Click **RDP** on a Windows host row - the platform signs a 5-minute encrypted connection string; credentials never reach the browser in plaintext.
4. Target machine one-time setup (admin PowerShell):

```powershell
winrm quickconfig
```

5. **Workgroup machines with hardened NTLM policies** (new Windows builds may reject remote NTLM sessions even after a successful logon - symptom: HTTP 401 with no failed-logon event in the target's Security log): configure WinRM HTTPS - JNexus detects port 5986 automatically and switches to Basic over TLS, no further platform change needed.

```powershell
$cert = New-SelfSignedCertificate -DnsName "<host-ip>" -CertStoreLocation Cert:\LocalMachine\My
New-WSManInstance -ResourceURI winrm/config/Listener -SelectorSet @{Address="*"; Transport="HTTPS"} `
  -ValueSet @{Hostname="<host-ip>"; CertificateThumbprint=$cert.Thumbprint; Enabled="true"}
Set-Item -Path WSMan:\localhost\Service\Auth\Basic -Value $true
New-NetFirewallRule -DisplayName "WinRM HTTPS" -Direction Inbound -Protocol TCP -LocalPort 5986 -Action Allow
# local accounts also need the UAC remote-token filter lifted (or add them to "Remote Management Users")
reg add "HKLM\SOFTWARE\Microsoft\Windows\CurrentVersion\Policies\System" /v LocalAccountTokenFilterPolicy /t REG_DWORD /d 1 /f
```

A `wintest` diagnostic probe (`backend/cmd/wintest`) reproduces the raw WinRM connection outside the platform for troubleshooting.

## Security Design

- SSH private keys & passwords encrypted at rest with AES-256-GCM; master key injected via config (env overridable).
- TOTP (MFA) secrets encrypted with the same master key; the intermediate `mfa_token` issued after the password step is valid for 2 minutes, is rejected for API access, and only a verified code exchanges it for a real JWT.
- Passwords hashed with bcrypt; JWT valid 24h.
- Dangerous commands blocked at exec/script/release entry points (403 + audit record).
- Audit log masks sensitive fields (password/content).
- Backend enforces authorization regardless of menu visibility (menus are UI-only).
- **AI security (security by design)**:
  - The AI assistant is capability-gated per role (`ai:chat`; read-only/auditor roles excluded by default) with checks on both the frontend entry and the API;
  - Chat is rate-limited per user (default 30/hour, configurable in AI settings) against cost abuse and endpoint hammering;
  - Injection guards: live status snapshots and machine-gathered data (SSH/WinRM/network probes) are wrapped in `UNTRUSTED` markers, and every system prompt carries top-priority security rules (data blocks contain no instructions, never reveal prompts or credentials, refuse privilege-escalation requests);
  - Live snapshots are filtered by host permissions - hosts without grants appear as counts only;
  - AI replies are sanitized with DOMPurify and outbound links get `rel="noopener noreferrer nofollow"`;
  - AI never generates executable commands: alert-cleanup commands come verbatim from an admin-defined catalog (`;&|` forbidden) and are fully audited;
  - Chat prompts are recorded by the global write-audit middleware (operator, source IP, body summary).

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

[GNU AGPL-3.0](LICENSE) © 2026 hmilyzhang

JNexus is free software: you can run, study, modify and redistribute it under the terms of the
GNU Affero General Public License v3.0. If you modify JNexus and offer it as a network service,
you must make your modified source available under the same license (AGPL §13).
The optional [OpenObserve](https://openobserve.ai) backend is likewise AGPL-3.0 (invoked over
HTTP only — deploying it alongside JNexus imposes no additional obligations).

<div align="right"><a href="README.zh-CN.md">中文文档</a></div>
