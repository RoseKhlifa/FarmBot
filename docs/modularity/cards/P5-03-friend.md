# 卡片 P5-03：friend 好友域（最密子图）

| 调度头 | 值 |
| --- | --- |
| 波次 | **W11** |
| 前置依赖 | P5-01、P5-02 |
| 独占文件 | `internal/domain/friend/**` |
| 可与谁并发 | P5-04, P5-05, P5-07（互不同目录）；不与 P5-02 同波（依赖它） |
| 风险 | 🔴 中高（现状最密耦合子图，回调注入需显式化） |

---

- 目标：合并好友相关 6 个服务，显式化回调依赖。
- 源（现状）：`friend-api.js`(867)、`friend-operation-limits.js`(616)、`friend-land-analyzer.js`(659)、`friend-visit.js`(991)、`friend-orchestrator.js`(1164, fan-out 10)、`golden-bug-service.js`(178)、`friend.js`(facade)。
- 目标（Go）：`internal/domain/friend/`——`api.go`、`limits.go`(每日 steal/help/bad 限额 + 操作)、`land_analyzer.go`、`visit.go`(进出好友农场+偷/帮序列)、`golden_bug.go`、`orchestrator.go`、`service.go`。
- 实现要点：
  - **消除回调注入**：`limits.go` 的 `setOnExpLimitReached/ResetCallback` 改为 orchestrator 通过接口方法查询/订阅（如 `limits.OnExpLimit() <-chan Event` 或返回结构体），依赖显式化。
  - 好友 GID/狗信息/列表缓存走 `store.CacheRepo`（P1-03）；黑名单增删走 CacheRepo。
  - quiet-hours、GID 归一化随 `api.go`。
  - orchestrator 依赖注入 farm/warehouse/limits/visit/analyzer + CacheRepo。
- 不要做：不保留隐藏回调；不在 analyzer 里 lazy-require api（Go 无循环 require，直接依赖）。
- 验收：`go test`——限额边界用例（每日上限/重置）、偷帮序列、黑名单；对拍 Node。
- 完成判据：☐ 6 服务归一域 ☐ 回调显式化 ☐ 限额对拍 ☐ 缓存走 repo。
