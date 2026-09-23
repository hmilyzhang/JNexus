<div align="right"><a href="README.md">English</a></div>

# JNexus — 轻量级运维平台

自研轻量级运维平台：主机管理（一台主机多 OS 账号）、批量命令执行（实时输出）、文件分发、
脚本中心、发布流水线（支持回滚）、可配置 RBAC 权限、审计日志、危险命令拦截、LDAP 认证、
按用户开启的 **MFA（TOTP 两步验证）**。数据存储于外部 PostgreSQL。

技术栈：Go（Gin + GORM）+ PostgreSQL + Vue3（Element Plus + xterm.js）。

**文档：** [项目说明文档](docs/README.md) —— 按模块的完整指南、任务跟踪与 [API 使用文档](docs/API.zh-CN.md)（通过 API 密钥集成外部系统，`/api/ext/*`）。

**在线文档：** 部署后直接访问 `http://<服务器>:8080/docs/api`（中英双语，无需登录）。

## 功能总览

| 模块 | 能力 |
|------|------|
| 仪表盘 | 登录后首页：纳管/在线主机、分组、任务、应用、发布、用户、启用规则统计 + 当前用户信息与快捷入口 |
| 资产管理 | 多级分组树、CSV 批量导入（NAME,IP,port,user,group，页面统一密码、自动推送公钥配对）、并发探活、每主机多 OS 账号（标签/默认标记）；工具栏批量移动分组；删除主机级联清理其 OS 账号；云资产同步：AWS / Azure / 华为云通过 AK/SK 或服务主体发现并导入云主机（每账号区域、定时同步、按标签分组、同 IP 冲突处理、云端删除联动、可选账号模板——导入即创建默认 OS 账号，开箱可用）；Windows 主机：WinRM（PowerShell）命令执行、CIM 指标采集、SMB 文件分发、Set-LocalUser 密码轮换、guacamole-lite + guacd 网关网页内 RDP（多账号登录选择器，支持 域\用户 与 UPN；凭据不经浏览器） |
| 批量执行 | 树形选主机 / 多 IP 直输、OS 账号选择、并发执行、WebSocket 实时逐主机输出、成功/失败计数与进度 |
| 任务中心 | 聚合任务视图：主机表（账号、状态、退出码、耗时、失败优先）、输出面板、只看失败、跨主机输出关键字搜索、导出汇总 .log / CSV |
| 计划任务 | Cron 定时在选定主机执行命令或脚本；启用/停用、立即执行、历史留痕、邮件通知 |
| 采集报告 | 预设采集模板——服务器账号、crontab 列表、健康检查、系统信息、端口&证书（监听端口、HTTP/HTTPS 协议识别、HTTPS 与本地证书到期）——跨主机执行（目标为空 = 全部主机；**双版本预设**：Linux 经 SSH 走 bash、Windows 经 WinRM 走 PowerShell，按主机系统自动分发）、逐主机查看结果、合并 HTML 导出（全部主机一份可打印文档）与跨主机端口&证书矩阵、.log / CSV 导出；账号预设另有跨主机账号矩阵；模块访问按角色控制 |
| 账号凭证 | 全部主机 OS 账号统一管理：按主机/关键字/轮换状态筛选、添加/编辑/轮换/查看（管理员）、密码自动轮换（长度/复杂度/周期可配）——普通账号经同主机 root/NOPASSWD sudo 特权链完成轮换；密码历史（管理员可查、记审计、每账号保留 24 条）；账号模板一次性录入 LDAP/AD 密码，添加主机或批量导入时复用；批量添加/删除带引用保护 |
| 文件分发 | 上传 → 并发 SFTP 到多主机（树/IP 选择）、实时进度 |
| 脚本中心 | CRUD + 一键批量执行；双版本脚本（bash + PowerShell 双标签）按主机系统自动分发，无 PowerShell 版本的 Windows 主机自动跳过；危险命令拦截覆盖两个版本 |
| 发布中心 | 应用 → 主机绑定（含发布 OS 账号）；流水线：停止 → 时间戳备份 → 上传 → 启动 → 健康检查；一键回滚至最近备份 |
| 用户与权限 | 用户 + 用户组两个页签。角色：admin / ops / publisher / viewer / auditor；用户组关联成员、主机、主机组、OS 账号与账号规则（主机范围 × 用户名，自动覆盖未来主机）。用户行含审计字段：创建人、最后修改人/时间、禁用时间/操作人。每用户 MFA 状态与管理员重置。自定义角色：复制现有角色并编辑能力矩阵（模块 × 动作），所有 API 路由强制校验。团队数据级权限：用户组绑定成员 + 主机组 + OS 账号 + 应用——成员只能看到并发布本组应用，可选主机可见性限制 |
| Web 终端 | 全屏终端工作区（Linux/SSH）：会话标签页置顶、资产树与主机页一致（未分组节点、Windows 主机置灰并一键打开 RDP）、主机下列出全部可用账号、布局变化自动重排、多会话并发 |
| 监控中心 | 主机资源（CPU / 内存 / 磁盘，SSH 每 60 秒采集，进度条 + 趋势图）+ 应用监控（Uptime Kuma 风格）：HTTP(s)（方法/状态码范围/关键字包含或不存在）、TCP 端口、Ping——间隔/超时可配、Up/Down 状态、心跳条、24h 可用率、立即检测、暂停。页面六个页签：CMD 监控（可分组/筛选）、应用监控、告警配置——通知渠道（系统 SMTP 邮件、Webhook、企业微信/钉钉/飞书机器人、Telegram）支持测试发送、按监控绑定；状态变化即触发通知。告警规则页签：全局规则（故障持续秒数阈值——0 立即告警——恢复通知开关）适用于所有监控项。维护窗口页签（日历日期范围 + 起止时间、支持跨午夜）：窗口内故障不计入可用率、心跳置灰、不告警；实时横幅显示当前是否有窗口生效；每次变更加入变更日志（管理员可删除/清空）。AI 诊断页签（管理员）：disk/mem/cpu 告警自动采集主机状态（Linux SSH / Windows WinRM）交 AI 分析，结果推送该级别绑定通道；应用监控宕机触发服务端网络探测（DNS / TCP / HTTP 证书 / Ping），目标匹配纳管主机时追加 SSH 采集，送达监控绑定通道；生效级别/指标、冷却与诊断提示词可配；磁盘告警可执行管理员定义的受控清理命令（逐字执行、全量审计）。模板设置页签：五类通知来源的默认模板可查看并编辑占位符。渠道支持可视化模板编辑——邮件渠道分别编辑标题与正文模板，其余渠道编辑消息模板；占位符发送时渲染，测试发送可预览。CMD 严重级别 P1-P4：按指标 CPU/内存/磁盘阈值 + 持续时长、通知渠道与自定义消息模板（占位符 {level} {host} {ip} {metric} {value} {threshold} {time}）；升级与恢复自动播报。主机重启自动检测（采集流程中 boot_id 变化，Linux 主机）并立即推送所有启用渠道。采样保留 30 天；趋势抽屉 6h/24h/7d/30d 范围（超 48h 按小时聚合）；HTTPS 证书生命周期纳入应用监控（到期天数、告警规则到期阈值）。个人中心（用户菜单）展示基本信息、自助邮箱/密码与 MFA 开关 |
| 可观测（OpenObserve） | 可选集成 OpenObserve（开源，AGPL-3.0）：主机指标、任务执行输出、告警事件、Windows 事件日志与数据库审计双写长期存储与全文检索——轻量内置监控在 OpenObserve 宕机时照常工作。日志检索菜单：流选择器（内置自动列出、自定义流自动发现）、SQL 查询编辑器（按流字段提示）、1h/24h/7d/30d + 自定义时间范围、动态列表格、CSV 导出。可观测管理页（系统管理员）：连接健康与延迟探测、按流启用开关 + 推送统计（成功/失败/最近推送/最近错误）、自定义推送测试器与外部推送 API 指南。自定义摄入：POST /api/ext/oo/{stream}（API Key 认证、审计、限流）允许外部系统向任意流推送 JSON；账号 100% JNexus——最终用户无需接触 OpenObserve UI；集成停用时菜单自动隐藏 |
| MFA | TOTP 两步验证（RFC 6238，兼容 Google/Microsoft Authenticator）：用户菜单扫码自助绑定，登录密码步骤后需输入 6 位动态码，可凭动态码自助解绑；丢失设备管理员重置；密钥 AES-256-GCM 加密 |
| Kubernetes | 多集群接入：粘贴 kubeconfig（自动解析 API Server / CA / 客户端证书，支持 base64 或 PEM）或 CA + 客户端证书三件套；凭据 AES-256-GCM 加密、绝不回显。证书到期跟踪红/黄高亮，到期前 30/7 天自动提醒告警通道；周期探测显示集群在线状态与版本。集群成员（管理员/用户/查看者）+ 全局 K8S 查看/管理角色权限。专属全屏管理页（管理按钮）左侧栏：概览（15 类资源计数卡 + 最近事件）、集群（节点实时 CPU/内存、命名空间）、工作负载（Pods 日志/页内 Shell 抽屉多容器分屏/删除；Deployments 与 StatefulSets 扩缩容、滚动重启；DaemonSets、Jobs；CronJobs 创建/暂停/恢复/删除）、服务发现（Services、Ingresses）、配置（ConfigMaps、Secrets）、存储（PVC、PV、StorageClasses）、访问控制（ServiceAccounts）、Helm Releases（chart/版本/revision/状态——从 release secrets 解码）。YAML 查看/编辑（审计、名称命名空间保护）/下载覆盖全部资源；Pod 日志 WebSocket 实时跟随（日志抽屉）；全局命名空间选择器作用于所有页签；资源用量来自 metrics-server（缺失显示 -）。容量规划页签：15 分钟采样保留 400 天，30d/180d/1y 趋势图（用量 vs 容量 80% 线）+ 线性回归预测剩余天数；Warning 事件聚合告警（20h 冷却）与 NotReady 节点检测推送告警通道。月度运营报表页签：容量红/黄/绿风险灯、逐主机与逐集群容量预测、告警统计 P1/P2 时间线、配置变更摘要——可打印 PDF、告警明细导出 CSV |
| Web 应用（PAM） | 托管常用 Web 管理台（地址 + 账号密码，AES 加密）。打开 = 全屏无头浏览器会话：服务器自动填入托管账密并登录，画面实时串流，鼠标键盘直接操作——密码全程不经过用户电脑。会话启动记审计，单会话上限 30 分钟，菜单按角色授权 |
| 数据库工作台 | 注册 MySQL / SQL Server / PostgreSQL 数据库源（凭据 AES 加密，与 OpenObserve 采集共用）。网页 SQL 编辑器带 schema 感知补全（表/列/关键字，CodeMirror 6，主题自适应），结果表格带类型徽标与行号、CSV 导出、查询历史；每源只读模式、危险 SQL 拦截、语句超时与行数上限；每源多账号并按用户组限制使用；每条语句全量审计 |
| 审计日志 | 全部写操作留痕（谁 / 动作 / 资源 / 来源 IP / 状态），任务输出完整保留；仅管理员与审计员可见 |
| 危险命令 | 正则规则库（rm -rf、mkfs、dd、shutdown、drop database… 内置 11 条），在执行/脚本/发布入口拦截并写审计；管理员可编辑与测试 |
| 邮件（SMTP） | SMTP 设置（SSL / STARTTLS、认证、密码打码）、测试发送；任务完成通知邮件含成功/失败计数与逐主机结果表（失败任务含输出片段） |
| 系统设置 | 页签：常规（系统名称）、LDAP 认证（服务器、组成员校验、连接测试）、邮件 SMTP、密码轮换（全局开关、密码长度/复杂度/默认周期）、API Keys、配对密钥（平台公钥 + 轮换配置：开关/间隔/立即轮换、后台执行带审计；分页配对凭据列表关键字搜索）、角色设置（每角色可编辑描述/菜单可见性/主机权限/OS 账号权限/报表权限/K8S 权限）、可观测（OpenObserve 地址/组织/加密凭据 + 连接测试） |
| API 集成 | API Keys（系统设置 → API Keys）让外部系统调用 /api/ext/*：主机实时状态/资源、监控状态与 24h 可用率、任务结果、异步批量执行（wait 选项）、自定义数据推送 OpenObserve（POST /ext/oo/{stream}）。Key 绑定用户并继承其角色与数据权限；SHA-256 哈希存储（仅显示一次）、可选过期 + IP 白名单、启用/停用、每 Key 限流（120/分钟）、全部调用审计 |
| i18n | 中文 / English 右上角切换 |

## 快速开始（本地开发）

### 1. 准备 PostgreSQL

任意外部 PG 均可，例如 Docker 起一个：

```bash
docker run -d --name jnexus-pg -e POSTGRES_USER=jnexus -e POSTGRES_PASSWORD=jnexus123 \
  -e POSTGRES_DB=jnexus -p 5432:5432 postgres:16-alpine
```

### 2. 配置后端

```bash
cd backend
cp config.example.yaml config.yaml
# 生成 AES 主密钥（用于加密落库的 SSH 私钥）
./jnexus-server -genkey   # 或 openssl rand -base64 32
# 编辑 config.yaml：填入 dsn / jwt_secret / aes_key
```

### 3. 启动

```bash
# 编译启动（自动建全部表 + 种子数据）
cd backend && go build -o jnexus-server ./cmd/server && ./jnexus-server -config config.yaml

# 前端开发联调
cd frontend && npm install && npm run dev   # http://localhost:5173 代理到 8080
```

访问 http://localhost:8080（前端构建产物 `frontend/dist` 由后端直接托管）。

**默认账号：`admin` / `admin123`**（请立即修改）。

### 4. 生产部署（Docker Compose）

捆绑数据库（JWT/AES 密钥首次启动自动生成并持久化在 `data` 卷；如需指定，设置 `JNEXUS_JWT_SECRET` / `JNEXUS_AES_KEY` 环境变量覆盖）：

```bash
cd deploy
# 版本号：无需设置——仓库 VERSION 文件（当前 2.x）会随镜像烤入并作为版本来源
# 如需覆盖：export JNEXUS_VERSION="2.x" 后再构建
docker compose up -d --build
```

可选可观测后端（[OpenObserve](https://openobserve.ai)，单容器，默认关闭）：

```bash
docker compose --profile observability up -d --build
# 然后在 JNexus：系统设置 → 可观测性集成 → 启用，服务地址 http://openobserve:5080、
# 组织 default、凭据 = 你配置的 ZO_ROOT_USER_EMAIL / ZO_ROOT_USER_PASSWORD。
# 集成启用且可达后，「日志检索」菜单自动出现。
```

#### 反向代理（HTTPS）——必须支持 WebSocket

如果你通过反向代理以 HTTPS 对外提供 JNexus，代理必须转发 **WebSocket 升级**——Web 终端、任务实时输出、日志跟随与 RDP 网关代理（`/rdp-gw`）均使用 WebSocket。nginx 示例：

```nginx
location / {
    proxy_pass http://127.0.0.1:8080;
    proxy_http_version 1.1;
    proxy_set_header Upgrade $http_upgrade;      # 必需：WebSocket 升级
    proxy_set_header Connection "upgrade";       # 必需：WebSocket 升级
    proxy_set_header Host $host;
    proxy_set_header X-Forwarded-Proto https;    # 告知 JNexus 当前为 HTTPS
    proxy_read_timeout 3600s;
}
```

缺少 `Upgrade`/`Connection` 转发时，登录与页面正常，但 RDP/Web Shell 会立即断开。

外部 PostgreSQL——启动时自动创建全部表结构（仅需预先建库）：

```bash
export JNEXUS_DB_HOST=10.3.0.100
export JNEXUS_DB_PORT=5432
export JNEXUS_DB_USER=jnexus
export JNEXUS_DB_PASSWORD=yourpass
export JNEXUS_DB_NAME=jnexus        # 需先 CREATE DATABASE
export JNEXUS_VERSION="2.0"   # 可选：注入精确版本（缺省用仓库 VERSION 文件）
docker compose -f docker-compose.external.yml up -d --build
# JWT/AES 密钥自动生成并持久化在 `data` 卷；如需指定，设置 JNEXUS_JWT_SECRET / JNEXUS_AES_KEY 覆盖
```

优先级：`JNEXUS_DSN` > `JNEXUS_DB_HOST/PORT/USER/PASSWORD/NAME` > `config.yaml`（本地二进制同样支持）。

## 使用说明

1. **SSH 接入**：先导入私钥（资产管理 → SSH 密钥）并把对应公钥写到目标机，或使用密码认证。
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
   （不选主机默认采集全部），逐主机结果留痕，支持导出 `.log` / CSV；全部主机结果可合并导出为一份可打印 HTML 文档，端口&证书预设另有跨主机端口&证书矩阵。「端口与证书检查」列出监听端口，
   对每个端口做 TLS 握手 + HTTP HEAD 探测识别 http / https / other，并检查过期证书——包括 HTTPS 端口
   实际下发的证书与 `/etc/ssl`、`/etc/pki` 等目录下的本机证书文件。
8. **密码轮换**：账号凭证页按账号开启轮换并设置周期；随机强密码加密保存、不明文展示；
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
15. **可观测性 / 日志检索**：在 系统设置 → 可观测性集成 启用 OpenObserve（或 compose 加 `--profile observability`）。此后主机指标、任务输出、告警事件实时双写；**日志检索**菜单用 SQL 查询全部数据（流选择、时间范围、CSV 导出），**可观测集成**管理页展示连接健康与流级开关/推送计数。外部系统用 API 密钥调 `POST /api/ext/oo/{stream}` 推送自定义数据——推送后立即可检索。

16. **Web 应用（PAM）**：在「Web 应用」登记常用管理台（地址 + 账密，AES 加密存储）。点击「打开」由服务器启动无头浏览器会话，自动填入托管账密并登录，页面实时串流——用本地鼠标键盘直接操作，密码全程不经过用户电脑。会话启动记审计，单次上限 30 分钟。
17. **数据库工作台**：在「资产管理 → 数据库」注册 MySQL / SQL Server / PostgreSQL 源（每源可登记多个账号并按用户组授权，如监控用只读、DBA 用读写）。SQL 编辑器带 schema 感知补全，结果表格支持 CSV 导出；支持每源只读模式、危险 SQL 拦截、语句超时与行数上限，每条语句全量审计。

## Windows 接入（WinRM + RDP）

1. 添加主机时 **OS 类型选 Windows**（默认 WinRM 5985、RDP 3389），账号填本地或域账号（支持 `DOMAIN\user`）。
2. 命令执行、指标采集与告警走 **WinRM**；传输自动协商——5986(HTTPS) 可达时优先加密通道（Basic 认证，不受 NTLM 硬化策略影响），否则回退 HTTP 5985（NTLM 认证，本地/域账号皆可）。**Kerberos（域环境，CIS 合规）**：主机表单启用「Kerberos 认证」开关（账号用 `user@REALM` 格式，Realm 自动取 UPN 后缀或系统配置 `winrm_krb5_realm`，可选 SPN 覆盖）。Kerberos 经 WinRM HTTPS + gokrb5 运行——不用 Basic、不用 NTLM、目标机无需放开 UAC 过滤；服务器需能读取 `krb5.conf`（挂载进容器或配置 `winrm_krb5_config`）。
3. **浏览器内 RDP**：依赖 `guacd` + `rdp-gateway` 两个 sidecar 容器（compose 已内置）。点击 Windows 主机行的 **RDP** —— 平台签发 5 分钟有效的加密连接串，凭据全程不经过浏览器明文。
4. 目标机一次性配置（管理员 PowerShell）：

```powershell
winrm quickconfig
```

5. **启用了 NTLM 硬化策略的工作组机器**（新版 Windows 可能在登录成功后仍拒绝远程 NTLM 会话——症状：HTTP 401 且目标机安全日志无失败登录事件）：为 WinRM 配置 HTTPS——JNexus 自动探测 5986 端口并切换 Basic over TLS，平台侧无需任何额外改动。

```powershell
$cert = New-SelfSignedCertificate -DnsName "<主机IP>" -CertStoreLocation Cert:\LocalMachine\My
New-WSManInstance -ResourceURI winrm/config/Listener -SelectorSet @{Address="*"; Transport="HTTPS"} `
  -ValueSet @{Hostname="<主机IP>"; CertificateThumbprint=$cert.Thumbprint; Enabled="true"}
Set-Item -Path WSMan:\localhost\Service\Auth\Basic -Value $true
New-NetFirewallRule -DisplayName "WinRM HTTPS" -Direction Inbound -Protocol TCP -LocalPort 5986 -Action Allow
# 本地账号还需放开 UAC 远程令牌过滤（或加入 "Remote Management Users" 组）
reg add "HKLM\SOFTWARE\Microsoft\Windows\CurrentVersion\Policies\System" /v LocalAccountTokenFilterPolicy /t REG_DWORD /d 1 /f
```

仓库附带 `wintest` 诊断探针（`backend/cmd/wintest`），可在平台之外复现原始 WinRM 连接，便于排查。

## 安全设计

- SSH 私钥与密码使用 AES-256-GCM 加密落库，主密钥由配置注入（环境变量可覆盖）。
- TOTP（MFA）密钥使用同一主密钥加密；密码校验通过后发放的 `mfa_token` 仅 2 分钟有效、
  不可调用业务 API，须凭验证通过的动态码换取正式 JWT。
- 密码使用 bcrypt 存储；JWT 有效期 24h。
- 危险命令在「批量执行 / 脚本执行 / 发布启停」入口统一拦截，命中返回 403 并记录审计。
- 审计日志自动脱敏（password/content 字段）。
- 接口权限由后端强制，菜单可见性仅控制界面显示。
- **AI 安全（Security by Design）**：
  - AI 助手按角色能力位授权（`ai:chat`，只读/审计角色默认排除），前端入口与后端接口双重校验；
  - 聊天接口每用户限流（默认 30 次/小时，可在 AI 设置中调整），防成本滥用与刷接口；
  - 注入防护：注入到提示词的实时状态与主机采集数据（SSH/WinRM/网络探测）以 `UNTRUSTED` 标记包裹，
    系统提示词附加最高优先级安全规则（数据块内指令不可信、不泄露提示词与凭据、拒绝越权请求）；
  - 实时状态快照按用户主机权限过滤，无权限主机仅显示数量、不暴露明细；
  - AI 回复经 DOMPurify 消毒后渲染，外链统一 `rel="noopener noreferrer nofollow"`；
  - AI 永不生成可执行命令：告警清理命令逐字来自管理员目录（禁止 `;&|`），全程审计；
  - 聊天提问经全局审计中间件留痕（操作人/来源 IP/请求摘要）。

## 目录结构

```
backend/     Go 后端（cmd/server 入口，internal/ 分层）
frontend/    Vue3 前端（src/views 按模块分页面，src/i18n 中英文案）
deploy/      docker-compose.yml（捆绑库）+ docker-compose.external.yml（外部库）+ Dockerfile
.smoke/      本地联调测试桩：假 SSH/SFTP 服务端、假 LDAP 服务端、假 SMTP 服务端、K8S mock apiserver
```

## 已知边界（MVP）

- SSH 交互终端与批量执行按「内网可信」设计，未做目标机 host key 校验。
- 文件分发为单文件粒度（目录分发可循环调用或后续迭代）。
- 任务无分布式调度（单实例执行）；多实例部署需引入任务队列。


## 开源协议

[GNU AGPL-3.0](LICENSE) © 2026 hmilyzhang

JNexus 是自由软件：你可以在 GNU Affero 通用公共许可证 v3.0 的条款下运行、研究、修改与再分发。
若你修改 JNexus 并将其作为网络服务对外提供，必须以同一许可证公开修改后的源码（AGPL §13）。
可选的 [OpenObserve](https://openobserve.ai) 后端同为 AGPL-3.0（仅通过 HTTP 调用——与之并列部署不会带来额外义务）。

<div align="right"><a href="README.md">English</a></div>

