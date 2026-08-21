# 卡片 P0-02：配置层（沿用现有 env 契约）

| 调度头 | 值 |
| --- | --- |
| 波次 | **W1** |
| 前置依赖 | P0-01 |
| 独占文件 | `internal/config/**` |
| 可与谁并发 | P0-03, P0-04（同波，文件不相交） |
| 风险 | 🟢 低 |

---

- 目标：把散在 Node 各处的 env/config 收敛成一个 `Config` 结构体 + 数据/资源路径解析。
- 源（现状）：`core/src/config/config.js`（server URL、clientVersion、间隔、TSDK、adminPort/password）、`core/src/config/runtime-paths.js`、`core/src/config/license-config.js`。
- 目标（Go）：`internal/config/config.go` + `internal/config/paths.go`。
- 实现要点：
  - `Config` 字段覆盖：`AdminPort`(env `ADMIN_PORT`,默认 3007)、`AdminPassword`、`DataDir`(env `FARM_DATA_DIR`)、`ServerURL`、`ClientVersion`(`1.13.0.4_20260723` 等常量)、`LicenseEnabled`(`FARM_LICENSE_ENABLED`)、`TSDK{GameID(3167),AppKey,AceEnabled}`(`FARM_TSDK_*`)、`Yyb{Enabled,...}`。
  - `paths.go`：解析数据目录（默认 `<exe同级>/data`）、资源目录；支持 `go:embed` 与磁盘覆盖两种模式。
  - **新增（商用）**：预留 `MasterKey`(env `FARM_MASTER_KEY`，凭据加密用，见 PC-02)。为空时记录告警但不阻断（自用可空，商用必填）。
  - **废弃并注明**：`FARM_WORKER`/`FARM_RUNTIME_MODE`/`FARM_ACCOUNT_ID`（goroutine 模型不再需要）；`YYB_API_URL/KEY/PORT`（yyb 收编后内部直调，保留 `YYB_ADMIN_USER/PASS` 若仍需 yyb 管理页）。
- 不要做：不引入重型配置框架（viper 可选但非必须）；不改 env 变量名语义。
- 验收：`go test ./internal/config/...`（覆盖默认值与 env 覆盖）；构造 Config 打印无 panic。
- 完成判据：☐ 全部现有 env 有映射 ☐ MasterKey 预留 ☐ 废弃变量已在代码注释与本卡记录 ☐ 测试通过。
