# 卡片 P0-03：日志层（slog + secret 脱敏）

| 调度头 | 值 |
| --- | --- |
| 波次 | **W1** |
| 前置依赖 | P0-01 |
| 独占文件 | `internal/platform/logger/**` |
| 可与谁并发 | P0-02, P0-04 |
| 风险 | 🟢 低 |

---

- 目标：提供全局结构化日志，保留现状的密钥脱敏能力。
- 源（现状）：`core/src/services/logger.js`（Winston + `sanitizeMeta`/`redactString` 脱敏 + 每日轮转）。
- 目标（Go）：`internal/platform/logger/logger.go`。
- 实现要点：
  - 基于标准库 `log/slog`，无三方依赖。提供 `New(module string) *slog.Logger` 工厂（等价 `createModuleLogger`）。
  - 移植 secret 脱敏：对 token/openid/login_buffer/密码/`FARM_MASTER_KEY` 等字段名做值遮蔽（`***`）。
  - 输出到 stdout + 可选文件（`FARM_LOG_DIR`），支持 `LOG_LEVEL`。文件轮转可用 `lumberjack`（可选依赖）或简单按日期切分。
- 不要做：不追求与 Winston 完全一致的格式；不记录明文密钥/凭据。
- 验收：`go test ./internal/platform/logger/...`（断言含敏感字段的日志被遮蔽）。
- 完成判据：☐ 模块 logger 工厂可用 ☐ 脱敏生效 ☐ 级别/输出可配。
