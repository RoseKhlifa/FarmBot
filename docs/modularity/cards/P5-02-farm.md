# 卡片 P5-02：farm 农场域

| 调度头 | 值 |
| --- | --- |
| 波次 | **W10** |
| 前置依赖 | P5-01 |
| 独占文件 | `internal/domain/farm/**` |
| 可与谁并发 | P5-04, P5-05, P5-06, P5-07（各占独立 domain 子目录） |
| 风险 | 🟠 中 |

---

- 目标：合并农场相关 7 个服务为一个内聚领域包。
- 源（现状）：`farm-api.js`(196)、`farm-land-analyzer.js`(493)、`farm-fertilizer.js`(398)、`planting-service.js`(1168)、`farming-orchestrator.js`(327)、`farm-scheduler.js`(79)、`analytics.js`(150)、`farm.js`(facade)。
- 目标（Go）：`internal/domain/farm/`——`api.go`(原始 RPC)、`land_analyzer.go`、`fertilizer.go`、`planting.go`(种子选择策略)、`analytics.go`(收益/经验排名)、`orchestrator.go`(主循环)、`service.go`(对外接口聚合)。
- 实现要点：
  - `orchestrator` 由组合根注入 api/analyzer/fertilizer/planting，而非包内硬 import 链。
  - `farm-scheduler`(化肥购买 timer) 并入 Runtime 调度器（P4-02）注册，逻辑放 `fertilizer.go`。
  - `analytics` 消费 gameConfig（`assets/gameConfig` via embed）。
  - 若需化肥购买调 mall，走注入的 `mall.Service` 接口（P5-04）；接口先占位，P5-04 完成后接线。
- 不要做：不保留 facade 空壳；不跨域 import friend/mall。
- 验收：`go test` + 对拍种子选择、地块相位分析、化肥策略。
- 完成判据：☐ 7 服务归一域 ☐ 策略对拍 ☐ 循环可跑。
