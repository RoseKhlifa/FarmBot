# 02 · 目标架构（Go 后端 + TS 前端）

## 1. 一句话形态

一个 **Go 二进制**，内嵌前端 `web/dist`、WASM 安全运行时、静态游戏数据；对外单端口（默认 3007）
同时提供 REST（Gin）+ 实时（WebSocket/SSE）+ SPA 托管；原 `yyb-go` 作为 `internal/yyb` 包在
**同进程内**被直接函数调用，不再有跨进程 HTTP 与双 Token。前端仍是 Vue3+Vite+TS，继续按协调器模式拆分。

## 2. 目标拓扑

```
┌────────────────────────────────────────────────────────────────┐
│  浏览器 (Vue3 SPA · 保留)                                         │
│   REST /api/*  ───────────────┐   WS /ws (实时) ───────────────┐ │
└───────────────────────────────┼────────────────────────────────┼┘
                                │                                │
┌───────────────────────────────▼────────────────────────────────▼┐
│  farmbot (单一 Go 二进制)   cmd/farmbot/main.go = 仅 wiring       │
│                                                                  │
│  internal/httpapi (Gin)   ── 领域 handler + 中间件 + 实时 Hub    │
│        │  依赖注入唯一入口: Application 容器                       │
│        ▼                                                         │
│  internal/app  Application{ AccountManager, Stores, Realtime,    │
│                             Config, Yyb, Auth }  ← 组合根          │
│        │                                                         │
│   ┌────┴───────────────┬───────────────┬─────────────────────┐  │
│   ▼                    ▼               ▼                     ▼  │
│ internal/account   internal/domain  internal/game       internal/yyb
│  Manager           farm/friend/     protocol(protoc-    (原 yyb-go
│  goroutine-per-    warehouse/mall/  gen-go) · ws传输 ·   收编: 扫码/
│  account +         task/activity/   tsdk(wazero WASM) ·  wxapp/getCode
│  Runtime(会话/     ...(接口化)      ace · 登录会话        直接函数调用)
│  调度/自动化循环)                                                 │
│        │                                                         │
│        ▼                                                         │
│  internal/store  (SQLite via sqlc/GORM) ← 单库多表, 替代散落 JSON │
│        │                                                         │
│        ▼   embed: web/dist · *.wasm · gameConfig · proto 生成码   │
└──────────────────────────────────────────────────────────────────┘
                        │ 单文件 data/farmbot.db (+ 运行时目录)
                        ▼
                   一份 SQLite（账号/用户/卡密/缓存/统计/微信账号）
```

对比现状消除的东西：**独立 yyb-go 进程**、**跨进程 IPC + RPC switch**、**双份 Token 布线**、
**散落 JSON 文件 + 无锁并发写**、**pkg 与 Docker 部署能力不一致**。

## 3. 六个关键工程决策

### 决策 1：并发模型 — 进程/线程 → goroutine-per-account
现状「一账号一 OS 进程/线程 + IPC」直接映射为 Go 的「一账号一组 goroutine + channel」。
- 每账号一个 `account.Runtime`，持有自己的 WS 连接、`UserState`、TSDK 实例、调度器。
- `account.Manager` 持有 `map[string]*Runtime` + `sync.RWMutex`，负责 start/stop/restart/重连退避。
- **RPC switch 消失**：admin handler 直接调用 `manager.Get(id).Farm().Harvest(ctx)` 这样的**类型安全方法**，
  取代 `callWorkerApi(id,'method',...args)` 的字符串契约。跨账号并发天然安全，不再依赖「单进程单例」这一脆弱前提。
- 模块级可变单例（`userState`/`ws`/`statusData`/`schedulerRegistry`）全部降为 `Runtime` 的**实例字段**。

### 决策 2：持久化 — 散落 JSON → 单一 SQLite
现状 `store.js`/`user-store.js`/`stats.js` 各写一堆 JSON，无锁、原子性不一致。目标：
- 一份 `data/farmbot.db`（SQLite，WAL）。表：`accounts` `account_config` `users` `cards` `login_logs`
  `friend_cache` `blacklist` `stats` `global_config` `wechat_accounts`(收编 yyb) 等（schema 见 P1 卡）。
- 访问走 **repository 接口**（`store.AccountRepo` / `store.UserRepo` …），领域层只依赖接口，便于测试与替换。
- 纯 Go 驱动 `modernc.org/sqlite`（yyb-go 已在用，无 CGO，保证跨平台静态编译）。
- 提供一次性 **JSON→SQLite 迁移器**（读旧 `core/data/*.json` 导入），保证老用户平滑升级。

