<div align="right"><b>中文说明在下方</b></div>

# JNexus — Lightweight Self-Hosted Ops Platform

A lightweight, self-built operations platform for small/medium teams: **asset & account management, batch execution, monitoring & alerting, application release, Kubernetes management, database workbench and PAM-style web-app vault** — in a single Go binary + Vue3 frontend, backed by an external PostgreSQL. No agent on managed hosts: everything works over SSH / WinRM.

## Highlights

- **Assets & accounts** — multi-level host groups, several OS accounts per host (password / SSH key), password auto-rotation, force key pairing, CSV import, cloud sync from AWS / Azure / Huawei Cloud
- **Batch execution** — concurrent commands & scripts with live per-host output (SSH for Linux, WinRM PowerShell for Windows), file distribution over SFTP/SMB, script center with dangerous-command interception
- **Monitoring & alerting** — Uptime-Kuma-style app monitors (HTTP/TCP/Ping, heartbeat bars, uptime %), host CPU/memory/disk collection, HTTPS certificate expiry tracking, P1–P4 severity levels, channels: Email / Webhook / WeCom / DingTalk / Feishu / Telegram; maintenance windows; **AI diagnostics** on alerts (analysis + controlled cleanup)
- **In-browser access** — SSH web terminal (multi-tab) and **RDP via guacamole** with account picker; PAM-style web-app vault auto-fills credentials in a headless browser — passwords never reach the user
- **Release center** — app → host pipelines (stop → backup → upload → start → health check) with one-click rollback
- **Kubernetes** — multi-cluster onboarding (kubeconfig / certs), full resource management with YAML editing, pod shell & logs, Helm releases, 400-day capacity planning with forecasts
- **Database workbench** — DBeaver-style navigator (MySQL / PostgreSQL / SQL Server / Oracle), schema-aware SQL editor, per-source read-only mode, **account password rotation** for the database itself
- **Security & compliance** — RBAC with custom roles (capability matrix), team-level data isolation, full audit log, LDAP, per-user MFA (TOTP), AES-256-GCM encrypted credentials
- **Observability** — optional OpenObserve integration (dual-write metrics/task output/alert events/Windows events/DB audit, full-text log search); keeps working standalone when OpenObserve is down
- **Bilingual** — Chinese / English UI; online API docs at `/docs/api`

## Quick Start

```bash
mkdir jnexus && cd jnexus
curl -O https://raw.githubusercontent.com/hmilyzhang/JNexus/main/docs/dockerhub-compose.yml
docker compose up -d
```

Or with Docker CLI (database required):

```bash
docker network create jnexus
docker run -d --name jnexus-pg --network jnexus -e POSTGRES_USER=jnexus \
  -e POSTGRES_PASSWORD=jnexus123 -e POSTGRES_DB=jnexus postgres:16-alpine
docker run -d --name jnexus --network jnexus -p 8080:8080 \
  -e JNEXUS_DB_HOST=jnexus-pg -v jnexus-data:/app/data hmilyzhang/jnexus:latest
```

Open **http://localhost:8080** — default account `admin` / `admin123` (**change it immediately**). JWT/AES secrets are auto-generated on first start and persisted in the `data` volume.

Tags: `latest` follows the newest release; pinned versions (e.g. `2.109`) are available too.

## Docs

Full documentation, screenshots guide and API reference: [GitHub project](https://github.com/hmilyzhang/JNexus)

---

<div align="right"><a href="#jnexus--lightweight-self-hosted-ops-platform">English above</a></div>

# JNexus — 轻量自建运维平台

面向中小团队的一体化运维平台:**资产与账号管理、批量执行、监控告警、应用发布、K8S 管理、数据库工作台、PAM 式应用代填**,单个 Go 二进制 + Vue3 前端,外部 PostgreSQL 存储。**被管主机无需安装任何 Agent**,全部通过 SSH / WinRM 完成。

## 核心能力

- **资产账号**:多级主机组、一机多账号(密码/密钥)、密码自动轮换、强制密钥配对、CSV 导入、AWS/Azure/华为云资产同步
- **批量执行**:并发命令/脚本、逐主机实时输出(Linux 走 SSH,Windows 走 WinRM PowerShell)、SFTP/SMB 文件分发、脚本中心、危险命令拦截
- **监控告警**:Uptime-Kuma 风格应用监控(HTTP/TCP/Ping、心跳条、可用率)、主机 CPU/内存/磁盘采集、HTTPS 证书到期跟踪、P1–P4 分级告警;通道支持邮件/Webhook/企业微信/钉钉/飞书/Telegram;维护窗口;**AI 告警诊断**(自动采集 + AI 分析 + 受控清理)
- **浏览器直达**:SSH Web 终端(多标签)、**RDP 网页化**(guacamole,多账号选择器);PAM 式应用代填——无头浏览器自动登录,密码全程不落地
- **发布中心**:应用→主机流水线(停止→备份→上传→启动→健康检查),一键回滚
- **K8S 管理**:多集群接入(kubeconfig/证书)、全资源管理、Pod Shell 与日志、Helm、400 天容量规划与预测
- **数据库工作台**:DBeaver 风格导航(MySQL/PostgreSQL/SQL Server/Oracle)、SQL 编辑器、只读模式、**数据库账号密码轮换**
- **安全合规**:自定义角色 RBAC、团队数据隔离、全量审计、LDAP、TOTP 双因素、凭据 AES-256-GCM 加密
- **可观测**:可选 OpenObserve 集成(指标/任务输出/告警事件/Windows 事件/DB 审计双写、日志检索);OpenObserve 宕机不影响内置监控
- **中英双语**,内置 API 文档(`/docs/api`)

## 快速开始

```bash
mkdir jnexus && cd jnexus
curl -O https://raw.githubusercontent.com/hmilyzhang/JNexus/main/docs/dockerhub-compose.yml
docker compose up -d
```

打开 **http://localhost:8080**,默认账号 `admin` / `admin123`(**请立即修改**)。JWT/AES 密钥首次启动自动生成并持久化到 `data` 卷。

标签:`latest` 跟随最新版,同时提供固定版本号(如 `2.109`)。

## 文档

完整文档与 API 说明:[GitHub 项目主页](https://github.com/hmilyzhang/JNexus)
