# 卡片 P2-04：WS 传输 + 入站分发

| 调度头 | 值 |
| --- | --- |
| 波次 | **W4** |
| 前置依赖 | P2-01、P2-02 |
| 独占文件 | `internal/game/transport/{ws,dispatch}.go` |
| 可与谁并发 | P1-03, P1-04, P2-03 |
| 风险 | 🔴 高（心跳/退避/kickout 时序错会掉线/被踢） |

---

- 目标：单连接 WS 客户端，承载 `sendMsgAsync` 语义与心跳/重连；入站 notify 分发。
- 源（现状）：`core/src/utils/network.js`(849)——`sendMsgAsync`、`userState`、心跳、`networkEvents`、`handleNotify`(:352)。
- 目标（Go）：`internal/game/transport/{ws.go,dispatch.go}`。
- 实现要点：
  - `gorilla/websocket` 建连游戏网关；请求体先经 tsdk `encrypt`，网关 `auth_token` 复刻 `AceManager.randomStr()`（64–127 字母数字 + `=`），首条 `AllLands` 用 `_get_encrypted_init_info` 一次性凭据（照 `tsdk-ace-runtime.md` §网关）。
  - `SendMsg(ctx, cmd, req) (resp, error)`：自增 seq、pending map、超时、解密响应 → protobuf decode。等价现状 `sendMsgAsync` 的唯一 RPC 原语。
  - `dispatch.go`：入站帧 decode 后按类型分发（等价 `handleNotify`），用 Go channel/回调把 `sell`/`farmHarvested`/`kickout`/`disconnect`/`taskInfoNotify` 等事件送给 Runtime（替代 `networkEvents` EventEmitter）。
  - 心跳与重连交给 P4 Runtime 编排；transport 只负责连接与读写，`UserState` 作为 transport/Runtime 的实例字段（**不再模块单例**）。保留 5s/25s/30s/150s/180s 节拍常量。
- 不要做：不把业务逻辑放进 transport；不用全局单例连接。
- 验收：单测用 mock server 验证 seq/超时/解密链路；集成验收在 P2 出口真机联调。
- 完成判据：☐ SendMsg 原语可用 ☐ 入站分发到事件 ☐ UserState 实例化。
