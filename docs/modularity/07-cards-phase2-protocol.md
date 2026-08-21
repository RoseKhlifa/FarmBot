# P2 · 协议内核任务卡（最高风险 · 地基）

目标：把游戏协议栈从 Node 搬到 Go——protobuf 预编译、WS 传输、**TSDK/ACE 安全运行时（wazero 加载同一 WASM）**、登录会话。
所有上层领域都依赖它，必须最先打通并**真机验证**。关键依据：`core/docs/tsdk-ace-runtime.md`（已把 22 宿主 import + 12 导出规格化）。

---

### 卡片 P2-01：protoc-gen-go 生成 22 个协议
- 目标：`.proto` 源改用 Go 官方编译，得到编译期类型安全的 `*.pb.go`。
- 前置依赖：P0-04。
- 源（现状）：`core/src/proto/*.proto`（22 个，1591 行）+ `utils/proto.js`（protobufjs 运行时加载 `types` map）。
- 目标（Go）：`proto/*.proto`（平移）→ `internal/game/pb/*.pb.go`（生成，提交入库）。
- 实现要点：
  - 平移 22 个 `.proto` 到 `proto/`；为每个补 `option go_package`（如 `.../internal/game/pb;pb`）。
  - `make gen-proto`：`protoc --go_out=... proto/*.proto`。**生成码提交仓库**，使用者无需装 protoc。
  - 注意现状可能存在的嵌套/opaque 字段（`activitypb` 返回 bytes，见 P5 activity）。
- 不要做：不手写 protobuf 结构；不改 `.proto` 的 message/field 编号（wire 兼容命脉）。
- 验收：`make gen-proto` 无错；`go build ./internal/game/pb/...` 通过。
- 完成判据：☐ 22 个 pb.go 生成 ☐ go_package 正确 ☐ 编译通过。

---

### 卡片 P2-02：TSDK 安全运行时（wazero 加载 WASM）——🔴 最高风险
- 目标：用纯 Go 的 wazero 加载官方 `tsdk-v3.8.2.wasm`，实现 22 个宿主 import 与 12 个导出封装，**不重写加密算法**。
- 前置依赖：P0-03。
- 源（现状）：`core/src/utils/tsdk-runtime.js`(594)、`crypto-wasm.js`、`core/src/utils/*.wasm`；**规格权威**：`core/docs/tsdk-ace-runtime.md`。
- 目标（Go）：`internal/game/tsdk/{runtime.go,imports.go,verify.go}` + `assets/wasm/`（go:embed）。
- 实现要点：
  - `verify.go`：校验 SHA-256（`705e326c...4850a5f`）、导入数量、必要导出；失败**显式中止**（保持现状不回退伪 Token 姿态）。
  - `runtime.go`：wazero 实例化；按官方顺序——实例化 22 个 `a.a`–`a.v` 宿主 → mergewasm 解密 17 段 → `__wasm_call_ctors` → `SdkInitEx(3167,0)` 写 appKey → `_init_runtime(3167, appKeyPtr)`。
  - 12 个导出封装：create/destroy buffer(A/B)、getResult(C, 64B, WASM 所有)、init(G)、heartbeat(M)、getDataToServer(N, WASM 所有)、sendDataFromServer(O)、generateToken(aa)、encrypt/decrypt in-place(ba/ca)、encrypt/decrypt v2(da/ea)。**严格按内存所有权规则**：调用者分配的输入与 Token 返回指针复制后 `_destroy_buffer`；`C`/`N` 返回的 WASM 内存不由 Go 释放；复制前校验指针/长度/memory 边界。
  - `imports.go`：逐一实现 a.a–a.v（assertion/read-write file 到账号独立目录、JS stack、version `v3.8.2.1783066265`、clock、device info、app id `1112386029`、TQOS 异步 POST 等，逐行照规格表）。
  - **每账号一个 tsdk 实例**（字段挂在 P4 Runtime），随重连销毁重建。`FARM_TSDK_ACE_ENABLED=false` 关闭整链。
- 不要做：不试图逆向或重写 WASM 内部算法；不共享单例实例给多账号。
- 验收：**离线单测**——固定输入下 `encrypt/decrypt/generateToken` 与 Node `tsdk-runtime.js` 输出**逐字节一致**（先在 Node 侧 dump 若干 fixture）；SHA256/导出校验用例。
- 完成判据：☐ 三 wasm 校验通过 ☐ 12 导出封装正确 ☐ 22 import 实现 ☐ 与 Node 输出对拍一致。