### 决策 3：游戏协议 — protobufjs 运行时 → protoc-gen-go 预编译
- 22 个 `.proto` 用 `protoc` + `protoc-gen-go` 生成 `internal/game/pb/*.pb.go`，编译期类型安全，去掉运行时反射加载。
- `.proto` 源文件原样保留在 `proto/`，生成码提交到仓库（避免使用者装 protoc）。

### 决策 4：安全运行时（最高风险）— WASM 保留 + wazero 宿主用 Go 重写
- **不重写反作弊加密**。继续加载官方 `tsdk-v3.8.2.wasm`，改用 **wazero**（纯 Go、无 CGO 的 WASM 运行时）。
- 依据 `core/docs/tsdk-ace-runtime.md` 的规格表，用 Go 重写 22 个宿主 import（a.a–a.v）与 12 个导出的调用封装、
  内存所有权、初始化 6 步、ACE 节拍（5/25/30/150/180s）。每账号一个 wazero 实例，随重连销毁重建。
- 校验 SHA-256 与导出表，失败显式中止（保持现状「不回退伪 Token」的安全姿态）。

### 决策 5：实时通道 — Socket.IO → 原生 WebSocket（或 SSE）
- Go 无一等 Socket.IO 服务端，改用原生 WS（`nhooyr/coder websocket` 或 `gorilla/websocket`）承载
  `status/log/account-log` 三类事件；按账号订阅（等价现状 `account:<id>` 房间）。
- 前端把 `socket.io-client` 换成轻量 WS 封装（协议自定义 JSON 帧），或用 SSE（日志/状态是单向推送，SSE 足够且更简单）。
- **把 socket 生命周期从 Dashboard 提到 app 级 composable**，修复「仅 Dashboard 建连、从不 disconnect」缺陷（P8）。

### 决策 6：yyb 收编 — 独立进程 → 内部包直调
- `yyb-go/internal/*` 平移为 `internal/yyb/*`（同一 Go module），protocol/qr/store 子包基本原样复用。
- 删除 Node 的 `admin-yyb-routes.js` 代理与 `YYB_API_URL/KEY/TOKEN` 布线；`/api/yyb/*` 由 Go handler
  **直接调用 `yyb.Service.GetCode(...)`**，无 HTTP 跳转、无 Bearer 中转。
- yyb 的 `wechat_accounts` 并入主库（或同库独立表），与 farmbot 账号通过 `yybOpenid` 关联，消除双份身份。
- 结果：源码 / 二进制 / Docker **三种方式都自带 yyb 能力**，彻底解决「部署两个项目」。

## 4. 技术选型（建议，非强制）

| 关注点 | 选型 | 理由 |
| --- | --- | --- |
| HTTP 框架 | Gin | yyb-go 已用，团队已熟；中间件生态足够 |
| WS 客户端(连游戏) | `gorilla/websocket` | 成熟稳定，对二进制帧支持好 |
| WS 服务端(推前端) | `coder/websocket` 或 SSE | 轻量；日志/状态单向推送 SSE 更省心 |
| WASM 运行时 | **wazero** | 纯 Go 无 CGO，保证 pkg 式单文件跨平台 |
| SQLite | `modernc.org/sqlite` | 纯 Go 无 CGO，与 wazero 一致的静态编译诉求 |
| DB 访问 | sqlc（首选）或 GORM | sqlc 生成类型安全查询，贴合「预编译」哲学 |
| protobuf | protoc-gen-go | 官方，编译期安全 |
| 配置 | env + 结构体（viper 可选） | 沿用现有 env 变量名，降低迁移认知负担 |
| 日志 | slog（Go 1.21+ 标准库） | 无三方依赖；保留 secret 脱敏封装 |
| 前端嵌入 | `embed.FS` | 单二进制内嵌 `web/dist` 与 WASM |
| 构建 | 单一 `go build` + `vite build` | 取代 3 阶段 Dockerfile + pkg |

## 5. 保持不变的对外契约

迁移期前端**尽量不改 API 形状**，降低联调成本：
- REST 路径与 JSON 响应结构对齐现状（`/api/status` 缺账号 200、其它资源缺账号 400 等既有行为逐路由核对）。
- 鉴权头 `x-admin-token` / 账号头 `x-account-id` 保留语义。
- 实时事件名 `status:update` / `log:new` / `account-log:new` 保留（帧封装可换传输，事件语义不变）。
- env 变量名（`ADMIN_PORT`/`ADMIN_PASSWORD`/`FARM_DATA_DIR`/`YYB_*`/`FARM_TSDK_*`）沿用，含义映射到 Go 配置。

> 例外：`FARM_WORKER`/`FARM_RUNTIME_MODE`/`FARM_ACCOUNT_ID` 这类「多进程」专用变量在 Go 里失去意义（goroutine 模型），迁移后废弃并在文档标注。
