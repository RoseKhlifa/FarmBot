# P6 · HTTP / 实时任务卡

目标：把 `controllers/admin.js`(760) + 40+ 个 `admin-*-routes.js` 手工 wiring，重构为 Gin 引擎 + 中间件链 + 一域一文件的 handler + 实时 Hub。
对外契约对齐现状（02 §5），使前端尽量不改。搬完一组 handler 即可在绞杀代理里把对应路由切到 Go。

> 现状（来自 00 §2、01 §3、01 §6）：`startAdminServer` 是 ~440 行过程，逐个调 30+ `register*Routes`；中间件顺序显式；Socket.IO 按 `account:<id>` 分房；会话纯内存重启失效；前端 API 单文件 + 事件名 `status:update`/`log:new`/`account-log:new`。

---

### 卡片 P6-01：Gin 服务器组装 + 静态托管 + SPA fallback
- 目标：立起 HTTP 服务器骨架，托管前端，优雅退出。
- 前置依赖：P0-02、P4-03。
- 源（现状）：`admin.js` startAdminServer 主干、`configureStaticAssets`(:168)、`registerSpaFallback`(:227)、`/game-config`(:368)、`/login-assets`(:409)、`/api/game-asset` CDN 代理(:391)。
- 目标（Go）：`internal/httpapi/server.go`。
- 实现要点：
  - Gin 引擎；`embed.FS` 托管 `web/dist`（P7 内嵌），SPA fallback：非 `/api`、非 `/game-config` 的 GET 回 `index.html`，`/api` 未匹配返回 404 JSON。
  - `/game-config`、`/login-assets`（保留 `X-Content-Type-Options:nosniff` + SVG 的 CSP sandbox 加固）、`/api/game-asset` CDN 图片代理。
  - 监听 `ADMIN_PORT`（默认 3007）绑定 `0.0.0.0`；`http.Server` + `context` 优雅退出（SIGINT/SIGTERM）。
  - 装配唯一入口：从 `internal/app.Application` 取依赖，handler 只依赖 Application 暴露的服务接口。
- 不要做：不手工逐个 register（改用分组路由 + handler 结构体）；不在 server.go 写业务。
- 验收：`go run` 起服务，`/api/health` 200，SPA fallback 返回 index，静态资源可取。
- 完成判据：☐ Gin 骨架 ☐ 内嵌前端托管 ☐ SPA fallback ☐ 优雅退出。

---

### 卡片 P6-02：中间件链（鉴权/账号/CORS/超时/角色）
- 目标：复刻现状显式中间件顺序与鉴权门。
- 前置依赖：P6-01、P1-04。
- 源（现状）：`admin.js` 中间件顺序、`registerAuthGate`(:179, allowlist `PUBLIC_API_PATHS`)、`admin-session-manager.js`、`admin-route-helpers.js`(`requireAdminRole`:112/`requireSuperAdminRole`:126)、`getAccountIdFromRequest`/`canAccessAccount`。
- 目标（Go）：`internal/httpapi/middleware/{cors,auth,session,account,timeout,role}.go`。
- 实现要点：
  - 顺序：CORS → 静态/资源 → 鉴权路由(login) → 健康 → **鉴权门**（`x-admin-token` 校验 + PUBLIC allowlist）→ 超时守卫 → 业务路由 → SPA fallback。
  - `auth.go`：校验 `x-admin-token`，注入 `currentUser`；封禁/过期卡用户强制失效。
  - `session.go`：**改进点**——会话可选持久化到 SQLite（修复现状重启即失效、不跨实例）；token 仍 `crypto/rand`。
  - `account.go`：解析 `x-account-id`（对齐 `resolveAccountId`），非特权角色做 per-account 访问控制。
  - `role.go`：admin / super-admin 门。
  - 5 分钟清扫过期会话（对齐现状 cleanup）。
- 不要做：不改鉴权头名；不放宽 PUBLIC allowlist（逐条对照现状）。
- 验收：`go test`——PUBLIC 路径放行、非法 token 401、角色门 403、账号访问控制。
- 完成判据：☐ 中间件顺序一致 ☐ allowlist 对齐 ☐ 会话可持久化 ☐ 角色/账号门。

---

