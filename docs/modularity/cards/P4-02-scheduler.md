# 卡片 P4-02：命名调度器（实例化）

| 调度头 | 值 |
| --- | --- |
| 波次 | **W7** |
| 前置依赖 | P4-01 |
| 独占文件 | `internal/account/scheduler.go` |
| 可与谁并发 | P4-01 |
| 风险 | 🟢 低 |

---

- 目标：把全局单例调度注册表降为 Runtime 的实例字段。
- 源（现状）：`core/src/services/scheduler.js`（全局 `schedulerRegistry` Map, :5；`createScheduler(ns)`）。
- 目标（Go）：`internal/account/scheduler.go`。
- 实现要点：
  - `Scheduler`：命名任务的注册/启停/查询；用 `time.Ticker` + context；支持抖动（jitter）与错峰（对齐现状三 tick staggered/jittered）。
  - 提供 `Every(name, interval, jitter, fn)` / `Stop(name)` / `Status()`。挂在 Runtime 上，随账号销毁。
- 不要做：不用包级全局 map；不跨账号共享定时器。
- 验收：`go test`——注册/触发/停止/状态查询；jitter 范围断言。
- 完成判据：☐ 实例化调度器 ☐ jitter/错峰 ☐ 状态可查。

> 注：P4-01 与本卡同波，两者独占不同文件（runtime.go/status.go vs scheduler.go）。若同一会话先做 P4-01，Runtime 里对 Scheduler 的引用先用接口占位，本卡补实现。
