<div align="right"><a href="README.md">English</a></div>

# JNexus — 轻量级运维平台

自研轻量级运维平台：主机管理（一台主机多 OS 账号）、批量命令执行（实时输出）、文件分发、
脚本中心、发布流水线（支持回滚）、可配置 RBAC 权限、审计日志、危险命令拦截、LDAP 认证、
按用户开启的 **MFA（TOTP 两步验证）**。数据存储于外部 PostgreSQL。

**By JJ Zhang · Version 1.0**

技术栈：Go（Gin + GORM）+ PostgreSQL + Vue3（Element Plus + xterm.js）。

**文档：** [API 使用文档](docs/API.zh-CN.md) —— 通过 API 密钥集成外部系统（`/api/ext/*`）。

**在线文档：** 部署后直接访问 `http://<服务器>:8080/docs/api`（中英双语，无需登录）。

## 功能一览

| 模块 | 能力 |
|------|------|
| 仪表盘 | 登录首页：纳管/在线主机、分组、任务、应用、发布单、用户、启用规则等统计 + 当前用户信息、快捷入口 |
| 主机管理 | **多级分组树**、批量导入（`名称,IP,端口,用户名,分组名` + 页面统一密码 + 自动配对密钥）、并发连通性探测、**一台主机多 OS 账号**（用途标签、默认标记）、Web 终端 |
| 批量执行 | 树状选主机 / 多 IP 输入、**OS 账号选择**、并发执行、WebSocket 实时逐主机输出、成功/失败计数与进度条 |
| 任务控制台 | 任务聚合视图：主机表格（账号、状态、退出码、耗时、失败优先）、输出面板、只看失败、跨主机输出关键字搜索、**导出汇总 .log / CSV** |
| 计划任务 | Cron 定时执行命令或脚本：常用周期模板、启停、立即执行、执行历史、邮件通知 |
| 采集报告 | 预设模板采集：**服务器账号信息** / **Crontab 定时任务** / **健康检查** / **系统信息** / **端口与证书检查**（监听端口、HTTP/HTTPS 协议探测、HTTPS 与本机证书过期检查）；不选主机默认采集全部；逐主机结果留痕，支持导出 `.log` / CSV；模块权限按角色分配 |
| 主机账号 | 独立**主机账号**页（跨主机全部账号）：按主机/关键字/轮换状态筛选，新增/编辑/轮换/查看密码（管理员），**密码自动轮换**（长度/复杂度/周期可配，LDAP 账号自动跳过），显示最后密码修改时间 |
| 文件分发 | 浏览器上传 → 并发 SFTP 分发（树状/IP 选择），进度实时可见 |
| 脚本中心 | 脚本增删改查 + 一键批量执行 |
| 发布中心 | 应用绑定主机（可选发布账号）；流水线：停止 → 时间戳备份 → 上传 → 启动 → 健康检查；失败一键回滚到最近备份 |
| 用户管理 | 双 Tab：用户管理 + 组管理。角色：管理员 / 运维 / 发布员 / 只读 / **审计员**；用户组可关联成员、主机、主机分组、**OS 账号**与**账号规则**（主机范围 × 账号名，自动覆盖新增主机）。用户行带审计字段：创建人/时间、最近修改人/时间、禁用时间/操作人；**MFA 状态**列 + 管理员重置 |
| Web 终端 | 全屏终端工作台：资产树按可用 OS 账号展开叶子、多终端会话并存、按账号连接 |
| **监控中心** | **主机资源**（CPU / 内存 / 磁盘，经 SSH 每 60 秒采集，进度条 + 近 6 小时趋势图）+ **应用监控**（参考 Uptime Kuma）：HTTP(s)（请求方法、允许状态码范围、关键字包含/不包含）、TCP 端口、Ping —— 间隔/超时可配，正常/故障状态、心跳条、24h 可用率、立即检测、暂停；采样保留 7 天（主机资源 24 小时）。页面分三个标签：**CMD 监控**（可按分组筛选）、**应用监控**、**Alert 配置**——通知通道（邮件（系统 SMTP）、Webhook、企业微信 / 钉钉 / 飞书机器人、Telegram），支持测试发送、按监控项绑定；状态变化时自动推送。**报警规则**标签为全局设置（对所有监控项生效）：故障持续阈值秒数（0 = 立即告警）与恢复通知开关。**主机系统重启自动检测**：资源采集时比对 boot_id，发现重启立即向所有启用通道推送，无需人工标注（Linux 主机）。**模板设置**标签：五类通知源（应用监控告警/恢复、主机重启、CMD 分级告警/恢复）的默认模板可视化编辑。通道支持**可视化模板编辑**——邮件通道分别编辑标题/正文模板，其他通道编辑消息模板；发送时渲染占位符，测试发送即可预览效果。**CMD 分级阈值 P1-P4**：每级可分别设置 CPU / 内存 / 磁盘阈值、持续时长、通知通道与自定义通知模板（占位符 {level} {host} {ip} {metric} {value} {threshold} {time}）；升级与恢复自动播报；支持全局**维护窗口**（对所有监控项生效；窗口内故障不计入可用率、灰色心跳、不触发告警）。采样保留 30 天，趋势抽屉支持 6h/24h/7d/30d 范围（超 48h 按小时聚合）。新增**个人中心**页面（用户菜单 → 个人中心）：基本信息、邮箱/密码自助维护与 MFA 开关 |
| **MFA** | **TOTP 两步验证**（RFC 6238，兼容 Google/Microsoft Authenticator）：用户菜单扫码自助绑定，登录密码后需输入 6 位动态码，可凭动态码自助关闭，管理员可重置（丢设备）；密钥 AES-256-GCM 加密存储 |
| **Kubernetes** | **多集群纳管**：粘贴 kubeconfig（自动解析 API Server / CA / 客户端证书，兼容 base64 与 PEM）或 CA + 客户端证书三件套；凭据 AES-256-GCM 加密存储、界面不回显。**证书有效期跟踪**（30 天黄 / 7 天红高亮，到期前 30/7 天自动向告警通道推送提醒）；定时探测展示在线状态与版本。**集群级成员权限**（admin / user / viewer）+ 全局 K8S 查看/管理角色权限位。**专属全屏管理页**（列表点「管理」进入），左侧菜单：**概览**（15 项资源计数卡片 + 最近事件）、集群（Nodes 含实时 CPU/内存用量、命名空间）、**工作负载**（Pods 日志 / 页内 Shell 抽屉（多容器、分屏）/ 删除；Deployments **伸缩** 与滚动重启；DaemonSets、StatefulSets、Jobs；CronJobs 新建/暂停/恢复/删除）、**服务发现**（Services、Ingresses）、**配置**（ConfigMaps、Secrets）、**存储**（PVC、PV、StorageClass）、**访问控制**（ServiceAccounts）、**Helm 发布**（Chart/版本/Revision/状态，从 release secret 解码）。所有资源支持**只读 YAML 查看**；全局命名空间选择器对全部页签生效；资源用量来自 metrics-server（未安装时显示 `-`）。Warning 事件聚合告警（20h 冷却）与 NotReady 节点检测推送告警通道 |
| **Kubernetes** | **多集群纳管**：粘贴 kubeconfig（自动解析 API Server / CA / 客户端证书，兼容 base64 与 PEM）或 CA + 客户端证书三件套；凭据 AES-256-GCM 加密存储、界面不回显。**证书有效期跟踪**（30 天黄 / 7 天红高亮，到期前 30/7 天自动向告警通道推送提醒）；定时探测展示在线状态与版本。**集群级成员权限**（admin / user / viewer）+ 全局 K8S 查看/管理角色权限位。**专属全屏管理页**（列表点「管理」进入），左侧菜单：**概览**（15 项资源计数卡片 + 最近事件）、集群（Nodes 含实时 CPU/内存用量、命名空间）、**工作负载**（Pods 日志 / 页内 Shell 抽屉（多容器、分屏）/ 删除；Deployments **伸缩** 与滚动重启；DaemonSets、StatefulSets、Jobs；CronJobs 新建/暂停/恢复/删除）、**服务发现**（Services、Ingresses）、**配置**（ConfigMaps、Secrets）、**存储**（PVC、PV、StorageClass）、**访问控制**（ServiceAccounts）、**Helm 发布**（Chart/版本/Revision/状态，从 release secret 解码）。所有资源支持**只读 YAML 查看**；全局命名空间选择器对全部页签生效；资源用量来自 metrics-server（未安装时显示 `-`）。Warning 事件聚合告警（20h 冷却）与 NotReady 节点检测推送告警通道 |
| 审计日志 | 所有写操作留痕（人 / 动作 / 资源 / 来源 IP / 状态码），任务完整输出可回溯；仅管理员与审计员可见 |
| 危险命令拦截 | 正则规则库（rm -rf、mkfs、dd、shutdown、drop database 等 11 条内置），在执行/脚本/发布入口阻断并写审计；管理员可维护、可测试 |
| 邮件 SMTP | SMTP 配置（SSL / STARTTLS、认证、密码打码）、测试发送；任务完成后自动发送结果摘要邮件（成功/失败计数 + 逐主机结果表，失败附输出片段） |
| 系统配置 | 标签页：基础设置（系统名称）、密码轮换（全局开关、密码长度/复杂度/默认周期）、LDAP 认证（服务器、用户组校验、连接测试）、邮件 SMTP、配对密钥、角色设置（每个角色的描述 / 可见菜单 / 主机权限 查看-新建-编辑-删除 / OS 账号权限 / 报告权限） |
| **API 集成** | **API 密钥**（系统设置 → API 密钥）允许外部系统调用 `/api/ext/*`：主机实时状态/资源、监控状态与 24h 可用率、任务结果查询、异步批量执行（支持等待结果）。密钥绑定用户并继承其角色与数据权限；SHA-256 哈希存储（仅创建时展示一次）、可选过期时间 + IP 白名单、启停开关、每密钥限流（120/分钟）、全部调用入审计 |
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

