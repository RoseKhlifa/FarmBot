# P0 · 地基任务卡

目标：立起可编译、可运行、可测试的 Go 骨架，为后续所有卡提供地基。不含任何业务逻辑。

---

### 卡片 P0-01：Go module 与仓库骨架
- 目标：初始化 Go module 与目标目录骨架，`go build ./...` 通过。
- 前置依赖：无。
- 源（现状）：`core/package.json`（沿用元信息）、`yyb-go/go.mod`（合并其依赖）。
- 目标（Go）：
  - 仓库根 `go.mod`：`module github.com/<owner>/farmbot`，`go 1.23`。合并 `yyb-go/go.mod` 的 require（gin、modernc.org/sqlite 等）。
  - 建空包目录（含 `doc.go` 占位）：`cmd/farmbot/`、`internal/{app,config,store,game,account,domain,yyb,httpapi,license,platform}`、`proto/`、`assets/`。
  - `cmd/farmbot/main.go`：仅打印版本并退出（后续卡填充 wiring）。
- 实现要点：目录用 Go 惯例；`internal/` 保证外部不可 import；每个空包放 `doc.go` 说明职责，防止空目录不被 git 跟踪。
- 不要做：不迁移任何业务代码；不动 `core/` 与 `web/`。
- 验收：`go build ./...` 与 `go vet ./...` 通过；`go run ./cmd/farmbot` 打印版本。
- 完成判据：☐ go.mod 就位 ☐ 目录骨架建好 ☐ 编译通过。

---

### 卡片 P0-02：配置层（沿用现有 env 契约）
- 目标：把散在 Node 各处的 env/config 收敛成一个 `Config` 结构体 + 数据/资源路径解析。
- 前置依赖：P0-01。
- 源（现状）：`core/src/config/config.js`（server URL、clientVersion、间隔、TSDK、adminPort/password）、`core/src/config/runtime-paths.js`、`core/src/config/license-config.js`。
- 目标（Go）：`internal/config/config.go` + `internal/config/paths.go`。
- 实现要点：
  - `Config` 字段覆盖：`AdminPort`(env `ADMIN_PORT`,默认 3007)、`AdminPassword`、`DataDir`(env `FARM_DATA_DIR`)、`ServerURL`、`ClientVersion`(`1.13.0.4_20260723` 等常量)、`LicenseEnabled`(`FARM_LICENSE_ENABLED`)、`TSDK{GameID(3167),AppKey,AceEnabled}`(`FARM_TSDK_*`)、`Yyb{Enabled,...}`。
  - `paths.go`：解析数据目录（默认 `<exe同级>/data`）、资源目录；支持 `go:embed` 与磁盘覆盖两种模式。
  - **废弃并注明**：`FARM_WORKER`/`FARM_RUNTIME_MODE`/`FARM_ACCOUNT_ID`（goroutine 模型不再需要）；`YYB_API_URL/KEY/PORT`（yyb 收编后内部直调，保留 `YYB_ADMIN_USER/PASS` 若仍需 yyb 管理页）。
- 不要做：不引入重型配置框架（viper 可选但非必须）；不改 env 变量名语义。
- 验收：`go test ./internal/config/...`（覆盖默认值与 env 覆盖）；构造 Config 打印无 panic。
- 完成判据：☐ 全部现有 env 有映射 ☐ 废弃变量已在代码注释与本卡记录 ☐ 测试通过。

---

### 卡片 P0-03：日志层（slog + secret 脱敏）
- 目标：提供全局结构化日志，保留现状的密钥脱敏能力。
- 前置依赖：P0-01。
- 源（现状）：`core/src/services/logger.js`（Winston + `sanitizeMeta`/`redactString` 脱敏 + 每日轮转）。
- 目标（Go）：`internal/platform/logger/logger.go`。
- 实现要点：
  - 基于标准库 `log/slog`，无三方依赖。提供 `New(module string) *slog.Logger` 工厂（等价 `createModuleLogger`）。
  - 移植 secret 脱敏：对 token/openid/login_buffer/密码 等字段名做值遮蔽（`***`）。
  - 输出到 stdout + 可选文件（`FARM_LOG_DIR`），支持 `LOG_LEVEL`。文件轮转可用 `lumberjack`（可选依赖）或简单按日期切分。
- 不要做：不追求与 Winston 完全一致的格式；不记录明文密钥。
- 验收：`go test ./internal/platform/logger/...`（断言含敏感字段的日志被遮蔽）。
- 完成判据：☐ 模块 logger 工厂可用 ☐ 脱敏生效 ☐ 级别/输出可配。

---

### 卡片 P0-04：构建脚手架（Makefile + lint + CI）
- 目标：统一构建/生成/测试/lint 入口，锁定验收命令。
- 前置依赖：P0-01。
- 源（现状）：根 `package.json` scripts、`web/package.json`。
- 目标（Go）：根 `Makefile`（或 `Taskfile`）+ `.golangci.yml`。
- 实现要点：
  - Make 目标：`build`(go build 出二进制)、`build-web`(pnpm -C web build)、`gen-proto`(protoc 生成，见 P2-01)、`test`(go test ./...)、`lint`(golangci-lint run + eslint web)、`release`(交叉编译，见 P7)。
  - `.golangci.yml`：启用 govet/staticcheck/gofmt/errcheck 等基础 linter。
  - 保留 `web` 的 `pnpm lint`/`vite build` 作为前端验收（沿用 progress.md 既有基线）。
- 不要做：不引入复杂 CI 平台绑定；先本地可跑。
- 验收：`make build`、`make lint`、`make test` 均可执行（此阶段 test 可为空但命令成立）。
- 完成判据：☐ Makefile 就位 ☐ golangci 配置就位 ☐ 三条命令可跑。