---

### 卡片 P2-03：ACE 生命周期服务
- 目标：移植反作弊心跳/上报节拍。
- 前置依赖：P2-02。
- 源（现状）：`core/src/services/ace-service.js` + `tsdk-ace-runtime.md` §网关与 ACE。
- 目标（Go）：`internal/game/ace/service.go`。
- 实现要点：
  - 节拍常量：每 5s 检查 `_get_data_to_server` 非空则调 `gamepb.acepb.AceService.AntiData`；`_process_received_data` 5s；`_send_heartbeat_tick` 25s；速度检测 30s；状态上报 150s；函数检查 180s。普通用户心跳独立 25s。
  - 同账号最多一个在途 AntiData，失败 2–30s 有限退避。服务器 `reply.data` 原样回灌 `_send_data_from_server`。
  - 用 Go timer/ticker + context 取消；挂在 Runtime 上，随账号停止/重连销毁。
- 不要做：不改节拍时长；不并发多条 AntiData。
- 验收：单测断言各 ticker 周期与在途去重逻辑；与 Node 行为对照。
- 完成判据：☐ 六类节拍就位 ☐ 在途去重+退避 ☐ 随账号生命周期销毁。

---

### 卡片 P2-04：WS 传输 + 入站分发
- 目标：单连接 WS 客户端，承载 `sendMsgAsync` 语义与心跳/重连；入站 notify 分发。
- 前置依赖：P2-01、P2-02。
- 源（现状）：`core/src/utils/network.js`(849)——`sendMsgAsync`、`userState`、心跳、`networkEvents`、`handleNotify`(:352)。
- 目标（Go）：`internal/game/transport/{ws.go,dispatch.go}`。
- 实现要点：
  - `gorilla/websocket` 建连游戏网关；请求体先经 tsdk `encrypt`，网关 `auth_token` 复刻 `AceManager.randomStr()`（64–127 字母数字 + `=`），首条 `AllLands` 用 `_get_encrypted_init_info` 一次性凭据（照 `tsdk-ace-runtime.md` §网关）。
  - `SendMsg(ctx, cmd, req) (resp, error)`：自增 seq、pending map、超时、解密响应 → protobuf decode。等价现状 `sendMsgAsync` 的唯一 RPC 原语。
  - `dispatch.go`：入站帧 decode 后按类型分发（等价 `handleNotify`），用 Go channel/回调把 `sell`/`farmHarvested`/`kickout`/`disconnect`/`taskInfoNotify` 等事件送给 Runtime（替代 `networkEvents` EventEmitter）。
  - 心跳与重连交给 P4 Runtime 编排；transport 只负责连接与读写，`UserState` 作为 transport/Runtime 的实例字段（**不再模块单例**）。
- 不要做：不把业务逻辑放进 transport；不用全局单例连接。
- 验收：单测用 mock server 验证 seq/超时/解密链路；集成验收在 P2 出口真机联调。
- 完成判据：☐ SendMsg 原语可用 ☐ 入站分发到事件 ☐ UserState 实例化。

---

### 卡片 P2-05：登录会话
- 目标：完成从登录 code 到游戏在线的握手会话。
- 前置依赖：P2-02、P2-04。
- 源（现状）：`network.js` 登录流程 + `tsdk-ace-runtime.md` 初始化 6 步。
- 目标（Go）：`internal/game/session/login.go`。
- 实现要点：
  - 输入登录 code（来自 P3 yyb），执行 `AnoUserLogin(0, openId)` 绑定账号，登录请求用 TSDK 请求体加密；成功后启动 ACE 生命周期与数据轮询。
  - 暴露 `Login(ctx, code) (*Session, error)`，Session 持 openid/uin/在线态，交给 Runtime。
- 不要做：不在此处启动自动化循环（那是 P4）。
- 验收：**P2 阶段出口真机验收**——单账号用真实 code 登录成功，跑通首条 `AllLands` + 一次好友操作，ACE 心跳无告警 ≥30 分钟。
- 完成判据：☐ 登录握手成功 ☐ AllLands 正常 ☐ 好友操作成功 ☐ ACE 稳定。

> ⚠️ go/no-go 关口：P2-02 + P2-05 真机验证是整个迁移的关键闸门。通过后再推进 P3+。失败则按 04 §6 折中方案，把桥限制在 TSDK 单点。