### 卡片 P6-03：实时 Hub（WS/SSE 替代 Socket.IO）
- 目标：按账号订阅推送 status/log/account-log，替代 Socket.IO 房间。
- 前置依赖：P6-02、P4-04。
- 源（现状）：`admin.js` Socket.IO(:718)、握手校验 `x-admin-token`、`subscribeSocketToAccount`(:617, 房间 `account:<id>`/`account:all`)、`emitRealtimeStatus/Log/AccountLog`(:121-145)；前端 `stores/status.ts` 事件名。
- 目标（Go）：`internal/httpapi/realtime/hub.go` + 路由 `/ws`（或 `/events` SSE）。
- 实现要点：
  - `Hub`：连接注册表按 `accountID` 分组（含 `all`）；`Broadcast(accountID, event, payload)`；握手校验 token + per-account 访问权（非特权仅可订阅授权账号）。
  - 事件语义保留：`status:update`/`log:new`/`account-log:new` + snapshot；帧用 JSON `{event, data}`。
  - P4-04 的 StatusState 变更与日志产生调 `Hub.Broadcast`（等价现状引擎回调 → emitRealtime*）。
  - 传输二选一：原生 WS（`coder/websocket`）或 SSE（日志/状态单向，SSE 更简单）——在卡内定夺，前端 P8 适配对应封装。
- 不要做：不引入 socket.io 协议兼容层（前端一并换掉，见 P8）。
- 验收：`go test`——订阅/广播/权限；手动用 wscat/curl 验证事件流。
- 完成判据：☐ 按账号订阅 ☐ 三事件+snapshot ☐ 握手鉴权 ☐ 状态/日志接入广播。

---

### 卡片 P6-04：领域 handler（一域一文件，对齐现状路由）
- 目标：把 40+ 路由模块重写为一域一文件的 handler，直调领域服务。
- 前置依赖：P6-02、P5-*（对应域搬完才切该 handler）。
- 源（现状）：`controllers/admin-*-routes.js`（account/friend/farm/bag/mall/task/activity/illustrated/career/analytics/settings/system/user/card/login-log/capture/qr/proxy/public/shop-* 等）+ `data-provider.js`(~90 方法边界)。
- 目标（Go）：`internal/httpapi/handlers/*.go`（见 03 目录树清单）。
- 实现要点：
  - 每个 handler 结构体持有它需要的领域服务接口（由 Application 注入），方法即路由处理函数。
  - **契约对齐**：逐路由核对 HTTP 方法/路径/请求体/响应 JSON 与现状一致；保留既有状态码语义（如 `/api/status` 缺账号 200、其它资源缺账号 400、`账号未运行` 不弹 toast 等，01 §6 前端拦截器依赖）。
  - 现状 `data-provider` 是唯一边界对象——Go 对等物是「Application 暴露的领域服务接口集合」，handler 只经它访问 AccountManager/领域服务，不直接碰 Runtime map。
  - 按批切换：每完成一组 handler，更新绞杀代理分流表（04 §3）把对应 `/api/xxx` 前缀指向 Go。
- 不要做：不在 handler 写业务算法（下沉领域）；不一次性切全部路由。
- 验收：**契约快照测试**——对每组路由，Node 与 Go 同请求响应 JSON 结构一致（P8 前建快照）；`go test ./internal/httpapi/...`。
- 完成判据：☐ 全部现状路由有 Go handler ☐ 契约快照一致 ☐ 边界只经 Application ☐ 分流表覆盖。

---

### 卡片 P6-05：Application 组合根
- 目标：把所有依赖装配集中到一个组合根，`main.go` 只调用它。
- 前置依赖：P6-01~04、P3-03、P5-08。
- 源（现状）：`runtime-engine.js`(组装) + `client.js`(顶层 wiring)。
- 目标（Go）：`internal/app/application.go` + `internal/app/wire.go`。
- 实现要点：
  - `Application` 持有并按依赖顺序构造：Config → Stores(repos) → game 工厂 → yyb.Service → AccountManager → 领域服务工厂 → Realtime Hub → Auth/Session → HTTP Server。
  - 领域服务与 Runtime 的注入在此定义（谁依赖谁一目了然，替代现状散落的回调注入）。
  - `cmd/farmbot/main.go` 仅：读 config → `app.New(cfg)` → `app.Run(ctx)` → 等待信号 → `app.Shutdown()`。
- 不要做：不在 main.go 放逻辑；不用包级 init 做隐式装配。
- 验收：`go build`；`go run ./cmd/farmbot` 全链路起服务并可优雅退出。
- 完成判据：☐ 组合根集中装配 ☐ main 仅 wiring ☐ 全链路可起可停。
