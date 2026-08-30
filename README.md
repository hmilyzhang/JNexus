<div align="right"><a href="README.zh-CN.md">中文文档</a></div>

# AutoOps — Lightweight Ops Platform

A self-built, lightweight operations platform: host management (multi OS accounts per host), batch command execution with live output, file distribution, script center, release pipeline with rollback, RBAC with configurable roles, audit logging, dangerous-command blocking, and LDAP authentication. Data is stored in an external PostgreSQL.

**By JJ Zhang · Version 1.0**

Tech stack: Go (Gin + GORM) + PostgreSQL + Vue3 (Element Plus + xterm.js).

## Features

| Module | Capabilities |
|--------|--------------|
| Dashboard | Post-login home: managed/online hosts, groups, tasks, apps, releases, users, active rules + current user info and quick access |
| Hosts | Tree group view, import (`NAME,IP,port,user,group`, unified password on the page, auto SSH-key pairing), concurrent connectivity probe, per-host **multiple OS accounts** (labels, default flag), Web Shell |
| Batch Exec | Tree host selection / multi-IP input, **OS account selection**, concurrent execution, WebSocket live per-host output, success/fail counters & progress |
| Task Console | Aggregate task view: host table (account, status, exit code, duration, failed-first), output panel, failed-only filter, cross-host output keyword search, **export summary `.log` / CSV** |
| File Distribution | Upload → concurrent SFTP to many hosts (tree/IP selection), live progress |
| Scripts | CRUD + one-click batch execution |
| Release Center | App → host bindings (with release OS account); pipeline: stop → timestamped backup → upload → start → health check; one-click rollback to latest backup |
| Users | Two tabs: Users + Group Management. Roles: admin / ops / publisher / viewer / **auditor**; user groups link members, hosts, host groups, **OS accounts** and **account rules** (`host scope × username`, auto-covers future hosts). User rows carry audit fields: creator, last modified by/at, disabled at/by |
| Web Shell | Full-screen terminal workspace: host asset tree expanded by usable OS account, multiple concurrent sessions, per-account connections |
| Audit Log | All write operations recorded (who / action / resource / source IP / status), full output retention in tasks; visible to admin & auditor only |
| Dangerous Commands | Regex rule library (rm -rf, mkfs, dd, shutdown, drop database… 11 built-in), blocks at exec/script/release entry points and writes audit; editable & testable by admin |
| Email (SMTP) | SMTP settings (SSL / STARTTLS, auth, masked password), test send; task-completion notification emails with success/fail counts and per-host result table (failed tasks include output snippets) |
| System Settings | Tabs: General (system name), LDAP auth (server, group-membership check, connection test), Role Settings (editable description / menu visibility / host permissions view-create-edit-delete per role) |
| i18n | Chinese / English switcher (top right) |

## Quick Start (Local Development)

### 1. Prepare PostgreSQL

Any external PostgreSQL works, e.g. via Docker:

```bash
docker run -d --name autoops-pg -e POSTGRES_USER=autoops -e POSTGRES_PASSWORD=autoops123 \
  -e POSTGRES_DB=autoops -p 5432:5432 postgres:16-alpine
```

### 2. Configure the backend

```bash
cd backend
cp config.example.yaml config.yaml
# Generate the AES master key (encrypts SSH keys at rest)
./autoops-server -genkey   # or: openssl rand -base64 32
# Edit config.yaml: dsn / jwt_secret / aes_key
```

### 3. Run

```bash
# Build & start (auto-creates all tables + seed data)
cd backend && go build -o autoops-server ./cmd/server && ./autoops-server -config config.yaml

# Or dev mode with hot frontend
cd frontend && npm install && npm run dev   # http://localhost:5173 proxies to :8080
```

Open http://localhost:8080 — the built frontend (`frontend/dist`) is served by the backend directly.

**Default account: `admin` / `admin123`** (change it immediately).

### 4. Production (Docker Compose)

Bundled database:

```bash
cd deploy
docker compose up -d --build
# Change JWT_SECRET and AES_KEY in docker-compose.yml (or via env) first!
```

External PostgreSQL — tables are created automatically on startup (only the database itself must exist):

```bash
export AUTOOPS_DB_HOST=10.3.0.100
export AUTOOPS_DB_PORT=5432
export AUTOOPS_DB_USER=autoops
export AUTOOPS_DB_PASSWORD=yourpass
export AUTOOPS_DB_NAME=autoops        # CREATE DATABASE first
export AUTOOPS_JWT_SECRET=<long random string>
export AUTOOPS_AES_KEY=$(openssl rand -base64 32)
docker compose -f docker-compose.external.yml up -d --build
```

Env precedence: `AUTOOPS_DSN` > `AUTOOPS_DB_HOST/PORT/USER/PASSWORD/NAME` > `config.yaml` (same for the plain binary).

## Usage Guide

1. **SSH access**: import a private key (Hosts → SSH Keys) and put the platform public key on targets, or use password auth. **Batch import** hosts with `NAME,IP,port,user,group`; enter the login password in the page-level "Common password" field and enable **auto key pairing** — the platform pushes its public key to each host and switches to key auth automatically.
2. **OS accounts**: each host can hold multiple accounts (Hosts → OS Accounts). Batch-add an account (e.g. `appuser`) to hundreds of existing hosts via the toolbar **Batch Add Accounts** (password + auto-pair).
3. **Team isolation**: create a user group, link it to OS accounts directly, or add an **account rule** (`host scope × username`, e.g. `All hosts × appuser`). Group members can then only use those accounts — ops team gets `root` via its own rule; app teams never see it.
4. **Batch execution**: pick hosts (tree) or type IPs, optionally pick an OS account, run. Live per-host output; the task console aggregates status and lets you download a summary `.log` or CSV.
5. **Release**: configure an app (hosts + deploy dir + jar name + optional commands/health URL/release account) in Applications, then upload the jar in Release Center. Failed releases roll back to the latest backup with one click.
6. **Roles**: System Settings → Role Settings controls each role's description, visible menus, and host permissions (view/create/edit/delete). Admin is always full.
7. **LDAP**: enable in System Settings, optionally require group membership (`Group Base DN` + filter + allowed groups). LDAP users are auto-created on first login with the configured default role.
8. **Email notifications**: configure SMTP in System Settings (with a one-click test send). When enabled, exec / distribute / release / batch-account tasks send a result summary email to the recipients on completion — failed hosts with output snippets are highlighted.

## Security Design

- SSH private keys & passwords encrypted at rest with AES-256-GCM; master key injected via config (env overridable).
- Passwords hashed with bcrypt; JWT valid 24h.
- Dangerous commands blocked at exec/script/release entry points (403 + audit record).
- Audit log masks sensitive fields (password/content).
- Backend enforces authorization regardless of menu visibility (menus are UI-only).

## Project Layout

```
backend/     Go backend (cmd/server entry, internal/ layered)
frontend/    Vue3 SPA (src/views per module, src/i18n zh-CN/en-US)
deploy/      docker-compose.yml (bundled DB) + docker-compose.external.yml + Dockerfile
.smoke/      Local test stubs: fake SSH/SFTP server, fake LDAP server, fake SMTP server
```

## Known Limitations (MVP)

- SSH host-key verification is skipped (designed for trusted intranets).
- File distribution is single-file granularity.
- Single-instance execution; multi-instance deployment needs a task queue.

<div align="right"><a href="README.zh-CN.md">中文文档</a></div>
