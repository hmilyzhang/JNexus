<div align="right"><a href="README.md">English</a></div>

# AutoOps — 轻量级运维平台

自研轻量级运维平台：主机管理（一台主机多 OS 账号）、批量命令执行（实时输出）、文件分发、
脚本中心、发布流水线（支持回滚）、可配置 RBAC 权限、审计日志、危险命令拦截、LDAP 认证。
数据存储于外部 PostgreSQL。

**By JJ Zhang · Version 1.0**

技术栈：Go（Gin + GORM）+ PostgreSQL + Vue3（Element Plus + xterm.js）。

## 功能一览

| 模块 | 能力 |
|------|------|
| 仪表盘 | 登录首页：纳管/在线主机、分组、任务、应用、发布单、用户、启用规则等统计 + 当前用户信息、快捷入口 |
| 主机管理 | 树状分组视图、批量导入（`名称,IP,端口,用户名,分组名` + 页面统一密码 + 自动配对密钥）、并发连通性探测、**一台主机多 OS 账号**（用途标签、默认标记）、Web 终端 |
| 批量执行 | 树状选主机 / 多 IP 输入、**OS 账号选择**、并发执行、WebSocket 实时逐主机输出、成功/失败计数与进度条 |
| 任务控制台 | 任务聚合视图：主机表格（账号、状态、退出码、耗时、失败优先）、输出面板、只看失败、跨主机输出关键字搜索、**导出汇总 .log / CSV** |
| 文件分发 | 浏览器上传 → 并发 SFTP 分发（树状/IP 选择），进度实时可见 |
| 脚本中心 | 脚本增删改查 + 一键批量执行 |
| 发布中心 | 应用绑定主机（可选发布账号）；流水线：停止 → 时间戳备份 → 上传 → 启动 → 健康检查；失败一键回滚到最近备份 |
| 用户管理 | 双 Tab：用户管理 + 组管理。角色：管理员 / 运维 / 发布员 / 只读 / **审计员**；用户组可关联成员、主机、主机分组、**OS 账号**与**账号规则**（主机范围 × 账号名，自动覆盖新增主机） |
| Web 终端 | 全屏终端工作台：资产树按可用 OS 账号展开叶子、多终端会话并存、按账号连接 |
| 审计日志 | 所有写操作留痕（人 / 动作 / 资源 / 来源 IP / 状态码），任务完整输出可回溯；仅管理员与审计员可见 |
| 危险命令拦截 | 正则规则库（rm -rf、mkfs、dd、shutdown、drop database 等 11 条内置），在执行/脚本/发布入口阻断并写审计；管理员可维护、可测试 |
| 系统配置 | 标签页：基础设置（系统名称）、LDAP 认证（服务器、用户组校验、连接测试）、角色设置（每个角色的描述 / 可见菜单 / 主机权限 查看-新建-编辑-删除） |
| 多语言 | 中文 / English 一键切换（右上角） |

## 快速开始（本地开发）

### 1. 准备 PostgreSQL

任意外部 PG 均可，例如 Docker 起一个：

```bash
docker run -d --name autoops-pg -e POSTGRES_USER=autoops -e POSTGRES_PASSWORD=autoops123 \
  -e POSTGRES_DB=autoops -p 5432:5432 postgres:16-alpine
```

### 2. 配置后端

```bash
cd backend
cp config.example.yaml config.yaml
# 生成 AES 主密钥（用于加密落库的 SSH 私钥）
./autoops-server -genkey   # 或 openssl rand -base64 32
# 编辑 config.yaml：填入 dsn / jwt_secret / aes_key
```

### 3. 启动

```bash
# 编译启动（自动建全部表 + 种子数据）
cd backend && go build -o autoops-server ./cmd/server && ./autoops-server -config config.yaml

# 前端开发联调
cd frontend && npm install && npm run dev   # http://localhost:5173 代理到 8080
```

访问 http://localhost:8080（前端构建产物 `frontend/dist` 由后端直接托管）。

