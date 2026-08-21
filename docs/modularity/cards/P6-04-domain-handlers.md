# 卡片 P6-04：领域 handler（一域一文件，对齐现状路由）

| 调度头 | 值 |
| --- | --- |
| 波次 | **W12**（每组 handler 依赖对应 P5 域搬完） |
| 前置依赖 | P6-02、对应 P5-* 域 |
| 独占文件 | `internal/httpapi/handlers/*.go`（一域一文件，可按域拆给多个会话） |
| 可与谁并发 | 自身可按域高度并发（account.go / friend.go / farm.go … 各占一文件） |
| 风险 | 🟠 中（契约对齐是重点） |

---

- 目标：把 40+ 路由模块重写为一域一文件的 handler，直调领域服务。
- 源（现状）：`controllers/admin-*-routes.js`（account/friend/farm/bag/mall/task/activity/illustrated/career/analytics/settings/system/user/card/login-log/capture/qr/proxy/public/shop-* 等）+ `data-provider.js`(~90 方法边界)。
- 目标（Go）：`internal/httpapi/handlers/*.go`（见 03 目录树清单）。
- 实现要点：
  - 每个 handler 结构体持有它需要的领域服务接口（由 Application 注入），方法即路由处理函数。
  - **契约对齐**：逐路由核对 HTTP 方法/路径/请求体/响应 JSON 与现状一致；保留既有状态码语义（如 `/api/status` 缺账号 200、其它资源缺账号 400、`账号未运行` 不弹 toast 等，前端拦截器依赖）。
  - 现状 `data-provider` 是唯一边界对象——Go 对等物是「Application 暴露的领域服务接口集合」，handler 只经它访问 AccountManager/领域服务，不直接碰 Runtime map。
  - 按批切换：每完成一组 handler，更新绞杀代理分流表（04 §3）把对应 `/api/xxx` 前缀指向 Go。
- 并发建议：本卡可拆成多会话，每会话领 1–3 个域的 handler，独占各自 `handlers/<域>.go`；共享的响应/错误辅助放 `handlers/common.go`（由第一个开工的会话建，其余只读）。
- 不要做：不在 handler 写业务算法（下沉领域）；不一次性切全部路由。
- 验收：**契约快照测试**——对每组路由，Node 与 Go 同请求响应 JSON 结构一致（P8 前建快照）；`go test ./internal/httpapi/...`。
- 完成判据：☐ 全部现状路由有 Go handler ☐ 契约快照一致 ☐ 边界只经 Application ☐ 分流表覆盖。
