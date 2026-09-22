# JNexus User Manual

> Operation manual for end users. Architecture and development details: [en-US.md](./en-US.md).

## Table of Contents

1. [Signing In & Account Security](#1-signing-in--account-security)
2. [Interface Overview](#2-interface-overview)
3. [Host Management](#3-host-management)
4. [OS Accounts & Credentials](#4-os-accounts--credentials)
5. [Web Terminal](#5-web-terminal)
6. [Batch Execution & File Distribution](#6-batch-execution--file-distribution)
7. [Script Library & Scheduled Jobs](#7-script-library--scheduled-jobs)
8. [Monitoring & Alerting](#8-monitoring--alerting)
9. [Log Search](#9-log-search)
10. [AI Capabilities](#10-ai-capabilities)
11. [RDP Remote Desktop](#11-rdp-remote-desktop)
12. [K8S Cluster Management](#12-k8s-cluster-management)
13. [Apps & Releases](#13-apps--releases)
14. [Reports](#14-reports)
15. [Permissions & Security](#15-permissions--security)
16. [FAQ](#16-faq)
17. [Web Apps (PAM-style)](#17-web-apps-pam-style)
18. [Database Workbench](#18-database-workbench)

---

## 1. Signing In & Account Security

- Sign in with the account assigned by your administrator; LDAP accounts are supported if enabled.
- **Change password**: user menu (top right) → Profile.
- **MFA**: user menu → MFA Security → scan the QR code with any authenticator app. A 6-digit code is required at every sign-in afterwards. Lost device? Ask an admin to reset.
- **Language / Theme**: top-right switcher (中文/English, dark/light).

## 2. Interface Overview

- The left sidebar shows the menus visible to your role.
- **Log Search** appears only after an admin enables the OpenObserve integration.
- Every list page supports keyword filtering; most operations are audit-logged.

## 3. Host Management

### Adding a Linux Host
1. Hosts → Add Host: name, IP, SSH port (default 22).
2. Auth: **password** (enter the login password; the platform auto-pairs its public key) or **key** (pick an imported SSH key).
3. Save, then click **Probe** to verify connectivity.

### Adding a Windows Host
- OS type: Windows; fill WinRM port (default 5985) and RDP port (default 3389).
- One-time target setup: run `winrm quickconfig` as admin.
- Domain environments: enable the per-host **Kerberos** switch and use `user@REALM` accounts. Workgroup machines: configure WinRM HTTPS (see the project documentation).

### Bulk Import
Hosts → Import: paste multiple `NAME,IP,port,username,group` lines, fill the page-level common password, and enable auto key pairing.

### Groups & Probing
- Create hierarchical groups in Group Management.
- Select hosts and click **Probe** for concurrent connectivity checks.

### Batch move to group
- Select multiple hosts in the toolbar -> "Move to group" -> pick the target group;
- Saving with no group selected removes them from any group (ungrouped);
- Deleting a host also removes its OS account rows.


- **Password history**: every change (create/pairing/manual/rotation) is archived encrypted (last 24 kept); admins can view it on the host accounts page (audited).
- **Bulk rotation**: System Settings -> Password Rotation can rotate all applicable accounts at once and shows the applicable list with due status.

### Batch move to group
- Select multiple hosts in the toolbar -> "Move to group" -> pick the target group;
- Saving with no group selected removes them from any group (ungrouped);
- Deleting a host also removes its OS account rows.

## 4. OS Accounts & Credentials

Each host can hold multiple OS accounts (root, appuser, …), distinguished by labels; the default account is used for terminals and execution.

- **Batch Add Accounts**: Host Accounts → Batch Add → select by host group (subgroups cascade; group hosts merge into the target list) or pick target hosts directly → choose from a credential template or fill manually → Run.
- Selecting existing table rows before opening the dialog **preloads those accounts** for re-apply to other hosts.
- **Password rotation**: per-account toggle with configurable period/length/complexity; passwords are encrypted and never displayed. LDAP accounts are skipped automatically.
- **Reveal password**: admins only (audited).
- **Account templates**: store LDAP/domain passwords once; referenced when adding hosts or importing.

## 5. Web Terminal

- Sessions appear as **tabs** on top of the terminal area (connection state dot + close button).
- The asset tree mirrors the hosts page: group hierarchy, a dedicated **Ungrouped** node; Windows hosts are greyed with an **RDP** tag - clicking them launches remote desktop (an account picker appears when the host has several usable accounts).
- Every usable account is listed under each host (key type first); click an account to connect as that account; several accounts can hold separate terminals at once.
- Terminal content auto-refits when the sidebar collapses or the window resizes; the tree has a manual refresh button.

## 6. Batch Execution & File Distribution

- **Batch Exec**: Job Execution → enter a command or pick a script → select hosts (tree) or IPs → Run. Live per-host streaming output; the Task History page aggregates results and exports `.log`/CSV.
- **Dangerous-command blocking**: commands matching danger rules (rm -rf, …) are rejected and audited.
- **File Distribution**: upload a file → pick target hosts → concurrent SFTP with live progress.

## 7. Script Library & Scheduled Jobs

- **Script Library**: save frequently used scripts (with parameters) and run them against hosts in one click.
- **Scheduled Jobs**: cron expressions to run commands/scripts on selected hosts; pause/resume/run-now; full run history.

- **Dual-version scripts**: the editor has "Linux (bash)" and "Windows (PowerShell)" tabs; batch exec dispatches the matching version per host OS. Windows hosts skip scripts without a PowerShell variant (no syntax errors).
## 8. Monitoring & Alerting

- **Host resources**: CPU/memory/disk collected every minute; progress bars + trend drawer (6h/24h/7d/30d); 30-day retention.
- **Application monitors**: HTTP(s)/TCP/Ping probes with heartbeat bars + 24h uptime.
- **Alert rules**: global thresholds (failure duration, recovery notices).
- **CMD severity alerts**: P1–P4 levels with per-metric thresholds, duration and notification channels.
- **Maintenance windows**: planned downtime excluded from uptime and alerting; a live banner shows whether a window is in effect right now; changes are audited with current-config/superseded tags (admins can delete or clear records).
- **AI Diagnostics** (admin tab): on disk/mem/cpu alerts the host state is gathered automatically (SSH for Linux, WinRM for Windows) and analyzed by AI, with the full analysis pushed to the channels bound to that level; app-monitor downtime triggers server-side network probes (DNS / TCP / HTTP certificate / ping), appending SSH gather when the target matches a managed host, delivered to the monitor's channels. Effective levels/metrics, cooldown and the diagnosis prompt are configurable; disk alerts may run admin-defined controlled cleanup commands (verbatim, no AI-generated commands, fully audited).
- **Notification channels**: Email/Webhook/WeCom/DingTalk/Feishu/Telegram with test send and template editing.

## 9. Log Search

Requires the OpenObserve integration (System Admin → Observability).

- Pick a data stream (host metrics / task output / alert events / Windows events / DB audit / custom streams);
- Time range presets or custom start/end;
- SQL query editor with per-stream field hints;
- Quick filters (log type, event ID, host) on Windows events / host metrics;
- Click a row's arrow to expand full fields; export CSV.

## 10. AI Capabilities

- **AI Assistant**: floating chat (bottom right). Ask ops questions; every question automatically carries a **live platform snapshot** (unrecovered alert events, app monitors currently down, offline hosts), so questions like "what active alerts do we have?" get real answers; page awareness injects the module context. Reasoning-model `<think>` chains are stripped automatically and never shown.
- **Alert diagnostics** (admin-enabled): configured under Monitoring → AI Diagnostics. Host alerts gather state automatically (SSH for Linux, WinRM for Windows) and are analyzed by AI, with the full analysis pushed through the bound channels; app-monitor downtime triggers server-side network probes delivered to the monitor's channels.
- **Controlled disk cleanup** (optional): on disk alerts, admin-predefined cleanup commands run verbatim — the AI never generates commands. The catalog lives under Monitoring → AI Diagnostics; Linux hosts only.

## 11. RDP Remote Desktop
- When the host has several usable accounts, clicking "RDP" first shows an **account picker** (default preselected) - different teams can connect with their own granted accounts. Domain logons accept both "DOMAIN\user" and UPN forms.

Click **RDP** on a Windows host row to open an in-browser remote desktop in the same tab (no mstsc; the Windows host menu offers RDP only). The desktop scales to fit the window and the remote resolution follows the container size; the Close button disconnects and returns to the hosts page. Requires the server-side guacd + rdp-gateway components. Credentials are exchanged via a one-time encrypted token; the browser never sees plaintext.

## 12. K8S Cluster Management

- Add a cluster: paste a kubeconfig or the CA + client-cert + client-key trio.
- Certificate expiry tracked automatically (30/7-day reminders).
- The management console covers nodes, workloads (pod logs/shell/delete, deployment scale/restart), services, config, storage, and Helm releases.
- Every resource supports YAML view/edit/download; capacity planning shows 30d/1y trends and exhaustion forecasts.

## 13. Apps & Releases

- **Apps**: define an application (deploy dir, artifact name, start/stop commands, health URL, release account) and bind hosts.
- **Release Center**: upload the artifact → release (stop → backup → upload → start → health check) → one-click rollback on failure.

## 14. Reports

- **Collection reports**: built-in templates (server accounts, crontab, health check, system info, ports & certificates) run across hosts; results exportable as `.log`/CSV.
- **Monthly ops report**: auto-loads on tab open; executive summary, capacity risk lights, host resource table, K8S table, alert statistics, config changes — print to PDF or export alert details as CSV.

## 15. Permissions & Security

- **Roles**: six built-in roles (admin/ops/publisher/viewer/auditor/k8s) + custom roles via a capability matrix (module × action); menu visibility per role.
- **User groups**: bind members + host groups + OS accounts for team data isolation.
- **API keys**: let external systems call `/api/ext/*` (host status, monitors, tasks, batch exec, custom data push). SHA-256 hashed, rate-limited, audited.
- **Audit**: every write operation recorded (who / action / resource / IP / status); visible to admins and auditors.

## 16. FAQ

**Q: Page keeps redirecting to login?**
The session token expired or the backend was restarted. Sign in again; if it persists, clear site data.

**Q: Windows host connection fails?**
Ensure `winrm quickconfig` ran on the target. For domain environments prefer Kerberos; for workgroup machines configure WinRM HTTPS (see project documentation).

**Q: The Log Search menu is missing?**
The OpenObserve integration is disabled. Re-enable it under System Admin → Observability.

**Q: No analysis was generated for an alert?**
Check that the AI service is enabled and reachable, that the alert level/metric is within the diagnostics scope, and that the cooldown period has passed.

---

## 17. Web Apps (PAM-style)

Vault common web consoles (switches, server out-of-band management, internal systems) as assets; a headless browser on the server auto-logs in with the vaulted credentials and streams the screen to you - the password never touches your computer.

### Manage (admins)
- "Web Apps" menu -> Add: name, target URL (http/https), username, password (AES encrypted), description.
- Editing changes every field; an empty password keeps the stored one.

### Open (any signed-in user)
1. Click "Open" on an asset -> a full-screen session page shows the target URL, account and an audit note;
2. Click "Confirm & Open" -> the server-side headless browser opens the page, **auto-fills the vaulted credentials and signs in**, streaming the screen live;
3. Mouse, wheel and keyboard act on the streamed page (enter CAPTCHAs/MFA manually if shown);
4. The password is never visible - it only exists in server memory.

### Audit & security
- Session starts are audited (WEBASSET_SESSION: operator/asset/account/source IP);
- There is no password reveal path, not even for admins;
- Sessions cap at 30 minutes and the browser instance is destroyed on disconnect.

### Authorization
- The "Web Apps" menu can be enabled per role in System Settings; access follows that menu permission.

---

## 18. Database Workbench

Run day-to-day SQL against registered databases (MySQL / SQL Server / PostgreSQL) from the web.

### Sources & accounts
- A database source = one instance (type/host/port/database); each source carries **multiple logon accounts** (e.g. a read-only collector account, a DBA account) with AES-encrypted passwords.
- Admins register sources via "Add database source" in the workbench; accounts are managed inside the workbench and can be **restricted to user groups** (e.g. the DBA account only for the DBA group - invisible to others).
- Shares the same connection registry as OpenObserve DB ingestion: sources configured for collection are immediately usable here.

### Running SQL
1. Pick a source and an account on the left;
2. Enter SQL (Ctrl+Enter to run) -> the result grid shows columns and data, affected rows / elapsed time;
3. **Export CSV** and query history with one-click refill are built in.

### Guardrails
- **Read-only mode** per source: only SELECT / SHOW / EXPLAIN / DESC pass;
- **Dangerous-statement interception**: DROP DATABASE/SCHEMA, TRUNCATE and DELETE-without-WHERE are blocked outright (database protection rules);
- **Timeout & row cap**: statement timeout (default 30s) and a row limit (default 1000);
- **Full audit**: every statement records operator, account, SQL text, outcome and source IP.
