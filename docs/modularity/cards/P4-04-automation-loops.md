# 卡片 P4-04：自动化循环编排 + 状态/事件桥接

| 调度头 | 值 |
| --- | --- |
| 波次 | **W8** |
| 前置依赖 | P4-01、P4-02、P2-04 |
| 独占文件 | `internal/account/loops.go`（循环编排；status.go 由 P4-01 建，本卡只读引用） |
| 可与谁并发 | P4-03 |
| 风险 | 🟠 中 |
| co-edit 提示 | 循环编排放独立 `loops.go`，避免与 P4-01 的 runtime.go 争抢；Runtime 暴露注入点 |

---

- 目标：移植 worker 的统一 tick 调度与状态同步，桥接入站游戏事件。
- 源（现状）：`core/worker.js` `runUnifiedTick`(:387)、`runFarmTick`/`runHelpTick`/`runStealTick`、每日例程(:225)、5s `syncStatus`、`onFarmHarvested`(:644)、订阅 `networkEvents`(:614-658)。
- 目标（Go）：`internal/account/loops.go`（循环编排）+ 复用 P4-01 的 `status.go`（状态快照）。
- 实现要点：
  - 三条错峰抖动循环（farm/help/steal）+ 每日例程 timer + 5s 状态同步，全部经 `Scheduler` 注册，用 context 取消。
  - 订阅 transport 分发的入站事件（`sell`/`farmHarvested`/`kickout`/`disconnect`/`taskInfoNotify`）：转成 Runtime 内部动作（如收获后自动出售）或上抛 Manager（如 kickout 触发重连）。
  - `StatusState`：per-account 运行态（在线/统计/下一次动作倒计时），供 P6 实时推送与 HTTP 查询。记录 gold/exp/ops（对齐 stats）。
- 不要做：不把领域算法写死在循环里（循环只调度，领域方法在 P5）。
- 验收：`go test`——mock 领域方法，断言循环按周期触发、事件正确路由、状态快照更新。
- 完成判据：☐ 三循环+每日+状态同步 ☐ 入站事件桥接 ☐ 状态快照就位。

> P4 出口真机验收：单账号能被 Manager 启动、登录、按调度自动挂机（哪怕仅 farm 领域桩），kickout 能触发重连。「能登录能挂机」最小闭环。
