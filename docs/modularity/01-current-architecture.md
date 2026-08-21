# 01 · 现状架构（迁移前 · 源清单）

本文是迁移的**源清单**：Go 改造时对照它逐模块搬运。所有 file:line 均为当前 Node 仓库真实位置。

## 1. 顶层拓扑

当前是 pnpm workspace 单体仓，三块技术栈：

```
┌─────────────────────────────────────────────────────────────────┐
│  浏览器 (Vue3 SPA)                                                 │
│  web/dist  ── axios /api/*  ─┐   ── socket.io /socket.io ─┐        │
└──────────────────────────────┼───────────────────────────┼───────┘
                               │                           │
┌──────────────────────────────▼───────────────────────────▼───────┐
│  Node 主进程  core/client.js (FARM_WORKER!=1)                      │
│  ┌───────────────────────────────────────────────────────────┐   │
│  │ Express + Socket.IO 管理服务  controllers/admin.js          │   │
│  │  · 静态托管 web/dist + SPA fallback                          │   │
│  │  · 40+ admin-*-routes.js 手工注册                            │   │
│  │  · Socket.IO 房间 account:<id>                               │   │
│  └───────────────────────────────────────────────────────────┘   │
│  ┌───────────────────────────────────────────────────────────┐   │
│  │ runtime/  引擎层                                             │   │
│  │  runtime-engine(组装) · runtime-state(共享态) ·              │   │
│  │  worker-manager(进程生命周期+IPC+重连) ·                     │   │
│  │  data-provider(唯一边界对象, ~90 方法)                       │   │
│  └───────────────────────────────────────────────────────────┘   │
│         │ IPC (proc.send / postMessage)  ▲ sendToMaster            │
│         ▼                                 │                        │
│  ┌───────────────────────────────────────────────────────────┐   │
│  │ Node 子进程 × N   core/core/worker.js (FARM_WORKER=1)        │   │
│  │  每账号一个：1 条 WS → QQ 游戏网关                            │   │
│  │  utils/network.js(WS+userState单例) → tsdk-runtime(WASM安全) │   │
│  │  → utils/proto.js(protobufjs) → 25+ services/*.js            │   │
│  └───────────────────────────────────────────────────────────┘   │
│         │ 共享磁盘 JSON                                            │
│         ▼  models/store.js · user-store.js → core/data/*.json      │
└───────────────────────────────────────────────────────────────────┘
        │ HTTP /api/yyb/* 代理 (admin-yyb-routes.js)
        ▼
┌───────────────────────────────────────────────────────────────────┐
│  yyb-go  独立 Go 进程 (Gin, :8450)                                 │
│  微信 MMTLS/WMPF 协议 + 扫码 OAuth + wxapp/getCode                  │
│  SQLite: resource/db/yyb.db (wechat_accounts / sessions / features) │
└───────────────────────────────────────────────────────────────────┘
```

## 2. 进程模型

- **单一入口** `core/client.js`：按 `FARM_WORKER==='1'` 分叉（`client.js:14,17`）。
  - 主进程：license 校验 → `createRuntimeEngine` → `runtimeEngine.start()` → 起 admin 服务。**本身不持有游戏连接。**
  - Worker 进程/线程：一账号一个，持有唯一 WS，跑全部自动化循环，应答 RPC。账号身份仅靠 `process.env.FARM_ACCOUNT_ID`。
- **spawn**（`worker-manager.js:41`）：两种模式——`worker_threads`（默认）或 `child_process.fork`；线程模式把 `send/kill` shim 成 `postMessage/terminate`，使上层对两者无感。
- **Main↔Worker 仅消息传递**（无共享内存 / 无 socket / 进程间无文件）：
  - Main→Worker：`start` / `config_sync` / `stop` / `api_call`。
  - Worker→Main：`log` / `status_sync` / `ws_error` / `account_kicked` / `automation_patch` / `api_response` 等。
- **RPC**：`callWorkerApi(accountId, method, ...args)`（`worker-manager.js:786`）自增 `reqId`，超时默认 10s；worker 端 `handleApiCall`（`worker.js:792`）是一个 **300 行 switch + 47 个内联 require**。
- **状态位置**：
  - 仅主进程：`workers` 注册表、`globalLogs`/`accountLogs`（各 cap 2000）、admin sessions/tokens、socket 房间、重连计数、config revision。
  - 仅 worker：WS 连接 + `userState`、`statusData`、在途自动化、per-worker `logs`（cap 1000）。
  - 共享磁盘（非 IPC）：`models/store.js` 的 JSON 文件——两进程都 `require` 同一文件读写，仅靠 `writeJsonFileAtomic`，无锁。

## 3. HTTP 服务装配（`admin.js`, 760 行）

