# 卡片 P7-01：资源内嵌（embed.FS）

| 调度头 | 值 |
| --- | --- |
| 波次 | **W15** |
| 前置依赖 | P2-02、P6-05；前端可 `vite build` |
| 独占文件 | `assets/embed.go`（+ `internal/httpapi` 消费点） |
| 可与谁并发 | P8-* 前端卡（不同技术栈） |
| 风险 | 🟡 中 |

---

- 目标：把 `web/dist`、WASM、gameConfig 编译进二进制。
- 源（现状）：pkg `assets`(package.json:41-50)、`utils/*.wasm`、`gameConfig/`、`web/dist`。
- 目标（Go）：`assets/embed.go` + `internal/httpapi` 消费。
- 实现要点：
  - `//go:embed` 声明：`assets/wasm/*.wasm`、`assets/gameConfig/**`；前端 `web/dist/**`（放 `internal/httpapi/webdist` 或 `assets/webdist`，构建前先 `vite build`）。
  - 提供访问函数（`WASM(name)`、`GameConfigFS()`、`WebDistFS()`）。
  - 支持「磁盘覆盖」调试模式：env 指定则从磁盘读，否则用 embed（便于开发不重编）。
- 不要做：不把 SQLite 数据库或用户数据 embed（那是运行时可写数据）。
- 验收：`go build` 后二进制单文件可离线启动并托管前端 + 加载 WASM，无需外部资源目录。
- 完成判据：☐ 三类资源内嵌 ☐ 磁盘覆盖模式 ☐ 单文件可跑。
