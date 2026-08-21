# FarmBot

FarmBot 是 QQ 农场多账号自动化面板。当前运行时是纯 Go 单体：Go 进程提供 API、账号运行时、实时连接、应用宝能力和内嵌 Vue 前端，不再需要 Node 后端、独立 yyb 服务或代理进程。

项目仅供学习和研究使用。使用者需要自行承担账号、服务条款和数据安全风险。

## 当前状态

- 后端：Go 1.23+、Gin、SQLite（纯 Go 驱动）
- 前端：Vue 3、Vite、TypeScript、Pinia、UnoCSS
- 默认端口：`3007`
- 默认管理员：首次启动默认是 `admin` / `admin`；首次启动前设置 `ADMIN_PASSWORD` 可覆盖初始密码，已有管理员密码不会被启动覆盖
- 应用宝：已内置于 Go 进程，所有部署方式都使用同一套实现
- 数据库：`data/farmbot.db`，运行时数据不放入二进制

## 首选部署：Docker

Docker 会在镜像构建阶段编译前端并编译 Go 二进制，最终容器只有一个 FarmBot 进程和一个数据卷。

```bash
git clone https://github.com/Aoluis1005/qq-farm-bot.git
cd qq-farm-bot
# FARM_MASTER_KEY 必须是 32 字节原文、64 位 hex 或 32 字节 base64
export FARM_MASTER_KEY="$(openssl rand -hex 32)"
export ADMIN_PASSWORD="change-this-before-first-start"
docker compose up -d --build
docker compose logs -f
```

打开 `http://localhost:3007`。健康检查使用 `GET /api/health`，就绪检查使用 `GET /api/ready`。

也可以复制 `.env.example` 为 `.env`，再设置 `FARM_MASTER_KEY` 和 `ADMIN_PASSWORD`；缺少 `FARM_MASTER_KEY` 时应用会拒绝启动，不会降级为明文凭据存储。停止服务：

```bash
docker compose down
```

数据卷 `farmbot-data` 会保留数据库。备份由外部 cron 或 sidecar 触发：

```bash
docker compose exec farmbot /app/farmbot backup --timestamp 20260820T171100Z --keep 7
```

## 源码运行

安装 Go 1.23+、Node.js 20+ 和 pnpm。开发前端时使用两个终端：

终端一：

```bash
export FARM_MASTER_KEY="$(openssl rand -hex 32)"
export ADMIN_PASSWORD="change-this-before-first-start"
go run ./cmd/farmbot
```

终端二：

```bash
corepack enable
pnpm install -r
pnpm -C web dev
```

Vite 会把 `/api`、`/game-config` 和 WebSocket 请求代理到 `localhost:3007`。生产构建后，Go 也可直接使用 `assets/webdist` 中的内嵌资源：

```bash
pnpm -C web build
go test ./...
go test ./... -race
go run ./cmd/farmbot
```

Linux/macOS 可执行 `./start.sh`，Windows 可执行 `start.bat`。

## 二进制发布

发布构建会先编译前端，再使用 `CGO_ENABLED=0` 交叉编译 Go 单文件：

```bash
corepack enable
pnpm install -r
export FARM_MASTER_KEY="$(openssl rand -hex 32)"
make release
```

产物位于 `dist/`，包含 Windows amd64、Linux amd64、macOS amd64 和 macOS arm64。将二进制放在任意目录后直接运行即可；默认在当前工作目录创建 `data/farmbot.db`，也可通过 `FARM_DATA_DIR` 指定目录。

## 数据迁移、备份和导出

旧版 JSON 数据导入只读取源目录：

```bash
farmbot --import-json ./old-data --conflict skip --data-dir ./data
```

备份和单账号导出由外部命令触发，应用内部不运行定时器：

```bash
farmbot backup --timestamp 20260820T171100Z --keep 7 --data-dir ./data
farmbot export --account-id account-1 --output ./account-1.json --data-dir ./data
```

管理端提供受保护的账号 JSON 下载和操作审计查询/导出接口。导出不会包含密码、登录 code、token、API key 或 secret。

## 构建检查

```bash
go build ./...
go test ./...
go test ./... -race
go vet ./...
pnpm -C web exec vue-tsc --noEmit
pnpm -C web exec eslint "src/**/*.{ts,vue}"
pnpm -C web build
make lint
```

Docker 环境可用时，再执行 `docker build -t farmbot:test .` 和 `docker compose config`。

## 目录结构

```text
assets/                  内嵌 WASM、gameConfig 和 webdist
cmd/farmbot/             Go 入口、backup/export/import CLI
internal/app/             组合根和生命周期
internal/account/         多账号运行时和调度
internal/game/             协议、TSDK、ACE、WebSocket
internal/httpapi/          HTTP、鉴权、管理接口、实时 Hub
internal/store/            SQLite schema、迁移和 repositories
internal/yyb/              进程内应用宝服务
web/                      Vue 前端
data/                     运行时 SQLite 数据（不提交）
```

请立即修改默认管理员密码，不要提交 `data/`、`.env`、日志或账号导出文件。生产环境建议将数据库纳入外部备份策略，并限制管理端口访问范围。
