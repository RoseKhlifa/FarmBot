# P3 · yyb 收编任务卡

目标：把独立的 `yyb-go` 进程收编为主 Go 后端的**内部包 `internal/yyb`**，通过**同进程函数调用**取代跨进程 HTTP 代理 + 双份 Token 布线。
这是消除「需分别部署两个项目」痛点的核心阶段。收编后源码/二进制/Docker 三种部署都自带 yyb 能力。

> 现状（来自 01 §5）：`yyb-go` 是 Gin + 纯 Go SQLite 的独立进程（默认 8000/部署 8450），实现微信 MMTLS/WMPF 协议 + 扫码 OAuth，暴露 `wxapp/getCode`（Node 真正消费）、`qr/*`、`accounts/*`。Node 侧 `admin-yyb-routes.js` 代理 + `worker-manager.js:162 refreshYybCodeIfNeeded` 消费。

---

### 卡片 P3-01：协议/扫码子包平移
- 目标：把 yyb-go 的协议与扫码实现原样搬进主 module，编译通过。
- 前置依赖：P0-01。
- 源（现状）：`yyb-go/internal/protocol/*`（mmtls/ilink/shortlink/transport/newdns/loginbuffer/bytes）、`yyb-go/internal/qr/*`、`yyb-go/internal/store/*`。
- 目标（Go）：`internal/yyb/protocol/`、`internal/yyb/qr/`、`internal/yyb/store.go`。
- 实现要点：
  - 直接复制源码，改包路径与 import 到 `github.com/<owner>/farmbot/internal/yyb/...`。这些子包本就是纯 Go 且相对自洽，改动主要是 import 路径与 package 名。
  - 合并依赖到根 `go.mod`（P0-01 已合并 gin/modernc 等；此处补齐 protocol 用到的加密/编码库）。
  - 保留 OAuth 常量（appid `wxd44977328b36e647`、回调 `yybadaccess.3g.qq.com`、小程序 `wx5306c5978fdb76e4`）。
- 不要做：不重写微信协议实现；不改协议常量。
- 验收：`go build ./internal/yyb/...` 通过；yyb-go 原有单测（若有）迁移后通过。
- 完成判据：☐ 三子包平移 ☐ import 路径修正 ☐ 编译通过。

---

### 卡片 P3-02：微信账号存储并入主库
- 目标：yyb 的 `wechat_accounts`/`sessions`/`features` 并入主 SQLite（或同库独立表），消除双份身份。
- 前置依赖：P3-01、P1-02。
- 源（现状）：`yyb-go/internal/store/store.go`（自带 SQLite，表 `wechat_accounts`(openid UNIQUE)/`sessions`/`features`）。
- 目标（Go）：`internal/yyb/store.go` 改为走主库 `internal/store` 的连接；表定义并入 `0001_init.sql`（P1-02 已列 `wechat_accounts`）。
- 实现要点：
  - 把 yyb 的建表/查询迁到主库连接；`sessions`（per account+tcp_proxy 的 MMTLS blob）、`features`（getCode/getPhoneNumber/operateWxData 开关）一并纳入。
  - farmbot 账号通过 `yyb_openid` 关联微信账号（P1-02 `accounts.yyb_openid`），实现单一身份。
  - 头像/QR 图片目录仍可用磁盘（`<DataDir>/yyb/{avatars,qr}`），无需入库。
- 不要做：不保留 yyb 的独立 db 文件；不做双写。
- 验收：`go test`——微信账号 upsert/list/delete 往返；与主库同事务无冲突。
- 完成判据：☐ 表并入主库 ☐ openid 关联就位 ☐ 独立 db 移除。

---

### 卡片 P3-03：yyb Service 门面（同进程 API）
- 目标：把原 HTTP handler 的能力封装成 Go 接口，供后端内部直调，无 HTTP、无 Bearer 中转。
- 前置依赖：P3-01、P3-02。
- 源（现状）：`yyb-go/internal/httpapi/app.go` 的 `wxapp/getCode`、`qr/*`、`accounts/*`；Node 消费点 `admin-yyb-routes.js`、`worker-manager.js:162`。
- 目标（Go）：`internal/yyb/service.go`。
- 实现要点：
  - `Service` 接口：`GetCode(ctx, openid, appID) (code, error)`、`QRCreate(ctx) (session, error)`、`QRPoll(ctx, id)`、`QRConfirm(ctx, id)`、`ListAccounts(ctx)`、`DeleteAccount(ctx, ref)`、`RefreshLoginBuffer`、`GetPhoneNumber`、`OperateWxData`。
  - 内部复用 protocol/qr 子包 + `store`；**去掉 Bearer/Token 层**（进程内无需自我鉴权，鉴权由 P6 主后端的 `x-admin-token` 统一负责）。
  - 保留 QR 会话 TTL（5m）、scan 超时（180s）等常量。
- 不要做：不再监听独立端口；不保留 `/token`/`/health` 明文 token 暴露（安全隐患，01 §5）。
- 验收：`go test`——mock 协议层下 GetCode/QR 流程闭环。
- 完成判据：☐ Service 接口就位 ☐ 无独立端口 ☐ 无 Bearer 中转 ☐ 复用协议子包。

---

### 卡片 P3-04：账号启动接入 yyb（替代 refreshYybCodeIfNeeded）
- 目标：账号启动/重连前刷新登录 code 改为直调 yyb Service。
- 前置依赖：P3-03、P2-05。
- 源（现状）：`worker-manager.js:162 refreshYybCodeIfNeeded`（内置账号走 globalWxConfig，thirdparty 走第三方 API）。
- 目标（Go）：在 P4 `account/manager.go` 启动流程调用 `yyb.Service.GetCode(...)` 得到 code 再交 `session.Login`。
- 实现要点：
  - 内置账号：直调 `yyb.Service.GetCode(openid, appID)`。
  - 第三方 provider：保留一个 `ThirdPartyProvider` 接口（对齐 `utils/thirdpartyYyb.js`），可插拔；默认走内置 yyb。
- 不要做：不保留 `YYB_API_URL/KEY` 环境布线（内部直调）。
- 验收：账号启动流程单测用 mock yyb Service 返回 code；真机在 P4 出口联调。
- 完成判据：☐ 启动/重连直调 yyb ☐ 第三方 provider 可插拔 ☐ env 布线移除。

> 结果核对：完成 P3 后，应能在**不启动任何外部 yyb 进程**的情况下，于主后端内完成扫码登录与 getCode。这是「两个项目合一」的验证点。
