# P4 · 账号运行时任务卡

目标：把现状「一账号一 OS 进程/线程 + IPC + RPC switch」重写为 Go 的「一账号一组 goroutine + channel」。
`AccountManager` 管生命周期，`Runtime` 持连接与自动化循环。这是从多进程模型转向单进程并发的核心转变。

> 现状（来自 00 §1、01 §2）：`runtime-engine`(组装) + `runtime-state`(共享态) + `worker-manager`(进程生命周期+IPC+重连) + `data-provider`(唯一边界, ~90 方法)；`core/worker.js`(1281, 含 300 行 RPC switch + 47 内联 require + 三 tick 调度)。

---

### 卡片 P4-01：Runtime 骨架（单账号容器）
- 目标：定义 `Runtime`——一个账号的全部运行态与依赖容器。
- 前置依赖：P2-04、P2-05、P1-03。
- 源（现状）：`core/worker.js` 顶部状态 + `network.js` 单例（userState/ws）+ `status.js`/`stats.js`/`scheduler.js` 单例。
- 目标（Go）：`internal/account/runtime.go`。
- 实现要点：
  - `Runtime` 字段（全部**实例化**，根治模块单例）：`accountID`、`ws *transport.WS`、`session *session.Session`、`tsdk *tsdk.Runtime`、`ace *ace.Service`、`sched *Scheduler`、`status *StatusState`、`stats StatsRepo`、`cfg *account.Config`、`ctx/cancel`、领域服务集合（P5 注入）。
  - `Start(ctx)`/`Stop()`：Start 建 WS→登录→起 ACE→起调度循环；Stop 取消 ctx、销毁 tsdk/ace/ws。
  - 每个账号一份，彼此隔离；跨账号并发天然安全。
- 不要做：不放业务算法（领域在 P5）；不共享任何单例。
- 验收：`go test`——mock transport 下 Start/Stop 生命周期正确、资源释放无泄漏。
- 完成判据：☐ Runtime 字段实例化 ☐ Start/Stop 就位 ☐ 无模块单例。

---

### 卡片 P4-02：命名调度器（实例化）
- 目标：把全局单例调度注册表降为 Runtime 的实例字段。
- 前置依赖：P4-01。
- 源（现状）：`core/src/services/scheduler.js`（全局 `schedulerRegistry` Map, :5；`createScheduler(ns)`）。
- 目标（Go）：`internal/account/scheduler.go`。
- 实现要点：
  - `Scheduler`：命名任务的注册/启停/查询；用 `time.Ticker` + context；支持抖动（jitter）与错峰（对齐现状三 tick staggered/jittered）。
  - 提供 `Every(name, interval, jitter, fn)` / `Stop(name)` / `Status()`。挂在 Runtime 上，随账号销毁。
- 不要做：不用包级全局 map；不跨账号共享定时器。
- 验收：`go test`——注册/触发/停止/状态查询；jitter 范围断言。
- 完成判据：☐ 实例化调度器 ☐ jitter/错峰 ☐ 状态可查。

---

### 卡片 P4-03：AccountManager（生命周期 + 重连退避）
- 目标：管理所有账号 Runtime 的启停与自动重连。
- 前置依赖：P4-01、P3-04。
- 源（现状）：`worker-manager.js`——spawn/stop/restart、`handleWorkerMessage`、`scheduleReconnect`(:477, 尝试计数 + 稳定性重置)、`refreshYybCodeIfNeeded`。
- 目标（Go）：`internal/account/manager.go`。
- 实现要点：
  - `Manager`：`map[string]*Runtime` + `sync.RWMutex`；`Start(id)`/`Stop(id)`/`Restart(id)`/`Get(id)`/`List()`。
  - `Start` 流程：取账号配置 → 调 `yyb.Service.GetCode` 取 code → `session.Login` → 建 Runtime → 跑循环。
  - 重连退避状态机：移植现状「有限退避 + 连接稳定则重置计数」；区分 kickout/网络断开/账号停止（对应现状 `ws_error`/`account_kicked`/`ws_reconnect_failed` 事件语义）。
  - **RPC switch 消失**：不再有 `handleApiCall`；上层通过 `manager.Get(id)` 拿 Runtime 直接调类型安全方法。
- 不要做：不引入进程/线程；不用字符串方法名派发。
- 验收：`go test`——mock 下 start/stop/restart、重连退避序列、稳定重置；并发操作多账号无 data race（`go test -race`）。
- 完成判据：☐ Manager CRUD ☐ 重连退避 ☐ 无 RPC switch ☐ -race 通过。

---

### 卡片 P4-04：自动化循环编排 + 状态/事件桥接
- 目标：移植 worker 的统一 tick 调度与状态同步，桥接入站游戏事件。
- 前置依赖：P4-01、P4-02、P2-04。
- 源（现状）：`core/worker.js` `runUnifiedTick`(:387)、`runFarmTick`/`runHelpTick`/`runStealTick`、每日例程(:225)、5s `syncStatus`、`onFarmHarvested`(:644)、订阅 `networkEvents`(:614-658)。
- 目标（Go）：`internal/account/runtime.go`（循环编排）+ `internal/account/status.go`（状态快照）。
- 实现要点：
  - 三条错峰抖动循环（farm/help/steal）+ 每日例程 timer + 5s 状态同步，全部经 `Scheduler` 注册，用 context 取消。
  - 订阅 transport 分发的入站事件（`sell`/`farmHarvested`/`kickout`/`disconnect`/`taskInfoNotify`）：转成 Runtime 内部动作（如收获后自动出售）或上抛 Manager（如 kickout 触发重连）。
  - `StatusState`：per-account 运行态（在线/统计/下一次动作倒计时），供 P6 实时推送与 HTTP 查询。记录 gold/exp/ops（对齐 stats）。
- 不要做：不把领域算法写死在循环里（循环只调度，领域方法在 P5）。
- 验收：`go test`——mock 领域方法，断言循环按周期触发、事件正确路由、状态快照更新。
- 完成判据：☐ 三循环+每日+状态同步 ☐ 入站事件桥接 ☐ 状态快照就位。

> P4 出口真机验收：单账号能被 Manager 启动、登录、按调度自动挂机（哪怕仅 farm 领域桩），kickout 能触发重连。这是「能登录能挂机」最小闭环。
