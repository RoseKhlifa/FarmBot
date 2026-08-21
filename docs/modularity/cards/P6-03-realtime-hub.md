# 卡片 P6-03：实时 Hub（WS/SSE 替代 Socket.IO）

| 调度头 | 值 |
| --- | --- |
| 波次 | **W11** |
| 前置依赖 | P6-02、P4-04 |
| 独占文件 | `internal/httpapi/realtime/**` + 路由 `/ws`（或 `/events`） |
| 可与谁并发 | P6-04（handler 不同目录） |
| 风险 | 🟠 中 |

---

- 目标：按账号订阅推送 status/log/account-log，替代 Socket.IO 房间。
- 源（现状）：`admin.js` Socket.IO(:718)、握手校验 `x-admin-token`、`subscribeSocketToAccount`(:617, 房间 `account:<id>`/`account:all`)、`emitRealtimeStatus/Log/AccountLog`(:121-145)；前端 `stores/status.ts` 事件名。
- 目标（Go）：`internal/httpapi/realtime/hub.go` + 路由 `/ws`（或 `/events` SSE）。
- 实现要点：
  - `Hub`：连接注册表按 `accountID` 分组（含 `all`）；`Broadcast(accountID, event, payload)`；握手校验 token + per-account 访问权（非特权仅可订阅授权账号）。
  - 事件语义保留：`status:update`/`log:new`/`account-log:new` + snapshot；帧用 JSON `{event, data}`。
  - P4-04 的 StatusState 变更与日志产生调 `Hub.Broadcast`（等价现状引擎回调 → emitRealtime*）。
  - 传输二选一：原生 WS（`coder/websocket`）或 SSE（日志/状态单向，SSE 更简单）——**在卡内定夺并记录选择**，前端 P8 适配对应封装。
- 不要做：不引入 socket.io 协议兼容层（前端一并换掉，见 P8）。
- 验收：`go test`——订阅/广播/权限；手动用 wscat/curl 验证事件流。
- 完成判据：☐ 按账号订阅 ☐ 三事件+snapshot ☐ 握手鉴权 ☐ 状态/日志接入广播。
