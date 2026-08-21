# 卡片 P0-01：Go module 与仓库骨架

| 调度头 | 值 |
| --- | --- |
| 波次 | **W0**（必须最先，单独一波） |
| 前置依赖 | 无 |
| 独占文件 | `go.mod` `go.sum` `cmd/farmbot/main.go`(占位) 目录骨架 + 各包 `doc.go` |
| 可与谁并发 | 无（它是所有卡的地基，先合并再开 W1） |
| 风险 | 🟢 低 |

---

- 目标：初始化 Go module 与目标目录骨架，`go build ./...` 通过。不含任何业务逻辑。
- 源（现状）：`core/package.json`（沿用元信息）、`yyb-go/go.mod`（合并其依赖）。
- 目标（Go）：
  - 仓库根 `go.mod`：`module github.com/<owner>/farmbot`，`go 1.23`。合并 `yyb-go/go.mod` 的 require（gin、modernc.org/sqlite 等）。
  - 建空包目录（含 `doc.go` 占位）：`cmd/farmbot/`、`internal/{app,config,store,game,account,domain,yyb,httpapi,license,platform}`、`proto/`、`assets/`。
  - `cmd/farmbot/main.go`：仅打印版本并退出（后续卡填充 wiring）。
- 实现要点：目录用 Go 惯例；`internal/` 保证外部不可 import；每个空包放 `doc.go` 说明职责，防止空目录不被 git 跟踪。
- 不要做：不迁移任何业务代码；不动 `core/` 与 `web/`。
- 验收：`go build ./...` 与 `go vet ./...` 通过；`go run ./cmd/farmbot` 打印版本。
- 完成判据：☐ go.mod 就位 ☐ 目录骨架建好 ☐ 编译通过。
