# 卡片 P3-03：yyb Service 门面（同进程 API）

| 调度头 | 值 |
| --- | --- |
| 波次 | **W6** |
| 前置依赖 | P3-01、P3-02 |
| 独占文件 | `internal/yyb/service.go` |
| 可与谁并发 | P3-02 |
| 风险 | 🟡 中 |

---

- 目标：把原 HTTP handler 的能力封装成 Go 接口，供后端内部直调，无 HTTP、无 Bearer 中转。
- 源（现状）：`yyb-go/internal/httpapi/app.go` 的 `wxapp/getCode`、`qr/*`、`accounts/*`；Node 消费点 `admin-yyb-routes.js`、`worker-manager.js:162`。
- 目标（Go）：`internal/yyb/service.go`。
- 实现要点：
  - `Service` 接口：`GetCode(ctx, openid, appID) (code, error)`、`QRCreate(ctx) (session, error)`、`QRPoll(ctx, id)`、`QRConfirm(ctx, id)`、`ListAccounts(ctx)`、`DeleteAccount(ctx, ref)`、`RefreshLoginBuffer`、`GetPhoneNumber`、`OperateWxData`。
  - 内部复用 protocol/qr 子包 + `store`；**去掉 Bearer/Token 层**（进程内无需自我鉴权，鉴权由 P6 主后端的 `x-admin-token` 统一负责）。
  - 保留 QR 会话 TTL（5m）、scan 超时（180s）等常量。
- 不要做：不再监听独立端口；不保留 `/token`/`/health` 明文 token 暴露（安全隐患，01 §5）。
- 验收：`go test`——mock 协议层下 GetCode/QR 流程闭环。
- 完成判据：☐ Service 接口就位 ☐ 无独立端口 ☐ 无 Bearer 中转 ☐ 复用协议子包。
