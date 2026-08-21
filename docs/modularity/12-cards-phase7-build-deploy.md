# P7 · 构建部署任务卡

目标：一个 `go build` 出内嵌前端 + WASM + 游戏数据的单二进制；单/双阶段 Dockerfile 取代现状 3 阶段 + `start.sh` 双进程编排；
源码 / 二进制 / Docker **三种方式都自带 yyb**，彻底解决「部署两个项目」。

> 现状（来自 01 §5、部署分析）：3 阶段 Dockerfile（builder/yyb-builder/runner）+ `start.sh` 后台起 yyb `&` 再 `exec node`；pkg 无法内嵌 Go；源码/二进制无 yyb。

---

### 卡片 P7-01：资源内嵌（embed.FS）
- 目标：把 `web/dist`、WASM、gameConfig 编译进二进制。
- 前置依赖：P2-02、P6-01；前端可 `vite build`。
- 源（现状）：pkg `assets`(package.json:41-50)、`utils/*.wasm`、`gameConfig/`、`web/dist`。
- 目标（Go）：`assets/embed.go` + `internal/httpapi` 消费。
- 实现要点：
  - `//go:embed` 声明：`assets/wasm/*.wasm`、`assets/gameConfig/**`；前端 `web/dist/**`（放 `internal/httpapi/webdist` 或 `assets/webdist`，构建前先 `vite build`）。
  - 提供访问函数（`WASM(name)`、`GameConfigFS()`、`WebDistFS()`）。
  - 支持「磁盘覆盖」调试模式：env 指定则从磁盘读，否则用 embed（便于开发不重编）。
- 不要做：不把 SQLite 数据库或用户数据 embed（那是运行时可写数据）。
- 验收：`go build` 后二进制单文件可离线启动并托管前端 + 加载 WASM，无需外部资源目录。
- 完成判据：☐ 三类资源内嵌 ☐ 磁盘覆盖模式 ☐ 单文件可跑。

---

### 卡片 P7-02：跨平台发布构建
- 目标：交叉编译出 Windows/Linux/macOS 二进制，替代 pkg。
- 前置依赖：P7-01、P0-04。
- 源（现状）：`package:release`(pkg)、`start.bat`/`start.sh` 源码启动脚本。
- 目标（Go）：`Makefile` `release` 目标（+ 可选 GoReleaser）。
- 实现要点：
  - `make build-web`（`pnpm -C web build`）→ `GOOS/GOARCH` 矩阵 `go build -ldflags "-s -w -X main.version=..."`。
  - **纯 Go 依赖保证交叉编译**：wazero + modernc.org/sqlite 均无 CGO，`CGO_ENABLED=0` 可静态交叉编译到各平台（这是选型的关键回报）。
  - 版本号注入（对齐现状 `__APP_VERSION__` 来自 core/package.json）。
- 不要做：不引入 CGO 依赖（会破坏交叉编译与单文件目标）。
- 验收：`make release` 产出至少 win-amd64 / linux-amd64 两个可运行二进制。
- 完成判据：☐ 交叉编译成功 ☐ CGO_ENABLED=0 ☐ 版本注入 ☐ 二进制自带 yyb。

---

### 卡片 P7-03：Docker 单/双阶段镜像
- 目标：用 2 阶段（web 构建 + go 构建）取代现状 3 阶段 + 双进程 shell 编排。
- 前置依赖：P7-01、P6-05。
- 源（现状）：`core/Dockerfile`(3 阶段)、`core/docker/start.sh`(tini + yyb `&` + `exec node`)、`docker-compose.yml`、`server-deploy.sh`。
- 目标（Go）：根 `Dockerfile`（2 阶段）+ 精简 `docker-compose.yml`。
- 实现要点：
  - 阶段 1 `node:20` 跑 `pnpm -C web build`；阶段 2 `golang:1.23` 拷入 `web/dist` 与源码 `go build`；最终 `FROM gcr.io/distroless/static` 或 `alpine` 拷单二进制。
  - **无 start.sh / 无 tini / 无 `&`**：单进程即入口（yyb 已在进程内），信号直达 Go 的优雅退出。
  - compose 单服务、单端口 3007、单数据卷（`data/farmbot.db` + 运行时目录）；健康检查探 `/api/health`。
  - 移除 `YYB_API_URL/KEY/TOKEN`/`YYB_PORT` 及其 auto-gen 逻辑（进程内直调不需要）。
- 不要做：不保留双进程编排；不保留跨进程 Token 布线。
- 验收：`docker build` 成功；`docker compose up` 起单容器，前端可用、yyb 扫码可用、无外部 yyb 进程。
- 完成判据：☐ 2 阶段镜像 ☐ 单进程单端口 ☐ 单数据卷 ☐ yyb 内置。

---

### 卡片 P7-04：删除旧 Node 后端 + 文档更新
- 目标：绞杀完成后移除 `core/` 与代理层，更新 README/部署文档。
- 前置依赖：P6-04（全部路由切 Go 并真机稳定运行一个周期）、P7-03。
- 源（现状）：`core/`、`yyb-go/`（已收编）、绞杀代理、`README.md` 部署章节、根 `package.json`(workspace)。
- 目标：删除 `core/`、`yyb-go/`（其代码已在 `internal/`）；更新根 `package.json` 或改为纯 `web/` 前端 + Go 后端结构；README 重写部署三方式。
- 实现要点：
  - 确认无路由仍指向 Node、无脚本引用 core 后再删。保留一个 git tag 作回退锚点。
  - README：源码（`go run` + `pnpm -C web dev`）、二进制（下载单文件运行）、Docker（compose up）三方式，均说明 yyb 已内置、数据在 `data/farmbot.db`、首启可 `--import-json` 迁移旧数据。
- 不要做：不在未全量验证前删 core；不删用户 `data/`。
- 验收：仓库 `go build ./...` + `vite build` 通过；三种部署文档实测可跑。
- 完成判据：☐ core/yyb-go 移除 ☐ 无残留引用 ☐ README 三方式更新 ☐ 回退 tag 就位。