- **手工 wiring，非注册表**：`startAdminServer(dataProvider)`（`admin.js:314`）是 ~440 行过程：建 app → 三个 helper 工厂（session / account-access / route-helpers）→ **逐个调用 30+ 个 `register*Routes`**（`admin.js:428–614`），每个手传一份定制依赖。加新路由要改这里 2–3 处。
- **中间件顺序显式**：CORS → 静态(`web/dist`) → `/game-config`+`/game-asset` CDN 代理 → 登录资源 → 鉴权路由 → 健康检查 → **鉴权门**（allowlist `PUBLIC_API_PATHS`）→ 超时守卫 → 全部业务路由 → SPA fallback。
- **Socket.IO**（`admin.js:718`）：握手中间件校验 `x-admin-token`；连接后按 `account:<id>` 分房；`emitRealtimeStatus/Log/AccountLog` 三个模块级函数向房间广播。`client.js:35-46` 把引擎回调接到它们。
- **鉴权/会话**（`admin-session-manager.js`）：纯内存——`Set<token>` + `Map<token,user>`，`crypto.randomBytes(24)`。**重启即失效、不跨进程共享。** 5 分钟清扫过期。

## 4. 领域 / 模型层（services + models）

- **单账号单进程是核心约束**：一 worker = 一登录账号，`userState` 是 `network.js:123` 的模块单例。多账号靠多进程；但 `store.js` 把所有账号配置写在同一份 `store.json`，各进程靠 `resolveAccountId()`→`FARM_ACCOUNT_ID` 只读自己那片（`store.js:498-502`）。
- **God 对象 `store.js`（2116 行，~89 导出，17 文件依赖）**：账号 CRUD、per-account 自动化/策略/间隔、黑名单、好友 GID/狗信息/列表三套磁盘缓存、全局 wx/系统配置、防倒卖、离线提醒、UI 主题、license 位……全部 JSON 持久化。层级倒挂：model 反向依赖 `services/json-db`（`store.js:5`）。
- **`user-store.js`（1201 行）**：SaaS 租户层——面板用户、PBKDF2、登录限流/锁定、**卡密（card-key）授权系统**、硬编码超管、登录审计。用非原子 `fs.writeFileSync`（与 store 的原子写不一致）。
- **协议栈**：`service → network.sendMsgAsync → cryptoWasm/tsdkRuntime(加密+Token) → ws`；入站 `ws → network.handleMessage → types.*.decode → networkEvents.emit`。`utils/network.js` 名义在 utils，实为基础设施服务，还反向 require 了 `services/{ace,scheduler,status,stats}`。
- **`services/activity.js`（2300 行）= 七个无关限时活动塞一起**：南瓜商店 / 荷露抽奖兑换 / 青梅酿酒 / 季节通行证 / 节气 / 观星礼录 / 星砂，外加约一半篇幅是手写 protobuf 线格式解码器（`readProtoFields` 等）。天然可按「一活动一模块 + 共享解码器 + 共享 `operateActivity`」拆分。
- **服务域划分**（约 40 个文件）：
  - 农场/种植：`farm-api` `farm-land-analyzer` `farm-fertilizer` `planting-service` `farming-orchestrator` `farm-scheduler` `analytics`。
  - 好友（最密子图）：`friend-api` `friend-operation-limits` `friend-land-analyzer` `friend-visit` `friend-orchestrator`(fan-out 10) `golden-bug-service`。
  - 商城/货币：`mall` `mystery-shop`+`mystery-scheduler` `monthcard` `qqvip` `share` `invite` `illustrated` `career-api`。
  - 仓库：`warehouse`（被 friend/planting/task/mall/activity 广泛复用）。
  - 任务：`task`。
  - 基础设施：`scheduler`(全局注册表单例) `logger`(单例) `status`(单例) `stats`(单例) `json-db` `license` `email` `push` `account-resolver` `qrlogin` `ace-service`。

## 5. yyb-go 与「两个项目」问题

