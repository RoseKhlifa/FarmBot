# 卡片 P5-01：warehouse 背包域（先行，被广泛复用）

| 调度头 | 值 |
| --- | --- |
| 波次 | **W9** |
| 前置依赖 | P4-04 |
| 独占文件 | `internal/domain/warehouse/**` |
| 可与谁并发 | 无（P5 各域依赖它，故 P5 内它先行，本波单独跑） |
| 风险 | 🟠 中（下游 friend/planting/task/mall/activity 都依赖） |

---

- 目标：背包/出售/使用能力，被 friend/planting/task/mall/activity 复用，故先搬。
- 源（现状）：`core/src/services/warehouse.js`(724)——`sellAllFruits`、bag 读取/使用、依赖 `network`/`proto`/`store`(isAutomationOn)/`status`。
- 目标（Go）：`internal/domain/warehouse/`（`service.go` 接口 + 实现）。
- 实现要点：`Service{ ListBag, SellAll, SellItem, UseItem, ... }`；依赖 `game.Transport` + `store.ConfigRepo`；`status` 更新改为返回值/回调接口。
- 不要做：不内联 network 单例；不写死账号 env。
- 验收：`go test` + 对拍 `sellAllFruits` 结果。
- 完成判据：☐ 接口化 ☐ 依赖注入 ☐ 对拍一致。

> 调度说明：warehouse 是 P5 的共同前置。建议**单独一个会话最先做完**，其余 P5 域（farm/mall/task/activity/其余）再并发起。
