# 卡片 P4-03：AccountManager（生命周期 + 重连退避）+ yyb 接入

| 调度头 | 值 |
| --- | --- |
| 波次 | **W8** |
| 前置依赖 | P4-01、P3-04（yyb Service 接入并入本卡） |
| 独占文件 | `internal/account/manager.go` |
| 可与谁并发 | P4-04 |
| 风险 | 🟠 中高（重连退避/kickout 时序） |

---

- 目标：管理所有账号 Runtime 的启停与自动重连。**RPC switch 彻底消失**。
- 源（现状）：`worker-manager.js`——spawn/stop/restart、`handleWorkerMessage`、`scheduleReconnect`(:477, 尝试计数 + 稳定性重置)、`refreshYybCodeIfNeeded`。
- 目标（Go）：`internal/account/manager.go`。
- 实现要点：
  - `Manager`：`map[string]*Runtime` + `sync.RWMutex`；`Start(id)`/`Stop(id)`/`Restart(id)`/`Get(id)`/`List()`。
  - `Start` 流程：取账号配置 → 调 `yyb.Service.GetCode` 取 code（并入 P3-04）→ `session.Login` → 建 Runtime → 跑循环。第三方 provider 可插拔。
  - 重连退避状态机：移植现状「有限退避 + 连接稳定则重置计数」；区分 kickout/网络断开/账号停止（对应现状 `ws_error`/`account_kicked`/`ws_reconnect_failed` 事件语义）。
  - **RPC switch 消失**：不再有 `handleApiCall`；上层通过 `manager.Get(id)` 拿 Runtime 直接调类型安全方法。
- 不要做：不引入进程/线程；不用字符串方法名派发；不保留 `YYB_API_URL/KEY` env 布线。
- 验收：`go test`——mock 下 start/stop/restart、重连退避序列、稳定重置；并发操作多账号无 data race（`go test -race`）。
- 完成判据：☐ Manager CRUD ☐ 重连退避 ☐ 无 RPC switch ☐ yyb 直调 ☐ -race 通过。