- **yyb-go**：Go 1.23 + Gin + 纯 Go SQLite（`modernc.org/sqlite`，无 CGO，可静态编译）。实现完整微信 MMTLS/WMPF 协议（`internal/protocol/`）+ 扫码 OAuth（`internal/qr`），暴露 `POST /wxapp/getCode`（Node 真正消费的端点）、`/qr/*`、`/accounts/*`。Bearer 鉴权（`YYB_API_TOKEN`）。默认端口 8000，但部署统一用 `-port 8450`。持久化在 `resource/db/yyb.db`（`wechat_accounts` / `sessions` / `features`）。
- **Node↔yyb-go 胶水**：`admin-yyb-routes.js` 把 `/api/yyb/*` 代理到 `YYB_API_URL`，转发时带 `Bearer YYB_API_KEY`；凭据优先取前端传入的 `apiBase/apiKey`，否则回退 env。`worker-manager.js:162 refreshYybCodeIfNeeded` 在每次 worker 启动前刷新登录 code（重连也走它）。
- **部署路径 × 是否含 yyb-go**：

  | 路径 | 含 yyb-go | 机制 |
  | --- | --- | --- |
  | Docker compose | ✅ | 3 阶段 Dockerfile 分别构建 Node+web 与 Go 静态二进制；`start.sh` 后台起 `yyb-go -port 8450 &` 再 `exec node client.js`；tini 作 PID1 |
  | pkg 二进制(exe) | ❌ | `pkg` 只打包 Node + `web/dist` + 静态资源，**无法内嵌 Go 二进制** |
  | 源码运行(pnpm dev:core) | ❌ | 只 `node client.js`，不构建 / 不启动 yyb-go |

  **痛点确证**：仅 Docker 捆绑两者；二进制 / 源码用户必须**自行单独部署 yyb-go**（独立 Go 工具链、独立端口、独立 SQLite / Token），再在前端「应用宝」页手填 apiBase + Token。这就是「需分别部署两个项目」的根因。
- **Docker 无进程守护**：一个 shell + 一个 `&`；yyb-go 崩溃不会自愈，健康检查只探 Node 的 3007。

## 6. 前端（web, Vue3+Vite+TS+Pinia+UnoCSS）

- **规模**：11 视图（7648 行）、13 Pinia store（3328）、10 composable（2894）、~75 组件。四个巨石：`Login.vue`(1489)、`AccountModal.vue`(1433)、`Friends.vue`(1413)、`Dashboard.vue`(1399)。
- **API 层是单文件**（`api/index.ts`, 77 行）：一个 axios 实例 `baseURL:'/'`；拦截器注入 `x-admin-token` + `x-account-id`；401→清 token 硬跳 `/login`。**无按域拆分的 API 模块**，事实上的「API 模块」就是各 Pinia store。
- **Socket.IO 只在 `stores/status.ts`**：`io('/',{path:'/socket.io',autoConnect:false})`；事件 `status:update` / `log:new` / `account-log:new` / `*:snapshot`。**socket 生命周期仅 Dashboard 拥有，且从不 disconnect**——其他页依赖 `realtimeConnected` 却不负责建连，非 Dashboard 激活时静默退化为轮询。
- **状态管理**：`user.ts`（认证+后台 CRUD 双职责）、`friend.ts`（数据+业务逻辑）、`setting.ts`（巨型扁平 state，默认值写两遍）偏重。多 store 各自重复实现 `isCurrentAccount()` 陈旧响应丢弃守卫。
- **巨石成因**：`Login.vue` 52% 是装饰性动画 CSS + 登录/注册/找回/续费/领卡/更新日志六合一；`AccountModal.vue` 五种登录流 + 15 处直连 `api.*` 无 store；`Friends.vue` 黑名单/访客/三个 Teleport 弹窗内联，`FriendsFriendList` 被下钻 16 props(含 8 函数 props)；`Dashboard.vue` 引 6 store，内联日志控制台与今日统计引擎。已被 `progress.md` 点名继续拆分。
- **服务托管**：`admin.js` 用 `express.static(web/dist)` + SPA fallback（`/api`、`/game-config` 除外返回 404）。生产同源，dev 靠 Vite proxy 转发到 `:3007`。

## 7. 协议与安全运行时资产（迁移最高风险项）

- **22 个 `.proto`（1591 行）**：`plantpb`(328) `activitypb`(198) `friendpb`(149) `taskpb`(126) `userpb`(123) … `acepb` `notifypb`。当前用 protobufjs 运行时加载；Go 侧改用 `protoc-gen-go` 预编译，**反而更简单更快**。
- **3 个 WASM 安全运行时**：`tsdk.wasm`/`tsdk-legacy.wasm`(各 117K)、`tsdk-v3.8.2.wasm`(158K)。通过 `WebAssembly.instantiate(wasm, imports)` 加载（`tsdk-runtime.js:338`）。
- **关键利好**：`core/docs/tsdk-ace-runtime.md` 已把 **22 个宿主 import（a.a–a.v）+ 12 个导出（A/B/C/G/M/N/O/aa/ba/ca/da/ea）** 逐一规格化，含内存所有权规则、初始化 6 步、ACE 生命周期节拍（5s/25s/30s/150s/180s）。这意味着 Go 迁移**不需重新实现反作弊加密**，只需用 **wazero（纯 Go WASM 运行时）加载同一批 `.wasm`，把这 22 个宿主函数用 Go 重写**。这张表直接喂给 P2 最高风险任务卡。