捆绑数据库（JWT/AES 密钥首次启动自动生成并持久化在 `data` 卷；如需指定，设置 `AUTOOPS_JWT_SECRET` / `AUTOOPS_AES_KEY` 环境变量覆盖）：

```bash
cd deploy
docker compose up -d --build
```

外部 PostgreSQL——启动时自动创建全部表结构（仅需预先建库）：

```bash
export AUTOOPS_DB_HOST=10.3.0.100
export AUTOOPS_DB_PORT=5432
export AUTOOPS_DB_USER=autoops
export AUTOOPS_DB_PASSWORD=yourpass
export AUTOOPS_DB_NAME=autoops        # 需先 CREATE DATABASE
docker compose -f docker-compose.external.yml up -d --build
# JWT/AES 密钥自动生成并持久化在 `data` 卷；如需指定，设置 AUTOOPS_JWT_SECRET / AUTOOPS_AES_KEY 覆盖
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
6. **计划任务**：创建 Cron 定时任务（含常用周期模板）在指定主机上定时执行命令或脚本，
   执行留痕于任务记录并支持邮件通知。
7. **采集报告**：选择预设模板（服务器账号 / Crontab / 健康检查 / 系统信息 / 端口与证书检查）与目标主机
   （不选主机默认采集全部），逐主机结果留痕，支持导出 `.log` / CSV。「端口与证书检查」列出监听端口，
   对每个端口做 TLS 握手 + HTTP HEAD 探测识别 http / https / other，并检查过期证书——包括 HTTPS 端口
   实际下发的证书与 `/etc/ssl`、`/etc/pki` 等目录下的本机证书文件。
8. **密码轮换**：主机账号页按账号开启轮换并设置周期；随机强密码加密保存、不明文展示；
   管理员可查看密码（记录审计）；LDAP/域账号自动检测跳过。全局开关与密码长度/复杂度/默认周期在系统设置配置。
9. **角色**：系统配置 → 角色设置，可配置每个角色的描述、可见菜单、主机权限（查看/新建/编辑/删除）、
   OS 账号管理权限、报告权限。Admin 角色恒为全部权限。
10. **LDAP**：系统配置中启用，可选强制用户组校验（组 Base DN + 过滤器 + 允许组）；
    LDAP 用户首次登录自动建号，角色取系统配置的默认角色。
11. **MFA（TOTP 两步验证）**：右上角用户菜单 → **MFA 安全** 按用户开启。用任意验证器 App
    （Google / Microsoft Authenticator 等）扫码并输入 6 位动态码确认；之后登录需「密码 + 动态码」。
    用户可凭动态码自助关闭；管理员可在用户管理页重置（如设备丢失）。TOTP 密钥 AES-256-GCM 加密存储。
12. **监控中心**：进入「监控中心」——主机资源表展示每台主机的 CPU / 内存 / 磁盘使用率（点行查看近 6 小时趋势）；
    应用监控支持 HTTP(s) 端点、TCP 端口、Ping 三种类型，可配置间隔、超时、关键字与状态码范围；
    结果以正常/故障状态、心跳条与 24h 可用率呈现。采集间隔与模块开关由 `monitor_interval_sec` / `monitor_enabled` 配置控制。
13. **邮件通知**：系统配置中填 SMTP 信息（可一键测试发送）。开启后，批量执行/文件分发/发布/
    批量账号任务结束自动向收件人发送结果摘要邮件，失败主机附输出片段高亮提示。
14. **Kubernetes**：在 **K8S 集群** 添加集群（粘贴 kubeconfig 或 CA + 客户端证书三件套，证书有效期自动跟踪）。
    点「管理」进入全屏集群控制台：概览卡片、工作负载（Pods / Deployments / CronJobs 等）、存储、配置、
    Helm 发布视图；Pod Shell 在页内抽屉打开。行内可对 Deployment 伸缩/滚动重启；成员只能看到自己所属的集群。

## 安全设计

- SSH 私钥与密码使用 AES-256-GCM 加密落库，主密钥由配置注入（环境变量可覆盖）。
- TOTP（MFA）密钥使用同一主密钥加密；密码校验通过后发放的 `mfa_token` 仅 2 分钟有效、
  不可调用业务 API，须凭验证通过的动态码换取正式 JWT。
- 密码使用 bcrypt 存储；JWT 有效期 24h。
- 危险命令在「批量执行 / 脚本执行 / 发布启停」入口统一拦截，命中返回 403 并记录审计。
- 审计日志自动脱敏（password/content 字段）。
- 接口权限由后端强制，菜单可见性仅控制界面显示。

## 目录结构

```
backend/     Go 后端（cmd/server 入口，internal/ 分层）
frontend/    Vue3 前端（src/views 按模块分页面，src/i18n 中英文案）
deploy/      docker-compose.yml（捆绑库）+ docker-compose.external.yml（外部库）+ Dockerfile
.smoke/      本地联调测试桩：假 SSH/SFTP 服务端、假 LDAP 服务端、假 SMTP 服务端
```

## 已知边界（MVP）

- SSH 交互终端与批量执行按「内网可信」设计，未做目标机 host key 校验。
- 文件分发为单文件粒度（目录分发可循环调用或后续迭代）。
- 任务无分布式调度（单实例执行）；多实例部署需引入任务队列。

<div align="right"><a href="README.md">English</a></div>
