<div align="right"><a href="API.md">English</a></div>

# AutoOps API 使用文档

面向外部系统集成的 REST API。所有端点位于 `/api/ext/*`，需要 **API 密钥**。

- 基础地址：`http://<服务器>:8080/api/ext`
- 认证头：`Authorization: Bearer aok_<keyID>.<secret>`
- 内容类型：`application/json`
- 响应格式：成功 `{"ok": true, "data": ...}`；失败 `{"ok": false, "error": "..."}`（4xx）

## 1. 获取密钥

1. 管理员登录 → **系统设置 → API 密钥 → 创建密钥**
2. 填写名称、选择**属主用户**（密钥以该用户身份调用，完全继承其角色与数据级权限），可选设置过期时间与 IP 白名单
3. 完整密钥**仅展示一次**——请立即复制保存

安全模型：服务端仅存 SHA-256 哈希；密钥无法访问用户管理、系统设置与密钥管理接口；每次调用均写入审计（密钥名、路径、IP、状态码）；限流 **120 次/分钟**（超限返回 429）。

## 2. 端点

### GET /hosts — 主机清单

```bash
curl -H "Authorization: Bearer aok_xxx.yyy" http://server:8080/api/ext/hosts
```

响应：

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

`cpu/mem/disk/metrics_at` 仅在 Linux 主机开启资源采集后返回。属主用户无权查看的主机不会出现在结果中。

### GET /monitors — 应用监控

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

### GET /tasks/:id — 任务结果

权限：admin/auditor 属主可查全部；其他属主仅能查自己创建的任务。

```bash
curl -H "Authorization: Bearer aok_xxx.yyy" http://server:8080/api/ext/tasks/42
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

### POST /exec — 执行命令或脚本（异步）

属主需具备 **admin** 或 **ops** 角色。执行权限与界面完全一致（主机授权、用户组规则、凭据隔离）。

请求：

```json
{
  "host_ids": [7, 8],
  "command": "df -hP /",
  "timeout_sec": 60,
  "credential_id": null,
  "wait": true
}
```

- `command` **或** `script_id`（脚本中心的脚本）二选一必填
- `timeout_sec` 默认 60
- `wait: true` 最长等待 120 秒并直接返回结果；否则仅返回任务 ID

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

`wait` 为 false 时，之后用 `GET /tasks/:id` 轮询。

## 3. 错误码

| 状态码 | 含义 |
|--------|------|
| 401 | 密钥缺失/无效/停用/过期、IP 不在白名单，或误用了登录令牌而非密钥 |
| 403 | 属主角色/权限不足以执行该操作 |
| 404 | 资源不存在（或属主不可见） |
| 429 | 超过限流（每密钥 120 次/分钟） |

错误响应体：`{"ok": false, "error": "描述"}`；认证失败为 `{"error": "描述"}`。

## 4. Python 示例

```python
import requests

BASE = "http://server:8080/api/ext"
HDRS = {"Authorization": "Bearer aok_xxx.yyy"}

hosts = requests.get(f"{BASE}/hosts", headers=HDRS).json()["data"]
down = [m for m in requests.get(f"{BASE}/monitors", headers=HDRS).json()["data"]
        if m["status"] == "down"]

r = requests.post(f"{BASE}/exec", headers=HDRS,
                  json={"host_ids": [h["id"] for h in hosts if h["name"] == "web-01"],
                        "command": "systemctl status nginx", "wait": True})
print(r.json()["data"]["status"])
```

<div align="right"><a href="API.md">English</a></div>
