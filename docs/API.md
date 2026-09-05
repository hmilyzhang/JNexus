<div align="right"><a href="API.zh-CN.md">中文文档</a></div>

# AutoOps API Reference

REST API for external integrations. All endpoints live under `/api/ext/*` and require an **API key**.

- Base URL: `http://<server>/api/ext`
- Auth header: `Authorization: Bearer aok_<keyID>.<secret>`
- Content type: `application/json`
- Response envelope: `{"ok": true, "data": ...}` on success; `{"ok": false, "error": "..."}` (4xx) on failure

## 1. Getting a key

1. Sign in as admin → **System Settings → API Keys → Create key**
2. Fill in a name, pick the **owner user** (the key runs as this user and inherits its role and data-level permissions), optionally set an expiry and IP allowlist
3. The full key is shown **once** — copy and store it safely

Security model: keys are stored server-side as SHA-256 hashes only; they cannot access user management, system settings or key management endpoints; every call is audited (key name, path, IP, status); per-key rate limit is **120 requests/minute** (HTTP 429 beyond that).

## 2. Endpoints

### GET /hosts — host inventory

```bash
curl -H "Authorization: Bearer aok_xxx.yyy" http://server/api/ext/hosts
```

Response:

```json
{
  "ok": true,
  "data": [
    {
      "id": 7, "name": "web-01", "ip": "10.0.0.8", "port": 22,
      "status": "online", "group": "prod",
      "cpu": 23.9, "mem": 41.2, "disk": 63.5,
      "metrics_at": "2026-09-05T13:30:00+08:00"
    }
  ]
}
```

`cpu/mem/disk/metrics_at` appear only for Linux hosts with resource collection enabled. Hosts the key owner cannot see are omitted.

### GET /monitors — application monitors

```json
{
  "ok": true,
  "data": [
    {
      "id": 3, "name": "Portal", "type": "http", "target": "https://portal.example.com",
      "enabled": true, "status": "up", "resp_ms": 66,
      "last_checked_at": "2026-09-05T13:31:20+08:00", "uptime_24h": 99.8
    }
  ]
}
```

### GET /tasks/:id — task result

Permission: admin/auditor owners see every task; other owners only tasks they created themselves.

```bash
curl -H "Authorization: Bearer aok_xxx.yyy" http://server/api/ext/tasks/42
```

```json
{
  "ok": true,
  "data": {
    "id": 42, "type": "command", "operator": "ops1", "status": "done",
    "params": "{\"command\":\"uptime\",\"hosts\":3}",
    "created_at": "...", "finished_at": "...",
    "results": [
      { "host": "web-01", "ip": "10.0.0.8", "account": "root",
        "status": "success", "exit_code": 0, "output": " 14:00:01 up 45 days ..." }
    ]
  }
}
```

### POST /exec — run a command or script (async)

Owner must have the **admin** or **ops** role. Execution is permission-checked against the owner (host grants, user-group rules, credentials) exactly like the UI.

Request:

```json
{
  "host_ids": [7, 8],
  "command": "df -hP /",
  "timeout_sec": 60,
  "credential_id": null,
  "wait": true
}
```

- `command` **or** `script_id` (script from the script center) is required
- `timeout_sec` default 60
- `wait: true` blocks up to 120 s and returns the finished results; otherwise only the task id

```json
{
  "ok": true,
  "data": {
    "task_id": 43, "status": "done",
    "results": [
      { "host": "web-01", "ip": "10.0.0.8", "status": "success", "exit_code": 0, "output": "..." }
    ]
  }
}
```

Poll afterwards with `GET /tasks/:id` when `wait` is false.

## 3. Errors

| Status | Meaning |
|--------|---------|
| 401 | Missing/invalid/disabled/expired key, IP not in allowlist, or a login JWT was used instead of a key |
| 403 | Owner role/permissions insufficient for the action |
| 404 | Resource not found (or not visible to the owner) |
| 429 | Rate limit exceeded (120 req/min per key) |

Error body: `{"ok": false, "error": "描述"}` or `{"error": "描述"}` for auth failures.

## 4. Python example

```python
import requests

BASE = "http://server/api/ext"
HDRS = {"Authorization": "Bearer aok_xxx.yyy"}

hosts = requests.get(f"{BASE}/hosts", headers=HDRS).json()["data"]
down = [m for m in requests.get(f"{BASE}/monitors", headers=HDRS).json()["data"]
        if m["status"] == "down"]

r = requests.post(f"{BASE}/exec", headers=HDRS,
                  json={"host_ids": [h["id"] for h in hosts if h["name"] == "web-01"],
                        "command": "systemctl status nginx", "wait": True})
print(r.json()["data"]["status"])
```

<div align="right"><a href="API.zh-CN.md">中文文档</a></div>
