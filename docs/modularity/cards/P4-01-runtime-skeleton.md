# 卡片 P4-01：Runtime 骨架（单账号容器）

| 调度头 | 值 |
| --- | --- |
| 波次 | **W7** |
| 前置依赖 | P2-04、P2-05、P1-03 |
| 独占文件 | `internal/account/runtime.go` `internal/account/status.go` |
| 可与谁并发 | P4-02 |
| 风险 | 🟠 中 |

---

- 目标：定义 `Runtime`——一个账号的全部运行态与依赖容器。这是从多进程模型转向单进程并发的核心转变。
- 源（现状）：`core/worker.js` 顶部状态 + `network.js` 单例（userState/ws）+ `status.js`/`stats.js`/`scheduler.js` 单例。
- 目标（Go）：`internal/account/runtime.go`。
- 实现要点：
  - `Runtime` 字段（全部**实例化**，根治模块单例）：`accountID`、`ws *transport.WS`、`session *session.Session`、`tsdk *tsdk.Runtime`、`ace *ace.Service`、`sched *Scheduler`、`status *StatusState`、`stats StatsRepo`、`cfg *account.Config`、`ctx/cancel`、领域服务集合（P5 注入）。
  - `Start(ctx)`/`Stop()`：Start 建 WS→登录→起 ACE→起调度循环；Stop 取消 ctx、销毁 tsdk/ace/ws。
  - 每个账号一份，彼此隔离；跨账号并发天然安全。
- 不要做：不放业务算法（领域在 P5）；不共享任何单例；循环编排细节留给 P4-04。
- 验收：`go test`——mock transport 下 Start/Stop 生命周期正确、资源释放无泄漏。
- 完成判据：☐ Runtime 字段实例化 ☐ Start/Stop 就位 ☐ 无模块单例。
