# JNexus Documentation

> JNexus — Open-source, lightweight, self-hosted ops platform
> 中文版本：[zh-CN.md](./zh-CN.md)

JNexus is built with **Go (Gin + GORM) + Vue 3 (Element Plus + xterm.js) + PostgreSQL** and ships as a single binary that serves both the API and the web UI. It covers everyday operations: host management, web terminal, in-browser RDP, batch execution, monitoring & alerting, Kubernetes management, application releases, and ops reporting.

---

## Table of Contents

1. [Introduction](#1-introduction)
2. [Architecture](#2-architecture)
3. [Quick Start](#3-quick-start)
4. [Configuration Reference](#4-configuration-reference)
5. [Hosts](#5-hosts)
6. [Account Credentials](#6-account-credentials)
7. [Web Terminal](#7-web-terminal)
8. [In-browser RDP](#8-in-browser-rdp)
9. [Batch Execution & File Distribution](#9-batch-execution--file-distribution)
10. [Script Library](#10-script-library)
11. [Scheduled Jobs](#11-scheduled-jobs)
12. [Monitoring & Alerting](#12-monitoring--alerting)
13. [Capacity Planning](#13-capacity-planning)
14. [Kubernetes Management](#14-kubernetes-management)
15. [Apps & Releases](#15-apps--releases)
16. [Ops Reports](#16-ops-reports)
17. [RBAC & Security](#17-rbac--security)
18. [System Administration](#18-system-administration)
19. [Upgrades & Data](#19-upgrades--data)
20. [FAQ](#20-faq)
21. [Observability Integration (OpenObserve)](#21-observability-integration-openobserve)
22. [AI Alert Diagnostics & Controlled Cleanup](#22-ai-alert-diagnostics--controlled-cleanup)
23. [Windows Domain Environments](#23-windows-domain-environments)
24. [Reverse Proxy Deployment](#24-reverse-proxy-deployment-https--websocket)
25. [Cloud Asset Sync (AWS / Azure / Huawei Cloud)](#25-cloud-asset-sync-aws--azure--huawei-cloud)
26. [Web Apps (PAM)](#26-web-apps-pam)
27. [Database Workbench](#27-database-workbench)
28. [Collection Reports](#28-collection-reports)

---

## 1. Introduction

JNexus targets self-hosted operations for small and mid-size teams. The goal: **run your whole fleet from one web page**.

- **Two platforms, one console**: Linux (SSH) and Windows (WinRM / in-browser RDP) managed side by side;
- **AI Assistant**: built-in floating chat panel compatible with any OpenAI protocol service (Ollama / vLLM / LM Studio), with page context awareness and role presets;
- **Dark / Light theme**: global toggle, fully adapted across all pages;
- **One-command deploy**: a single binary plus one Docker Compose file (database and RDP gateway included);
- **All modules in one console**: assets (hosts / databases / web apps), account credentials, batch execution, file distribution, script library, scheduled jobs, monitoring & alerting, capacity planning, Kubernetes, apps & releases, collection reports, ops reports, log search, AI alert diagnostics, web apps (PAM), database workbench;
- **Built-in governance**: capability-based RBAC + custom roles + MFA + LDAP + full audit trail;
- **Bilingual UI**: Chinese and English out of the box, auto-switched by browser language.

Scale: from a handful to hundreds of hosts, and one to many Kubernetes clusters.

## 2. Architecture

```
Browser (Vue 3 SPA)
   │  HTTP / WebSocket
JNexus Server (single binary: API + static assets + scheduler + WS gateway)
   ├─ SSH / WinRM ──── Linux / Windows hosts
   ├─ Kubernetes API ─ multiple K8S clusters
   ├─ guacamole-lite → guacd ─ Windows RDP 3389
   └─ GORM ─ PostgreSQL (business data, metric samples, audit)
```

- **Backend**: Go 1.25. Gin serves the API and static assets; GORM handles models with auto-migration. A built-in scheduler loop drives metric collection, sample archiving, cron jobs, and alert evaluation.
- **Frontend**: Vue 3 + Element Plus + xterm.js. The build output is pure static files served by the backend; Chinese and English ship in the box.
- **RDP gateway**: `guacamole-lite` (Node.js) + `guacd` (Apache Guacamole 1.6.0). The browser reaches Windows desktops over WebSocket; connection credentials are exchanged via **one-time tokens**, AES-encrypted in transit.
- **Deployment modes**: Docker Compose (recommended — four services: jnexus / postgres / guacd / rdp-gateway) or a bare binary with an external PostgreSQL.

## 3. Quick Start

### Docker Compose (recommended)

```bash
git clone https://github.com/hmilyzhang/JNexus.git
cd JNexus/deploy
docker compose up -d
```

- Console: `http://<server-ip>:8080`
- Default administrator: `admin / admin123` — **change it immediately after first login** (see [System Administration](#18-system-administration)).
- Database, schema, and encryption keys are created/generated automatically on first boot.

### Bare binary

1. Download a release binary for your platform (or build with `go build ./cmd/server`);
2. Prepare PostgreSQL and provide connection details via environment variables (see [Configuration Reference](#4-configuration-reference));
3. Run the binary — it listens on `:8080` by default.

### Adding your first host

Go to **Assets → Hosts** after login: add a host (IP, SSH port, root password or key) → once it turns green you can open a terminal, run commands, and see metrics. For Windows hosts pick the Windows OS type and set the WinRM port (default 5985).

## 4. Configuration Reference

JNexus reads environment variables first. Keys generated on first boot are persisted in the data directory (`/app/data` inside the container) and survive restarts.

| Variable | Description | Default |
| --- | --- | --- |
| `JNEXUS_DB_HOST` / `JNEXUS_DB_PORT` | PostgreSQL address | `postgres` / `5432` |
| `JNEXUS_DB_USER` / `JNEXUS_DB_PASSWORD` / `JNEXUS_DB_NAME` | Database account | `jnexus` / `jnexus123` / `jnexus` |
| `JNEXUS_DSN` | Full DSN (overrides the DB variables above) | – |
| `JNEXUS_AES_KEY` | Master key for credential encryption (32 chars); auto-generated when unset | auto |
| `JNEXUS_JWT_SECRET` | JWT signing secret; auto-generated when unset | auto |
| `JNEXUS_DATA_DIR` | Persistence dir for keys/data (default `/app/data` in container) | `/app/data` |
| `JNEXUS_VERSION` | Version injection (build arg `APP_VERSION`) | dev |
| Server port | Always listens on `:8080` (mapped by Compose) | `8080` |

**Key safety**: `JNEXUS_AES_KEY` encrypts all host credentials — if it is lost, encrypted data cannot be recovered. The container entrypoint stores it in `data/aes_key` on the data volume; back that volume up regularly.

**Internal CA trust**: certificates signed by your company CA (LDAPS, HTTPS targets, K8S API) are not trusted by
the container by default — either mount a `./cacerts/` directory (`.crt`/`.pem`) or set `JNEXUS_TRUST_CA_URL`
(space/comma-separated URLs) so the entrypoint fetches and merges them into the `SSL_CERT_FILE` bundle at startup
(PEM-validated; failures only warn, never block the start). Inside the container reach the host's localhost via
`host.docker.internal` (Linux needs `extra_hosts: ["host.docker.internal:host-gateway"]`). For bare-metal
deployments run `sudo sh deploy/install-ca-trust.sh <cert-or-URL>` — it detects the distro
(Rocky/RHEL: update-ca-trust; Ubuntu/Debian: update-ca-certificates); `DRY_RUN=1` previews the actions.

## 5. Hosts

**Menu: Assets → Hosts**

- **Unified asset entry**: the Assets page organizes **hosts / databases / web apps** into tabs with one "Add asset" action;
- **Group tree**: multi-level host groups; filter by group; the toolbar supports **batch move to group**;
- **Host fields**: name, IP, SSH port, OS type (Linux / Windows), description, group; paginated host list (10/20/50/100/200 per page);
- **OS accounts**: attach multiple accounts (password or key) per host and mark a default; execution/terminal/releases resolve credentials as "specified account → host default"; central management in [Account Credentials](#6-account-credentials);
- **Credential templates**: apply a shared account template when batch-adding hosts;
- **Windows support**: with the Windows OS type, hosts use WinRM (default port 5985, NTLM domain accounts supported) for PowerShell execution and metric collection; RDP offers a multi-account logon picker (`DOMAIN\user` and UPN both work);
- **Cascade delete**: deleting a host (single or batch) removes its OS accounts;
- **Cloud asset sync**: discover and import instances from AWS / Azure / Huawei Cloud (see [Cloud Asset Sync](#25-cloud-asset-sync-aws--azure--huawei-cloud));
- **Host actions**: test connection, open terminal, file distribution, capacity drawer, batch delete.

## 6. Account Credentials

**Menu: Account Credentials**

- Every host OS account in one place: filter by host / keyword / rotation status; add / edit / rotate / reveal (admin, audited);
- **Password auto-rotation**: enable per account with a configurable period; random strong passwords are AES-encrypted at rest and never displayed. Normal accounts rotate through a same-host root/NOPASSWD-sudo **privilege chain**; paired-key accounts with a stored password rotate via key login; LDAP/domain accounts are detected and skipped;
- **Password history**: every rotation is archived (last 24 per account), visible to admins and fully audited;
- **Account templates**: store LDAP/AD passwords once and reuse them when adding hosts or bulk importing;
- **Force key pairing**: existing stored-password accounts can be paired directly - the vaulted password installs the platform public key and a default key account is created (the password account is kept; rotation is unaffected). Available as a row action or a fleet-wide sweep (Windows / no-password accounts are skipped; domain-template accounts are included since pairing never changes a password; already-paired ones are not re-processed), fully audited;
- Sensitive data — host passwords, private keys, kubeconfigs — is always **AES-encrypted at rest** and never shown in clear text;
- Batch add/delete with reference protection.

## 7. Web Terminal

**Menu: Jobs → Web Terminal**

- Asset tree on the left (groups → hosts → OS accounts); click to open a terminal, run many tabs in parallel;
- xterm.js rendering: **6 themes** (Default Dark, Dracula, One Dark, Solarized Dark/Light, GitHub Light), **in-page fullscreen**, auto-fitting;
- Sessions go over WebSocket straight to the backend SSH channel and stay alive with the page (the workspace lists Linux hosts only; Windows uses RDP);
- K8S Pod shells share the same experience (see [Kubernetes Management](#14-kubernetes-management)).

## 8. In-browser RDP

**Prerequisite**: remote desktop enabled on the Windows host (3389). Compose bundles `guacd 1.6.0` and `rdp-gateway`.

- Click **RDP** on a host row: the backend issues a one-time connection token (valid for 5 minutes, burned after use) and opens the desktop in a new browser window;
- The token travels in an AES-encrypted query string; the gateway callback exchanges it for real credentials — no connection secrets stored in clear text;
- Resolution auto-fit and clipboard support follow guacd capabilities.

## 9. Batch Execution & File Distribution

**Menu: Jobs → Exec / Files**

- **Exec**: pick multiple hosts (by group) and run Shell / PowerShell concurrently with a controllable timeout; results are aggregated per host (exit code + output) and can be copied as a report;
- Jobs use the host's default OS account, or an explicitly chosen credential;
- **Files**: upload a file and distribute it to a target path on many hosts, with per-host results;
- All runs land in **Task Records**, exportable to CSV for audit.

## 10. Script Library

**Menu: Jobs → Scripts**

- Turn repeated operations into scripts: name, description, language (Shell / PowerShell), content;
- Parameter templates supported; one-click reuse from the exec page;
- Scripts can be referenced by scheduled jobs for recurring work.

## 11. Scheduled Jobs

**Menu: Cron Jobs**

- Standard cron expressions; runs scripts/commands on target hosts on schedule;
- Every run records status, duration, and output; failures can trigger alerts;
- Supports manual "run now", enable/disable, and editing.

## 12. Monitoring & Alerting

**Menu: Monitor**

- **Metric collection**: the scheduler samples host CPU / memory / disk on interval (SSH for Linux, WinRM for Windows). Raw samples are kept for 30 days, then archived into hourly aggregates (supporting 180-day / 1-year views);
- **Trend views**: per-host 6h / 24h / 7d / 30d / 180d / 1y charts;
- **Availability**: 24h / 30d availability computed with maintenance windows — incidents inside a window don't count;
- **Alert rules**: CPU / memory / disk thresholds evaluated in real time with recovery-notify toggle; host reboots (boot-id / boot-time change) alert automatically;
- **Alert channels**: email / webhook / WeCom / DingTalk / Feishu / Telegram — configure SMTP and channels in System Settings, then bind them to rules;
- **Monitors**: HTTP / TCP / Ping probes with availability and response time.

## 13. Capacity Planning

- **Hosts**: the capacity drawer shows CPU / memory / disk history plus a linear-regression forecast of "days until the 90% watermark", color-coded green/amber/red;
- **Kubernetes**: cluster-level CPU / memory capacity trends (15-minute sampling; raw points ≤48h, hourly buckets for 7d/30d, daily buckets for 180d/1y) with a forecast of days-until-full; pod-level usage trends (Top10 sampling);
- One unified chart component: percent or auto-scaled Y axis, grid lines, time axis, hover tooltips.

## 14. Kubernetes Management

**Menu: K8S**

- **Multi-cluster**: onboard clusters via kubeconfig or CA + client certificate; cluster membership grants per-cluster admin / user / viewer roles to users;
- **Resources**: Nodes / Namespaces / Deployments / StatefulSets / DaemonSets / Jobs / CronJobs / Pods / Services / Ingresses / ConfigMaps / Secrets / ServiceAccounts — full listings with customizable columns, search, and batch delete;
- **Actions**: scale & restart workloads, delete pods, suspend/resume CronJobs, view/edit/download YAML, create resources from YAML;
- **Pod Shell / logs**: interactive in-browser terminal (tty), live log streaming, and a Cluster Shell (ephemeral busybox pod, auto-cleaned on close);
- **Helm**: browse chart repos; install, upgrade, rollback and uninstall releases;
- **Capacity planning**: cluster and pod usage trends with forecasts (see above).

## 15. Apps & Releases

**Menu: Apps / Releases**

- **Apps**: define an app and register target hosts; each host carries a deploy config — deploy directory, artifact name, stop command (empty = kill by process name), start command, backup dir (default `deploy_dir/backup`), release OS account (falls back to host default);
- **Release pipeline**: upload a package → pick app + target hosts → run **stop → backup → upload → start** per host, with live per-step output and fail-fast;
- **Dangerous-command interception**: every pipeline command is checked before execution;
- **Rollback**: one click, restored from the release backup;
- Full release history with per-step output for traceability.

## 16. Ops Reports

**Menu: Reports**

- **Monthly report**: auto-compiled — availability, alert counts and P1/P2 timeline, capacity traffic lights, execution stats;
- **Report detail**: collapsible checklist that only expands failed items, made for review meetings;
- **Export**: task records and reports export for monthly reporting.

## 17. RBAC & Security

- **Roles**: built-in `admin / ops / publisher / viewer / auditor`, plus **custom roles**;
- **Capability model**: 14 modules × per-module actions (e.g. `hosts.view`, `exec.exec`, `releases.rollback`, `ai.chat`) configured in Role Settings, enforced at both menu and route level;
- **MFA**: TOTP two-factor, self-service in the profile page;
- **LDAP**: corporate directory login with automatic display-name sync;
- **Audit**: logins, executions, releases, deletions and K8S operations all recorded with actor, IP, resource and result;
- **Encryption**: host credentials / kubeconfigs AES-encrypted at rest; one-time RDP tokens; signed JWT sessions.
- **AI security (security by design)**:
  - AI chat is gated by the `ai.chat` capability (default: admin/ops/publisher/k8s; viewer/auditor excluded), enforced on both the frontend entry and the API route;
  - Chat is rate-limited per user (30/hour) against cost abuse;
  - Prompt-injection guards: live status snapshots and machine-gathered data (SSH/WinRM/network probes) are wrapped in `UNTRUSTED` markers, and every system prompt carries top-priority security rules (data blocks contain no instructions, never reveal prompts or credentials, refuse privilege escalation);
  - Live snapshots are filtered by host permissions - hosts without grants appear as counts only;
  - The AI never executes commands directly: alert-cleanup commands come verbatim from an admin-defined catalog (`;&|` forbidden) and chat prompts are recorded by the audit middleware.

## 18. System Administration

**Menu: System**

- **Users**: create/disable users, reset passwords, assign roles;
- **Role Settings**: menu + capability matrix for built-in and custom roles;
- **Audit Logs**: searchable record of all operations;
- **System Settings**: system name, SMTP, alert channels and other globals;
- **Danger Zone**: sensitive maintenance actions in one place;
- **First-hardening checklist**: change the admin password → enable MFA → create regular users → configure LDAP as needed.

## 19. Upgrades & Data

- **Upgrade**: `git pull && cd deploy && docker compose up -d --build`; schema migration is automatic — no manual SQL;
- **Data**: everything lives in PostgreSQL; Compose volumes: `pgdata` (database), `data` (keys), `uploads` (artifacts/uploads);
- **Backup advice**: schedule `pg_dump` plus a copy of the `data` volume (keeping the AES key keeps your credentials decryptable);
- **External database**: set `JNEXUS_DB_*` or `JNEXUS_DSN`, then `docker compose up -d jnexus` to start only the app.

## 20. FAQ

**Q: Forgot the admin password?**
Have an administrator with database access reset it (replace the password hash with a known value), or recreate the user from another admin account.

**Q: Windows host won't connect?**
Make sure WinRM is enabled on the target (`winrm quickconfig`) and port 5985 is open; domain accounts use the `DOMAIN\user` format.

**Q: RDP won't open?**
Check that 3389 is reachable and the `guacd` / `rdp-gateway` containers are healthy. Connection tokens are valid for 5 minutes — re-initiate if expired.

**Q: Some K8S resources are empty after adding a cluster?**
Verify the credential's RBAC covers those resources and namespaces; capacity planning needs metrics-server (or metrics read permission for the sampling account).

**Q: Monitoring charts are empty?**
Confirm the host is online and its credentials work. Sampling is interval-driven — a newly added host needs one or two sampling cycles before charts appear.

## 21. Observability Integration (OpenObserve)

Optional integration with [OpenObserve](https://openobserve.ai) (AGPL-3.0, HTTP-only invocation)
for **long-term storage and full-text search**:

- **Five built-in dual-write streams**: host metrics, task output, alert events, Windows event
  logs, database audit trail — each with an independent toggle;
- **Log Search menu**: stream picker (built-ins + auto-discovered custom streams), SQL query,
  1h/24h/7d/30d + custom time ranges, pagination (50–500/page), event-ID/host/log-type quick
  filters, CSV export;
- **Observability admin page** (System Admin): connection health & latency probe, per-stream
  toggles + push statistics, push tester, external push API (`POST /api/ext/oo/{stream}`, API-key
  auth, audited);
- **Deploy**: `docker compose --profile observability up -d --build` (single container, off by default);
- **Menu visibility**: the Log Search menu auto-hides while the integration is disabled;
- **Credential isolation**: OpenObserve credentials live only in JNexus (AES-256 encrypted);
  end users never touch the OpenObserve UI.

## 22. AI Alert Diagnostics & Controlled Cleanup

When a disk/mem/cpu threshold alert fires, an **AI diagnosis pipeline** runs automatically:

1. **Gather**: fixed read-only diagnostics over SSH on Linux hosts and via WinRM on Windows hosts
   (resource levels / process leaderboard / directory usage);
   - App monitors (HTTP/TCP/ping) on downtime get server-side network probes instead: DNS resolve,
     TCP connect, HTTP re-probe (status / latency / TLS cert expiry / body snippet), system ping
     (regex-validated target); when the resolved IP matches a managed Linux host, SSH gather is
     appended to separate service failures from system issues;
2. **Analyze**: alert context + diagnostics are sent to the AI (system prompt customizable in the
   UI); it returns root-cause analysis / suspect processes / remediation advice;
3. **Deliver**: host-alert analyses go through the channels bound to that alert level, app-monitor
   reports through the channels bound to the monitor; also recorded as an audit event and pushed
   to the OpenObserve stream;
4. **Controlled cleanup** (disk alerts only): admin-predefined cleanup commands from the catalog
   run VERBATIM (e.g. 7-day stale /tmp file purge, journald vacuum), each with a 60-second timeout;
   results are appended to the report.

**Safety guardrails**:
- The AI only outputs analysis text — it **never generates or modifies commands**; cleanup
  commands come verbatim from the admin-defined catalog;
- Cleanup runs on disk alerts only and only on Linux hosts; config lives in Monitoring → AI Diagnostics (admin); per-host/per-monitor cooldown (default 30 min, shared setting);
- The catalog is admin-managed; commands must not contain `;`, `&`, `|` or line breaks;
- The diagnosis system prompt is customizable (empty = built-in default).

## 23. Windows Domain Environments

- **Kerberos auth** (recommended for domain environments, CIS-compliant): per-host switch;
  account in `user@REALM` form (realm auto-derived from the UPN suffix or the
  `winrm_krb5_realm` setting); runs over WinRM HTTPS + gokrb5 — no Basic auth, no NTLM, no UAC
  token-filter changes on targets; the server must be able to read a `krb5.conf` (set
  `winrm_krb5_config`).
- **HTTPS + Basic** (workgroup machines): configure a WinRM HTTPS listener (5986) + self-signed
  certificate on the target; JNexus auto-detects port 5986 and switches to Basic over TLS —
  immune to NTLM hardening policies.
- **NTLM** (legacy/direct): HTTP 5985 fallback; works for local and domain accounts.

## 24. Reverse Proxy Deployment (HTTPS + WebSocket)

When serving JNexus behind a reverse proxy (nginx etc.) over HTTPS, the proxy **must forward
WebSocket upgrades** — the Web Terminal, live task output, log tail, and the RDP gateway proxy
(`/rdp-gw`) all use WebSocket:

```nginx
location / {
    proxy_pass http://127.0.0.1:8080;
    proxy_http_version 1.1;
    proxy_set_header Upgrade $http_upgrade;
    proxy_set_header Connection "upgrade";
    proxy_set_header Host $host;
    proxy_set_header X-Forwarded-Proto https;
    proxy_read_timeout 3600s;
}
```

Without Upgrade/Connection forwarding, login and pages work but terminals / RDP / live output
disconnect immediately. `X-Forwarded-Proto https` tells JNexus the page is HTTPS (affects RDP
gateway address derivation).

## 25. Cloud Asset Sync (AWS / Azure / Huawei Cloud)

**Entry: Assets → Hosts → Cloud sync**

- **Cloud accounts**: register an AK/SK (AWS / Huawei Cloud) or a service principal (Azure), pick regions per account; credentials AES-encrypted at rest;
- **Discovery**: cloud instances are imported as host assets and grouped automatically by **tags**; hosts whose IP is already managed are not imported twice;
- **Scheduled sync**: per-account interval executed by the scheduler; instances released in the cloud are removed automatically on the next sync;
- **Account template**: a cloud account can bind a credential template; imported hosts automatically get the template OS account (default flag, template password) - usable for exec / terminal right after the sync; resyncs never duplicate accounts;
- **Automatic key pairing**: after the template account is created, Linux hosts are immediately paired with the platform key using the template password, and the key account becomes the default (unreachable hosts keep the password default - run the force-pair sweep later);
- Every sync is audited.

## 26. Web Apps (PAM)

**Menu: Web Apps**

- Vault frequently used web consoles (URL + username/password, AES-256-GCM encrypted, never echoed back);
- Clicking **Open** starts a **headless-browser session** on the server: it auto-fills the vaulted credentials, signs in and streams the page live — operate it with your own mouse / wheel / keyboard;
- **The password never leaves the server**: credentials are injected server-side only and never reach the user's machine;
- Session starts are audited; each session is capped at 30 minutes; menu access is role-controlled.

## 27. Database Workbench

**Entry: Assets → Databases**

- **Sources**: register MySQL / SQL Server / PostgreSQL / Oracle instances (AES-encrypted credentials, shared with OpenObserve ingestion; Oracle uses the pure-Go go-ora driver - no Instant Client required, the Database field holds the service name, default port 1521); each source can hold **multiple accounts** with per-user-group authorization (read-only for monitoring, read-write for DBAs);
- **SQL editor**: CodeMirror 6 with **schema-aware completion** (tables / columns / keywords, 5-minute schema cache), syntax highlighting, Ctrl+Enter execution, theme-adaptive colors;
- **Result grid**: row numbers, per-column type badges, right-aligned numerics, dimmed NULLs, CSV export;
- **Guardrails**: per-source **read-only mode**; dangerous-SQL interception (DROP/TRUNCATE, UPDATE/DELETE without WHERE, ...); statement timeout and row cap; **every statement audited** (SQL, account, source IP, duration, rows).

## 28. Collection Reports

**Menu: Reports → Collection reports**

- **Preset templates**: server accounts, crontab listing, health check, system info, ports & certificates (listening ports, HTTP/HTTPS detection, expiry of certificates served on HTTPS ports and of local certificate files);
- **Cross-host runs**: empty target = all hosts; per-host results are archived; **dual-version presets** dispatch bash over SSH on Linux and PowerShell over WinRM on Windows by host OS; the accounts preset adds a cross-host **account matrix** with suspicious-account highlighting;
- **Merged export**: all hosts' results merge into **one printable HTML document**; the ports & certificates preset adds a cross-host **ports-and-certificates matrix** (per-host port / HTTPS / certificate valid-and-expired counts);
- **Export**: `.log` / CSV; module access is role-controlled.

---

© JNexus · Open-source ops platform · Website [jnexus.tech](http://jnexus.tech/)