**默认账号：`admin` / `admin123`**（请立即修改）。

### 4. 生产部署（Docker Compose）

捆绑数据库：

```bash
cd deploy
docker compose up -d --build
# 先修改 docker-compose.yml（或环境变量）中的 JWT_SECRET 与 AES_KEY！
```

外部 PostgreSQL——启动时自动创建全部表结构（仅需预先建库）：

```bash
export AUTOOPS_DB_HOST=10.3.0.100
export AUTOOPS_DB_PORT=5432
export AUTOOPS_DB_USER=autoops
export AUTOOPS_DB_PASSWORD=yourpass
export AUTOOPS_DB_NAME=autoops        # 需先 CREATE DATABASE
export AUTOOPS_JWT_SECRET=<随机长字符串>
export AUTOOPS_AES_KEY=$(openssl rand -base64 32)
docker compose -f docker-compose.external.yml up -d --build
```

优先级：`AUTOOPS_DSN` > `AUTOOPS_DB_HOST/PORT/USER/PASSWORD/NAME` > `config.yaml`（本地二进制同样支持）。

## 使用说明

1. **SSH 接入**：先导入私钥（主机管理 → SSH 密钥）并把对应公钥写到目标机，或使用密码认证。
   **批量导入**格式 `名称,IP,端口,用户名,分组名`；登录密码填在页面级「统一密码」并开启
   **自动配对密钥**——平台自动推送公钥到目标机并切换为密钥认证。
2. **OS 账号**：一台主机可挂多个账号（主机行 → OS 账号）。给存量机器批量补账号用工具栏
   **「批量添加账号」**（统一密码 + 自动配对，几百台一次搞定）。
3. **团队账号隔离**：建用户组后，直接「关联 OS 账号」，或添加**账号规则**（主机范围 × 账号名，
   如 `全部主机 × appuser`）。组成员只能用被分配的账号——运维组的 root 规则与应用组的
   appuser 规则互不可见。
4. **批量执行**：树状选主机或直接填 IP，可选 OS 账号，执行后实时看逐主机输出；
   任务控制台聚合所有主机状态，可下载汇总 .log 或导出 CSV。
5. **发布**：先在「应用管理」配置应用（主机 + 部署目录 + jar 名 + 可选启停命令/健康检查/发布账号），
   再到「发布中心」上传 jar 发布；失败可一键回滚到最近备份。
6. **角色**：系统配置 → 角色设置，可配置每个角色的描述、可见菜单、主机权限（查看/新建/编辑/删除）。
   Admin 角色恒为全部权限。
7. **LDAP**：系统配置中启用，可选强制用户组校验（组 Base DN + 过滤器 + 允许组）；
   LDAP 用户首次登录自动建号，角色取系统配置的默认角色。

## 安全设计

- SSH 私钥与密码使用 AES-256-GCM 加密落库，主密钥由配置注入（环境变量可覆盖）。
- 密码使用 bcrypt 存储；JWT 有效期 24h。
- 危险命令在「批量执行 / 脚本执行 / 发布启停」入口统一拦截，命中返回 403 并记录审计。
- 审计日志自动脱敏（password/content 字段）。
- 接口权限由后端强制，菜单可见性仅控制界面显示。

## 目录结构

```
backend/     Go 后端（cmd/server 入口，internal/ 分层）
frontend/    Vue3 前端（src/views 按模块分页面，src/i18n 中英文案）
deploy/      docker-compose.yml（捆绑库）+ docker-compose.external.yml（外部库）+ Dockerfile
.smoke/      本地联调测试桩：假 SSH/SFTP 服务端、假 LDAP 服务端
```

## 已知边界（MVP）

- SSH 交互终端与批量执行按「内网可信」设计，未做目标机 host key 校验。
- 文件分发为单文件粒度（目录分发可循环调用或后续迭代）。
- 任务无分布式调度（单实例执行）；多实例部署需引入任务队列。
