# 卡片 P8-01：实时通道客户端（替代 socket.io-client）

| 调度头 | 值 |
| --- | --- |
| 波次 | **W12** |
| 前置依赖 | P6-03 |
| 独占文件 | `web/src/realtime/client.ts`、`web/src/composables/useRealtime.ts`（+ 改 `stores/status.ts`） |
| 可与谁并发 | P8-02（不同文件；status.ts 改动小，注意与 P8-06 store 瘦身错峰） |
| 风险 | 🟠 中 |

---

- 目标：把前端从 Socket.IO 切到 Go 的原生 WS/SSE（P6-03 定的传输），并修复生命周期缺陷。
- 源（现状）：`stores/status.ts` 的 `io(...)`、事件 `status:update`/`log:new`/`account-log:new`/`*:snapshot`、`connectRealtime`/`disconnectRealtime`。
- 目标（前端）：新增 `web/src/realtime/client.ts`（轻量 WS/SSE 封装）+ `web/src/composables/useRealtime.ts`。
- 实现要点：
  - 封装 connect/subscribe(accountId)/disconnect/自动重连；保留事件名语义，帧解析 `{event,data}`。
  - **修复**：把 socket 生命周期从 Dashboard 提到 **app 级 composable**（在 `DefaultLayout` 或 app 初始化建连，登出/卸载 disconnect），根治「仅 Dashboard 建连、从不 disconnect、其他页静默退化轮询」。
  - `stores/status.ts` 改为消费该 composable，不再自持 socket。
- 不要做：不保留 socket.io 依赖；不在多个页面各自建连。
- 验收：`pnpm -C web build` + `lint` 通过；实时事件在任意页面在线时到达；登出正确断开。
- 完成判据：☐ WS/SSE 封装 ☐ app 级生命周期 ☐ 事件语义保留 ☐ socket.io 移除。
