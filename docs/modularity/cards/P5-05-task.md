# 卡片 P5-05：task 任务域

| 调度头 | 值 |
| --- | --- |
| 波次 | **W10** |
| 前置依赖 | P5-01 |
| 独占文件 | `internal/domain/task/**` |
| 可与谁并发 | P5-02, P5-04, P5-06, P5-07 |
| 风险 | 🟡 中 |

---

- 目标：任务/每日奖励领取，监听 taskInfoNotify。
- 源（现状）：`task.js`(552)——监听 `taskInfoNotify`，依赖 network/proto/store/scheduler/stats + lazy warehouse。
- 目标（Go）：`internal/domain/task/`（`service.go`）。
- 实现要点：订阅 P2-04 分发的 `taskInfoNotify` 事件；领取调 warehouse 注入接口；调度并入 P4-02。
- 验收：`go test` + 对拍任务领取。
- 完成判据：☐ 事件订阅 ☐ 领取对拍 ☐ 调度并入。
