# AutoOps 运维平台

轻量级自研运维平台：主机管理（SSH 密钥登录、分组、并发探测）、批量命令执行（实时输出）、
批量文件分发、脚本中心、发布中心（jar 发布流水线 + 回滚）、RBAC 权限控制、审计日志、
危险命令拦截。数据存储于外部 PostgreSQL。

技术栈：Go（Gin + GORM）+ PostgreSQL + Vue3（Element Plus + xterm.js）。

## 功能一览

| 模块 | 能力 |
|------|------|
| 主机管理 | SSH 密钥/密码登录、分组、文本批量导入、并发连通性探测、Web 终端 |
| 批量执行 | 选主机/分组并发执行，WebSocket 实时推送逐主机输出，结果留档可回看 |
| 文件分发 | 浏览器上传 → 并发 SFTP 分发到多台主机指定目录，进度实时可见 |
| 脚本中心 | 脚本增删改查，一键批量执行（复用执行引擎） |
| 发布中心 | 应用绑定主机与部署路径；发布流水线：停止 → 备份(带时间戳) → 上传 → 启动 → 健康检查；失败可一键回滚到最近备份 |
| 权限控制 | 管理员/运维/发布员/只读 四种角色 + 数据级授权（可执行的主机分组、可发布的应用） |
| 审计日志 | 所有写操作自动记录（人、动作、资源、来源 IP、状态码），执行任务全留痕 |
| 危险命令拦截 | 正则规则库（rm -rf、mkfs、dd、shutdown、drop database 等内置 11 条），命中即阻断并写审计；管理员可增删改、可测试 |

## 快速开始（本地开发）

### 1. 准备 PostgreSQL

使用任意外部 PG，例如 Docker 起一个：

```bash
docker run -d --name autoops-pg -e POSTGRES_USER=autoops -e POSTGRES_PASSWORD=autoops123 \
  -e POSTGRES_DB=autoops -p 5432:5432 postgres:16-alpine
```

### 2. 配置后端

```bash
cd backend
cp config.example.yaml config.yaml
# 生成 AES 主密钥（用于加密 SSH 私钥）
autoops-server -genkey        # 或 openssl rand -base64 32
# 编辑 config.yaml：填入 dsn / jwt_secret / aes_key
```

### 3. 启动

```bash
# 方式一：直接运行（自动建表 + 种子数据）
cd backend && go build -o autoops-server ./cmd/server && ./autoops-server -config config.yaml

# 方式二：开发联调
cd frontend && npm install && npm run dev   # 前端 http://localhost:5173，代理到 8080
```

访问 http://localhost:8080（前端已构建到 `frontend/dist` 时由后端直接托管）。

默认账号：`admin / admin123`（首次登录请立即修改密码）。

### 4. 生产部署（Docker Compose）

```bash
cd deploy
docker compose up -d --build
# 修改 docker-compose.yml 中的 JWT_SECRET 与 AES_KEY！
```

## 使用说明

1. **密钥准备**：主机管理 → SSH 密钥管理 → 导入私钥；把对应公钥写入目标机
   `~/.ssh/authorized_keys`。或改用密码认证。
2. **导入主机**：支持粘贴批量导入，格式 `IP,端口,用户名,分组名`（后三项可省略，分组自动创建）。
3. **批量执行**：勾选目标主机 → 输入命令或选脚本 → 执行。输出实时滚动，任务详情永久留档。
4. **发布**：先在「应用管理」配置应用（绑定主机 + 部署目录 + jar 名 + 可选启停命令/备份目录/健康检查 URL），
   再到「发布中心」上传 jar 发起发布。单主机流水线串行、多主机并发，失败主机可一键回滚。
5. **权限**：管理员在「用户权限」给用户分配角色，并按主机分组（可执行/可部署）、应用（可发布）授权。

## 安全设计

- SSH 私钥与密码使用 AES-256-GCM 加密落库，主密钥由配置注入（支持环境变量覆盖）。
- 密码使用 bcrypt 存储；JWT 有效期 24h。
- 危险命令在「批量执行 / 脚本执行 / 发布启停」入口统一拦截，命中返回 403 并记录审计。
- 审计日志自动脱敏（password/content 字段）。

## 目录结构

```
backend/          Go 后端（cmd/server 入口，internal 分层）
frontend/         Vue3 前端（src/views 按模块分页面）
deploy/           docker-compose.yml + Dockerfile
```

## 已知边界（MVP）

- SSH 交互终端与批量执行暂按「内网可信」设计，未做目标机 host key 校验。
- 文件分发为单文件粒度（目录分发可循环调用或后续迭代）。
- 任务无分布式调度（单实例执行）；多实例部署需引入任务队列。
